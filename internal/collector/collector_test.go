package collector

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ctifeed/internal/config"
	"ctifeed/internal/model"
)

func TestCollectorWithMockFeed(t *testing.T) {
	recentTime := time.Now().Add(-6 * time.Hour).Format(time.RFC1123Z)
	oldTime := time.Now().Add(-72 * time.Hour).Format(time.RFC1123Z)

	mockRSS := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Mock CTI Feed</title>
    <link>https://example.com</link>
    <description>Mock threat intelligence updates</description>
    <item>
      <title>Turkish Energy Grid Targeted by Zero-Day Campaign (CVE-2024-9999)</title>
      <link>https://example.com/item-recent</link>
      <description><![CDATA[<p>USOM reported ongoing exploits targeting Fortinet devices in Ankara. C2 server identified at 194.26.29.112 and domain evil-c2[.]top.</p>]]></description>
      <pubDate>%s</pubDate>
    </item>
    <item>
      <title>Ancient Incident Report from Last Week</title>
      <link>https://example.com/item-old</link>
      <description><![CDATA[<p>Old news report.</p>]]></description>
      <pubDate>%s</pubDate>
    </item>
  </channel>
</rss>`, recentTime, oldTime)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(mockRSS))
	}))
	defer server.Close()

	cfg := config.NewDefaultConfig()
	cfg.Sources = []model.FeedSource{
		{Name: "Mock Source", URL: server.URL, Category: "Test"},
	}
	cfg.Workers = 1
	cfg.Timeout = 5 * time.Second
	cfg.MaxAgeHours = 48 * time.Hour

	col := New(cfg)
	res := col.CollectAll(context.Background())

	if res.FeedsSuccess != 1 {
		t.Fatalf("expected 1 successful feed, got %d (failed: %d)", res.FeedsSuccess, res.FeedsFailed)
	}

	if len(res.Articles) != 1 {
		t.Fatalf("expected 1 valid article within 48h, got %d", len(res.Articles))
	}

	if res.FilteredOld != 1 {
		t.Errorf("expected 1 old article filtered, got %d", res.FilteredOld)
	}

	article := res.Articles[0]
	if article.Title != "Turkish Energy Grid Targeted by Zero-Day Campaign (CVE-2024-9999)" {
		t.Errorf("unexpected article title: %s", article.Title)
	}

	// Cumulative scoring check:
	// TR-Focus (50) + Fortinet (30) + CVE-2024-9999 (35) + Zero-Day (20) = 135
	if article.Score < 135 {
		t.Errorf("expected score >= 135, got %d (tags: %v)", article.Score, article.Tags)
	}

	expectedTags := []string{"CVE-2024-9999", "TR-Focus", "fortinet", "zero-day"}
	for _, expTag := range expectedTags {
		found := false
		for _, tag := range article.Tags {
			if tag == expTag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected tag %s in article tags %v", expTag, article.Tags)
		}
	}

	// Verify IoC extraction
	if len(article.IoCs) < 2 {
		t.Errorf("expected at least 2 IoCs extracted from feed, got %d", len(article.IoCs))
	}
}
