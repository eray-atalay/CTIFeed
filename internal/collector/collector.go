package collector

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"

	"ctifeed/internal/config"
	"ctifeed/internal/ioc"
	"ctifeed/internal/model"
	"ctifeed/internal/scorer"
)

// Collector, RSS/Atom ve Telegram beslemelerinin eşzamanlı çekilmesini yönetir.
type Collector struct {
	cfg        *config.Config
	httpClient *http.Client
}

// Result, bir besleme toplama döngüsünün sonucunu barındırır.
type Result struct {
	Articles     []*model.Article
	FeedsSuccess int
	FeedsFailed  int
	TotalFetched int
	FilteredOld  int
	Duration     time.Duration
}

type feedJob struct {
	source model.FeedSource
}

type feedResult struct {
	source   model.FeedSource
	articles []*model.Article
	oldCnt   int
	err      error
}

type headerTransport struct {
	base      http.RoundTripper
	userAgent string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", t.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	return t.base.RoundTrip(req)
}

// New, özel HTTP istemcisine sahip yeni bir Collector oluşturur.
func New(cfg *config.Config) *Collector {
	baseTransport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     60 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	client := &http.Client{
		Transport: &headerTransport{
			base:      baseTransport,
			userAgent: cfg.UserAgent,
		},
		Timeout: cfg.Timeout + 5*time.Second,
	}

	return &Collector{
		cfg:        cfg,
		httpClient: client,
	}
}

// CollectAll, tüm kaynakları paralel olarak çeker.
func (c *Collector) CollectAll(ctx context.Context) Result {
	startTime := time.Now()
	sources := c.cfg.Sources
	workers := c.cfg.Workers
	if workers <= 0 {
		workers = 5
	}
	if workers > len(sources) {
		workers = len(sources)
	}

	jobs := make(chan feedJob, len(sources))
	results := make(chan feedResult, len(sources))

	var wg sync.WaitGroup
	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.worker(ctx, workerID, jobs, results)
		}(w)
	}

	for _, src := range sources {
		jobs <- feedJob{source: src}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var allArticles []*model.Article
	successCount := 0
	failedCount := 0
	totalFetched := 0
	filteredOld := 0

	for res := range results {
		if res.err != nil {
			failedCount++
			slog.Warn("Feed collection failed",
				slog.String("source", res.source.Name),
				slog.String("url", res.source.URL),
				slog.String("error", res.err.Error()),
			)
		} else {
			successCount++
			allArticles = append(allArticles, res.articles...)
			totalFetched += len(res.articles) + res.oldCnt
			filteredOld += res.oldCnt
			slog.Info("Feed processed successfully",
				slog.String("source", res.source.Name),
				slog.Int("valid_items", len(res.articles)),
				slog.Int("filtered_old", res.oldCnt),
			)
		}
	}

	return Result{
		Articles:     allArticles,
		FeedsSuccess: successCount,
		FeedsFailed:  failedCount,
		TotalFetched: totalFetched,
		FilteredOld:  filteredOld,
		Duration:     time.Since(startTime),
	}
}

func (c *Collector) worker(ctx context.Context, id int, jobs <-chan feedJob, results chan<- feedResult) {
	for job := range jobs {
		select {
		case <-ctx.Done():
			results <- feedResult{source: job.source, err: ctx.Err()}
			return
		default:
			// Telegram kanali mi standart RSS mi kontrol et
			if strings.HasPrefix(job.source.URL, "telegram://") || strings.Contains(job.source.URL, "t.me/s/") {
				articles, oldCnt, err := c.fetchTelegramFeed(ctx, job.source)
				results <- feedResult{
					source:   job.source,
					articles: articles,
					oldCnt:   oldCnt,
					err:      err,
				}
			} else {
				articles, oldCnt, err := c.fetchFeed(ctx, job.source)
				results <- feedResult{
					source:   job.source,
					articles: articles,
					oldCnt:   oldCnt,
					err:      err,
				}
			}
		}
	}
}

// fetchTelegramFeed, t.me/s/<channel> sayfasindan mesajlari ceker ve Article nesnelerine donusturur.
func (c *Collector) fetchTelegramFeed(parentCtx context.Context, src model.FeedSource) ([]*model.Article, int, error) {
	ctx, cancel := context.WithTimeout(parentCtx, c.cfg.Timeout)
	defer cancel()

	channelName := strings.TrimPrefix(src.URL, "telegram://")
	channelName = strings.TrimPrefix(channelName, "https://t.me/s/")
	channelName = strings.TrimPrefix(channelName, "http://t.me/s/")
	targetURL := fmt.Sprintf("https://t.me/s/%s", channelName)

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("telegram status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	htmlContent := string(body)

	// Regex ile mesaj, tarih ve tekil post linki tespiti
	msgRegex := regexp.MustCompile(`(?s)<div class="tgme_widget_message_text[^"]*"[^>]*>(.*?)</div>`)
	linkRegex := regexp.MustCompile(`data-post="([^"]+)"`)
	timeRegex := regexp.MustCompile(`<time datetime="([^"]+)"`)

	matches := msgRegex.FindAllStringSubmatch(htmlContent, -1)
	postLinks := linkRegex.FindAllStringSubmatch(htmlContent, -1)
	times := timeRegex.FindAllStringSubmatch(htmlContent, -1)

	cutoff := time.Now().Add(-c.cfg.MaxAgeHours)
	var articles []*model.Article
	oldCnt := 0

	for i, m := range matches {
		rawText := m[1]
		cleanSummary := scorer.StripHTML(rawText)
		if cleanSummary == "" {
			continue
		}

		// Post Linki
		link := fmt.Sprintf("https://t.me/%s/%d", channelName, i)
		if i < len(postLinks) {
			link = "https://t.me/" + postLinks[i][1]
		}

		// Yayin Tarihi
		pubDate := time.Now().UTC()
		if i < len(times) {
			if parsedT, err := time.Parse(time.RFC3339, times[i][1]); err == nil {
				pubDate = parsedT
			}
		}

		if !pubDate.IsZero() && pubDate.Before(cutoff) {
			oldCnt++
			continue
		}

		// Ilk satirdan baslik cikarma
		lines := strings.Split(cleanSummary, "\n")
		cleanTitle := strings.TrimSpace(lines[0])
		if len(cleanTitle) > 120 {
			cleanTitle = cleanTitle[:117] + "..."
		}
		if cleanTitle == "" {
			cleanTitle = fmt.Sprintf("[%s] Yeni Tehdit Bildirimi", src.Name)
		}

		// IoC ve Puanlama
		extractedIoCs := ioc.Extract(cleanSummary)
		scoringResult := scorer.Evaluate(cleanTitle, cleanSummary)

		articles = append(articles, &model.Article{
			Source:      src.Name,
			Title:       cleanTitle,
			Link:        link, // Link UNIQUE oldugu icin SQLite mukerrerleri otomatik eler!
			Summary:     cleanSummary,
			Score:       scoringResult.Score,
			Tags:        scoringResult.Tags,
			PublishedAt: pubDate.UTC(),
			CreatedAt:   time.Now().UTC(),
			IoCs:        extractedIoCs,
		})
	}

	return articles, oldCnt, nil
}

// fetchFeed, standart RSS/Atom beslemelerini ceker.
func (c *Collector) fetchFeed(parentCtx context.Context, src model.FeedSource) ([]*model.Article, int, error) {
	ctx, cancel := context.WithTimeout(parentCtx, c.cfg.Timeout)
	defer cancel()

	fp := gofeed.NewParser()
	fp.Client = c.httpClient
	fp.UserAgent = c.cfg.UserAgent

	feed, err := fp.ParseURLWithContext(src.URL, ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("parse feed error: %w", err)
	}

	if feed == nil || len(feed.Items) == 0 {
		return nil, 0, nil
	}

	cutoff := time.Now().Add(-c.cfg.MaxAgeHours)
	var articles []*model.Article
	oldCnt := 0

	for _, item := range feed.Items {
		if item == nil {
			continue
		}

		pubDate := parseItemDate(item)
		if !pubDate.IsZero() && pubDate.Before(cutoff) {
			oldCnt++
			continue
		}

		if pubDate.IsZero() {
			pubDate = time.Now().UTC()
		}

		cleanTitle := scorer.StripHTML(item.Title)
		link := item.Link
		if link == "" && len(item.Links) > 0 {
			link = item.Links[0]
		}
		if link == "" {
			continue
		}

		cleanSummary := scorer.StripHTML(item.Description)
		if cleanSummary == "" && item.Content != "" {
			cleanSummary = scorer.StripHTML(item.Content)
		}

		fullText := cleanTitle + " " + item.Description + " " + item.Content
		extractedIoCs := ioc.Extract(fullText)

		if len(cleanSummary) > 500 {
			cleanSummary = cleanSummary[:497] + "..."
		}

		scoringResult := scorer.Evaluate(cleanTitle, cleanSummary)

		article := &model.Article{
			Source:      src.Name,
			Title:       cleanTitle,
			Link:        link,
			Summary:     cleanSummary,
			Score:       scoringResult.Score,
			Tags:        scoringResult.Tags,
			PublishedAt: pubDate.UTC(),
			CreatedAt:   time.Now().UTC(),
			IoCs:        extractedIoCs,
		}

		articles = append(articles, article)
	}

	return articles, oldCnt, nil
}

func parseItemDate(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil && !item.PublishedParsed.IsZero() {
		return *item.PublishedParsed
	}
	if item.UpdatedParsed != nil && !item.UpdatedParsed.IsZero() {
		return *item.UpdatedParsed
	}
	return time.Time{}
}