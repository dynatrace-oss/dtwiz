package otel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

const testEndpoint = "http://127.0.0.1:4318"

func newGoProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "shop")
	if files == nil {
		files = map[string]string{}
	}
	if _, ok := files["go.mod"]; !ok {
		files["go.mod"] = "module example.com/shop\n\ngo 1.22\n"
	}
	writeTree(t, root, files)
	return root
}

func stubGoToolchain(t *testing.T, present bool) {
	t.Helper()
	orig := goLookPath
	goLookPath = func(string) (string, error) {
		if present {
			return "/usr/bin/go", nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() { goLookPath = orig })
}

func goDetected(path string) detectedProject {
	return detectedProject{ScannedProject: ScannedProject{Path: path, Markers: []string{"go.mod"}}, Runtime: "Go"}
}

func TestBuildGoInstrumentationPlan_Services(t *testing.T) {
	stubGoToolchain(t, true)
	root := newGoProject(t, map[string]string{"cmd/api/main.go": mainSrc, "cmd/worker/main.go": mainSrc})

	plan := buildGoInstrumentationPlan(goDetected(root), testEndpoint)
	if plan == nil || !plan.Automatic || plan.ManualReason != "" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if len(plan.Services) != 2 || plan.Services[0].Method != goMethodOtelc {
		t.Fatalf("services = %#v", plan.Services)
	}
	want := filepath.Join(goOtelDir(root), "bin", goBinaryName("api"))
	if plan.Services[0].BinPath != want {
		t.Fatalf("BinPath = %q, want %q", plan.Services[0].BinPath, want)
	}
}

func TestBuildGoInstrumentationPlan_LibraryOnly(t *testing.T) {
	stubGoToolchain(t, true)
	root := newGoProject(t, map[string]string{"lib.go": "package lib\n"})
	var plan *GoInstrumentationPlan
	out := helpers.CaptureStdout(t, func() { plan = buildGoInstrumentationPlan(goDetected(root), testEndpoint) })
	if plan != nil || !strings.Contains(out, "nothing to instrument") {
		t.Fatalf("plan=%v out=%q", plan, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".otel")); !os.IsNotExist(err) {
		t.Fatal("nothing must be created for a library")
	}
}

func TestBuildGoInstrumentationPlan_SkipsModuleWithSDK(t *testing.T) {
	stubGoToolchain(t, true)
	root := newGoProject(t, map[string]string{
		"go.mod":          "module x\n\nrequire go.opentelemetry.io/otel/sdk v1.30.0\n",
		"cmd/api/main.go": mainSrc, "cmd/worker/main.go": mainSrc,
	})
	var plan *GoInstrumentationPlan
	out := helpers.CaptureStdout(t, func() { plan = buildGoInstrumentationPlan(goDetected(root), testEndpoint) })
	if plan != nil {
		t.Fatal("all services are skipped: no plan expected")
	}
	for _, want := range []string{"api", "worker", testEndpoint, "Skipping"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output should contain %q:\n%s", want, out)
		}
	}
}

func TestBuildGoInstrumentationPlan_IndirectSDKNotSkipped(t *testing.T) {
	stubGoToolchain(t, true)
	root := newGoProject(t, map[string]string{
		"go.mod":  "module x\n\nrequire go.opentelemetry.io/otel/sdk v1.30.0 // indirect\n",
		"main.go": mainSrc,
	})
	if plan := buildGoInstrumentationPlan(goDetected(root), testEndpoint); plan == nil {
		t.Fatal("an indirect SDK requirement must not skip the module")
	}
}

func TestBuildGoInstrumentationPlan_Workspace(t *testing.T) {
	stubGoToolchain(t, true)
	root := newGoProject(t, map[string]string{"main.go": mainSrc})
	if findGoWork(root) != "" {
		t.Skip("go.work above temp dir")
	}

	t.Run("none", func(t *testing.T) {
		if plan := buildGoInstrumentationPlan(goDetected(root), testEndpoint); plan.ManualReason != "" {
			t.Fatalf("unexpected manual plan: %s", plan.ManualReason)
		}
	})
	t.Run("go.work at root", func(t *testing.T) {
		writeTree(t, root, map[string]string{"go.work": "go 1.22\n"})
		plan := buildGoInstrumentationPlan(goDetected(root), testEndpoint)
		if plan == nil || plan.ManualReason != goReasonWork || plan.Services[0].Method != goMethodManual {
			t.Fatalf("plan = %#v", plan)
		}
	})
	t.Run("go.work in parent", func(t *testing.T) {
		parent := t.TempDir()
		writeTree(t, parent, map[string]string{"go.work": "go 1.22\n", "svc/go.mod": "module s\n", "svc/main.go": mainSrc})
		plan := buildGoInstrumentationPlan(goDetected(filepath.Join(parent, "svc")), testEndpoint)
		if plan == nil || plan.ManualReason != goReasonWork {
			t.Fatalf("plan = %#v", plan)
		}
	})
}

func TestBuildGoInstrumentationPlan_NoToolchain(t *testing.T) {
	stubGoToolchain(t, false)
	root := newGoProject(t, map[string]string{"main.go": mainSrc})
	plan := buildGoInstrumentationPlan(goDetected(root), testEndpoint)
	if plan == nil || plan.ManualReason != goReasonNoTool {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestCreateRuntimePlan_GoFlag(t *testing.T) {
	stubGoToolchain(t, true)
	root := newGoProject(t, map[string]string{"main.go": mainSrc})

	setGoFlag(t, false)
	plan := createRuntimePlan(goDetected(root), 4318, "", "", "")
	if p, ok := plan.(*GoInstrumentationPlan); !ok || p.Automatic {
		t.Fatalf("flag off must keep the legacy plan, got %#v", plan)
	}

	setGoFlag(t, true)
	plan = createRuntimePlan(goDetected(root), 4318, "", "", "")
	if p, ok := plan.(*GoInstrumentationPlan); !ok || !p.Automatic || p.CollectorEndpoint != testEndpoint {
		t.Fatalf("flag on must build the automatic plan, got %#v", plan)
	}

	lib := newGoProject(t, map[string]string{"lib.go": "package lib\n"})
	if plan := createRuntimePlan(goDetected(lib), 4318, "", "", ""); plan != nil {
		t.Fatalf("expected nil interface for a library, got %#v", plan)
	}
}

func TestPrintPlanSteps_Automatic(t *testing.T) {
	root := newGoProject(t, map[string]string{"cmd/api/main.go": mainSrc, "cmd/worker/main.go": mainSrc})
	stubGoToolchain(t, true)
	stubOtelcLookup(t, nil)
	plan := buildGoInstrumentationPlan(goDetected(root), testEndpoint)
	plan.Services[1].Method = goMethodClientOnly

	out := helpers.CaptureStdout(t, plan.PrintPlanSteps)
	for _, want := range []string{root, "example.com/shop", "api", "cmd/api", "compile-time", "worker", "client-only",
		"will be installed", ".opentelemetry", "not modified", "network connection", "outgoing HTTP"} {
		if !strings.Contains(out, want) {
			t.Fatalf("preview should contain %q:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".otel")); !os.IsNotExist(err) {
		t.Fatal("preview (dry-run) must not create files")
	}
}

func TestPrintPlanSteps_ShowsInstalledOtelcVersion(t *testing.T) {
	root := newGoProject(t, map[string]string{"main.go": mainSrc})
	stubGoToolchain(t, true)
	stubOtelcLookup(t, &otelcTool{Path: "/x/otelc", Version: "v1.2.0"})
	plan := buildGoInstrumentationPlan(goDetected(root), testEndpoint)

	out := helpers.CaptureStdout(t, plan.PrintPlanSteps)
	if !strings.Contains(out, "otelc v1.2.0") {
		t.Fatalf("preview should name the otelc version:\n%s", out)
	}
}

func TestPrintPlanSteps_Manual(t *testing.T) {
	root := newGoProject(t, map[string]string{"main.go": mainSrc})
	stubGoToolchain(t, false)
	plan := buildGoInstrumentationPlan(goDetected(root), testEndpoint)

	out := helpers.CaptureStdout(t, plan.PrintPlanSteps)
	for _, want := range []string{"manual", goReasonNoTool, "OTel Collector is still installed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("manual preview should contain %q:\n%s", want, out)
		}
	}
}

func stubOtelcLookup(t *testing.T, tool *otelcTool) {
	t.Helper()
	orig := goFindOtelcFn
	goFindOtelcFn = func() (*otelcTool, bool) { return tool, tool != nil }
	t.Cleanup(func() { goFindOtelcFn = orig })
}

// executeHarness stubs every side effect of Execute.
type executeHarness struct {
	otelcErr      error
	otelcFails    map[string]bool
	fallbackFails map[string]bool
	launchFails   map[string]bool
	built         []string
	launched      []string
	env           map[string][]string
	stopped       []int
}

func newHarness(t *testing.T) *executeHarness {
	t.Helper()
	h := &executeHarness{otelcFails: map[string]bool{}, fallbackFails: map[string]bool{}, launchFails: map[string]bool{}, env: map[string][]string{}}

	origEnsure, origBuild, origPrep, origStop, origList, origLaunch, origSummary, origRunner :=
		goEnsureOtelcFn, goBuildOtelcFn, goPrepareFallbackFn, goStopProcessesFn, goListProcessesFn, goLaunchFn, goSummaryFn, goBuildRunner
	t.Cleanup(func() {
		goEnsureOtelcFn, goBuildOtelcFn, goPrepareFallbackFn, goStopProcessesFn, goListProcessesFn, goLaunchFn, goSummaryFn, goBuildRunner =
			origEnsure, origBuild, origPrep, origStop, origList, origLaunch, origSummary, origRunner
	})

	goEnsureOtelcFn = func() (*otelcTool, error) {
		if h.otelcErr != nil {
			return nil, h.otelcErr
		}
		return &otelcTool{Path: "/x/otelc", Version: "v1.2.0"}, nil
	}
	goBuildOtelcFn = func(ctx context.Context, tool *otelcTool, module string, svc GoService, bin string) error {
		if h.otelcFails[svc.Name] {
			return errors.New("otelc build failed")
		}
		h.built = append(h.built, "otelc:"+svc.Name)
		return nil
	}
	goBuildRunner = func(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
		// client-only builds go through goFallback → record by -o target.
		for i, a := range args {
			if a == "-o" && i+1 < len(args) {
				svc := strings.TrimSuffix(filepath.Base(args[i+1]), ".exe")
				if h.fallbackFails[svc] {
					return "", errors.New("fallback build failed")
				}
				h.built = append(h.built, "client:"+svc)
			}
		}
		return "", nil
	}
	goStopProcessesFn = func(pids []int) { h.stopped = append(h.stopped, pids...) }
	goListProcessesFn = func() []DetectedProcess { return nil }
	goLaunchFn = func(svc *GoServicePlan, module string, env []string) (*ManagedProcess, error) {
		if h.launchFails[svc.Name] {
			return nil, errors.New("exec failed")
		}
		h.launched = append(h.launched, svc.Name)
		h.env[svc.Name] = env
		return &ManagedProcess{Name: svc.Name, PID: 1}, nil
	}
	goSummaryFn = func(procs []*ManagedProcess, d time.Duration) ([]string, []int) {
		var names []string
		var pids []int
		for _, p := range procs {
			names = append(names, p.Name)
			pids = append(pids, p.PID)
		}
		return names, pids
	}
	return h
}

func newTwoServicePlan(t *testing.T) (*GoInstrumentationPlan, string) {
	t.Helper()
	root := newGoProject(t, map[string]string{"cmd/api/main.go": mainSrc, "cmd/worker/main.go": mainSrc})
	stubGoToolchain(t, true)
	plan := buildGoInstrumentationPlan(goDetected(root), testEndpoint)
	if plan == nil || len(plan.Services) != 2 {
		t.Fatalf("plan = %#v", plan)
	}
	return plan, root
}

func methods(plan *GoInstrumentationPlan) map[string]goMethod {
	m := map[string]goMethod{}
	for _, s := range plan.Services {
		m[s.Name] = s.Method
	}
	return m
}

func TestExecute_TierSelection(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(h *executeHarness)
		want        map[string]goMethod
		wantBuilt   []string
		wantLaunch  []string
		wantErr     bool
		wantManual  bool
		wantOutputs []string
	}{
		{
			name: "all compile-time",
			want: map[string]goMethod{"api": goMethodOtelc, "worker": goMethodOtelc}, wantBuilt: []string{"otelc:api", "otelc:worker"},
			wantLaunch: []string{"api", "worker"},
		},
		{
			name:  "otelc unavailable → all client-only",
			setup: func(h *executeHarness) { h.otelcErr = &GoVersionTooOldError{Required: "1.26.0"} },
			want:  map[string]goMethod{"api": goMethodClientOnly, "worker": goMethodClientOnly}, wantBuilt: []string{"client:api", "client:worker"},
			wantLaunch: []string{"api", "worker"}, wantOutputs: []string{"upgrade Go to 1.26.0", "outgoing HTTP"},
		},
		{
			name:  "one otelc build fails → that service client-only",
			setup: func(h *executeHarness) { h.otelcFails["worker"] = true },
			want:  map[string]goMethod{"api": goMethodOtelc, "worker": goMethodClientOnly}, wantBuilt: []string{"otelc:api", "client:worker"},
			wantLaunch: []string{"api", "worker"}, wantOutputs: []string{"Outgoing HTTP calls only", "worker"},
		},
		{
			name:  "client-only fails too → manual for that service, others continue",
			setup: func(h *executeHarness) { h.otelcFails["worker"] = true; h.fallbackFails["worker"] = true },
			want:  map[string]goMethod{"api": goMethodOtelc, "worker": goMethodManual}, wantBuilt: []string{"otelc:api"},
			wantLaunch: []string{"api"}, wantManual: true,
		},
		{
			name: "everything fails → manual, not an error",
			setup: func(h *executeHarness) {
				h.otelcErr = errors.New("network")
				h.fallbackFails["api"] = true
				h.fallbackFails["worker"] = true
			},
			want: map[string]goMethod{"api": goMethodManual, "worker": goMethodManual}, wantManual: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			if tt.setup != nil {
				tt.setup(h)
			}
			plan, root := newTwoServicePlan(t)

			var err error
			out := helpers.CaptureStdout(t, func() { err = plan.Execute() })
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v\n%s", err, tt.wantErr, out)
			}
			got := methods(plan)
			for k, v := range tt.want {
				if got[k] != v {
					t.Fatalf("method[%s] = %s, want %s\n%s", k, got[k], v, out)
				}
			}
			if strings.Join(h.built, ",") != strings.Join(tt.wantBuilt, ",") {
				t.Fatalf("built = %v, want %v\n%s", h.built, tt.wantBuilt, out)
			}
			if strings.Join(h.launched, ",") != strings.Join(tt.wantLaunch, ",") {
				t.Fatalf("launched = %v, want %v", h.launched, tt.wantLaunch)
			}
			if tt.wantManual != strings.Contains(out, "go get go.opentelemetry.io/otel") {
				t.Fatalf("manual steps printed = %v, want %v\n%s", !tt.wantManual, tt.wantManual, out)
			}
			for _, w := range tt.wantOutputs {
				if !strings.Contains(out, w) {
					t.Fatalf("output should contain %q:\n%s", w, out)
				}
			}
			if _, statErr := os.Stat(filepath.Join(goOtelDir(root), goMarkerFile)); statErr != nil {
				t.Fatalf("marker file must be written: %v", statErr)
			}
		})
	}
}

func TestExecute_LaunchOutcomes(t *testing.T) {
	t.Run("one failed", func(t *testing.T) {
		h := newHarness(t)
		h.launchFails["worker"] = true
		plan, _ := newTwoServicePlan(t)
		var err error
		helpers.CaptureStdout(t, func() { err = plan.Execute() })
		if err != nil || len(h.launched) != 1 {
			t.Fatalf("err=%v launched=%v: partial failure is not an error", err, h.launched)
		}
	})
	t.Run("none started", func(t *testing.T) {
		h := newHarness(t)
		h.launchFails["api"], h.launchFails["worker"] = true, true
		plan, _ := newTwoServicePlan(t)
		var err error
		helpers.CaptureStdout(t, func() { err = plan.Execute() })
		if err == nil {
			t.Fatal("expected an error when no service started")
		}
	})
}

func TestExecute_StopsReplacedProcessesOnly(t *testing.T) {
	h := newHarness(t)
	plan, root := newTwoServicePlan(t)
	plan.Project.RunningProcessIDs = []int{10, 11, 12, 13}
	goListProcessesFn = func() []DetectedProcess {
		return []DetectedProcess{
			{PID: 10, Command: "go run ./cmd/api"},
			{PID: 11, Command: filepath.Join(root, ".otel", "go", "bin", "worker")},
			{PID: 12, Command: "/usr/local/bin/terraform apply"},
			{PID: 13, Command: filepath.Join(root, "bin", goBinaryName("api"))},
		}
	}
	helpers.CaptureStdout(t, func() { _ = plan.Execute() })
	if got := h.stopped; len(got) != 3 || got[0] != 10 || got[1] != 11 || got[2] != 13 {
		t.Fatalf("stopped = %v, want [10 11 13] (unrelated Go tools must stay running)", got)
	}
}

func TestGoServiceEnv_NoTokensAndLocalCollector(t *testing.T) {
	parent := []string{"PATH=/bin", "MY_CI_SECRET=dt0s16.abc.def", "DT_PLATFORM_TOKEN=dt0s16.secret", "DT_ACCESS_TOKEN=dt0c01.x",
		"OTEL_EXPORTER_OTLP_HEADERS=Authorization=Api-Token%20abc", "OTEL_SERVICE_NAME=old"}
	base := map[string]string{
		"OTEL_SERVICE_NAME": "shop", "OTEL_EXPORTER_OTLP_ENDPOINT": testEndpoint,
		"OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf", "OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE": "delta",
	}
	env := goServiceEnv(parent, base, "api")
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, want := range []string{"\nOTEL_SERVICE_NAME=api\n", "\nOTEL_EXPORTER_OTLP_ENDPOINT=" + testEndpoint + "\n",
		"\nOTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf\n", "\nOTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=delta\n", "\nPATH=/bin\n"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("env should contain %q: %v", strings.TrimSpace(want), env)
		}
	}
	for _, banned := range []string{"DT_PLATFORM_TOKEN", "DT_ACCESS_TOKEN", "OTEL_EXPORTER_OTLP_HEADERS", "dt0s16", "Api-Token"} {
		if strings.Contains(joined, banned) {
			t.Fatalf("env must not contain %q: %v", banned, env)
		}
	}
	if base["OTEL_SERVICE_NAME"] != "shop" {
		t.Fatal("goServiceEnv must not mutate the base map")
	}
}

func TestExecute_LaunchedEnvironment(t *testing.T) {
	h := newHarness(t)
	t.Setenv("DT_PLATFORM_TOKEN", "dt0s16.secret")
	plan, _ := newTwoServicePlan(t)
	helpers.CaptureStdout(t, func() { _ = plan.Execute() })
	joined := strings.Join(h.env["api"], "\n")
	if !strings.Contains(joined, "OTEL_SERVICE_NAME=api") || !strings.Contains(joined, "OTEL_EXPORTER_OTLP_ENDPOINT="+testEndpoint) {
		t.Fatalf("launched env incomplete: %s", joined)
	}
	if strings.Contains(joined, "dt0s16") {
		for _, kv := range h.env["api"] {
			if strings.Contains(kv, "dt0s16") {
				t.Fatalf("no Dynatrace token may reach the service, found in %q", strings.SplitN(kv, "=", 2)[0])
			}
		}
	}
}

func TestExecute_ManualPlan(t *testing.T) {
	h := newHarness(t)
	root := newGoProject(t, map[string]string{"main.go": mainSrc})
	stubGoToolchain(t, false)
	plan := buildGoInstrumentationPlan(goDetected(root), testEndpoint)
	if plan == nil {
		t.Fatal("manual outcomes must still be a plan (no 'Select another project?' prompt)")
	}

	var err error
	out := helpers.CaptureStdout(t, func() { err = plan.Execute() })
	if err != nil {
		t.Fatalf("manual outcome must succeed: %v", err)
	}
	for _, want := range []string{"export OTEL_EXPORTER_OTLP_ENDPOINT=\"" + testEndpoint + "\"", "go get go.opentelemetry.io/otel", "dt-url.net/otel-go", goReasonNoTool} {
		if !strings.Contains(out, want) {
			t.Fatalf("manual steps should contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Authorization") || strings.Contains(out, "OTEL_EXPORTER_OTLP_HEADERS") {
		t.Fatalf("manual steps must not show credentials:\n%s", out)
	}
	if len(h.built)+len(h.launched) != 0 {
		t.Fatal("nothing may be built or launched for a manual plan")
	}
	if _, statErr := os.Stat(filepath.Join(root, ".otel")); !os.IsNotExist(statErr) {
		t.Fatal("manual plan must not create files")
	}
}

func TestGoBinaryName(t *testing.T) {
	want := "api"
	if runtime.GOOS == "windows" {
		want = "api.exe"
	}
	if got := goBinaryName("api"); got != want {
		t.Fatalf("goBinaryName = %q, want %q", got, want)
	}
}

func setGoFlag(t *testing.T, on bool) {
	t.Helper()
	featureflags.SetCLIOverrideForTest(t, featureflags.GoInstrumentation, on)
}
