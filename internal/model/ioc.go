package model

import "time"

// Supported IoC types
const (
	IoCTypeIP     = "ip"
	IoCTypeDomain = "domain"
	IoCTypeSHA256 = "sha256"
	IoCTypeMD5    = "md5"
	IoCTypeURL    = "url"
)

// IoC represents an Indicator of Compromise extracted from threat reports.
type IoC struct {
	ID            int64     `json:"id"`
	ArticleID     int64     `json:"article_id"`
	Type          string    `json:"type"`           // "ip", "domain", "sha256", "md5", "url"
	Value         string    `json:"value"`          // e.g. "194.26.29.112"
	ThreatContext string    `json:"threat_context"` // Related article title
	Source        string    `json:"source,omitempty"`
	URL           string    `json:"url,omitempty"`
	FirstSeen     time.Time `json:"first_seen"`
}

// IoCFilter defines criteria for querying stored indicators.
type IoCFilter struct {
	Type      string
	Search    string
	ArticleID int64
	Limit     int
	Offset    int
}
