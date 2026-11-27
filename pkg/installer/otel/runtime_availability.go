package otel

import (
	"fmt"
	"os/exec"
)

// Overridable in tests to simulate missing or broken runtime toolchains.
var (
	runtimeLookPath   = exec.LookPath
	runtimeFindPython = DetectPython
)

// runtimeAvailability describes whether a runtime's toolchain is usable here.
type runtimeAvailability struct {
	available bool
	reason    string // short form, e.g. "python3 not found on PATH"
	message   string // full line printed by the install flow when skipping
}

// checkRuntimeAvailability reports whether rt's binary is usable. Runtimes
// without a binary (empty binName) are always available.
func checkRuntimeAvailability(rt runtimeInfo) runtimeAvailability {
	if rt.binName == "" {
		return runtimeAvailability{available: true}
	}

	// Python's binary may exist as a non-functional stub even when no real
	// interpreter is installed, so run it to verify it's usable.
	if rt.binName == "python3" || rt.binName == "python" {
		if _, err := runtimeFindPython(); err == nil {
			return runtimeAvailability{available: true}
		}
		binPath, lookErr := runtimeLookPath(rt.binName)
		switch {
		case lookErr != nil:
			return notFoundOnPath(rt)
		case isWindowsStorePythonStub(binPath):
			return runtimeAvailability{
				reason:  fmt.Sprintf("%s is a Windows Store stub, not a real interpreter", rt.binName),
				message: fmt.Sprintf("  Skipping %s instrumentation — '%s' is a Windows Store stub, not a real interpreter.\n  Install Python 3 from https://python.org or disable it in Settings → Apps → Advanced app settings → App execution aliases.\n", rt.name, rt.binName),
			}
		default:
			return runtimeAvailability{
				reason:  fmt.Sprintf("%s found on PATH but not usable", rt.binName),
				message: fmt.Sprintf("  Skipping %s instrumentation — '%s' found on PATH but not usable.\n", rt.name, rt.binName),
			}
		}
	}

	if _, err := runtimeLookPath(rt.binName); err != nil {
		return notFoundOnPath(rt)
	}
	return runtimeAvailability{available: true}
}

func notFoundOnPath(rt runtimeInfo) runtimeAvailability {
	return runtimeAvailability{
		reason:  fmt.Sprintf("%s not found on PATH", rt.binName),
		message: fmt.Sprintf("  Skipping %s instrumentation — '%s' not found on PATH.\n", rt.name, rt.binName),
	}
}
