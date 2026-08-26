package display

import (
	"errors"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

func TestHeader_PrintsIndentedTitle(t *testing.T) {
	got := helpers.CaptureOutput(t, func() {
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
	got := helpers.CaptureOutput(t, func() {
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
	got := helpers.CaptureOutput(t, func() {
		PrintStatusLine("Environment", "✓ https://abc.live.com", ColorOK)
	})
	want := "  Environment:  ✓ https://abc.live.com\n"
	if got != want {
		t.Errorf("PrintStatusLine() = %q, want %q", got, want)
	}
}

func TestPrintStatusLine_ErrorMessage(t *testing.T) {
	got := helpers.CaptureOutput(t, func() {
		PrintStatusLine("Access Token", "✗ not set (use --access-token)", ColorError)
	})
	want := "  Access Token:  ✗ not set (use --access-token)\n"
	if got != want {
		t.Errorf("PrintStatusLine() = %q, want %q", got, want)
	}
}

func TestPrintStatusLine_EmptyMessage(t *testing.T) {
	got := helpers.CaptureOutput(t, func() {
		PrintStatusLine("Label", "", ColorOK)
	})
	want := "  Label:  \n"
	if got != want {
		t.Errorf("PrintStatusLine() with empty message = %q, want %q", got, want)
	}
}

func TestPrintFlagLine_NoColonAfterLabel(t *testing.T) {
	got := helpers.CaptureOutput(t, func() {
		PrintFlagLine("DTWIZ_ALL_RUNTIMES", "✓ enabled (env)", ColorOK)
	})
	want := "  DTWIZ_ALL_RUNTIMES  ✓ enabled (env)\n"
	if got != want {
		t.Errorf("PrintFlagLine() = %q, want %q", got, want)
	}
}

func TestPrintError_FormatsLabelAndError(t *testing.T) {
	got := helpers.CaptureOutput(t, func() {
		PrintError("Setup", errors.New("connection refused"))
	})
	want := "  Setup: ✗ connection refused\n"
	if got != want {
		t.Errorf("PrintError() = %q, want %q", got, want)
	}
}

func TestPrintWarning_FormatsLabelAndError(t *testing.T) {
	got := helpers.CaptureErrorOutput(t, func() {
		PrintWarning("EKS Bottlerocket probe", errors.New("exit status 1"))
	})
	want := "  EKS Bottlerocket probe: ⚠ exit status 1\n"
	if got != want {
		t.Errorf("PrintWarning() = %q, want %q", got, want)
	}
}

func TestPrintWarning_DiffersFromPrintError(t *testing.T) {
	err := errors.New("kubectl timeout")
	warning := helpers.CaptureErrorOutput(t, func() { PrintWarning("probe", err) })
	errOut := helpers.CaptureOutput(t, func() { PrintError("probe", err) })
	if warning == errOut {
		t.Error("PrintWarning() and PrintError() should produce different output (⚠ vs ✗)")
	}
}

func TestHyperlink_NonTTY_PlainTextFallback(t *testing.T) {
	got := hyperlink("Visit docs", "https://example.com", false)
	want := "Visit docs: https://example.com"
	if got != want {
		t.Errorf("hyperlink(tty=false) = %q, want %q", got, want)
	}
}

func TestHyperlink_TTY_OSC8Format(t *testing.T) {
	got := hyperlink("Visit docs", "https://example.com", true)
	if !strings.HasPrefix(got, "\033]8;;") {
		t.Errorf("hyperlink(tty=true) = %q, want OSC 8 prefix \\033]8;;", got)
	}
	if !strings.Contains(got, "https://example.com") {
		t.Errorf("hyperlink(tty=true) = %q, want URL in output", got)
	}
	if !strings.Contains(got, "Visit docs") {
		t.Errorf("hyperlink(tty=true) = %q, want link text in output", got)
	}
	if !strings.Contains(got, "\033\\") {
		t.Errorf("hyperlink(tty=true) = %q, want ST terminator (ESC \\)", got)
	}
}

func TestHyperlink_NonTTYEnvironment_NoEscapeCodes(t *testing.T) {
	got := Hyperlink("Visit docs", "https://example.com")
	if strings.Contains(got, "\033") {
		t.Errorf("Hyperlink() in non-TTY = %q, want no escape codes", got)
	}
}

func TestPrintInfoBox_RendersContentRow(t *testing.T) {
	const boxWidth = 97
	got := helpers.CaptureStdout(t, func() {
		PrintInfoBox("hello world")
	})
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("PrintInfoBox(1 line) produced %d lines, want 3 (top, content, bottom)", len(lines))
	}
	content := lines[1]
	if !strings.HasPrefix(content, "  │ hello world") {
		t.Errorf("content row = %q, want prefix \"  │ hello world\"", content)
	}
	if !strings.HasSuffix(content, " │") {
		t.Errorf("content row = %q, want suffix \" │\"", content)
	}
	if len([]rune(content)) != len([]rune(lines[0])) {
		t.Errorf("content row width %d differs from border width %d", len([]rune(content)), len([]rune(lines[0])))
	}
	_ = boxWidth
}

func TestPrintInfoBox_BlankLineRendersEmptyRow(t *testing.T) {
	got := helpers.CaptureStdout(t, func() {
		PrintInfoBox("line one", "", "line two")
	})
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("PrintInfoBox with blank separator produced %d lines, want 5", len(lines))
	}
	blank := lines[2]
	if !strings.HasPrefix(blank, "  │") || !strings.HasSuffix(blank, "│") {
		t.Errorf("blank row = %q, want bordered row of spaces", blank)
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(blank, "  │"), "│")
	if strings.TrimSpace(inner) != "" {
		t.Errorf("blank row inner = %q, want only spaces", inner)
	}
}

func TestPrintAlignedStatusLine_PadsLabelToWidth(t *testing.T) {
	got := helpers.CaptureOutput(t, func() {
		PrintAlignedStatusLine("API URL", "https://foo.com", 13, ColorDefault)
	})
	want := "  API URL:       https://foo.com\n"
	if got != want {
		t.Errorf("PrintAlignedStatusLine() = %q, want %q", got, want)
	}
}

func TestPrintAlignedStatusLines_ComputesSharedLabelWidth(t *testing.T) {
	got := helpers.CaptureOutput(t, func() {
		PrintAlignedStatusLines(ColorDefault, "Short", "ok", "Longer Label", "ready", "Dangling")
	})
	want := "  Short:         ok\n  Longer Label:  ready\n"
	if got != want {
		t.Errorf("PrintAlignedStatusLines() = %q, want %q", got, want)
	}
	if strings.Contains(got, "Dangling") {
		t.Errorf("PrintAlignedStatusLines() printed incomplete label/value pair: %q", got)
	}
}

func TestPrintln_IndentsAndFormats(t *testing.T) {
	got := helpers.CaptureStdout(t, func() {
		Println("hello %s", "world")
	})
	want := "  hello world\n"
	if got != want {
		t.Errorf("Println() = %q, want %q", got, want)
	}
}

func TestPrintlnColored_IndentsMessage(t *testing.T) {
	got := helpers.CaptureOutput(t, func() {
		PrintlnColored(ColorDefault, "Dynatrace Kubernetes Integration")
	})
	want := "  Dynatrace Kubernetes Integration\n"
	if got != want {
		t.Errorf("PrintlnColored() = %q, want %q", got, want)
	}
}

func TestPrintSteps_NumbersAndIndents(t *testing.T) {
	got := helpers.CaptureStdout(t, func() {
		PrintSteps("", "Ensure Helm is installed", "kubectl apply -f dynakube.yaml")
	})
	want := "  Steps:\n    1. Ensure Helm is installed\n    2. kubectl apply -f dynakube.yaml\n"
	if got != want {
		t.Errorf("PrintSteps() = %q, want %q", got, want)
	}
}

func TestPrintStepsColored_NumbersAndIndents(t *testing.T) {
	got := helpers.CaptureOutput(t, func() {
		PrintStepsColored(ColorDefault, "", "step one", "step two")
	})
	want := "  Steps:\n    1. step one\n    2. step two\n"
	if got != want {
		t.Errorf("PrintStepsColored() = %q, want %q", got, want)
	}
}

func TestPrintFlagLine_DiffersFromPrintStatusLine(t *testing.T) {
	label, message := "DTWIZ_ALL_RUNTIMES", "✓ enabled (cli)"
	flagLine := helpers.CaptureOutput(t, func() { PrintFlagLine(label, message, ColorOK) })
	statusLine := helpers.CaptureOutput(t, func() { PrintStatusLine(label, message, ColorOK) })
	if flagLine == statusLine {
		t.Error("PrintFlagLine() and PrintStatusLine() should produce different output (colon vs no colon)")
	}
	if strings.Contains(flagLine, label+":") {
		t.Errorf("PrintFlagLine() must not include a colon after the label, got %q", flagLine)
	}
}
