package scorer

import (
	"testing"
)

func TestTurkeyFocusScoring(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		summary  string
		expected int
		tag      string
	}{
		{
			name:     "Turkey mention in title",
			title:    "Critical Infrastructure in Turkey Targeted by Attackers",
			summary:  "A new cyber campaign was detected.",
			expected: 50,
			tag:      "TR-Focus",
		},
		{
			name:     "USOM alert",
			title:    "Security Alert from USOM Regarding Phishing",
			summary:  "Turkish authorities warned users.",
			expected: 50,
			tag:      "TR-Focus",
		},
		{
			name:     "Istanbul municipality targeted",
			title:    "Ransomware Gang Claims Attack on Municipal Services",
			summary:  "Servers in Istanbul were reportedly breached.",
			expected: 50,
			tag:      "TR-Focus",
		},
		{
			name:     "Turkish bank BDDK alert",
			title:    "Financial Sector Security Warning Issued by BDDK",
			summary:  "Banking trojans targeting online accounts in Ankara.",
			expected: 50,
			tag:      "TR-Focus",
		},
		{
			name:     "Aselsan and defense sector mention",
			title:    "Espionage Group Targets Turkish Defense Contractor Aselsan",
			summary:  "Phishing emails impersonating government agencies.",
			expected: 50,
			tag:      "TR-Focus",
		},
		{
			name:     "E-Devlet phishing warning",
			title:    "Fraudsters Launch Fake E-Devlet Portals",
			summary:  "Citizens warned against credential harvesting campaigns.",
			expected: 50,
			tag:      "TR-Focus",
		},
		{
			name:     "KVKK data breach notification",
			title:    "Data Breach Notification Published by KVKK",
			summary:  "Over 100,000 user credentials compromised.",
			expected: 50,
			tag:      "TR-Focus",
		},
		{
			name:     "Turkish institutional domain (.gov.tr)",
			title:    "Ransomware Gang Claims Attack on portal.icisleri.gov.tr",
			summary:  "Extortion group threatens data leak.",
			expected: 50,
			tag:      "TR-Focus",
		},
		{
			name:     "Turkish municipal domain (.bel.tr)",
			title:    "Municipal Services Disrupted at ankara.bel.tr",
			summary:  "DDoS attack mitigation underway.",
			expected: 50,
			tag:      "TR-Focus",
		},
		{
			name:     "Turkish accented characters in institution (TÜBİTAK and TUSAŞ)",
			title:    "Siber Saldırganlar TÜBİTAK ve TUSAŞ Altyapısını Hedef Aldı",
			summary:  "Oltalama saldırıları tespit edildi.",
			expected: 50,
			tag:      "TR-Focus",
		},
		{
			name:     "Türksat and CBDDO mentions",
			title:    "CBDDO and Türksat Issue Emergency Cybersecurity Advisory",
			summary:  "Critical infrastructure security alert.",
			expected: 50,
			tag:      "TR-Focus",
		},
		{
			name:     "No Turkey mention (boundary check for short words like gibberish)",
			title:    "Gibberish Text in Open Source Repository",
			summary:  "Security updates for worldwide organizations.",
			expected: 0,
			tag:      "TR-Focus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Evaluate(tt.title, tt.summary)
			if tt.expected > 0 {
				if res.Breakdown["TR-Focus"] != tt.expected {
					t.Errorf("expected TR-Focus score %d, got %d", tt.expected, res.Breakdown["TR-Focus"])
				}
				found := false
				for _, tag := range res.Tags {
					if tag == tt.tag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected tag %s in %v", tt.tag, res.Tags)
				}
			} else {
				if _, exists := res.Breakdown["TR-Focus"]; exists {
					t.Errorf("did not expect TR-Focus score, got %d", res.Breakdown["TR-Focus"])
				}
			}
		})
	}
}

func TestCriticalProductsScoring(t *testing.T) {
	res := Evaluate("Critical Fortinet FortiOS Flaw Actively Exploited", "Attackers are targeting Cisco routers and VMware ESXi servers.")
	if res.Breakdown["Critical-Products"] != 30 {
		t.Fatalf("expected Critical-Products score 30, got %d", res.Breakdown["Critical-Products"])
	}

	expectedTags := []string{"cisco", "fortinet", "vmware"}
	for _, exp := range expectedTags {
		found := false
		for _, tag := range res.Tags {
			if tag == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected product tag %s in %v", exp, res.Tags)
		}
	}
}

func TestNewCriticalProductsScoring(t *testing.T) {
	res := Evaluate("Critical Citrix NetScaler and Veeam Backup Vulnerabilities", "Atlassian Confluence servers and SonicWall gateways are under siege.")
	if res.Breakdown["Critical-Products"] != 30 {
		t.Fatalf("expected Critical-Products score 30, got %d", res.Breakdown["Critical-Products"])
	}

	expectedTags := []string{"atlassian", "citrix", "sonicwall", "veeam"}
	for _, exp := range expectedTags {
		found := false
		for _, tag := range res.Tags {
			if tag == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected product tag %s in %v", exp, res.Tags)
		}
	}
}

func TestCVEDetectionScoring(t *testing.T) {
	res := Evaluate("Proof of Concept Released for CVE-2024-21762 and CVE-2023-48788", "Patch immediately to prevent remote exploitation.")
	if res.Breakdown["CVE-Detected"] != 35 {
		t.Fatalf("expected CVE-Detected score 35, got %d", res.Breakdown["CVE-Detected"])
	}

	expectedCVEs := []string{"CVE-2024-21762", "CVE-2023-48788"}
	for _, cve := range expectedCVEs {
		found := false
		for _, tag := range res.Tags {
			if tag == cve {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected CVE tag %s in %v", cve, res.Tags)
		}
	}
}

func TestActiveExploitationScoring(t *testing.T) {
	res := Evaluate("Flaw Observed Actively Exploited in the Wild", "Security researchers noted that exploit code and PoC available on GitHub.")
	if res.Breakdown["Active-Exploitation"] != 25 {
		t.Fatalf("expected Active-Exploitation score 25, got %d", res.Breakdown["Active-Exploitation"])
	}

	expectedTags := []string{"in-the-wild", "poc"}
	for _, exp := range expectedTags {
		found := false
		for _, tag := range res.Tags {
			if tag == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected exploit tag %s in %v", exp, res.Tags)
		}
	}
}

func TestThreatVectorsScoring(t *testing.T) {
	res := Evaluate("New Zero-Day Exploit Leads to RCE and Ransomware Deployment", "APT group leaked internal corporate documents after a major data breach.")
	if res.Breakdown["Threat-Vectors"] != 20 {
		t.Fatalf("expected Threat-Vectors score 20, got %d", res.Breakdown["Threat-Vectors"])
	}

	expectedVectors := []string{"apt", "data-breach", "leak", "ransomware", "rce", "zero-day"}
	for _, exp := range expectedVectors {
		found := false
		for _, tag := range res.Tags {
			if tag == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected threat vector tag %s in %v", exp, res.Tags)
		}
	}
}

func TestNewThreatVectorsScoring(t *testing.T) {
	res := Evaluate("Authentication Bypass and SSRF Flaws Allow Pre-Auth Remote Access", "Attackers deploy Infostealer and connect to external C2 servers.")
	if res.Breakdown["Threat-Vectors"] != 20 {
		t.Fatalf("expected Threat-Vectors score 20, got %d", res.Breakdown["Threat-Vectors"])
	}

	expectedVectors := []string{"auth-bypass", "c2", "infostealer", "pre-auth", "ssrf"}
	for _, exp := range expectedVectors {
		found := false
		for _, tag := range res.Tags {
			if tag == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected threat vector tag %s in %v", exp, res.Tags)
		}
	}
}

func TestCumulativeScoring(t *testing.T) {
	// Sample article matching all scoring criteria:
	// TR-Focus (50) + Product (30) + CVE (35) + Exploited (25) + Threat Vector (20) = 160
	title := "Turkish Institutions Warned by USOM: Zero-Day RCE in Fortinet (CVE-2024-12345) Actively Exploited in Ransomware Attacks"
	summary := "A widespread campaign affecting WordPress sites and Fortinet firewalls in Istanbul."

	res := Evaluate(title, summary)
	expectedTotal := 50 + 35 + 30 + 25 + 20
	if res.Score != expectedTotal {
		t.Fatalf("expected total score %d, got %d (breakdown: %+v)", expectedTotal, res.Score, res.Breakdown)
	}
}

func TestStripHTML(t *testing.T) {
	input := "<p>This is <b>bold</b> text &amp; a <a href=\"https://example.com\">link</a>.</p>"
	expected := "This is bold text & a link."
	output := StripHTML(input)
	if output != expected {
		t.Fatalf("expected %q, got %q", expected, output)
	}
}
