package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
)

// K8s distribution constants used across detection, installation, and recommendation.
const (
	DistroGKE             = "GKE"
	DistroEKS             = "EKS"
	DistroAKS             = "AKS"
	DistroIKS             = "IKS"
	DistroOpenShift       = "OpenShift"
	DistroK3s             = "k3s"
	DistroRKE             = "RKE"
	DistroKubernetes      = "kubernetes"
	DistroGKEAutopilot    = "GKE-Autopilot"
	DistroEKSBottlerocket = "EKS-Bottlerocket"
	DistroMinikube        = "minikube"
	DistroKind            = "kind"
	DistroTKGI            = "TKGI"
	DistroNone            = "None was detected"
)

// fetchKubeContext returns the current kubectl context name, or empty string on failure.
func fetchKubeContext() string {
	_, ctx := runCmd("kubectl", "config", "current-context")
	return ctx
}

// FetchKubeCluster returns the cluster name from the active kubeconfig context.
func FetchKubeCluster() string {
	_, cluster := runCmd("kubectl", "config", "view", "--minify", "-o", "jsonpath={.clusters[0].name}")
	return cluster
}

// fetchKubeServerURL returns the API server URL from the active kubeconfig context.
func fetchKubeServerURL() string {
	_, serverURL := runCmd("kubectl", "config", "view", "--minify", "-o", "jsonpath={.clusters[0].cluster.server}")
	return serverURL
}

// fetchKubeIdentity fetches context, cluster, and server URL concurrently.
func fetchKubeIdentity() (kubeCtx, cluster, serverURL string) {
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); kubeCtx = fetchKubeContext() }()
	go func() { defer wg.Done(); cluster = FetchKubeCluster() }()
	go func() { defer wg.Done(); serverURL = fetchKubeServerURL() }()
	wg.Wait()
	return
}

// DetectKubernetes checks for a reachable Kubernetes cluster.
func DetectKubernetes() *KubernetesInfo {
	info := &KubernetesInfo{}

	ok, _ := runCmd("kubectl", "cluster-info", "--request-timeout=5s")
	if !ok {
		info.Distribution = DistroNone
		return info
	}
	info.Available = true

	var ver, nodesOut string
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, ver = runCmd("kubectl", "version", "-o", "json") }()
	go func() { defer wg.Done(); _, nodesOut = runCmd("kubectl", "get", "nodes", "--no-headers", "-o", "name") }()

	ctx, cluster, serverURL := fetchKubeIdentity()
	wg.Wait()

	info.Context = ctx
	info.Cluster = cluster
	info.ServerVersion = parseK8sServerVersion(ver)
	if nodesOut != "" {
		info.NodeCount = len(strings.Split(strings.TrimSpace(nodesOut), "\n"))
	}

	info.Distribution = ProbeK8sSubVariant(DetectK8sDistribution(ctx, cluster, serverURL, info.ServerVersion))
	return info
}

// DetectKubernetesIdentity returns only the kubectl context and K8s distribution.
// It skips the availability check, version lookup, node count, and sub-variant probes
// that require a live cluster connection — suitable for use before user confirmation.
func DetectKubernetesIdentity() *KubernetesInfo {
	info := &KubernetesInfo{}
	ctx, cluster, serverURL := fetchKubeIdentity()
	info.Context = ctx
	info.Distribution = ProbeK8sSubVariant(DetectK8sDistribution(ctx, cluster, serverURL, ""))
	return info
}

// parseK8sServerVersion extracts gitVersion from `kubectl version -o json` output.
func parseK8sServerVersion(out string) string {
	var v struct {
		ServerVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"serverVersion"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return ""
	}
	return v.ServerVersion.GitVersion
}

// DetectK8sDistribution heuristically identifies the Kubernetes distribution.
// It is exported for testing.
func DetectK8sDistribution(kubeCtx, cluster, serverURL, serverVersion string) string {
	ctxLower := strings.ToLower(kubeCtx)
	clusterLower := strings.ToLower(cluster)
	serverURLLower := strings.ToLower(serverURL)
	verLower := strings.ToLower(serverVersion)

	// GKE
	if strings.HasPrefix(ctxLower, "gke_") || strings.Contains(clusterLower, "gke") ||
		strings.Contains(serverURLLower, "googleapis.com") {
		return DistroGKE
	}
	// EKS
	if strings.HasPrefix(ctxLower, "arn:") || strings.Contains(serverURLLower, ".eks.amazonaws.com") ||
		strings.Contains(ctxLower, ":eks:") {
		return DistroEKS
	}
	// AKS
	if strings.Contains(serverURLLower, ".azmk8s.io") || strings.Contains(clusterLower, ".azmk8s.io") ||
		strings.Contains(ctxLower, "aks") {
		return DistroAKS
	}
	// IKS
	if strings.Contains(serverURLLower, ".containers.cloud.ibm.com") {
		return DistroIKS
	}
	// OpenShift
	if strings.Contains(ctxLower, "openshift") || strings.Contains(verLower, "openshift") {
		return DistroOpenShift
	}
	// k3s
	if strings.Contains(verLower, "k3s") {
		return DistroK3s
	}
	// RKE (RKE2 — gitVersion contains +rke2)
	if strings.Contains(verLower, "+rke2") {
		return DistroRKE
	}

	return DistroKubernetes
}

type cmdRunner func(timeout time.Duration, cmd string, args ...string) (string, error)

// ProbeK8sSubVariant runs conditional kubectl probes to refine the parent distribution.
// On error or timeout the parent distro is returned unchanged.
func ProbeK8sSubVariant(distro string) string {
	return probeK8sSubVariant(distro, runCmdWithTimeout)
}

func probeK8sSubVariant(distro string, run cmdRunner) string {
	switch distro {
	case DistroGKE:
		// GKE Autopilot nodes always use the "gk3-" name prefix; Standard nodes use "gke-".
		// This is the officially documented signal per GKE Autopilot node naming conventions.
		output, err := run(5*time.Second, "kubectl", "get", "nodes",
			"-o", "jsonpath={.items[*].metadata.name}")
		if err != nil {
			display.PrintWarning("GKE Autopilot probe", err)
		}
		return ClassifyK8sSubVariant(distro, output, err)
	case DistroEKS:
		output, err := run(5*time.Second, "kubectl", "get", "nodes",
			"-o", "jsonpath={.items[*].status.nodeInfo.osImage}")
		if err != nil {
			display.PrintWarning("EKS Bottlerocket probe", err)
		}
		return ClassifyK8sSubVariant(distro, output, err)
	case DistroKubernetes:
		minikubeOut, minikubeErr := run(5*time.Second, "kubectl", "get", "nodes",
			"-l", "minikube.k8s.io/name", "--no-headers", "-o", "name")
		if minikubeErr == nil && strings.TrimSpace(minikubeOut) != "" {
			return DistroMinikube
		}

		kindOut, kindErr := run(5*time.Second, "kubectl", "get", "nodes",
			"-o", "jsonpath={.items[0].spec.providerID}")
		if kindErr == nil && strings.HasPrefix(strings.TrimSpace(kindOut), "kind://") {
			return DistroKind
		}

		output, err := run(5*time.Second, "kubectl", "get", "namespace", "pks-system",
			"--ignore-not-found", "-o", "jsonpath={.status.phase}")
		if err != nil {
			display.PrintWarning("TKGI namespace probe", err)
		}
		return ClassifyK8sSubVariant(distro, output, err)
	default:
		return distro
	}
}

// ClassifyK8sSubVariant applies sub-variant detection rules given the parent
// distro, kubectl output, and whether the kubectl call succeeded.
// It is exported for testing.
func ClassifyK8sSubVariant(distro, output string, err error) string {
	if err != nil {
		return distro
	}
	switch distro {
	case DistroGKE:
		for _, name := range strings.Fields(output) {
			if strings.HasPrefix(name, "gk3-") {
				return DistroGKEAutopilot
			}
		}
	case DistroEKS:
		if strings.Contains(strings.ToLower(output), "bottlerocket") {
			return DistroEKSBottlerocket
		}
	case DistroKubernetes:
		if strings.TrimSpace(output) == "Active" {
			return DistroTKGI
		}
	}
	return distro
}

// runCmdWithTimeout runs the command with a hard deadline, returning (trimmed output, error).
func runCmdWithTimeout(timeout time.Duration, command string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c := exec.CommandContext(ctx, command, args...)
	var buf bytes.Buffer
	c.Stdout = &buf
	err := c.Run()
	return strings.TrimSpace(buf.String()), err
}
