package otel

import (
	"os"
	"path/filepath"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const dotNetRuntimeName = ".NET"

// dotNetProjectMarkers are suffix markers: solution and project files have arbitrary names.
var dotNetProjectMarkers = []string{"*.sln", "*.slnx", "*.csproj", "*.fsproj", "*.vbproj"}

// detectDotNetProjects finds .NET projects under roots. A solution directory is
// one project; project directories below it are its services, not projects.
func detectDotNetProjects(roots []string) []ScannedProject {
	scanned := scanProjectDirs(dotNetProjectMarkers, dotnet.IgnoredDirNames, roots)
	return dropSolutionMembers(scanned)
}

func dropSolutionMembers(projects []ScannedProject) []ScannedProject {
	var solutionDirs []string
	for _, p := range projects {
		for _, m := range p.Markers {
			if dotnet.IsSolutionFile(m) {
				solutionDirs = append(solutionDirs, p.Path)
				break
			}
		}
	}
	kept := make([]ScannedProject, 0, len(projects))
	for _, p := range projects {
		member := false
		for _, dir := range solutionDirs {
			if p.Path != dir && isDescendant(p.Path, dir) {
				logger.Debug("dropping project below solution directory", "project", p.Path, "solution_dir", dir)
				member = true
				break
			}
		}
		if !member {
			kept = append(kept, p)
		}
	}
	return kept
}

// hasDotNetMarker reports whether dir directly contains a solution or project file.
func hasDotNetMarker(dir string) bool {
	return len(dotNetMarkersIn(dir)) > 0
}

// dotNetMarkersIn returns the names of the solution and project files directly inside dir.
func dotNetMarkersIn(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var markers []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if dotnet.IsSolutionFile(e.Name()) || dotnet.IsProjectFile(e.Name()) {
			markers = append(markers, e.Name())
		}
	}
	return markers
}

// detectDotNetRuntimeProjects scans for .NET projects and running .NET
// processes in parallel and annotates the projects with their PIDs.
func detectDotNetRuntimeProjects(roots []string) []detectedProject {
	projects, processes := runInParallel(
		func() []ScannedProject { return detectDotNetProjects(roots) },
		detectDotNetProcesses,
	)
	matchDotNetProcessesToProjects(projects, processes)
	return detectedProjectsFromScan(dotNetRuntimeName, projects)
}

// dotNetServices returns the services of a detected .NET project. When the
// project came from --project (no markers recorded) the directory is inspected.
func dotNetServices(proj ScannedProject) []dotnet.Service {
	markers := proj.Markers
	if len(markers) == 0 {
		markers = dotNetMarkersIn(proj.Path)
	}
	return dotnet.DiscoverServices(filepath.Clean(proj.Path), markers)
}
