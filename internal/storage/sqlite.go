package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"ctifeed/internal/model"
)

// DB, SQLite veritabanı işlemlerini yönetir.
type DB struct {
	conn *sql.DB
}

// NewDB, belirtilen dosya yolunda SQLite veritabanını açar veya oluşturur,
// pragma ayarlarını yapılandırır ve gerekli tablo ve indekslerin varlığını doğrular.
func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Eşzamanlılık ve güvenilirlik için bağlantı havuzunu ve pragma kurallarını ayarla
	conn.SetMaxOpenConns(1) // SQLite tek yazıcı bağlantısıyla en kararlı şekilde çalışır
	conn.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	return db, nil
}

// Close, alt katmandaki veritabanı bağlantısını kapatır.
func (d *DB) Close() error {
	return d.conn.Close()
}

// migrate, veritabanı şemasını ve indekslerini başlatır.
func (d *DB) migrate(ctx context.Context) error {
	queries := []string{
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA busy_timeout = 5000;`,
		`PRAGMA synchronous = NORMAL;`,
		`CREATE TABLE IF NOT EXISTS articles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source TEXT NOT NULL,
			title TEXT NOT NULL,
			link TEXT NOT NULL UNIQUE,
			summary TEXT,
			score INTEGER NOT NULL,
			tags TEXT NOT NULL,
			published_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_articles_score ON articles(score DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_articles_published ON articles(published_at DESC);`,

		`CREATE TABLE IF NOT EXISTS user_subscriptions (
			chat_id INTEGER NOT NULL,
			tag TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			PRIMARY KEY (chat_id, tag)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_subscriptions_tag ON user_subscriptions(tag);`,
	}

	for _, q := range queries {
		if _, err := d.conn.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("exec query %q failed: %w", q, err)
		}
	}

	return nil
}

// SaveArticle, haber kaydını henüz mevcut değilse veritabanına ekler.
func (d *DB) SaveArticle(ctx context.Context, article *model.Article) (bool, error) {
	tagsJSON, err := json.Marshal(article.Tags)
	if err != nil {
		return false, fmt.Errorf("failed to marshal tags: %w", err)
	}

	if article.CreatedAt.IsZero() {
		article.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT OR IGNORE INTO articles (source, title, link, summary, score, tags, published_at, created_at)
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
		article.PublishedAt.UTC().Format(time.RFC3339),
		article.CreatedAt.UTC().Format(time.RFC3339),
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
		return true, nil
	}

	return false, nil
}

// SaveArticles, bir dizi haberi tek bir transaction içinde kaydeder.
func (d *DB) SaveArticles(ctx context.Context, articles []*model.Article) (int, int, error) {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO articles (source, title, link, summary, score, tags, published_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

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
			a.PublishedAt.UTC().Format(time.RFC3339),
			a.CreatedAt.UTC().Format(time.RFC3339),
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
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return inserted, skipped, nil
}

// GetTopArticles, isteğe bağlı minimum puan ve limit kısıtlamalarıyla en yüksek puanlı haberleri sorgular.
func (d *DB) GetTopArticles(ctx context.Context, limit int, minScore int) ([]*model.Article, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, source, title, link, summary, score, tags, published_at, created_at
		FROM articles
		WHERE score >= ?
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
		var pubStr, createdStr string

		if err := rows.Scan(
			&a.ID,
			&a.Source,
			&a.Title,
			&a.Link,
			&a.Summary,
			&a.Score,
			&tagsJSON,
			&pubStr,
			&createdStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan article row: %w", err)
		}

		if err := json.Unmarshal([]byte(tagsJSON), &a.Tags); err != nil {
			a.Tags = []string{}
		}

		if t, err := time.Parse(time.RFC3339, pubStr); err == nil {
			a.PublishedAt = t
		}
		if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
			a.CreatedAt = t
		}

		articles = append(articles, &a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return articles, nil
}

// GetArticlesByTag, geriye dönük uyumluluk için son 48 saatteki haberleri getirir.
func (d *DB) GetArticlesByTag(ctx context.Context, tag string, limit int) ([]*model.Article, error) {
	return d.GetArticlesByTagAndTime(ctx, tag, time.Now().Add(-48*time.Hour), limit)
}

// GetArticlesByTagAndTime, belirli bir kategori ve zaman aralığına göre haberleri getirir (1 gün, 1 hafta, 1 ay).
func (d *DB) GetArticlesByTagAndTime(ctx context.Context, tag string, since time.Time, limit int) ([]*model.Article, error) {
	if limit <= 0 {
		limit = 5
	}

	var query string
	var args []any

	cleanTag := strings.ToLower(strings.TrimSpace(tag))
	sinceStr := since.UTC().Format(time.RFC3339)

	switch cleanTag {
	case "critical":
		query = `
			SELECT id, source, title, link, summary, score, tags, published_at, created_at
			FROM articles 
			WHERE score >= 50 AND published_at >= ?
			ORDER BY score DESC, published_at DESC 
			LIMIT ?;
		`
		args = []any{sinceStr, limit}
	case "tr-focus":
		query = `
			SELECT id, source, title, link, summary, score, tags, published_at, created_at
			FROM articles 
			WHERE (LOWER(tags) LIKE '%tr-focus%' OR LOWER(source) LIKE '%usom%') AND published_at >= ?
			ORDER BY score DESC, published_at DESC 
			LIMIT ?;
		`
		args = []any{sinceStr, limit}
	case "cve":
		query = `
			SELECT id, source, title, link, summary, score, tags, published_at, created_at
			FROM articles 
			WHERE (LOWER(tags) LIKE '%cve-%' OR LOWER(title) LIKE '%cve-%') AND published_at >= ?
			ORDER BY score DESC, published_at DESC 
			LIMIT ?;
		`
		args = []any{sinceStr, limit}
	default:
		query = `
			SELECT id, source, title, link, summary, score, tags, published_at, created_at
			FROM articles 
			WHERE (LOWER(tags) LIKE ? OR LOWER(title) LIKE ?) AND published_at >= ?
			ORDER BY score DESC, published_at DESC 
			LIMIT ?;
		`
		pattern := "%" + cleanTag + "%"
		args = []any{pattern, pattern, sinceStr, limit}
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
		var pubStr, createdStr string

		if err := rows.Scan(
			&a.ID,
			&a.Source,
			&a.Title,
			&a.Link,
			&a.Summary,
			&a.Score,
			&tagsJSON,
			&pubStr,
			&createdStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan article row: %w", err)
		}

		if err := json.Unmarshal([]byte(tagsJSON), &a.Tags); err != nil {
			a.Tags = []string{}
		}

		if t, err := time.Parse(time.RFC3339, pubStr); err == nil {
			a.PublishedAt = t
		}
		if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
			a.CreatedAt = t
		}

		articles = append(articles, &a)
	}

	return articles, nil
}

// ArticleFilter, haber arama ve filtreleme parametrelerini tanımlar.
type ArticleFilter struct {
	Search   string
	Tag      string
	Source   string
	MinScore int
	Limit    int
	Offset   int
	SortBy   string
}

// QueryArticles, verilen kriterlere göre haberleri filtreler ve eşleşen listeyle toplam kayıt sayısını döner.
func (d *DB) QueryArticles(ctx context.Context, filter ArticleFilter) ([]*model.Article, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	whereClauses := []string{"1=1"}
	var args []any

	if filter.MinScore > 0 {
		whereClauses = append(whereClauses, "score >= ?")
		args = append(args, filter.MinScore)
	}

	if filter.Source != "" {
		whereClauses = append(whereClauses, "source = ?")
		args = append(args, filter.Source)
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
		var pubStr, createdStr string

		if err := rows.Scan(
			&a.ID,
			&a.Source,
			&a.Title,
			&a.Link,
			&a.Summary,
			&a.Score,
			&tagsJSON,
			&pubStr,
			&createdStr,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan article row: %w", err)
		}

		if err := json.Unmarshal([]byte(tagsJSON), &a.Tags); err != nil {
			a.Tags = []string{}
		}

		if t, err := time.Parse(time.RFC3339, pubStr); err == nil {
			a.PublishedAt = t
		}
		if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
			a.CreatedAt = t
		}

		articles = append(articles, &a)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	return articles, totalCount, nil
}

// Stats, toplanan haberlerin istatistik özetini barındırır.
type Stats struct {
	TotalArticles           int `json:"total_articles"`
	HighPriorityCount       int `json:"high_priority_count"`
	CriticalVulnerabilities int `json:"critical_vulnerabilities"`
	TRFocusCount            int `json:"tr_focus_count"`
}

// GetStats, toplanan haberler hakkında istatistiksel özetleri döndürür.
func (d *DB) GetStats(ctx context.Context) (Stats, error) {
	var s Stats
	row := d.conn.QueryRowContext(ctx, `
		SELECT 
			COUNT(*),
			COALESCE(SUM(CASE WHEN score >= 50 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN tags LIKE '%CVE-%' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN tags LIKE '%TR-Focus%' THEN 1 ELSE 0 END), 0)
		FROM articles;
	`)

	if err := row.Scan(&s.TotalArticles, &s.HighPriorityCount, &s.CriticalVulnerabilities, &s.TRFocusCount); err != nil {
		return s, fmt.Errorf("failed to get stats: %w", err)
	}

	return s, nil
}

// ToggleSubscription, kullanıcının seçtiği kategoriyi tam eşleşmeyle açar veya kapatır.
func (d *DB) ToggleSubscription(ctx context.Context, chatID int64, tag string) (bool, error) {
	tag = strings.TrimSpace(tag)

	var exists int
	err := d.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_subscriptions WHERE chat_id = ? AND LOWER(tag) = LOWER(?)", chatID, tag).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("abonelik kontrol hatasi: %w", err)
	}

	if exists > 0 {
		_, err := d.conn.ExecContext(ctx, "DELETE FROM user_subscriptions WHERE chat_id = ? AND LOWER(tag) = LOWER(?)", chatID, tag)
		return false, err
	}

	_, err = d.conn.ExecContext(ctx,
		"INSERT INTO user_subscriptions (chat_id, tag, created_at) VALUES (?, ?, ?)",
		chatID, strings.ToLower(tag), time.Now().UTC().Format(time.RFC3339),
	)
	return true, err
}

// GetUserSubscriptions, kullanıcının aktif aboneliklerini listeler.
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

// GetAllSubscribers, tüm kullanıcıların izlediği etiketleri bir harita olarak döner.
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

// GetSubscribersForTags, gelen haberin etiketlerine abone olan kişilerin chat_id'lerini döner.
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
