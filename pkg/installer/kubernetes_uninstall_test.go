package installer

import (
	"errors"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/testutil"
)

func withFakeRunCmdQuiet(t *testing.T, fn func(name string, args ...string) error) {
	t.Helper()
	orig := runCmdQuietFunc
	runCmdQuietFunc = fn
	t.Cleanup(func() { runCmdQuietFunc = orig })
}

func TestUninstallKubernetes_Cancelled(t *testing.T) {
	withStdin(t, "n\n", func() {
		out := testutil.CaptureStdout(t, func() {
			err := UninstallKubernetes("my-ctx", "AKS", false)
			if err != nil {
				t.Fatalf("expected nil on cancel, got: %v", err)
			}
		})
		if !strings.Contains(out, "Uninstall cancelled") {
			t.Errorf("expected cancellation message, got: %q", out)
		}
	})
}

func TestUninstallKubernetes_ShowsClusterInfo(t *testing.T) {
	orig := AutoConfirm
	AutoConfirm = true
	t.Cleanup(func() { AutoConfirm = orig })

	withFakeRunCmdQuiet(t, func(_ string, _ ...string) error { return nil })

	out := testutil.CaptureStdout(t, func() {
		_ = UninstallKubernetes("FreeTrialKubernetesTest", "AKS", false)
	})
	if !strings.Contains(out, "The affected cluster is: AKS context=FreeTrialKubernetesTest") {
		t.Errorf("expected cluster info line in output, got: %q", out)
	}
}

func TestUninstallKubernetes_NoClusterInfoWhenContextEmpty(t *testing.T) {
	orig := AutoConfirm
	AutoConfirm = true
	t.Cleanup(func() { AutoConfirm = orig })

	withFakeRunCmdQuiet(t, func(_ string, _ ...string) error { return nil })

	out := testutil.CaptureStdout(t, func() {
		_ = UninstallKubernetes("", "", false)
	})
	if strings.Contains(out, "The affected cluster is") {
		t.Errorf("expected no cluster info line when context is empty, got: %q", out)
	}
}

func TestUninstallKubernetes_Success(t *testing.T) {
	orig := AutoConfirm
	AutoConfirm = true
	t.Cleanup(func() { AutoConfirm = orig })

	var calls []string
	withFakeRunCmdQuiet(t, func(name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	})

	out := testutil.CaptureStdout(t, func() {
		if err := UninstallKubernetes("my-ctx", "GKE", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "uninstalled successfully") {
		t.Errorf("expected success message, got: %q", out)
	}
	if len(calls) == 0 {
		t.Error("expected kubectl/helm commands to be called")
	}
}

func TestUninstallKubernetes_EdgeConnectDeletedEvenWhenDynaKubeFails(t *testing.T) {
	orig := AutoConfirm
	AutoConfirm = true
	t.Cleanup(func() { AutoConfirm = orig })

	var calls []string
	withFakeRunCmdQuiet(t, func(name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if name == "kubectl" && len(args) > 1 && args[0] == "delete" && args[1] == "dynakube" {
			return errors.New("kubectl: not found")
		}
		return nil
	})

	testutil.CaptureStdout(t, func() { _ = UninstallKubernetes("my-ctx", "EKS", false) })

	ranEdgeConnect := false
	for _, c := range calls {
		if strings.Contains(c, "delete edgeconnect") {
			ranEdgeConnect = true
		}
	}
	if !ranEdgeConnect {
		t.Error("expected EdgeConnect deletion to run even when DynaKube delete fails")
	}
}

func TestUninstallKubernetes_KubectlDeleteFails(t *testing.T) {
	orig := AutoConfirm
	AutoConfirm = true
	t.Cleanup(func() { AutoConfirm = orig })

	withFakeRunCmdQuiet(t, func(_ string, args ...string) error {
		if len(args) > 0 && args[0] == "delete" && args[1] == "dynakube" {
			return errors.New("kubectl: not found")
		}
		return nil
	})

	testutil.CaptureStdout(t, func() {
		err := UninstallKubernetes("my-ctx", "EKS", false)
		if err == nil {
			t.Fatal("expected error when kubectl delete dynakube fails")
		}
	})
}

func TestUninstallKubernetes_HelmUninstallFails(t *testing.T) {
	orig := AutoConfirm
	AutoConfirm = true
	t.Cleanup(func() { AutoConfirm = orig })

	withFakeRunCmdQuiet(t, func(name string, _ ...string) error {
		if name == "helm" {
			return errors.New("helm: release not found")
		}
		return nil
	})

	testutil.CaptureStdout(t, func() {
		err := UninstallKubernetes("my-ctx", "EKS", false)
		if err == nil {
			t.Fatal("expected error when helm uninstall fails")
		}
		if !strings.Contains(err.Error(), "one or more steps failed") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestUninstallKubernetes_HelmFailContinuesToNamespaceDeletion(t *testing.T) {
	orig := AutoConfirm
	AutoConfirm = true
	t.Cleanup(func() { AutoConfirm = orig })

	var calls []string
	withFakeRunCmdQuiet(t, func(name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if name == "helm" {
			return errors.New("helm: release not found")
		}
		return nil
	})

	out := testutil.CaptureStdout(t, func() {
		err := UninstallKubernetes("my-ctx", "EKS", false)
		if err == nil {
			t.Fatal("expected error when helm uninstall fails")
		}
	})

	deletedNamespace := false
	for _, c := range calls {
		if strings.Contains(c, "delete namespace dynatrace") {
			deletedNamespace = true
		}
	}
	if !deletedNamespace {
		t.Error("expected namespace deletion to run even after helm failure")
	}
	if !strings.Contains(out, "Namespace deleted") {
		t.Errorf("expected namespace deleted message in output, got: %q", out)
	}
}

func TestUninstallKubernetes_KubectlDeleteFailContinuesToEnd(t *testing.T) {
	orig := AutoConfirm
	AutoConfirm = true
	t.Cleanup(func() { AutoConfirm = orig })

	var calls []string
	withFakeRunCmdQuiet(t, func(name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if name == "kubectl" && len(args) > 1 && args[0] == "delete" && args[1] == "dynakube" {
			return errors.New("kubectl: not found")
		}
		return nil
	})

	testutil.CaptureStdout(t, func() {
		err := UninstallKubernetes("my-ctx", "EKS", false)
		if err == nil {
			t.Fatal("expected error when kubectl delete dynakube fails")
		}
	})

	ranHelm, ranNamespace := false, false
	for _, c := range calls {
		if strings.Contains(c, "helm uninstall") {
			ranHelm = true
		}
		if strings.Contains(c, "delete namespace dynatrace") {
			ranNamespace = true
		}
	}
	if !ranHelm {
		t.Error("expected helm uninstall to run even after step 1 failure")
	}
	if !ranNamespace {
		t.Error("expected namespace deletion to run even after step 1 failure")
	}
}

func TestUninstallKubernetes_DryRun(t *testing.T) {
	var calls []string
	withFakeRunCmdQuiet(t, func(name string, args ...string) error {
		calls = append(calls, name)
		return nil
	})

	out := testutil.CaptureStdout(t, func() {
		if err := UninstallKubernetes("my-ctx", "GKE", true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if len(calls) > 0 {
		t.Errorf("expected no commands to run in dry-run mode, got: %v", calls)
	}
	for _, step := range []string{
		"Delete all DynaKube and EdgeConnect custom resources",
		"Wait for managed pods to terminate",
		"Helm uninstall dynatrace-operator",
		"Delete the dynatrace namespace",
	} {
		if !strings.Contains(out, step) {
			t.Errorf("expected step %q in dry-run output, got: %q", step, out)
		}
	}
}

func TestUninstallKubernetes_MultipleStepsFail(t *testing.T) {
	orig := AutoConfirm
	AutoConfirm = true
	t.Cleanup(func() { AutoConfirm = orig })

	withFakeRunCmdQuiet(t, func(name string, args ...string) error {
		if name == "helm" {
			return errors.New("helm: release not found")
		}
		if name == "kubectl" && len(args) > 0 && args[0] == "delete" && len(args) > 1 && args[1] == "namespace" {
			return errors.New("kubectl: namespace not found")
		}
		return nil
	})

	out := testutil.CaptureStdout(t, func() {
		err := UninstallKubernetes("my-ctx", "EKS", false)
		if err == nil {
			t.Fatal("expected error when multiple steps fail")
		}
		if !strings.Contains(err.Error(), "one or more steps failed") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Uninstall completed with errors") {
		t.Errorf("expected completion-with-errors message, got: %q", out)
	}
}
