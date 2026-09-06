package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ctifeed/internal/model"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	tmpDir, err := os.MkdirTemp("", "ctifeed_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := NewDB(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to init db: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestSaveArticleAndDeduplication(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	article := &model.Article{
		Source:      "BleepingComputer",
		Title:       "New Ransomware Campaign Targets ESXi Servers",
		Link:        "https://www.bleepingcomputer.com/news/security/sample-1",
		Summary:     "Attackers deployed ransomware targeting VMware ESXi vulnerabilities.",
		Score:       50,
		Tags:        []string{"ransomware", "vmware"},
		PublishedAt: time.Now().Add(-2 * time.Hour),
	}

	// 1. İlk ekleme başarılı olmalıdır
	inserted, err := db.SaveArticle(ctx, article)
	if err != nil {
		t.Fatalf("unexpected error on first insert: %v", err)
	}
	if !inserted {
		t.Fatalf("expected article to be inserted, got false")
	}
	if article.ID == 0 {
		t.Errorf("expected non-zero article ID after insert")
	}

	// 2. Aynı bağlantının ikinci eklemesi atlanmalıdır (mükerrerlik engelleme)
	articleDuplicate := &model.Article{
		Source:      "BleepingComputer",
		Title:       "New Ransomware Campaign Targets ESXi Servers",
		Link:        "https://www.bleepingcomputer.com/news/security/sample-1",
		Summary:     "Duplicate summary",
		Score:       50,
		Tags:        []string{"ransomware"},
		PublishedAt: time.Now().Add(-1 * time.Hour),
	}

	insertedAgain, err := db.SaveArticle(ctx, articleDuplicate)
	if err != nil {
		t.Fatalf("unexpected error on duplicate insert: %v", err)
	}
	if insertedAgain {
		t.Fatalf("expected duplicate insert to be ignored (inserted = false), got true")
	}

	// Toplam makale sayısının hala 1 olduğunu doğrula
	stats, err := db.GetStats(ctx)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}
	if stats.TotalArticles != 1 {
		t.Fatalf("expected 1 total article, got %d", stats.TotalArticles)
	}
}

func TestSaveArticlesBatchAndQuery(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	articles := []*model.Article{
		{
			Source:      "The Hacker News",
			Title:       "Turkish Telecom Targeted in Major Phishing Wave",
			Link:        "https://thehackernews.com/2024/01/sample-tr.html",
			Summary:     "USOM issued warnings across Turkey.",
			Score:       50,
			Tags:        []string{"TR-Focus"},
			PublishedAt: time.Now().Add(-3 * time.Hour),
		},
		{
			Source:      "SecurityWeek",
			Title:       "Critical Zero-Day in Fortinet FortiOS (CVE-2024-21762)",
			Link:        "https://www.securityweek.com/fortinet-cve-2024-21762",
			Summary:     "Active exploitation reported.",
			Score:       85,
			Tags:        []string{"CVE-2024-21762", "fortinet", "zero-day"},
			PublishedAt: time.Now().Add(-1 * time.Hour),
		},
	}

	inserted, skipped, err := db.SaveArticles(ctx, articles)
	if err != nil {
		t.Fatalf("SaveArticles batch failed: %v", err)
	}
	if inserted != 2 || skipped != 0 {
		t.Fatalf("expected 2 inserted and 0 skipped, got %d inserted, %d skipped", inserted, skipped)
	}

	// 1 mevcut ve 1 yeni makale ile toplu işlemi yeniden çalıştır
	newBatch := []*model.Article{
		articles[0], // mükerrer
		{
			Source:      "Unit 42",
			Title:       "Analysis of Recent APT Campaign",
			Link:        "https://unit42.paloaltonetworks.com/new-apt-report",
			Summary:     "Detailed IoCs and threat actor TTPs.",
			Score:       20,
			Tags:        []string{"apt"},
			PublishedAt: time.Now().Add(-5 * time.Hour),
		},
	}

	inserted, skipped, err = db.SaveArticles(ctx, newBatch)
	if err != nil {
		t.Fatalf("Second SaveArticles batch failed: %v", err)
	}
	if inserted != 1 || skipped != 1 {
		t.Fatalf("expected 1 inserted and 1 skipped, got %d inserted, %d skipped", inserted, skipped)
	}

	// En yüksek puanlı makaleleri sorgula
	top, err := db.GetTopArticles(ctx, 10, 0)
	if err != nil {
		t.Fatalf("GetTopArticles failed: %v", err)
	}
	if len(top) != 3 {
		t.Fatalf("expected 3 articles, got %d", len(top))
	}
	// En yüksek skor ilk sırada olmalıdır (85)
	if top[0].Score != 85 {
		t.Fatalf("expected top score 85, got %d (%s)", top[0].Score, top[0].Title)
	}

	// İstatistikleri kontrol et
	stats, err := db.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalArticles != 3 {
		t.Errorf("expected 3 total articles, got %d", stats.TotalArticles)
	}
	if stats.HighPriorityCount != 2 { // 85 ve 50 skorları >= 50'dir
		t.Errorf("expected 2 high priority articles, got %d", stats.HighPriorityCount)
	}
	if stats.CriticalVulnerabilities != 1 {
		t.Errorf("expected 1 CVE article, got %d", stats.CriticalVulnerabilities)
	}

	// Analitik verilerini test et
	analytics, err := db.GetAnalytics(ctx)
	if err != nil {
		t.Fatalf("GetAnalytics failed: %v", err)
	}
	if analytics == nil {
		t.Fatal("expected non-nil analytics data")
	}
	if len(analytics.SourceShare) == 0 {
		t.Errorf("expected at least 1 source in source share, got 0")
	}
	if len(analytics.TopTags) == 0 {
		t.Errorf("expected at least 1 top tag, got 0")
	}
	if len(analytics.TopVendors) == 0 {
		t.Errorf("expected at least 1 vendor (vmware), got 0")
	}

	// IoC depolama ve sorgulama testleri
	testIoCs := []model.IoC{
		{Type: model.IoCTypeIP, Value: "194.26.29.112"},
		{Type: model.IoCTypeDomain, Value: "evil-campaign.top"},
		{Type: model.IoCTypeSHA256, Value: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
	}

	err = db.SaveIoCs(ctx, 1, "Test Campaign", "BleepingComputer", testIoCs)
	if err != nil {
		t.Fatalf("SaveIoCs failed: %v", err)
	}

	// IoC listeleme testi
	iocs, count, err := db.GetIoCs(ctx, model.IoCFilter{Limit: 10})
	if err != nil {
		t.Fatalf("GetIoCs failed: %v", err)
	}
	if count != 3 || len(iocs) != 3 {
		t.Fatalf("expected 3 iocs, got count=%d, len=%d", count, len(iocs))
	}

	// Tip filtresi testi
	ipIoCs, _, err := db.GetIoCs(ctx, model.IoCFilter{Type: "ip"})
	if err != nil || len(ipIoCs) != 1 {
		t.Fatalf("expected 1 ip ioc, got %d (err: %v)", len(ipIoCs), err)
	}

	// TXT ve CSV Export testi
	txtBytes, err := db.ExportIoCs(ctx, "", "txt")
	if err != nil || len(txtBytes) == 0 {
		t.Fatalf("ExportIoCs txt failed: %v", err)
	}

	csvBytes, err := db.ExportIoCs(ctx, "", "csv")
	if err != nil || len(csvBytes) == 0 {
		t.Fatalf("ExportIoCs csv failed: %v", err)
	}
}
