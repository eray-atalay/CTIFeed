// Package config provides application-wide configuration structures and defaults.
package config

import (
	"fmt"
	"time"

	"ctifeed/internal/model"
)

// Config represents runtime configuration options.
type Config struct {
	Sources       []model.FeedSource
	MySQLDSN      string
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	Workers       int
	Timeout       time.Duration
	Interval      time.Duration
	MaxAgeHours   time.Duration
	DaemonMode    bool
	TopArticles   int
	MinScore      int
	UserAgent     string
	TelegramToken string
}

// GetDSN returns the active MySQL connection string, building from host/port/user/pass if DSN is not directly specified.
func (c *Config) GetDSN() string {
	if c.MySQLDSN != "" {
		return c.MySQLDSN
	}
	host := c.DBHost
	if host == "" {
		host = "127.0.0.1"
	}
	port := c.DBPort
	if port == "" {
		port = "3306"
	}
	user := c.DBUser
	if user == "" {
		user = "ctifeed"
	}
	pass := c.DBPassword
	if pass == "" {
		pass = "ctifeed_secret"
	}
	name := c.DBName
	if name == "" {
		name = "ctifeed"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=UTC", user, pass, host, port, name)
}

// DefaultSources returns the pre-configured CTI threat feed sources.
func DefaultSources() []model.FeedSource {
	return []model.FeedSource{
		{Name: "The Hacker News", URL: "https://feeds.feedburner.com/TheHackersNews", Category: "General Security"},
		{Name: "BleepingComputer", URL: "https://www.bleepingcomputer.com/feed/", Category: "Vulnerability & Ransomware"},
		{Name: "SecurityWeek", URL: "https://www.securityweek.com/feed/", Category: "Enterprise Security"},
		{Name: "The DFIR Report", URL: "https://thedfirreport.com/feed/", Category: "DFIR & Intrusions"},
		{Name: "Unit 42", URL: "https://unit42.paloaltonetworks.com/feed/", Category: "Threat Research"},
		{Name: "Securelist", URL: "https://securelist.com/feed/", Category: "Kaspersky Research"},
		{Name: "Cisco Talos", URL: "https://blog.talosintelligence.com/rss/", Category: "Threat Intelligence"},
		{Name: "The Record", URL: "https://therecord.media/feed", Category: "Cyber News"},
		{Name: "Dark Reading", URL: "https://www.darkreading.com/rss.xml", Category: "Enterprise Defense"},
		{Name: "Microsoft Threat Intelligence", URL: "https://www.microsoft.com/en-us/security/blog/topic/threat-intelligence/feed/", Category: "Cloud & OS Security"},
		{Name: "CyberScoop", URL: "https://cyberscoop.com/feed/", Category: "Government & Policy"},
		{Name: "HackRead Data Breaches", URL: "https://hackread.com/category/data-breaches/feed/", Category: "Data Breaches"},
		{Name: "GBHackers Cyber Attack", URL: "https://gbhackers.com/category/cyber-attack/feed/", Category: "Attacks & Exploits"},
		{Name: "Infosecurity Magazine", URL: "https://www.infosecurity-magazine.com/rss/news/", Category: "InfoSec News"},
		{Name: "Rapid7 Blog", URL: "https://www.rapid7.com/blog/rss/", Category: "Vulnerability Research"},
		{Name: "Krebs on Security", URL: "https://krebsonsecurity.com/category/data-breaches/feed/", Category: "Investigative Security"},
		{Name: "DarkWebInformer", URL: "https://darkwebinformer.com/rss/", Category: "Dark Web & Leaks"},
		{Name: "DailyDarkWeb", URL: "https://dailydarkweb.net/feed/", Category: "Dark Web & Leaks"},
		{Name: "ANY.RUN Blog", URL: "https://any.run/cybersecurity-blog/feed/", Category: "Malware Analysis & IoCs"},
		{Name: "SANS ISC InfoSec", URL: "https://isc.sans.edu/rssfeed_full.xml", Category: "Honeypot & Attack Diaries"},
		{Name: "CrowdStrike Blog", URL: "https://www.crowdstrike.com/en-us/blog/feed", Category: "Threat Research & Adversaries"},
		{Name: "ESET WeLiveSecurity", URL: "https://www.welivesecurity.com/en/rss/feed/", Category: "APT & Malware Research"},
		{Name: "Red Canary Blog", URL: "https://redcanary.com/blog/feed/", Category: "Threat Research & Detection"},
		{Name: "Telegram: cveNotify", URL: "telegram://cveNotify", Category: "Telegram CVE"},
		{Name: "Telegram: breachdetect", URL: "telegram://breachdetect", Category: "Telegram Breach"},
	}
}

// NewDefaultConfig initializes the default configuration settings.
func NewDefaultConfig() *Config {
	return &Config{
		Sources:       DefaultSources(),
		MySQLDSN:      "",
		DBHost:        "127.0.0.1",
		DBPort:        "3306",
		DBUser:        "ctifeed",
		DBPassword:    "ctifeed_secret",
		DBName:        "ctifeed",
		Workers:       5,
		Timeout:       10 * time.Second,
		Interval:      15 * time.Minute,
		MaxAgeHours:   7 * 24 * time.Hour,
		DaemonMode:    false,
		TopArticles:   10,
		MinScore:      0,
		UserAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		TelegramToken: "",
	}
}
