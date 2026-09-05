package notifier

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ctifeed/internal/model"
	"ctifeed/internal/storage"
)

// Kullanıcıların abone olabileceği kategoriler ve buton metinleri
var AvailableTags = []struct {
	Key   string
	Label string
}{
	{Key: "tr-focus", Label: "🇹🇷 TR-Focus (USOM/TR)"},
	{Key: "kritik", Label: "🚨 Kritik (Skor 50+)"},
	{Key: "cve", Label: "🛡️ CVE Zafiyetleri"},
	{Key: "zero-day", Label: "⚡ Zero-Day"},
	{Key: "ransomware", Label: "💀 Ransomware"},
	{Key: "wordpress", Label: "🧩 WordPress"},
	{Key: "fortinet", Label: "🔥 Fortinet"},
	{Key: "cisco", Label: "🌐 Cisco"},
	{Key: "data-breach", Label: "📂 Veri Sızıntısı"},
}

type TelegramBot struct {
	bot    *tgbotapi.BotAPI
	db     *storage.DB
	token  string
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewTelegramBot, botu başlatır. Token boşsa hata vermez, botu nil döner.
func NewTelegramBot(token string, db *storage.DB) (*TelegramBot, error) {
	if token == "" {
		return nil, nil
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("telegram bot baslatilamadi: %w", err)
	}

	slog.Info("Telegram bot aktif", slog.String("bot_username", bot.Self.UserName))

	return &TelegramBot{
		bot:    bot,
		db:     db,
		token:  token,
		stopCh: make(chan struct{}),
	}, nil
}

// Start, Telegram'dan gelen mesajları ve buton tıklamalarını dinlemeye başlar
func (tb *TelegramBot) Start() {
	if tb == nil || tb.bot == nil {
		return
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := tb.bot.GetUpdatesChan(u)

	tb.wg.Add(1)
	go func() {
		defer tb.wg.Done()
		for {
			select {
			case <-tb.stopCh:
				tb.bot.StopReceivingUpdates()
				return
			case update, ok := <-updates:
				if !ok {
					return
				}
				// Buton tıklaması (Inline Callback)
				if update.CallbackQuery != nil {
					tb.handleCallback(update.CallbackQuery)
					continue
				}
				// Metin mesajı veya komut
				if update.Message != nil {
					tb.handleMessage(update.Message)
				}
			}
		}
	}()
}

// Stop, bot dinleyicisini zarifçe durdurur
func (tb *TelegramBot) Stop() {
	if tb == nil || tb.stopCh == nil {
		return
	}
	close(tb.stopCh)
	tb.wg.Wait()
}

// handleMessage, /start, /abone, /son gibi komutları işler
func (tb *TelegramBot) handleMessage(msg *tgbotapi.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch strings.ToLower(msg.Command()) {
	case "start", "yardim", "help":
		welcome := `<b>CTIFeed Siber Tehdit Radarı Botuna Hoş Geldiniz!</b>

İstihbarat akışından sadece ilgilendiğiniz kategoriler için anlık bildirim alabilirsiniz.

<b>Komutlar:</b>
/abone - Bildirim tercihlerinizi yönetin
/son - Son tespit edilen 5 kritik tehdidi görün`
		m := tgbotapi.NewMessage(msg.Chat.ID, welcome)
		m.ParseMode = tgbotapi.ModeHTML
		_, _ = tb.bot.Send(m)
		tb.sendSubscriptionMenu(ctx, msg.Chat.ID, 0)

	case "abone", "ayarlar":
		tb.sendSubscriptionMenu(ctx, msg.Chat.ID, 0)

	case "son":
		articles, err := tb.db.GetTopArticles(ctx, 5, 50)
		if err != nil || len(articles) == 0 {
			m := tgbotapi.NewMessage(msg.Chat.ID, "Kritik öncelikli yeni kayıt bulunamadı.")
			_, _ = tb.bot.Send(m)
			return
		}
		for _, a := range articles {
			m := tgbotapi.NewMessage(msg.Chat.ID, formatArticleMessage(a))
			m.ParseMode = tgbotapi.ModeHTML
			m.DisableWebPagePreview = true
			_, _ = tb.bot.Send(m)
		}
	}
}

// handleCallback, butona tıklandığında etiketi açar/kapatır ve buton menüsünü günceller
func (tb *TelegramBot) handleCallback(cb *tgbotapi.CallbackQuery) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data := cb.Data
	if strings.HasPrefix(data, "sub:") {
		tag := strings.TrimPrefix(data, "sub:")
		_, err := tb.db.ToggleSubscription(ctx, cb.Message.Chat.ID, tag)
		if err != nil {
			slog.Error("Abonelik guncellenemedi", slog.String("error", err.Error()))
		}

		// Menüyü anında güncelle (buton üzerindeki [ X ] / [   ] değişir)
		tb.sendSubscriptionMenu(ctx, cb.Message.Chat.ID, cb.Message.MessageID)

		ack := tgbotapi.NewCallback(cb.ID, "Abonelik tercihiniz güncellendi.")
		_, _ = tb.bot.Request(ack)
	}
}

// sendSubscriptionMenu, inline keyboard butonlarını dinamik oluşturup gönderir veya düzenler
func (tb *TelegramBot) sendSubscriptionMenu(ctx context.Context, chatID int64, messageID int) {
	userTags, _ := tb.db.GetUserSubscriptions(ctx, chatID)
	activeMap := make(map[string]bool)
	for _, t := range userTags {
		activeMap[t] = true
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, item := range AvailableTags {
		status := "[   ]"
		if activeMap[item.Key] {
			status = "[ X ]"
		}
		btnText := fmt.Sprintf("%s %s", status, item.Label)
		btn := tgbotapi.NewInlineKeyboardButtonData(btnText, "sub:"+item.Key)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	text := "<b>Bildirim Tercihleriniz:</b>\nBildirim almak istediğiniz kategorilerin üzerine tıklayarak seçin:"

	if messageID > 0 {
		edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, messageID, text, keyboard)
		edit.ParseMode = tgbotapi.ModeHTML
		_, _ = tb.bot.Send(edit)
	} else {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		_, _ = tb.bot.Send(msg)
	}
}

// DispatchAlert, toplanan yeni haberleri ilgili abonelere dağıtır
func (tb *TelegramBot) DispatchAlert(ctx context.Context, articles []*model.Article) {
	if tb == nil || tb.bot == nil || len(articles) == 0 {
		return
	}

	for _, a := range articles {
		var queryTags []string
		for _, t := range a.Tags {
			tLow := strings.ToLower(t)
			queryTags = append(queryTags, tLow)
			if strings.HasPrefix(tLow, "cve-") {
				queryTags = append(queryTags, "cve")
			}
		}
		if a.Score >= 50 {
			queryTags = append(queryTags, "kritik")
		}

		chatIDs, err := tb.db.GetSubscribersForTags(ctx, queryTags)
		if err != nil || len(chatIDs) == 0 {
			continue
		}

		msgText := formatArticleMessage(a)
		for _, chatID := range chatIDs {
			m := tgbotapi.NewMessage(chatID, msgText)
			m.ParseMode = tgbotapi.ModeHTML
			m.DisableWebPagePreview = true
			if _, err := tb.bot.Send(m); err != nil {
				slog.Warn("Telegram mesaji gonderilemedi", slog.Int64("chat_id", chatID), slog.String("error", err.Error()))
			}
		}
	}
}

func formatArticleMessage(a *model.Article) string {
	scoreBadge := fmt.Sprintf("Skor: %d", a.Score)
	if a.Score >= 80 {
		scoreBadge = fmt.Sprintf("🚨 KRİTİK (%d)", a.Score)
	} else if a.Score >= 50 {
		scoreBadge = fmt.Sprintf("⚠️ YÜKSEK (%d)", a.Score)
	}

	tagsStr := "Yok"
	if len(a.Tags) > 0 {
		tagsStr = html.EscapeString(strings.Join(a.Tags, ", "))
	}

	summarySnippet := a.Summary
	if len(summarySnippet) > 280 {
		summarySnippet = summarySnippet[:277] + "..."
	}

	return fmt.Sprintf(`<b>[%s] %s</b>

<b>Kaynak:</b> %s
<b>Etiketler:</b> <code>[%s]</code>

%s

🔗 <a href="%s">Habere Git</a>`,
		scoreBadge,
		html.EscapeString(a.Title),
		html.EscapeString(a.Source),
		tagsStr,
		html.EscapeString(summarySnippet),
		a.Link,
	)
}
