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

// fetchKubeContext returns the current kubectl context name, or empty string on failure.
func fetchKubeContext() string {
	_, ctx := runCmd("kubectl", "config", "current-context")
	return ctx
}

// fetchKubeCluster returns the cluster name from the active kubeconfig context.
func fetchKubeCluster() string {
	_, cluster := runCmd("kubectl", "config", "view", "--minify", "-o", "jsonpath={.clusters[0].name}")
	return cluster
}

// fetchKubeServerURL returns the API server URL from the active kubeconfig context.
func fetchKubeServerURL() string {
	_, serverURL := runCmd("kubectl", "config", "view", "--minify", "-o", "jsonpath={.clusters[0].cluster.server}")
	return serverURL
}

// resolveDistro identifies the K8s distribution by running heuristic detection
// followed by sub-variant probing.
func resolveDistro(kubeCtx, cluster, serverURL, serverVersion string) string {
	return ProbeK8sSubVariant(DetectK8sDistribution(kubeCtx, cluster, serverURL, serverVersion))
}

// DetectKubernetes checks for a reachable Kubernetes cluster.
func DetectKubernetes() *KubernetesInfo {
	info := &KubernetesInfo{}

	ok, _ := runCmd("kubectl", "cluster-info", "--request-timeout=5s")
	if !ok {
		info.Distribution = "None was detected"
		return info
	}
	info.Available = true

	var (
		ctx, cluster, serverURL, ver, nodesOut string
		wg                                     sync.WaitGroup
	)
	wg.Add(5)
	go func() { defer wg.Done(); ctx = fetchKubeContext() }()
	go func() { defer wg.Done(); cluster = fetchKubeCluster() }()
	go func() { defer wg.Done(); serverURL = fetchKubeServerURL() }()
	go func() { defer wg.Done(); _, ver = runCmd("kubectl", "version", "-o", "json") }()
	go func() { defer wg.Done(); _, nodesOut = runCmd("kubectl", "get", "nodes", "--no-headers", "-o", "name") }()
	wg.Wait()

	info.Context = ctx
	info.Cluster = cluster
	info.ServerVersion = parseK8sServerVersion(ver)
	if nodesOut != "" {
		info.NodeCount = len(strings.Split(strings.TrimSpace(nodesOut), "\n"))
	}

	info.Distribution = resolveDistro(ctx, cluster, serverURL, info.ServerVersion)
	return info
}

// DetectKubernetesIdentity returns only the kubectl context and K8s distribution.
// It skips the availability check, version lookup, node count, and sub-variant probes
// that require a live cluster connection — suitable for use before user confirmation.
func DetectKubernetesIdentity() *KubernetesInfo {
	info := &KubernetesInfo{}

	var cluster, serverURL string
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); info.Context = fetchKubeContext() }()
	go func() { defer wg.Done(); cluster = fetchKubeCluster() }()
	go func() { defer wg.Done(); serverURL = fetchKubeServerURL() }()
	wg.Wait()

	info.Distribution = resolveDistro(info.Context, cluster, serverURL, "")
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
		return "GKE"
	}
	// EKS
	if strings.HasPrefix(ctxLower, "arn:") || strings.Contains(serverURLLower, ".eks.amazonaws.com") ||
		strings.Contains(ctxLower, ":eks:") {
		return "EKS"
	}
	// AKS
	if strings.Contains(serverURLLower, ".azmk8s.io") || strings.Contains(clusterLower, ".azmk8s.io") ||
		strings.Contains(ctxLower, "aks") {
		return "AKS"
	}
	// IKS
	if strings.Contains(serverURLLower, ".containers.cloud.ibm.com") {
		return "IKS"
	}
	// OpenShift
	if strings.Contains(ctxLower, "openshift") || strings.Contains(verLower, "openshift") {
		return "OpenShift"
	}
	// k3s
	if strings.Contains(verLower, "k3s") {
		return "k3s"
	}
	// RKE (RKE2 — gitVersion contains +rke2)
	if strings.Contains(verLower, "+rke2") {
		return "RKE"
	}

	return "kubernetes"
}

type cmdRunner func(timeout time.Duration, cmd string, args ...string) (string, error)

// ProbeK8sSubVariant runs conditional kubectl probes to refine the parent distribution.
// On error or timeout the parent distro is returned unchanged.
func ProbeK8sSubVariant(distro string) string {
	return probeK8sSubVariant(distro, runCmdWithTimeout)
}

func probeK8sSubVariant(distro string, run cmdRunner) string {
	switch distro {
	case "GKE":
		output, err := run(5*time.Second, "kubectl", "get", "namespace", "kube-system",
			"-o", "jsonpath={.metadata.annotations}")
		if err != nil {
			display.PrintWarning("GKE Autopilot probe", err)
		}
		return ClassifyK8sSubVariant(distro, output, err)
	case "EKS":
		output, err := run(5*time.Second, "kubectl", "get", "nodes",
			"-o", "jsonpath={.items[*].status.nodeInfo.osImage}")
		if err != nil {
			display.PrintWarning("EKS Bottlerocket probe", err)
		}
		return ClassifyK8sSubVariant(distro, output, err)
	case "kubernetes":
		minikubeOut, minikubeErr := run(5*time.Second, "kubectl", "get", "nodes",
			"-l", "minikube.k8s.io/name", "--no-headers", "-o", "name")
		if minikubeErr == nil && strings.TrimSpace(minikubeOut) != "" {
			return "minikube"
		}

		kindOut, kindErr := run(5*time.Second, "kubectl", "get", "nodes",
			"-o", "jsonpath={.items[0].spec.providerID}")
		if kindErr == nil && strings.HasPrefix(strings.TrimSpace(kindOut), "kind://") {
			return "kind"
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
	case "GKE":
		if strings.Contains(output, "autopilot.gke.io") {
			return "GKE-Autopilot"
		}
	case "EKS":
		if strings.Contains(strings.ToLower(output), "bottlerocket") {
			return "EKS-Bottlerocket"
		}
	case "kubernetes":
		if strings.TrimSpace(output) == "Active" {
			return "TKGI"
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
