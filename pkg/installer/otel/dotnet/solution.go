// Package dotnet holds the .NET-specific detection logic for OpenTelemetry
// auto-instrumentation: solution and project file reading, service discovery
// and running-process recognition. It takes plain parameters and imports
// nothing from pkg/installer/otel; the otel package adapts its results.
package dotnet

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ProjectFileExtensions lists the project file extensions that identify a .NET project.
var ProjectFileExtensions = []string{".csproj", ".fsproj", ".vbproj"}

// SolutionFileExtensions lists the solution file extensions.
var SolutionFileExtensions = []string{".sln", ".slnx"}

// ErrSolutionUnreadable is returned when a solution file cannot be read or parsed,
// as opposed to a readable solution that lists no projects.
var ErrSolutionUnreadable = errors.New("solution file unreadable")

// slnProjectLine matches `Project("{type}") = "Name", "rel\path.csproj", "{guid}"`.
var slnProjectLine = regexp.MustCompile(`(?m)^\s*Project\("[^"]*"\)\s*=\s*"[^"]*"\s*,\s*"([^"]+)"`)

// utf8BOM is the byte order mark some tools write at the start of text files.
const utf8BOM = "\ufeff"

// IsSolutionFile reports whether name has a solution file extension.
func IsSolutionFile(name string) bool { return hasAnySuffix(name, SolutionFileExtensions) }

// IsProjectFile reports whether name has a project file extension.
func IsProjectFile(name string) bool { return hasAnySuffix(name, ProjectFileExtensions) }

func hasAnySuffix(name string, suffixes []string) bool {
	lower := strings.ToLower(name)
	for _, s := range suffixes {
		if strings.HasSuffix(lower, s) {
			return true
		}
	}
	return false
}

// ReadSolutionMembers returns the absolute paths of the project files listed in
// a .sln or .slnx file. Windows-style separators are normalized, and members
// that lie outside the solution's directory are dropped. A solution that cannot
// be read or parsed yields ErrSolutionUnreadable; a readable solution without
// projects yields an empty slice and no error.
func ReadSolutionMembers(solutionPath string) ([]string, error) {
	data, err := os.ReadFile(solutionPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSolutionUnreadable, err)
	}

	var rawPaths []string
	if strings.HasSuffix(strings.ToLower(solutionPath), ".slnx") {
		rawPaths, err = slnxProjectPaths(data)
	} else {
		rawPaths, err = slnProjectPaths(data)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSolutionUnreadable, err)
	}

	solutionDir := filepath.Dir(solutionPath)
	members := make([]string, 0, len(rawPaths))
	for _, raw := range rawPaths {
		if !IsProjectFile(raw) {
			continue // solution folders, web sites, shared projects
		}
		full := filepath.Clean(filepath.Join(solutionDir, filepath.FromSlash(strings.ReplaceAll(raw, `\`, "/"))))
		if !IsWithin(solutionDir, full) {
			continue
		}
		members = append(members, full)
	}
	return members, nil
}

func slnProjectPaths(data []byte) ([]string, error) {
	text := strings.TrimPrefix(string(data), utf8BOM)
	if !strings.Contains(text, "Microsoft Visual Studio Solution File") {
		return nil, errors.New("missing solution file header")
	}
	var paths []string
	for _, m := range slnProjectLine.FindAllStringSubmatch(text, -1) {
		paths = append(paths, m[1])
	}
	return paths, nil
}

func slnxProjectPaths(data []byte) ([]string, error) {
	decoder := xml.NewDecoder(strings.NewReader(strings.TrimPrefix(string(data), utf8BOM)))
	var paths []string
	sawRoot := false
	for {
		tok, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !sawRoot {
			if start.Name.Local != "Solution" {
				return nil, fmt.Errorf("unexpected root element %q", start.Name.Local)
			}
			sawRoot = true
			continue
		}
		if start.Name.Local == "Project" {
			for _, attr := range start.Attr {
				if attr.Name.Local == "Path" {
					paths = append(paths, attr.Value)
				}
			}
		}
	}
	if !sawRoot {
		return nil, errors.New("empty solution file")
	}
	return paths, nil
}

// IsWithin reports whether path equals dir or lies below it.
func IsWithin(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}
