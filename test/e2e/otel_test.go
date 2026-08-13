//go:build integration

package e2e_test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel"
	"github.com/dynatrace-oss/dtwiz/test/integration"
	"github.com/dynatrace-oss/dtwiz/test/integration/grail"
)

// otelCase describes a single language's OTel auto-instrumentation test.
type otelCase struct {
	lang      string
	fixture   string // subdir under test/fixtures/
	port      int
	portEnv   string // env var the fixture app reads for its port
	skipCheck func(t *testing.T)
	install   func(env *integration.TestEnv, appDir, svcName string) error
}

func TestOTelAutoInstrumentation(t *testing.T) {
	integration.Parallelize(t)

	cases := []otelCase{
		{
			lang:    "python",
			fixture: "python-flask",
			port:    18080,
			portEnv: "TEST_FLASK_APP_PORT",
			skipCheck: func(t *testing.T) {
				if _, err := exec.LookPath("python3"); err != nil {
					t.Skip("python3 not found in PATH")
				}
			},
			install: func(env *integration.TestEnv, appDir, svcName string) error {
				return otel.InstallOtelPython(env.EnvURL, env.ClassicToken, env.PlatformToken, svcName, appDir, false)
			},
		},
		{
			lang:    "node",
			fixture: "node-http",
			port:    18082,
			portEnv: "TEST_NODE_APP_PORT",
			skipCheck: func(t *testing.T) {
				if _, err := exec.LookPath("node"); err != nil {
					t.Skip("node not found in PATH")
				}
				if _, err := exec.LookPath("npm"); err != nil {
					t.Skip("npm not found in PATH")
				}
			},
			install: func(env *integration.TestEnv, appDir, svcName string) error {
				// Execute() checks that node_modules/ exists before launching.
				// The fixture has no dependencies, so create the directory directly
				// rather than relying on npm install (npm v7+ skips it when there's nothing to install).
				if err := os.MkdirAll(filepath.Join(appDir, "node_modules"), 0755); err != nil {
					return err
				}
				return otel.InstallOtelNode(env.EnvURL, env.ClassicToken, env.PlatformToken, svcName, appDir, false)
			},
		},
		{
			lang:    "java",
			fixture: "java-maven",
			port:    18081,
			portEnv: "TEST_JAVA_APP_PORT",
			skipCheck: func(t *testing.T) {
				if _, err := exec.LookPath("java"); err != nil {
					t.Skip("java not found in PATH")
				}
				if _, err := exec.LookPath("mvn"); err != nil {
					t.Skip("mvn not found in PATH")
				}
			},
			install: func(env *integration.TestEnv, appDir, svcName string) error {
				// Build the fat JAR first so detectJavaEntrypoints finds java -jar
				// instead of falling back to mvn exec:java.
				out, err := exec.Command("mvn", "clean", "package", "-DskipTests", "-f", filepath.Join(appDir, "pom.xml")).CombinedOutput()
				if err != nil {
					return fmt.Errorf("mvn build failed: %w\n%s", err, out)
				}
				return otel.InstallOtelJava(env.EnvURL, env.ClassicToken, svcName, appDir, false)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			t.Parallel()
			runOTelTest(t, tc)
		})
	}
}

func runOTelTest(t *testing.T, tc otelCase) {
	t.Helper()
	tc.skipCheck(t)

	env := integration.SetupIntegration(t)
	t.Logf("test ID: %s", env.TestID)

	svcName := integration.ServiceName(env, tc.lang)

	t.Logf("preparing fixture %q for service %q", tc.fixture, svcName)
	appDir := integration.PrepareFixture(t, env, tc.fixture, svcName)

	prev, wasSet := os.LookupEnv(tc.portEnv)
	os.Setenv(tc.portEnv, strconv.Itoa(tc.port))
	t.Cleanup(func() {
		if wasSet {
			os.Setenv(tc.portEnv, prev)
		} else {
			os.Unsetenv(tc.portEnv)
		}
	})

	t.Logf("installing OTel instrumentation (lang: %s, service: %s)", tc.lang, svcName)
	if err := tc.install(env, appDir, svcName); err != nil {
		t.Fatalf("install: %v", err)
	}

	integration.RegisterPortCleanup(t, tc.port)

	t.Logf("waiting for app on port %d", tc.port)
	integration.WaitForPort(t, tc.port, 10*time.Second)

	t.Logf("triggering request on port %d", tc.port)
	integration.TriggerRequestOnPort(t, tc.port)

	t.Logf("waiting for traces in Grail (service: %q)", svcName)
	traces := grail.RequireTraces(t, env.Client, svcName,
		grail.WithTimeout(180*time.Second),
		grail.WithInterval(20*time.Second),
	)
	t.Logf("found %d trace(s) for service %q", len(traces), svcName)
}

// TestOTelHostMonitoring installs the OTel Collector with the experimental
// (host monitoring) flag enabled, waits for system.cpu.utilization metrics to
// arrive in Dynatrace via the OTLP pipeline, then uninstalls the collector.
//
// Required env vars: TEST_DT_ENVIRONMENT, TEST_DT_PLATFORM_TOKEN.
func TestOTelHostMonitoring(t *testing.T) {
	env := integration.SetupIntegration(t)

	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)

	// AutoConfirm skips all confirmation prompts inside InstallOtelCollectorOnly.
	origAC := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = origAC })

	t.Log("installing OTel Collector with host monitoring (--experimental)")
	if err := otel.InstallOtelCollectorOnly(env.EnvURL, env.ClassicToken, env.PlatformToken, false); err != nil {
		t.Fatalf("InstallOtelCollectorOnly: %v", err)
	}

	// Uninstall the collector when the test ends so the binary and config are
	// removed regardless of pass/fail. AutoConfirm is still true here.
	t.Cleanup(func() {
		t.Log("uninstalling OTel Collector")
		if err := otel.UninstallOtelCollector(env.EnvURL, env.PlatformToken, false); err != nil {
			t.Logf("warning: UninstallOtelCollector: %v", err)
		}
	})

	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("os.Hostname: %v", err)
	}

	// The hostmetrics/10s receiver scrapes every 10 s; allow generous time for
	// the first data to propagate through the collector and be ingested by Grail.
	t.Logf("waiting for host metrics in Dynatrace (host: %q)", hostname)
	records := grail.RequireHostMetrics(t, env.Client, hostname,
		grail.WithTimeout(3*time.Minute),
		grail.WithInterval(15*time.Second),
	)
	t.Logf("received %d metric record(s) for host %q", len(records), hostname)
}

// TestOTelInstallAvoidsOccupiedPorts occupies the collector's default ports
// with decoys bound the same way otel.tmpl binds them (0.0.0.0 for
// otlp/health_check, "localhost" for the Prometheus reader) and asserts
// install still succeeds by picking different ports.
//
// Not marked parallel: like TestOTelHostMonitoring, it installs on the
// collector's real default ports and must not race another test doing the
// same.
//
// Required env vars: TEST_DT_ENVIRONMENT, TEST_DT_PLATFORM_TOKEN.
func TestOTelInstallAvoidsOccupiedPorts(t *testing.T) {
	env := integration.SetupIntegration(t)

	var listeners []net.Listener
	occupy := func(addr string) {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.Skipf("cannot occupy %s for this test: %v", addr, err)
		}
		listeners = append(listeners, l)
	}
	occupy("0.0.0.0:4317")
	occupy("0.0.0.0:4318")
	occupy("0.0.0.0:13133")
	occupy("localhost:8888")
	t.Cleanup(func() {
		for _, l := range listeners {
			l.Close()
		}
	})

	// AutoConfirm skips all confirmation prompts inside InstallOtelCollectorOnly.
	origAC := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = origAC })

	t.Log("installing OTel Collector with its default ports occupied by decoy listeners")
	err := otel.InstallOtelCollectorOnly(env.EnvURL, env.ClassicToken, env.PlatformToken, false)

	// Uninstall regardless of the assertion below, best-effort: the install may
	// have partially completed even on failure.
	t.Cleanup(func() {
		t.Log("uninstalling OTel Collector")
		if uerr := otel.UninstallOtelCollector(env.EnvURL, env.PlatformToken, false); uerr != nil {
			t.Logf("warning: UninstallOtelCollector: %v", uerr)
		}
	})

	if err != nil {
		t.Fatalf("InstallOtelCollectorOnly failed with default ports occupied (port allocation regression): %v", err)
	}
}
