package display

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
)

// captureOutput redirects color.Output (used by fatih/color Printf/Println)
// to a buffer for the duration of fn. Colors are disabled,
// so assertions are not fragile against terminal capability detection.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer

	origOutput := color.Output
	color.Output = &buf
	t.Cleanup(func() { color.Output = origOutput })

	origNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = origNoColor })

	fn()

	return buf.String()
}

func TestHeader_PrintsIndentedTitle(t *testing.T) {
	got := captureOutput(t, func() {
		Header("Connection Status")
	})
	if !strings.Contains(got, "  Connection Status\n") {
		t.Errorf("Header() = %q, want output to contain indented title", got)
	}
	if !strings.Contains(got, "─") {
		t.Errorf("Header() = %q, want output to contain section divider", got)
	}
}

func TestPrintSectionDivider_PrintsIndentedSeparator(t *testing.T) {
	got := captureOutput(t, func() {
		PrintSectionDivider()
	})
	if !strings.HasPrefix(got, "  ") {
		t.Errorf("PrintSectionDivider() output missing two-space indent: %q", got)
	}
	if !strings.Contains(got, "─") {
		t.Errorf("PrintSectionDivider() output missing separator character: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("PrintSectionDivider() output missing trailing newline: %q", got)
	}
}

func TestPrintStatusLine_FormatsLabelAndMessage(t *testing.T) {
	got := captureOutput(t, func() {
		PrintStatusLine("Environment", "✓ https://abc.live.com", ColorOK)
	})
	want := "  Environment:  ✓ https://abc.live.com\n"
	if got != want {
		t.Errorf("PrintStatusLine() = %q, want %q", got, want)
	}
}

func TestPrintStatusLine_ErrorMessage(t *testing.T) {
	got := captureOutput(t, func() {
		PrintStatusLine("Access Token", "✗ not set (use --access-token or DT_ACCESS_TOKEN)", ColorError)
	})
	want := "  Access Token:  ✗ not set (use --access-token or DT_ACCESS_TOKEN)\n"
	if got != want {
		t.Errorf("PrintStatusLine() = %q, want %q", got, want)
	}
}

func TestPrintStatusLine_EmptyMessage(t *testing.T) {
	got := captureOutput(t, func() {
		PrintStatusLine("Label", "", ColorOK)
	})
	want := "  Label:  \n"
	if got != want {
		t.Errorf("PrintStatusLine() with empty message = %q, want %q", got, want)
	}
}

func TestPrintFlagLine_NoColonAfterLabel(t *testing.T) {
	got := captureOutput(t, func() {
		PrintFlagLine("DTWIZ_ALL_RUNTIMES", "✓ enabled (env)", ColorOK)
	})
	want := "  DTWIZ_ALL_RUNTIMES  ✓ enabled (env)\n"
	if got != want {
		t.Errorf("PrintFlagLine() = %q, want %q", got, want)
	}
}

func TestPrintFlagLine_DiffersFromPrintStatusLine(t *testing.T) {
	label, message := "DTWIZ_ALL_RUNTIMES", "✓ enabled (cli)"
	flagLine := captureOutput(t, func() { PrintFlagLine(label, message, ColorOK) })
	statusLine := captureOutput(t, func() { PrintStatusLine(label, message, ColorOK) })
	if flagLine == statusLine {
		t.Error("PrintFlagLine() and PrintStatusLine() should produce different output (colon vs no colon)")
	}
	if strings.Contains(flagLine, label+":") {
		t.Errorf("PrintFlagLine() must not include a colon after the label, got %q", flagLine)
	}
}
