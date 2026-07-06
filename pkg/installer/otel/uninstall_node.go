package otel

import (
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// Short, unique filenames are used rather than full paths to stay robust
// against path variations (symlinks, /private prefix on macOS, absolute vs
// relative paths).
var nodeOtelProcessPatterns = []string{
	"@opentelemetry/auto-instrumentations-node/register",
	"next-otel-bootstrap.js",
	"nuxt-otel-bootstrap.mjs",
}

// findInstrumentedNodeProcesses detects running node processes instrumented by dtwiz.
//
// Next.js rewrites process.title to "next-server (vX.Y.Z)", erasing the
// original command line from ps output. These are detected by scanning for
// "next-server" processes whose CWD contains a valid .otel/ dir.
//
// As a fallback, processes whose CWD IS a valid .otel/ dir are also detected,
// catching regular-app processes launched with cmd.Dir = .otel/ even if the
// command-line pattern is not matched.
func findInstrumentedNodeProcesses() []otelProcessInfo {
	procs := detectProcesses("node", nil)
	var result []otelProcessInfo
	seen := map[int]bool{}
	for _, p := range procs {
		cmdLower := strings.ToLower(p.Command)
		for _, pattern := range nodeOtelProcessPatterns {
			if strings.Contains(cmdLower, strings.ToLower(pattern)) {
				logger.Debug("instrumented Node.js process found", "pid", p.PID, "pattern", pattern)
				result = append(result, otelProcessInfo{
					pid:        p.PID,
					binaryPath: "",
					command:    p.Command,
					workingDir: p.WorkingDirectory,
				})
				seen[p.PID] = true
				break
			}
		}
		// Fallback: CWD is a dtwiz .otel/ dir.
		if !seen[p.PID] && p.WorkingDirectory != "" && filepath.Base(p.WorkingDirectory) == ".otel" {
			if isNodeOtelDir(p.WorkingDirectory) {
				logger.Debug("instrumented Node.js process found via CWD", "pid", p.PID, "cwd", p.WorkingDirectory)
				result = append(result, otelProcessInfo{
					pid:        p.PID,
					binaryPath: "",
					command:    p.Command,
					workingDir: p.WorkingDirectory,
				})
				seen[p.PID] = true
			}
		}
	}

	// next-server processes don't appear in node scans — detect separately.
	nextProcs := detectProcesses("next-server", nil)
	for _, p := range nextProcs {
		if seen[p.PID] {
			continue
		}
		// Confirm it's dtwiz-launched: CWD must contain a valid .otel/ dir.
		if p.WorkingDirectory == "" {
			continue
		}
		otelDir := filepath.Join(p.WorkingDirectory, ".otel")
		if isNodeOtelDir(otelDir) {
			logger.Debug("instrumented Next.js process found via next-server title", "pid", p.PID, "cwd", p.WorkingDirectory)
			result = append(result, otelProcessInfo{
				pid:        p.PID,
				binaryPath: "",
				command:    p.Command,
				workingDir: p.WorkingDirectory,
			})
			seen[p.PID] = true
		} else {
			logger.Debug("next-server process skipped — no valid .otel/ in CWD", "pid", p.PID, "cwd", p.WorkingDirectory)
		}
	}

	return result
}

type nodeCleaner struct{}

func (nodeCleaner) Label() string { return "Node.js" }

// DetectProcesses implements RuntimeCleaner.
func (nodeCleaner) DetectProcesses() []DetectedProcess {
	infos := findInstrumentedNodeProcesses()
	procs := make([]DetectedProcess, 0, len(infos))
	for _, info := range infos {
		desc := nodeProcessDescription(info)
		procs = append(procs, DetectedProcess{
			PID:     info.pid,
			Command: desc,
		})
	}
	return procs
}

func nodeProcessDescription(info otelProcessInfo) string {
	projectDir := info.workingDir
	// If the working directory is .otel/, the project is one level up.
	if filepath.Base(projectDir) == ".otel" {
		projectDir = filepath.Dir(projectDir)
	}
	if projectDir == "" {
		if info.command != "" {
			return info.command
		}
		return info.binaryPath
	}
	svcName := nodeServiceNameFromCommand(projectDir, info.command)
	return projectDir + "  " + svcName
}

// nodeServiceNameFromCommand derives the service name from the process command
// line. For regular entrypoints the command ends with a relative path like
// "../s-frontend/index.js"; for framework wrappers (next/nuxt) it falls back
// to the project directory name.
func nodeServiceNameFromCommand(projectDir, command string) string {
	fields := strings.Fields(command)
	// Last arg is a relative entrypoint path (e.g. ../s-frontend/index.js).
	if len(fields) > 0 {
		last := fields[len(fields)-1]
		if strings.HasPrefix(last, ".."+string(filepath.Separator)) || strings.HasPrefix(last, "../") {
			// Resolve relative to .otel/ → actual entrypoint relative to project.
			ep := strings.TrimPrefix(last, ".."+string(filepath.Separator))
			ep = strings.TrimPrefix(ep, "../")
			return nodeServiceNameFromEntrypoint(projectDir, ep)
		}
	}
	// Framework (next/nuxt) or unrecognized pattern — use project name.
	return projectServiceName(projectDir)
}
