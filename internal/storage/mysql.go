package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"ctifeed/internal/model"
)

// DB handles MySQL storage operations.
type DB struct {
	conn *sql.DB
}

// NewDB opens or initializes the MySQL database at the given DSN.
func NewDB(dsn string) (*DB, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql connection: %w", err)
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(5 * time.Minute)
	conn.SetConnMaxIdleTime(2 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to ping mysql database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

// TruncateTables removes all records from all tables (used for testing and maintenance).
func (d *DB) TruncateTables(ctx context.Context) error {
	_, _ = d.conn.ExecContext(ctx, "DELETE FROM iocs;")
	_, _ = d.conn.ExecContext(ctx, "DELETE FROM user_subscriptions;")
	_, _ = d.conn.ExecContext(ctx, "DELETE FROM feed_sources;")
	_, err := d.conn.ExecContext(ctx, "DELETE FROM articles;")
	_, _ = d.conn.ExecContext(ctx, "ALTER TABLE articles AUTO_INCREMENT = 1;")
	_, _ = d.conn.ExecContext(ctx, "ALTER TABLE iocs AUTO_INCREMENT = 1;")
	_, _ = d.conn.ExecContext(ctx, "ALTER TABLE feed_sources AUTO_INCREMENT = 1;")
	return err
}

// migrate ensures database schema, indices, and cleanup tasks are executed.
func (d *DB) migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS articles (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			source VARCHAR(255) NOT NULL,
			title VARCHAR(512) NOT NULL,
			link VARCHAR(512) NOT NULL,
			summary TEXT,
			score INT NOT NULL,
			tags TEXT NOT NULL,
			published_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL,
			UNIQUE KEY uq_articles_link (link),
			INDEX idx_articles_score (score DESC),
			INDEX idx_articles_published (published_at DESC)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`CREATE TABLE IF NOT EXISTS user_subscriptions (
			chat_id BIGINT NOT NULL,
			tag VARCHAR(128) NOT NULL,
			created_at DATETIME NOT NULL,
			PRIMARY KEY (chat_id, tag),
			INDEX idx_subscriptions_tag (tag)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`CREATE TABLE IF NOT EXISTS iocs (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			article_id BIGINT NOT NULL,
			type VARCHAR(64) NOT NULL,
			value VARCHAR(512) NOT NULL,
			threat_context TEXT,
			source VARCHAR(255),
			first_seen DATETIME NOT NULL,
			UNIQUE KEY uq_iocs_unique (type, value(255), article_id),
			INDEX idx_iocs_type (type),
			INDEX idx_iocs_value (value(255)),
			INDEX idx_iocs_first_seen (first_seen DESC),
			CONSTRAINT fk_iocs_article FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		`CREATE TABLE IF NOT EXISTS feed_sources (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			url VARCHAR(512) NOT NULL,
			category VARCHAR(128) NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			last_fetched_at DATETIME,
			last_status VARCHAR(64) DEFAULT 'pending',
			response_time_ms BIGINT DEFAULT 0,
			article_count INT DEFAULT 0,
			last_error TEXT,
			created_at DATETIME NOT NULL,
			UNIQUE KEY uq_sources_url (url),
			INDEX idx_sources_active (is_active)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,
	}

	for _, q := range queries {
		if _, err := d.conn.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("exec migration failed: %w (query: %s)", err, q)
		}
	}

	// Normalize future timestamps if published_at is ahead of current time
	_, _ = d.conn.ExecContext(ctx, "UPDATE articles SET published_at = created_at WHERE published_at > DATE_ADD(NOW(), INTERVAL 5 MINUTE);")

	return nil
}

// SeedSources inserts default feeds into feed_sources if the table is currently empty.
func (d *DB) SeedSources(ctx context.Context, defaults []model.FeedSource) error {
	var count int
	if err := d.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM feed_sources").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	stmt, err := d.conn.PrepareContext(ctx, `
		INSERT IGNORE INTO feed_sources (name, url, category, is_active, last_status, created_at)
		VALUES (?, ?, ?, TRUE, 'pending', NOW());
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, s := range defaults {
		_, _ = stmt.ExecContext(ctx, s.Name, s.URL, s.Category)
	}
	return nil
}

// GetSources returns all configured feed sources along with health metrics.
func (d *DB) GetSources(ctx context.Context) ([]model.FeedSource, error) {
	rows, err := d.conn.QueryContext(ctx, `
		SELECT id, name, url, category, is_active, last_fetched_at, last_status, response_time_ms, article_count, COALESCE(last_error, '')
		FROM feed_sources
		ORDER BY id ASC;
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query feed sources: %w", err)
	}
	defer rows.Close()

	var sources []model.FeedSource
	for rows.Next() {
		var s model.FeedSource
		var fetchedVal any
		if err := rows.Scan(
			&s.ID,
			&s.Name,
			&s.URL,
			&s.Category,
			&s.IsActive,
			&fetchedVal,
			&s.LastStatus,
			&s.ResponseTimeMs,
			&s.ArticleCount,
			&s.LastError,
		); err != nil {
			return nil, fmt.Errorf("failed to scan feed source: %w", err)
		}
		if t := parseDBTime(fetchedVal); !t.IsZero() {
			s.LastFetchedAt = &t
		}
		sources = append(sources, s)
	}
	return sources, nil
}

// GetActiveSources returns only active feed sources for scanning.
func (d *DB) GetActiveSources(ctx context.Context) ([]model.FeedSource, error) {
	rows, err := d.conn.QueryContext(ctx, `
		SELECT id, name, url, category, is_active, last_fetched_at, last_status, response_time_ms, article_count, COALESCE(last_error, '')
		FROM feed_sources
		WHERE is_active = TRUE
		ORDER BY id ASC;
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query active feed sources: %w", err)
	}
	defer rows.Close()

	var sources []model.FeedSource
	for rows.Next() {
		var s model.FeedSource
		var fetchedVal any
		if err := rows.Scan(
			&s.ID,
			&s.Name,
			&s.URL,
			&s.Category,
			&s.IsActive,
			&fetchedVal,
			&s.LastStatus,
			&s.ResponseTimeMs,
			&s.ArticleCount,
			&s.LastError,
		); err != nil {
			return nil, fmt.Errorf("failed to scan active feed source: %w", err)
		}
		if t := parseDBTime(fetchedVal); !t.IsZero() {
			s.LastFetchedAt = &t
		}
		sources = append(sources, s)
	}
	return sources, nil
}

// ToggleSource switches a feed source between active and inactive.
func (d *DB) ToggleSource(ctx context.Context, id int64) (bool, error) {
	var currentActive bool
	err := d.conn.QueryRowContext(ctx, "SELECT is_active FROM feed_sources WHERE id = ?", id).Scan(&currentActive)
	if err != nil {
		return false, fmt.Errorf("source not found: %w", err)
	}

	newActive := !currentActive
	_, err = d.conn.ExecContext(ctx, "UPDATE feed_sources SET is_active = ? WHERE id = ?", newActive, id)
	if err != nil {
		return false, fmt.Errorf("failed to toggle source: %w", err)
	}
	return newActive, nil
}

// UpdateSourceHealth updates health status, latency, and statistics for a feed source.
func (d *DB) UpdateSourceHealth(ctx context.Context, url string, status string, latencyMs int64, articleCount int, lastError string) error {
	now := time.Now().UTC()
	_, err := d.conn.ExecContext(ctx, `
		UPDATE feed_sources 
		SET last_fetched_at = ?, last_status = ?, response_time_ms = ?, article_count = ?, last_error = ?
		WHERE url = ? OR url LIKE ?;
	`, now, status, latencyMs, articleCount, lastError, url, url+"%")
	return err
}

// SaveArticle inserts an article if it does not already exist.
func (d *DB) SaveArticle(ctx context.Context, article *model.Article) (bool, error) {
	tagsJSON, err := json.Marshal(article.Tags)
	if err != nil {
		return false, fmt.Errorf("failed to marshal tags: %w", err)
	}

	if article.CreatedAt.IsZero() {
		article.CreatedAt = time.Now().UTC()
	}

	// Normalize future timestamps if published_at is ahead of current time
	if article.PublishedAt.After(time.Now().Add(5 * time.Minute)) {
		article.PublishedAt = article.CreatedAt
	}

	query := `
		INSERT IGNORE INTO articles (source, title, link, summary, score, tags, published_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`

	res, err := d.conn.ExecContext(
		ctx,
		query,
		article.Source,
		article.Title,
		article.Link,
		article.Summary,
		article.Score,
		string(tagsJSON),
		article.PublishedAt.UTC(),
		article.CreatedAt.UTC(),
	)
	if err != nil {
		return false, fmt.Errorf("failed to execute insert article: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to read rows affected: %w", err)
	}

	if rowsAffected > 0 {
		if id, err := res.LastInsertId(); err == nil {
			article.ID = id
		}
		if len(article.IoCs) > 0 {
			_ = d.SaveIoCs(ctx, article.ID, article.Title, article.Source, article.IoCs)
		}
		return true, nil
	}

	return false, nil
}

// SaveArticles writes a slice of articles and their extracted IoCs in a single transaction.
func (d *DB) SaveArticles(ctx context.Context, articles []*model.Article) (int, int, error) {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT IGNORE INTO articles (source, title, link, summary, score, tags, published_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	iocStmt, err := tx.PrepareContext(ctx, `
		INSERT IGNORE INTO iocs (article_id, type, value, threat_context, source, first_seen)
		VALUES (?, ?, ?, ?, ?, ?);
	`)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to prepare ioc statement: %w", err)
	}
	defer iocStmt.Close()

	now := time.Now().UTC()
	inserted := 0
	skipped := 0

	for _, a := range articles {
		tagsJSON, err := json.Marshal(a.Tags)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to marshal tags: %w", err)
		}

		if a.CreatedAt.IsZero() {
			a.CreatedAt = now
		}

		res, err := stmt.ExecContext(
			ctx,
			a.Source,
			a.Title,
			a.Link,
			a.Summary,
			a.Score,
			string(tagsJSON),
			a.PublishedAt.UTC(),
			a.CreatedAt.UTC(),
		)
		if err != nil {
			return inserted, skipped, fmt.Errorf("failed to insert article (%s): %w", a.Link, err)
		}

		rows, err := res.RowsAffected()
		if err == nil && rows > 0 {
			inserted++
			if id, err := res.LastInsertId(); err == nil {
				a.ID = id
			}
		} else {
			skipped++
			if a.ID <= 0 {
				_ = tx.QueryRowContext(ctx, "SELECT id FROM articles WHERE link = ?", a.Link).Scan(&a.ID)
			}
		}

		if a.ID > 0 && len(a.IoCs) > 0 {
			for _, item := range a.IoCs {
				val := strings.TrimSpace(item.Value)
				if val == "" {
					continue
				}
				_, _ = iocStmt.ExecContext(ctx, a.ID, item.Type, val, a.Title, a.Source, now)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return inserted, skipped, nil
}

// GetTopArticles returns the highest scoring articles.
func (d *DB) GetTopArticles(ctx context.Context, limit int, minScore int) ([]*model.Article, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, source, title, link, summary, score, tags, published_at, created_at
		FROM articles
		WHERE score >= ? AND source NOT LIKE 'Telegram:%'
		ORDER BY score DESC, published_at DESC
		LIMIT ?;
	`

	rows, err := d.conn.QueryContext(ctx, query, minScore, limit)
	if err != nil {
		return nil, fmt.Errorf("query top articles failed: %w", err)
	}
	defer rows.Close()

	var articles []*model.Article
	for rows.Next() {
		var a model.Article
		var tagsJSON string
		var pubVal, createdVal any

		if err := rows.Scan(
			&a.ID,
			&a.Source,
			&a.Title,
			&a.Link,
			&a.Summary,
			&a.Score,
			&tagsJSON,
			&pubVal,
			&createdVal,
		); err != nil {
			return nil, fmt.Errorf("failed to scan article row: %w", err)
		}

		if err := json.Unmarshal([]byte(tagsJSON), &a.Tags); err != nil {
			a.Tags = []string{}
		}

		a.PublishedAt = parseDBTime(pubVal)
		a.CreatedAt = parseDBTime(createdVal)

		articles = append(articles, &a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return articles, nil
}

// GetArticlesByTag returns articles matching a tag within the last 48 hours.
func (d *DB) GetArticlesByTag(ctx context.Context, tag string, limit int) ([]*model.Article, error) {
	return d.GetArticlesByTagAndTime(ctx, tag, time.Now().Add(-48*time.Hour), limit)
}

// GetArticlesByTagAndTime returns articles filtered by tag and minimum publication time.
func (d *DB) GetArticlesByTagAndTime(ctx context.Context, tag string, since time.Time, limit int) ([]*model.Article, error) {
	if limit <= 0 {
		limit = 5
	}

	var query string
	var args []any

	cleanTag := strings.ToLower(strings.TrimSpace(tag))

	switch cleanTag {
	case "critical":
		query = `
			SELECT id, source, title, link, summary, score, tags, published_at, created_at
			FROM articles 
			WHERE score >= 50 AND published_at >= ?
			ORDER BY score DESC, published_at DESC 
			LIMIT ?;
		`
		args = []any{since.UTC(), limit}
	case "tr-focus":
		query = `
			SELECT id, source, title, link, summary, score, tags, published_at, created_at
			FROM articles 
			WHERE (LOWER(tags) LIKE '%tr-focus%' OR LOWER(source) LIKE '%usom%') AND published_at >= ?
			ORDER BY score DESC, published_at DESC 
			LIMIT ?;
		`
		args = []any{since.UTC(), limit}
	case "cve":
		query = `
			SELECT id, source, title, link, summary, score, tags, published_at, created_at
			FROM articles 
			WHERE (LOWER(tags) LIKE '%cve-%' OR LOWER(title) LIKE '%cve-%') AND published_at >= ?
			ORDER BY score DESC, published_at DESC 
			LIMIT ?;
		`
		args = []any{since.UTC(), limit}
	default:
		query = `
			SELECT id, source, title, link, summary, score, tags, published_at, created_at
			FROM articles 
			WHERE (LOWER(tags) LIKE ? OR LOWER(title) LIKE ?) AND published_at >= ?
			ORDER BY score DESC, published_at DESC 
			LIMIT ?;
		`
		pattern := "%" + cleanTag + "%"
		args = []any{pattern, pattern, since.UTC(), limit}
	}

	rows, err := d.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query articles failed: %w", err)
	}
	defer rows.Close()

	var articles []*model.Article
	for rows.Next() {
		var a model.Article
		var tagsJSON string
		var pubVal, createdVal any

		if err := rows.Scan(
			&a.ID,
			&a.Source,
			&a.Title,
			&a.Link,
			&a.Summary,
			&a.Score,
			&tagsJSON,
			&pubVal,
			&createdVal,
		); err != nil {
			return nil, fmt.Errorf("failed to scan article row: %w", err)
		}

		if err := json.Unmarshal([]byte(tagsJSON), &a.Tags); err != nil {
			a.Tags = []string{}
		}

		a.PublishedAt = parseDBTime(pubVal)
		a.CreatedAt = parseDBTime(createdVal)

		articles = append(articles, &a)
	}

	return articles, nil
}

// ArticleFilter defines criteria for querying stored articles.
type ArticleFilter struct {
	Search    string
	Tag       string
	Source    string
	MinScore  int
	Limit     int
	Offset    int
	SortBy    string
	TimeRange string
}

// QueryArticles searches and paginates articles based on the given filter.
func (d *DB) QueryArticles(ctx context.Context, filter ArticleFilter) ([]*model.Article, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 300 {
		filter.Limit = 300
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	whereClauses := []string{"1=1"}
	var args []any

	if filter.TimeRange != "" {
		now := time.Now().UTC()
		switch filter.TimeRange {
		case "today":
			whereClauses = append(whereClauses, "published_at >= ?")
			args = append(args, now.Add(-24*time.Hour))
		case "1w":
			whereClauses = append(whereClauses, "published_at >= ?")
			args = append(args, now.Add(-7*24*time.Hour))
		case "2w":
			whereClauses = append(whereClauses, "published_at >= ?")
			args = append(args, now.Add(-14*24*time.Hour))
		case "1m":
			whereClauses = append(whereClauses, "published_at >= ?")
			args = append(args, now.Add(-30*24*time.Hour))
		}
	}
	if filter.MinScore > 0 {
		whereClauses = append(whereClauses, "score >= ?")
		args = append(args, filter.MinScore)
	}

	if filter.Source != "" {
		whereClauses = append(whereClauses, "(source = ? OR source LIKE ?)")
		args = append(args, filter.Source, "%"+filter.Source+"%")
	}

	if filter.Tag != "" {
		whereClauses = append(whereClauses, "tags LIKE ?")
		args = append(args, "%\""+filter.Tag+"\"%")
	}

	if filter.Search != "" {
		whereClauses = append(whereClauses, "(title LIKE ? OR summary LIKE ?)")
		searchPattern := "%" + filter.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	countQuery := "SELECT COUNT(*) FROM articles WHERE " + whereSQL
	var totalCount int
	if err := d.conn.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count query failed: %w", err)
	}

	orderBy := "score DESC, published_at DESC"
	if filter.SortBy == "date" {
		orderBy = "published_at DESC, score DESC"
	}

	query := fmt.Sprintf(`
		SELECT id, source, title, link, summary, score, tags, published_at, created_at
		FROM articles
		WHERE %s
		ORDER BY %s
		LIMIT ? OFFSET ?;
	`, whereSQL, orderBy)

	queryArgs := append(args, filter.Limit, filter.Offset)
	rows, err := d.conn.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query articles failed: %w", err)
	}
	defer rows.Close()

	var articles []*model.Article
	for rows.Next() {
		var a model.Article
		var tagsJSON string
		var pubVal, createdVal any

		if err := rows.Scan(
			&a.ID,
			&a.Source,
			&a.Title,
			&a.Link,
			&a.Summary,
			&a.Score,
			&tagsJSON,
			&pubVal,
			&createdVal,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan article row: %w", err)
		}

		if err := json.Unmarshal([]byte(tagsJSON), &a.Tags); err != nil {
			a.Tags = []string{}
		}

		a.PublishedAt = parseDBTime(pubVal)
		a.CreatedAt = parseDBTime(createdVal)

		articles = append(articles, &a)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	return articles, totalCount, nil
}

// Stats holds summary metrics of collected intelligence.
type Stats struct {
	TotalArticles           int `json:"total_articles"`
	HighPriorityCount       int `json:"high_priority_count"`
	CriticalVulnerabilities int `json:"critical_vulnerabilities"`
	TRFocusCount            int `json:"tr_focus_count"`
}

// GetStats returns summary counts excluding auxiliary channels.
func (d *DB) GetStats(ctx context.Context) (Stats, error) {
	var s Stats
	row := d.conn.QueryRowContext(ctx, `
		SELECT 
			COUNT(*),
			COALESCE(SUM(CASE WHEN score >= 50 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN tags LIKE '%CVE-%' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN tags LIKE '%TR-Focus%' THEN 1 ELSE 0 END), 0)
		FROM articles
		WHERE source NOT LIKE 'Telegram:%';
	`)

	if err := row.Scan(&s.TotalArticles, &s.HighPriorityCount, &s.CriticalVulnerabilities, &s.TRFocusCount); err != nil {
		return s, fmt.Errorf("failed to get stats: %w", err)
	}

	return s, nil
}

// ToggleSubscription toggles a user's subscription to a specific tag.
func (d *DB) ToggleSubscription(ctx context.Context, chatID int64, tag string) (bool, error) {
	tag = strings.TrimSpace(tag)

	var exists int
	err := d.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_subscriptions WHERE chat_id = ? AND LOWER(tag) = LOWER(?)", chatID, tag).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("subscription check error: %w", err)
	}

	if exists > 0 {
		_, err := d.conn.ExecContext(ctx, "DELETE FROM user_subscriptions WHERE chat_id = ? AND LOWER(tag) = LOWER(?)", chatID, tag)
		return false, err
	}

	_, err = d.conn.ExecContext(ctx,
		"INSERT INTO user_subscriptions (chat_id, tag, created_at) VALUES (?, ?, ?)",
		chatID, strings.ToLower(tag), time.Now().UTC(),
	)
	return true, err
}

// GetUserSubscriptions returns the active subscription tags for a chat ID.
func (d *DB) GetUserSubscriptions(ctx context.Context, chatID int64) ([]string, error) {
	rows, err := d.conn.QueryContext(ctx, "SELECT tag FROM user_subscriptions WHERE chat_id = ?", chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err == nil {
			cleanTag := strings.ToLower(strings.TrimSpace(tag))
			if cleanTag != "" {
				tags = append(tags, cleanTag)
			}
		}
	}
	return tags, nil
}

// GetAllSubscribers returns a mapping of chat IDs to subscribed tags.
func (d *DB) GetAllSubscribers(ctx context.Context) (map[int64][]string, error) {
	rows, err := d.conn.QueryContext(ctx, "SELECT chat_id, tag FROM user_subscriptions")
	if err != nil {
		return nil, fmt.Errorf("failed to query all subscribers: %w", err)
	}
	defer rows.Close()

	subscribers := make(map[int64][]string)
	for rows.Next() {
		var chatID int64
		var tag string
		if err := rows.Scan(&chatID, &tag); err == nil {
			subscribers[chatID] = append(subscribers[chatID], tag)
		}
	}

	return subscribers, nil
}

// GetSubscribersForTags returns chat IDs subscribed to any of the given tags.
func (d *DB) GetSubscribersForTags(ctx context.Context, tags []string) ([]int64, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(tags))
	args := make([]any, len(tags))
	for i, t := range tags {
		placeholders[i] = "?"
		args[i] = strings.ToLower(strings.TrimSpace(t))
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT chat_id 
		FROM user_subscriptions 
		WHERE tag IN (%s);
	`, strings.Join(placeholders, ","))

	rows, err := d.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chatIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			chatIDs = append(chatIDs, id)
		}
	}
	return chatIDs, nil
}

// TagStat tracks occurrence metrics for a tag.
type TagStat struct {
	Tag   string `json:"tag"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// VendorStat tracks vendor occurrences.
type VendorStat struct {
	Vendor string `json:"vendor"`
	Count  int    `json:"count"`
}

// TimelineStat tracks daily threat counts.
type TimelineStat struct {
	Date     string `json:"date"`
	Total    int    `json:"total"`
	Critical int    `json:"critical"`
}

// AnalyticsData contains aggregated metrics for visualizations.
type AnalyticsData struct {
	TopTags      []TagStat      `json:"top_tags"`
	TopVendors   []VendorStat   `json:"top_vendors"`
	Timeline     []TimelineStat `json:"timeline"`
	SourceShare  []TagStat      `json:"source_share"`
	AverageScore float64        `json:"average_score"`
}

// GetAnalytics collects analytics data for charts.
func (d *DB) GetAnalytics(ctx context.Context) (*AnalyticsData, error) {
	data := &AnalyticsData{
		TopTags:     make([]TagStat, 0),
		TopVendors:  make([]VendorStat, 0),
		Timeline:    make([]TimelineStat, 0),
		SourceShare: make([]TagStat, 0),
	}

	// Average score
	_ = d.conn.QueryRowContext(ctx, "SELECT COALESCE(AVG(score), 0) FROM articles WHERE source NOT LIKE 'Telegram:%';").Scan(&data.AverageScore)

	// Source distribution
	srcRows, err := d.conn.QueryContext(ctx, `
		SELECT source, COUNT(*) as cnt 
		FROM articles 
		WHERE source NOT LIKE 'Telegram:%'
		GROUP BY source 
		ORDER BY cnt DESC 
		LIMIT 8;
	`)
	if err == nil {
		defer srcRows.Close()
		for srcRows.Next() {
			var s TagStat
			if err := srcRows.Scan(&s.Tag, &s.Count); err == nil {
				s.Label = s.Tag
				data.SourceShare = append(data.SourceShare, s)
			}
		}
	}

	// 7-day activity timeline using MySQL DATE_FORMAT and parameter
	since7d := time.Now().UTC().Add(-7 * 24 * time.Hour)
	timeRows, err := d.conn.QueryContext(ctx, `
		SELECT 
			DATE_FORMAT(published_at, '%Y-%m-%d') AS day,
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN score >= 50 THEN 1 ELSE 0 END), 0) AS critical
		FROM articles
		WHERE source NOT LIKE 'Telegram:%' AND published_at >= ?
		GROUP BY day
		ORDER BY day ASC;
	`, since7d)
	if err == nil {
		defer timeRows.Close()
		for timeRows.Next() {
			var t TimelineStat
			if err := timeRows.Scan(&t.Date, &t.Total, &t.Critical); err == nil {
				data.Timeline = append(data.Timeline, t)
			}
		}
	}

	// Tehdit Kategorileri ve Hedeflenen Teknolojiler (Telegram Hariç)
	categoryLabels := map[string]string{
		"cve":          "CVE Zafiyetleri",
		"tr-focus":     "TR-Focus (USOM/TR)",
		"zero-day":     "Zero-Day",
		"in-the-wild":  "Aktif İstismar (In-the-Wild)",
		"ransomware":   "Ransomware",
		"rce":          "RCE İstismarı",
		"auth-bypass":  "Yetki Atlama (Auth Bypass)",
		"data-breach":  "Veri Sızıntısı",
		"infostealer":  "Veri Hırsızı (Infostealer)",
		"phishing":     "Oltalama (Phishing)",
		"malware":      "Zararlı Yazılım",
		"supply-chain": "Tedarik Zinciri",
	}

	vendorLabels := map[string]string{
		"microsoft": "Microsoft",
		"fortinet":  "Fortinet",
		"cisco":     "Cisco",
		"vmware":    "VMware",
		"wordpress": "WordPress",
		"citrix":    "Citrix",
		"sonicwall": "SonicWall",
		"atlassian": "Atlassian",
		"veeam":     "Veeam",
		"linux":     "Linux",
		"apache":    "Apache",
		"ivanti":    "Ivanti",
		"apple":     "Apple",
		"google":    "Google",
		"palo-alto": "Palo Alto",
	}

	tagCounts := make(map[string]int)
	vendorCounts := make(map[string]int)

	tagRows, err := d.conn.QueryContext(ctx, "SELECT tags FROM articles WHERE source NOT LIKE 'Telegram:%';")
	if err == nil {
		defer tagRows.Close()
		for tagRows.Next() {
			var rawTags string
			if err := tagRows.Scan(&rawTags); err != nil {
				continue
			}
			var tags []string
			if err := json.Unmarshal([]byte(rawTags), &tags); err != nil {
				continue
			}

			hasCVE := false
			for _, t := range tags {
				tLow := strings.ToLower(t)

				if strings.HasPrefix(tLow, "cve-") {
					hasCVE = true
				}

				if _, ok := categoryLabels[tLow]; ok {
					tagCounts[tLow]++
				}

				if vName, ok := vendorLabels[tLow]; ok {
					vendorCounts[vName]++
				}
			}
			if hasCVE {
				tagCounts["cve"]++
			}
		}
	}

	for k, label := range categoryLabels {
		cnt := tagCounts[k]
		if cnt > 0 {
			data.TopTags = append(data.TopTags, TagStat{Tag: k, Label: label, Count: cnt})
		}
	}

	for _, vName := range vendorLabels {
		cnt := vendorCounts[vName]
		if cnt > 0 {
			data.TopVendors = append(data.TopVendors, VendorStat{Vendor: vName, Count: cnt})
		}
	}

	return data, nil
}

// SaveIoCs stores extracted IoCs associated with an article.
func (d *DB) SaveIoCs(ctx context.Context, articleID int64, threatContext, source string, iocs []model.IoC) error {
	if len(iocs) == 0 {
		return nil
	}

	stmt, err := d.conn.PrepareContext(ctx, `
		INSERT IGNORE INTO iocs (article_id, type, value, threat_context, source, first_seen)
		VALUES (?, ?, ?, ?, ?, ?);
	`)
	if err != nil {
		return fmt.Errorf("prepare ioc insert failed: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, item := range iocs {
		val := strings.TrimSpace(item.Value)
		if val == "" {
			continue
		}
		_, _ = stmt.ExecContext(ctx, articleID, item.Type, val, threatContext, source, now)
	}
	return nil
}

// GetIoCs queries IoCs matching the specified filter.
func (d *DB) GetIoCs(ctx context.Context, filter model.IoCFilter) ([]model.IoC, int, error) {
	var whereClauses []string
	var args []any

	if filter.Type != "" {
		whereClauses = append(whereClauses, "i.type = ?")
		args = append(args, strings.ToLower(strings.TrimSpace(filter.Type)))
	}

	if filter.Search != "" {
		whereClauses = append(whereClauses, "(i.value LIKE ? OR i.threat_context LIKE ?)")
		searchTerm := "%" + strings.TrimSpace(filter.Search) + "%"
		args = append(args, searchTerm, searchTerm)
	}

	if filter.ArticleID > 0 {
		whereClauses = append(whereClauses, "i.article_id = ?")
		args = append(args, filter.ArticleID)
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM iocs i %s;", whereSQL)
	var totalCount int
	if err := d.conn.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count iocs failed: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := fmt.Sprintf(`
		SELECT 
			i.id, 
			i.article_id, 
			i.type, 
			i.value, 
			i.threat_context, 
			COALESCE(i.source, a.source, ''), 
			COALESCE(a.link, ''), 
			i.first_seen
		FROM iocs i
		LEFT JOIN articles a ON i.article_id = a.id
		%s
		ORDER BY i.first_seen DESC
		LIMIT ? OFFSET ?;
	`, whereSQL)

	queryArgs := append(args, limit, filter.Offset)
	rows, err := d.conn.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query iocs failed: %w", err)
	}
	defer rows.Close()

	var iocs []model.IoC
	for rows.Next() {
		var item model.IoC
		var firstSeenVal any
		if err := rows.Scan(&item.ID, &item.ArticleID, &item.Type, &item.Value, &item.ThreatContext, &item.Source, &item.URL, &firstSeenVal); err != nil {
			continue
		}
		item.FirstSeen = parseDBTime(firstSeenVal)
		iocs = append(iocs, item)
	}

	return iocs, totalCount, nil
}

// GetIoCsForArticle returns IoCs associated with a specific article ID.
func (d *DB) GetIoCsForArticle(ctx context.Context, articleID int64) ([]model.IoC, error) {
	iocs, _, err := d.GetIoCs(ctx, model.IoCFilter{ArticleID: articleID, Limit: 100})
	return iocs, err
}

// ExportIoCs formats IoCs as CSV or plain text blocklist.
func (d *DB) ExportIoCs(ctx context.Context, iocType, format string) ([]byte, error) {
	filter := model.IoCFilter{
		Type:  iocType,
		Limit: 2000,
	}
	iocs, _, err := d.GetIoCs(ctx, filter)
	if err != nil {
		return nil, err
	}

	if strings.ToLower(format) == "csv" {
		var sb strings.Builder
		sb.WriteString("Type,Value,ThreatContext,Source,URL,FirstSeen\n")
		for _, item := range iocs {
			cleanContext := strings.ReplaceAll(item.ThreatContext, "\"", "\"\"")
			cleanSource := strings.ReplaceAll(item.Source, "\"", "\"\"")
			cleanURL := strings.ReplaceAll(item.URL, "\"", "\"\"")
			sb.WriteString(fmt.Sprintf("%s,%s,\"%s\",\"%s\",\"%s\",%s\n",
				item.Type,
				item.Value,
				cleanContext,
				cleanSource,
				cleanURL,
				item.FirstSeen.Format(time.RFC3339),
			))
		}
		return []byte(sb.String()), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# CTIFeed Otomatik IoC Blok Listesi - %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("# Toplam Kayit: %d\n\n", len(iocs)))
	for _, item := range iocs {
		sb.WriteString(item.Value + "\n")
	}
	return []byte(sb.String()), nil
}

// BackfillIoCs extracts and indexes IoCs for existing articles in the database.
func (d *DB) BackfillIoCs(ctx context.Context, extractFn func(text string) []model.IoC) (int, error) {
	_, _ = d.conn.ExecContext(ctx, "DELETE FROM iocs WHERE type = 'domain';")

	rows, err := d.conn.QueryContext(ctx, "SELECT id, title, summary, source FROM articles;")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type articleSnippet struct {
		id      int64
		title   string
		summary string
		source  string
	}

	var articles []articleSnippet
	for rows.Next() {
		var a articleSnippet
		if err := rows.Scan(&a.id, &a.title, &a.summary, &a.source); err == nil {
			articles = append(articles, a)
		}
	}

	totalExtracted := 0
	for _, a := range articles {
		combinedText := a.title + " " + a.summary
		extracted := extractFn(combinedText)
		if len(extracted) > 0 {
			if err := d.SaveIoCs(ctx, a.id, a.title, a.source, extracted); err == nil {
				totalExtracted += len(extracted)
			}
		}
	}

	return totalExtracted, nil
}

// GetLatestTelegramPostID returns the highest numeric Telegram post ID stored for a source.
func (d *DB) GetLatestTelegramPostID(ctx context.Context, source string) (int, error) {
	rows, err := d.conn.QueryContext(ctx, `
		SELECT link FROM articles 
		WHERE source = ? OR source LIKE ? 
		ORDER BY id DESC LIMIT 50;
	`, source, source+"%")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	maxID := 0
	for rows.Next() {
		var link string
		if err := rows.Scan(&link); err == nil {
			idx := strings.LastIndex(link, "/")
			if idx != -1 && idx < len(link)-1 {
				var postID int
				if _, err := fmt.Sscanf(link[idx+1:], "%d", &postID); err == nil {
					if postID > maxID {
						maxID = postID
					}
				}
			}
		}
	}
	return maxID, nil
}

// parseDBTime safely parses database date/time values from various possible types.
func parseDBTime(val any) time.Time {
	switch v := val.(type) {
	case time.Time:
		return v
	case []byte:
		return parseTimeString(string(v))
	case string:
		return parseTimeString(v)
	}
	return time.Time{}
}

func parseTimeString(s string) time.Time {
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05.999999",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
