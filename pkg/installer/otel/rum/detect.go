package rum

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/markers"
)

// InjectionMode classifies how RUM auto-injection can be applied.
type InjectionMode string

const (
	ModeAuto   InjectionMode = "auto"
	ModeManual InjectionMode = "manual"
)

// DetectionResult holds the outcome of a RUM detection scan.
type DetectionResult struct {
	Mode            InjectionMode
	InjectableFiles []string // absolute paths; non-empty when Mode == ModeAuto
	ManualReason    string   // human-readable; non-empty when Mode == ModeManual
}

var excludedDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	".next":        true,
	".nuxt":        true,
	".svelte-kit":  true,
	"dist":         true,
	"build":        true,
	"out":          true,
	".output":      true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
}

func detectFramework(dir string) (string, bool) {
	if markers.IsNextJSProject(dir) {
		return "Next.js", true
	}
	if markers.IsNuxtProject(dir) {
		return "Nuxt", true
	}
	return "", false
}

func walkHTML(dir string) ([]string, error) {
	var files []string
	err := fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if excludedDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		var isHTML bool
		if runtime.GOOS == "windows" {
			isHTML = strings.EqualFold(filepath.Ext(name), ".html")
		} else {
			isHTML = filepath.Ext(name) == ".html"
		}
		if isHTML {
			files = append(files, filepath.Join(dir, filepath.FromSlash(path)))
		}
		return nil
	})
	return files, err
}

func isWritable(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// Detect scans dir to determine RUM injection mode.
func Detect(dir string) (DetectionResult, error) {
	if name, found := detectFramework(dir); found {
		return DetectionResult{Mode: ModeManual, ManualReason: name}, nil
	}

	htmlFiles, err := walkHTML(dir)
	if err != nil {
		return DetectionResult{}, err
	}

	var writable []string
	for _, f := range htmlFiles {
		if isWritable(f) {
			writable = append(writable, f)
		}
	}

	var hasIndex bool
	for _, f := range writable {
		if filepath.Base(f) == "index.html" {
			hasIndex = true
			break
		}
	}
	if hasIndex {
		var filtered []string
		for _, f := range writable {
			if filepath.Base(f) == "index.html" {
				filtered = append(filtered, f)
			}
		}
		writable = filtered
	}

	if len(writable) == 0 {
		return DetectionResult{Mode: ModeManual, ManualReason: "no writable HTML files found"}, nil
	}

	sort.Strings(writable)
	return DetectionResult{Mode: ModeAuto, InjectableFiles: writable}, nil
}
