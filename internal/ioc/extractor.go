package ioc

import (
	"net"
	"regexp"
	"strings"
	"sync"

	"ctifeed/internal/model"
)

var (
	// Extraction regular expressions
	ipv4Regex   = regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`)
	sha256Regex = regexp.MustCompile(`\b[a-fA-F0-9]{64}\b`)
	md5Regex    = regexp.MustCompile(`\b[a-fA-F0-9]{32}\b`)
	domainRegex = regexp.MustCompile(`\b(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+(?:com|net|org|io|ru|cn|top|xyz|biz|info|cc|to|co|uk|de|me|live|online|site|club|vip|pro|tk|ml|ga|cf|gq|su|onion)\b`)

	// Defanged domain and URL patterns (e.g. evil[.]com, hxxps://bad.xyz)
	defangedDotRegex = regexp.MustCompile(`(?i)\b[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\s*(?:\[\.\]|\(\.\)|\{\.\}|\[dot\]|\(dot\))\s*[a-zA-Z0-9-]+)+\b`)
	defangedURLRegex = regexp.MustCompile(`(?i)\b(?:hxxps?|hXXps?):\/\/([^\s/:"'>]+)`)

	// Defang sanitization mappings
	defangReplacer = strings.NewReplacer(
		"hxxps://", "https://",
		"hxxp://", "http://",
		"hXXps://", "https://",
		"hXXp://", "http://",
		"[.]", ".",
		"(.)", ".",
		"{.}", ".",
		"[dot]", ".",
		"(dot)", ".",
		"[:]", ":",
		"[@]", "@",
	)

	// Public DNS resolvers to exclude
	publicDNS = map[string]bool{
		"1.1.1.1":         true,
		"1.0.0.1":         true,
		"8.8.8.8":         true,
		"8.8.4.4":         true,
		"9.9.9.9":         true,
		"149.112.112.112": true,
		"208.67.222.222":  true,
		"208.67.220.220":  true,
	}

	// Common non-threat domains and platforms to ignore
	domainWhitelist = map[string]bool{
		"google.com":                true,
		"microsoft.com":             true,
		"github.com":                true,
		"apple.com":                 true,
		"amazon.com":                true,
		"cloudflare.com":            true,
		"twitter.com":               true,
		"x.com":                     true,
		"linkedin.com":              true,
		"youtube.com":               true,
		"w3.org":                    true,
		"mozilla.org":               true,
		"wikipedia.org":             true,
		"wordpress.org":             true,
		"apache.org":                true,
		"cisco.com":                 true,
		"fortinet.com":              true,
		"vmware.com":                true,
		"adobe.com":                 true,
		"cisa.gov":                  true,
		"usom.gov.tr":               true,
		"bleepingcomputer.com":      true,
		"thehackernews.com":         true,
		"securityweek.com":          true,
		"darkreading.com":           true,
		"trendmicro.com":            true,
		"sans.edu":                  true,
		"rapid7.com":                true,
		"krebsonsecurity.com":       true,
		"infosecurity-magazine.com": true,
		"therecord.media":           true,
		"cyberscoop.com":            true,
		"darkwebinformer.com":       true,
		"cert.org":                  true,
		"nist.gov":                  true,
		"mitre.org":                 true,
		"virustotal.com":            true,
		"any.run":                   true,
		"crowdstrike.com":           true,
		"welivesecurity.com":        true,
		"eset.com":                  true,
		"redcanary.com":             true,
		"live.com":                  true,
		"telegram.org":              true,
		"discord.com":               true,
	}

	mu sync.RWMutex
)

// Refang, defang edilmiş metinleri (örn: hxxp://evil[.]com) standart formata dönüştürür.
func Refang(text string) string {
	return defangReplacer.Replace(text)
}

// Extract, verilen metinden (başlık, özet, gövde) geçerli ve filtrelenmiş IoC'leri ayıklar.
func Extract(text string) []model.IoC {
	if text == "" {
		return nil
	}

	normalized := Refang(text)
	var results []model.IoC
	seen := make(map[string]bool)

	// IPv4 extraction and validation
	ipMatches := ipv4Regex.FindAllString(normalized, -1)
	for _, ipStr := range ipMatches {
		ip := net.ParseIP(ipStr)
		if ip == nil || ip.To4() == nil {
			continue
		}

		// Exclude private (RFC1918), loopback, and multicast ranges
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsMulticast() || ip.IsUnspecified() {
			continue
		}

		if publicDNS[ipStr] || ipStr == "255.255.255.255" {
			continue
		}

		key := model.IoCTypeIP + ":" + ipStr
		if !seen[key] {
			seen[key] = true
			results = append(results, model.IoC{
				Type:  model.IoCTypeIP,
				Value: ipStr,
			})
		}
	}

	// SHA256 extraction
	sha256Matches := sha256Regex.FindAllString(normalized, -1)
	for _, h := range sha256Matches {
		hLow := strings.ToLower(h)
		if strings.Trim(hLow, "0") == "" || strings.Trim(hLow, "f") == "" {
			continue
		}
		key := model.IoCTypeSHA256 + ":" + hLow
		if !seen[key] {
			seen[key] = true
			results = append(results, model.IoC{
				Type:  model.IoCTypeSHA256,
				Value: hLow,
			})
		}
	}

	// MD5 extraction
	md5Matches := md5Regex.FindAllString(normalized, -1)
	for _, h := range md5Matches {
		hLow := strings.ToLower(h)
		if strings.Trim(hLow, "0") == "" || strings.Trim(hLow, "f") == "" {
			continue
		}
		key := model.IoCTypeMD5 + ":" + hLow
		if !seen[key] {
			seen[key] = true
			results = append(results, model.IoC{
				Type:  model.IoCTypeMD5,
				Value: hLow,
			})
		}
	}

	// Domain extraction: only accept indicators explicitly defanged in raw text
	// to avoid false positives on cited victim organizations or vendors.
	var candidateDomains []string

	for _, m := range defangedDotRegex.FindAllString(text, -1) {
		candidateDomains = append(candidateDomains, Refang(m))
	}

	for _, sub := range defangedURLRegex.FindAllStringSubmatch(text, -1) {
		if len(sub) > 1 {
			candidateDomains = append(candidateDomains, Refang(sub[1]))
		}
	}

	for _, cand := range candidateDomains {
		dom := domainRegex.FindString(cand)
		if dom == "" {
			continue
		}
		domLow := strings.ToLower(strings.TrimSpace(dom))
		if isWhitelistedDomain(domLow) {
			continue
		}

		key := model.IoCTypeDomain + ":" + domLow
		if !seen[key] {
			seen[key] = true
			results = append(results, model.IoC{
				Type:  model.IoCTypeDomain,
				Value: domLow,
			})
		}
	}

	return results
}

// isWhitelistedDomain, verilen alan adının beyaz listede olup olmadığını denetler.
func isWhitelistedDomain(domain string) bool {
	if domainWhitelist[domain] {
		return true
	}

	// Alt alan adları için ana alan adını kontrol et (örn: api.github.com -> github.com)
	for wl := range domainWhitelist {
		if strings.HasSuffix(domain, "."+wl) {
			return true
		}
	}

	// Güvenilir devlet veya akademik uzantıları yok say
	if strings.HasSuffix(domain, ".gov") || strings.HasSuffix(domain, ".gov.tr") ||
		strings.HasSuffix(domain, ".mil") || strings.HasSuffix(domain, ".edu.tr") {
		return true
	}

	return false
}
