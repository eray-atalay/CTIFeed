package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ctifeed/internal/collector"
	"ctifeed/internal/config"
	"ctifeed/internal/model"
	"ctifeed/internal/storage"
)

func setupTestServer(t *testing.T) (*Server, func()) {
	tmpDir, err := os.MkdirTemp("", "ctifeed_api_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.NewDB(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to init db: %v", err)
	}

	// Örnek bir makale ekle
	ctx := context.Background()
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
		_ = db.Close()
		_ = os.RemoveAll(tmpDir)
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
		{"/style.css", http.StatusOK, "--bg-base"},
		{"/app.js", http.StatusOK, "CTIFeed Tehdit Radari"},
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

	// 1. /api/stats testi
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

	// 2. /api/sources testi
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
		if data.Count != 18 {
			t.Errorf("expected 18 sources, got %d", data.Count)
		}
	}

	// 3. Filtreleme ile /api/articles testi
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

	// 4. /api/analytics endpoint testi
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

	// 5. /api/iocs ve /api/iocs/export testleri
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
