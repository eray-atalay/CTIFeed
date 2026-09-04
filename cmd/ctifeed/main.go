package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ctifeed/internal/api"
	"ctifeed/internal/collector"
	"ctifeed/internal/config"
	"ctifeed/internal/storage"
)

const banner = `
========================================================================
   ____ _____ ___   _____             _   ____        _     
  / ___|_   _|_ _| |  ___|__  ___  __| | | __ )  ___ | |_   
 | |     | |  | |  | |_ / _ \/ _ \/ _` + "`" + ` | |  _ \ / _ \| __|  
 | |___  | |  | |  |  _|  __/  __/ (_| | | |_) | (_) | |_   
  \____| |_| |___| |_|  \___|\___|\__,_| |____/ \___/ \__|  
  Cyber Threat Intelligence Feed Collector & ThreatRadar Web UI
========================================================================
`

func main() {
	cfg := config.NewDefaultConfig()

	port := flag.String("port", "8080", "Port for Web Dashboard server (e.g. 8080)")
	cliMode := flag.Bool("cli", false, "Run in one-off CLI terminal output mode instead of Web UI")
	flag.BoolVar(&cfg.DaemonMode, "daemon", false, "Run periodically in daemon mode (CLI mode only)")
	flag.DurationVar(&cfg.Interval, "interval", 15*time.Minute, "Polling interval for periodic feed scanning (e.g. 15m, 1h)")
	flag.IntVar(&cfg.Workers, "workers", 5, "Number of concurrent workers for feed fetching")
	flag.DurationVar(&cfg.Timeout, "timeout", 10*time.Second, "Timeout per feed fetch request")
	flag.StringVar(&cfg.DBPath, "db", "ctifeed.db", "Path to SQLite database file")
	flag.IntVar(&cfg.TopArticles, "top", 10, "Number of top-priority articles to display in CLI report")
	flag.IntVar(&cfg.MinScore, "min-score", 0, "Minimum score filter for CLI report display")
	verbose := flag.Bool("verbose", false, "Enable verbose debug logs")
	flag.Parse()

	// Konteyner/bulut ortamı için ortam değişkenlerini oku
	if envPort := os.Getenv("PORT"); envPort != "" && *port == "8080" {
		*port = envPort
	}
	if envDB := os.Getenv("DB_PATH"); envDB != "" && cfg.DBPath == "ctifeed.db" {
		cfg.DBPath = envDB
	}
	if envInterval := os.Getenv("INTERVAL"); envInterval != "" {
		if d, err := time.ParseDuration(envInterval); err == nil {
			cfg.Interval = d
		}
	}
	if envWorkers := os.Getenv("WORKERS"); envWorkers != "" {
		if w, err := strconv.Atoi(envWorkers); err == nil && w > 0 {
			cfg.Workers = w
		}
	}
	if envTimeout := os.Getenv("TIMEOUT"); envTimeout != "" {
		if t, err := time.ParseDuration(envTimeout); err == nil {
			cfg.Timeout = t
		}
	}

	// Yapılandırılmış günlükleyiciyi (slog) ayarla
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	fmt.Print(banner)

	// SQLite veritabanı depolamasını başlat
	db, err := storage.NewDB(cfg.DBPath)
	if err != nil {
		slog.Error("Failed to initialize database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		_ = db.Close()
		slog.Info("Database connection closed cleanly")
	}()

	col := collector.New(cfg)

	// Zarif kapatma (graceful shutdown) dinleyicisini yapılandır
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Eski CLI modu talep edilmişse:
	if *cliMode {
		slog.Info("Running in CLI mode", slog.Int("sources", len(cfg.Sources)), slog.Int("workers", cfg.Workers))
		go func() {
			sig := <-sigChan
			slog.Warn("Received shutdown signal", slog.String("signal", sig.String()))
			cancel()
		}()

		runCollectionCycle(ctx, col, db, cfg)

		if !cfg.DaemonMode {
			slog.Info("Single run completed successfully. Exiting.")
			return
		}

		slog.Info("CLI daemon mode active. Next scan scheduled.", slog.Duration("interval", cfg.Interval))
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("CTIFeed daemon shutting down gracefully...")
				return
			case <-ticker.C:
				slog.Info("Scheduled ticker triggered. Starting feed collection cycle...")
				runCollectionCycle(ctx, col, db, cfg)
			}
		}
	}

	// Varsayılan: Modern Web Paneli Sunucusunu çalıştır
	addr := ":" + *port
	server := api.NewServer(cfg, db, col, addr)

	// Veritabanında makale olup olmadığını kontrol et; 0 ise arka planda ilk taramayı başlat
	stats, err := db.GetStats(ctx)
	if err == nil && stats.TotalArticles == 0 {
		slog.Info("Database is empty. Starting initial feed collection cycle in background...")
		go func() {
			_, _, _ = server.TriggerScan(context.Background())
		}()
	}

	// Aralık yapılandırılmışsa arka plan zamanlanmış taramaları başlat
	if cfg.Interval > 0 {
		go func() {
			ticker := time.NewTicker(cfg.Interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					slog.Info("Background scheduler triggered scan cycle...")
					_, _, _ = server.TriggerScan(context.Background())
				}
			}
		}()
	}

	// Sunucu goroutine'i
	serverErr := make(chan error, 1)
	go func() {
		dashboardURL := fmt.Sprintf("http://localhost:%s", *port)
		slog.Info(">>> CTIFeed ThreatRadar Web Dashboard is LIVE!",
			slog.String("url", dashboardURL),
			slog.String("port", *port),
			slog.Int("sources", len(cfg.Sources)),
			slog.Duration("auto_scan_interval", cfg.Interval),
		)
		fmt.Printf("\n  [+] Web Arayuzu:   \033[1;36m%s\033[0m\n", dashboardURL)
		fmt.Printf("  [+] REST API:      \033[1;36m%s/api/articles\033[0m\n", dashboardURL)
		fmt.Printf("  [*] Durdurmak icin Ctrl+C tuslarina basin.\n\n")

		if err := server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Kapatma sinyalini veya kritik sunucu hatasını bekle
	select {
	case sig := <-sigChan:
		slog.Warn("Received shutdown signal, terminating web server...", slog.String("signal", sig.String()))
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("Server shutdown error", slog.String("error", err.Error()))
		}
	case err := <-serverErr:
		slog.Error("Fatal server error", slog.String("error", err.Error()))
	}
}

func runCollectionCycle(ctx context.Context, col *collector.Collector, db *storage.DB, cfg *config.Config) {
	cycleStart := time.Now()
	slog.Info(">>> Starting CTI feed collection cycle")

	res := col.CollectAll(ctx)
	if ctx.Err() != nil {
		slog.Warn("Collection cycle aborted due to cancellation")
		return
	}

	slog.Info("Feed fetching completed",
		slog.Int("feeds_ok", res.FeedsSuccess),
		slog.Int("feeds_failed", res.FeedsFailed),
		slog.Int("total_articles_fetched", res.TotalFetched),
		slog.Int("valid_last_48h", len(res.Articles)),
		slog.Int("filtered_older_than_48h", res.FilteredOld),
		slog.Duration("duration", res.Duration),
	)

	// Toplanan makaleleri mükerrerlik kontrolüyle SQLite'a kaydet
	inserted, skipped, err := db.SaveArticles(ctx, res.Articles)
	if err != nil {
		slog.Error("Error saving articles to database", slog.String("error", err.Error()))
	} else {
		slog.Info("Database sync completed",
			slog.Int("new_inserted", inserted),
			slog.Int("duplicates_skipped", skipped),
		)
	}

	printReport(ctx, db, cfg, cycleStart)
}

func printReport(ctx context.Context, db *storage.DB, cfg *config.Config, cycleStart time.Time) {
	stats, err := db.GetStats(ctx)
	if err == nil {
		fmt.Printf("\n--- [ DATABASE SUMMARY ] ---\n")
		fmt.Printf("Total Saved Articles:     %d\n", stats.TotalArticles)
		fmt.Printf("High Priority (Score>=50): %d\n", stats.HighPriorityCount)
		fmt.Printf("CVE Vulnerabilities:       %d\n", stats.CriticalVulnerabilities)
		fmt.Printf("Turkey Focus (TR-Focus):   %d\n", stats.TRFocusCount)
		fmt.Printf("Cycle Duration:            %s\n", time.Since(cycleStart).Round(time.Millisecond))
		fmt.Println("----------------------------")
	}

	topArticles, err := db.GetTopArticles(ctx, cfg.TopArticles, cfg.MinScore)
	if err != nil {
		slog.Error("Failed to retrieve top articles", slog.String("error", err.Error()))
		return
	}

	if len(topArticles) == 0 {
		fmt.Println("\nNo articles found matching criteria.")
		return
	}

	fmt.Printf("\n=== [ TOP %d HIGHEST PRIORITY THREAT INTEL ALERTS ] ===\n\n", len(topArticles))
	for i, a := range topArticles {
		tagsStr := "None"
		if len(a.Tags) > 0 {
			tagsStr = strings.Join(a.Tags, ", ")
		}

		scoreBadge := fmt.Sprintf("[%d PTS]", a.Score)
		if a.Score >= 50 {
			scoreBadge = fmt.Sprintf("\033[1;31m[%d PTS - CRITICAL]\033[0m", a.Score)
		} else if a.Score >= 30 {
			scoreBadge = fmt.Sprintf("\033[1;33m[%d PTS - HIGH]\033[0m", a.Score)
		}

		fmt.Printf("%2d. %s %s\n", i+1, scoreBadge, a.Title)
		fmt.Printf("    Source:    %s | Published: %s\n", a.Source, a.PublishedAt.Format("2006-01-02 15:04 MST"))
		fmt.Printf("    Tags:      [%s]\n", tagsStr)
		fmt.Printf("    Link:      %s\n", a.Link)
		if a.Summary != "" {
			summarySnippet := a.Summary
			if len(summarySnippet) > 160 {
				summarySnippet = summarySnippet[:157] + "..."
			}
			fmt.Printf("    Summary:   %s\n", summarySnippet)
		}
		fmt.Println()
	}
	fmt.Println("========================================================")
	fmt.Println()
}
