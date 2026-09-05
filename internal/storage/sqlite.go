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
	}

	for _, q := range queries {
		if _, err := d.conn.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("exec query %q failed: %w", q, err)
		}
	}

	return nil
}

// SaveArticle, haber kaydını henüz mevcut değilse veritabanına ekler.
// Yeni eklendiyse (inserted = true, nil), mükerrer ise (inserted = false, nil) döndürür.
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

// SaveArticles, bir dizi haberi tek bir işlem (transaction) içinde kaydeder; eklenen ve atlanan sayıları döner.
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

// ArticleFilter, haber arama ve filtreleme parametrelerini tanımlar.
type ArticleFilter struct {
	Search   string
	Tag      string
	Source   string
	MinScore int
	Limit    int
	Offset   int
	SortBy   string // "score" veya "date"
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

	// 1. Kriterlere uyan toplam kayıt sayısını al
	countQuery := "SELECT COUNT(*) FROM articles WHERE " + whereSQL
	var totalCount int
	if err := d.conn.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("count query failed: %w", err)
	}

	// 2. Sıralama düzenini belirle
	orderBy := "score DESC, published_at DESC"
	if filter.SortBy == "date" {
		orderBy = "published_at DESC, score DESC"
	}

	// 3. Sayfalanmış sonuçları sorgula
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
	HighPriorityCount       int `json:"high_priority_count"` // Puan >= 50 olanlar
	CriticalVulnerabilities int `json:"critical_vulnerabilities"` // CVE içeren etiketler
	TRFocusCount            int `json:"tr_focus_count"` // TR-Focus etiketli olanlar
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
