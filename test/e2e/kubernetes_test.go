//go:build integration

package e2e_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	k8s "github.com/dynatrace-oss/dtwiz/pkg/installer/kubernetes"
	"github.com/dynatrace-oss/dtwiz/test/integration"
	"github.com/dynatrace-oss/dtwiz/test/integration/grail"
)

// countDynakubes returns the number of DynaKube custom resources in the
// dynatrace namespace, or -1 if the query itself fails (e.g. namespace gone).
func countDynakubes(t *testing.T) int {
	t.Helper()
	out, err := exec.Command("kubectl", "get", "dynakube", "-n", "dynatrace", "--no-headers").CombinedOutput()
	if err != nil {
		return -1
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}


// TestKubernetesLifecycle exercises the full Dynatrace Operator lifecycle
// against a real Kubernetes cluster and a real Dynatrace tenant: install
// (Helm chart + DynaKube CR, wait for pods to become ready) → uninstall
// (delete DynaKube CRs, wait for managed pods to terminate, Helm uninstall,
// delete the dynatrace namespace).
//
// It requires kubectl and helm on PATH and a cluster reachable via the
// current kubeconfig context — this test does not provision a cluster. Point
// kubectl at a disposable cluster (e.g. kind or minikube) before running it.
func TestKubernetesLifecycle(t *testing.T) {
	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Skip("kubectl not found in PATH")
	}
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not found in PATH")
	}

	k8sInfo := analyzer.DetectKubernetes()
	if !k8sInfo.Available {
		t.Skip("no reachable Kubernetes cluster (kubectl cluster-info failed); point kubectl at a disposable cluster before running this test")
	}

	integration.Parallelize(t)
	env := integration.SetupIntegration(t)
	t.Logf("test ID: %s", env.TestID)
	t.Logf("target cluster: context=%s distro=%s", k8sInfo.Context, k8sInfo.Distribution)

	// Safety-net cleanup: runs on partial failure — e.g. `helm install
	// --create-namespace` can leave the dynatrace namespace (or CRs) behind,
	// and --atomic only rolls back the Helm release, not the namespace.
	// Skipped when the explicit uninstall step below already succeeded.
	uninstalled := false
	t.Cleanup(func() {
		if uninstalled {
			return
		}
		if err := k8s.UninstallKubernetes(k8sInfo.Context, k8sInfo.Distribution, false); err != nil {
			t.Logf("cleanup: uninstall failed: %v", err)
		}
	})
	if os.Getenv("TEST_DEBUG") != "" {
		t.Cleanup(func() {
			if !t.Failed() {
				return
			}
			if out, err := exec.Command("kubectl", "get", "pods", "-n", "dynatrace", "-o", "wide").CombinedOutput(); err == nil {
				t.Logf("debug: pods in dynatrace namespace:\n%s", out)
			}
			if out, err := exec.Command("kubectl", "get", "events", "-n", "dynatrace", "--sort-by=.lastTimestamp", "--field-selector", "type=Warning").CombinedOutput(); err == nil {
				t.Logf("debug: warning events:\n%s", out)
			}
		})
	}

	t.Log("installing Dynatrace Operator (waits for pods to become ready, up to 10 min)")
	if err := k8s.InstallKubernetes(env.EnvURL, env.ClassicToken, env.TestID, k8sInfo.Distribution, false); err != nil {
		t.Fatalf("InstallKubernetes: %v", err)
	}

	t.Log("verifying DynaKube custom resources exist")
	// The installer renders two DynaKube CRs by design: one for
	// kubernetes-monitoring/KSPM and one ("-agents") for OneAgent + routing
	// ActiveGate. See pkg/installer/kubernetes/dynakube.tmpl.
	if n := countDynakubes(t); n != 2 {
		t.Fatalf("expected exactly 2 dynakubes in namespace dynatrace, got %d", n)
	}

	t.Log("verifying cluster topology was reported to Dynatrace")
	grail.RequireKubernetesCluster(t, env.Client, strings.ToLower(k8sInfo.Context),
		grail.WithTimeout(5*time.Minute),
		grail.WithInterval(20*time.Second),
	)

	t.Log("uninstalling Dynatrace Operator (waits for pods to terminate, up to 5 min)")
	if err := k8s.UninstallKubernetes(k8sInfo.Context, k8sInfo.Distribution, false); err != nil {
		t.Fatalf("UninstallKubernetes: %v", err)
	}
	uninstalled = true

	t.Log("verifying dynatrace namespace is gone")
	if err := exec.Command("kubectl", "get", "namespace", "dynatrace").Run(); err == nil {
		t.Error("expected namespace dynatrace to be gone after uninstall")
	}
}
