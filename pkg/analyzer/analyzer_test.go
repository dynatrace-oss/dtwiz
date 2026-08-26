package analyzer_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
)

func TestAnalyzeSystem_BasicFields(t *testing.T) {
	info, err := analyzer.AnalyzeSystem()
	if err != nil {
		t.Fatalf("AnalyzeSystem() returned error: %v", err)
	}

	switch runtime.GOOS {
	case "linux":
		if info.Platform != analyzer.PlatformLinux {
			t.Errorf("expected platform %q, got %q", analyzer.PlatformLinux, info.Platform)
		}
	case "darwin":
		if info.Platform != analyzer.PlatformDarwin {
			t.Errorf("expected platform %q, got %q", analyzer.PlatformDarwin, info.Platform)
		}
	case "windows":
		if info.Platform != analyzer.PlatformWindows {
			t.Errorf("expected platform %q, got %q", analyzer.PlatformWindows, info.Platform)
		}
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("expected arch %q, got %q", runtime.GOARCH, info.Arch)
	}
	if s := info.Summary(); s == "" {
		t.Error("Summary() returned empty string")
	}
}

func TestSystemInfoSummary_WithDetectedCloudAndRuntimeSignals(t *testing.T) {
	t.Parallel()

	info := &analyzer.SystemInfo{
		Hostname:         "host-1",
		Platform:         analyzer.PlatformLinux,
		Arch:             "amd64",
		ContainerRuntime: analyzer.ContainerRuntimeDocker,
		Orchestrator:     analyzer.OrchestratorKubernetes,
		Docker: &analyzer.DockerInfo{
			Available:             true,
			ServerVersion:         "27.0.0",
			Variant:               "Docker Desktop",
			RunningContainerCount: 3,
		},
		Kubernetes: &analyzer.KubernetesInfo{
			Available:    true,
			Distribution: "EKS Bottlerocket",
			Context:      "prod",
			NodeCount:    4,
		},
		OtelCollector:   true,
		OtelBinaryPath:  "/opt/otelcol",
		OneAgentRunning: true,
		AWS: &analyzer.AWSInfo{
			Available: true,
			AccountID: "111111111111",
			Region:    "eu-west-1",
			Services:  []analyzer.AWSService{{Name: "Lambda", Count: 2}, {Name: "RDS", Count: 1}},
		},
		Azure: &analyzer.AzureInfo{
			Available:      true,
			SubscriptionID: "sub-123",
			Services:       []analyzer.AzureService{{Name: "VMs", Count: 5}},
		},
		GCP: &analyzer.GCPInfo{
			Available: true,
			ProjectID: "project-123",
			Services:  []analyzer.GCPService{{Name: "Cloud Run", Count: 6}},
		},
		Services: []string{"nginx", "redis"},
	}

	summary := info.Summary()
	for _, want := range []string{
		"Linux  amd64  (host-1)",
		"/opt/otelcol  (running)",
		"Docker Desktop  version 27.0.0, 3 containers running",
		"EKS Bottlerocket  context=prod  nodes=4",
		"account=111111111111  region=eu-west-1  services: Lambda (2), RDS (1)",
		"subscription=sub-123  services: VMs (5)",
		"project=project-123  services: Cloud Run (6)",
		"This host:",
		"OneAgent:",
		"running",
		"Runtimes:",
		"nginx, redis",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("Summary() missing %q:\n%s", want, summary)
		}
	}
}

func TestSystemInfoSummary_WithMissingClouds(t *testing.T) {
	t.Parallel()

	info := &analyzer.SystemInfo{
		Hostname: "host-2",
		Platform: analyzer.PlatformDarwin,
		Arch:     "arm64",
		GCP:      &analyzer.GCPInfo{Authenticated: true},
	}

	summary := info.Summary()
	for _, want := range []string{
		"macOS  arm64  (host-2)",
		"OpenTelemetry:",
		"<none>",
		"set a project with 'gcloud config set project PROJECT_ID'",
		"macOS not supported",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("Summary() missing %q:\n%s", want, summary)
		}
	}
}

func TestSystemInfoCloudDetected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		info      analyzer.SystemInfo
		wantAzure bool
		wantGCP   bool
	}{
		{name: "nil clouds"},
		{name: "available clouds", info: analyzer.SystemInfo{Azure: &analyzer.AzureInfo{Available: true}, GCP: &analyzer.GCPInfo{Available: true}}, wantAzure: true, wantGCP: true},
		{name: "configured but unavailable", info: analyzer.SystemInfo{Azure: &analyzer.AzureInfo{}, GCP: &analyzer.GCPInfo{}}, wantAzure: false, wantGCP: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.info.AzureDetected(); got != tt.wantAzure {
				t.Fatalf("AzureDetected() = %v, want %v", got, tt.wantAzure)
			}
			if got := tt.info.GCPDetected(); got != tt.wantGCP {
				t.Fatalf("GCPDetected() = %v, want %v", got, tt.wantGCP)
			}
		})
	}
}
