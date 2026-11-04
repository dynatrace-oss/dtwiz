package otel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// otelcInstallTarget is installed at @latest: there is deliberately no version
// constant, and no hard-coded Go gate (the minimum Go version is whatever the
// latest otelc declares).
const otelcInstallTarget = "go.opentelemetry.io/otelc/tool/cmd/otelc@latest"

// goRunner runs go tool commands for install and version lookups. A variable so
// tests can stub it.
var goRunner installer.CmdRunner = installer.RealRunner

// goBuildRunner runs a command in dir with the given environment and returns
// its combined output. A variable so tests can stub it.
var goBuildRunner = func(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// GoVersionTooOldError reports that the local Go toolchain cannot build the
// latest otelc.
type GoVersionTooOldError struct {
	Required string
	Cause    error
}

func (e *GoVersionTooOldError) Error() string {
	return fmt.Sprintf("upgrade Go to %s or newer for compile-time instrumentation", e.Required)
}

func (e *GoVersionTooOldError) Unwrap() error { return e.Cause }

var goRequiresRe = regexp.MustCompile(`requires go >= ([0-9][0-9A-Za-z.\-]*)`)

// classifyOtelcInstallError turns a "requires go >= X" failure into a
// GoVersionTooOldError; any other error is returned unchanged.
func classifyOtelcInstallError(err error) error {
	if err == nil {
		return nil
	}
	if m := goRequiresRe.FindStringSubmatch(err.Error()); m != nil {
		return &GoVersionTooOldError{Required: strings.TrimSuffix(m[1], "."), Cause: err}
	}
	return err
}

func otelcBinaryName() string {
	if runtime.GOOS == "windows" {
		return "otelc.exe"
	}
	return "otelc"
}

// isolatedOtelcDir is the GOBIN dtwiz installs otelc into, keeping the user's
// GOPATH/bin untouched.
func isolatedOtelcDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine home directory: %w", err)
	}
	return filepath.Join(home, ".opentelemetry", "go", "bin"), nil
}

// otelcTool is a usable otelc executable.
type otelcTool struct {
	Path    string
	Version string
	// Isolated is true when dtwiz owns the binary (installed in the isolated
	// directory) as opposed to the user's own copy found on PATH.
	Isolated bool
}

// findOtelc returns an otelc from PATH (the user's own) or from the isolated
// directory, without installing anything.
func findOtelc() (*otelcTool, bool) {
	if path, err := installer.ExecLookPath("otelc"); err == nil {
		return &otelcTool{Path: path, Version: otelcVersion(path)}, true
	}
	if dir, err := isolatedOtelcDir(); err == nil {
		candidate := filepath.Join(dir, otelcBinaryName())
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return &otelcTool{Path: candidate, Version: otelcVersion(candidate), Isolated: true}, true
		}
	}
	return nil, false
}

// ensureOtelc returns a usable otelc, installing the latest one into the
// isolated directory when none exists. A PATH copy always wins.
func ensureOtelc() (*otelcTool, error) {
	if tool, ok := findOtelc(); ok {
		logger.Debug("using existing otelc", "path", tool.Path, "version", tool.Version)
		return tool, nil
	}
	dir, err := isolatedOtelcDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	env := append(os.Environ(), "GOBIN="+dir, "GOTOOLCHAIN=local")
	if _, err := goRunner("go", []string{"install", otelcInstallTarget}, env); err != nil {
		return nil, fmt.Errorf("install otelc on %s/%s: %w", runtime.GOOS, runtime.GOARCH, classifyOtelcInstallError(err))
	}
	path := filepath.Join(dir, otelcBinaryName())
	tool := &otelcTool{Path: path, Version: otelcVersion(path), Isolated: true}
	logger.Debug("installed otelc", "path", path, "version", tool.Version)
	return tool, nil
}

// otelcVersion reads the module version embedded in the otelc binary
// (`go version -m`). It returns "" when it cannot be determined.
func otelcVersion(path string) string {
	out, err := goRunner("go", []string{"version", "-m", path}, nil)
	if err != nil {
		logger.Debug("could not read otelc version", "path", path, "err", err)
		return ""
	}
	return parseOtelcVersion(out)
}

// parseOtelcVersion extracts the version of the otelc module from
// `go version -m` output ("\tmod\t<module>\t<version>").
func parseOtelcVersion(out string) string {
	var first string
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && f[0] == "mod" {
			if strings.Contains(f[1], "otelc") {
				return f[2]
			}
			if first == "" {
				first = f[2]
			}
		}
	}
	return first
}

// goBuildEnv returns the environment for builds: workspace mode is always
// off so an inherited GOWORK cannot interfere.
func goBuildEnv(extra ...string) []string {
	return append(append(os.Environ(), "GOWORK=off"), extra...)
}

var goNetworkErrorMarkers = []string{
	"dial tcp", "no such host", "i/o timeout", "tls handshake", "connection refused",
	"connection reset", "proxy.golang.org", "network is unreachable", "temporary failure in name resolution",
}

func isGoNetworkError(output string) bool {
	lower := strings.ToLower(output)
	for _, m := range goNetworkErrorMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// buildWithOtelc builds one service with compile-time instrumentation. The
// module's go.mod/go.sum are protected for the duration of the build. The binary
// is written to binPath. workDir is otelc's work directory; the caller owns it
// and may share it between services of one module. When empty, a private
// temporary directory is used and removed afterwards.
func buildWithOtelc(ctx context.Context, tool *otelcTool, modulePath, workDir string, svc GoService, binPath string) error {
	if workDir == "" {
		dir, err := os.MkdirTemp("", "dtwiz-otelc-*")
		if err != nil {
			return fmt.Errorf("create otelc work directory: %w", err)
		}
		defer os.RemoveAll(dir)
		workDir = dir
	}

	if err := os.MkdirAll(filepath.Dir(binPath), 0755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(binPath), err)
	}
	pkg := "./" + svc.RelDir
	if svc.RelDir == "." {
		pkg = "."
	}
	args := []string{"--work-dir", workDir, "go", "build", "-o", binPath, pkg}

	return withModGuard(modulePath, func() error {
		out, err := goBuildRunner(ctx, modulePath, goBuildEnv(), tool.Path, args...)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return fmt.Errorf("otelc build of %s cancelled: %w", svc.Name, ctx.Err())
			}
			msg := fmt.Sprintf("otelc build of %s failed on %s/%s: %v", svc.Name, runtime.GOOS, runtime.GOARCH, err)
			if isGoNetworkError(out) {
				msg += " (network error: the first build downloads modules, check your connection or GOPROXY)"
			}
			logger.Debug("otelc build output", "service", svc.Name, "output", out)
			return fmt.Errorf("%s: %s", msg, lastLines(out, 8))
		}
		return nil
	})
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
