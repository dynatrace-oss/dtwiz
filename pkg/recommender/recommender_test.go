package recommender_test

import (
	"strings"
	"testing"

	"github.com/fatih/color"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/recommender"
)

func TestGenerateRecommendations_OneAgentAlreadyRunning(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:        analyzer.PlatformLinux,
		OneAgentRunning: true,
	}
	recs := recommender.GenerateRecommendations(system)
	if len(recs) < 2 {
		t.Fatalf("expected at least 2 recommendations (already-installed + otel), got %d", len(recs))
	}
	if recs[0].Method != recommender.MethodAlreadyInstalled {
		t.Errorf("expected first method %q, got %q", recommender.MethodAlreadyInstalled, recs[0].Method)
	}
	if !recs[0].Done {
		t.Error("expected Done=true for already-installed recommendation")
	}
	// OTel should still be recommended alongside OneAgent.
	foundOtel := false
	for _, r := range recs {
		if r.Method == recommender.MethodOtelCollector {
			foundOtel = true
		}
	}
	if !foundOtel {
		t.Error("expected otel recommendation even when OneAgent is running")
	}
}

func TestGenerateRecommendations_OneAgentRunning_NoOneAgentRec(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformLinux,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
		OneAgentRunning:  true,
	}
	recs := recommender.GenerateRecommendations(system)
	for _, r := range recs {
		if r.Method == recommender.MethodOneAgent {
			t.Error("should not recommend OneAgent install when OneAgent is already running")
		}
	}
}

func TestGenerateRecommendations_Kubernetes(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:     analyzer.PlatformLinux,
		Orchestrator: analyzer.OrchestratorKubernetes,
		Kubernetes: &analyzer.KubernetesInfo{
			Available:    true,
			Distribution: analyzer.DistroGKE,
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
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, true)
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

func TestGenerateRecommendations_DockerHiddenWithoutExperimental(t *testing.T) {
	featureflags.SetCLIOverrideForTest(t, featureflags.Experimental, false)
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformLinux,
		ContainerRuntime: analyzer.ContainerRuntimeDocker,
		Docker:           &analyzer.DockerInfo{Available: true},
		Orchestrator:     analyzer.OrchestratorNone,
	}
	recs := recommender.GenerateRecommendations(system)
	for _, r := range recs {
		if r.Method == recommender.MethodDocker {
			t.Error("docker recommendation should not appear without --experimental")
		}
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

func TestFormatRecommendations_AlwaysShowsDemoOption(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformLinux,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
	}
	recs := recommender.GenerateRecommendations(system)
	result := recommender.FormatRecommendations(recs)
	if !strings.Contains(result, "[d]") {
		t.Error("FormatRecommendations should always include the [d] demo option regardless of feature flags")
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

func TestFormatRecommendations_NumbersOnlyActionableRecommendations(t *testing.T) {
	origNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = origNoColor })

	recs := []recommender.Recommendation{
		{Method: recommender.MethodAlreadyInstalled, Title: "OneAgent already running", Done: true},
		{Method: recommender.MethodOtelCollector, Title: "Install OpenTelemetry Collector"},
		{Method: recommender.MethodDocker, Title: "Docker OneAgent", ComingSoon: true},
		{Method: recommender.MethodNotSupported, Title: "Unsupported platform"},
		{Method: recommender.MethodAWS, Title: "AWS cloud services"},
	}

	result := recommender.FormatRecommendations(recs)
	installLine := lineContaining(result, "Install OpenTelemetry Collector")
	awsLine := lineContaining(result, "AWS cloud services")
	doneLine := lineContaining(result, "OneAgent already running")
	comingSoonLine := lineContaining(result, "Docker OneAgent")
	unsupportedLine := lineContaining(result, "Unsupported platform")

	if !strings.Contains(installLine, " 1 ") {
		t.Fatalf("first actionable recommendation line = %q, want numbered 1", installLine)
	}
	if !strings.Contains(awsLine, " 2 ") {
		t.Fatalf("second actionable recommendation line = %q, want numbered 2", awsLine)
	}
	if strings.Contains(doneLine, " 1 ") || strings.Contains(comingSoonLine, " 2 ") || strings.Contains(unsupportedLine, " 2 ") {
		t.Fatalf("non-actionable recommendations should not consume menu numbers:\n%s", result)
	}
}

func lineContaining(s, needle string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
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
			if r.ComingSoon {
				t.Error("expected ComingSoon=false for Azure recommendation")
			}
			if r.Title != "Azure cloud services" {
				t.Errorf("expected title %q, got %q", "Azure cloud services", r.Title)
			}
		}
	}
	if !found {
		t.Error("expected azure install recommendation when Azure is available and not configured")
	}
}

func TestGenerateRecommendations_AzureConfigured(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformLinux,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
		Azure: &analyzer.AzureInfo{
			Available:      true,
			SubscriptionID: "sub-123",
		},
		AzureConfigured: true,
	}
	recs := recommender.GenerateRecommendations(system)
	foundUpdate := false
	for _, r := range recs {
		if r.Method == recommender.MethodAzure {
			t.Error("expected MethodAzureUpdate, not MethodAzure, when Azure is already configured")
		}
		if r.Method == recommender.MethodAzureUpdate {
			foundUpdate = true
			if r.Title != "Azure cloud services (update)" {
				t.Errorf("expected title %q, got %q", "Azure cloud services (update)", r.Title)
			}
		}
	}
	if !foundUpdate {
		t.Error("expected azure-update recommendation when Azure is available and already configured")
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
			if r.ComingSoon {
				t.Error("expected ComingSoon=false for GCP recommendation")
			}
			if r.Title != "GCP cloud services" {
				t.Errorf("expected title %q, got %q", "GCP cloud services", r.Title)
			}
		}
	}
	if !found {
		t.Error("expected gcp install recommendation when GCP is available and not configured")
	}
}

func TestGenerateRecommendations_GCPConfigured(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformLinux,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
		GCP: &analyzer.GCPInfo{
			Available: true,
			ProjectID: "my-project",
		},
		GCPConfigured: true,
	}
	recs := recommender.GenerateRecommendations(system)
	foundUpdate := false
	for _, r := range recs {
		if r.Method == recommender.MethodGCP {
			t.Error("expected MethodGCPUpdate, not MethodGCP, when GCP is already configured")
		}
		if r.Method == recommender.MethodGCPUpdate {
			foundUpdate = true
			if r.Title != "GCP cloud services (update)" {
				t.Errorf("expected title %q, got %q", "GCP cloud services (update)", r.Title)
			}
		}
	}
	if !foundUpdate {
		t.Error("expected gcp-update recommendation when GCP is available and already configured")
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

func TestGenerateRecommendations_UnavailableEntriesPresent(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformDarwin,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
	}
	recs := recommender.GenerateRecommendations(system)

	methods := map[recommender.IngestMethod]bool{}
	for _, r := range recs {
		if r.Unavailable {
			methods[r.Method] = true
		}
	}
	for _, want := range []recommender.IngestMethod{
		recommender.MethodKubernetes,
		recommender.MethodAWS,
		recommender.MethodAzure,
		recommender.MethodGCP,
	} {
		if !methods[want] {
			t.Errorf("expected unavailable entry for %s", want)
		}
	}
}

func TestGenerateRecommendations_DetectedProviderNotUnavailable(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformLinux,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorKubernetes,
		Kubernetes:       &analyzer.KubernetesInfo{Available: true, Distribution: "GKE"},
	}
	recs := recommender.GenerateRecommendations(system)
	for _, r := range recs {
		if r.Method == recommender.MethodKubernetes && r.Unavailable {
			t.Error("detected Kubernetes should not produce an unavailable entry")
		}
	}
}

func TestGenerateRecommendations_HostDetectionInfoContainsDots(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:     analyzer.PlatformDarwin,
		Arch:         "arm64",
		Hostname:     "myhost",
		Orchestrator: analyzer.OrchestratorNone,
		ProjectTechs: []analyzer.ProjectTech{
			{Name: "Go", Path: "~/Code/dtwiz/go.mod"},
			{Name: "Node.js", Path: "~/Code/dtwiz/package.json"},
		},
	}
	recs := recommender.GenerateRecommendations(system)
	for _, r := range recs {
		if r.Method == recommender.MethodOtelCollector {
			if !strings.Contains(r.DetectionInfo, " · ") {
				t.Errorf("DetectionInfo should use · as separator between host and techs: %q", r.DetectionInfo)
			}
			if !strings.Contains(r.DetectionInfo, "Go") || !strings.Contains(r.DetectionInfo, "Node.js") {
				t.Errorf("DetectionInfo should contain tech names: %q", r.DetectionInfo)
			}
			return
		}
	}
	t.Error("no OtelCollector recommendation found")
}

func TestActionableItems_ExcludesDoneAndUnavailable(t *testing.T) {
	recs := []recommender.Recommendation{
		{Method: recommender.MethodAlreadyInstalled, Done: true},
		{Method: recommender.MethodOtelCollector},
		{Method: recommender.MethodKubernetes, Unavailable: true},
	}
	actionable := recommender.ActionableItems(recs, false)
	if len(actionable) != 1 {
		t.Errorf("expected 1 actionable item, got %d", len(actionable))
	}
	if actionable[0].Method != recommender.MethodOtelCollector {
		t.Errorf("expected MethodOtelCollector, got %s", actionable[0].Method)
	}
}

func TestActionableItems_ExcludesExperimentalWithoutFlag(t *testing.T) {
	recs := []recommender.Recommendation{
		{Method: recommender.MethodOtelCollector},
		{Method: recommender.MethodDocker},
	}
	actionable := recommender.ActionableItems(recs, false)
	for _, r := range actionable {
		if r.Method == recommender.MethodDocker {
			t.Error("Docker should be excluded when experimental=false")
		}
	}
}

func TestActionableItems_IncludesExperimentalWithFlag(t *testing.T) {
	recs := []recommender.Recommendation{
		{Method: recommender.MethodOtelCollector},
		{Method: recommender.MethodDocker},
	}
	actionable := recommender.ActionableItems(recs, true)
	found := false
	for _, r := range actionable {
		if r.Method == recommender.MethodDocker {
			found = true
		}
	}
	if !found {
		t.Error("Docker should be included when experimental=true")
	}
}

func TestFormatSetupMenu_NumberedItemsAndUnavailableSection(t *testing.T) {
	origNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = origNoColor })

	recs := []recommender.Recommendation{
		{Method: recommender.MethodOtelCollector, Title: "OTel Collector", DetectionInfo: "myhost (macOS arm64)"},
		{Method: recommender.MethodKubernetes, Title: "Kubernetes cluster", Unavailable: true, ShortTitle: "Kubernetes", UnlockCommand: "kubectl config use-context <name>"},
		{Method: recommender.MethodAWS, Title: "AWS cloud services", Unavailable: true, ShortTitle: "AWS", UnlockCommand: "aws configure"},
	}
	result := recommender.FormatSetupMenu(recs, false, false)

	if !strings.Contains(result, "[1]") {
		t.Error("expected numbered entry [1] in setup menu")
	}
	if !strings.Contains(result, "myhost (macOS arm64)") {
		t.Error("expected detection info on numbered entry")
	}
	if !strings.Contains(result, "Connect to unlock") {
		t.Error("expected 'Connect to unlock' section for unavailable entries")
	}
	if !strings.Contains(result, "kubectl config use-context <name>") {
		t.Error("expected Kubernetes unlock command in unavailable section")
	}
	if !strings.Contains(result, "aws configure") {
		t.Error("expected AWS unlock command in unavailable section")
	}
}

func TestFormatSetupMenu_DoneEntryShowsContext(t *testing.T) {
	origNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = origNoColor })

	recs := []recommender.Recommendation{
		{Method: recommender.MethodAlreadyInstalled, Title: "OneAgent running", Done: true, Description: "OneAgent is detected on this host."},
		{Method: recommender.MethodOtelCollector, Title: "OTel Collector"},
	}
	result := recommender.FormatSetupMenu(recs, false, false)

	if !strings.Contains(result, "✓") {
		t.Error("expected ✓ badge for done entry")
	}
	if !strings.Contains(result, "OneAgent is detected on this host.") {
		t.Error("expected description shown as context for done entry when DetectionInfo is empty")
	}
}

func TestFormatSetupMenu_NoUnavailableSectionWhenAllDetected(t *testing.T) {
	origNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = origNoColor })

	recs := []recommender.Recommendation{
		{Method: recommender.MethodOtelCollector, Title: "OTel Collector"},
	}
	result := recommender.FormatSetupMenu(recs, false, false)

	if strings.Contains(result, "Sign in to unlock") {
		t.Error("should not show 'Sign in to unlock' when there are no unavailable entries")
	}
}

func TestFormatRecommendations_SkipsUnavailableInJSON(t *testing.T) {
	system := &analyzer.SystemInfo{
		Platform:         analyzer.PlatformDarwin,
		ContainerRuntime: analyzer.ContainerRuntimeNone,
		Orchestrator:     analyzer.OrchestratorNone,
	}
	recs := recommender.GenerateRecommendations(system)
	for _, r := range recs {
		if r.Unavailable {
			// FormatRecommendations (text) must skip unavailable.
			result := recommender.FormatRecommendations(recs)
			if strings.Contains(result, r.ShortTitle+" ") && strings.Contains(result, r.UnlockCommand) {
				t.Errorf("FormatRecommendations should not render unavailable entry for %s", r.ShortTitle)
			}
			return
		}
	}
}
