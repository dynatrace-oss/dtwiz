//go:build !windows

package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

func findRunningOtelCollectors() []runningCollector {
	out, err := exec.Command("pgrep", "-f", "dynatrace-otel-collector").Output()
	if err != nil {
		return nil
	}
	var result []runningCollector
	for _, s := range strings.Fields(strings.TrimSpace(string(out))) {
		pid, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		rc := runningCollector{pid: pid}
		if exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid)); err == nil {
			rc.path = exe
		} else if out2, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(pid)).Output(); err == nil {
			rc.path = strings.TrimSpace(string(out2))
		}
		result = append(result, rc)
	}
	return result
}

// findAllRunningOtelCollectors returns all running OTel Collector processes
// on this host, including both Dynatrace and non-Dynatrace distributions.
// It uses pgrep to locate candidate PIDs by binary name pattern, then verifies
// that the process binary name actually matches an OTel collector (to avoid
// false positives from processes that merely have a collector name in their args).
// Binary patterns are sourced from otelCollectorBinaryPatterns (otel_collector_select.go).
// processWorkingDir returns the working directory of the given PID.
// Tries /proc/<pid>/cwd (Linux) first, then falls back to lsof (macOS).
// Returns empty string when the CWD cannot be determined.
func processWorkingDir(pid int) string {
	if cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid)); err == nil {
		return cwd
	}
	out, err := exec.Command("lsof", "-p", strconv.Itoa(pid), "-a", "-d", "cwd", "-Fn").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") {
			return strings.TrimPrefix(line, "n")
		}
	}
	return ""
}

// resolveRelativePath resolves path against baseDir if path is not absolute.
// Returns path unchanged when path is empty or already absolute.
func resolveRelativePath(path, baseDir string) string {
	if path == "" || filepath.IsAbs(path) || baseDir == "" {
		return path
	}
	return filepath.Join(baseDir, path)
}

func findAllRunningOtelCollectors() []collectorInstance {
	seen := map[int]bool{}
	currentPID := os.Getpid()
	var result []collectorInstance

	for _, pattern := range otelCollectorBinaryPatterns {
		out, err := exec.Command("pgrep", "-f", pattern).Output()
		if err != nil {
			continue
		}
		for _, s := range strings.Fields(strings.TrimSpace(string(out))) {
			pid, err := strconv.Atoi(s)
			if err != nil || pid == currentPID || seen[pid] {
				continue
			}

			// Get full command line to parse --config and the binary path.
			var args string
			if argsOut, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "args=").Output(); err == nil {
				args = strings.TrimSpace(string(argsOut))
			}

			// Determine binary path: prefer /proc/<pid>/exe (Linux), fall back to first args field.
			var binaryPath string
			if exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid)); err == nil {
				binaryPath = exe
			} else if args != "" {
				binaryPath = strings.Fields(args)[0]
			}

			// Skip processes whose binary name does not match an OTel collector.
			// This avoids matching e.g. "java -classpath /opt/otelcol.jar ..." where
			// pgrep matched "otelcol" in the classpath argument.
			baseName := strings.ToLower(filepath.Base(binaryPath))
			if !looksLikeOtelCollector(baseName) {
				logger.Debug("findAllRunningOtelCollectors: skipping non-collector binary", "pid", pid, "binary", binaryPath)
				continue
			}

			// Resolve relative paths (binary and config) against the process's CWD.
			// Paths in ps args are relative to the directory the process was launched from,
			// which may differ from dtwiz's working directory.
			configPath := detectConfigFromArgs(args)
			if !filepath.IsAbs(binaryPath) || (!filepath.IsAbs(configPath) && configPath != "") {
				if procCWD := processWorkingDir(pid); procCWD != "" {
					binaryPath = resolveRelativePath(binaryPath, procCWD)
					configPath = resolveRelativePath(configPath, procCWD)
				}
			}

			seen[pid] = true
			inst := collectorInstance{
				pid:         pid,
				binaryPath:  binaryPath,
				configPath:  configPath,
				isDynatrace: isDynatraceOtelCollector(binaryPath),
			}
			logger.Debug("findAllRunningOtelCollectors: found", "pid", pid, "binary", binaryPath, "isDynatrace", inst.isDynatrace)
			result = append(result, inst)
		}
	}
	return result
}
