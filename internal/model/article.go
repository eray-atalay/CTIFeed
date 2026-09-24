// Package model defines core data models for threat intelligence articles and IoCs.
package model

import "time"

// FeedSource represents an RSS/Atom or Telegram feed endpoint.
type FeedSource struct {
	ID             int64      `json:"id,omitempty"`
	Name           string     `json:"name"`
	URL            string     `json:"url"`
	Category       string     `json:"category,omitempty"`
	IsActive       bool       `json:"is_active"`
	LastFetchedAt  *time.Time `json:"last_fetched_at,omitempty"`
	LastStatus     string     `json:"last_status,omitempty"` // "ok", "error", "pending"
	ResponseTimeMs int64      `json:"response_time_ms,omitempty"`
	ArticleCount   int        `json:"article_count,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
}

// Article represents an ingested and analyzed cyber threat intelligence item.
type Article struct {
	ID          int64     `json:"id"`
	Source      string    `json:"source"`
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Summary     string    `json:"summary"`
	Score       int       `json:"score"`
	Tags        []string  `json:"tags"`
	PublishedAt time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	IoCs        []IoC     `json:"iocs,omitempty"`
}

// ScoringResult contains the computed threat score, matching tags, and score breakdown.
type ScoringResult struct {
	Score     int            `json:"score"`
	Tags      []string       `json:"tags"`
	Breakdown map[string]int `json:"breakdown"`
}
