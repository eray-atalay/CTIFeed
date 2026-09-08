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

// Türkiye odağı için taranacak anahtar kelimeler ve kurumlar (büyük/küçük harf duyarsız kontrol edilir)
var turkeyKeywords = []string{
	"turkey",
	"türkiye",
	"turkish",
	"turk",
	"usom",
	"btk",
	"ankara",
	"istanbul",
	"e-devlet",
	"sgk",
	"gib",
	"tcmb",
	"bddk",
	"epdk",
	"tubitak",
	"tübitak",
	"aselsan",
	"havelsan",
	"tai",
	"tusas",
	"tusaş",
	"botas",
	"botaş",
	"tupras",
	"tüpraş",
	"turkcell",
	"vodafone",
	"türk telekom",
	"turk telekom",
	"bkm",
	"bist",
}

// Aktif İstismar ve PoC (In-the-Wild / Exploit) göstergeleri (+25 Puan)
var activeExploitation = map[string]string{
	"actively exploited": "in-the-wild",
	"in the wild":        "in-the-wild",
	"in-the-wild":        "in-the-wild",
	"under attack":       "active-exploitation",
	"targeted attack":    "active-exploitation",
	"targeted attacks":   "active-exploitation",
	"poc available":      "poc",
	"exploit code":       "poc",
	"proof-of-concept":   "poc",
	"proof of concept":   "poc",
	"weaponized":         "active-exploitation",
	"cisa kev":           "in-the-wild",
	"known exploited":    "in-the-wild",
}

// Kritik kurumsal servis, ağ cihazı ve ürünler ile kanonik etiket adları (+30 Puan)
var criticalProducts = map[string]string{
	// Ağ ve Güvenlik Çevresi (Firewall / VPN / Gateway)
	"fortinet":    "fortinet",
	"fortios":     "fortinet",
	"palo alto":   "palo-alto",
	"cisco":       "cisco",
	"ivanti":      "ivanti",
	"citrix":      "citrix",
	"netscaler":   "citrix",
	"sonicwall":   "sonicwall",
	"check point": "check-point",
	"checkpoint":  "check-point",
	"f5":          "f5",
	"big-ip":      "f5",
	"juniper":     "juniper",

	// Yedekleme ve Dosya Transferi (Fidye Yazılımlarının Ana Hedefleri)
	"veeam":      "veeam",
	"moveit":     "moveit",
	"goanywhere": "goanywhere",

	// Sanallaştırma ve Bulut Altyapısı
	"vmware":       "vmware",
	"kubernetes":   "kubernetes",
	"openssh":      "openssh",
	"linux kernel": "linux",

	// Kimlik Yönetimi ve Kurumsal İş Yazılımları
	"exchange":         "microsoft-exchange",
	"active directory": "active-directory",
	"entra id":         "entra-id",
	"azure ad":         "entra-id",
	"sharepoint":       "sharepoint",
	"outlook":          "outlook",
	"atlassian":        "atlassian",
	"confluence":       "atlassian",
	"jira":             "atlassian",
	"wordpress":        "wordpress",
}

// Kritik tehdit vektörleri ve kanonik etiket adları (+20 Puan)
var threatVectors = map[string]string{
	// Kod Çalıştırma ve Yetki
	"zero-day":              "zero-day",
	"0-day":                 "zero-day",
	"rce":                   "rce",
	"auth bypass":           "auth-bypass",
	"authentication bypass": "auth-bypass",
	"privilege escalation":  "privilege-escalation",
	"privesc":               "privilege-escalation",
	"pre-auth":              "pre-auth",

	// Zararlı Yazılım ve Casusluk
	"ransomware": "ransomware",
	"infostealer": "infostealer",
	"stealer":    "infostealer",
	"wiper":      "wiper",
	"spyware":    "spyware",
	"c2":         "c2",
	"command and control": "c2",

	// Sızıntı, Casusluk ve Altyapı
	"data breach":  "data-breach",
	"leak":         "leak",
	"apt":          "apt",
	"supply chain": "supply-chain",
	"ssrf":         "ssrf",
	"sql injection": "sqli",
	"sqli":         "sqli",
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
		if containsWordOrPhrase(combined, kw) {
			hasTR = true
			break
		}
	}
	if hasTR {
		score += 50
		tagSet["TR-Focus"] = struct{}{}
		breakdown["TR-Focus"] = 50
	}

	// 2. Regex Tabanlı CVE Tespiti (+35 Puan)
	rawCVEs := cveRegex.FindAllString(title+" "+summary, -1)
	if len(rawCVEs) > 0 {
		score += 35
		for _, raw := range rawCVEs {
			cve := strings.ToUpper(strings.TrimSpace(raw))
			tagSet[cve] = struct{}{}
		}
		breakdown["CVE-Detected"] = 35
	}

	// 3. Kritik Kurumsal Servis/Ürün Zafiyetleri (+30 Puan)
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

	// 4. Aktif Sömürü ve PoC Göstergeleri (+25 Puan)
	matchedExploits := make(map[string]struct{})
	for kw, canonicalTag := range activeExploitation {
		if containsWordOrPhrase(combined, kw) {
			matchedExploits[canonicalTag] = struct{}{}
		}
	}
	if len(matchedExploits) > 0 {
		score += 25
		for expTag := range matchedExploits {
			tagSet[expTag] = struct{}{}
		}
		breakdown["Active-Exploitation"] = 25
	}

	// 5. Kritik Tehdit Vektörleri (+20 Puan)
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
	// Çoklu kelime öbeklerinde (ör. "palo alto", "active directory", "in the wild") doğrudan arama yapılır
	if strings.Contains(target, " ") || strings.Contains(target, "-") {
		return strings.Contains(text, target)
	}

	// "leak" gibi kök kelimelerde çekim eklerine (leak, leaked, leaks, leaking) izin verilir
	if target == "leak" {
		re := regexp.MustCompile(`(?i)\bleak(?:s|ed|ing)?\b`)
		return re.MatchString(text)
	}

	// Kısa kısaltmalarda veya tekil kelimelerde hatalı pozitifleri önlemek için kelime sınırları (\b) aranır
	// (Örn. "rce", "apt", "f5", "c2", "sgk", "gib", "btk", "bist", "ssrf")
	if len(target) <= 4 {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(target) + `\b`)
		return re.MatchString(text)
	}

	return strings.Contains(text, target)
}
