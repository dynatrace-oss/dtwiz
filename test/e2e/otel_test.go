//go:build integration

package e2e_test

import (
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/test/integration"
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
				installer.AutoConfirm = true
				defer func() { installer.AutoConfirm = false }()
				return installer.InstallOtelPython(env.EnvURL, env.AccessToken, env.PlatformToken, svcName, appDir, false)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
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

	t.Setenv(tc.portEnv, strconv.Itoa(tc.port))

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
	traces := integration.RequireTraces(t, env.Client, svcName,
		integration.WithTimeout(180*time.Second),
		integration.WithInterval(5*time.Second),
	)
	t.Logf("found %d trace(s) for service %q", len(traces), svcName)
}
