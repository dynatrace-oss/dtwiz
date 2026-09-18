package markers

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// IsNextJSProject checks for Next.js config files or next in package.json dependencies.
func IsNextJSProject(dir string) bool {
	for _, name := range []string{"next.config.js", "next.config.ts", "next.config.mjs"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return HasDependency(dir, "next")
}

// IsNuxtProject checks for Nuxt config files or nuxt in package.json dependencies.
func IsNuxtProject(dir string) bool {
	for _, name := range []string{"nuxt.config.js", "nuxt.config.ts", "nuxt.config.mjs", "nuxt.config.mts"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return HasDependency(dir, "nuxt")
}

// HasDependency checks if a package name appears in dependencies or devDependencies.
func HasDependency(dir, pkgName string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	_, ok1 := pkg.Dependencies[pkgName]
	_, ok2 := pkg.DevDependencies[pkgName]
	return ok1 || ok2
}
