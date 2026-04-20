package installer

import (
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// nodeOtelProcessPatterns lists command-line patterns that identify a Node.js
// process instrumented by dtwiz. We use short, unique filenames rather than
// full paths to be robust against path variations (symlinks, /private prefix
// on macOS, absolute vs relative paths).
var nodeOtelProcessPatterns = []string{
	"@opentelemetry/auto-instrumentations-node/register",
	"next-otel-bootstrap.js",
	"nuxt-otel-bootstrap.mjs",
}

// findInstrumentedNodeProcesses detects running node processes whose command
// line contains OTel instrumentation patterns — regular --require hooks,
// Next.js bootstrap wrappers, or Nuxt ESM bootstrap scripts.
//
// Next.js is special: it rewrites process.title to "next-server (vX.Y.Z)",
// erasing the original command line from ps output. We detect these by
// scanning for "next-server" processes whose CWD contains a valid .otel/ dir.
//
// As a fallback, processes whose working directory IS a valid .otel/ dir
// (contains package.json with @opentelemetry) are also detected. This catches
// regular-app processes launched with cmd.Dir = .otel/ even if the command-line
// pattern is not matched (e.g., due to module path resolution).
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
				})
				seen[p.PID] = true
				break
			}
		}
		// Fallback: detect processes whose CWD is a dtwiz .otel/ directory.
		if !seen[p.PID] && p.WorkingDirectory != "" && filepath.Base(p.WorkingDirectory) == ".otel" {
			if isNodeOtelDir(p.WorkingDirectory) {
				logger.Debug("instrumented Node.js process found via CWD", "pid", p.PID, "cwd", p.WorkingDirectory)
				result = append(result, otelProcessInfo{
					pid:        p.PID,
					binaryPath: "",
				})
				seen[p.PID] = true
			}
		}
	}

	// Next.js rewrites process.title to "next-server (vX.Y.Z)", so it won't
	// appear in "node" process scans. Detect these separately.
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
			})
			seen[p.PID] = true
		}
	}

	return result
}

type nodeCleaner struct{}

func (nodeCleaner) Label() string { return "Node.js" }

// DetectProcesses implements RuntimeCleaner. It returns instrumented Node.js
// processes as DetectedProcess values so they are included in the generic
// runtime stop flow in UninstallOtelCollector.
func (nodeCleaner) DetectProcesses() []DetectedProcess {
	infos := findInstrumentedNodeProcesses()
	procs := make([]DetectedProcess, 0, len(infos))
	for _, info := range infos {
		procs = append(procs, DetectedProcess{
			PID:     info.pid,
			Command: info.binaryPath,
		})
	}
	return procs
}
