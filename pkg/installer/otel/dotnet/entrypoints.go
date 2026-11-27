package dotnet

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// IgnoredDirNames are .NET build output and package directories that are never
// searched for projects (dot directories such as .vs are skipped separately).
var IgnoredDirNames = []string{"bin", "obj", "packages"}

// maxSearchDepth bounds the project file search below a project directory.
const maxSearchDepth = 5

// Service is a runnable .NET project that could be instrumented.
type Service struct {
	Name string
	// AssemblyName is the name of the built library (without extension); it
	// differs from Name when two services share a name.
	AssemblyName    string
	ProjectFile     string // absolute path
	Dir             string // absolute project directory
	Frameworks      []string
	DotNetFramework bool
	SelfContained   bool
}

// FrameworkLabel returns the target frameworks joined for display, or "unknown".
func (s Service) FrameworkLabel() string {
	if len(s.Frameworks) == 0 {
		return "unknown"
	}
	return strings.Join(s.Frameworks, ", ")
}

// UnsupportedReason returns why the service cannot be instrumented, or "".
func (s Service) UnsupportedReason() string {
	switch {
	case s.DotNetFramework:
		return ".NET Framework, not supported yet"
	case s.SelfContained:
		return "self-contained, not supported"
	}
	return ""
}

func isIgnoredDirName(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") || name == "node_modules" {
		return true
	}
	for _, n := range IgnoredDirNames {
		if strings.EqualFold(name, n) {
			return true
		}
	}
	return false
}

// DiscoverServices finds the runnable services of the project in projectDir.
// markers are the file names that identified the directory (solution and
// project files). Solution members are used when a solution lists them;
// otherwise, or when the solution is unreadable, project files are searched up
// to five levels below the directory.
func DiscoverServices(projectDir string, markers []string) []Service {
	candidates := map[string]string{} // lowercase path → path
	add := func(p string) { candidates[strings.ToLower(p)] = p }

	needSearch := false
	hasSolution := false
	for _, m := range markers {
		switch {
		case IsSolutionFile(m):
			hasSolution = true
			members, err := ReadSolutionMembers(filepath.Join(projectDir, m))
			if err != nil {
				logger.Debug("solution unreadable, searching for project files", "solution", m, "error", err)
				needSearch = true
				continue
			}
			for _, member := range members {
				if inIgnoredDir(projectDir, member) {
					logger.Debug("skipping solution member in ignored directory", "project", member)
					continue
				}
				add(member)
			}
		case IsProjectFile(m):
			add(filepath.Join(projectDir, m))
		}
	}
	if !hasSolution {
		needSearch = true
	}
	if needSearch {
		for _, p := range searchProjectFiles(projectDir) {
			add(p)
		}
	}

	paths := make([]string, 0, len(candidates))
	for _, p := range candidates {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var services []Service
	for _, p := range paths {
		pf, err := ParseProjectFile(p)
		if err != nil {
			logger.Debug("skipping unreadable project file", "project", p, "error", err)
			continue
		}
		if !pf.IsService {
			logger.Debug("skipping non-service project", "project", p, "test", pf.IsTest)
			continue
		}
		name := pf.AssemblyName
		if name == "" {
			name = pf.Name
		}
		services = append(services, Service{
			Name:            name,
			AssemblyName:    name,
			ProjectFile:     p,
			Dir:             filepath.Dir(p),
			Frameworks:      pf.Frameworks,
			DotNetFramework: pf.DotNetFramework,
			SelfContained:   pf.SelfContained,
		})
		logger.Debug("discovered .NET service", "name", name, "project", p, "frameworks", strings.Join(pf.Frameworks, ";"))
	}
	disambiguateNames(projectDir, services)
	return services
}

func inIgnoredDir(root, path string) bool {
	rel, err := filepath.Rel(root, filepath.Dir(path))
	if err != nil || rel == "." {
		return false
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if isIgnoredDirName(part) {
			return true
		}
	}
	return false
}

func searchProjectFiles(root string) []string {
	var found []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			if isIgnoredDirName(d.Name()) || strings.Count(rel, string(filepath.Separator)) >= maxSearchDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if IsProjectFile(d.Name()) {
			found = append(found, path)
		}
		return nil
	})
	return found
}

// disambiguateNames renames services that share a name (case-insensitively) to
// their project-relative directory path.
func disambiguateNames(projectDir string, services []Service) {
	count := map[string]int{}
	for _, s := range services {
		count[strings.ToLower(s.Name)]++
	}
	for i, s := range services {
		if count[strings.ToLower(s.Name)] < 2 {
			continue
		}
		rel, err := filepath.Rel(projectDir, s.Dir)
		if err != nil || rel == "." {
			rel = s.Name
		}
		services[i].Name = filepath.ToSlash(rel)
	}
}
