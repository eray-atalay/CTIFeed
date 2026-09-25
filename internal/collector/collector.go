package collector

import (
	"context"
	"crypto/tls"
	"fmt"
	"html"
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

// TelegramCursorProvider supplies the latest known post ID from persistent storage.
type TelegramCursorProvider interface {
	GetLatestTelegramPostID(ctx context.Context, source string) (int, error)
}

// SourceProvider supplies active feed sources and records diagnostic health metrics.
type SourceProvider interface {
	GetActiveSources(ctx context.Context) ([]model.FeedSource, error)
	UpdateSourceHealth(ctx context.Context, url string, status string, latencyMs int64, articleCount int, lastError string) error
}

// Collector manages concurrent fetching and parsing of CTI feeds.
type Collector struct {
	cfg            *config.Config
	httpClient     *http.Client
	cursorDB       TelegramCursorProvider
	sourceProvider SourceProvider
	latestIDMu     sync.Mutex
	latestIDs      map[string]int
}

// SetCursorProvider attaches a persistent storage provider for tracking Telegram cursors.
func (c *Collector) SetCursorProvider(provider TelegramCursorProvider) {
	c.cursorDB = provider
}

// SetSourceProvider attaches a persistent storage provider for dynamic sources and health reporting.
func (c *Collector) SetSourceProvider(provider SourceProvider) {
	c.sourceProvider = provider
}

// Result represents the aggregated outcome of a feed collection cycle.
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
	source    model.FeedSource
	articles  []*model.Article
	oldCnt    int
	latencyMs int64
	err       error
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

// New initializes a Collector with configured HTTP client and timeout settings.
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
		latestIDs:  make(map[string]int),
	}
}

// CollectAll fetches all configured feed sources concurrently.
func (c *Collector) CollectAll(ctx context.Context) Result {
	startTime := time.Now()
	sources := c.cfg.Sources

	// Prefer dynamically configured active sources from storage if available
	if c.sourceProvider != nil {
		if activeSources, err := c.sourceProvider.GetActiveSources(ctx); err == nil && len(activeSources) > 0 {
			sources = activeSources
		}
	}

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
		status := "ok"
		errStr := ""
		if res.err != nil {
			failedCount++
			status = "error"
			errStr = res.err.Error()
			slog.Warn("Feed collection failed",
				slog.String("source", res.source.Name),
				slog.String("url", res.source.URL),
				slog.String("error", errStr),
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
				slog.Int64("latency_ms", res.latencyMs),
			)
		}

		if c.sourceProvider != nil {
			_ = c.sourceProvider.UpdateSourceHealth(ctx, res.source.URL, status, res.latencyMs, len(res.articles), errStr)
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
			start := time.Now()
			var articles []*model.Article
			var oldCnt int
			var err error

			// Check whether target is a Telegram channel or standard RSS/Atom
			if strings.HasPrefix(job.source.URL, "telegram://") || strings.Contains(job.source.URL, "t.me/s/") {
				articles, oldCnt, err = c.fetchTelegramFeed(ctx, job.source)
			} else {
				articles, oldCnt, err = c.fetchFeed(ctx, job.source)
			}
			latencyMs := time.Since(start).Milliseconds()

			results <- feedResult{
				source:    job.source,
				articles:  articles,
				oldCnt:    oldCnt,
				latencyMs: latencyMs,
				err:       err,
			}
		}
	}
}

// fetchTelegramFeed parses public Telegram channel web previews or ID-based posts.
func (c *Collector) fetchTelegramFeed(parentCtx context.Context, src model.FeedSource) ([]*model.Article, int, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 45*time.Second)
	defer cancel()

	channelURL := src.URL
	channelName := strings.TrimPrefix(channelURL, "telegram://")

	cleanCh := channelName
	var seedID int
	if strings.Contains(cleanCh, "?latest=") {
		parts := strings.Split(cleanCh, "?latest=")
		cleanCh = parts[0]
		_, _ = fmt.Sscanf(parts[1], "%d", &seedID)
	} else if strings.Contains(cleanCh, "?seed=") {
		parts := strings.Split(cleanCh, "?seed=")
		cleanCh = parts[0]
		_, _ = fmt.Sscanf(parts[1], "%d", &seedID)
	} else if strings.Contains(cleanCh, "?") {
		cleanCh = cleanCh[:strings.Index(cleanCh, "?")]
	}

	cleanCh = strings.TrimPrefix(cleanCh, "https://t.me/s/")
	cleanCh = strings.TrimPrefix(cleanCh, "http://t.me/s/")
	cleanCh = strings.TrimPrefix(cleanCh, "https://t.me/")
	cleanCh = strings.TrimPrefix(cleanCh, "http://t.me/")
	cleanCh = strings.Trim(cleanCh, "/")

	// Channels requiring ID-based scraping (e.g. breachdetect which redirects /s/ preview) or with seed/latest specified
	if cleanCh == "breachdetect" || src.Category == "Telegram Breach" || seedID > 0 {
		latestID := c.resolveLatestTelegramID(ctx, src, cleanCh, seedID)
		if latestID > 0 {
			return c.fetchTelegramByID(ctx, src, cleanCh, latestID)
		}
	}

	// Standard public channel preview
	targetURL := fmt.Sprintf("https://t.me/s/%s", cleanCh)

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
		return nil, 0, fmt.Errorf("telegram status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	htmlContent := string(body)
	msgRegex := regexp.MustCompile(`(?s)<div class="[^"]*tgme_widget_message_text[^"]*"[^>]*>(.*?)</div>`)
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

		link := fmt.Sprintf("https://t.me/%s/%d", cleanCh, i)
		if i < len(postLinks) {
			link = "https://t.me/" + postLinks[i][1]
		}

		pubDate := time.Now().UTC()
		if i < len(times) {
			if parsedT, err := time.Parse(time.RFC3339, times[i][1]); err == nil {
				pubDate = parsedT
			}
		}
		if pubDate.After(time.Now().Add(5 * time.Minute)) {
			pubDate = time.Now().UTC()
		}

		if !pubDate.IsZero() && pubDate.Before(cutoff) {
			oldCnt++
			continue
		}

		lines := strings.Split(cleanSummary, "\n")
		cleanTitle := strings.TrimSpace(lines[0])
		if len(cleanTitle) > 120 {
			cleanTitle = cleanTitle[:117] + "..."
		}
		if cleanTitle == "" {
			cleanTitle = fmt.Sprintf("[%s] Yeni Tehdit", src.Name)
		}

		extractedIoCs := ioc.Extract(cleanSummary)
		scoringResult := scorer.Evaluate(cleanTitle, cleanSummary)

		articles = append(articles, &model.Article{
			Source:      src.Name,
			Title:       cleanTitle,
			Link:        link,
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

func (c *Collector) resolveLatestTelegramID(ctx context.Context, src model.FeedSource, channel string, seedID int) int {
	c.latestIDMu.Lock()
	cachedID := c.latestIDs[channel]
	c.latestIDMu.Unlock()

	if cachedID > seedID {
		seedID = cachedID
	}

	if c.cursorDB != nil {
		if dbID, err := c.cursorDB.GetLatestTelegramPostID(ctx, src.Name); err == nil && dbID > seedID {
			seedID = dbID
		}
	}

	// Safe fallback baseline for breachdetect
	if channel == "breachdetect" && seedID < 1286500 {
		seedID = 1286500
	}

	latestID := c.probeLatestTelegramID(ctx, channel, seedID)
	if latestID > 0 {
		c.latestIDMu.Lock()
		if latestID > c.latestIDs[channel] {
			c.latestIDs[channel] = latestID
		}
		c.latestIDMu.Unlock()
	}

	return latestID
}

func isTelegramPostHTMLValid(body string) bool {
	if strings.Contains(body, "tgme_page_additional") || strings.Contains(body, "You can view and join") {
		return false
	}
	if strings.Contains(body, "Monitoring and detection of threats that occur through the main channels") {
		return false
	}
	if strings.Contains(body, "widget_actions_helper") || strings.Contains(body, "widget_actions") {
		return true
	}
	return false
}

func isTelegramPostContentValid(rawText string) bool {
	if len(rawText) < 15 {
		return false
	}
	if strings.HasPrefix(rawText, "You can view and join") {
		return false
	}
	if strings.Contains(rawText, "Monitoring and detection of threats that occur through the main channels") {
		return false
	}
	return true
}

func (c *Collector) checkTelegramPostExists(ctx context.Context, channel string, id int) bool {
	postURL := fmt.Sprintf("https://t.me/%s/%d", channel, id)
	req, err := http.NewRequestWithContext(ctx, "GET", postURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return false
	}

	return isTelegramPostHTMLValid(string(body))
}

func (c *Collector) probeLatestTelegramID(ctx context.Context, channel string, seedID int) int {
	if seedID <= 0 {
		seedID = 1286500
	}

	// 1. Verify if seed exists
	if !c.checkTelegramPostExists(ctx, channel, seedID) {
		low := seedID - 500
		if low < 1 {
			low = 1
		}
		if !c.checkTelegramPostExists(ctx, channel, low) {
			return seedID
		}
		return c.binarySearchTelegramID(ctx, channel, low, seedID)
	}

	// 2. Exponential step forward to find upper bound
	low := seedID
	step := 10
	high := low + step

	for {
		select {
		case <-ctx.Done():
			return low
		default:
		}

		if c.checkTelegramPostExists(ctx, channel, high) {
			low = high
			step *= 2
			high = low + step
		} else {
			break
		}
	}

	// 3. Binary search between low and high
	return c.binarySearchTelegramID(ctx, channel, low, high)
}

func (c *Collector) binarySearchTelegramID(ctx context.Context, channel string, low, high int) int {
	best := low
	for low <= high {
		select {
		case <-ctx.Done():
			return best
		default:
		}

		if low == high {
			if c.checkTelegramPostExists(ctx, channel, low) {
				return low
			}
			return best
		}

		mid := low + (high-low)/2
		if mid == low {
			if c.checkTelegramPostExists(ctx, channel, high) {
				return high
			}
			return low
		}

		if c.checkTelegramPostExists(ctx, channel, mid) {
			best = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	return best
}

// fetchTelegramByID parses channel posts concurrently via OpenGraph meta tags.
func (c *Collector) fetchTelegramByID(ctx context.Context, src model.FeedSource, channel string, latestID int) ([]*model.Article, int, error) {
	descRegex := regexp.MustCompile(`<meta property="og:description" content="([^"]+)"`)
	contentRegex := regexp.MustCompile(`(?i)"Content":\s*"([^"]+)"`)
	dateRegex := regexp.MustCompile(`(?i)"Detection Date":\s*"([^"]+)"`)
	typeRegex := regexp.MustCompile(`(?i)"Type":\s*"([^"]+)"`)
	sourceForumRegex := regexp.MustCompile(`(?i)"Source":\s*"([^"]+)"`)
	authorRegex := regexp.MustCompile(`(?i)"author":\s*"([^"]+)"`)
	timeRegex := regexp.MustCompile(`datetime="([^"]+)"`)

	totalPosts := 30
	type postResult struct {
		article *model.Article
	}

	resChan := make(chan postResult, totalPosts)
	var wg sync.WaitGroup

	// Fetch posts concurrently
	for i := 0; i < totalPosts; i++ {
		postID := latestID - i
		if postID <= 0 {
			break
		}
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			postURL := fmt.Sprintf("https://t.me/%s/%d", channel, id)
			req, err := http.NewRequestWithContext(ctx, "GET", postURL, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", c.cfg.UserAgent)

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return
			}

			body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
			if err != nil {
				return
			}

			htmlStr := string(body)
			if !isTelegramPostHTMLValid(htmlStr) {
				return
			}

			m := descRegex.FindStringSubmatch(htmlStr)
			if len(m) < 2 {
				return
			}

			rawText := html.UnescapeString(m[1])
			rawText = strings.TrimSpace(rawText)
			if !isTelegramPostContentValid(rawText) {
				return
			}

			title := ""
			if cSub := contentRegex.FindStringSubmatch(rawText); len(cSub) > 1 {
				title = cSub[1]
			}
			if title == "" {
				lines := strings.Split(rawText, "\n")
				title = strings.TrimSpace(lines[0])
			}
			if len(title) > 120 {
				title = title[:117] + "..."
			}

			pubDate := time.Now().UTC()
			if tSub := timeRegex.FindStringSubmatch(htmlStr); len(tSub) > 1 {
				if t, err := time.Parse(time.RFC3339, strings.TrimSpace(tSub[1])); err == nil {
					pubDate = t.UTC()
				}
			} else if dSub := dateRegex.FindStringSubmatch(rawText); len(dSub) > 1 {
				if t, err := time.Parse("02 Jan 2006", strings.TrimSpace(dSub[1])); err == nil {
					now := time.Now().UTC()
					if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
						pubDate = now
					} else {
						pubDate = t.UTC()
					}
				}
			}
			if pubDate.After(time.Now().Add(5 * time.Minute)) {
				pubDate = time.Now().UTC()
			}

			rawLower := strings.ToLower(rawText)

			detectedType := ""
			if tSub := typeRegex.FindStringSubmatch(rawText); len(tSub) > 1 {
				detectedType = strings.ToLower(strings.TrimSpace(tSub[1]))
			}

			// Akıllı Dark Web Kategori Sınıflandırması
			categoryTag := "data-leak"
			if strings.Contains(detectedType, "ransomware") || strings.Contains(rawLower, "lockbit") || strings.Contains(rawLower, "published a new victim") || strings.Contains(rawLower, "ransomware") {
				categoryTag = "ransomware"
			} else if strings.Contains(detectedType, "combolist") || strings.Contains(rawLower, "combolist") || strings.Contains(rawLower, "combo list") || strings.Contains(rawLower, "webmail login") || strings.Contains(rawLower, "stealer") || strings.Contains(rawLower, "email:pass") || strings.Contains(rawLower, "user:pass") {
				categoryTag = "combolist"
			} else if strings.Contains(rawLower, "network access") || strings.Contains(rawLower, "rdp access") || strings.Contains(rawLower, "vpn access") || strings.Contains(rawLower, "initial access") || strings.Contains(rawLower, "domain admin") || strings.Contains(rawLower, "access for sale") {
				categoryTag = "initial-access"
			} else if strings.Contains(rawLower, "sql dump") || strings.Contains(rawLower, "database dump") || strings.Contains(rawLower, "db dump") || (strings.Contains(rawLower, "lines") && strings.Contains(rawLower, "database")) {
				categoryTag = "database-dump"
			} else if strings.Contains(rawLower, "socks5") || strings.Contains(rawLower, "proxies") || strings.Contains(rawLower, "botnet") {
				categoryTag = "infra-proxy"
			}

			extractedIoCs := ioc.Extract(rawText)
			scoringResult := scorer.Evaluate(title, rawText)

			tags := scoringResult.Tags
			tags = append(tags, categoryTag, "darkweb")
			if categoryTag != "data-leak" {
				tags = append(tags, "leak")
			}

			// Kaynak Dark Web Forumu ve Tehdit Aktörü Tespiti
			if sSub := sourceForumRegex.FindStringSubmatch(rawText); len(sSub) > 1 {
				cleanForum := strings.TrimSpace(sSub[1])
				cleanForum = strings.ReplaceAll(cleanForum, "[.]", ".")
				cleanForum = strings.ReplaceAll(cleanForum, "[dot]", ".")
				tags = append(tags, "forum:"+cleanForum)
			}
			if aSub := authorRegex.FindStringSubmatch(rawText); len(aSub) > 1 {
				cleanAuthor := strings.TrimSpace(aSub[1])
				cleanAuthor = strings.Trim(cleanAuthor, " ()")
				if cleanAuthor != "" && len(cleanAuthor) < 30 {
					tags = append(tags, "actor:"+cleanAuthor)
				}
			}

			resChan <- postResult{
				article: &model.Article{
					Source:      src.Name,
					Title:       title,
					Link:        postURL,
					Summary:     rawText,
					Score:       scoringResult.Score,
					Tags:        tags,
					PublishedAt: pubDate,
					CreatedAt:   time.Now().UTC(),
					IoCs:        extractedIoCs,
				},
			}
		}(postID)
	}

	wg.Wait()
	close(resChan)

	var articles []*model.Article
	for r := range resChan {
		if r.article != nil {
			articles = append(articles, r.article)
		}
	}

	return articles, 0, nil
}

// fetchFeed parses standard RSS/Atom feeds.
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

		now := time.Now().UTC()
		if pubDate.IsZero() || pubDate.After(now.Add(5*time.Minute)) {
			pubDate = now
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
