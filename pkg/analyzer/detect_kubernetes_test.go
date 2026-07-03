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
		{"gke_project_region_cluster", "", "", "", DistroGKE},
		{"arn:aws:eks:us-east-1:123:cluster/my-cluster", "", "", "", DistroEKS},
		{"my-cluster-context", "", "https://my-cluster-abc123.hcp.eastus.azmk8s.io:443", "", DistroAKS},
		{"my-aks-context", "my-cluster.azmk8s.io", "", "", DistroAKS},
		{"openshift-context", "", "", "", DistroOpenShift},
		{"docker-desktop", "", "", "v1.30.0-k3s1", DistroK3s},
		{"some-other-context", "", "", "v1.30.0", DistroKubernetes},
		// IKS: server URL signal
		{"prod-context", "", "https://c1.us-south.containers.cloud.ibm.com:31234", "", DistroIKS},
		// RKE: gitVersion +rke2 signal
		{"rke-context", "", "", "v1.28.9+rke2r1", DistroRKE},
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
			distro: DistroGKE,
			calls:  []fakeCall{{"gk3-my-cluster-pool-1-abc123-xyz", nil}},
			want:   DistroGKEAutopilot,
		},
		{
			name:   "GKE Standard unchanged",
			distro: DistroGKE,
			calls:  []fakeCall{{"gke-my-cluster-pool-1-abc123-xyz", nil}},
			want:   DistroGKE,
		},
		{
			name:   "GKE probe error falls back to parent",
			distro: DistroGKE,
			calls:  []fakeCall{{"", errProbe}},
			want:   DistroGKE,
		},

		// EKS → Bottlerocket
		{
			name:   "EKS Bottlerocket detected",
			distro: DistroEKS,
			calls:  []fakeCall{{"Bottlerocket OS 1.14.0", nil}},
			want:   DistroEKSBottlerocket,
		},
		{
			name:   "EKS Standard unchanged",
			distro: DistroEKS,
			calls:  []fakeCall{{"Amazon Linux 2", nil}},
			want:   DistroEKS,
		},
		{
			name:   "EKS probe error falls back to parent",
			distro: DistroEKS,
			calls:  []fakeCall{{"", errProbe}},
			want:   DistroEKS,
		},

		// kubernetes → minikube (first probe: minikube node label)
		{
			name:   "minikube detected via node label",
			distro: DistroKubernetes,
			calls:  []fakeCall{{"node/minikube", nil}},
			want:   DistroMinikube,
		},

		// kubernetes → kind (second probe: providerID)
		{
			name:   "kind detected via providerID",
			distro: DistroKubernetes,
			calls: []fakeCall{
				{"", nil},                  // minikube label probe: empty → not minikube
				{"kind://docker/...", nil}, // kind providerID probe
			},
			want: DistroKind,
		},

		// kubernetes → TKGI (third probe: pks-system namespace)
		{
			name:   "TKGI detected via pks-system namespace",
			distro: DistroKubernetes,
			calls: []fakeCall{
				{"", nil},
				{"", nil},
				{"Active", nil},
			},
			want: DistroTKGI,
		},

		// kubernetes → generic fallthrough
		{
			name:   "generic kubernetes when no probe matches",
			distro: DistroKubernetes,
			calls:  []fakeCall{{"", nil}, {"", nil}, {"", nil}},
			want:   DistroKubernetes,
		},
		{
			name:   "TKGI namespace Terminating falls through to kubernetes",
			distro: DistroKubernetes,
			calls:  []fakeCall{{"", nil}, {"", nil}, {"Terminating", nil}},
			want:   DistroKubernetes,
		},
		{
			name:   "TKGI probe error falls through to kubernetes",
			distro: DistroKubernetes,
			calls:  []fakeCall{{"", nil}, {"", nil}, {"", errProbe}},
			want:   DistroKubernetes,
		},

		// unrecognised distro passes through unchanged
		{
			name:   "unknown distro unchanged",
			distro: DistroAKS,
			calls:  nil,
			want:   DistroAKS,
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
		{DistroGKE, "gk3-my-cluster-pool-1-abc123-xyz", nil, DistroGKEAutopilot},
		{DistroGKE, "gke-my-cluster-pool-1-abc123-xyz", nil, DistroGKE},
		{DistroGKE, "", errProbe, DistroGKE},
		{DistroEKS, "Bottlerocket OS 1.14.0", nil, DistroEKSBottlerocket},
		{DistroEKS, "Amazon Linux 2", nil, DistroEKS},
		{DistroEKS, "", errProbe, DistroEKS},
		{DistroKubernetes, "Active", nil, DistroTKGI},
		{DistroKubernetes, "", nil, DistroKubernetes},
		{DistroKubernetes, "Terminating", nil, DistroKubernetes},
		{DistroKubernetes, "", errProbe, DistroKubernetes},
		{DistroAKS, "", nil, DistroAKS},
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
