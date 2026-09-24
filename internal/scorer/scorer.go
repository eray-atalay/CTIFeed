package scorer

import (
	"html"
	"regexp"
	"sort"
	"strings"

	"ctifeed/internal/model"
)

var (
	// cveRegex matches standard CVE identifiers.
	cveRegex = regexp.MustCompile(`(?i)\bCVE-\d{4}-\d{4,7}\b`)
	// htmlTagRegex strips HTML tags from feed summary descriptions.
	htmlTagRegex = regexp.MustCompile(`<[^>]*>`)
	// trDomainRegex matches Turkish institutional and top-level domain targets (.gov.tr, .bel.tr, .edu.tr, etc.).
	trDomainRegex = regexp.MustCompile(`(?i)\b[a-z0-9._%+-]+\.(?:gov|bel|edu|k12|org|com|net)\.tr\b`)
	// trKeywordsRegex matches normalized Turkish CTI keywords, institutions, and regulators with word boundaries.
	trKeywordsRegex = regexp.MustCompile(`(?i)\b(?:turkey|turkiye|turkish|turk|usom|btk|kvkk|cbddo|tr-cert|spk|tcmb|bddk|epdk|e-devlet|edevlet|mernis|e-nabiz|enabiz|sgk|gib|mhrs|osym|kamusm|afad|kizilay|tubitak|aselsan|havelsan|tusas|roketsan|stm|ssb|turksat|turkcell|turktelekom|superonline|papara|ininal|bkm|ziraat|vakifbank|halkbank|akbank|yapikredi|ankara|istanbul)\b`)
	// leakRegex matches leak and its inflectional forms.
	leakRegex = regexp.MustCompile(`(?i)\bleak(?:s|ed|ing)?\b`)

	// shortWordRegexes caches pre-compiled regexes for short keywords to avoid runtime compilations.
	shortWordRegexes = map[string]*regexp.Regexp{}
)

func init() {
	keywords := []string{"f5", "rce", "c2", "apt", "ssrf", "sqli", "poc"}
	for _, kw := range keywords {
		shortWordRegexes[kw] = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw) + `\b`)
	}
}

// Keywords and institutions for TR-Focus evaluation
var turkeyKeywords = []string{
	"turkey",
	"türkiye",
	"turkish",
	"turk",
	"usom",
	"btk",
	"kvkk",
	"cbddo",
	"tr-cert",
	"spk",
	"tcmb",
	"bddk",
	"epdk",
	"e-devlet",
	"edevlet",
	"mernis",
	"e-nabiz",
	"enabiz",
	"sgk",
	"gib",
	"mhrs",
	"osym",
	"kamusm",
	"afad",
	"kizilay",
	"tubitak",
	"tübitak",
	"aselsan",
	"havelsan",
	"tusas",
	"tusaş",
	"roketsan",
	"stm",
	"ssb",
	"turksat",
	"türksat",
	"turkcell",
	"turktelekom",
	"superonline",
	"papara",
	"ininal",
	"bkm",
	"ziraat",
	"vakifbank",
	"halkbank",
	"akbank",
	"yapikredi",
	"ankara",
	"istanbul",
}

// Active exploitation and PoC indicators (+25 points)
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

// Enterprise products, infrastructure, and services (+30 points)
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

// Threat vectors and techniques (+20 points)
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
	"ransomware":          "ransomware",
	"infostealer":         "infostealer",
	"stealer":             "infostealer",
	"wiper":               "wiper",
	"spyware":             "spyware",
	"c2":                  "c2",
	"command and control": "c2",

	// Sızıntı, Casusluk ve Altyapı
	"data breach":   "data-breach",
	"leak":          "leak",
	"apt":           "apt",
	"supply chain":  "supply-chain",
	"ssrf":          "ssrf",
	"sql injection": "sqli",
	"sqli":          "sqli",
}

// StripHTML strips HTML markup and unescapes entities.
func StripHTML(input string) string {
	blockRegex := regexp.MustCompile(`(?i)</?(?:p|div|br|h[1-6]|li|tr|blockquote)[^>]*>`)
	text := blockRegex.ReplaceAllString(input, " ")
	text = htmlTagRegex.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}

// Evaluate analyzes title and summary to calculate cumulative threat score and assign tags.
func Evaluate(title, summary string) model.ScoringResult {
	rawText := title + " " + summary
	combined := strings.ToLower(rawText)
	normalized := normalizeTurkish(rawText)

	score := 0
	tagSet := make(map[string]struct{})
	breakdown := make(map[string]int)

	// Turkey Focus (+50)
	if trKeywordsRegex.MatchString(normalized) || trDomainRegex.MatchString(rawText) {
		score += 50
		tagSet["TR-Focus"] = struct{}{}
		breakdown["TR-Focus"] = 50
	}

	// CVE Identification (+35)
	rawCVEs := cveRegex.FindAllString(title+" "+summary, -1)
	if len(rawCVEs) > 0 {
		score += 35
		for _, raw := range rawCVEs {
			cve := strings.ToUpper(strings.TrimSpace(raw))
			tagSet[cve] = struct{}{}
		}
		breakdown["CVE-Detected"] = 35
	}

	// Critical Enterprise Products (+30)
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

	// Active Exploitation / PoC (+25)
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

	// Threat Vectors (+20)
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

func containsWordOrPhrase(text, target string) bool {
	// Çoklu kelime öbeklerinde (ör. "palo alto", "active directory", "in the wild") doğrudan arama yapılır
	if strings.Contains(target, " ") || strings.Contains(target, "-") {
		return strings.Contains(text, target)
	}

	// "leak" gibi kök kelimelerde çekim eklerine (leak, leaked, leaks, leaking) izin verilir
	if target == "leak" {
		return leakRegex.MatchString(text)
	}

	// Önbelleklenmiş kısa kelime regex kontrolü
	if re, ok := shortWordRegexes[target]; ok {
		return re.MatchString(text)
	}

	// Kısa kısaltmalarda veya tekil kelimelerde hatalı pozitifleri önlemek için kelime sınırları (\b) aranır
	if len(target) <= 4 {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(target) + `\b`)
		return re.MatchString(text)
	}

	return strings.Contains(text, target)
}

// normalizeTurkish converts Turkish characters to their ASCII equivalents and lowercases.
func normalizeTurkish(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'ç', 'Ç':
			b.WriteRune('c')
		case 'ğ', 'Ğ':
			b.WriteRune('g')
		case 'ı', 'İ', 'I':
			b.WriteRune('i')
		case 'ö', 'Ö':
			b.WriteRune('o')
		case 'ş', 'Ş':
			b.WriteRune('s')
		case 'ü', 'Ü':
			b.WriteRune('u')
		default:
			if r >= 'A' && r <= 'Z' {
				b.WriteRune(r + ('a' - 'A'))
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
