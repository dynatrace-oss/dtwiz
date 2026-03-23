package analyzer

import (
	"encoding/json"
	"strings"
	"sync"
)

// detectKubernetes checks for a reachable Kubernetes cluster.
func detectKubernetes() *KubernetesInfo {
	info := &KubernetesInfo{}

	ok, _ := runCmd("kubectl", "cluster-info", "--request-timeout=5s")
	if !ok {
		return info
	}
	info.Available = true

	var (
		ctx, cluster, serverURL, ver, nodesOut string
		wg sync.WaitGroup
	)
	wg.Add(5)
	go func() { defer wg.Done(); _, ctx = runCmd("kubectl", "config", "current-context") }()
	go func() { defer wg.Done(); _, cluster = runCmd("kubectl", "config", "view", "--minify", "-o", "jsonpath={.clusters[0].name}") }()
	go func() { defer wg.Done(); _, serverURL = runCmd("kubectl", "config", "view", "--minify", "-o", "jsonpath={.clusters[0].cluster.server}") }()
	go func() { defer wg.Done(); _, ver = runCmd("kubectl", "version", "-o", "json") }()
	go func() { defer wg.Done(); _, nodesOut = runCmd("kubectl", "get", "nodes", "--no-headers", "-o", "name") }()
	wg.Wait()

	info.Context = ctx
	info.Cluster = cluster
	info.ServerVersion = parseK8sServerVersion(ver)
	if nodesOut != "" {
		info.NodeCount = len(strings.Split(strings.TrimSpace(nodesOut), "\n"))
	}

	info.Distribution = DetectK8sDistribution(ctx, cluster, serverURL, info.ServerVersion)
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
func DetectK8sDistribution(context, cluster, serverURL, serverVersion string) string {
	ctxLower := strings.ToLower(context)
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
	// OpenShift
	if strings.Contains(ctxLower, "openshift") || strings.Contains(verLower, "openshift") {
		return "OpenShift"
	}
	// k3s
	if strings.Contains(verLower, "k3s") {
		return "k3s"
	}
	// minikube
	if ctxLower == "minikube" || strings.Contains(ctxLower, "minikube") {
		return "minikube"
	}
	// kind
	if strings.HasPrefix(ctxLower, "kind-") {
		return "kind"
	}

	return "kubernetes"
}
