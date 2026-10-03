package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"ctifeed/internal/collector"
	"ctifeed/internal/config"
	"ctifeed/internal/ioc"
	"ctifeed/internal/model"
	"ctifeed/internal/storage"
	"ctifeed/web"
)

// Server coordinates HTTP web endpoints and REST API routes.
type Server struct {
	cfg       *config.Config
	db        *storage.DB
	collector *collector.Collector
	notifier  interface {
		DispatchAlert(ctx context.Context, articles []*model.Article)
	}
	scanMu sync.Mutex
	server *http.Server
}

// NewServer creates and initializes a new Server instance.
func NewServer(cfg *config.Config, db *storage.DB, col *collector.Collector, addr string) *Server {
	if cfg == nil {
		cfg = config.NewDefaultConfig()
	}
	if strings.TrimSpace(cfg.AdminJWTSecret) == "" {
		secret, err := randomJWTSecret()
		if err != nil {
			slog.Error("Failed to generate admin JWT secret", slog.String("error", err.Error()))
		} else {
			cfg.AdminJWTSecret = secret
			slog.Warn("CTIFEED_ADMIN_JWT_SECRET is not configured; admin sessions will reset on restart")
		}
	}
	s := &Server{
		cfg:       cfg,
		db:        db,
		collector: col,
	}

	if col != nil && db != nil {
		col.SetSourceProvider(db)
	}

	mux := http.NewServeMux()

	// REST API routes
	mux.HandleFunc("GET /api/stats", s.handleGetStats)
	mux.HandleFunc("GET /api/analytics", s.handleGetAnalytics)
	mux.HandleFunc("GET /api/iocs", s.handleGetIoCs)
	mux.HandleFunc("GET /api/iocs/export", s.handleExportIoCs)
	mux.HandleFunc("GET /api/sources", s.handleGetSources)
	mux.HandleFunc("POST /api/admin/login", s.handleAdminLogin)
	mux.HandleFunc("POST /api/admin/logout", s.handleAdminLogout)
	mux.HandleFunc("GET /api/admin/status", s.handleGetAdminStatus)
	mux.HandleFunc("POST /api/sources", s.handleAddSource)
	mux.HandleFunc("DELETE /api/sources", s.handleDeleteSource)
	mux.HandleFunc("POST /api/sources/toggle", s.handleToggleSource)
	mux.HandleFunc("GET /api/articles", s.handleGetArticles)
	mux.HandleFunc("POST /api/scan", s.handlePostScan)
	mux.HandleFunc("GET /api/rss/twitter/{username}", s.handleGetTwitterRSS)

	// Static asset file server from embedded assets
	staticFS, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		slog.Error("Failed to create static sub-FS", slog.String("error", err.Error()))
	}
	fileServer := http.FileServer(http.FS(staticFS))
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/admin.html"
		fileServer.ServeHTTP(w, r)
	})
	mux.HandleFunc("GET /admin/", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/admin.html"
		fileServer.ServeHTTP(w, r)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Eğer istek API ile başlıyorsa yönlendirme yapma
		if strings.HasPrefix(path, "/api/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Kök dizin veya dosya uzantısı içeren istekler (örn: .js, .css, .ico) doğrudan sunulur
		if path == "/" || path == "" || strings.Contains(path, ".") {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Çok sayfalı yapımız için fiziksel HTML dosyasını kontrol et (örn: /radars/cve -> /radars/cve.html)
		htmlPath := strings.TrimPrefix(path, "/") + ".html"
		if _, err := fs.Stat(staticFS, htmlPath); err == nil {
			r.URL.Path = path + ".html"
			fileServer.ServeHTTP(w, r)
			return
		}

		// Tanımlı alt sayfalardan biri değilse index.html sun (SPA / Router fallback)
		r.URL.Path = "/index.html"
		fileServer.ServeHTTP(w, r)
	})

	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.corsMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// One-time background IoC index backfill for existing articles
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if count, err := db.BackfillIoCs(ctx, ioc.Extract); err == nil && count > 0 {
			slog.Info("IoC index backfill completed", slog.Int("extracted_iocs", count))
		}
	}()

	return s
}

// SetNotifier registers an alert dispatcher.
func (s *Server) SetNotifier(n interface {
	DispatchAlert(ctx context.Context, articles []*model.Article)
}) {
	s.notifier = n
}

// Start begins listening and serving HTTP requests.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg == nil || s.cfg.AdminJWTSecret == "" {
		http.Error(w, `{"error": "Server configuration is not available"}`, http.StatusInternalServerError)
		return false
	}

	if _, ok := s.adminTokenFromRequest(r); !ok {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Admin authorization required",
		})
		return false
	}

	return true
}

func (s *Server) handleGetAdminStatus(w http.ResponseWriter, r *http.Request) {
	_, isAdmin := s.adminTokenFromRequest(r)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"is_admin":       isAdmin,
		"requires_admin": true,
	})
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Username), []byte(s.cfg.AdminUsername)) != 1 ||
		subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.cfg.AdminPassword)) != 1 {
		http.Error(w, `{"error":"Invalid admin credentials"}`, http.StatusUnauthorized)
		return
	}

	token, err := signAdminToken(s.cfg.AdminJWTSecret, req.Username, 8*time.Hour)
	if err != nil {
		http.Error(w, `{"error":"Could not create admin session"}`, http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, adminCookie(r, token, 8*60*60))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true, "is_admin": true})
}

func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, adminCookie(r, "", -1))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetStats(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleGetAnalytics(w http.ResponseWriter, r *http.Request) {
	analytics, err := s.db.GetAnalytics(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(analytics)
}

func (s *Server) handleGetIoCs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	articleID, _ := strconv.ParseInt(q.Get("article_id"), 10, 64)

	filter := model.IoCFilter{
		Type:      q.Get("type"),
		Search:    q.Get("search"),
		ArticleID: articleID,
		Limit:     limit,
		Offset:    offset,
	}

	iocs, total, err := s.db.GetIoCs(r.Context(), filter)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"total": total,
		"count": len(iocs),
		"iocs":  iocs,
	})
}

func (s *Server) handleExportIoCs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	iocType := q.Get("type")
	format := strings.ToLower(q.Get("format"))
	if format == "" {
		format = "txt"
	}

	data, err := s.db.ExportIoCs(r.Context(), iocType, format)
	if err != nil {
		http.Error(w, fmt.Sprintf("Export error: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\"ctifeed-iocs.csv\"")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\"ctifeed-blocklist.txt\"")
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleGetSources(w http.ResponseWriter, r *http.Request) {
	var sources []model.FeedSource
	var err error

	if s.db != nil {
		sources, err = s.db.GetSources(r.Context())
	}
	if err != nil || len(sources) == 0 {
		sources = s.cfg.Sources
	}

	resp := map[string]any{
		"count":   len(sources),
		"sources": sources,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleToggleSource(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}

	var req struct {
		ID int64 `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.ID <= 0 {
		http.Error(w, `{"error": "Invalid source ID"}`, http.StatusBadRequest)
		return
	}

	if s.db == nil {
		http.Error(w, `{"error": "Database not initialized"}`, http.StatusInternalServerError)
		return
	}

	newActive, err := s.db.ToggleSource(r.Context(), req.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":   true,
		"id":        req.ID,
		"is_active": newActive,
	})
}

func (s *Server) handleAddSource(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.db == nil {
		http.Error(w, `{"error": "Database not initialized"}`, http.StatusInternalServerError)
		return
	}

	var req struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Category string `json:"category"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		http.Error(w, `{"error": "URL veya kullanici adi zorunludur"}`, http.StatusBadRequest)
		return
	}

	var finalURL string
	var finalName string
	finalCategory := strings.TrimSpace(req.Category)

	isTwitter := strings.HasPrefix(rawURL, "@") ||
		strings.HasPrefix(rawURL, "twitter://") ||
		strings.HasPrefix(rawURL, "x://") ||
		strings.Contains(rawURL, "x.com/") ||
		strings.Contains(rawURL, "twitter.com/") ||
		strings.Contains(rawURL, "/api/rss/twitter/")

	if isTwitter {
		username := rawURL
		username = strings.TrimPrefix(username, "@")
		username = strings.TrimPrefix(username, "twitter://")
		username = strings.TrimPrefix(username, "x://")
		username = strings.TrimPrefix(username, "https://x.com/")
		username = strings.TrimPrefix(username, "http://x.com/")
		username = strings.TrimPrefix(username, "https://twitter.com/")
		username = strings.TrimPrefix(username, "http://twitter.com/")
		if idx := strings.Index(username, "/api/rss/twitter/"); idx != -1 {
			username = username[idx+len("/api/rss/twitter/"):]
		}
		username = strings.Trim(username, "/")
		if idx := strings.Index(username, "?"); idx != -1 {
			username = username[:idx]
		}
		if username == "" {
			http.Error(w, `{"error": "Gecersiz Twitter kullanici adi"}`, http.StatusBadRequest)
			return
		}
		finalURL = fmt.Sprintf("http://localhost:8080/api/rss/twitter/%s", username)
		if strings.TrimSpace(req.Name) != "" {
			finalName = strings.TrimSpace(req.Name)
		} else {
			finalName = "X: @" + username
		}
		if finalCategory == "" {
			finalCategory = "Twitter Threat Intel"
		}
	} else {
		finalURL = rawURL
		if !strings.HasPrefix(finalURL, "http://") && !strings.HasPrefix(finalURL, "https://") && !strings.HasPrefix(finalURL, "telegram://") {
			finalURL = "https://" + finalURL
		}
		if strings.TrimSpace(req.Name) != "" {
			finalName = strings.TrimSpace(req.Name)
		} else {
			finalName = finalURL
		}
		if finalCategory == "" {
			finalCategory = "Custom RSS"
		}
	}

	newSrc, err := s.db.AddSource(r.Context(), finalName, finalURL, finalCategory)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"source":  newSrc,
	})
}

func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.db == nil {
		http.Error(w, `{"error": "Database not initialized"}`, http.StatusInternalServerError)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		var req struct {
			ID int64 `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.ID > 0 {
			idStr = strconv.FormatInt(req.ID, 10)
		}
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, `{"error": "Invalid source ID"}`, http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteSource(r.Context(), id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"id":      id,
	})
}

func (s *Server) handleGetArticles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 {
		limit = 30
	}

	offset, _ := strconv.Atoi(query.Get("offset"))
	minScore, _ := strconv.Atoi(query.Get("min_score"))

	filter := storage.ArticleFilter{
		Search:    query.Get("search"),
		Tag:       query.Get("tag"),
		Source:    query.Get("source"),
		MinScore:  minScore,
		Limit:     limit,
		Offset:    offset,
		SortBy:    query.Get("sort"),
		TimeRange: query.Get("time_range"),
	}

	articles, totalCount, err := s.db.QueryArticles(r.Context(), filter)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"total":    totalCount,
		"count":    len(articles),
		"limit":    filter.Limit,
		"offset":   filter.Offset,
		"articles": articles,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handlePostScan(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if !s.scanMu.TryLock() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Scan already in progress. Please wait for the current cycle to complete.",
		})
		return
	}
	defer s.scanMu.Unlock()

	startTime := time.Now()
	res := s.collector.CollectAll(r.Context())
	inserted, skipped, err := s.db.SaveArticles(r.Context(), res.Articles)
	if inserted > 0 && s.notifier != nil {
		var newArticles []*model.Article
		for _, a := range res.Articles {
			if a.ID > 0 {
				newArticles = append(newArticles, a)
				if len(newArticles) >= inserted {
					break
				}
			}
		}
		if len(newArticles) > 0 {
			s.notifier.DispatchAlert(context.Background(), newArticles)
		}
	}
	if err != nil {
		slog.Error("Database save error during scan", slog.String("error", err.Error()))
	}

	resp := map[string]any{
		"status":             "success",
		"feeds_success":      res.FeedsSuccess,
		"feeds_failed":       res.FeedsFailed,
		"total_fetched":      res.TotalFetched,
		"valid_last_48h":     len(res.Articles),
		"filtered_old":       res.FilteredOld,
		"new_inserted":       inserted,
		"duplicates_skipped": skipped,
		"duration_ms":        time.Since(startTime).Milliseconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// TriggerScan runs a complete collection cycle and saves new articles to storage.
func (s *Server) TriggerScan(ctx context.Context) (int, int, error) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	res := s.collector.CollectAll(ctx)
	if res.FeedsSuccess == 0 && res.FeedsFailed > 0 {
		return 0, 0, fmt.Errorf("all %d feeds failed to fetch", res.FeedsFailed)
	}

	inserted, skipped, err := s.db.SaveArticles(ctx, res.Articles)
	if inserted > 0 && s.notifier != nil {
		var newArticles []*model.Article
		for _, a := range res.Articles {
			if a.ID > 0 {
				newArticles = append(newArticles, a)
				if len(newArticles) >= inserted {
					break
				}
			}
		}
		if len(newArticles) > 0 {
			s.notifier.DispatchAlert(context.Background(), newArticles)
		}
	}
	return inserted, skipped, err
}

type rssItem struct {
	XMLName     xml.Name `xml:"item"`
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Description string   `xml:"description"`
	PubDate     string   `xml:"pubDate"`
	Guid        string   `xml:"guid"`
}

type rssChannel struct {
	XMLName       xml.Name  `xml:"channel"`
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []rssItem `xml:"item"`
}

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

func (s *Server) handleGetTwitterRSS(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		username = r.URL.Query().Get("user")
	}
	username = strings.TrimSpace(username)
	username = strings.TrimPrefix(username, "@")
	if username == "" {
		http.Error(w, "missing username", http.StatusBadRequest)
		return
	}

	if s.collector == nil {
		http.Error(w, "collector not initialized", http.StatusInternalServerError)
		return
	}

	articles, err := s.collector.FetchTwitterArticles(r.Context(), username)
	if err != nil && len(articles) == 0 {
		http.Error(w, fmt.Sprintf("failed to fetch twitter feed: %v", err), http.StatusBadGateway)
		return
	}

	feed := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:         fmt.Sprintf("X: @%s", username),
			Link:          fmt.Sprintf("https://x.com/%s", username),
			Description:   fmt.Sprintf("Real-time threat intelligence feed from @%s on X", username),
			LastBuildDate: time.Now().UTC().Format(time.RFC1123Z),
		},
	}

	for _, a := range articles {
		feed.Channel.Items = append(feed.Channel.Items, rssItem{
			Title:       a.Title,
			Link:        a.Link,
			Description: a.Summary,
			PubDate:     a.PublishedAt.Format(time.RFC1123Z),
			Guid:        a.Link,
		})
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(feed)
}
