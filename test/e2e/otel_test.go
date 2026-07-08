//go:build integration

package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

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
