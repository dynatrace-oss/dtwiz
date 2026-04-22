package installer

import (
	"regexp"
	"strings"
	"testing"
)

// testRuntimeCleaner is a mock RuntimeCleaner used for deterministic testing.
type testRuntimeCleaner struct {
	label     string
	processes []DetectedProcess
}

func (m *testRuntimeCleaner) Label() string             { return m.label }
func (m *testRuntimeCleaner) DetectProcesses() []DetectedProcess {
	return m.processes
}

// stripANSI removes ANSI escape sequences from s so plain-text assertions work
// regardless of whether fatih/color emits codes to a pipe.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x80`)

func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// TestPythonCleaner_Label verifies the label is "Python".
func TestPythonCleaner_Label(t *testing.T) {
	c := pythonCleaner{}
	if c.Label() != "Python" {
		t.Errorf("Label() = %q, want \"Python\"", c.Label())
	}
}

// TestRuntimeCleaners_RegistryContainsPython verifies pythonCleaner is registered.
func TestRuntimeCleaners_RegistryContainsPython(t *testing.T) {
	found := false
	for _, c := range runtimeCleaners {
		if c.Label() == "Python" {
			found = true
		}
	}
	if !found {
		t.Error("pythonCleaner not found in runtimeCleaners registry")
	}
}

// TestUninstallOtelCollector_PythonSectionAlwaysPresent verifies that the
// Python section header appears when Python processes are detected, and
// is absent when none are found. It injects a deterministic test cleaner
// to avoid flakiness from background processes.
func TestUninstallOtelCollector_PythonSectionAlwaysPresent(t *testing.T) {
	origCleaners := runtimeCleaners
	t.Cleanup(func() { runtimeCleaners = origCleaners })

	testCleaner := &testRuntimeCleaner{label: "Python", processes: []DetectedProcess{}}
	runtimeCleaners = []RuntimeCleaner{testCleaner}

	output := stripANSI(captureStdout(t, func() {
		_ = UninstallOtelCollector(true)
	}))

	if strings.Contains(output, "Instrumented Python processes that will be stopped:") {
		t.Errorf("unexpected Python section header when no Python procs detected\nfull output:\n%s", output)
	}

	testCleaner.processes = []DetectedProcess{
		{PID: 1234, Command: "python app.py"},
	}

	output = stripANSI(captureStdout(t, func() {
		_ = UninstallOtelCollector(true)
	}))

	if !strings.Contains(output, "Instrumented Python processes that will be stopped:") {
		t.Errorf("expected Python section header when processes detected\nfull output:\n%s", output)
	}
}

// TestUninstallOtelCollector_PythonPIDsInOutput verifies that a detected PID
// appears in the preview output.
func TestUninstallOtelCollector_PythonPIDsInOutput(t *testing.T) {
	origCleaners := runtimeCleaners
	t.Cleanup(func() { runtimeCleaners = origCleaners })

	runtimeCleaners = []RuntimeCleaner{&testRuntimeCleaner{
		label:     "Python",
		processes: []DetectedProcess{{PID: 5555, Command: "python app.py"}},
	}}

	output := stripANSI(captureStdout(t, func() {
		_ = UninstallOtelCollector(true)
	}))

	if !strings.Contains(output, "Instrumented Python processes that will be stopped:") {
		t.Errorf("Python section header missing\nfull output:\n%s", output)
	}
}

// TestUninstallOtelCollector_ScanErrorHandling verifies that a nil return from
// DetectProcesses (scan error) is silently skipped and produces no output.
func TestUninstallOtelCollector_ScanErrorHandling(t *testing.T) {
	origCleaners := runtimeCleaners
	t.Cleanup(func() { runtimeCleaners = origCleaners })

	runtimeCleaners = []RuntimeCleaner{&testRuntimeCleaner{label: "Python", processes: nil}}

	output := stripANSI(captureStdout(t, func() {
		_ = UninstallOtelCollector(true)
	}))

	if strings.Contains(output, "Instrumented Python processes") {
		t.Errorf("unexpected Python section when scan failed\nfull output:\n%s", output)
	}
}
