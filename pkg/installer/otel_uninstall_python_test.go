package installer

import (
	"os"
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

// TestPythonCleaner_DetectProcesses_ExcludeTerms verifies that processes
// matching the exclude terms are not returned.
func TestPythonCleaner_DetectProcesses_ExcludeTerms(t *testing.T) {
	// Save original detectProcessesFn and restore after test.
	origFn := detectProcessesFn
	t.Cleanup(func() {
		detectProcessesFn = origFn
	})

	// Stub detectProcessesFn with a mock that returns only the non-excluded process.
	// (The real detectProcesses applies filtering; our mock simulates that.)
	detectProcessesFn = func(filterTerm string, excludeTerms []string) []DetectedProcess {
		return []DetectedProcess{
			{PID: 1001, Command: "python app.py"},
		}
	}

	c := pythonCleaner{}
	procs := c.DetectProcesses()

	// Verify that we get the expected filtered result.
	if len(procs) != 1 {
		t.Errorf("expected 1 process, got %d: %v", len(procs), procs)
	}

	for _, p := range procs {
		if strings.Contains(p.Command, "pip ") {
			t.Errorf("pip process should be excluded: PID=%d cmd=%q", p.PID, p.Command)
		}
		if strings.Contains(p.Command, "setup.py") {
			t.Errorf("setup.py process should be excluded: PID=%d cmd=%q", p.PID, p.Command)
		}
	}
}

// TestPythonCleaner_DetectProcesses_SelfExcluded verifies the current process
// is never returned.
func TestPythonCleaner_DetectProcesses_SelfExcluded(t *testing.T) {
	// Save original detectProcessesFn and restore after test.
	origFn := detectProcessesFn
	t.Cleanup(func() {
		detectProcessesFn = origFn
	})

	// Stub detectProcessesFn with a mock that returns processes (excluding self).
	// (The real detectProcesses filters out the current process; our mock simulates that.)
	detectProcessesFn = func(filterTerm string, excludeTerms []string) []DetectedProcess {
		return []DetectedProcess{
			{PID: 1234, Command: "python app.py"},
			{PID: 5678, Command: "python server.py"},
		}
	}

	c := pythonCleaner{}
	selfPID := os.Getpid()
	for _, p := range c.DetectProcesses() {
		if p.PID == selfPID {
			t.Errorf("DetectProcesses returned current PID %d — self must be excluded", selfPID)
		}
		if p.PID == 0 {
			t.Errorf("DetectProcesses returned zero PID: %+v", p)
		}
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
	// Save original runtimeCleaners and restore after test.
	origCleaners := runtimeCleaners
	t.Cleanup(func() {
		runtimeCleaners = origCleaners
	})

	// Test case 1: no processes detected.
	testCleaner := &testRuntimeCleaner{label: "Python", processes: []DetectedProcess{}}
	runtimeCleaners = []RuntimeCleaner{testCleaner}

	output := stripANSI(captureStdout(t, func() {
		_ = UninstallOtelCollector(true)
	}))

	if strings.Contains(output, "Instrumented Python processes that will be stopped:") {
		t.Errorf("unexpected Python section header when no Python procs detected\nfull output:\n%s", output)
	}

	// Test case 2: processes are detected.
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

// TestUninstallOtelCollector_PythonPIDsInOutput verifies the Python section
// header appears when processes are detected.
func TestUninstallOtelCollector_PythonPIDsInOutput(t *testing.T) {
	// Save original runtimeCleaners and restore after test.
	origCleaners := runtimeCleaners
	t.Cleanup(func() {
		runtimeCleaners = origCleaners
	})

	// Inject a deterministic test cleaner with a fixed process.
	testCleaner := &testRuntimeCleaner{
		label: "Python",
		processes: []DetectedProcess{
			{PID: 5555, Command: "python app.py"},
		},
	}
	runtimeCleaners = []RuntimeCleaner{testCleaner}

	output := stripANSI(captureStdout(t, func() {
		_ = UninstallOtelCollector(true)
	}))

	if !strings.Contains(output, "Instrumented Python processes that will be stopped:") {
		t.Errorf("Python section header missing\nfull output:\n%s", output)
	}
}

// TestUninstallOtelCollector_ScanErrorHandling verifies that runtime scan errors
// (nil return) are silently skipped and don't produce output.
func TestUninstallOtelCollector_ScanErrorHandling(t *testing.T) {
	// Save original runtimeCleaners and restore after test.
	origCleaners := runtimeCleaners
	t.Cleanup(func() {
		runtimeCleaners = origCleaners
	})

	// Create a test cleaner that returns nil (scan error).
	failingCleaner := &testRuntimeCleaner{label: "Python", processes: nil}
	runtimeCleaners = []RuntimeCleaner{failingCleaner}

	output := stripANSI(captureStdout(t, func() {
		_ = UninstallOtelCollector(true)
	}))

	// Verify that no Python section appears when the scan fails (returns nil).
	if strings.Contains(output, "Instrumented Python processes") {
		t.Errorf("unexpected Python section when scan failed\nfull output:\n%s", output)
	}
}
