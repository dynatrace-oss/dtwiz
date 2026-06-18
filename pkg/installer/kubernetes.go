package installer

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
)

//go:embed dynakube.tmpl
var dynakubeTemplateText string

// Pinned image references used in the DynaKube manifest.
// Update these constants when rolling forward to a new Dynatrace release.
const (
	dynakubeActiveGateImage  = "public.ecr.aws/dynatrace/dynatrace-activegate:1.337.36.20260526-135434"
	dynakubeEECRepository    = "public.ecr.aws/dynatrace/dynatrace-eec"
	dynakubeEECTag           = "1.337.60.20260603-063549"
	dynakubeCodeModulesImage = "public.ecr.aws/dynatrace/dynatrace-codemodules:1.337.60.20260603-063549"

	// helmOperatorNightlyVersion is used for all non-GKE distros.
	helmOperatorNightlyVersion = "0.0.0-nightly-chart"
	// helmChartGHCR is the default chart source for non-GKE distros.
	helmChartGHCR = "oci://ghcr.io/dynatrace/dynatrace-operator"
)

// dynakubeTemplateData holds the values substituted into dynakube.tmpl.
type dynakubeTemplateData struct {
	ClusterName          string // sanitised Kubernetes resource name
	APIURL               string // full Dynatrace API URL incl. /api suffix
	APIToken             string // raw API token
	DataIngestToken      string // raw data-ingest token
	ActiveGateImage      string // full image reference for ActiveGate pods
	EECRepository        string // OCI repository for the EEC image
	EECTag               string // tag for the EEC image
	CodeModulesImage     string // full image reference for OneAgent code modules
	EnableKSPM           bool   // inject kspm.mappedHostPaths + kspmNodeConfigurationCollector block
	PrivilegedAnnotation bool   // add feature.dynatrace.com/oneagent-privileged: "true" (OpenShift)
	ReadOnlyVolume       bool   // add feature.dynatrace.com/injection-readonly-volume: "true" (Bottlerocket)
	KubeletPath          string // non-standard kubelet path (IKS: /var/data/kubelet, TKGI: /var/vcap/data/kubelet)
}

// distroTemplateData applies per-distribution overrides to the base template
// data. Distributions not listed here use no KSPM, no annotations, and no
// kubeletPath override (GKE, GKE-Autopilot, RKE).
func distroTemplateData(base dynakubeTemplateData, distro string) dynakubeTemplateData {
	switch distro {
	case "EKS", "AKS", "kubernetes", "minikube", "kind", "k3s", "":
		base.EnableKSPM = true
	case "OpenShift":
		base.PrivilegedAnnotation = true
	case "EKS-Bottlerocket":
		base.ReadOnlyVolume = true
	case "IKS":
		base.KubeletPath = "/var/data/kubelet"
	case "TKGI":
		base.KubeletPath = "/var/vcap/data/kubelet"
	}
	return base
}

// renderDynakubeTemplate fills dynakube.tmpl with the provided data and
// returns the rendered YAML manifest.
func renderDynakubeTemplate(d dynakubeTemplateData) (string, error) {
	tmpl, err := template.New("dynakube").Parse(dynakubeTemplateText)
	if err != nil {
		return "", fmt.Errorf("parsing dynakube template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, d); err != nil {
		return "", fmt.Errorf("rendering dynakube template: %w", err)
	}
	return buf.String(), nil
}

// sanitizeK8sName converts a string to a valid RFC 1123 DNS label suitable
// for use as a Kubernetes resource name.
func sanitizeK8sName(name string) string {
	name = strings.ToLower(name)
	// Replace any character that is not alphanumeric or hyphen with a hyphen.
	re := regexp.MustCompile(`[^a-z0-9-]+`)
	name = re.ReplaceAllString(name, "-")
	// Trim leading/trailing hyphens.
	name = strings.Trim(name, "-")
	if name == "" {
		return "dynakube"
	}
	// DynaKube names are limited to 32 characters when OTel collectors are enabled.
	// The operator also generates labels which adds 39 chars to the base name.
	// Kubernetes labels must be ≤ 63 chars, so base ≤ 24.
	const maxLen = 23
	if len(name) > maxLen {
		name = name[:maxLen]
		name = strings.TrimRight(name, "-")
	}
	return name
}

// isHelmInstalled returns true when the `helm` binary is on PATH.
func isHelmInstalled() bool {
	_, err := exec.LookPath("helm")
	return err == nil
}

// helmMajorVersion returns the major version number of the installed Helm CLI.
func helmMajorVersion() (int, error) {
	out, err := exec.Command("helm", "version", "--short").Output()
	if err != nil {
		return 0, fmt.Errorf("getting helm version: %w", err)
	}
	// Output looks like: v3.14.0+g... or v4.0.0+g...
	ver := strings.TrimSpace(string(out))
	ver = strings.TrimPrefix(ver, "v")
	parts := strings.SplitN(ver, ".", 2)
	if len(parts) == 0 {
		return 0, fmt.Errorf("unexpected helm version output: %q", ver)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("parsing helm major version from %q: %w", ver, err)
	}
	return major, nil
}

// installHelm attempts to install Helm via the official get-helm-3 script.
// NOTE: This downloads and executes a script from the internet.  Users who
// require a verified installation should install Helm manually:
//
//	https://helm.sh/docs/intro/install/
func installHelm() error {
	fmt.Println("  Helm not found — installing via get.helm.sh...")
	fmt.Println("  NOTE: This executes a script from https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3")
	return RunCommand("bash", "-c",
		"curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash")
}

// isOperatorInstalled checks whether the dynatrace-operator Helm release
// exists in the dynatrace namespace.
func isOperatorInstalled() bool {
	out, err := exec.Command("helm", "list",
		"--namespace", "dynatrace",
		"--filter", "dynatrace-operator",
		"--short").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "dynatrace-operator")
}

// helmOperatorArgs builds the `helm install` argument slice.
// Helm v3 uses --atomic; Helm v4+ uses --rollback-on-failure.
// disableCSI adds --set csidriver.enabled=false, required on GKE Autopilot.
// version selects the chart version (nightly or stable).
func helmOperatorArgs(helmMajor int, disableCSI bool) []string {
	rollbackFlag := "--atomic"
	if helmMajor >= 4 {
		rollbackFlag = "--rollback-on-failure"
	}
	args := []string{
		"install", "dynatrace-operator",
		helmChartGHCR,
		"--version", helmOperatorNightlyVersion,
		"--create-namespace",
		"--namespace", "dynatrace",
		rollbackFlag,
		"--timeout", "10m",
	}
	if disableCSI {
		args = append(args, "--set", "csidriver.enabled=false")
	}
	return args
}

// helmOperatorUpgradeArgs builds the `helm upgrade` argument slice used when
// the dynatrace-operator release already exists.
// disableCSI adds --set csidriver.enabled=false, required on GKE Autopilot.
// version selects the chart version (nightly or stable).
func helmOperatorUpgradeArgs(helmMajor int, disableCSI bool) []string {
	rollbackFlag := "--atomic"
	if helmMajor >= 4 {
		rollbackFlag = "--rollback-on-failure"
	}
	args := []string{
		"upgrade", "dynatrace-operator",
		helmChartGHCR,
		"--version", helmOperatorNightlyVersion,
		"--namespace", "dynatrace",
		rollbackFlag,
		"--timeout", "10m",
	}
	if disableCSI {
		args = append(args, "--set", "csidriver.enabled=false")
	}
	return args
}

// applyDynakube writes the DynaKube CR YAML to a temp file and runs
// `kubectl apply -f` on it.
func applyDynakube(yaml string) error {
	tmpFile, err := os.CreateTemp("", "dynakube-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp file for DynaKube CR: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yaml); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing DynaKube CR to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing DynaKube CR temp file: %w", err)
	}

	return RunCommandQuiet("kubectl", "apply", "-f", tmpFile.Name())
}

// podStatus holds the ready state of a single pod.
type podStatus struct {
	name  string
	ready bool
}

// queryPodStatuses fetches pod info from the dynatrace namespace.
// A pod is counted ready when all its containers are ready and its phase is Running.
func queryPodStatuses() ([]podStatus, error) {
	out, err := exec.Command("kubectl", "get", "pods",
		"--namespace", "dynatrace",
		"--no-headers").Output()
	if err != nil {
		return nil, err
	}
	var pods []podStatus
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name, readyCol, phase := fields[0], fields[1], fields[2]
		ready := false
		if phase == "Running" {
			parts := strings.SplitN(readyCol, "/", 2)
			if len(parts) == 2 && parts[0] != "0" && parts[0] == parts[1] {
				ready = true
			}
		}
		pods = append(pods, podStatus{name: name, ready: ready})
	}
	return pods, nil
}

// waitForPods polls until every pod in the dynatrace namespace is ready and at
// least one pod whose name contains "activegate" exists and is ready.
// Progress is printed on a single line that refreshes every 5 seconds.
func waitForPods(timeout time.Duration) error {
	start := time.Now()
	deadline := start.Add(timeout)

	formatElapsed := func(d time.Duration) string {
		d = d.Round(time.Second)
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%d:%02d", m, s)
	}

	clearLine := "\r" + strings.Repeat(" ", 60) + "\r"

	fmt.Print("  0/0 pods ready  activegate: …  [0:00]")

	for {
		pods, _ := queryPodStatuses()

		readyCount := 0
		total := len(pods)
		hasActiveGate := false
		for _, p := range pods {
			if p.ready {
				readyCount++
			}
			if strings.Contains(strings.ToLower(p.name), "activegate") && p.ready {
				hasActiveGate = true
			}
		}

		elapsed := time.Since(start)
		fmt.Printf("\r  %d/%d pods ready  activegate: %s  [%s]",
			readyCount, total,
			map[bool]string{true: "✓", false: "…"}[hasActiveGate],
			formatElapsed(elapsed),
		)

		if total > 0 && readyCount == total && hasActiveGate {
			fmt.Print(clearLine)
			fmt.Println("  All Dynatrace pods ready.")
			return nil
		}

		if time.Now().After(deadline) {
			fmt.Print(clearLine)
			return fmt.Errorf("timed out after %s: %d/%d pods ready, activegate ready: %v",
				elapsed.Round(time.Second), readyCount, total, hasActiveGate)
		}

		time.Sleep(5 * time.Second)
	}
}

// fetchClusterName returns the current kubectl cluster name, sanitized for use
// as a Kubernetes resource name. Falls back to fallback if detection fails.
func fetchClusterName(fallback string) string {
	out, err := exec.Command("kubectl", "config", "view",
		"--minify", "-o", "jsonpath={.clusters[0].name}").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return fallback
	}
	name := sanitizeK8sName(strings.TrimSpace(string(out)))
	if name == "" {
		return fallback
	}
	return name
}

// resolveClusterName returns a sanitized Kubernetes resource name for the DynaKube CR.
// Priority: explicit name → kubectl context → tenant ID derived from envURL → "dynakube".
func resolveClusterName(name, envURL string) string {
	if name != "" {
		return sanitizeK8sName(name)
	}
	if resolved := fetchClusterName(sanitizeK8sName(ExtractTenantID(envURL))); resolved != "" {
		return resolved
	}
	return "dynakube"
}

// InstallKubernetes deploys the Dynatrace Operator on a Kubernetes cluster.
//
// Parameters:
// Parameters:
//   - envURL:      Dynatrace environment URL
//   - token:       platform token used for both apiToken and dataIngestToken in the DynaKube Secret
//   - clusterName: DynaKube CR name; when empty, it is derived from the current kubectl context (falling back to a value derived from envURL)
//   - distro:      detected Kubernetes distribution (e.g. "GKE", "EKS"); empty falls back to defaults
//   - dryRun:      when true, only print what would be done
func InstallKubernetes(envURL, token, clusterName, distro string, dryRun bool) error {
	apiURL := APIURL(envURL)

	clusterName = resolveClusterName(clusterName, envURL)

	// --- Build manifest ---
	tmplData := distroTemplateData(dynakubeTemplateData{
		ClusterName:      clusterName,
		APIURL:           apiURL + "/api",
		APIToken:         token,
		DataIngestToken:  token,
		ActiveGateImage:  dynakubeActiveGateImage,
		EECRepository:    dynakubeEECRepository,
		EECTag:           dynakubeEECTag,
		CodeModulesImage: dynakubeCodeModulesImage,
	}, distro)
	manifest, err := renderDynakubeTemplate(tmplData)
	if err != nil {
		return fmt.Errorf("rendering DynaKube manifest: %w", err)
	}

	// --- Determine Helm command ---
	disableCSI := distro == "GKE-Autopilot"
	var helmArgs []string
	helmMajor := 3 // sensible default for display; re-detected before execution
	if isHelmInstalled() {
		if v, err := helmMajorVersion(); err == nil {
			helmMajor = v
		}
	}
	if isOperatorInstalled() {
		helmArgs = helmOperatorUpgradeArgs(helmMajor, disableCSI)
	} else {
		helmArgs = helmOperatorArgs(helmMajor, disableCSI)
	}
	helmCmd := "helm " + strings.Join(helmArgs, " ")

	if dryRun {
		fmt.Println("[dry-run] Would deploy Dynatrace Operator on Kubernetes")
		fmt.Printf("  API URL:      %s\n", apiURL)
		fmt.Printf("  DynaKube:     %s\n", clusterName)
		fmt.Printf("  Distribution: %s\n", distro)
		fmt.Println("  Steps:")
		fmt.Println("    1. Ensure Helm is installed")
		fmt.Printf("    2. %s\n", helmCmd)
		fmt.Printf("    3. kubectl apply Secret + DynaKube CRs (cluster: %s)\n", clusterName)
		fmt.Println("    4. Wait for pods to become ready")
		return nil
	}

	// --- Preview ---
	sep := strings.Repeat("─", 60)

	fmt.Println()
	display.ColorMessage.Println("  Dynatrace Kubernetes Integration")
	fmt.Println()
	fmt.Printf("  Cluster name:  %s\n", clusterName)
	fmt.Printf("  Distribution:  %s\n", distro)
	fmt.Printf("  API URL:       %s\n\n", apiURL)
	fmt.Printf("  %s\n", sep)
	display.ColorMessage.Println("  dynakube.yaml manifest to be applied:")
	fmt.Printf("  %s\n", sep)
	for _, line := range strings.Split(strings.TrimRight(manifest, "\n"), "\n") {
		fmt.Printf("    %s\n", line)
	}
	fmt.Printf("\n  %s\n", sep)
	display.ColorMessage.Printf("  Commands to be executed:\n")
	fmt.Printf("  %s\n", sep)
	display.ColorMessage.Printf("    1. %s\n", helmCmd)
	display.ColorMessage.Printf("    2. kubectl apply -f dynakube.yaml  # manifest shown above\n")
	fmt.Printf("  %s\n\n", sep)

	// --- Confirm ---
	ok, err := confirmProceed("  Proceed with installation?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Installation cancelled.")
		return ErrInstallCancelled
	}
	fmt.Println()

	// 1. Ensure Helm is present.
	if !isHelmInstalled() {
		if err := installHelm(); err != nil {
			return fmt.Errorf("helm installation failed: %w", err)
		}
		// Re-detect version after installation.
		if v, err := helmMajorVersion(); err == nil {
			helmMajor = v
			if isOperatorInstalled() {
				helmArgs = helmOperatorUpgradeArgs(helmMajor, disableCSI)
			} else {
				helmArgs = helmOperatorArgs(helmMajor, disableCSI)
			}
		}
	}

	// 2. Install / upgrade dynatrace-operator via Helm.
	if isOperatorInstalled() {
		fmt.Printf("  Dynatrace Operator already installed — upgrading (helm v%d)...\n", helmMajor)
	} else {
		fmt.Printf("  Installing Dynatrace Operator via Helm (helm v%d)...\n", helmMajor)
	}
	if err := RunCommandQuiet("helm", helmArgs...); err != nil {
		return fmt.Errorf("Helm operator install failed: %w", err) //nolint:staticcheck // ST1005: keep brand capitalization
	}
	fmt.Println("  Helm chart deployed.")

	// 3. Apply manifest (Secret + DynaKube CRs in one pass).
	fmt.Println("  Applying DynaKube manifests (Secret + DynaKube CRs)...")
	if err := applyDynakube(manifest); err != nil {
		return fmt.Errorf("applying DynaKube manifests: %w", err)
	}

	// 4. Wait for pods.
	if err := waitForPods(10 * time.Minute); err != nil {
		return err
	}

	fmt.Println("  Dynatrace Operator installed successfully.")
	return nil
}
