package rum

import (
	"os"
	"path/filepath"
)

var layoutExtensions = []string{".tsx", ".jsx", ".ts", ".js"}

// nuxtConfigExtensions lists the Nuxt config extensions in priority order.
var nuxtConfigExtensions = []string{".ts", ".js", ".mjs", ".mts"}

// nextLayoutCandidates lists the root layout locations of a Next.js project in
// priority order: App Router layouts first, then Pages Router documents.
var nextLayoutCandidates = []string{
	filepath.Join("app", "layout"),
	filepath.Join("src", "app", "layout"),
	filepath.Join("pages", "_document"),
	filepath.Join("src", "pages", "_document"),
}

// findManualTarget returns the file where the user should add the RUM tag for
// a detected framework, or "" when it cannot be determined.
func findManualTarget(dir, framework string) string {
	switch framework {
	case "Next.js":
		return firstExistingFile(dir, nextLayoutCandidates, layoutExtensions)
	case "Nuxt":
		return firstExistingFile(dir, []string{"nuxt.config"}, nuxtConfigExtensions)
	}
	return ""
}

func firstExistingFile(dir string, bases, exts []string) string {
	for _, base := range bases {
		for _, ext := range exts {
			p := filepath.Join(dir, base+ext)
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p
			}
		}
	}
	return ""
}
