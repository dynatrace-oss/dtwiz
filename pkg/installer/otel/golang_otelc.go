package otel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"syscall"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// Compile-time instrumentation via otelc (https://opentelemetry.io/docs/zero-code/go/compile-time/).
// `otelc go build` wraps the Go toolchain through -toolexec, injects hooks into net/http,
// gRPC, database/sql, ... (including third-party dependencies) and initializes the OTel SDK
// from the OTEL_* environment variables. This yields server spans and connected traces
// without touching project sources.

const (
	otelcInstallPath = "go.opentelemetry.io/otelc/tool/cmd/otelc@latest"
	otelcMinGoMajor  = 1
	otelcMinGoMinor  = 25
)

var goVersionRe = regexp.MustCompile(`go(\d+)\.(\d+)`)

// goVersionSupportsOtelc reports whether `go version` output names Go >= 1.25.
func goVersionSupportsOtelc(versionOutput string) bool {
	m := goVersionRe.FindStringSubmatch(versionOutput)
	if m == nil {
		return false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return major > otelcMinGoMajor || (major == otelcMinGoMajor && minor >= otelcMinGoMinor)
}

func goToolchainSupportsOtelc() bool {
	out, err := exec.Command("go", "version").Output()
	if err != nil {
		return false
	}
	return goVersionSupportsOtelc(string(out))
}

func otelcBinaryName() string {
	if runtime.GOOS == "windows" {
		return "otelc.exe"
	}
	return "otelc"
}

// otelcInstallDir is the isolated GOBIN for otelc, so it never lands in the user's GOPATH/bin.
func otelcInstallDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".opentelemetry", "go", "bin"), nil
}

// ensureOtelc returns the path of an otelc binary: one already on PATH, the isolated copy
// from a previous run, or a fresh `go install` into the isolated directory.
func ensureOtelc() (string, error) {
	if p, err := exec.LookPath("otelc"); err == nil {
		logger.Debug("using otelc from PATH", "path", p)
		return p, nil
	}
	binDir, err := otelcInstallDir()
	if err != nil {
		return "", err
	}
	bin := filepath.Join(binDir, otelcBinaryName())
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}
	cmd := exec.Command("go", "install", otelcInstallPath)
	cmd.Env = append(os.Environ(), "GOBIN="+binDir)
	if err := runGo(cmd); err != nil {
		return "", fmt.Errorf("installing otelc (%s/%s): %w", runtime.GOOS, runtime.GOARCH, err)
	}
	return bin, nil
}

// otelcBuildCmd builds `otelc go build` for one main package. otelc rejects -modfile, so the
// build runs against the project's own go.mod; the caller restores go.mod/go.sum afterwards.
func otelcBuildCmd(ctx context.Context, otelcBin string, mp goMainPackage, binPath, workDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, otelcBin, "--work-dir", workDir, "go", "build", "-o", binPath, "./"+mp.PkgDir)
	cmd.Dir = mp.ModuleDir
	// GOWORK=off: otelc raises the module's go directive to its own minimum, which a go.work
	// pinning an older version rejects ("module . listed in go.work file requires go >= ...").
	cmd.Env = append(os.Environ(), "GOWORK=off")
	return cmd
}

// buildWithOtelc builds the package with otelc and always puts the project's go.mod/go.sum back,
// including when the user presses Ctrl+C or dtwiz receives SIGTERM during the build. A failed
// restore is reported as an error, never swallowed.
func buildWithOtelc(otelcBin string, mp goMainPackage, binPath string) (err error) {
	guard, err := newModGuard(mp.ModuleDir)
	if err != nil {
		return err
	}
	// Catch signals so the process survives until the restore below has run; the signal also
	// cancels ctx, which kills otelc.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer func() {
		if rerr := guard.Restore(); rerr != nil {
			err = errors.Join(err, fmt.Errorf("could not restore go.mod/go.sum: %w", rerr))
		}
	}()

	// otelc's scratch space (hundreds of generated files) lives in a temp dir, not the project.
	workDir, err := os.MkdirTemp("", "dtwiz-otelc-")
	if err != nil {
		return fmt.Errorf("creating otelc work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	buildErr := runGo(otelcBuildCmd(ctx, otelcBin, mp, binPath, workDir))
	if ctx.Err() != nil {
		return fmt.Errorf("build interrupted (go.mod and go.sum are being restored): %w", ctx.Err())
	}
	return buildErr
}
