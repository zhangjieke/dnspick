package i18n

import "testing"

func TestDetect(t *testing.T) {
	tests := []struct {
		explicit string
		want     Lang
	}{
		{"en", EN},
		{"zh", ZH},
		{"zh-CN", ZH},
		{"zh_TW.UTF-8", ZH},
		{"ZH", ZH},
		{"", EN}, // no env vars set in test → default EN
		{"fr", EN},
		{"ja", EN},
	}
	for _, tt := range tests {
		if got := Detect(tt.explicit); got != tt.want {
			t.Errorf("Detect(%q) = %q, want %q", tt.explicit, got, tt.want)
		}
	}
}

func TestSetAndL(t *testing.T) {
	// Ensure Set(EN) gives English messages.
	Set(EN)
	if L().CatDomestic != "Domestic" {
		t.Fatalf("Set(EN): CatDomestic = %q, want %q", L().CatDomestic, "Domestic")
	}

	// Ensure Set(ZH) gives Chinese messages.
	Set(ZH)
	if L().CatDomestic != "国内" {
		t.Fatalf("Set(ZH): CatDomestic = %q, want %q", L().CatDomestic, "国内")
	}

	// Restore for other tests.
	Set(EN)
}
