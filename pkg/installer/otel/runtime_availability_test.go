package otel

import (
	"errors"
	"testing"
)

// stubRuntimeToolchain replaces binary lookup and Python detection for the
// duration of the test. lookPaths maps a binary name to its path; a missing
// entry means "not found on PATH". pythonUsable controls DetectPython.
func stubRuntimeToolchain(t *testing.T, lookPaths map[string]string, pythonUsable bool) {
	t.Helper()
	oldLook, oldPython := runtimeLookPath, runtimeFindPython
	t.Cleanup(func() { runtimeLookPath, runtimeFindPython = oldLook, oldPython })
	runtimeLookPath = func(file string) (string, error) {
		if p, ok := lookPaths[file]; ok {
			return p, nil
		}
		return "", errors.New("not found")
	}
	runtimeFindPython = func() (string, error) {
		if pythonUsable {
			return "/usr/bin/python3", nil
		}
		return "", errors.New("no python")
	}
}

func TestCheckRuntimeAvailability(t *testing.T) {
	tests := []struct {
		name         string
		rt           runtimeInfo
		lookPaths    map[string]string
		pythonUsable bool
		wantAvail    bool
		wantReason   string
		wantMessage  string
	}{
		{
			name:      "binary present",
			rt:        runtimeInfo{name: "Java", binName: "java"},
			lookPaths: map[string]string{"java": "/usr/bin/java"},
			wantAvail: true,
		},
		{
			name:        "binary missing",
			rt:          runtimeInfo{name: "Node.js", binName: "node"},
			wantReason:  "node not found on PATH",
			wantMessage: "  Skipping Node.js instrumentation — 'node' not found on PATH.\n",
		},
		{
			name:      "no binary required",
			rt:        runtimeInfo{name: ".NET", binName: ""},
			wantAvail: true,
		},
		{
			name:         "usable python",
			rt:           runtimeInfo{name: "Python", binName: "python3"},
			pythonUsable: true,
			wantAvail:    true,
		},
		{
			name:        "python stub on PATH",
			rt:          runtimeInfo{name: "Python", binName: "python3"},
			lookPaths:   map[string]string{"python3": "/usr/local/bin/python3"},
			wantReason:  "python3 found on PATH but not usable",
			wantMessage: "  Skipping Python instrumentation — 'python3' found on PATH but not usable.\n",
		},
		{
			name:        "windows store python stub",
			rt:          runtimeInfo{name: "Python", binName: "python3"},
			lookPaths:   map[string]string{"python3": `C:\Users\me\AppData\Local\Microsoft\WindowsApps\python3.exe`},
			wantReason:  "python3 is a Windows Store stub, not a real interpreter",
			wantMessage: "  Skipping Python instrumentation — 'python3' is a Windows Store stub, not a real interpreter.\n  Install Python 3 from https://python.org or disable it in Settings → Apps → Advanced app settings → App execution aliases.\n",
		},
		{
			name:        "python missing",
			rt:          runtimeInfo{name: "Python", binName: "python3"},
			wantReason:  "python3 not found on PATH",
			wantMessage: "  Skipping Python instrumentation — 'python3' not found on PATH.\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			stubRuntimeToolchain(t, tt.lookPaths, tt.pythonUsable)

			// Act
			got := checkRuntimeAvailability(tt.rt)

			// Assert
			if got.available != tt.wantAvail || got.reason != tt.wantReason || got.message != tt.wantMessage {
				t.Errorf("checkRuntimeAvailability() = %+v, want available=%v reason=%q message=%q",
					got, tt.wantAvail, tt.wantReason, tt.wantMessage)
			}
		})
	}
}
