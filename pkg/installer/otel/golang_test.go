package otel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

func TestDetectGoProjects_Found(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)
	goMod := "module github.com/example/myapp\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	helpers.SetTestWorkingDir(t, dir)
	projects := detectGoProjects(defaultScanRoots())
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
			if p.ModuleName != "github.com/example/myapp" {
				t.Errorf("expected module name github.com/example/myapp, got %q", p.ModuleName)
			}
		}
	}
	if !found {
		t.Errorf("expected Go project at %s, got %v", dir, projects)
	}
}

func TestDetectGoProjects_None(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)

	helpers.SetTestWorkingDir(t, dir)
	projects := detectGoProjects(defaultScanRoots())
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			t.Errorf("unexpected Go project at %s with no go.mod", dir)
		}
	}
}

func TestExtractGoModuleName(t *testing.T) {
	dir := t.TempDir()
	content := "module github.com/acme/service\n\ngo 1.22\n"
	goModPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := extractGoModuleName(goModPath)
	if got != "github.com/acme/service" {
		t.Errorf("expected github.com/acme/service, got %q", got)
	}
}

func TestExtractGoModuleName_MissingOrWithoutModule(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		got := extractGoModuleName(filepath.Join(t.TempDir(), "go.mod"))
		if got != "" {
			t.Fatalf("expected empty module name, got %q", got)
		}
	})

	t.Run("no module line", func(t *testing.T) {
		dir := t.TempDir()
		goModPath := filepath.Join(dir, "go.mod")
		if err := os.WriteFile(goModPath, []byte("go 1.22\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := extractGoModuleName(goModPath)
		if got != "" {
			t.Fatalf("expected empty module name, got %q", got)
		}
	})
}

func TestDetectGoPlan_NoGoOnPath(t *testing.T) {
	t.Setenv("PATH", "")

	plan := DetectGoPlan("http://127.0.0.1:4318")
	if plan != nil {
		t.Fatalf("expected nil plan, got %#v", plan)
	}
}

func TestDetectGoPlan_FindsProject(t *testing.T) {
	dir := t.TempDir()
	goMod := "module github.com/example/myapp\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	writeGoFiles(t, dir, map[string]string{"main.go": "package main\n"})

	helpers.SetTestWorkingDir(t, dir)
	setTestStdin(t, "1\n")

	plan := DetectGoPlan("http://127.0.0.1:4318")
	if plan == nil {
		t.Fatal("expected Go plan")
	}
	if plan.Project.ModuleName != "github.com/example/myapp" {
		t.Fatalf("ModuleName = %q, want %q", plan.Project.ModuleName, "github.com/example/myapp")
	}
}

func TestGoInstrumentationPlan_Runtime(t *testing.T) {
	plan := &GoInstrumentationPlan{}
	if got := plan.Runtime(); got != "Go" {
		t.Fatalf("Runtime() = %q, want %q", got, "Go")
	}
}

func TestGoInstrumentationPlan_PrintPlanSteps(t *testing.T) {
	plan := &GoInstrumentationPlan{
		Project:  GoProject{ScannedProject: ScannedProject{Path: "/tmp/go-svc"}, ModuleName: "github.com/example/go-svc"},
		Packages: []goMainPackage{{RelDir: ".", ServiceName: "go-svc"}, {RelDir: "cmd/worker", ServiceName: "worker"}},
	}

	output := helpers.CaptureStdout(t, func() {
		plan.PrintPlanSteps()
	})

	checks := []string{"/tmp/go-svc", "github.com/example/go-svc", "go.opentelemetry.io/otel/sdk", "./cmd/worker", "service worker"}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}

func writeGoFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDetectGoMainPackages(t *testing.T) {
	dir := t.TempDir()
	writeGoFiles(t, dir, map[string]string{
		"main.go":                       "package main\n",
		"cmd/api/main.go":               "package main\n",
		"cmd/lib/lib.go":                "package lib\n",
		"cmd/tool/tool_test.go":         "package main\n",
		"services/billing/main.go":      "package main\n",
		"services/a/cmd/server/main.go": "package main\n",
		"services/b/cmd/server/main.go": "package main\n",
		"vendor/x/main.go":              "package main\n",
		"nested/go.mod":                 "module nested\n",
		"nested/main.go":                "package main\n",
	})

	got := detectGoMainPackages(dir, "root-svc")
	want := []goMainPackage{
		{RelDir: ".", ServiceName: "root-svc"},
		{RelDir: "cmd/api", ServiceName: "api"},
		{RelDir: "services/a/cmd/server", ServiceName: "services-a-cmd-server"},
		{RelDir: "services/b/cmd/server", ServiceName: "services-b-cmd-server"},
		{RelDir: "services/billing", ServiceName: "billing"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i].RelDir != want[i].RelDir || got[i].ServiceName != want[i].ServiceName {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestBuildGoInstrumentationPlan_NoMainPackage(t *testing.T) {
	dir := t.TempDir()
	writeGoFiles(t, dir, map[string]string{"lib.go": "package lib\n"})
	var plan *GoInstrumentationPlan
	helpers.CaptureStdout(t, func() {
		plan = buildGoInstrumentationPlan(GoProject{ScannedProject: ScannedProject{Path: dir}}, "http://127.0.0.1:4318")
	})
	if plan != nil {
		t.Fatalf("expected nil plan, got %#v", plan)
	}
}

func TestGoInstrumentationPlan_PrepareAndSharedFiles(t *testing.T) {
	dir := t.TempDir()
	writeGoFiles(t, dir, map[string]string{
		"go.mod":              "module example.com/app\n\ngo 1.21\n",
		"go.sum":              "x\n",
		"main.go":             "package main\n",
		"svc/go.mod":          "module example.com/svc\n",
		"svc/cmd/api/main.go": "package main\n",
	})
	if err := prepareGoOtelDir(dir); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"go.mod", "go.sum"} {
		if _, err := os.Stat(filepath.Join(dir, goOtelDirName, f)); err != nil {
			t.Fatalf("expected %s: %v", f, err)
		}
	}

	pkgs := []goMainPackage{
		{RelDir: ".", ModuleDir: dir, PkgDir: ".", ServiceName: "app"},
		{RelDir: "svc/cmd/api", ModuleDir: filepath.Join(dir, "svc"), PkgDir: "cmd/api", ServiceName: "api"},
	}
	overlayPath, err := writeGoSharedFiles(dir, pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if overlayPath != filepath.Join(dir, goOtelDirName, "overlay.json") {
		t.Fatalf("unexpected overlay path %s", overlayPath)
	}
	if _, err := os.Stat(filepath.Join(dir, goOtelDirName, goInitFileName)); err != nil {
		t.Fatalf("expected shared init file: %v", err)
	}
	data, _ := os.ReadFile(overlayPath)
	var overlay struct{ Replace map[string]string }
	if err := json.Unmarshal(data, &overlay); err != nil {
		t.Fatal(err)
	}
	sharedInit := filepath.Join(dir, goOtelDirName, goInitFileName)
	for _, dest := range []string{
		filepath.Join(dir, goInitFileName),
		filepath.Join(dir, "svc", "cmd", "api", goInitFileName),
	} {
		if overlay.Replace[dest] != sharedInit {
			t.Fatalf("overlay missing %s → %s, got %v", dest, sharedInit, overlay.Replace)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, goInitFileName)); err == nil {
		t.Fatal("init file must not be written into the project root")
	}
}

func TestDetectGoMainPackages_Workspace(t *testing.T) {
	dir := t.TempDir()
	writeGoFiles(t, dir, map[string]string{
		"go.mod":                   "module example.com/root\n",
		"doc.go":                   "package root\n",
		"go.work":                  "go 1.24\n\nuse (\n\t.\n\t./orders // comment\n\t./payments\n)\nuse ./extra\n",
		"orders/go.mod":            "module example.com/orders\n",
		"orders/main.go":           "package main\n",
		"payments/go.mod":          "module example.com/payments\n",
		"payments/cmd/pay/main.go": "package main\n",
		"extra/go.mod":             "module example.com/extra\n",
		"extra/main.go":            "package main\n",
		"other/go.mod":             "module example.com/other\n",
		"other/main.go":            "package main\n",
	})

	got := detectGoMainPackages(dir, "root")
	want := map[string]goMainPackage{
		"orders":           {RelDir: "orders", ModuleDir: filepath.Join(dir, "orders"), PkgDir: ".", ServiceName: "orders"},
		"payments/cmd/pay": {RelDir: "payments/cmd/pay", ModuleDir: filepath.Join(dir, "payments"), PkgDir: "cmd/pay", ServiceName: "pay"},
		"extra":            {RelDir: "extra", ModuleDir: filepath.Join(dir, "extra"), PkgDir: ".", ServiceName: "extra"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, p := range got {
		if w, ok := want[p.RelDir]; !ok || w != p {
			t.Fatalf("unexpected package %v (want %v)", p, w)
		}
	}
}

func TestGoModuleUsesOtelSDK(t *testing.T) {
	cases := map[string]struct {
		goMod string
		want  bool
	}{
		"sdk direct":     {"module a\n\nrequire (\n\tgo.opentelemetry.io/otel/sdk v1.34.0\n)\n", true},
		"exporter":       {"module a\n\nrequire go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.34.0\n", true},
		"sdk indirect":   {"module a\n\nrequire (\n\tgo.opentelemetry.io/otel/sdk v1.34.0 // indirect\n)\n", false},
		"api only":       {"module a\n\nrequire (\n\tgo.opentelemetry.io/otel v1.34.0\n)\n", false},
		"no otel at all": {"module a\n\ngo 1.24\n", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeGoFiles(t, dir, map[string]string{"go.mod": c.goMod})
			if got := goModuleUsesOtelSDK(dir); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestBuildGoInstrumentationPlan_SkipsAlreadyInstrumented(t *testing.T) {
	dir := t.TempDir()
	writeGoFiles(t, dir, map[string]string{
		"go.mod":  "module a\n\nrequire go.opentelemetry.io/otel/sdk v1.34.0\n",
		"main.go": "package main\n",
	})
	var plan *GoInstrumentationPlan
	out := helpers.CaptureStdout(t, func() {
		plan = buildGoInstrumentationPlan(GoProject{ScannedProject: ScannedProject{Path: dir}}, "http://127.0.0.1:4318")
	})
	if plan != nil || !strings.Contains(out, "already") {
		t.Fatalf("expected nil plan with explanation, got plan=%v out=%q", plan, out)
	}
}
