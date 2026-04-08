package recommender_test

import (
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/recommender"
)

func TestGenerateRecommendations_OneAgentAlreadyRunning(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:        analyzer.PlatformLinux,
		OneAgentRunning: true,
	}
	recs := recommender.GenerateRecommendations(system)
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].Method != recommender.MethodAlreadyInstalled {
		t.Errorf("expected method %q, got %q", recommender.MethodAlreadyInstalled, recs[0].Method)
	}
	if !recs[0].Done {
		t.Error("expected Done=true for already-installed recommendation")
	}
}

func TestGenerateRecommendations_Kubernetes(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:     analyzer.PlatformLinux,
		Orchestrator: analyzer.OrchestratorKubernetes,
		Kubernetes: &analyzer.KubernetesInfo{
			Available:    true,
			Distribution: "GKE",
		},
	}
	recs := recommender.GenerateRecommendations(system)
	if len(recs) == 0 {
		t.Fatal("expected at least one recommendation")
	}
	// OTel is always first; kubernetes should appear in the list.
	found := false
	for _, r := range recs {
		if r.Method == recommender.MethodKubernetes {
			found = true
		}
	}
	if !found {
		t.Error("expected kubernetes recommendation")
	}
}

func TestGenerateRecommendations_DockerOnly(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformLinux,
		ContainerRuntime: analyzer.ContainerRuntimeDocker,
		Docker:           &analyzer.DockerInfo{Available: true},
		Orchestrator:     analyzer.OrchestratorNone,
	}
	recs := recommender.GenerateRecommendations(system)
	found := false
	for _, r := range recs {
		if r.Method == recommender.MethodDocker {
			found = true
		}
	}
	if !found {
		t.Error("expected docker recommendation")
	}
}

func TestGenerateRecommendations_BareMetal(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformLinux,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
	}
	recs := recommender.GenerateRecommendations(system)
	found := false
	for _, r := range recs {
		if r.Method == recommender.MethodOneAgent {
			found = true
		}
	}
	if !found {
		t.Error("expected oneagent recommendation for bare metal Linux")
	}
}

func TestGenerateRecommendations_macOS(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformDarwin,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
	}
	recs := recommender.GenerateRecommendations(system)
	// macOS platform limitations are shown inline in the system analysis, not as a recommendation.
	for _, r := range recs {
		if r.Method == recommender.MethodNotSupported {
			t.Error("macOS not-supported entry should not appear in recommendations")
		}
	}
}

func TestFormatRecommendations_Empty(t *testing.T) {
	result := recommender.FormatRecommendations(nil)
	if result == "" {
		t.Error("FormatRecommendations(nil) should not return empty string")
	}
}

func TestFormatRecommendations_NonEmpty(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformLinux,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
	}
	recs := recommender.GenerateRecommendations(system)
	result := recommender.FormatRecommendations(recs)
	if result == "" {
		t.Error("FormatRecommendations should not return empty string for non-empty recs")
	}
}

func TestGenerateRecommendations_OtelCollectorNotRunning(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformLinux,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
		OtelCollector:    false,
	}
	recs := recommender.GenerateRecommendations(system)
	found := false
	for _, r := range recs {
		if r.Method == recommender.MethodOtelCollector {
			found = true
		}
	}
	if !found {
		t.Error("expected otel-collector recommendation even when no collector is running")
	}
}

func TestGenerateRecommendations_OtelCollectorRunning(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformLinux,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
		OtelCollector:    true,
		OtelConfigPath:   "/etc/otel/config.yaml",
	}
	recs := recommender.GenerateRecommendations(system)
	foundUpdate := false
	foundInstall := false
	for _, r := range recs {
		if r.Method == recommender.MethodOtelUpdate {
			foundUpdate = true
		}
		if r.Method == recommender.MethodOtelCollector {
			foundInstall = true
		}
	}
	if !foundUpdate {
		t.Error("expected otel-update recommendation when collector is already running")
	}
	if !foundInstall {
		t.Error("expected otel-collector install option even when collector is already running")
	}
}

func TestGenerateRecommendations_Azure(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformLinux,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
		Azure: &analyzer.AzureInfo{
			Available:      true,
			SubscriptionID: "sub-123",
		},
	}
	recs := recommender.GenerateRecommendations(system)
	found := false
	for _, r := range recs {
		if r.Method == recommender.MethodAzure {
			found = true
			if !r.ComingSoon {
				t.Error("expected ComingSoon=true for Azure recommendation")
			}
		}
	}
	if !found {
		t.Error("expected azure recommendation when Azure is available")
	}
}

func TestGenerateRecommendations_GCP(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformLinux,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
		GCP: &analyzer.GCPInfo{
			Available: true,
			ProjectID: "my-project",
		},
	}
	recs := recommender.GenerateRecommendations(system)
	found := false
	for _, r := range recs {
		if r.Method == recommender.MethodGCP {
			found = true
			if !r.ComingSoon {
				t.Error("expected ComingSoon=true for GCP recommendation")
			}
		}
	}
	if !found {
		t.Error("expected gcp recommendation when GCP is available")
	}
}

func TestGenerateRecommendations_macOSGetsOtel(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformDarwin,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
	}
	recs := recommender.GenerateRecommendations(system)
	found := false
	for _, r := range recs {
		if r.Method == recommender.MethodOtelCollector {
			found = true
		}
	}
	if !found {
		t.Error("expected otel-collector recommendation on macOS")
	}
}
