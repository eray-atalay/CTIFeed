package api

import (
	"context"
	"encoding/json"
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

// Server, HTTP web arayüzü ve REST API uç noktalarını koordine eder.
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

// NewServer, yeni bir API ve Web sunucusu örneği oluşturur.
func NewServer(cfg *config.Config, db *storage.DB, col *collector.Collector, addr string) *Server {
	s := &Server{
		cfg:       cfg,
		db:        db,
		collector: col,
	}

	mux := http.NewServeMux()

	// REST API Yönlendirmeleri
	mux.HandleFunc("GET /api/stats", s.handleGetStats)
	mux.HandleFunc("GET /api/analytics", s.handleGetAnalytics)
	mux.HandleFunc("GET /api/iocs", s.handleGetIoCs)
	mux.HandleFunc("GET /api/iocs/export", s.handleExportIoCs)
	mux.HandleFunc("GET /api/sources", s.handleGetSources)
	mux.HandleFunc("GET /api/articles", s.handleGetArticles)
	mux.HandleFunc("POST /api/scan", s.handlePostScan)

	// Gömülü web.Assets üzerinden Statik Varlık Sunucusu
	staticFS, err := fs.Sub(web.Assets, ".")
	if err != nil {
		slog.Error("Failed to create static sub-FS", slog.String("error", err.Error()))
	}
	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/", fileServer)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.corsMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Mevcut haberler için arka planda tek seferlik IoC indeksi oluştur
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if count, err := db.BackfillIoCs(ctx, ioc.Extract); err == nil && count > 0 {
			slog.Info("Mevcut haberler icin IoC indeksi olusturuldu", slog.Int("extracted_iocs", count))
		}
	}()

	return s
}

// SetNotifier, bot veya alert mekanizmasını sunucuya bağlar.
func (s *Server) SetNotifier(n interface {
	DispatchAlert(ctx context.Context, articles []*model.Article)
}) {
	s.notifier = n
}

// Start, HTTP sunucusunu dinlemeye başlatır.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown, sunucuyu zarif bir şekilde (graceful) durdurur.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleGetStats, tehdit istihbaratı özet sayılarını döndürür.
func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetStats(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

// handleGetAnalytics, analitik ve grafik verilerini döndürür.
func (s *Server) handleGetAnalytics(w http.ResponseWriter, r *http.Request) {
	analytics, err := s.db.GetAnalytics(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(analytics)
}

// handleGetIoCs, filtrelenebilir IoC listesini döndürür.
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

// handleExportIoCs, IoC listesini TXT veya CSV formatında dosya olarak indirir.
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

// handleGetSources, yapılandırılmış CTI besleme kaynakları listesini döndürür.
func (s *Server) handleGetSources(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"count":   len(s.cfg.Sources),
		"sources": s.cfg.Sources,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGetArticles, filtrelenmiş ve sayfalanmış tehdit istihbaratı haberlerini döndürür.
func (s *Server) handleGetArticles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 {
		limit = 30
	}

	offset, _ := strconv.Atoi(query.Get("offset"))
	minScore, _ := strconv.Atoi(query.Get("min_score"))

	filter := storage.ArticleFilter{
		Search:   query.Get("search"),
		Tag:      query.Get("tag"),
		Source:   query.Get("source"),
		MinScore: minScore,
		Limit:    limit,
		Offset:   offset,
		SortBy:   query.Get("sort"),
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

// handlePostScan, istek üzerine anlık besleme toplama döngüsünü tetikler.
func (s *Server) handlePostScan(w http.ResponseWriter, r *http.Request) {
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
	if inserted > 0 {
		// Yeni eklenen haberlerin IoC'lerini çıkar ve kaydet
		for _, a := range res.Articles {
			if a.ID > 0 {
				extracted := ioc.Extract(a.Title + " " + a.Summary)
				if len(extracted) > 0 {
					_ = s.db.SaveIoCs(context.Background(), a.ID, a.Title, a.Source, extracted)
				}
			}
		}

		// Yeni eklenen haber varsa abonelere Telegram'dan ilet
		if s.notifier != nil {
			s.notifier.DispatchAlert(context.Background(), res.Articles[:inserted])
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

// TriggerScan, harici tetikleyiciler için tüm beslemeleri tarar ve sonuçları kaydeder.
func (s *Server) TriggerScan(ctx context.Context) (int, int, error) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	res := s.collector.CollectAll(ctx)
	if res.FeedsSuccess == 0 && res.FeedsFailed > 0 {
		return 0, 0, fmt.Errorf("all %d feeds failed to fetch", res.FeedsFailed)
	}

	inserted, skipped, err := s.db.SaveArticles(ctx, res.Articles)
	if inserted > 0 {
		for _, a := range res.Articles {
			if a.ID > 0 {
				extracted := ioc.Extract(a.Title + " " + a.Summary)
				if len(extracted) > 0 {
					_ = s.db.SaveIoCs(context.Background(), a.ID, a.Title, a.Source, extracted)
				}
			}
		}
		if s.notifier != nil {
			s.notifier.DispatchAlert(context.Background(), res.Articles[:inserted])
		}
	}
	return inserted, skipped, err
}
