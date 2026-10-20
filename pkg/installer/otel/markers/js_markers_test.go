package markers

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestIsNextJSProject_ConfigFile(t *testing.T) {
	for _, configFile := range []string{"next.config.js", "next.config.ts", "next.config.mjs"} {
		t.Run(configFile, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "package.json", `{"name":"app"}`)
			writeFile(t, dir, configFile, "")
			if !IsNextJSProject(dir) {
				t.Errorf("expected IsNextJSProject=true for %s", configFile)
			}
		})
	}
}

func TestIsNextJSProject_PackageDep(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"dependencies":{"next":"14.0.0"}}`)
	if !IsNextJSProject(dir) {
		t.Error("expected IsNextJSProject=true for next in dependencies")
	}
}

func TestIsNextJSProject_DevDep(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"devDependencies":{"next":"14.0.0"}}`)
	if !IsNextJSProject(dir) {
		t.Error("expected IsNextJSProject=true for next in devDependencies")
	}
}

func TestIsNextJSProject_NotNextJS(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"dependencies":{"express":"4.0.0"}}`)
	if IsNextJSProject(dir) {
		t.Error("expected IsNextJSProject=false for non-Next.js project")
	}
}

func TestIsNuxtProject_ConfigFile(t *testing.T) {
	for _, configFile := range []string{"nuxt.config.js", "nuxt.config.ts", "nuxt.config.mjs", "nuxt.config.mts"} {
		t.Run(configFile, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "package.json", `{"name":"app"}`)
			writeFile(t, dir, configFile, "")
			if !IsNuxtProject(dir) {
				t.Errorf("expected IsNuxtProject=true for %s", configFile)
			}
		})
	}
}

func TestIsNuxtProject_PackageDep(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"dependencies":{"nuxt":"3.0.0"}}`)
	if !IsNuxtProject(dir) {
		t.Error("expected IsNuxtProject=true for nuxt in dependencies")
	}
}

func TestIsNuxtProject_NotNuxt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"dependencies":{"express":"4.0.0"}}`)
	if IsNuxtProject(dir) {
		t.Error("expected IsNuxtProject=false for non-Nuxt project")
	}
}

func TestHasDependency(t *testing.T) {
	tests := []struct {
		name string
		json string // empty means no package.json
		want bool
	}{
		{"in dependencies", `{"dependencies":{"foo":"1"}}`, true},
		{"in devDependencies", `{"devDependencies":{"foo":"1"}}`, true},
		{"absent", `{"dependencies":{"bar":"1"}}`, false},
		{"malformed json", `{not valid`, false},
		{"no package.json", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.json != "" {
				writeFile(t, dir, "package.json", tt.json)
			}
			if got := HasDependency(dir, "foo"); got != tt.want {
				t.Errorf("HasDependency=%v, want %v", got, tt.want)
			}
		})
	}
}
