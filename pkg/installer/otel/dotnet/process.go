package dotnet

import (
	"os"
	"path/filepath"
	"strings"
)

// Process is a listed OS process. WorkingDirectory may be empty when it could
// not be determined (always on Windows).
type Process struct {
	PID              int
	Command          string
	WorkingDirectory string
}

// dotnetVerbs are `dotnet` sub-commands; run and watch start an application,
// exec runs a library (looked up in the arguments), the rest are tooling.
var dotnetVerbs = map[string]bool{
	"run": true, "watch": true, "exec": true, "build": true, "test": true, "restore": true,
	"msbuild": true, "publish": true, "pack": true, "clean": true, "build-server": true,
	"tool": true, "nuget": true, "new": true, "format": true, "vstest": true, "store": true,
	"sdk": true, "workload": true, "add": true, "remove": true, "list": true, "fsi": true,
}

// toolingNames are the lowercase assembly/executable names (without extension)
// of .NET tooling that is not a user application.
var toolingNames = map[string]bool{
	"msbuild": true, "vbcscompiler": true, "csc": true, "vbc": true, "fsc": true,
	"testhost": true, "vstest.console": true, "nuget.commandline.xplat": true,
	"microsoft.codeanalysis.languageserver": true, "omnisharp": true, "fsautocomplete": true,
	"dotnet-watch": true, "dotnet": true, "dtwiz": true,
}

// SelectProcesses keeps the running .NET applications among all: `dotnet run`,
// `dotnet watch`, `dotnet <app>.dll` and application-host executables. Tooling
// (build and compiler servers, MSBuild nodes, test hosts, IDE language
// services, the bare dotnet CLI) and dtwiz itself are dropped.
func SelectProcesses(all []Process) []Process {
	selected := make([]Process, 0)
	for _, p := range all {
		if isDotNetApplication(p) {
			selected = append(selected, p)
		}
	}
	return selected
}

func isDotNetApplication(p Process) bool {
	fields := SplitCommandLine(p.Command)
	if len(fields) == 0 {
		return false
	}
	exe := fields[0]
	stem := executableStem(exe)

	if stem == "dotnet" {
		return isDotNetHostApplication(fields[1:])
	}
	if toolingNames[stem] || strings.HasPrefix(stem, "servicehub.") {
		return false
	}
	if !filepath.IsAbs(exe) && !isWindowsAbs(exe) {
		if p.WorkingDirectory == "" || !strings.ContainsAny(exe, `/\`) {
			return false
		}
		exe = filepath.Join(p.WorkingDirectory, filepath.FromSlash(strings.ReplaceAll(exe, `\`, "/")))
	}
	if isToolingPath(exe) {
		return false
	}
	return isAppHost(exe)
}

// isDotNetHostApplication decides whether the arguments of a `dotnet` process
// start an application.
func isDotNetHostApplication(args []string) bool {
	for _, arg := range args {
		lower := strings.ToLower(arg)
		switch {
		case strings.HasPrefix(lower, "-"):
			continue
		case lower == "run" || lower == "watch":
			return true
		case lower == "exec":
			continue
		case dotnetVerbs[lower]:
			return false
		case strings.HasSuffix(lower, ".dll"):
			return !isToolingLibrary(arg)
		}
	}
	return false
}

func isToolingLibrary(path string) bool {
	return toolingNames[strings.TrimSuffix(strings.ToLower(baseName(path)), ".dll")] || isToolingPath(path)
}

// isToolingPath reports whether path lies in the .NET SDK, a shared runtime,
// the NuGet package cache or an IDE extension directory.
func isToolingPath(path string) bool {
	parts := strings.Split(strings.ToLower(strings.ReplaceAll(path, `\`, "/")), "/")
	for i, part := range parts {
		switch {
		case part == "sdk" && i+1 < len(parts) && len(parts[i+1]) > 0 && parts[i+1][0] >= '0' && parts[i+1][0] <= '9':
			return true
		case part == "packages" && i > 0 && parts[i-1] == ".nuget":
			return true
		case part == "extensions" && i > 0 && (parts[i-1] == ".vscode" || parts[i-1] == ".vscode-server"):
			return true
		case part == "shared" && i+1 < len(parts) && strings.HasPrefix(parts[i+1], "microsoft."):
			return true
		}
	}
	return false
}

// isAppHost reports whether exe is a native application host: a managed
// library and a runtime configuration file with the same name sit beside it.
func isAppHost(exe string) bool {
	stem := strings.TrimSuffix(exe, filepath.Ext(exe))
	if !strings.EqualFold(filepath.Ext(exe), ".exe") {
		stem = exe
	}
	if !fileExists(stem+".dll") || !fileExists(stem+".runtimeconfig.json") {
		return false
	}
	return true
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func executableStem(exe string) string {
	name := strings.ToLower(baseName(exe))
	return strings.TrimSuffix(name, ".exe")
}

// baseName returns the last element of path, accepting either separator.
func baseName(path string) string {
	path = strings.TrimRight(path, `/\`)
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}

func isWindowsAbs(p string) bool {
	return len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') || strings.HasPrefix(p, `\\`)
}

// SplitCommandLine splits a command line into whitespace-separated fields,
// honouring double quotes around a field (Windows paths with spaces). Unquoted
// Unix paths containing spaces cannot be split reliably.
func SplitCommandLine(command string) []string {
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

// MatchPIDs returns the PIDs of the processes that belong to the project in
// projectDir: the working directory, the command line or the executable lies
// inside the directory (build output included). Comparison ignores letter case
// and path separator style.
func MatchPIDs(projectDir string, processes []Process) []int {
	dir := normalizePath(projectDir)
	pids := make([]int, 0)
	if dir == "" {
		return pids
	}
	for _, p := range processes {
		if processInDir(dir, p) {
			pids = append(pids, p.PID)
		}
	}
	return pids
}

func processInDir(dir string, p Process) bool {
	wd := normalizePath(p.WorkingDirectory)
	if wd != "" && (wd == dir || strings.HasPrefix(wd, dir+"/")) {
		return true
	}
	command := " " + strings.ToLower(strings.NewReplacer(`\`, "/", `"`, " ").Replace(p.Command)) + " "
	for from := 0; ; {
		i := strings.Index(command[from:], dir)
		if i < 0 {
			break
		}
		start, end := from+i, from+i+len(dir)
		prev, next := command[start-1], command[end]
		if (prev == ' ' || prev == '=' || prev == '\'') && (next == '/' || next == ' ' || next == '\'') {
			return true
		}
		from = start + 1
	}
	if fields := SplitCommandLine(p.Command); len(fields) > 0 && wd != "" {
		exe := fields[0]
		if !filepath.IsAbs(exe) && !isWindowsAbs(exe) && strings.ContainsAny(exe, `/\`) {
			resolved := normalizePath(filepath.Join(p.WorkingDirectory, filepath.FromSlash(strings.ReplaceAll(exe, `\`, "/"))))
			return strings.HasPrefix(resolved, dir+"/")
		}
	}
	return false
}

func normalizePath(p string) string {
	p = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(p), `\`, "/"))
	return strings.TrimRight(p, "/")
}
