package rum

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDetect(t *testing.T) {
	type testCase struct {
		name       string
		setup      func(t *testing.T, dir string)
		wantMode   InjectionMode
		wantReason string
		wantFiles  []string // relative paths; nil means don't check InjectableFiles
	}

	tests := []testCase{
		{
			name: "static HTML project: one html in root",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "index.html", "")
			},
			wantMode:  ModeAuto,
			wantFiles: []string{"index.html"},
		},
		{
			name: "Next.js via config file",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "next.config.js", "")
			},
			wantMode:   ModeManual,
			wantReason: "Next.js",
		},
		{
			name: "Next.js via dependency in package.json",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "package.json", `{"dependencies":{"next":"14.0.0"}}`)
			},
			wantMode:   ModeManual,
			wantReason: "Next.js",
		},
		{
			name: "Nuxt via config file",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "nuxt.config.js", "")
			},
			wantMode:   ModeManual,
			wantReason: "Nuxt",
		},
		{
			name: "Nuxt via dependency in package.json",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "package.json", `{"dependencies":{"nuxt":"3.0.0"}}`)
			},
			wantMode:   ModeManual,
			wantReason: "Nuxt",
		},
		{
			name: "malformed package.json: framework not detected, continues to HTML scan",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "package.json", `{not valid json}`)
				writeFile(t, dir, "index.html", "")
			},
			wantMode:  ModeAuto,
			wantFiles: []string{"index.html"},
		},
		{
			name: "CRA-style: public/index.html, no framework",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "public/index.html", "")
			},
			wantMode:  ModeAuto,
			wantFiles: []string{"public/index.html"},
		},
		{
			name: "HTML under dist/ excluded; root html is injectable",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "dist/about.html", "")
				writeFile(t, dir, "index.html", "")
			},
			wantMode:  ModeAuto,
			wantFiles: []string{"index.html"},
		},
		{
			name: "HTML under node_modules/ excluded",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "node_modules/pkg/index.html", "")
			},
			wantMode:   ModeManual,
			wantReason: "no writable HTML files found",
		},
		{
			name: "HTML under build/ excluded",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "build/index.html", "")
			},
			wantMode:   ModeManual,
			wantReason: "no writable HTML files found",
		},
		{
			name: "no HTML files at all",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "README.md", "")
			},
			wantMode:   ModeManual,
			wantReason: "no writable HTML files found",
		},
		{
			name: "index.html alongside other html files: only index.html returned",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "index.html", "")
				writeFile(t, dir, "about.html", "")
				writeFile(t, dir, "contact.html", "")
			},
			wantMode:  ModeAuto,
			wantFiles: []string{"index.html"},
		},
		{
			name: "Angular-style: index.html plus component templates",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "index.html", "")
				writeFile(t, dir, "src/app/app.component.html", "")
				writeFile(t, dir, "src/app/header.component.html", "")
			},
			wantMode:  ModeAuto,
			wantFiles: []string{"index.html"},
		},
		{
			name: "no index.html: multiple html files all returned sorted",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "contact.html", "")
				writeFile(t, dir, "about.html", "")
			},
			wantMode:  ModeAuto,
			wantFiles: []string{"about.html", "contact.html"},
		},
		{
			name: "mixed: framework plus html files, framework takes precedence",
			setup: func(t *testing.T, dir string) {
				writeFile(t, dir, "next.config.js", "")
				writeFile(t, dir, "index.html", "")
			},
			wantMode:   ModeManual,
			wantReason: "Next.js",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.setup(t, dir)

			result, err := Detect(dir)
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if result.Mode != tc.wantMode {
				t.Errorf("Mode = %q, want %q", result.Mode, tc.wantMode)
			}
			if tc.wantReason != "" && result.ManualReason != tc.wantReason {
				t.Errorf("ManualReason = %q, want %q", result.ManualReason, tc.wantReason)
			}
			if tc.wantFiles != nil {
				var wantAbs []string
				for _, rel := range tc.wantFiles {
					wantAbs = append(wantAbs, filepath.Join(dir, filepath.FromSlash(rel)))
				}
				sort.Strings(wantAbs)
				got := make([]string, len(result.InjectableFiles))
				copy(got, result.InjectableFiles)
				sort.Strings(got)
				if len(got) != len(wantAbs) {
					t.Errorf("InjectableFiles = %v, want %v", got, wantAbs)
				} else {
					for i := range wantAbs {
						if got[i] != wantAbs[i] {
							t.Errorf("InjectableFiles[%d] = %q, want %q", i, got[i], wantAbs[i])
						}
					}
				}
			}
		})
	}
}
