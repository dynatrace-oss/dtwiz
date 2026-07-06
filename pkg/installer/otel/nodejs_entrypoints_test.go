package otel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectNodeEntrypoints_Main(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"main":"server.js"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "server.js" {
		t.Errorf("expected [server.js], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_ScriptsStart(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"start":"node app.js"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "app.js" {
		t.Errorf("expected [app.js], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_Fallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "index.js" {
		t.Errorf("expected [index.js], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_TypeScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	// Only a TypeScript variant exists.
	if err := os.WriteFile(filepath.Join(dir, "app.ts"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) == 0 || eps[0] != "app.ts" {
		t.Errorf("expected [app.ts], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_None(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) != 0 {
		t.Errorf("expected empty entrypoints, got %v", eps)
	}
}

// --- Task 1.4/1.5: detectNodeEntrypoints for Next.js / Nuxt ---

func TestDetectNodeEntrypoints_NextJS(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"14.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) != 1 || eps[0] != "next:start" {
		t.Errorf("expected [next:start], got %v", eps)
	}
}

func TestDetectNodeEntrypoints_Nuxt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"nuxt":"3.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) != 1 || eps[0] != "nuxt:start" {
		t.Errorf("expected [nuxt:start], got %v", eps)
	}
}

// --- Other scripts entrypoint detection ---

func TestDetectNodeEntrypoints_OtherScripts(t *testing.T) {
	dir := t.TempDir()
	// scripts.start uses npm-run-all (no direct file reference),
	// but sub-scripts reference actual files.
	pkgJSON := `{
		"scripts": {
			"start": "npm-run-all --parallel start:frontend start:order start:delivery",
			"start:frontend": "node s-frontend/app.js",
			"start:order": "node s-order/app.js",
			"start:delivery": "node s-delivery/app.js"
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Create the entrypoint files.
	for _, sub := range []string{"s-frontend", "s-order", "s-delivery"} {
		subDir := filepath.Join(dir, sub)
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subDir, "app.js"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) != 3 {
		t.Fatalf("expected 3 entrypoints, got %v", eps)
	}

	// Should be sorted.
	want := []string{"s-delivery/app.js", "s-frontend/app.js", "s-order/app.js"}
	for i, ep := range eps {
		if ep != want[i] {
			t.Errorf("entrypoints[%d] = %q, want %q", i, ep, want[i])
		}
	}
}

func TestDetectNodeEntrypoints_OtherScripts_SkipsMissing(t *testing.T) {
	dir := t.TempDir()
	// Script references a file that doesn't exist on disk.
	pkgJSON := `{
		"scripts": {
			"start": "npm-run-all --parallel start:api",
			"start:api": "node src/api.js"
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) != 0 {
		t.Errorf("expected 0 entrypoints (file missing), got %v", eps)
	}
}

func TestDetectNodeEntrypoints_IgnoresNonRuntimeScripts(t *testing.T) {
	dir := t.TempDir()
	// Only non-runtime scripts reference existing files.
	// These should NOT be treated as entrypoints.
	pkgJSON := `{
		"scripts": {
			"lint": "eslint --config eslint.config.js src/",
			"build": "esbuild build.config.ts",
			"test": "jest jest.config.js",
			"typecheck": "tsc --project tsconfig.json"
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"eslint.config.js", "build.config.ts", "jest.config.js", "tsconfig.json"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) != 0 {
		t.Errorf("expected 0 entrypoints (non-runtime scripts), got %v", eps)
	}
}

func TestDetectNodeEntrypoints_RuntimeScriptPrefixes(t *testing.T) {
	dir := t.TempDir()
	// All of these script names should be scanned for entrypoints.
	pkgJSON := `{
		"scripts": {
			"dev": "node dev-server.js",
			"serve:api": "node api.js",
			"server:worker": "node worker.js"
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"dev-server.js", "api.js", "worker.js"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	eps := detectNodeEntrypoints(dir)
	if len(eps) != 3 {
		t.Fatalf("expected 3 entrypoints, got %v", eps)
	}
}

func TestIsRuntimeScript(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"start", true},
		{"start:api", true},
		{"start:frontend", true},
		{"dev", true},
		{"dev:watch", true},
		{"serve", true},
		{"serve:api", true},
		{"server", true},
		{"server:worker", true},
		{"build", false},
		{"test", false},
		{"lint", false},
		{"typecheck", false},
		{"format", false},
		{"prestart", false},
		{"postinstall", false},
		{"starting", false},
		{"developer", false},
	}
	for _, tc := range cases {
		if got := isRuntimeScript(tc.name); got != tc.want {
			t.Errorf("isRuntimeScript(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
