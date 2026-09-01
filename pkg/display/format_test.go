package display

import (
	"testing"
	"time"
)

func TestFormatCount(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		expected string
	}{
		{"zero", 0, "0"},
		{"single digit", 7, "7"},
		{"three digits", 999, "999"},
		{"exactly 1000", 1000, "1,000"},
		{"four digits", 1234, "1,234"},
		{"six digits", 123456, "123,456"},
		{"seven digits", 1234567, "1,234,567"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCount(tt.n)
			if got != tt.expected {
				t.Errorf("FormatCount(%d) = %q, want %q", tt.n, got, tt.expected)
			}
		})
	}
}

func TestFormatElapsed(t *testing.T) {
	tests := []struct {
		name     string
		d        time.Duration
		expected string
	}{
		{"zero", 0, "0s"},
		{"negative clamped to zero", -5 * time.Second, "0s"},
		{"seconds only", 45 * time.Second, "45s"},
		{"exactly one minute", time.Minute, "1m 0s"},
		{"minutes and seconds", 2*time.Minute + 30*time.Second, "2m 30s"},
		{"59 seconds", 59 * time.Second, "59s"},
		{"60 seconds", 60 * time.Second, "1m 0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatElapsed(tt.d)
			if got != tt.expected {
				t.Errorf("FormatElapsed(%v) = %q, want %q", tt.d, got, tt.expected)
			}
		})
	}
}
