package otel

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

//go:embed golang_init.go.tmpl
var goInitSource string

// goInitVirtualName is the name of the file the overlay adds to every main package.
const goInitVirtualName = "dtwiz_otel_init.go"

// goClientOnlyPackages are added to the isolated go.mod copy for the fallback.
var goClientOnlyPackages = []string{
	"go.opentelemetry.io/otel",
	"go.opentelemetry.io/otel/sdk",
	"go.opentelemetry.io/otel/sdk/metric",
	"go.opentelemetry.io/otel/sdk/log",
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp",
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp",
	"go.opentelemetry.io/contrib/bridges/otelslog",
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
	"go.opentelemetry.io/contrib/instrumentation/runtime",
}

// goFallback holds the generated files of the client-only build of one module.
type goFallback struct {
	modulePath string
	modFile    string // isolated copy of go.mod
	overlay    string // overlay.json
	depsReady  bool
}

// prepareGoFallback writes the isolated go.mod/go.sum copies, the init source
// and the overlay (one entry per main package) under <module>/.otel/go/. The
// project's own files are only read.
func prepareGoFallback(modulePath string, services []GoService) (*goFallback, error) {
	dir := goOtelDir(modulePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	f := &goFallback{modulePath: modulePath, modFile: filepath.Join(dir, "go.mod"), overlay: filepath.Join(dir, "overlay.json")}

	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(modulePath, name))
		if errors.Is(err, os.ErrNotExist) && name == "go.sum" {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			return nil, fmt.Errorf("write isolated %s: %w", name, err)
		}
	}

	initPath := filepath.Join(dir, "otel_init.go.src")
	if err := os.WriteFile(initPath, []byte(goInitSource), 0644); err != nil {
		return nil, fmt.Errorf("write init source: %w", err)
	}
	overlay, err := goOverlayJSON(services, initPath)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(f.overlay, overlay, 0644); err != nil {
		return nil, fmt.Errorf("write overlay: %w", err)
	}
	return f, nil
}

// goOverlayJSON builds the -overlay file mapping a virtual init file in every
// main package directory to the generated source. encoding/json escapes
// Windows path separators.
func goOverlayJSON(services []GoService, initPath string) ([]byte, error) {
	replace := make(map[string]string, len(services))
	for _, s := range services {
		replace[filepath.Join(s.Dir, goInitVirtualName)] = initPath
	}
	data, err := json.MarshalIndent(map[string]map[string]string{"Replace": replace}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode overlay: %w", err)
	}
	return data, nil
}

func (f *goFallback) env() []string {
	return goBuildEnv()
}

// ensureDeps adds the OTel packages to the isolated go.mod copy (once).
func (f *goFallback) ensureDeps(ctx context.Context) error {
	if f.depsReady {
		return nil
	}
	args := append([]string{"get", "-modfile=" + f.modFile}, goClientOnlyPackages...)
	out, err := goBuildRunner(ctx, f.modulePath, f.env(), "go", args...)
	if err != nil {
		return goBuildFailure("add OpenTelemetry packages", "client-only", out, err)
	}
	f.depsReady = true
	return nil
}

// buildClientOnly builds one service with the injected init and the isolated
// go.mod; the module's own go.mod/go.sum are not touched.
func (f *goFallback) buildClientOnly(ctx context.Context, svc GoService, binPath string) error {
	if err := f.ensureDeps(ctx); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(binPath), 0755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(binPath), err)
	}
	pkg := "./" + svc.RelDir
	if svc.RelDir == "." {
		pkg = "."
	}
	args := []string{"build", "-mod=mod", "-modfile=" + f.modFile, "-overlay=" + f.overlay, "-o", binPath, pkg}
	out, err := goBuildRunner(ctx, f.modulePath, f.env(), "go", args...)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("client-only build of %s cancelled: %w", svc.Name, ctx.Err())
		}
		return goBuildFailure("build "+svc.Name, "client-only", out, err)
	}
	return nil
}

func goBuildFailure(what, mode, out string, err error) error {
	msg := fmt.Sprintf("%s (%s) failed on %s/%s: %v", what, mode, runtime.GOOS, runtime.GOARCH, err)
	if isGoNetworkError(out) {
		msg += " (network error: the first build downloads modules, check your connection or GOPROXY)"
	}
	logger.Debug("go build output", "step", what, "output", out)
	return fmt.Errorf("%s: %s", msg, lastLines(out, 8))
}
