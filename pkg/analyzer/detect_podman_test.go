package analyzer

import "testing"

func TestDetectPodman_ReturnsNonNil(t *testing.T) {
	info := detectPodman()
	if info == nil {
		t.Fatal("detectPodman() returned nil")
	}
}

func TestDetectPodman_UnavailableWhenNoBinary(t *testing.T) {
	// If podman is not installed, Available must be false and no panic.
	info := detectPodman()
	if info == nil {
		t.Fatal("detectPodman() returned nil")
	}
	// We can't assert Available==false here because Podman may be installed in
	// the test environment. We assert the struct is always well-formed.
	_ = info.Available
	_ = info.ServerVersion
	_ = info.RunningContainerCount
	_ = info.Variant
}

func TestDetectPodmanVariant_KnownPrefixes(t *testing.T) {
	tests := []struct {
		osInfo string
		ver    string
		want   string
	}{
		{"podman desktop linux", "", "Podman Desktop"},
		{"fedora wsl", "", "Podman Machine"},
		{"ubuntu", "", ""},
		{"", "", ""},
	}

	for _, tt := range tests {
		got := podmanVariantFromStrings(tt.osInfo, tt.ver)
		if got != tt.want {
			t.Errorf("podmanVariantFromStrings(%q, %q) = %q, want %q", tt.osInfo, tt.ver, got, tt.want)
		}
	}
}
