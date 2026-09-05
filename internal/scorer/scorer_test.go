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
			name:     "No Turkey mention",
			title:    "Global Cyber Trends 2024",
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

func TestCumulativeScoring(t *testing.T) {
	// 4 kategorinin tumuyle eslesen ornek haber:
	// 1. Turkiye Odagi (+50): "Turkish", "USOM"
	// 2. Kritik Urun (+30): "Fortinet", "WordPress"
	// 3. CVE (+35): "CVE-2024-12345"
	// 4. Tehdit Vektoru (+20): "zero-day", "RCE", "ransomware"
	// Toplam Puan: 50 + 30 + 35 + 20 = 135
	title := "Turkish Institutions Warned by USOM: Zero-Day RCE in Fortinet (CVE-2024-12345) Leveraged in Ransomware Attacks"
	summary := "A widespread campaign affecting WordPress sites and Fortinet firewalls in Istanbul."

	res := Evaluate(title, summary)
	expectedTotal := 50 + 30 + 35 + 20
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
