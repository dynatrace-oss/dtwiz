package analyzer

import (
	"errors"
	"testing"
	"time"
)

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
		{"docker-desktop", "", "", "v1.30.0-k3s1", "k3s"},
		{"some-other-context", "", "", "v1.30.0", "kubernetes"},
		// IKS: server URL signal
		{"prod-context", "", "https://c1.us-south.containers.cloud.ibm.com:31234", "", "IKS"},
		// RKE: gitVersion +rke2 signal
		{"rke-context", "", "", "v1.28.9+rke2r1", "RKE"},
	}

	for _, tt := range tests {
		t.Run(tt.context, func(t *testing.T) {
			got := DetectK8sDistribution(tt.context, tt.cluster, tt.serverURL, tt.version)
			if got != tt.want {
				t.Errorf("DetectK8sDistribution(%q, %q, %q, %q) = %q, want %q",
					tt.context, tt.cluster, tt.serverURL, tt.version, got, tt.want)
			}
		})
	}
}

type fakeCall struct {
	out string
	err error
}

func TestProbeK8sSubVariant(t *testing.T) {
	errProbe := errors.New("exit status 1")

	tests := []struct {
		name   string
		distro string
		calls  []fakeCall
		want   string
	}{
		// GKE → Autopilot
		{
			name:   "GKE Autopilot detected",
			distro: "GKE",
			calls:  []fakeCall{{"autopilot.gke.io/enabled:true", nil}},
			want:   "GKE-Autopilot",
		},
		{
			name:   "GKE Standard unchanged",
			distro: "GKE",
			calls:  []fakeCall{{"other-annotation:value", nil}},
			want:   "GKE",
		},
		{
			name:   "GKE probe error falls back to parent",
			distro: "GKE",
			calls:  []fakeCall{{"", errProbe}},
			want:   "GKE",
		},

		// EKS → Bottlerocket
		{
			name:   "EKS Bottlerocket detected",
			distro: "EKS",
			calls:  []fakeCall{{"Bottlerocket OS 1.14.0", nil}},
			want:   "EKS-Bottlerocket",
		},
		{
			name:   "EKS Standard unchanged",
			distro: "EKS",
			calls:  []fakeCall{{"Amazon Linux 2", nil}},
			want:   "EKS",
		},
		{
			name:   "EKS probe error falls back to parent",
			distro: "EKS",
			calls:  []fakeCall{{"", errProbe}},
			want:   "EKS",
		},

		// kubernetes → minikube (first probe: minikube node label)
		{
			name:   "minikube detected via node label",
			distro: "kubernetes",
			calls:  []fakeCall{{"node/minikube", nil}},
			want:   "minikube",
		},

		// kubernetes → kind (second probe: providerID)
		{
			name:   "kind detected via providerID",
			distro: "kubernetes",
			calls: []fakeCall{
				{"", nil},                  // minikube label probe: empty → not minikube
				{"kind://docker/...", nil}, // kind providerID probe
			},
			want: "kind",
		},

		// kubernetes → TKGI (third probe: pks-system namespace)
		{
			name:   "TKGI detected via pks-system namespace",
			distro: "kubernetes",
			calls: []fakeCall{
				{"", nil},
				{"", nil},
				{"Active", nil},
			},
			want: "TKGI",
		},

		// kubernetes → generic fallthrough
		{
			name:   "generic kubernetes when no probe matches",
			distro: "kubernetes",
			calls:  []fakeCall{{"", nil}, {"", nil}, {"", nil}},
			want:   "kubernetes",
		},
		{
			name:   "TKGI namespace Terminating falls through to kubernetes",
			distro: "kubernetes",
			calls:  []fakeCall{{"", nil}, {"", nil}, {"Terminating", nil}},
			want:   "kubernetes",
		},
		{
			name:   "TKGI probe error falls through to kubernetes",
			distro: "kubernetes",
			calls:  []fakeCall{{"", nil}, {"", nil}, {"", errProbe}},
			want:   "kubernetes",
		},

		// unrecognised distro passes through unchanged
		{
			name:   "unknown distro unchanged",
			distro: "AKS",
			calls:  nil,
			want:   "AKS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callIdx := 0
			fake := func(_ time.Duration, _ string, _ ...string) (string, error) {
				if callIdx >= len(tt.calls) {
					t.Fatalf("unexpected extra kubectl call (index %d)", callIdx)
				}
				c := tt.calls[callIdx]
				callIdx++
				return c.out, c.err
			}

			got := probeK8sSubVariant(tt.distro, fake)
			if got != tt.want {
				t.Errorf("probeK8sSubVariant(%q) = %q, want %q", tt.distro, got, tt.want)
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
			got := ClassifyK8sSubVariant(tt.distro, tt.output, tt.err)
			if got != tt.want {
				t.Errorf("ClassifyK8sSubVariant(%q, %q, %v) = %q, want %q",
					tt.distro, tt.output, tt.err, got, tt.want)
			}
		})
	}
}
