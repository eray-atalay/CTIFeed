package notifier

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"sync"
	"time"

	"ctifeed/internal/model"
	"ctifeed/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var CategoryCatalog = []struct {
	Key   string
	Label string
}{
	{Key: "tr-focus", Label: "🇹🇷 TR-Focus (USOM/TR)"},
	{Key: "critical", Label: "🚨 Kritik (Skor 50+)"},
	{Key: "cve", Label: "🛡️ CVE Zafiyetleri"},
	{Key: "zero-day", Label: "⚡ Zero-Day"},
	{Key: "ransomware", Label: "💀 Ransomware"},
	{Key: "wordpress", Label: "🧩 WordPress"},
	{Key: "fortinet", Label: "🔥 Fortinet"},
	{Key: "cisco", Label: "🌐 Cisco"},
	{Key: "data-breach", Label: "📁 Veri Sızıntısı"},
}

type TelegramBot struct {
	bot         *tgbotapi.BotAPI
	storage     *storage.DB
	logger      *slog.Logger
	stopCh      chan struct{}
	mu          sync.RWMutex
	sentAlerts  map[string]time.Time
	userPeriods map[int64]string // Kullanıcının seçtiği aktif zaman aralığı (1d, 7d, 30d)
}

func NewTelegramBot(token string, store *storage.DB) (*TelegramBot, error) {
	if token == "" {
		return nil, nil
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("telegram bot olusturulamadi: %w", err)
	}

	return &TelegramBot{
		bot:         bot,
		storage:     store,
		logger:      slog.Default(),
		stopCh:      make(chan struct{}),
		sentAlerts:  make(map[string]time.Time),
		userPeriods: make(map[int64]string),
	}, nil
}

func (tb *TelegramBot) Start() {
	tb.logger.Info("Telegram bot aktif", slog.String("bot_username", tb.bot.Self.UserName))

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := tb.bot.GetUpdatesChan(u)

	go func() {
		for {
			select {
			case <-tb.stopCh:
				return
			case update, ok := <-updates:
				if !ok {
					return
				}
				if update.Message != nil {
					tb.handleMessage(update.Message)
				} else if update.CallbackQuery != nil {
					tb.handleCallback(update.CallbackQuery)
				}
			}
		}
	}()
}

func (tb *TelegramBot) Stop() {
	close(tb.stopCh)
}

func (tb *TelegramBot) handleMessage(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	switch {
	case strings.HasPrefix(text, "/start"):
		tb.sendWelcome(chatID)
	case strings.HasPrefix(text, "/filtre") || strings.HasPrefix(text, "/kategori") || strings.HasPrefix(text, "/abone"):
		tb.sendFilterKeyboard(chatID)
	case strings.HasPrefix(text, "/son"):
		tb.sendLatestCritical(chatID)
	case strings.HasPrefix(text, "/kapsam") || strings.HasPrefix(text, "/durum"):
		tb.sendFilterStatus(chatID)
	default:
		reply := tgbotapi.NewMessage(chatID, "Komut anlaşılamadı.\n\n/filtre - Radar filtrelerini yönet ve tehditleri dök\n/kapsam - Aktif radar kapsamını incele\n/son - En kritik son 5 tehdit")
		tb.bot.Send(reply)
	}
}

func (tb *TelegramBot) sendWelcome(chatID int64) {
	text := `🛡️ <b>CTIFeed Siber Tehdit Radarı Botuna Hoş Geldiniz!</b>

İstihbarat akışını filtreleyebilir, belirli zaman aralıklarındaki bulguları getirebilir ve yeni tehditler için anlık bildirim alabilirsiniz.

<b>Komutlar:</b>
🎯 /filtre - Zaman aralığı & kategorileri seç ve tehditleri getir
📋 /kapsam - Sadece seçtiğin aktif filtreleri gör
🚨 /son - En kritik son 5 tehdit kaydı`

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	tb.bot.Send(msg)
	tb.sendFilterKeyboard(chatID)
}

func (tb *TelegramBot) getSelectedPeriod(chatID int64) string {
	tb.mu.RLock()
	defer tb.mu.RUnlock()
	if p, ok := tb.userPeriods[chatID]; ok {
		return p
	}
	return "1d" // Varsayılan: Son 24 Saat
}

func (tb *TelegramBot) sendFilterKeyboard(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "🎯 <b>Tehdit Radarı Filtreleme Paneli:</b>\n<i>Aşağıdan zaman aralığını ve izlemek istediğiniz kategorileri belirleyin. Ardından en alttaki butonla haberleri dökün:</i>")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tb.buildKeyboard(chatID)
	tb.bot.Send(msg)
}

func (tb *TelegramBot) buildKeyboard(chatID int64) tgbotapi.InlineKeyboardMarkup {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	userSubs, _ := tb.storage.GetUserSubscriptions(ctx, chatID)
	activeMap := make(map[string]bool)
	for _, s := range userSubs {
		clean := strings.ToLower(strings.TrimSpace(s))
		if clean != "" {
			activeMap[clean] = true
		}
	}

	currentPeriod := tb.getSelectedPeriod(chatID)

	// 1. Satır: Zaman Aralığı Butonları
	p1, p2, p3 := "Son 24 Saat", "Son 1 Hafta", "Son 1 Ay"
	if currentPeriod == "1d" {
		p1 = "🎯 24 Saat"
	} else if currentPeriod == "7d" {
		p2 = "🎯 1 Hafta"
	} else if currentPeriod == "30d" {
		p3 = "🎯 1 Ay"
	}

	btnDay := tgbotapi.NewInlineKeyboardButtonData(p1, "period:1d")
	btnWeek := tgbotapi.NewInlineKeyboardButtonData(p2, "period:7d")
	btnMonth := tgbotapi.NewInlineKeyboardButtonData(p3, "period:30d")

	var rows [][]tgbotapi.InlineKeyboardButton
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(btnDay, btnWeek, btnMonth))

	// 2. Kategori Butonları (Tam normalize eşleşme)
	for _, cat := range CategoryCatalog {
		state := "[   ]"
		catKeyClean := strings.ToLower(strings.TrimSpace(cat.Key))
		if activeMap[catKeyClean] {
			state = "[ 🎯 Aktif ]"
		}
		btnText := fmt.Sprintf("%s %s", state, cat.Label)
		btn := tgbotapi.NewInlineKeyboardButtonData(btnText, "toggle:"+catKeyClean)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}

	// 3. Toplu İşlem Butonları
	btnSelectAll := tgbotapi.NewInlineKeyboardButtonData("✅ Tümünü Seç", "cmd:select_all")
	btnClearAll := tgbotapi.NewInlineKeyboardButtonData("❌ Tümünü Temizle", "cmd:clear_all")
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(btnSelectAll, btnClearAll))

	// 4. Haberleri Getir Butonu
	btnFetch := tgbotapi.NewInlineKeyboardButtonData("📥 Seçilenlerle Tehditleri Getir", "cmd:fetch_news")
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(btnFetch))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (tb *TelegramBot) handleCallback(cb *tgbotapi.CallbackQuery) {
	chatID := cb.Message.Chat.ID
	data := cb.Data

	// Zaman Aralığı Değiştirme
	if strings.HasPrefix(data, "period:") {
		period := strings.TrimPrefix(data, "period:")
		tb.mu.Lock()
		tb.userPeriods[chatID] = period
		tb.mu.Unlock()

		tb.bot.Send(tgbotapi.NewCallback(cb.ID, "Zaman aralığı güncellendi."))
		editMarkup := tgbotapi.NewEditMessageReplyMarkup(chatID, cb.Message.MessageID, tb.buildKeyboard(chatID))
		tb.bot.Send(editMarkup)
		return
	}

	// Kategori Aç / Kapa (Sadece butonu günceller)
	if strings.HasPrefix(data, "toggle:") {
		key := strings.ToLower(strings.TrimPrefix(data, "toggle:"))

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		isActive, _ := tb.storage.ToggleSubscription(ctx, chatID, key)
		cancel()

		editMarkup := tgbotapi.NewEditMessageReplyMarkup(chatID, cb.Message.MessageID, tb.buildKeyboard(chatID))
		tb.bot.Send(editMarkup)

		statusText := "Kapsamdan çıkarıldı."
		if isActive {
			statusText = "Kapsama eklendi!"
		}
		tb.bot.Send(tgbotapi.NewCallback(cb.ID, statusText))
		return
	}

	// Tümünü Seç
	if data == "cmd:select_all" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		subs, _ := tb.storage.GetUserSubscriptions(ctx, chatID)
		activeMap := make(map[string]bool)
		for _, s := range subs {
			activeMap[strings.ToLower(strings.TrimSpace(s))] = true
		}
		for _, cat := range CategoryCatalog {
			if !activeMap[cat.Key] {
				_, _ = tb.storage.ToggleSubscription(ctx, chatID, cat.Key)
			}
		}
		cancel()

		tb.bot.Send(tgbotapi.NewCallback(cb.ID, "Tüm kategoriler seçildi!"))
		editMarkup := tgbotapi.NewEditMessageReplyMarkup(chatID, cb.Message.MessageID, tb.buildKeyboard(chatID))
		tb.bot.Send(editMarkup)
		return
	}

	// Tümünü Temizle
	if data == "cmd:clear_all" || data == "cmd:clear_filter" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		subs, _ := tb.storage.GetUserSubscriptions(ctx, chatID)
		for _, s := range subs {
			_, _ = tb.storage.ToggleSubscription(ctx, chatID, s)
		}
		cancel()

		tb.bot.Send(tgbotapi.NewCallback(cb.ID, "Tüm seçimler temizlendi."))
		if data == "cmd:clear_filter" {
			emptyMsg := tgbotapi.NewEditMessageText(chatID, cb.Message.MessageID, "🗑️ <b>İzleme kapsamı tamamen temizlendi.</b>\nYeni kategoriler eklemek için butona dokunabilirsiniz.")
			emptyMsg.ParseMode = "HTML"
			btn := tgbotapi.NewInlineKeyboardButtonData("🎯 Kapsam Belirle", "cmd:open_filter")
			emptyMsg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{{btn}}}
			tb.bot.Send(emptyMsg)
		} else {
			editMarkup := tgbotapi.NewEditMessageReplyMarkup(chatID, cb.Message.MessageID, tb.buildKeyboard(chatID))
			tb.bot.Send(editMarkup)
		}
		return
	}

	// Filtre Menüsünü Aç
	if data == "cmd:open_filter" {
		tb.bot.Send(tgbotapi.NewCallback(cb.ID, ""))
		tb.sendFilterKeyboard(chatID)
		return
	}

	// Seçilenlerle Tehditleri Getir
	if data == "cmd:fetch_news" {
		tb.bot.Send(tgbotapi.NewCallback(cb.ID, "İstihbarat taranıyor..."))
		tb.fetchFilteredArticles(chatID)
		return
	}
}

// fetchFilteredArticles: Seçilen zaman aralığı ve seçili kategorilere göre haberleri tek seferde döker
func (tb *TelegramBot) fetchFilteredArticles(chatID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	subs, _ := tb.storage.GetUserSubscriptions(ctx, chatID)
	period := tb.getSelectedPeriod(chatID)

	var since time.Time
	var periodLabel string
	now := time.Now()

	switch period {
	case "7d":
		since = now.Add(-7 * 24 * time.Hour)
		periodLabel = "Son 1 Hafta"
	case "30d":
		since = now.Add(-30 * 24 * time.Hour)
		periodLabel = "Son 1 Ay"
	default:
		since = now.Add(-24 * time.Hour)
		periodLabel = "Son 24 Saat"
	}

	if len(subs) == 0 {
		infoMsg := tgbotapi.NewMessage(chatID, "⚠️ <i>Henüz hiçbir kategori seçmediniz. Lütfen listeden en az bir kategori seçip tekrar deneyin.</i>")
		infoMsg.ParseMode = "HTML"
		tb.bot.Send(infoMsg)
		return
	}

	// Seçili her kategori için haberleri topla
	seen := make(map[string]bool)
	var finalArticles []*model.Article

	for _, tag := range subs {
		arts, err := tb.storage.GetArticlesByTagAndTime(ctx, tag, since, 3)
		if err == nil {
			for _, a := range arts {
				if !seen[a.Link] {
					seen[a.Link] = true
					finalArticles = append(finalArticles, a)
				}
			}
		}
	}

	if len(finalArticles) == 0 {
		infoMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("ℹ️ Seçtiğiniz kategorilerde <b>(%s)</b> eşleşen tehdit kaydı bulunamadı.", periodLabel))
		infoMsg.ParseMode = "HTML"
		tb.bot.Send(infoMsg)
		return
	}

	headerMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📡 <b>Seçili Kapsam Sonuçları (%s | %d Tehdit):</b>\n──────────────────────────", periodLabel, len(finalArticles)))
	headerMsg.ParseMode = "HTML"
	tb.bot.Send(headerMsg)

	// En fazla 5-6 kart basarak ekranı taşırmayalım
	maxShow := 5
	if len(finalArticles) < maxShow {
		maxShow = len(finalArticles)
	}

	for _, a := range finalArticles[:maxShow] {
		cardText := formatArticleCard(a)
		msg := tgbotapi.NewMessage(chatID, cardText)
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true
		tb.bot.Send(msg)
	}
}

func (tb *TelegramBot) sendLatestCritical(chatID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	articles, err := tb.storage.GetArticlesByTagAndTime(ctx, "critical", time.Now().Add(-30*24*time.Hour), 5)
	if err != nil || len(articles) == 0 {
		msg := tgbotapi.NewMessage(chatID, "Kritik tehdit kaydı bulunamadı.")
		tb.bot.Send(msg)
		return
	}

	headerMsg := tgbotapi.NewMessage(chatID, "🚨 <b>En Yüksek Öncelikli Tehditler (Skor ≥ 50):</b>\n──────────────────────────")
	headerMsg.ParseMode = "HTML"
	tb.bot.Send(headerMsg)

	for _, a := range articles {
		cardText := formatArticleCard(a)
		msg := tgbotapi.NewMessage(chatID, cardText)
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true
		tb.bot.Send(msg)
	}
}

// sendFilterStatus: /kapsam komutu - SADECE seçilen filtreleri listeler
func (tb *TelegramBot) sendFilterStatus(chatID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	subs, _ := tb.storage.GetUserSubscriptions(ctx, chatID)
	if len(subs) == 0 {
		msg := tgbotapi.NewMessage(chatID, "📡 <b>Radar İzleme Kapsamı Boş</b>\n\nŞu anda hiçbir kategori seçilmemiş.")
		msg.ParseMode = "HTML"
		btn := tgbotapi.NewInlineKeyboardButtonData("🎯 Kapsam Belirle", "cmd:open_filter")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
		tb.bot.Send(msg)
		return
	}

	var items []string
	for _, s := range subs {
		found := false
		for _, cat := range CategoryCatalog {
			if strings.EqualFold(cat.Key, s) {
				items = append(items, fmt.Sprintf("• <b>%s</b>", cat.Label))
				found = true
				break
			}
		}
		if !found {
			items = append(items, fmt.Sprintf("• <b>%s</b>", html.EscapeString(s)))
		}
	}

	text := fmt.Sprintf("📋 <b>Aktif Radar İzleme Kapsamınız (%d Kategori):</b>\n\n%s\n\n<i>Bu kategorilere yeni bir tehdit eklendiğinde radar otomatik olarak bildirim gönderecektir.</i>", len(subs), strings.Join(items, "\n"))

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"

	btnEdit := tgbotapi.NewInlineKeyboardButtonData("⚙️ Filtreleri Düzenle", "cmd:open_filter")
	btnClear := tgbotapi.NewInlineKeyboardButtonData("🗑️ Kapsamı Sıfırla", "cmd:clear_filter")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(btnEdit, btnClear),
	)

	tb.bot.Send(msg)
}

func formatArticleCard(a *model.Article) string {
	var cves []string
	for _, t := range a.Tags {
		if strings.HasPrefix(strings.ToUpper(t), "CVE-") {
			cves = append(cves, t)
		}
	}

	cveText := ""
	if len(cves) > 0 {
		cveText = fmt.Sprintf("\n🛡️ <b>CVE:</b> <code>%s</code>", html.EscapeString(strings.Join(cves, ", ")))
	}

	tagsText := ""
	if len(a.Tags) > 0 {
		tagsText = fmt.Sprintf("\n🏷️ <b>Etiketler:</b> <i>%s</i>", html.EscapeString(strings.Join(a.Tags, ", ")))
	}

	summary := a.Summary
	if len(summary) > 220 {
		summary = summary[:217] + "..."
	}

	return fmt.Sprintf("⚡ <b>%s</b>\n🎯 <b>Öncelik Skoru:</b> <code>%d/100</code> | 📰 <b>Kaynak:</b> <code>%s</code>%s%s\n\n📝 %s\n\n🔗 <a href=\"%s\">Detaylı Rapor</a>",
		html.EscapeString(a.Title),
		a.Score,
		html.EscapeString(a.Source),
		cveText,
		tagsText,
		html.EscapeString(summary),
		a.Link,
	)
}

func (tb *TelegramBot) DispatchAlert(ctx context.Context, articles []*model.Article) {
	subscribers, err := tb.storage.GetAllSubscribers(ctx)
	if err != nil || len(subscribers) == 0 {
		return
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	for k, t := range tb.sentAlerts {
		if now.Sub(t) > 24*time.Hour {
			delete(tb.sentAlerts, k)
		}
	}

	for _, a := range articles {
		card := formatArticleCard(a)
		alertText := fmt.Sprintf("🔔 <b>YENİ TEHDİT BULGUSU!</b>\n%s", card)

		for chatID, userTags := range subscribers {
			alertKey := fmt.Sprintf("%d:%s", chatID, a.Link)
			if _, alreadySent := tb.sentAlerts[alertKey]; alreadySent {
				continue
			}

			if matchArticleWithTags(a, userTags) {
				msg := tgbotapi.NewMessage(chatID, alertText)
				msg.ParseMode = "HTML"
				msg.DisableWebPagePreview = true
				if _, err := tb.bot.Send(msg); err == nil {
					tb.sentAlerts[alertKey] = now
				}
			}
		}
	}
}

func matchArticleWithTags(a *model.Article, userTags []string) bool {
	joinedTags := strings.ToLower(strings.Join(a.Tags, " "))
	for _, t := range userTags {
		cleanT := strings.ToLower(strings.TrimSpace(t))
		if cleanT == "critical" && a.Score >= 50 {
			return true
		}
		if cleanT == "cve" && (strings.Contains(joinedTags, "cve-") || strings.Contains(strings.ToLower(a.Title), "cve-")) {
			return true
		}
		if cleanT == "tr-focus" && (strings.Contains(joinedTags, "tr-focus") || strings.Contains(strings.ToLower(a.Source), "usom")) {
			return true
		}
		if cleanT != "critical" && cleanT != "cve" && strings.Contains(joinedTags, cleanT) {
			return true
		}
	}
	return false
}
