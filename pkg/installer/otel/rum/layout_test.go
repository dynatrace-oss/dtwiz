package rum

import (
	"path/filepath"
	"testing"
)

func TestFindManualTarget(t *testing.T) {
	tests := []struct {
		name      string
		framework string
		files     []string
		want      string // relative; "" means no target
	}{
		{"app router js", "Next.js", []string{"app/layout.js"}, "app/layout.js"},
		{"app router tsx", "Next.js", []string{"app/layout.tsx"}, "app/layout.tsx"},
		{"app router jsx", "Next.js", []string{"app/layout.jsx"}, "app/layout.jsx"},
		{"app router ts", "Next.js", []string{"app/layout.ts"}, "app/layout.ts"},
		{"src app router", "Next.js", []string{"src/app/layout.js"}, "src/app/layout.js"},
		{"pages router document", "Next.js", []string{"pages/_document.js"}, "pages/_document.js"},
		{"src pages router document", "Next.js", []string{"src/pages/_document.tsx"}, "src/pages/_document.tsx"},
		{"app router wins over pages", "Next.js", []string{"pages/_document.js", "app/layout.js"}, "app/layout.js"},
		{"no layout file", "Next.js", []string{"README.md"}, ""},
		{"nuxt has no target", "Nuxt", []string{"app/layout.js"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				writeFile(t, dir, f, "")
			}
			got := findManualTarget(dir, tt.framework)
			want := ""
			if tt.want != "" {
				want = filepath.Join(dir, filepath.FromSlash(tt.want))
			}
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestDetect_NextJSReportsManualTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "next.config.js", "")
	writeFile(t, dir, "app/layout.js", "")

	got, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "app", "layout.js")
	if got.Mode != ModeManual || got.ManualTarget != want {
		t.Fatalf("got %+v, want manual with target %q", got, want)
	}
}
