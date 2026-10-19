package rum

import (
	"os"
	"path/filepath"
)

var layoutExtensions = []string{".tsx", ".jsx", ".ts", ".js"}

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
	if framework != "Next.js" {
		return ""
	}
	for _, base := range nextLayoutCandidates {
		for _, ext := range layoutExtensions {
			p := filepath.Join(dir, base+ext)
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p
			}
		}
	}
	return ""
}
