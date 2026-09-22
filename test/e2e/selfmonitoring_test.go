//go:build integration

package e2e_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/test/integration"
	"github.com/dynatrace-oss/dtwiz/test/integration/grail"
)

// TestSelfMonitoringInstrumentation verifies that each instrumented dtwiz command
// emits at least one self-monitoring event that lands in Grail.
//
// Every command fires StepInvoked via root's PersistentPreRun, so that is the
// minimal check. Commands that run indefinitely (watch) are killed after a short
// warmup — long enough for the event goroutine to complete its HTTP call (3s timeout).
func TestSelfMonitoringInstrumentation(t *testing.T) {
	_, testFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(testFile), "..", "..")

	// Compile once so each subtest runs the binary directly.
	// go run spawns a child process; killing go run leaves the grandchild running
	// causing CombinedOutput to block forever for long-running commands like watch.
	binary := filepath.Join(t.TempDir(), "dtwiz-test")
	buildOut, err := exec.Command("go", "build", "-o", binary, repoRoot).CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, buildOut)
	}

	cases := []struct {
		cmd       string        // dtwiz sub-command; also the expected event.name suffix
		args      []string      // full args passed to the binary
		stdin     string        // optional stdin (for interactive prompts)
		killAfter time.Duration // if >0, kill the process after this delay
	}{
		{cmd: "status", args: []string{"status"}},
		{cmd: "setup", args: []string{"setup", "--dry-run"}, stdin: "1\n"},
		{cmd: "watch", args: []string{"watch"}, killAfter: 6 * time.Second},
	}

	env := integration.SetupIntegration(t)

	pollOpts := []grail.PollOption{
		grail.WithTimeout(3 * time.Minute),
		grail.WithInterval(10 * time.Second),
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.cmd, func(t *testing.T) {
			t.Parallel()

			startTime := time.Now()

			var cmd *exec.Cmd
			if tc.killAfter > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), tc.killAfter)
				defer cancel()
				cmd = exec.CommandContext(ctx, binary, tc.args...)
			} else {
				cmd = exec.Command(binary, tc.args...)
			}

			if tc.stdin != "" {
				cmd.Stdin = strings.NewReader(tc.stdin)
			}
			cmd.Env = append(os.Environ(),
				"DT_ENVIRONMENT="+env.EnvURL,
				"DT_PLATFORM_TOKEN="+env.PlatformToken,
				"DTWIZ_SELF_MONITORING_POC=true",
			)

			out, runErr := cmd.CombinedOutput()
			t.Logf("output:\n%s", out)

			// For commands killed by context (watch), non-zero exit is expected.
			if tc.killAfter == 0 && runErr != nil {
				t.Fatalf("dtwiz %s failed: %v", tc.cmd, runErr)
			}

			q := grail.SelfMonitoringQuery{
				EventName: "dtwiz " + tc.cmd,
				Step:      "invoked",
				From:      startTime,
			}
			t.Logf("waiting for dtwiz %s StepInvoked in Grail", tc.cmd)
			grail.RequireSelfMonitoringEvent(t, env.Client, q, pollOpts...)
		})
	}
}
