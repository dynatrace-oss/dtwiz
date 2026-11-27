package display

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const (
	spinnerInterval = 100 * time.Millisecond
	// errorTailLines caps how many lines of a failed step's error are printed.
	errorTailLines = 10
)

// stepMu serializes writes while a spinner may be redrawing, so StepDone,
// StepWarn and Detail can be called from inside a running step: they clear the
// spinner line first and the next tick redraws it below.
var (
	stepMu   sync.Mutex
	spinning bool
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// stepOutput overrides where step lines are written. Nil means os.Stdout,
// resolved at write time so tests that swap os.Stdout still capture output.
var stepOutput io.Writer

func stepWriter() io.Writer {
	if stepOutput != nil {
		return stepOutput
	}
	return os.Stdout
}

// stepAnimated reports whether the spinner may be drawn: only on a terminal
// (not in CI or pipes). It stays on with -v/--debug so the current step is
// always visible; log lines clear the spinner line first (see spinnerAwareLog).
var stepAnimated = isStdoutTTY

// spinnerAwareLog is the logger's output: it clears a live spinner line on the
// terminal before each log write so log lines are not appended to it.
type spinnerAwareLog struct{}

func (spinnerAwareLog) Write(p []byte) (int, error) {
	stepMu.Lock()
	defer stepMu.Unlock()
	if spinning && isTTY() {
		fmt.Fprint(os.Stderr, "\r\033[2K")
	}
	return os.Stderr.Write(p)
}

func init() { logger.Output = spinnerAwareLog{} }

// Stream is a writer for subprocess output shown during a step (-v). Like the
// logger output it clears a live spinner line before each write.
var Stream io.Writer = streamWriter{}

type streamWriter struct{}

func (streamWriter) Write(p []byte) (int, error) {
	stepMu.Lock()
	defer stepMu.Unlock()
	if spinning {
		fmt.Fprint(stepWriter(), "\r\033[2K")
	}
	return stepWriter().Write(p)
}

// Step is a handle to a running step. Detail updates the text shown after the
// label (e.g. "3/5 ready"); it is safe to call from any goroutine.
type Step struct {
	label  string
	mu     sync.Mutex
	detail string
	start  time.Time
}

// Detail sets the live detail text shown next to the spinner.
func (s *Step) Detail(format string, args ...any) {
	s.mu.Lock()
	s.detail = fmt.Sprintf(format, args...)
	s.mu.Unlock()
}

func (s *Step) currentDetail() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.detail
}

// RunStep runs fn as one visible install step. On a terminal it shows a
// spinner with elapsed time and collapses to "✓ label  detail" when fn returns
// nil, or "✗ label" followed by the tail of the error when it fails. Without a
// terminal it prints only the final line. fn must not print to stdout while
// running; use Detail for live status.
func RunStep(label string, fn func(s *Step) error) error {
	return runStep(label, fn, true)
}

// RunQuiet shows a spinner with label (present tense, e.g. "Configuring tenant")
// while fn runs, then removes it without a result line. Use it for work whose
// own helpers print their result lines (StepDone/StepWarn), so a long silent
// phase still shows progress. A failure is printed like a failed RunStep.
func RunQuiet(label string, fn func(s *Step) error) error {
	return runStep(label, fn, false)
}

func runStep(label string, fn func(s *Step) error, printSuccess bool) error {
	s := &Step{label: label, start: time.Now()}
	animated := stepAnimated()

	var stop chan struct{}
	var done sync.WaitGroup
	if animated {
		stepMu.Lock()
		spinning = true
		stepMu.Unlock()
		stop = make(chan struct{})
		done.Add(1)
		go func() {
			defer done.Done()
			s.spin(stop)
		}()
	}

	err := fn(s)

	if animated {
		close(stop)
		done.Wait()
		stepMu.Lock()
		spinning = false
		fmt.Fprint(stepWriter(), "\r\033[2K")
		stepMu.Unlock()
	}
	if err != nil {
		printStepFailure(label, err)
		return &reportedError{err: err}
	}
	if printSuccess {
		printStepDone(label, s.currentDetail())
	}
	return nil
}

// reportedError marks an error whose message RunStep already printed under the
// failed step, so the top-level handler does not print it a second time.
type reportedError struct{ err error }

func (e *reportedError) Error() string { return e.err.Error() }
func (e *reportedError) Unwrap() error { return e.err }

// IsReported reports whether err (or an error it wraps) was already shown to
// the user by a failed RunStep.
func IsReported(err error) bool {
	var r *reportedError
	return errors.As(err, &r)
}

func (s *Step) spin(stop <-chan struct{}) {
	ticker := time.NewTicker(spinnerInterval)
	defer ticker.Stop()
	for i := 0; ; i++ {
		line := fmt.Sprintf("  %s %s", ColorMessage.Sprint(spinnerFrames[i%len(spinnerFrames)]), s.label+"…")
		if d := s.currentDetail(); d != "" {
			line += "  " + ColorMuted.Sprint(d)
		}
		if elapsed := time.Since(s.start); elapsed >= 3*time.Second {
			line += "  " + ColorMuted.Sprint(FormatElapsed(elapsed))
		}
		stepMu.Lock()
		fmt.Fprintf(stepWriter(), "\r\033[2K%s", line)
		stepMu.Unlock()
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
	}
}

// StepDone prints a completed step line without running anything, for work
// that already happened (e.g. a detected prerequisite).
func StepDone(label, detail string) {
	printStepDone(label, detail)
}

// StepFail prints a failed step line without running anything.
func StepFail(label, detail string) {
	printStepLine(ColorError, "✗", label, detail)
}

// StepWarn prints a warning step line (the step finished but needs attention).
func StepWarn(label, detail string) {
	printStepLine(ColorWarning, "⚠", label, detail)
}

func printStepDone(label, detail string) {
	printStepLine(ColorOK, "✓", label, detail)
}

func printStepLine(mark *color.Color, symbol, label, detail string) {
	line := fmt.Sprintf("  %s %s", mark.Sprint(symbol), label)
	if detail != "" {
		line += "  " + ColorMuted.Sprint(detail)
	}
	writeLines(line)
}

func printStepFailure(label string, err error) {
	lines := []string{fmt.Sprintf("  %s %s  %s", ColorError.Sprint("✗"), label, ColorError.Sprint("failed"))}
	for _, line := range tailLines(err.Error(), errorTailLines) {
		lines = append(lines, "    "+line)
	}
	if !logger.Enabled() {
		lines = append(lines, "    "+ColorMuted.Sprint("Re-run with -v for commands and --debug for full details."))
	}
	writeLines(lines...)
}

// writeLines prints whole lines, first clearing a live spinner line.
func writeLines(lines ...string) {
	stepMu.Lock()
	defer stepMu.Unlock()
	w := stepWriter()
	if spinning {
		fmt.Fprint(w, "\r\033[2K")
	}
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
}

// tailLines returns the last n non-empty lines of s.
func tailLines(s string, n int) []string {
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if l = strings.TrimRight(l, " \t\r"); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// installHeaderShown is set once an outer command (dtwiz setup) has already
// printed the "Installing: <method>" header, so installers do not add a second.
var installHeaderShown bool

// MarkInstallHeaderShown records that the caller printed its own install header.
func MarkInstallHeaderShown() { installHeaderShown = true }

// InstallHeader prints an installer's section header unless the calling
// command already printed one for this install.
func InstallHeader(title string) {
	if installHeaderShown {
		return
	}
	Header(title)
}

// Detail prints a dim, indented line that only appears with -v or --debug.
// Use it for what dtwiz is doing in more depth: commands, URLs, file paths.
func Detail(format string, args ...any) {
	if !logger.Enabled() {
		return
	}
	writeLines("    " + ColorMuted.Sprint(fmt.Sprintf(format, args...)))
}
