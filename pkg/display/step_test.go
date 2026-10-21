package display

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

func captureSteps(t *testing.T, animated bool) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	oldOut, oldAnim, oldNoColor := stepOutput, stepAnimated, color.NoColor
	stepOutput = buf
	stepAnimated = func() bool { return animated }
	color.NoColor = true
	t.Cleanup(func() {
		stepOutput, stepAnimated, color.NoColor = oldOut, oldAnim, oldNoColor
		logger.Init(false, 0)
	})
	logger.Init(false, 0)
	return buf
}

func TestRunStepSuccessPlain(t *testing.T) {
	buf := captureSteps(t, false)
	err := RunStep("Downloaded installer", func(s *Step) error {
		s.Detail("84 MB")
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := buf.String(), "  ✓ Downloaded installer  84 MB\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestRunStepFailureShowsErrorAndHint(t *testing.T) {
	buf := captureSteps(t, false)
	wantErr := errors.New("command \"helm\" failed: exit status 1\nError: chart not found")
	err := RunStep("Deployed Operator", func(*Step) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("error not propagated: %v", err)
	}
	if !IsReported(err) || IsReported(wantErr) {
		t.Errorf("expected only the returned error to be marked reported")
	}
	out := buf.String()
	for _, want := range []string{"✗ Deployed Operator", "Error: chart not found", "Re-run with -v"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunStepFailureOmitsHintWhenVerbose(t *testing.T) {
	buf := captureSteps(t, false)
	logger.Init(false, 1)
	_ = RunStep("step", func(*Step) error { return errors.New("boom") })
	if strings.Contains(buf.String(), "Re-run with") {
		t.Errorf("hint should be omitted in verbose mode:\n%s", buf.String())
	}
}

func TestRunStepFailureTailsLongErrors(t *testing.T) {
	buf := captureSteps(t, false)
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	_ = RunStep("step", func(*Step) error { return errors.New(strings.Join(lines, "\n")) })
	out := buf.String()
	if strings.Contains(out, "line 1\n") || !strings.Contains(out, "line 30") {
		t.Errorf("expected only the last %d lines:\n%s", errorTailLines, out)
	}
}

func TestRunStepAnimatedClearsSpinnerLine(t *testing.T) {
	buf := captureSteps(t, true)
	_ = RunStep("Working", func(*Step) error { return nil })
	out := buf.String()
	if !strings.Contains(out, "Working…") {
		t.Errorf("spinner line missing:\n%q", out)
	}
	if !strings.HasSuffix(out, "\r\033[2K  ✓ Working\n") {
		t.Errorf("spinner not collapsed to a done line:\n%q", out)
	}
}

func TestDetailOnlyInVerbose(t *testing.T) {
	buf := captureSteps(t, false)
	Detail("hidden %d", 1)
	if buf.Len() != 0 {
		t.Errorf("Detail printed without -v: %q", buf.String())
	}
	logger.Init(false, 1)
	Detail("shown %d", 2)
	if got, want := buf.String(), "    shown 2\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestStepWarnAndDone(t *testing.T) {
	buf := captureSteps(t, false)
	StepDone("Detected OS", "darwin/arm64")
	StepWarn("Extension", "not activated")
	want := "  ✓ Detected OS  darwin/arm64\n  ⚠ Extension  not activated\n"
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

func TestPrintInsideRunningStepClearsSpinnerLine(t *testing.T) {
	buf := captureSteps(t, true)
	_ = RunStep("Working", func(*Step) error {
		time.Sleep(2 * spinnerInterval)
		StepWarn("Heads up", "careful")
		time.Sleep(2 * spinnerInterval)
		return nil
	})
	if !strings.Contains(buf.String(), "\r\033[2K  \u26a0 Heads up  careful\n") {
		t.Errorf("warning not printed on a cleared line:\n%q", buf.String())
	}
}
