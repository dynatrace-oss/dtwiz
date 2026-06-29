package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func baseTemplateData() dynakubeTemplateData {
	return dynakubeTemplateData{
		ClusterName:      "test-cluster",
		APIURL:           "https://abc123.live.dynatracelabs.com/api",
		APIToken:         "dt0c01.token",
		DataIngestToken:  "dt0c01.token",
		ActiveGateImage:  dynakubeActiveGateImage,
		EECRepository:    dynakubeEECRepository,
		EECTag:           dynakubeEECTag,
		CodeModulesImage: dynakubeCodeModulesImage,
	}
}

func TestRenderDynakubeTemplate_PinnedImages(t *testing.T) {
	out, err := renderDynakubeTemplate(baseTemplateData())
	if err != nil {
		t.Fatalf("renderDynakubeTemplate: %v", err)
	}

	checks := []struct {
		desc string
		want string
	}{
		{"ActiveGate image pinned", "image: " + dynakubeActiveGateImage},
		{"EEC repository", "repository: " + dynakubeEECRepository},
		{"EEC tag", "tag: " + dynakubeEECTag},
		{"codeModulesImage pinned", "codeModulesImage: " + dynakubeCodeModulesImage},
	}

	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: rendered manifest does not contain %q", c.desc, c.want)
		}
	}
}

func TestRenderDynakubeTemplate_NoPlaceholders(t *testing.T) {
	out, err := renderDynakubeTemplate(baseTemplateData())
	if err != nil {
		t.Fatalf("renderDynakubeTemplate: %v", err)
	}

	if strings.Contains(out, "{{") || strings.Contains(out, "}}") {
		t.Error("rendered manifest still contains unresolved template placeholders")
	}
}

func TestRenderDynakubeTemplate_NoAggregationLabel(t *testing.T) {
	// aggregate-to-monitoring label deprecated since Operator 1.9.0; must never appear.
	for _, distro := range []string{"EKS", "EKS-Bottlerocket", "GKE", "OpenShift", "AKS", "kubernetes", ""} {
		out, err := renderDynakubeTemplate(distroTemplateData(baseTemplateData(), distro))
		if err != nil {
			t.Fatalf("distro %q: renderDynakubeTemplate: %v", distro, err)
		}
		if strings.Contains(out, "aggregate-to-monitoring") {
			t.Errorf("distro %q: manifest must not contain rbac.dynatrace.com/aggregate-to-monitoring label", distro)
		}
	}
}

func TestRenderDynakubeTemplate_NoReadonlyVolumeAnnotation(t *testing.T) {
	// injection-readonly-volume must never appear — it was only needed for
	// Operator 0.12.0+ and < 1.7.0; dtwiz targets Operator >= 1.7.0.
	for _, distro := range []string{"EKS-Bottlerocket", "EKS", "GKE", "OpenShift", "kubernetes", ""} {
		out, err := renderDynakubeTemplate(distroTemplateData(baseTemplateData(), distro))
		if err != nil {
			t.Fatalf("distro %q: renderDynakubeTemplate: %v", distro, err)
		}
		if strings.Contains(out, "injection-readonly-volume") {
			t.Errorf("distro %q: manifest must not contain injection-readonly-volume annotation", distro)
		}
	}
}

func TestDistroTemplateData_KSPM(t *testing.T) {
	kspmDistros := []string{"EKS", "AKS", "kubernetes", "minikube", "kind", "k3s", ""}
	for _, distro := range kspmDistros {
		d := distroTemplateData(baseTemplateData(), distro)
		if !d.EnableKSPM {
			t.Errorf("distro %q: expected EnableKSPM=true", distro)
		}
	}

	noKSPMDistros := []string{"GKE", "GKE-Autopilot", "OpenShift", "EKS-Bottlerocket", "RKE", "IKS", "TKGI"}
	for _, distro := range noKSPMDistros {
		d := distroTemplateData(baseTemplateData(), distro)
		if d.EnableKSPM {
			t.Errorf("distro %q: expected EnableKSPM=false", distro)
		}
	}
}

func TestDistroTemplateData_PrivilegedAnnotation(t *testing.T) {
	d := distroTemplateData(baseTemplateData(), "OpenShift")
	if !d.PrivilegedAnnotation {
		t.Error("OpenShift: expected PrivilegedAnnotation=true")
	}

	for _, distro := range []string{"EKS", "AKS", "GKE", "EKS-Bottlerocket", "kubernetes", ""} {
		d := distroTemplateData(baseTemplateData(), distro)
		if d.PrivilegedAnnotation {
			t.Errorf("distro %q: expected PrivilegedAnnotation=false", distro)
		}
	}
}

func TestDistroTemplateData_KubeletPath(t *testing.T) {
	cases := []struct {
		distro string
		want   string
	}{
		{"IKS", "/var/data/kubelet"},
		{"TKGI", "/var/vcap/data/kubelet"},
	}
	for _, c := range cases {
		d := distroTemplateData(baseTemplateData(), c.distro)
		if d.KubeletPath != c.want {
			t.Errorf("distro %q: KubeletPath = %q, want %q", c.distro, d.KubeletPath, c.want)
		}
	}

	for _, distro := range []string{"EKS", "AKS", "GKE", "OpenShift", "kubernetes", ""} {
		d := distroTemplateData(baseTemplateData(), distro)
		if d.KubeletPath != "" {
			t.Errorf("distro %q: expected empty KubeletPath, got %q", distro, d.KubeletPath)
		}
	}
}

func TestRenderDynakubeTemplate_KSPM_Present(t *testing.T) {
	for _, distro := range []string{"EKS", "AKS", "kubernetes", ""} {
		out, err := renderDynakubeTemplate(distroTemplateData(baseTemplateData(), distro))
		if err != nil {
			t.Fatalf("distro %q: renderDynakubeTemplate: %v", distro, err)
		}
		if !strings.Contains(out, "mappedHostPaths") {
			t.Errorf("distro %q: expected mappedHostPaths in manifest", distro)
		}
		if !strings.Contains(out, "kspmNodeConfigurationCollector") {
			t.Errorf("distro %q: expected kspmNodeConfigurationCollector in manifest", distro)
		}
	}
}

func TestRenderDynakubeTemplate_KSPM_Absent(t *testing.T) {
	for _, distro := range []string{"GKE", "GKE-Autopilot", "OpenShift", "EKS-Bottlerocket", "RKE", "IKS", "TKGI"} {
		out, err := renderDynakubeTemplate(distroTemplateData(baseTemplateData(), distro))
		if err != nil {
			t.Fatalf("distro %q: renderDynakubeTemplate: %v", distro, err)
		}
		if strings.Contains(out, "mappedHostPaths") {
			t.Errorf("distro %q: unexpected mappedHostPaths in manifest", distro)
		}
		if strings.Contains(out, "kspmNodeConfigurationCollector") {
			t.Errorf("distro %q: unexpected kspmNodeConfigurationCollector in manifest", distro)
		}
	}
}

func TestRenderDynakubeTemplate_OpenShiftAnnotation(t *testing.T) {
	out, err := renderDynakubeTemplate(distroTemplateData(baseTemplateData(), "OpenShift"))
	if err != nil {
		t.Fatalf("renderDynakubeTemplate: %v", err)
	}
	count := strings.Count(out, "feature.dynatrace.com/oneagent-privileged: \"true\"")
	if count != 2 {
		t.Errorf("OpenShift: expected privileged annotation on both DynaKubes, found %d occurrence(s)", count)
	}
}

func TestRenderDynakubeTemplate_OpenShiftAnnotation_Absent(t *testing.T) {
	for _, distro := range []string{"EKS", "AKS", "GKE", "EKS-Bottlerocket", "kubernetes", ""} {
		out, err := renderDynakubeTemplate(distroTemplateData(baseTemplateData(), distro))
		if err != nil {
			t.Fatalf("distro %q: renderDynakubeTemplate: %v", distro, err)
		}
		if strings.Contains(out, "oneagent-privileged") {
			t.Errorf("distro %q: unexpected oneagent-privileged annotation in manifest", distro)
		}
	}
}

func TestRenderDynakubeTemplate_KubeletPath(t *testing.T) {
	cases := []struct {
		distro string
		want   string
	}{
		{"IKS", "kubeletPath: /var/data/kubelet"},
		{"TKGI", "kubeletPath: /var/vcap/data/kubelet"},
	}
	for _, c := range cases {
		out, err := renderDynakubeTemplate(distroTemplateData(baseTemplateData(), c.distro))
		if err != nil {
			t.Fatalf("distro %q: renderDynakubeTemplate: %v", c.distro, err)
		}
		if !strings.Contains(out, c.want) {
			t.Errorf("distro %q: expected %q in manifest", c.distro, c.want)
		}
	}

	for _, distro := range []string{"EKS", "AKS", "GKE", "OpenShift", "kubernetes", ""} {
		out, err := renderDynakubeTemplate(distroTemplateData(baseTemplateData(), distro))
		if err != nil {
			t.Fatalf("distro %q: renderDynakubeTemplate: %v", distro, err)
		}
		if strings.Contains(out, "kubeletPath") {
			t.Errorf("distro %q: unexpected kubeletPath in manifest", distro)
		}
	}
}

func TestRenderDynakubeTemplate_SecretAndRBAC(t *testing.T) {
	out, err := renderDynakubeTemplate(baseTemplateData())
	if err != nil {
		t.Fatalf("renderDynakubeTemplate: %v", err)
	}

	for _, want := range []string{
		"kind: Secret",
		"kind: ClusterRole",
		"kind: ClusterRoleBinding",
		"name: dynatrace-kubernetes-monitoring-sensitive",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in manifest", want)
		}
	}
}

func TestHelmOperatorArgs_RollbackFlag(t *testing.T) {
	for _, tc := range []struct {
		helmMajor int
		wantFlag  string
	}{
		{3, "--atomic"},
		{4, "--rollback-on-failure"},
	} {
		args := helmOperatorArgs(tc.helmMajor, false)
		found := false
		for _, a := range args {
			if a == tc.wantFlag {
				found = true
			}
		}
		if !found {
			t.Errorf("helmMajor=%d: expected %q in install args", tc.helmMajor, tc.wantFlag)
		}

		args = helmOperatorUpgradeArgs(tc.helmMajor, false)
		found = false
		for _, a := range args {
			if a == tc.wantFlag {
				found = true
			}
		}
		if !found {
			t.Errorf("helmMajor=%d: expected %q in upgrade args", tc.helmMajor, tc.wantFlag)
		}
	}
}

func TestHelmOperatorArgs_DisableCSI(t *testing.T) {
	for _, disableCSI := range []bool{true, false} {
		for _, fn := range []struct {
			name string
			args []string
		}{
			{"install", helmOperatorArgs(3, disableCSI)},
			{"upgrade", helmOperatorUpgradeArgs(3, disableCSI)},
		} {
			hasCSIFlag := false
			for i, a := range fn.args {
				if a == "--set" && i+1 < len(fn.args) && fn.args[i+1] == "csidriver.enabled=false" {
					hasCSIFlag = true
				}
			}
			if disableCSI && !hasCSIFlag {
				t.Errorf("%s: disableCSI=true but --set csidriver.enabled=false not found", fn.name)
			}
			if !disableCSI && hasCSIFlag {
				t.Errorf("%s: disableCSI=false but --set csidriver.enabled=false present", fn.name)
			}
		}
	}
}

func TestRenderDynakubeTemplate_TwoDynaKubes(t *testing.T) {
	out, err := renderDynakubeTemplate(baseTemplateData())
	if err != nil {
		t.Fatalf("renderDynakubeTemplate: %v", err)
	}

	if count := strings.Count(out, "kind: DynaKube"); count != 2 {
		t.Errorf("expected 2 DynaKube objects, found %d", count)
	}
	if !strings.Contains(out, "- kubernetes-monitoring") {
		t.Error("expected kubernetes-monitoring capability in DynaKube #1")
	}
	if !strings.Contains(out, "- routing") {
		t.Error("expected routing capability in DynaKube #2")
	}
}

func TestRenderDynakubeTemplate_AgentsDynaKubeStructure(t *testing.T) {
	out, err := renderDynakubeTemplate(baseTemplateData())
	if err != nil {
		t.Fatalf("renderDynakubeTemplate: %v", err)
	}

	for _, want := range []string{
		"applicationMonitoring",
		"extensionExecutionController",
		"otelCollector",
		"logMonitoring",
		"telemetryIngest",
		"- otlp",
		"- jaeger",
		"- statsd",
		"- zipkin",
		"extensions:",
		"prometheus: {}",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("DynaKube #2: expected %q in manifest", want)
		}
	}
}

// createFakeWinget writes a fake winget executable to a temp directory and
// returns the directory path. exitCode controls what the fake binary exits with.
func createFakeWinget(t *testing.T, exitCode int) string {
	t.Helper()
	dir := t.TempDir()

	if runtime.GOOS == "windows" {
		script := filepath.Join(dir, "winget.bat")
		content := "@echo off\r\n"
		if exitCode != 0 {
			content += "exit /b 1\r\n"
		}
		if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
			t.Fatalf("createFakeWinget: %v", err)
		}
	} else {
		script := filepath.Join(dir, "winget")
		content := "#!/bin/sh\n"
		if exitCode != 0 {
			content += "exit 1\n"
		}
		if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
			t.Fatalf("createFakeWinget: %v", err)
		}
	}
	return dir
}

func TestInstallHelmWindows_WingetNotFound(t *testing.T) {
	// Use an empty temp dir so exec.LookPath("winget") fails.
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	err := installHelmWindows()
	if err == nil {
		t.Fatal("expected error when winget is not on PATH")
	}
	if !strings.Contains(err.Error(), "winget was not found") {
		t.Errorf("error should mention 'winget was not found', got: %v", err)
	}
	if !strings.Contains(err.Error(), "https://helm.sh/docs/intro/install/") {
		t.Errorf("error should contain install URL, got: %v", err)
	}
}

func TestInstallHelmWindows_WingetFails(t *testing.T) {
	dir := createFakeWinget(t, 1)
	t.Setenv("PATH", dir)

	err := installHelmWindows()
	if err == nil {
		t.Fatal("expected error when winget exits non-zero")
	}
	if !strings.Contains(err.Error(), "winget failed") {
		t.Errorf("error should mention 'winget failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "https://helm.sh/docs/intro/install/") {
		t.Errorf("error should contain install URL, got: %v", err)
	}
}

func TestInstallHelmWindows_WingetSucceeds(t *testing.T) {
	dir := createFakeWinget(t, 0)
	t.Setenv("PATH", dir)

	if err := installHelmWindows(); err != nil {
		t.Errorf("expected no error when winget succeeds, got: %v", err)
	}
}
