package model

import "time"

// FeedSource, siber tehdit istihbaratı için bir RSS/Atom besleme kaynağını temsil eder.
type FeedSource struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Category string `json:"category,omitempty"`
}

// Article, toplanmış ve analiz edilmiş bir CTI haberini veya güvenlik uyarısını temsil eder.
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

// ScoringResult, hesaplanan puanı, eşleşen etiketleri ve puanlama detay dağılımını içerir.
type ScoringResult struct {
	Score     int               `json:"score"`
	Tags      []string          `json:"tags"`
	Breakdown map[string]int    `json:"breakdown"`
}
