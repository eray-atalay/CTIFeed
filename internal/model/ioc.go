package model

import "time"

// IoCType sabitleri
const (
	IoCTypeIP     = "ip"
	IoCTypeDomain = "domain"
	IoCTypeSHA256 = "sha256"
	IoCTypeMD5    = "md5"
	IoCTypeURL    = "url"
)

// IoC, tehdit haberlerinden çıkarılan bir Tehdit Göstergesini (Indicator of Compromise) temsil eder.
type IoC struct {
	ID            int64     `json:"id"`
	ArticleID     int64     `json:"article_id"`
	Type          string    `json:"type"`           // "ip", "domain", "sha256", "md5", "url"
	Value         string    `json:"value"`          // örn: "194.26.29.112", "e3b0c44298fc..."
	ThreatContext string    `json:"threat_context"` // İlişkili haber başlığı
	Source        string    `json:"source,omitempty"`
	FirstSeen     time.Time `json:"first_seen"`
}

// IoCFilter, IoC listeleme sorguları için filtreleme kriterlerini barındırır.
type IoCFilter struct {
	Type      string
	Search    string
	ArticleID int64
	Limit     int
	Offset    int
}
