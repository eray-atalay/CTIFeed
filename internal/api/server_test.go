package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"ctifeed/internal/collector"
	"ctifeed/internal/config"
	"ctifeed/internal/model"
	"ctifeed/internal/storage"
)

func setupTestServer(t *testing.T) (*Server, func()) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		dsn = "ctifeed:ctifeed_secret@tcp(127.0.0.1:3306)/ctifeed?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=UTC"
	}

	db, err := storage.NewDB(dsn)
	if err != nil {
		t.Skipf("MySQL not available (%v), skipping API test. (Set TEST_MYSQL_DSN to run)", err)
		return nil, func() {}
	}

	ctx := context.Background()
	_ = db.TruncateTables(ctx)

	// Seed test article
	_, _ = db.SaveArticle(ctx, &model.Article{
		Source:      "SecurityWeek",
		Title:       "Test CVE Article for API (CVE-2024-1111)",
		Link:        "https://example.com/test-article",
		Summary:     "Test vulnerability description.",
		Score:       65,
		Tags:        []string{"CVE-2024-1111", "TR-Focus"},
		PublishedAt: time.Now(),
	})

	cfg := config.NewDefaultConfig()
	col := collector.New(cfg)

	srv := NewServer(cfg, db, col, ":0")

	cleanup := func() {
		_ = db.TruncateTables(ctx)
		_ = db.Close()
	}

	return srv, cleanup
}

func TestStaticAssetsEndpoints(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		path         string
		expectedCode int
		contains     string
	}{
		{"/", http.StatusOK, "CTIFeed"},
		{"/global.css", http.StatusOK, "--bg-base"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		rec := httptest.NewRecorder()

		srv.server.Handler.ServeHTTP(rec, req)

		if rec.Code != tt.expectedCode {
			t.Errorf("path %s: expected status %d, got %d", tt.path, tt.expectedCode, rec.Code)
		}
		body := rec.Body.String()
		if len(body) == 0 {
			t.Errorf("path %s: expected non-empty body", tt.path)
		}
	}
}

func TestAPIEndpoints(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Test /api/stats
	{
		req := httptest.NewRequest("GET", "/api/stats", nil)
		rec := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for /api/stats, got %d", rec.Code)
		}

		var stats storage.Stats
		if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
			t.Fatalf("failed to decode stats json: %v", err)
		}
		if stats.TotalArticles != 1 {
			t.Errorf("expected 1 total article, got %d", stats.TotalArticles)
		}
		if stats.TRFocusCount != 1 {
			t.Errorf("expected 1 TR-Focus article, got %d", stats.TRFocusCount)
		}
	}

	// Test /api/sources
	{
		req := httptest.NewRequest("GET", "/api/sources", nil)
		rec := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for /api/sources, got %d", rec.Code)
		}

		var data struct {
			Count   int                `json:"count"`
			Sources []model.FeedSource `json:"sources"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&data); err != nil {
			t.Fatalf("failed to decode sources json: %v", err)
		}
		if data.Count != len(srv.cfg.Sources) {
			t.Errorf("expected %d sources, got %d", len(srv.cfg.Sources), data.Count)
		}

		// Test /api/sources/toggle if we have sources in DB
		_ = srv.db.SeedSources(context.Background(), srv.cfg.Sources)
		dbSources, _ := srv.db.GetSources(context.Background())
		if len(dbSources) > 0 {
			targetID := dbSources[0].ID
			toggleBody := strings.NewReader(fmt.Sprintf(`{"id": %d}`, targetID))
			reqToggle := httptest.NewRequest("POST", "/api/sources/toggle", toggleBody)
			reqToggle.Header.Set("Content-Type", "application/json")
			recToggle := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(recToggle, reqToggle)

			if recToggle.Code != http.StatusOK {
				t.Fatalf("expected 200 for /api/sources/toggle, got %d", recToggle.Code)
			}

			var toggleRes struct {
				Success  bool  `json:"success"`
				ID       int64 `json:"id"`
				IsActive bool  `json:"is_active"`
			}
			if err := json.NewDecoder(recToggle.Body).Decode(&toggleRes); err != nil {
				t.Fatalf("failed to decode toggle response: %v", err)
			}
			if toggleRes.IsActive != false {
				t.Errorf("expected source to be toggled to false, got %v", toggleRes.IsActive)
			}
		}
	}

	// Test /api/articles with filtering
	{
		req := httptest.NewRequest("GET", "/api/articles?tag=TR-Focus", nil)
		rec := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for /api/articles, got %d", rec.Code)
		}

		var data struct {
			Total    int              `json:"total"`
			Count    int              `json:"count"`
			Articles []*model.Article `json:"articles"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&data); err != nil {
			t.Fatalf("failed to decode articles json: %v", err)
		}
		if data.Total != 1 {
			t.Errorf("expected 1 article matching TR-Focus, got %d", data.Total)
		}
		if len(data.Articles) == 0 || data.Articles[0].Title != "Test CVE Article for API (CVE-2024-1111)" {
			t.Errorf("unexpected article in response: %+v", data.Articles)
		}
	}

	// Test /api/analytics
	{
		req := httptest.NewRequest("GET", "/api/analytics", nil)
		rec := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for /api/analytics, got %d", rec.Code)
		}

		var data storage.AnalyticsData
		if err := json.NewDecoder(rec.Body).Decode(&data); err != nil {
			t.Fatalf("failed to decode analytics json: %v", err)
		}
		if len(data.SourceShare) == 0 {
			t.Errorf("expected source share to have items")
		}
	}

	// Test /api/iocs and /api/iocs/export
	{
		req := httptest.NewRequest("GET", "/api/iocs", nil)
		rec := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for /api/iocs, got %d", rec.Code)
		}

		reqExport := httptest.NewRequest("GET", "/api/iocs/export?format=txt", nil)
		recExport := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(recExport, reqExport)

		if recExport.Code != http.StatusOK {
			t.Fatalf("expected 200 for /api/iocs/export, got %d", recExport.Code)
		}
	}
}
