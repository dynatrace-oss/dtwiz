package otel

import (
	"debug/buildinfo"
	"path/filepath"
	"strings"
)

// goToolBinaries are long-running Go developer tools that are not user services.
var goToolBinaries = map[string]bool{
	"gopls": true, "dlv": true, "golangci-lint": true, "golangci-lint-langserver": true,
	"staticcheck": true, "go": true,
}

// isGoBinary reports whether path is an executable built by the Go toolchain.
// It is a variable so tests can stub it.
var isGoBinary = func(path string) bool {
	_, err := buildinfo.ReadFile(path)
	return err == nil
}

// detectGoProcesses returns running `go run` processes and compiled Go binaries
// so both can be associated with detected projects.
func detectGoProcesses() []DetectedProcess {
	return selectGoProcesses(listProcesses(), isGoBinary)
}

// selectGoProcesses keeps `go run` invocations and processes whose executable
// is a Go binary. all must carry the command line and, when known, the working
// directory (used to resolve relative executable paths).
func selectGoProcesses(all []DetectedProcess, isGo func(path string) bool) []DetectedProcess {
	selected := make([]DetectedProcess, 0)
	for _, p := range all {
		fields := splitCommandLine(p.Command)
		if len(fields) == 0 {
			continue
		}
		exe := fields[0]
		name := strings.TrimSuffix(strings.ToLower(filepath.Base(exe)), ".exe")
		if name == "go" {
			if len(fields) > 1 && fields[1] == "run" {
				selected = append(selected, p)
			}
			continue
		}
		if goToolBinaries[name] || strings.HasPrefix(name, "dynatrace-otel-collector") || strings.HasPrefix(name, "otelcol") {
			continue
		}
		if !filepath.IsAbs(exe) {
			if p.WorkingDirectory == "" || !strings.ContainsAny(exe, `/\`) {
				continue
			}
			exe = filepath.Join(p.WorkingDirectory, exe)
		}
		if isGo(exe) {
			selected = append(selected, p)
		}
	}
	return selected
}

// splitCommandLine splits a command line into whitespace-separated fields,
// honouring double quotes around a field (Windows paths with spaces). Unquoted
// absolute Unix paths containing spaces are not supported.
func splitCommandLine(command string) []string {
	var fields []string
	var cur strings.Builder
	inQuotes := false
	for _, r := range command {
		switch {
		case r == '"':
			inQuotes = !inQuotes
		case (r == ' ' || r == '\t') && !inQuotes:
			if cur.Len() > 0 {
				fields = append(fields, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		fields = append(fields, cur.String())
	}
	return fields
}
