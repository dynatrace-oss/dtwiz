package dotnet

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maxSourceWalk bounds how many files NewestSource looks at; a larger tree is
// reported as incomplete so that the caller builds.
const maxSourceWalk = 20000

// sourceExtensions are the file kinds whose change makes a built library stale.
var sourceExtensions = map[string]bool{
	".cs": true, ".razor": true, ".cshtml": true, ".fs": true, ".vb": true,
	".csproj": true, ".fsproj": true, ".vbproj": true, ".props": true,
}

// NewestSource returns the latest modification time of the source files below
// root: code, project and props files, and appsettings*.json (a build copies
// those to the output). Ignored directories (bin, obj, packages, hidden ones)
// are skipped. complete is false when the walk stopped at its file limit.
func NewestSource(root string) (newest time.Time, complete bool) {
	return NewestSourceWithin(root, maxSourceWalk)
}

// NewestSourceWithin is NewestSource with an explicit file limit.
func NewestSourceWithin(root string, maxFiles int) (newest time.Time, complete bool) {
	complete = true
	seen := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && isIgnoredDirName(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if seen++; seen > maxFiles {
			complete = false
			return filepath.SkipAll
		}
		if !isSourceFile(d.Name()) {
			return nil
		}
		if info, err := d.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	return newest, complete
}

func isSourceFile(name string) bool {
	lower := strings.ToLower(name)
	if sourceExtensions[filepath.Ext(lower)] {
		return true
	}
	return strings.HasPrefix(lower, "appsettings") && strings.HasSuffix(lower, ".json")
}

// BuildInfo says which library a service is launched from and whether it must
// be built first.
type BuildInfo struct {
	// Library is the library the service is launched from: the existing one, or
	// the default output path of the build. Empty when it cannot be predicted.
	Library string
	// Existing is true when a usable built library exists and no source is newer.
	Existing bool
	// Stale is true when a usable library exists but a source is newer, or the
	// source check could not be completed.
	Stale bool
}

// NeedsBuild reports whether the service must be built before it is launched.
func (b BuildInfo) NeedsBuild() bool { return !b.Existing }

// ResolveBuild looks for the built library of the service. newestSource and
// sourcesComplete come from NewestSource for the project: a library older than
// the newest source, or an incomplete source check, means the service is built
// again. The most recently written library wins when several exist.
func ResolveBuild(s Service, newestSource time.Time, sourcesComplete bool) BuildInfo {
	lib, written, found := findBuiltLibrary(s)
	if found {
		if !sourcesComplete || newestSource.After(written) {
			return BuildInfo{Library: lib, Stale: true}
		}
		return BuildInfo{Library: lib, Existing: true}
	}
	return BuildInfo{Library: defaultLibrary(s)}
}

// findBuiltLibrary returns the newest usable library below the service's bin
// directory: bin/<configuration>/<framework>/<assembly>.dll with its
// .runtimeconfig.json beside it.
func findBuiltLibrary(s Service) (path string, written time.Time, found bool) {
	name := s.AssemblyName
	if name == "" {
		name = s.Name
	}
	binDir := filepath.Join(s.Dir, "bin")
	configs, err := os.ReadDir(binDir)
	if err != nil {
		return "", time.Time{}, false
	}
	for _, cfg := range configs {
		if !cfg.IsDir() {
			continue
		}
		for _, tfm := range candidateFrameworks(filepath.Join(binDir, cfg.Name()), s) {
			lib := filepath.Join(binDir, cfg.Name(), tfm, name+".dll")
			info, err := os.Stat(lib)
			if err != nil || info.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(lib), name+".runtimeconfig.json")); err != nil {
				continue
			}
			if !found || info.ModTime().After(written) {
				path, written, found = lib, info.ModTime(), true
			}
		}
	}
	return path, written, found
}

// candidateFrameworks lists the framework directories to look in below a
// configuration directory: the service's own frameworks, or every directory
// when they are unknown.
func candidateFrameworks(configDir string, s Service) []string {
	if len(s.Frameworks) > 0 {
		return s.Frameworks
	}
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs
}

// defaultLibrary is where a default build puts the library, or "" when the
// framework is unknown.
func defaultLibrary(s Service) string {
	if len(s.Frameworks) == 0 {
		return ""
	}
	name := s.AssemblyName
	if name == "" {
		name = s.Name
	}
	return filepath.Join(s.Dir, "bin", "Debug", s.Frameworks[0], name+".dll")
}
