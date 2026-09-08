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

	conn.SetMaxOpenConns(1)
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

		`CREATE TABLE IF NOT EXISTS iocs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			article_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			value TEXT NOT NULL,
			threat_context TEXT,
			source TEXT,
			first_seen DATETIME NOT NULL,
			FOREIGN KEY(article_id) REFERENCES articles(id) ON DELETE CASCADE,
			UNIQUE(type, value, article_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_iocs_type ON iocs(type);`,
		`CREATE INDEX IF NOT EXISTS idx_iocs_value ON iocs(value);`,
		`CREATE INDEX IF NOT EXISTS idx_iocs_first_seen ON iocs(first_seen DESC);`,
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
		if len(article.IoCs) > 0 {
			_ = d.SaveIoCs(ctx, article.ID, article.Title, article.Source, article.IoCs)
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

	iocStmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO iocs (article_id, type, value, threat_context, source, first_seen)
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
			if a.ID <= 0 {
				_ = tx.QueryRowContext(ctx, "SELECT id FROM articles WHERE link = ?", a.Link).Scan(&a.ID)
			}
		}

		if a.ID > 0 && len(a.IoCs) > 0 {
			nowStr := now.Format(time.RFC3339)
			for _, item := range a.IoCs {
				val := strings.TrimSpace(item.Value)
				if val == "" {
					continue
				}
				_, _ = iocStmt.ExecContext(ctx, a.ID, item.Type, val, a.Title, a.Source, nowStr)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return inserted, skipped, nil
}

// GetTopArticles, en yüksek puanlı haberleri sorgular.
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

// GetArticlesByTagAndTime, belirli bir kategori ve zaman aralığına göre haberleri getirir.
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
	Search    string
	Tag       string
	Source    string
	MinScore  int
	Limit     int
	Offset    int
	SortBy    string
	TimeRange string
}

// QueryArticles, verilen kriterlere göre haberleri filtreler.
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
		switch filter.TimeRange {
		case "today":
			whereClauses = append(whereClauses, "published_at >= datetime('now', '-1 day')")
		case "1w":
			whereClauses = append(whereClauses, "published_at >= datetime('now', '-7 days')")
		case "1m":
			whereClauses = append(whereClauses, "published_at >= datetime('now', '-30 days')")
		}
	}
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

// GetStats, Telegram kanallarını hariç tutarak ana istatistikleri döner.
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

// TagStat, etiket ve kategori istatistiğini tutar.
type TagStat struct {
	Tag   string `json:"tag"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// VendorStat, hedeflenen üretici istatistiğini tutar.
type VendorStat struct {
	Vendor string `json:"vendor"`
	Count  int    `json:"count"`
}

// TimelineStat, günlük tehdit aktivite istatistiğini tutar.
type TimelineStat struct {
	Date     string `json:"date"`
	Total    int    `json:"total"`
	Critical int    `json:"critical"`
}

// AnalyticsData, analitik grafikleri ve trend verilerini barındırır.
type AnalyticsData struct {
	TopTags      []TagStat      `json:"top_tags"`
	TopVendors   []VendorStat   `json:"top_vendors"`
	Timeline     []TimelineStat `json:"timeline"`
	SourceShare  []TagStat      `json:"source_share"`
	AverageScore float64        `json:"average_score"`
}

// GetAnalytics, grafikler için Telegram hariç tutulmuş analitik verilerini toplar.
func (d *DB) GetAnalytics(ctx context.Context) (*AnalyticsData, error) {
	data := &AnalyticsData{
		TopTags:     make([]TagStat, 0),
		TopVendors:  make([]VendorStat, 0),
		Timeline:    make([]TimelineStat, 0),
		SourceShare: make([]TagStat, 0),
	}

	// 1. Ortalama Skor (Telegram Hariç)
	_ = d.conn.QueryRowContext(ctx, "SELECT COALESCE(AVG(score), 0) FROM articles WHERE source NOT LIKE 'Telegram:%';").Scan(&data.AverageScore)

	// 2. Kaynak Dağılımı (Top 8, Telegram Hariç)
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

	// 3. Son 7 Günün Aktivite Zaman Çizelgesi (Telegram Hariç)
	timeRows, err := d.conn.QueryContext(ctx, `
		SELECT 
			strftime('%Y-%m-%d', published_at) AS day,
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN score >= 50 THEN 1 ELSE 0 END), 0) AS critical
		FROM articles
		WHERE source NOT LIKE 'Telegram:%' AND published_at >= datetime('now', '-7 days')
		GROUP BY day
		ORDER BY day ASC;
	`)
	if err == nil {
		defer timeRows.Close()
		for timeRows.Next() {
			var t TimelineStat
			if err := timeRows.Scan(&t.Date, &t.Total, &t.Critical); err == nil {
				data.Timeline = append(data.Timeline, t)
			}
		}
	}

	// 4. Tehdit Kategorileri ve Hedeflenen Teknolojiler (Telegram Hariç)
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

// SaveIoCs, bir makaleyle ilişkili tespit edilen IoC'leri kaydeder.
func (d *DB) SaveIoCs(ctx context.Context, articleID int64, threatContext, source string, iocs []model.IoC) error {
	if len(iocs) == 0 {
		return nil
	}

	stmt, err := d.conn.PrepareContext(ctx, `
		INSERT OR IGNORE INTO iocs (article_id, type, value, threat_context, source, first_seen)
		VALUES (?, ?, ?, ?, ?, ?);
	`)
	if err != nil {
		return fmt.Errorf("prepare ioc insert failed: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, item := range iocs {
		val := strings.TrimSpace(item.Value)
		if val == "" {
			continue
		}
		_, _ = stmt.ExecContext(ctx, articleID, item.Type, val, threatContext, source, now)
	}
	return nil
}

// GetIoCs, filtrelenebilir kriterlere göre IoC listesini döndürür.
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
		var firstSeenStr string
		if err := rows.Scan(&item.ID, &item.ArticleID, &item.Type, &item.Value, &item.ThreatContext, &item.Source, &item.URL, &firstSeenStr); err != nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339, firstSeenStr); err == nil {
			item.FirstSeen = t
		}
		iocs = append(iocs, item)
	}

	return iocs, totalCount, nil
}

// GetIoCsForArticle, belirli bir makaleye ait IoC'leri döner.
func (d *DB) GetIoCsForArticle(ctx context.Context, articleID int64) ([]model.IoC, error) {
	iocs, _, err := d.GetIoCs(ctx, model.IoCFilter{ArticleID: articleID, Limit: 100})
	return iocs, err
}

// ExportIoCs, IoC'leri TXT veya CSV formatında döndürür.
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

// BackfillIoCs, veritabanında daha önce kaydedilmiş haberlerden geriye dönük IoC çıkarımı yapar.
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