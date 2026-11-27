package otel

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

// dotNetDownloadURL is where users get .NET when it is missing.
const dotNetDownloadURL = "https://dotnet.microsoft.com/download"

// Seams replaced in tests.
var (
	dotnetLookPathFn     = func(file string) (string, error) { return installer.ExecLookPath(file) }
	dotnetListRuntimesFn = func() (string, error) {
		out, err := exec.Command("dotnet", "--list-runtimes").Output()
		return string(out), err
	}
)

var (
	netCoreRuntimeLine = regexp.MustCompile(`(?m)^Microsoft\.NETCore\.App (\d+)\.`)
	// modernFramework matches the target framework monikers of modern .NET:
	// net5.0 and later, and netcoreapp2.0 and later.
	modernFramework = regexp.MustCompile(`^(?:net|netcoreapp)(\d+)\.\d+`)
)

// dotNetServiceCheck is the prerequisite verdict for one service.
type dotNetServiceCheck struct {
	Service dotnet.Service
	// Instrumented is true when the service can be instrumented.
	Instrumented bool
	// Note explains why a service is skipped, or qualifies one that is not.
	Note string
}

// dotNetPrerequisites is the result of checking the machine against a project's services.
type dotNetPrerequisites struct {
	// DotnetMissing is true when `dotnet` is not on PATH.
	DotnetMissing bool
	Services      []dotNetServiceCheck
}

// checkDotNetPrerequisites checks that `dotnet` is available and that, for each
// service the instrumentation supports, a runtime of its target framework's
// major version (or newer) is installed. Services that cannot be instrumented
// regardless of the machine are reported with their reason; services whose
// framework or installed runtimes cannot be determined are not blocked.
func checkDotNetPrerequisites(services []dotnet.Service) dotNetPrerequisites {
	res := dotNetPrerequisites{}
	if _, err := dotnetLookPathFn("dotnet"); err != nil {
		res.DotnetMissing = true
	}

	var majors []int
	runtimesKnown := false
	if !res.DotnetMissing {
		if out, err := dotnetListRuntimesFn(); err == nil {
			majors = parseNetCoreRuntimeMajors(out)
			runtimesKnown = len(majors) > 0
		}
	}

	for _, s := range services {
		check := dotNetServiceCheck{Service: s}
		switch {
		case s.UnsupportedReason() != "":
			check.Note = s.UnsupportedReason()
		case res.DotnetMissing:
			check.Note = ".NET is not installed"
		default:
			check.Instrumented, check.Note = checkRuntimeFor(s, majors, runtimesKnown)
		}
		res.Services = append(res.Services, check)
	}
	return res
}

func checkRuntimeFor(s dotnet.Service, installed []int, runtimesKnown bool) (ok bool, note string) {
	var required []int
	for _, f := range s.Frameworks {
		if m := modernFramework.FindStringSubmatch(f); m != nil {
			if major, err := strconv.Atoi(m[1]); err == nil {
				required = append(required, major)
			}
		}
	}
	if len(required) == 0 {
		return true, "framework not found in the project or Directory.Build.props, runtime not checked"
	}
	if !runtimesKnown {
		return true, "installed runtimes could not be determined, runtime not checked"
	}
	for _, need := range required {
		for _, have := range installed {
			if have >= need {
				return true, ""
			}
		}
	}
	return false, "requires " + describeMajors(required) + " runtime, not installed"
}

// describeMajors renders ".NET 8" or ".NET 6 or 8" for the required majors.
func describeMajors(majors []int) string {
	parts := make([]string, len(majors))
	for i, m := range majors {
		parts[i] = strconv.Itoa(m)
	}
	return ".NET " + strings.Join(parts, " or ")
}

// parseNetCoreRuntimeMajors extracts the major versions of the installed
// Microsoft.NETCore.App runtimes from `dotnet --list-runtimes` output.
func parseNetCoreRuntimeMajors(out string) []int {
	var majors []int
	for _, m := range netCoreRuntimeLine.FindAllStringSubmatch(out, -1) {
		if major, err := strconv.Atoi(m[1]); err == nil {
			majors = append(majors, major)
		}
	}
	return majors
}
