package collector

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"

	"ctifeed/internal/config"
	"ctifeed/internal/ioc"
	"ctifeed/internal/model"
	"ctifeed/internal/scorer"
)

// Collector, RSS/Atom beslemelerinin eşzamanlı çekilmesini ve ayrıştırılmasını yönetir.
type Collector struct {
	cfg        *config.Config
	httpClient *http.Client
}

// Result, bir besleme toplama döngüsünün sonuçlarını barındırır.
type Result struct {
	Articles     []*model.Article
	FeedsSuccess int
	FeedsFailed  int
	TotalFetched int
	FilteredOld  int
	Duration     time.Duration
}

// feedJob, işçi havuzu (worker pool) için besleme kaynağını sarmalar.
type feedJob struct {
	source model.FeedSource
}

// feedResult, tek bir besleme kaynağından toplanan haberleri taşır.
type feedResult struct {
	source   model.FeedSource
	articles []*model.Article
	oldCnt   int
	err      error
}

// headerTransport, standart tarayıcı feed okuyucularını taklit etmek için HTTP başlıkları ekler.
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

// New, özel HTTP istemcisine sahip yeni bir Collector örneği oluşturur.
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

// CollectAll, tanımlı tüm beslemeleri paralel olarak çekmek ve işlemek için işçi havuzunu (worker pool) koordine eder.
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

	// İşçi havuzunu başlat
	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.worker(ctx, workerID, jobs, results)
		}(w)
	}

	// Tüm besleme kaynaklarını kuyruğa ekle
	for _, src := range sources {
		jobs <- feedJob{source: src}
	}
	close(jobs)

	// İşçileri ayrı bir goroutine içinde bekle ve sonuç kanalını kapat
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

// worker, kanal boşalana veya context iptal edilene kadar besleme işlerini yürütür.
func (c *Collector) worker(ctx context.Context, id int, jobs <-chan feedJob, results chan<- feedResult) {
	for job := range jobs {
		select {
		case <-ctx.Done():
			results <- feedResult{source: job.source, err: ctx.Err()}
			return
		default:
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

// fetchFeed, zaman aşımı korumasıyla tek bir beslemeyi çeker ve maddelerini işler.
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

		// Filtre: Yalnızca son 48 saat içinde yayımlanmış haberler işleme alınır
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

		// Tam içerik üzerinden (özet 500 karaktere budanmadan önce) IoC çıkarımı yap
		fullText := cleanTitle + " " + item.Description + " " + item.Content
		extractedIoCs := ioc.Extract(fullText)

		// Önizleme için aşırı uzun özetleri kısalt
		if len(cleanSummary) > 500 {
			cleanSummary = cleanSummary[:497] + "..."
		}

		// Siber tehdit puanını ve atanan etiketleri hesapla
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

// parseItemDate, gofeed.Item içerisindeki PublishedParsed ve UpdatedParsed tarihlerini inceler.
func parseItemDate(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil && !item.PublishedParsed.IsZero() {
		return *item.PublishedParsed
	}
	if item.UpdatedParsed != nil && !item.UpdatedParsed.IsZero() {
		return *item.UpdatedParsed
	}
	return time.Time{}
}
