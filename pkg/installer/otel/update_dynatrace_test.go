package otel

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

func TestPortsFromConfig_ReturnsDefaults_OnMissingFile(t *testing.T) {
	grpc, http, metrics, hc := portsFromConfig("/nonexistent/path/config.yaml")
	if grpc != 4317 || http != 4318 || metrics != 8888 || hc != 13133 {
		t.Errorf("expected canonical defaults, got grpc=%d http=%d metrics=%d hc=%d", grpc, http, metrics, hc)
	}
}

func TestPortsFromConfig_ParsesAllPorts(t *testing.T) {
	cfg := `
extensions:
  health_check:
    endpoint: 0.0.0.0:13200
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:5317
      http:
        endpoint: 0.0.0.0:5318
service:
  telemetry:
    metrics:
      readers:
        - pull:
            exporter:
              prometheus:
                host: localhost
                port: 9999
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp_http]
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	grpc, http, metrics, hc := portsFromConfig(path)
	if grpc != 5317 {
		t.Errorf("grpc: want 5317, got %d", grpc)
	}
	if http != 5318 {
		t.Errorf("http: want 5318, got %d", http)
	}
	if metrics != 9999 {
		t.Errorf("metrics: want 9999, got %d", metrics)
	}
	if hc != 13200 {
		t.Errorf("health_check: want 13200, got %d", hc)
	}
}

// ── updateDynatraceCollector prerequisite tests ───────────────────────────────

func minimalDynatraceConfig() string {
	return "# minimal config for tests\nreceivers:\n  otlp: {}\n"
}

func writeDynatraceConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(minimalDynatraceConfig()), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func captureUpdateOutput(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	oldStdout := os.Stdout
	oldColorOut := color.Output
	oldColorErr := color.Error
	os.Stdout = w
	color.Output = w
	color.Error = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		color.Output = oldColorOut
		color.Error = oldColorErr
	})

	// Drain the pipe concurrently. Without this, fn() blocks once the OS pipe
	// buffer fills (4 KB on Windows, 64 KB on Linux/macOS), causing a deadlock
	// that the test runner reports as a timeout.
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	_ = w.Close()
	<-done
	_ = r.Close()
	return buf.String()
}

func stubDynatraceExtensionPreview(t *testing.T) {
	t.Helper()
	orig := buildExtensionActivationPreviewFn
	t.Cleanup(func() { buildExtensionActivationPreviewFn = orig })
	buildExtensionActivationPreviewFn = func(_, _ string) (installer.ExtensionStatus, error) {
		return installer.ExtensionInstalledActive, nil
	}
}

func stubGrailRoutePlans(t *testing.T) *fakeGrailClient {
	t.Helper()
	fc := happyFakeClient()
	orig := buildGrailRoutePlansFn
	t.Cleanup(func() { buildGrailRoutePlansFn = orig })
	buildGrailRoutePlansFn = func(_, _ string) (grailRouteClient, []grailSignalPlan, error) {
		plans, _ := buildGrailPlans(context.Background(), fc)
		return fc, plans, nil
	}
	return fc
}

func stubWaitForPipelines(t *testing.T) *bool {
	t.Helper()
	called := false
	orig := waitForGrailPipelinesFn
	t.Cleanup(func() { waitForGrailPipelinesFn = orig })
	waitForGrailPipelinesFn = func(_ context.Context, _ grailRouteClient, _ func(time.Duration)) error {
		called = true
		return nil
	}
	return &called
}

func runUpdateDynatrace(t *testing.T, configPath, platformTok string, dryRun bool) error {
	t.Helper()
	return updateDynatraceCollector(configPath, nil, "https://env.example.com", "tok", platformTok, dryRun)
}

func TestUpdateDynatraceCollector_ShowsPreviewSections(t *testing.T) {
	configPath := writeDynatraceConfig(t)
	stubDynatraceExtensionPreview(t)
	stubGrailRoutePlans(t)

	output := captureUpdateOutput(t, func() {
		_ = runUpdateDynatrace(t, configPath, "dt0s16.test", true /* dryRun */)
	})

	if !strings.Contains(output, "OpenTelemetry Host Monitoring extension") {
		t.Errorf("expected extension section in preview, got:\n%s", output)
	}
	if !strings.Contains(output, "OpenPipeline") {
		t.Errorf("expected OpenPipeline route plan section in preview, got:\n%s", output)
	}
}

func TestUpdateDynatraceCollector_NoToken_SkipsPreviewSections(t *testing.T) {
	configPath := writeDynatraceConfig(t)

	extCalled := false
	orig := buildExtensionActivationPreviewFn
	t.Cleanup(func() { buildExtensionActivationPreviewFn = orig })
	buildExtensionActivationPreviewFn = func(_, _ string) (installer.ExtensionStatus, error) {
		extCalled = true
		return 0, nil
	}

	grailCalled := false
	origGrail := buildGrailRoutePlansFn
	t.Cleanup(func() { buildGrailRoutePlansFn = origGrail })
	buildGrailRoutePlansFn = func(_, _ string) (grailRouteClient, []grailSignalPlan, error) {
		grailCalled = true
		return nil, nil, fmt.Errorf("should not be called")
	}

	captureUpdateOutput(t, func() {
		_ = runUpdateDynatrace(t, configPath, "" /* no token */, true /* dryRun */)
	})

	if extCalled {
		t.Error("expected buildExtensionActivationPreviewFn NOT to be called when platform token is empty")
	}
	if grailCalled {
		t.Error("expected buildGrailRoutePlansFn NOT to be called when platform token is empty")
	}
}

func TestUpdateDynatraceCollector_PostConfirmation(t *testing.T) {
	configPath := writeDynatraceConfig(t)
	stubDynatraceExtensionPreview(t)
	fc := stubGrailRoutePlans(t)
	activationCalled := stubActivation(t)
	pipelineWaitCalled := stubWaitForPipelines(t)

	origAC := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = origAC })

	captureUpdateOutput(t, func() {
		_ = runUpdateDynatrace(t, configPath, "dt0s16.test", false /* not dryRun */)
	})

	if !*activationCalled {
		t.Error("expected activateHostMonitoringExtensionFn to be called after confirmation")
	}
	if !*pipelineWaitCalled {
		t.Error("expected waitForGrailPipelinesFn to be called after confirmation")
	}
	if len(fc.putCalls)+len(fc.createCalls) == 0 {
		t.Error("expected applyGrailPlan to make route create or put calls, but none were recorded")
	}
}

func TestUpdateDynatraceCollector_DryRun_SkipsPostConfirmation(t *testing.T) {
	configPath := writeDynatraceConfig(t)
	stubDynatraceExtensionPreview(t)
	fc := stubGrailRoutePlans(t)
	activationCalled := stubActivation(t)
	pipelineWaitCalled := stubWaitForPipelines(t)

	captureUpdateOutput(t, func() {
		_ = runUpdateDynatrace(t, configPath, "dt0s16.test", true /* dryRun */)
	})

	if *activationCalled {
		t.Error("expected activateHostMonitoringExtensionFn NOT to be called on dry-run")
	}
	if *pipelineWaitCalled {
		t.Error("expected waitForGrailPipelinesFn NOT to be called on dry-run")
	}
	if len(fc.putCalls)+len(fc.createCalls) > 0 {
		t.Error("expected no route create or put calls on dry-run")
	}
}

func TestUpdateDynatraceCollector_ConfigUpToDate_StillRunsPrerequisites(t *testing.T) {
	// Write the exact config that renderOtelTemplate would produce so bytes.Equal is true.
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	cfgData := otelConfigData{
		Endpoint:        strings.TrimRight(installer.APIURL("https://env.example.com"), "/"),
		AuthHeader:      installer.AuthHeader("tok"),
		GRPCPort:        4317,
		HTTPPort:        4318,
		MetricsPort:     8888,
		IncludeJournald: runtime.GOOS == "linux",
		HealthCheckPort: 13133,
	}
	rendered, err := renderOtelTemplate(cfgData)
	if err != nil {
		t.Fatalf("renderOtelTemplate: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(rendered), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stubDynatraceExtensionPreview(t)
	fc := stubGrailRoutePlans(t)
	activationCalled := stubActivation(t)
	pipelineWaitCalled := stubWaitForPipelines(t)

	origAC := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = origAC })

	var runErr error
	captureUpdateOutput(t, func() {
		runErr = updateDynatraceCollector(configPath, nil, "https://env.example.com", "tok", "dt0s16.test", false)
	})

	if runErr != nil {
		t.Errorf("expected success when config is up to date but prerequisites need checking, got: %v", runErr)
	}
	if !*activationCalled {
		t.Error("expected activateHostMonitoringExtensionFn to be called even when config is up to date")
	}
	if !*pipelineWaitCalled {
		t.Error("expected waitForGrailPipelinesFn to be called even when config is up to date")
	}
	if len(fc.putCalls)+len(fc.createCalls) == 0 {
		t.Error("expected applyGrailPlan to make route calls even when config is up to date")
	}
}

func TestUpdateDynatraceCollector_ExtensionPreviewFailure(t *testing.T) {
	configPath := writeDynatraceConfig(t)
	stubGrailRoutePlans(t)

	orig := buildExtensionActivationPreviewFn
	t.Cleanup(func() { buildExtensionActivationPreviewFn = orig })
	buildExtensionActivationPreviewFn = func(_, _ string) (installer.ExtensionStatus, error) {
		return 0, fmt.Errorf("api unavailable")
	}

	var runErr error
	output := captureUpdateOutput(t, func() {
		runErr = runUpdateDynatrace(t, configPath, "dt0s16.test", true /* dryRun */)
	})

	if runErr != nil {
		t.Errorf("expected dry-run to succeed despite preview error, got: %v", runErr)
	}
	// Verify the flow continued past the failed preview check and reached dry-run output.
	if !strings.Contains(output, "dry-run") {
		t.Errorf("expected dry-run output after preview failure, got:\n%s", output)
	}
}
