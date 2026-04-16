//go:build !windows

package installer

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
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
