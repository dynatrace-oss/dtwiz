package installer

import (
	"errors"
	"strings"
	"testing"
)

func withFakeRunCmdQuiet(t *testing.T, fn func(name string, args ...string) error) {
	t.Helper()
	orig := runCmdQuietFunc
	runCmdQuietFunc = fn
	t.Cleanup(func() { runCmdQuietFunc = orig })
}

func TestUninstallKubernetes_Cancelled(t *testing.T) {
	withStdin(t, "n\n", func() {
		out := captureStdout(t, func() {
			err := UninstallKubernetes("my-ctx", "AKS")
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

	out := captureStdout(t, func() {
		_ = UninstallKubernetes("FreeTrialKubernetesTest", "AKS")
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

	out := captureStdout(t, func() {
		_ = UninstallKubernetes("", "")
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

	out := captureStdout(t, func() {
		if err := UninstallKubernetes("my-ctx", "GKE"); err != nil {
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

	captureStdout(t, func() {
		err := UninstallKubernetes("my-ctx", "EKS")
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

	captureStdout(t, func() {
		err := UninstallKubernetes("my-ctx", "EKS")
		if err == nil {
			t.Fatal("expected error when helm uninstall fails")
		}
		if !strings.Contains(err.Error(), "helm uninstall failed") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}
