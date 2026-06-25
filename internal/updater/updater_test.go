package updater

import "testing"

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"v1.0.0", "v1.0.1", true},
		{"v1.0.0", "v2.0.0", true},
		{"v1.0.1", "v1.0.0", false},
		{"v1.0.0", "v1.0.0", false},
		// Non-semver current (e.g. "dev") → always treat as needing update.
		{"dev", "v1.0.0", true},
		{"", "v1.0.0", true},
		{"dirty-abc123", "v1.0.0", true},
	}
	for _, tt := range tests {
		if got := isNewer(tt.current, tt.latest); got != tt.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}
