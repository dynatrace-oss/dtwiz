package integration

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ServiceName returns the canonical "<testID>-<lang>" service name.
func ServiceName(env *TestEnv, lang string) string {
	return fmt.Sprintf("%s-%s", env.TestID, lang)
}

// WaitForPort blocks until the given TCP port accepts connections or the
// timeout is exceeded. It calls t.Fatal if the port is not ready in time.
func WaitForPort(t *testing.T, port int, timeout time.Duration) {
	t.Helper()
	addr := fmt.Sprintf("localhost:%d", port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("WaitForPort: port %d not ready within %v", port, timeout)
}

// RegisterPortCleanup registers a t.Cleanup that kills any process on port when the test ends.
func RegisterPortCleanup(t *testing.T, port int) {
	t.Helper()
	t.Cleanup(func() { KillProcessOnPort(t, port) })
}

// KillProcessOnPort kills any process listening on the given TCP port.
// Uses lsof — Unix/macOS only, which is sufficient for CI.
func KillProcessOnPort(t *testing.T, port int) {
	t.Helper()
	out, err := exec.Command("lsof", "-t", "-i", fmt.Sprintf(":%d", port)).Output()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		return
	}
	for _, pidStr := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Kill()
		}
	}
}
