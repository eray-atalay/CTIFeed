package main

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type BreachMessage struct {
	ID      int
	Source  string
	Content string
	Author  string
	Date    string
	RawText string
}

func fetchPost(channel string, postID int) (*BreachMessage, error) {
	url := fmt.Sprintf("https://t.me/%s/%d", channel, postID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	htmlContent := string(body)
	descRegex := regexp.MustCompile(`<meta property="og:description" content="([^"]+)"`)
	m := descRegex.FindStringSubmatch(htmlContent)
	if len(m) < 2 {
		return nil, fmt.Errorf("og:description bulunamadi")
	}

	rawText := html.UnescapeString(m[1])
	rawText = strings.TrimSpace(rawText)

	// Telegram varsayilan kanal aciklamasi donmusse gercek post degildir
	if strings.HasPrefix(rawText, "You can view and join") || len(rawText) < 15 {
		return nil, fmt.Errorf("gecersiz veya silinmis post")
	}

	bm := &BreachMessage{
		ID:      postID,
		RawText: rawText,
	}

	// Content, Source, Author, Detection Date alanlarini yakala
	contentRe := regexp.MustCompile(`(?i)"Content":\s*"([^"]+)"`)
	if c := contentRe.FindStringSubmatch(rawText); len(c) > 1 {
		bm.Content = c[1]
	}

	sourceRe := regexp.MustCompile(`(?i)"Source":\s*"([^"]+)"`)
	if s := sourceRe.FindStringSubmatch(rawText); len(s) > 1 {
		bm.Source = s[1]
	}

	authorRe := regexp.MustCompile(`(?i)"author":\s*"([^"]+)"`)
	if a := authorRe.FindStringSubmatch(rawText); len(a) > 1 {
		bm.Author = a[1]
	}

	dateRe := regexp.MustCompile(`(?i)"Detection Date":\s*"([^"]+)"`)
	if d := dateRe.FindStringSubmatch(rawText); len(d) > 1 {
		bm.Date = d[1]
	}

	return bm, nil
}

func main() {
	channel := "breachdetect"
	latestID := 1281687 // Tespit ettigin guncel post numarasi

	fmt.Printf("[*] %s kanalindan son mesajlar cekiliyor...\n\n", channel)

	count := 0
	for id := latestID; id > latestID-10; id-- {
		msg, err := fetchPost(channel, id)
		if err != nil {
			continue
		}
		count++
		title := msg.Content
		if title == "" {
			title = msg.RawText
		}
		fmt.Printf("--- [Post #%d] ---\n", msg.ID)
		fmt.Printf("Kaynak Hedef : %s\n", msg.Source)
		fmt.Printf("Baslik       : %s\n", title)
		fmt.Printf("Tarih        : %s\n", msg.Date)
		fmt.Printf("Baglanti     : https://t.me/%s/%d\n\n", channel, msg.ID)
	}

	fmt.Printf("[+] Test tamamlandi. Toplam %d gecerli sizinti mesaji cekildi.\n", count)
}