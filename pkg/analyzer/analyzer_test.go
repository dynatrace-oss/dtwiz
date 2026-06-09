package analyzer_test

import (
	"errors"
	"runtime"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
)

func TestAnalyzeSystem_ReturnsPlatform(t *testing.T) {
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
}

func TestAnalyzeSystem_ReturnsArch(t *testing.T) {
	info, err := analyzer.AnalyzeSystem()
	if err != nil {
		t.Fatalf("AnalyzeSystem() returned error: %v", err)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("expected arch %q, got %q", runtime.GOARCH, info.Arch)
	}
}

func TestAnalyzeSystem_SummaryNotEmpty(t *testing.T) {
	info, err := analyzer.AnalyzeSystem()
	if err != nil {
		t.Fatalf("AnalyzeSystem() returned error: %v", err)
	}
	s := info.Summary()
	if s == "" {
		t.Error("Summary() returned empty string")
	}
}

func TestDetectK8sDistribution(t *testing.T) {
	tests := []struct {
		context   string
		cluster   string
		serverURL string
		version   string
		want      string
	}{
		{"gke_project_region_cluster", "", "", "", "GKE"},
		{"arn:aws:eks:us-east-1:123:cluster/my-cluster", "", "", "", "EKS"},
		{"my-cluster-context", "", "https://my-cluster-abc123.hcp.eastus.azmk8s.io:443", "", "AKS"},
		{"my-aks-context", "my-cluster.azmk8s.io", "", "", "AKS"},
		{"openshift-context", "", "", "", "OpenShift"},
		{"minikube", "", "", "", "minikube"},
		{"kind-mycluster", "", "", "", "kind"},
		{"docker-desktop", "", "", "v1.30.0-k3s1", "k3s"},
		{"some-other-context", "", "", "v1.30.0", "kubernetes"},
		// IKS: server URL signal
		{"prod-context", "", "https://c1.us-south.containers.cloud.ibm.com:31234", "", "IKS"},
		// RKE: gitVersion +rke2 signal
		{"rke-context", "", "", "v1.28.9+rke2r1", "RKE"},
	}

	for _, tt := range tests {
		t.Run(tt.context, func(t *testing.T) {
			got := analyzer.DetectK8sDistribution(tt.context, tt.cluster, tt.serverURL, tt.version)
			if got != tt.want {
				t.Errorf("DetectK8sDistribution(%q, %q, %q, %q) = %q, want %q",
					tt.context, tt.cluster, tt.serverURL, tt.version, got, tt.want)
			}
		})
	}
}

func TestClassifyK8sSubVariant(t *testing.T) {
	errProbe := errors.New("exit status 1")

	tests := []struct {
		distro string
		output string
		err    error
		want   string
	}{
		{"GKE", `{"autopilot.gke.io/enabled":"true"}`, nil, "GKE-Autopilot"},
		{"GKE", `{"other-annotation":"value"}`, nil, "GKE"},
		{"GKE", "", errProbe, "GKE"},
		{"EKS", "Bottlerocket OS 1.14.0", nil, "EKS-Bottlerocket"},
		{"EKS", "Amazon Linux 2", nil, "EKS"},
		{"EKS", "", errProbe, "EKS"},
		{"kubernetes", "Active", nil, "TKGI"},
		{"kubernetes", "", nil, "kubernetes"},
		{"kubernetes", "Terminating", nil, "kubernetes"},
		{"kubernetes", "", errProbe, "kubernetes"},
		{"AKS", "", nil, "AKS"},
	}

	for _, tt := range tests {
		t.Run(tt.distro, func(t *testing.T) {
			got := analyzer.ClassifyK8sSubVariant(tt.distro, tt.output, tt.err)
			if got != tt.want {
				t.Errorf("ClassifyK8sSubVariant(%q, %q, %v) = %q, want %q",
					tt.distro, tt.output, tt.err, got, tt.want)
			}
		})
	}
}
