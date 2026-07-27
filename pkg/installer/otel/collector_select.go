package otel

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// collectorInstance represents a discovered OTel Collector process (running or installed).
type collectorInstance struct {
	pid                 int    // 0 if the collector is installed but not currently running
	binaryPath          string // absolute path to the collector binary, or container image name
	configPath          string // host-accessible config file path; used for patching operations
	isDynatrace         bool   // true for dynatrace-otel-collector binaries
	containerRuntime    string // non-empty when the collector runs inside a container (docker/podman/nerdctl)
	containerName       string // container name when containerRuntime is set
	containerConfigPath string // container-internal config path; shown when configPath is empty
}

// displayName returns a short human-readable label for the collector.
func (c collectorInstance) displayName() string {
	if c.containerName != "" {
		return c.containerName
	}
	base := filepath.Base(c.binaryPath)
	if base == "" || base == "." {
		return "(unknown)"
	}
	return base
}

// detectConfigFromArgs parses a process command line and extracts the value of
// --config (or -c).  Returns empty string when no config flag is found.
func detectConfigFromArgs(args string) string {
	fields := splitArgs(args)
	for i, f := range fields {
		if (f == "--config" || f == "-c") && i+1 < len(fields) {
			return fields[i+1]
		}
		if strings.HasPrefix(f, "--config=") {
			return strings.TrimPrefix(f, "--config=")
		}
	}
	return ""
}

// splitArgs splits a command-line string into tokens with basic quote handling.
// Both single and double quotes are supported; quotes are stripped from the result.
func splitArgs(s string) []string {
	var tokens []string
	var cur strings.Builder
	inQuote := rune(0)
	for _, r := range s {
		switch {
		case inQuote != 0:
			if r == inQuote {
				inQuote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			inQuote = r
		case r == ' ' || r == '\t':
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// findAllRunningOtelCollectorsFunc is the function used to enumerate all running
// OTel Collector instances. Overridable in tests.
var findAllRunningOtelCollectorsFunc = findAllRunningOtelCollectors

// otelCollectorBinaryPatterns lists the binary name substrings that identify
// OTel Collector processes (Dynatrace and upstream distributions).
// Used by both the Unix pgrep scan and the binary-name filter.
// Note: "otelcorecol" must be listed before "otelcol" is irrelevant here
// (containment check, not prefix), but both must be present because
// "otelcol" is NOT a substring of "otelcorecol".
var otelCollectorBinaryPatterns = []string{
	"dynatrace-otel-collector",
	"otelcorecol",
	"otelcol",
	"opentelemetry-collector",
}

// looksLikeOtelCollector reports whether a binary base name matches a known
// OTel Collector naming pattern (Dynatrace or upstream distributions).
func looksLikeOtelCollector(name string) bool {
	lower := strings.ToLower(name)
	for _, p := range otelCollectorBinaryPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// isDynatraceOtelCollector reports whether the binary path belongs to the
// Dynatrace OTel Collector distribution.
func isDynatraceOtelCollector(binaryPath string) bool {
	return strings.Contains(strings.ToLower(filepath.Base(binaryPath)), "dynatrace-otel-collector")
}

// processFullArgs returns the full command line string for the given PID.
// Returns an empty string when the information cannot be retrieved.
func processFullArgs(pid int) string {
	pidStr := strconv.Itoa(pid)
	if runtime.GOOS == "windows" {
		out, err := exec.Command("powershell", "-NoProfile", "-Command",
			fmt.Sprintf("(Get-CimInstance Win32_Process -Filter \"ProcessId=%s\").CommandLine", pidStr)).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	out, err := exec.Command("ps", "-p", pidStr, "-o", "args=").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// findDynatraceOtelCollectors returns all Dynatrace OTel Collector instances on
// this machine — both currently running processes and binaries present in the
// well-known dtwiz install directories (~/opentelemetry and ./opentelemetry)
// that are not currently running.
func findDynatraceOtelCollectors() []collectorInstance {
	running := findRunningOtelCollectors()
	seenBinaries := map[string]bool{}
	var result []collectorInstance

	for _, rc := range running {
		binaryPath := rc.path
		if binaryPath == "" {
			binaryPath = binaryPathFromPID(rc.pid)
		}
		if binaryPath != "" {
			seenBinaries[binaryPath] = true
		}
		args := processFullArgs(rc.pid)
		result = append(result, collectorInstance{
			pid:         rc.pid,
			binaryPath:  binaryPath,
			configPath:  detectConfigFromArgs(args),
			isDynatrace: true,
		})
	}
	logger.Debug("findDynatraceOtelCollectors: running processes", "count", len(running))

	// Also surface installed (but not running) DT collectors in well-known dirs.
	binName := otelCollectorBinaryName()
	checkDir := func(dir string) {
		binPath := filepath.Join(dir, binName)
		if !fileExists(binPath) || seenBinaries[binPath] {
			return
		}
		seenBinaries[binPath] = true
		result = append(result, collectorInstance{
			pid:         0,
			binaryPath:  binPath,
			isDynatrace: true,
		})
		logger.Debug("findDynatraceOtelCollectors: installed (not running)", "binary", binPath)
	}
	if home, err := os.UserHomeDir(); err == nil {
		checkDir(filepath.Join(home, "opentelemetry"))
	}
	if cwd, err := os.Getwd(); err == nil {
		checkDir(filepath.Join(cwd, "opentelemetry"))
	}

	return result
}

// selectCollector prints a numbered list of all discovered OTel collectors and
// prompts the user to select one.  Returns (nil, nil) when instances is empty,
// and (nil, installer.ErrInstallCancelled) when the user selects [0] Cancel or enters invalid input.
func selectCollector(instances []collectorInstance) (*collectorInstance, error) {
	if len(instances) == 0 {
		return nil, nil
	}
	for i, c := range instances {
		var status string
		switch {
		case c.containerRuntime != "":
			status = "container (" + c.containerRuntime + ")"
		case c.pid > 0:
			status = fmt.Sprintf("PID %d", c.pid)
		default:
			status = "not running"
		}
		fmt.Printf("  [%d]  %-36s  (%s)\n", i+1, c.displayName(), status)
		if c.binaryPath != "" {
			display.ColorMuted.Printf("       %s\n", c.binaryPath)
		}
		switch {
		case c.configPath != "":
			display.ColorMuted.Printf("       Config: %s\n", c.configPath)
		case c.containerConfigPath != "":
			display.ColorMuted.Printf("       Config: %s (inside container, not host-mounted)\n", c.containerConfigPath)
		}
	}
	fmt.Printf("  [0]  Cancel\n")
	fmt.Println()
	display.ColorMessage.Print("  Enter number: ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil, scanner.Err()
	}
	answer := strings.TrimSpace(scanner.Text())
	num, err := strconv.Atoi(answer)
	if err != nil || num < 0 || num > len(instances) {
		return nil, installer.ErrInstallCancelled
	}
	if num == 0 {
		return nil, installer.ErrInstallCancelled
	}
	chosen := instances[num-1]
	return &chosen, nil
}

// selectCollectorForUninstall prints a numbered list of Dynatrace OTel
// Collectors with a note that non-Dynatrace collectors are not shown, then
// prompts the user to select one (or all when multiple are available).
// Returns installer.ErrInstallCancelled when the user cancels or enters invalid input.
func selectCollectorForUninstall(instances []collectorInstance) ([]collectorInstance, error) {
	if installer.AutoConfirm {
		return instances, nil
	}

	fmt.Println()
	display.ColorMuted.Println("  Note: Only Dynatrace OTel Collectors are shown here.")
	display.ColorMuted.Println("        Regular (non-Dynatrace) OTel Collectors are not managed by this command.")
	fmt.Println()

	for i, c := range instances {
		var status string
		switch {
		case c.containerRuntime != "":
			status = "container (" + c.containerRuntime + ")"
		case c.pid > 0:
			status = fmt.Sprintf("PID %d", c.pid)
		default:
			status = "not running"
		}
		fmt.Printf("  [%d]  %-36s  (%s)\n", i+1, c.displayName(), status)
		if c.binaryPath != "" {
			display.ColorMuted.Printf("       %s\n", c.binaryPath)
		}
		switch {
		case c.configPath != "":
			display.ColorMuted.Printf("       Config: %s\n", c.configPath)
		case c.containerConfigPath != "":
			display.ColorMuted.Printf("       Config: %s (inside container, not host-mounted)\n", c.containerConfigPath)
		}
	}
	allIdx := 0
	if len(instances) > 1 {
		allIdx = len(instances) + 1
		fmt.Printf("  [%d]  Uninstall all\n", allIdx)
	}
	fmt.Printf("  [0]  Cancel\n")
	fmt.Println()
	display.ColorMessage.Print("  Enter number: ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil, scanner.Err()
	}
	upperBound := len(instances)
	if allIdx != 0 {
		upperBound = allIdx
	}
	num, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || num < 0 || num > upperBound {
		return nil, installer.ErrInstallCancelled
	}
	switch {
	case num == 0:
		return nil, installer.ErrInstallCancelled
	case num >= 1 && num <= len(instances):
		return []collectorInstance{instances[num-1]}, nil
	default: // allIdx
		return instances, nil
	}
}
