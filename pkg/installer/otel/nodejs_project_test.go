package otel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/testutil"
)

func TestDetectNodeProjects_Found(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
		t.Fatal(err)
	}

	testutil.SetTestWorkingDir(t, dir)
	projects := detectNodeProjects()
	found := false
	for _, p := range projects {
		if p.Path == dir || p.Path == realDir {
			found = true
		}
	}
	if !found {
		t.Errorf("expected project at %s, got %v", dir, projects)
	}
}

func TestDetectNodeProjects_ExcludesNodeModules(t *testing.T) {
	dir := t.TempDir()
	// Create node_modules subdirectory with a package.json inside.
	nmDir := filepath.Join(dir, "node_modules", "somelib")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	// Create the real project package.json.
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
		t.Fatal(err)
	}

	testutil.SetTestWorkingDir(t, dir)
	projects := detectNodeProjects()
	for _, p := range projects {
		if filepath.Base(filepath.Dir(p.Path)) == "node_modules" {
			t.Errorf("node_modules project should be excluded, found: %s", p.Path)
		}
	}
}

// --- Task 1.1a: isNextJSProject tests ---

func TestIsNextJSProject_ConfigFile(t *testing.T) {
	for _, configFile := range []string{"next.config.js", "next.config.ts", "next.config.mjs"} {
		t.Run(configFile, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, configFile), []byte(""), 0644); err != nil {
				t.Fatal(err)
			}
			if !isNextJSProject(dir) {
				t.Errorf("expected isNextJSProject=true for %s", configFile)
			}
		})
	}
}

func TestIsNextJSProject_PackageDep(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"14.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if !isNextJSProject(dir) {
		t.Error("expected isNextJSProject=true for next in dependencies")
	}
}

func TestIsNextJSProject_DevDep(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"devDependencies":{"next":"14.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if !isNextJSProject(dir) {
		t.Error("expected isNextJSProject=true for next in devDependencies")
	}
}

func TestIsNextJSProject_NotNextJS(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"express":"4.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if isNextJSProject(dir) {
		t.Error("expected isNextJSProject=false for non-Next.js project")
	}
}

// --- Task 1.1b: detectNodeFramework tests ---

func TestDetectNodeFramework_NuxtConfigFile(t *testing.T) {
	for _, configFile := range []string{"nuxt.config.js", "nuxt.config.ts", "nuxt.config.mjs"} {
		t.Run(configFile, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, configFile), []byte(""), 0644); err != nil {
				t.Fatal(err)
			}
			if got := detectNodeFramework(dir); got != "nuxt" {
				t.Errorf("detectNodeFramework() = %q, want %q for %s", got, "nuxt", configFile)
			}
		})
	}
}

func TestDetectNodeFramework_NuxtDep(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"nuxt":"3.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodeFramework(dir); got != "nuxt" {
		t.Errorf("detectNodeFramework() = %q, want %q", got, "nuxt")
	}
}

func TestDetectNodeFramework_NextTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"14.0.0","nuxt":"3.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodeFramework(dir); got != "next" {
		t.Errorf("detectNodeFramework() = %q, want %q (Next.js takes precedence)", got, "next")
	}
}

func TestDetectNodeFramework_Regular(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"express":"4.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodeFramework(dir); got != "" {
		t.Errorf("detectNodeFramework() = %q, want empty string for regular project", got)
	}
}

// --- Task 1.2: detectNodePackageManager tests ---

func TestDetectNodePackageManager_NPM(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodePackageManager(dir); got != "npm" {
		t.Errorf("detectNodePackageManager() = %q, want %q", got, "npm")
	}
}

func TestDetectNodePackageManager_Yarn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodePackageManager(dir); got != "yarn" {
		t.Errorf("detectNodePackageManager() = %q, want %q", got, "yarn")
	}
}

func TestDetectNodePackageManager_PNPM(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodePackageManager(dir); got != "pnpm" {
		t.Errorf("detectNodePackageManager() = %q, want %q", got, "pnpm")
	}
}

func TestDetectNodePackageManager_Default(t *testing.T) {
	dir := t.TempDir()
	if got := detectNodePackageManager(dir); got != "npm" {
		t.Errorf("detectNodePackageManager() = %q, want %q (default)", got, "npm")
	}
}

// --- Task 1.3: Monorepo detection tests ---

func TestDetectNodeProjects_Monorepo(t *testing.T) {
	dir := t.TempDir()
	realDir, _ := filepath.EvalSymlinks(dir)

	// Root package.json with workspaces.
	rootPkg := `{"name":"monorepo","workspaces":["packages/*"]}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(rootPkg), 0644); err != nil {
		t.Fatal(err)
	}

	// Create workspace packages.
	for _, pkg := range []string{"api", "web"} {
		pkgDir := filepath.Join(dir, "packages", pkg)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{"name":"`+pkg+`"}`), 0644); err != nil {
			t.Fatal(err)
		}
	}

	testutil.SetTestWorkingDir(t, dir)
	projects := detectNodeProjects()

	// Should include the root and both workspace packages.
	paths := make(map[string]bool)
	for _, p := range projects {
		paths[p.Path] = true
	}

	// Check root is present.
	if !paths[dir] && !paths[realDir] {
		t.Errorf("expected monorepo root in projects, got %v", projects)
	}

	// Check workspace packages are present.
	for _, pkg := range []string{"api", "web"} {
		pkgPath := filepath.Join(dir, "packages", pkg)
		realPkgPath := filepath.Join(realDir, "packages", pkg)
		if !paths[pkgPath] && !paths[realPkgPath] {
			t.Errorf("expected workspace package %s in projects, got %v", pkg, projects)
		}
	}
}

func TestResolveWorkspaces_ArrayFormat(t *testing.T) {
	dir := t.TempDir()

	rootPkg := `{"workspaces":["packages/*"]}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(rootPkg), 0644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(dir, "packages", "lib")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	dirs := resolveWorkspaces(dir)
	if len(dirs) != 1 {
		t.Fatalf("expected 1 workspace dir, got %v", dirs)
	}
	if dirs[0] != pkgDir {
		t.Errorf("expected %q, got %q", pkgDir, dirs[0])
	}
}

func TestResolveWorkspaces_ObjectFormat(t *testing.T) {
	dir := t.TempDir()

	rootPkg := `{"workspaces":{"packages":["packages/*"]}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(rootPkg), 0644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(dir, "packages", "lib")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	dirs := resolveWorkspaces(dir)
	if len(dirs) != 1 {
		t.Fatalf("expected 1 workspace dir, got %v", dirs)
	}
	if dirs[0] != pkgDir {
		t.Errorf("expected %q, got %q", pkgDir, dirs[0])
	}
}

func TestResolveWorkspaces_NoWorkspaces(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
		t.Fatal(err)
	}

	dirs := resolveWorkspaces(dir)
	if len(dirs) != 0 {
		t.Errorf("expected no workspaces, got %v", dirs)
	}
}

func TestResolveWorkspaces_SkipsDirWithoutPackageJSON(t *testing.T) {
	dir := t.TempDir()

	rootPkg := `{"workspaces":["packages/*"]}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(rootPkg), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a workspace dir without package.json.
	emptyDir := filepath.Join(dir, "packages", "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}

	dirs := resolveWorkspaces(dir)
	if len(dirs) != 0 {
		t.Errorf("expected no workspaces (no package.json), got %v", dirs)
	}
}
