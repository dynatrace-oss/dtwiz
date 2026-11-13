//go:build windows

package otel

import "testing"

func TestParseWinProcessLines(t *testing.T) {
	got := parseWinProcessLines([]string{`10|"C:\w\api.exe" -x`, "bad", "20|go run ."}, 20)
	if len(got) != 1 || got[0].PID != 10 || got[0].Command != `"C:\w\api.exe" -x` {
		t.Fatalf("parseWinProcessLines = %#v", got)
	}
}
