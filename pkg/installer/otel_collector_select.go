package installer

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
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// collectorInstance represents a discovered OTel Collector process (running or installed).
type collectorInstance struct {
	pid         int    // 0 if the collector is installed but not currently running
	binaryPath  string // absolute path to the collector binary
	configPath  string // config file path parsed from --config arg; empty if unknown
	isDynatrace bool   // true for dynatrace-otel-collector binaries
}

// displayName returns a short human-readable label for the collector.
func (c collectorInstance) displayName() string {
	base := filepath.Base(c.binaryPath)
	if base == "" || base == "." {
		return "(unknown)"
	}
	return base
}

// detectConfigFromArgs parses a process command line and extracts the value of
// --config (or -c).  Returns empty string when no config flag is found.
func detectConfigFromArgs(args string) string {
	fields := strings.Fields(args)
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
// prompts the user to select one.  Returns (nil, ErrInstallCancelled) when the
// user selects [0] Cancel, and (nil, nil) when they choose to enter a path manually.
func selectCollector(instances []collectorInstance) (*collectorInstance, error) {
	if len(instances) == 0 {
		return nil, nil
	}
	for i, c := range instances {
		status := fmt.Sprintf("PID %d", c.pid)
		if c.pid == 0 {
			status = "not running"
		}
		fmt.Printf("  [%d]  %-36s  (%s)\n", i+1, c.displayName(), status)
		if c.binaryPath != "" {
			display.ColorMuted.Printf("       %s\n", c.binaryPath)
		}
		if c.configPath != "" {
			display.ColorMuted.Printf("       Config: %s\n", c.configPath)
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
		return nil, ErrInstallCancelled
	}
	if num == 0 {
		return nil, ErrInstallCancelled
	}
	chosen := instances[num-1]
	return &chosen, nil
}

// selectCollectorForUninstall prints a numbered list of Dynatrace OTel
// Collectors with a note that non-Dynatrace collectors are not shown, then
// prompts the user to select one (or all when multiple are available).
// Returns an empty slice when the user cancels.
func selectCollectorForUninstall(instances []collectorInstance) ([]collectorInstance, error) {
	fmt.Println()
	display.ColorMuted.Println("  Note: Only Dynatrace OTel Collectors are shown here.")
	display.ColorMuted.Println("        Regular (non-Dynatrace) OTel Collectors are not managed by this command.")
	fmt.Println()

	for i, c := range instances {
		status := fmt.Sprintf("PID %d", c.pid)
		if c.pid == 0 {
			status = "not running"
		}
		fmt.Printf("  [%d]  %-36s  (%s)\n", i+1, c.displayName(), status)
		if c.binaryPath != "" {
			display.ColorMuted.Printf("       %s\n", c.binaryPath)
		}
		if c.configPath != "" {
			display.ColorMuted.Printf("       Config: %s\n", c.configPath)
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
		return nil, ErrInstallCancelled
	}
	switch {
	case num == 0:
		return nil, ErrInstallCancelled
	case num >= 1 && num <= len(instances):
		return []collectorInstance{instances[num-1]}, nil
	default: // allIdx
		return instances, nil
	}
}
