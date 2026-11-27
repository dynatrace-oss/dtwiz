package dotnet

import (
	"fmt"
	"strings"
)

// archivePrefix is the common prefix of the upstream release archive names.
const archivePrefix = "opentelemetry-dotnet-instrumentation-"

// SupportedPlatforms names the platforms the upstream release provides an
// archive for, for use in messages.
const SupportedPlatforms = "Linux (glibc or musl, x64 or arm64), macOS (Apple silicon) and Windows (x64)"

// Archive is the upstream release archive for one platform.
type Archive struct {
	// Name is the release asset file name.
	Name string
	// NativeDirs are the top-level directories of the extracted archive that
	// hold the native profiler library; every one of them must exist.
	NativeDirs []string
	// NativeFile is the file name of the native profiler library inside each of
	// NativeDirs.
	NativeFile string
}

// UnsupportedPlatformError says that the upstream release has no archive for
// the machine.
type UnsupportedPlatformError struct {
	OS, Arch, Libc string
	Reason         string
}

func (e *UnsupportedPlatformError) Error() string {
	return fmt.Sprintf("%s is not supported by the upstream OpenTelemetry .NET release (supported: %s)", e.Reason, SupportedPlatforms)
}

// SelectArchive returns the release archive for the operating system (a GOOS
// value), architecture (a GOARCH value) and, on Linux, the C library ("glibc"
// or "musl"; "" when unknown). It fails with *UnsupportedPlatformError for
// combinations the upstream release does not provide.
func SelectArchive(goos, goarch, libc string) (Archive, error) {
	unsupported := func(reason string) (Archive, error) {
		return Archive{}, &UnsupportedPlatformError{OS: goos, Arch: goarch, Libc: libc, Reason: reason}
	}

	switch goos {
	case "linux":
		arch, ok := linuxArch(goarch)
		if !ok {
			return unsupported("Linux on " + goarch)
		}
		switch libc {
		case "glibc":
			return Archive{
				Name:       archivePrefix + "linux-glibc-" + arch + ".zip",
				NativeDirs: []string{"linux-" + arch},
				NativeFile: "OpenTelemetry.AutoInstrumentation.Native.so",
			}, nil
		case "musl":
			return Archive{
				Name:       archivePrefix + "linux-musl-" + arch + ".zip",
				NativeDirs: []string{"linux-musl-" + arch},
				NativeFile: "OpenTelemetry.AutoInstrumentation.Native.so",
			}, nil
		}
		return unsupported("Linux with an unrecognized C library")
	case "darwin":
		if goarch != "arm64" {
			return unsupported("macOS on " + goarch + " (Intel)")
		}
		return Archive{
			Name:       archivePrefix + "macos.zip",
			NativeDirs: []string{"osx-arm64"},
			NativeFile: "OpenTelemetry.AutoInstrumentation.Native.dylib",
		}, nil
	case "windows":
		if goarch != "amd64" {
			return unsupported("Windows on " + goarch)
		}
		return Archive{
			Name:       archivePrefix + "windows.zip",
			NativeDirs: []string{"win-x64", "win-x86"},
			NativeFile: "OpenTelemetry.AutoInstrumentation.Native.dll",
		}, nil
	}
	return unsupported(goos)
}

func linuxArch(goarch string) (string, bool) {
	switch goarch {
	case "amd64":
		return "x64", true
	case "arm64":
		return "arm64", true
	}
	return "", false
}

// CommandRunner runs a command and returns its combined output; it is the seam
// for external commands (ldd, gh) in tests.
type CommandRunner func(name string, args ...string) (string, error)

// DetectLibc returns "glibc" or "musl" for the Linux C library, judged from
// the libraries the system's own `ls` links against (the method the upstream
// installer uses), or "" when it cannot be told. The runner is a seam for tests.
func DetectLibc(run CommandRunner) string {
	// ldd exits non-zero on musl systems, so the output is read regardless of err.
	out, _ := run("ldd", "/bin/ls")
	lower := strings.ToLower(out)
	switch {
	case strings.Contains(lower, "musl"):
		return "musl"
	case strings.Contains(lower, "libc.so.6"), strings.Contains(lower, "glibc"):
		return "glibc"
	}
	return ""
}
