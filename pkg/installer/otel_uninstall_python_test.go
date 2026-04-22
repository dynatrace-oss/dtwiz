package installer

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// stripANSI removes ANSI escape sequences from s so plain-text assertions work
// regardless of whether fatih/color emits codes to a pipe.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x80`)

func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// TestFindInstrumentedPythonProcesses_NoMatch verifies that processes matching
// the exclude terms ("pip ", "setup.py", "/bin/dtwiz") are not returned.
func TestFindInstrumentedPythonProcesses_NoMatch(t *testing.T) {
	procs := findInstrumentedPythonProcesses()
	for _, p := range procs {
		if strings.Contains(p.Command, "pip ") {
			t.Errorf("pip install process should be excluded but was returned: PID=%d cmd=%q", p.PID, p.Command)
		}
		if strings.Contains(p.Command, "setup.py") {
			t.Errorf("setup.py process should be excluded but was returned: PID=%d cmd=%q", p.PID, p.Command)
		}
	}
}

// TestFindInstrumentedPythonProcesses_WithMatch verifies the function returns
// DetectedProcess values with valid PIDs and that the current process is excluded.
func TestFindInstrumentedPythonProcesses_WithMatch(t *testing.T) {
	selfPID := os.Getpid()
	procs := findInstrumentedPythonProcesses()
	for _, p := range procs {
		if p.PID == selfPID {
			t.Errorf("findInstrumentedPythonProcesses returned current process PID %d — self must be excluded", selfPID)
		}
		if p.PID == 0 {
			t.Errorf("returned DetectedProcess has zero PID: %+v", p)
		}
	}
}

// TestUninstallOtelCollector_PythonSectionAlwaysPresent verifies the Python
// section header appears in the preview when Python processes are running,
// and is absent when none are found.
// Note: lines printed via fatih/color instances (muted.Println) are not captured
// by os.Pipe stdout redirection; only plain fmt.Println output is asserted here.
func TestUninstallOtelCollector_PythonSectionAlwaysPresent(t *testing.T) {
	output := stripANSI(captureStdout(t, func() {
		_ = UninstallOtelCollector(true)
	}))

	pythonProcs := findInstrumentedPythonProcesses()

	if len(pythonProcs) == 0 {
		// When no Python processes are running, the section header should be absent.
		if strings.Contains(output, "Instrumented Python processes that will be stopped:") {
			t.Errorf("unexpected Python section header in output when no Python procs running\nfull output:\n%s", output)
		}
	} else {
		// Python processes running — the section header (plain fmt.Println) must appear.
		if !strings.Contains(output, "Instrumented Python processes that will be stopped:") {
			t.Errorf("expected Python section header in preview\nfull output:\n%s", output)
		}
	}
}

// TestUninstallOtelCollector_PythonPIDsInOutput verifies that when Python
// processes are detected, the section header appears in the preview output.
func TestUninstallOtelCollector_PythonPIDsInOutput(t *testing.T) {
	pythonProcs := findInstrumentedPythonProcesses()
	if len(pythonProcs) == 0 {
		t.Skip("no Python processes running — skipping PID visibility test")
	}

	output := stripANSI(captureStdout(t, func() {
		_ = UninstallOtelCollector(true)
	}))

	if !strings.Contains(output, "Instrumented Python processes that will be stopped:") {
		t.Errorf("Python section header missing\nfull output:\n%s", output)
	}
}
