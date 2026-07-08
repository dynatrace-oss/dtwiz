package kubernetes

import (
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/test/helpers"
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
	for _, distro := range []string{analyzer.DistroEKS, analyzer.DistroEKSBottlerocket, analyzer.DistroGKE, analyzer.DistroOpenShift, analyzer.DistroAKS, analyzer.DistroKubernetes, ""} {
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
	for _, distro := range []string{analyzer.DistroEKSBottlerocket, analyzer.DistroEKS, analyzer.DistroGKE, analyzer.DistroOpenShift, analyzer.DistroKubernetes, ""} {
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
	kspmDistros := []string{analyzer.DistroEKS, analyzer.DistroAKS, analyzer.DistroKubernetes, analyzer.DistroMinikube, analyzer.DistroKind, analyzer.DistroK3s, ""}
	for _, distro := range kspmDistros {
		d := distroTemplateData(baseTemplateData(), distro)
		if !d.EnableKSPM {
			t.Errorf("distro %q: expected EnableKSPM=true", distro)
		}
	}

	noKSPMDistros := []string{analyzer.DistroGKE, analyzer.DistroGKEAutopilot, analyzer.DistroOpenShift, analyzer.DistroEKSBottlerocket, analyzer.DistroRKE, analyzer.DistroIKS, analyzer.DistroTKGI}
	for _, distro := range noKSPMDistros {
		d := distroTemplateData(baseTemplateData(), distro)
		if d.EnableKSPM {
			t.Errorf("distro %q: expected EnableKSPM=false", distro)
		}
	}
}

func TestDistroTemplateData_PrivilegedAnnotation(t *testing.T) {
	d := distroTemplateData(baseTemplateData(), analyzer.DistroOpenShift)
	if !d.PrivilegedAnnotation {
		t.Error("OpenShift: expected PrivilegedAnnotation=true")
	}

	for _, distro := range []string{analyzer.DistroEKS, analyzer.DistroAKS, analyzer.DistroGKE, analyzer.DistroEKSBottlerocket, analyzer.DistroKubernetes, ""} {
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
		{analyzer.DistroIKS, "/var/data/kubelet"},
		{analyzer.DistroTKGI, "/var/vcap/data/kubelet"},
	}
	for _, c := range cases {
		d := distroTemplateData(baseTemplateData(), c.distro)
		if d.KubeletPath != c.want {
			t.Errorf("distro %q: KubeletPath = %q, want %q", c.distro, d.KubeletPath, c.want)
		}
	}

	for _, distro := range []string{analyzer.DistroEKS, analyzer.DistroAKS, analyzer.DistroGKE, analyzer.DistroOpenShift, analyzer.DistroKubernetes, ""} {
		d := distroTemplateData(baseTemplateData(), distro)
		if d.KubeletPath != "" {
			t.Errorf("distro %q: expected empty KubeletPath, got %q", distro, d.KubeletPath)
		}
	}
}

func TestRenderDynakubeTemplate_KSPM_Present(t *testing.T) {
	for _, distro := range []string{analyzer.DistroEKS, analyzer.DistroAKS, analyzer.DistroKubernetes, ""} {
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
	for _, distro := range []string{analyzer.DistroGKE, analyzer.DistroGKEAutopilot, analyzer.DistroOpenShift, analyzer.DistroEKSBottlerocket, analyzer.DistroRKE, analyzer.DistroIKS, analyzer.DistroTKGI} {
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
	out, err := renderDynakubeTemplate(distroTemplateData(baseTemplateData(), analyzer.DistroOpenShift))
	if err != nil {
		t.Fatalf("renderDynakubeTemplate: %v", err)
	}
	count := strings.Count(out, "feature.dynatrace.com/oneagent-privileged: \"true\"")
	if count != 2 {
		t.Errorf("OpenShift: expected privileged annotation on both DynaKubes, found %d occurrence(s)", count)
	}
}

func TestRenderDynakubeTemplate_OpenShiftAnnotation_Absent(t *testing.T) {
	for _, distro := range []string{analyzer.DistroEKS, analyzer.DistroAKS, analyzer.DistroGKE, analyzer.DistroEKSBottlerocket, analyzer.DistroKubernetes, ""} {
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
		{analyzer.DistroIKS, "kubeletPath: /var/data/kubelet"},
		{analyzer.DistroTKGI, "kubeletPath: /var/vcap/data/kubelet"},
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

	for _, distro := range []string{analyzer.DistroEKS, analyzer.DistroAKS, analyzer.DistroGKE, analyzer.DistroOpenShift, analyzer.DistroKubernetes, ""} {
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

func TestSanitizeK8sName(t *testing.T) {
	cases := []struct{ input, want string }{
		// Basic lowercasing
		{"MyCluster", "mycluster"},
		// Special chars replaced with hyphens
		{"my_cluster.name", "my-cluster-name"},
		// Leading/trailing hyphens trimmed
		{"--my-cluster--", "my-cluster"},
		// Empty input → fallback
		{"", "dynakube"},
		// All special chars → fallback
		{"___", "dynakube"},
		// 24 chars, truncated to 23 — no trailing hyphen
		{"this-is-a-very-long-clux", "this-is-a-very-long-clu"},
		// 24 chars, truncated to 23 — trailing hyphen trimmed
		{"this-is-a-very-long-cl-x", "this-is-a-very-long-cl"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got := sanitizeK8sName(c.input)
			if got != c.want {
				t.Errorf("sanitizeK8sName(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestBuildDynakubeManifest(t *testing.T) {
	const apiURL = "https://abc123.live.dynatracelabs.com"
	const token = "dt0c01.test-token"
	const clusterName = "test-cluster"

	manifest, err := buildDynakubeManifest(apiURL, token, clusterName, "")
	if err != nil {
		t.Fatalf("buildDynakubeManifest: %v", err)
	}
	if manifest == "" {
		t.Fatal("expected non-empty manifest")
	}
	if !strings.Contains(manifest, apiURL+"/api") {
		t.Errorf("manifest does not contain API URL %q", apiURL+"/api")
	}
	if !strings.Contains(manifest, "name: "+clusterName) {
		t.Errorf("manifest does not contain cluster name %q", clusterName)
	}
	if !strings.Contains(manifest, token) {
		t.Error("manifest does not contain the token")
	}
}

func TestResolveClusterName_ExplicitName(t *testing.T) {
	cases := []struct{ input, want string }{
		{"MyCluster", "mycluster"},
		{"my_cluster", "my-cluster"},
		{"this-is-a-very-long-cluster-name-wow", "this-is-a-very-long-clu"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got := resolveClusterName(c.input, "https://abc123.live.dynatracelabs.com")
			if got != c.want {
				t.Errorf("resolveClusterName(%q, ...) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestResolveClusterName_FallsBackToNonEmpty(t *testing.T) {
	// With no explicit name the function calls fetchClusterName, which always
	// returns at least "dynakube". Verify the result is never empty.
	got := resolveClusterName("", "https://abc123.live.dynatracelabs.com")
	if got == "" {
		t.Error("resolveClusterName with no explicit name returned empty string")
	}
}

func TestHandleK8sDryRun(t *testing.T) {
	// fmt.Println and display.PrintSteps go to os.Stdout (captured by CaptureStdout).
	// display.PrintAlignedStatusLines goes to color.Output (not captured here).
	out := helpers.CaptureStdout(t, func() {
		handleK8sDryRun(
			"https://abc123.live.dynatracelabs.com/api",
			"my-cluster",
			"EKS",
			"install dynatrace-operator ...",
		)
	})
	for _, want := range []string{
		"[dry-run]",
		"Ensure Helm is installed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("handleK8sDryRun: expected %q in output, got: %q", want, out)
		}
	}
}

func TestPrintK8sPreview(t *testing.T) {
	// display.PrintlnColored, PrintAlignedStatusLines, and PrintStepsColored all
	// write to color.Output — use CaptureOutput to capture them.
	manifest := "kind: Secret\nmetadata:\n  name: test\n"
	out := helpers.CaptureOutput(t, func() {
		printK8sPreview(
			"my-cluster",
			"EKS",
			"https://abc123.live.dynatracelabs.com/api",
			manifest,
			"helm install dynatrace-operator ...",
		)
	})
	for _, want := range []string{
		"my-cluster",
		"EKS",
		"helm install",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("printK8sPreview: expected %q in output, got: %q", want, out)
		}
	}
}
