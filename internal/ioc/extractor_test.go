package ioc

import (
	"testing"

	"ctifeed/internal/model"
)

func TestExtractIoCs(t *testing.T) {
	sampleText := `
		Security researchers spotted a new attack campaign deploying LockBit ransomware.
		Malicious C2 servers identified: 194.26.29.112 and 45.142.214.22.
		Defanged payload URL: hxxps://evil-campaign[.]top/payload.exe
		Another domain mentioned: c2-server[.]ru
		Local tests were on 127.0.0.1 and 192.168.1.100 with DNS 8.8.8.8.
		Legitimate research reported by bleepingcomputer.com, github.com and cisa.gov.
		Victim companies mentioned in breach alerts: hopcharge.com, science.co and pattons.com should NOT be extracted.
		Malware SHA256: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
		Dropper MD5: 5d41402abc4b2a76b9719d911017c592
	`

	iocs := Extract(sampleText)

	if len(iocs) == 0 {
		t.Fatal("expected extracted IoCs, got 0")
	}

	foundMap := make(map[string]bool)
	for _, item := range iocs {
		foundMap[item.Type+":"+item.Value] = true
	}

	// Verify extracted malicious IoCs
	expected := []struct {
		iocType string
		val     string
	}{
		{model.IoCTypeIP, "194.26.29.112"},
		{model.IoCTypeIP, "45.142.214.22"},
		{model.IoCTypeDomain, "evil-campaign.top"},
		{model.IoCTypeDomain, "c2-server.ru"},
		{model.IoCTypeSHA256, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{model.IoCTypeMD5, "5d41402abc4b2a76b9719d911017c592"},
	}

	for _, exp := range expected {
		key := exp.iocType + ":" + exp.val
		if !foundMap[key] {
			t.Errorf("expected IoC %s was NOT extracted", key)
		}
	}

	// Verify whitelisted and non-defanged domains are excluded
	prohibited := []struct {
		iocType string
		val     string
	}{
		{model.IoCTypeIP, "127.0.0.1"},
		{model.IoCTypeIP, "192.168.1.100"},
		{model.IoCTypeIP, "8.8.8.8"},
		{model.IoCTypeDomain, "bleepingcomputer.com"},
		{model.IoCTypeDomain, "github.com"},
		{model.IoCTypeDomain, "cisa.gov"},
		{model.IoCTypeDomain, "hopcharge.com"},
		{model.IoCTypeDomain, "science.co"},
		{model.IoCTypeDomain, "pattons.com"},
	}

	for _, prh := range prohibited {
		key := prh.iocType + ":" + prh.val
		if foundMap[key] {
			t.Errorf("prohibited/whitelisted IoC %s should NOT have been extracted", key)
		}
	}
}
