package scorer

import (
	"html"
	"regexp"
	"sort"
	"strings"

	"ctifeed/internal/model"
)

var (
	// cveRegex, standart CVE kimliklerini eşleştiren düzenli ifadedir.
	cveRegex = regexp.MustCompile(`(?i)\bCVE-\d{4}-\d{4,7}\b`)
	// htmlTagRegex, özet metinlerdeki HTML etiketlerini ayıklar.
	htmlTagRegex = regexp.MustCompile(`<[^>]*>`)
)

// Türkiye odağı için taranacak anahtar kelimeler (büyük/küçük harf duyarsız kontrol edilir)
var turkeyKeywords = []string{
	"turkey",
	"türkiye",
	"turkish",
	"turk",
	"usom",
	"btk",
	"ankara",
	"istanbul",
}

// Kritik kurumsal servis ve ürünler ile kanonik etiket adları
var criticalProducts = map[string]string{
	"fortinet":         "fortinet",
	"fortios":          "fortinet",
	"wordpress":        "wordpress",
	"palo alto":        "palo-alto",
	"cisco":            "cisco",
	"ivanti":           "ivanti",
	"vmware":           "vmware",
	"exchange":         "microsoft-exchange",
	"active directory": "active-directory",
}

// Kritik tehdit vektörleri ve kanonik etiket adları
var threatVectors = map[string]string{
	"zero-day":    "zero-day",
	"0-day":       "zero-day",
	"rce":         "rce",
	"ransomware":  "ransomware",
	"data breach": "data-breach",
	"leak":        "leak",
	"apt":         "apt",
}

// StripHTML, metin içerisindeki HTML etiketlerini ve özel karakter kodlamalarını temizler.
func StripHTML(input string) string {
	// Paragraf, başlık ve satır sonu gibi blok etiketlerini boşluk ile değiştir
	blockRegex := regexp.MustCompile(`(?i)</?(?:p|div|br|h[1-6]|li|tr|blockquote)[^>]*>`)
	text := blockRegex.ReplaceAllString(input, " ")
	// Kalan tüm diğer etiketleri temizle
	text = htmlTagRegex.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}

// Evaluate, haberin başlık ve özetini analiz ederek kümülatif tehdit/öncelik puanını hesaplar ve ilgili etiketleri atar.
func Evaluate(title, summary string) model.ScoringResult {
	combined := strings.ToLower(title + " " + summary)

	score := 0
	tagSet := make(map[string]struct{})
	breakdown := make(map[string]int)

	// 1. Türkiye Odağı (+50 Puan)
	hasTR := false
	for _, kw := range turkeyKeywords {
		if strings.Contains(combined, kw) {
			hasTR = true
			break
		}
	}
	if hasTR {
		score += 50
		tagSet["TR-Focus"] = struct{}{}
		breakdown["TR-Focus"] = 50
	}

	// 2. Kritik Servis/Ürün Zafiyetleri (+30 Puan)
	matchedProducts := make(map[string]struct{})
	for kw, canonicalTag := range criticalProducts {
		if containsWordOrPhrase(combined, kw) {
			matchedProducts[canonicalTag] = struct{}{}
		}
	}
	if len(matchedProducts) > 0 {
		score += 30
		for prodTag := range matchedProducts {
			tagSet[prodTag] = struct{}{}
		}
		breakdown["Critical-Products"] = 30
	}

	// 3. Regex Tabanlı CVE Tespiti (+35 Puan)
	rawCVEs := cveRegex.FindAllString(title+" "+summary, -1)
	if len(rawCVEs) > 0 {
		score += 35
		for _, raw := range rawCVEs {
			cve := strings.ToUpper(strings.TrimSpace(raw))
			tagSet[cve] = struct{}{}
		}
		breakdown["CVE-Detected"] = 35
	}

	// 4. Kritik Tehdit Vektörleri (+20 Puan)
	matchedVectors := make(map[string]struct{})
	for kw, canonicalTag := range threatVectors {
		if containsWordOrPhrase(combined, kw) {
			matchedVectors[canonicalTag] = struct{}{}
		}
	}
	if len(matchedVectors) > 0 {
		score += 20
		for vecTag := range matchedVectors {
			tagSet[vecTag] = struct{}{}
		}
		breakdown["Threat-Vectors"] = 20
	}

	// Etiket kümesini sıralı dilime dönüştür
	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)

	return model.ScoringResult{
		Score:     score,
		Tags:      tags,
		Breakdown: breakdown,
	}
}

// containsWordOrPhrase, kelime öbeği veya kelime sınırı kurallarına göre metin eşleşmesi yapar.
func containsWordOrPhrase(text, target string) bool {
	// Çoklu kelime öbeklerinde (ör. "palo alto", "active directory", "data breach") doğrudan arama yapılır
	if strings.Contains(target, " ") || strings.Contains(target, "-") {
		return strings.Contains(text, target)
	}

	// "leak" gibi kök kelimelerde çekim eklerine (leak, leaked, leaks, leaking) izin verilir
	if target == "leak" {
		re := regexp.MustCompile(`(?i)\bleak(?:s|ed|ing)?\b`)
		return re.MatchString(text)
	}

	// "rce", "apt" gibi çok kısa kısaltmalarda hatalı pozitifleri önlemek için kelime sınırları (\b) aranır
	if len(target) <= 3 {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(target) + `\b`)
		return re.MatchString(text)
	}

	return strings.Contains(text, target)
}
