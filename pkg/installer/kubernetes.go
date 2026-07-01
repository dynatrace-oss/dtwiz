package installer

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
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

	// dynatraceOperatorVersion is the operator chart version pinned by dtwiz.
	dynatraceOperatorVersion = "1.10.0-rc.0"
	// dynatraceOperatorOCI is the OCI chart reference hosted on public.ecr.aws.
	dynatraceOperatorOCI = "oci://public.ecr.aws/dynatrace/dynatrace-operator"
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

var k8sNameRe = regexp.MustCompile(`[^a-z0-9-]+`)

// sanitizeK8sName converts a string to a valid RFC 1123 DNS label suitable
// for use as a Kubernetes resource name.
func sanitizeK8sName(name string) string {
	name = strings.ToLower(name)
	name = k8sNameRe.ReplaceAllString(name, "-")
	// Trim leading/trailing hyphens.
	name = strings.Trim(name, "-")
	if name == "" {
		return "dynakube"
	}
	// DynaKube names are limited to 32 characters when OTel collectors are enabled.
	// Kubernetes labels must be ≤ 63 chars; the operator appends suffixes to the
	// DynaKube name in generated labels, so base names are capped at 23 chars.
	const maxLen = 23
	if len(name) > maxLen {
		name = name[:maxLen]
		name = strings.TrimRight(name, "-")
	}
	return name
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
		pods, err := queryPodStatuses()
		if err != nil {
			fmt.Print(clearLine)
			return fmt.Errorf("querying pod statuses: %w", err)
		}

		readyCount := 0
		total := len(pods)
		hasActiveGate := false
		for _, p := range pods {
			if !p.ready {
				continue
			}
			readyCount++
			if strings.Contains(strings.ToLower(p.name), "activegate") {
				hasActiveGate = true
			}
		}

		agGlyph := "…"
		if hasActiveGate {
			agGlyph = "✓"
		}

		now := time.Now()
		elapsed := now.Sub(start)
		fmt.Printf("\r  %d/%d pods ready  activegate: %s  [%s]",
			readyCount, total, agGlyph, formatElapsed(elapsed),
		)

		if total > 0 && readyCount == total && hasActiveGate {
			fmt.Print(clearLine)
			display.Println("All Dynatrace pods ready.")
			return nil
		}

		if now.After(deadline) {
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
	cluster := analyzer.FetchKubeCluster()
	name := sanitizeK8sName(cluster)
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

// buildDynakubeManifest constructs the DynaKube template data for the given
// environment and renders it into a YAML manifest string.
func buildDynakubeManifest(apiURL, token, clusterName, distro string) (string, error) {
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
		return "", fmt.Errorf("rendering DynaKube manifest: %w", err)
	}
	return manifest, nil
}

// helmInstallArgs detects the installed Helm version and returns the major
// version and the appropriate install or upgrade args for the operator chart.
func helmInstallArgs(disableCSI bool) (helmMajor int, args []string) {
	helmMajor = 3
	if isHelmInstalled() {
		if v, err := helmMajorVersion(); err == nil {
			helmMajor = v
		}
	}
	if isOperatorInstalled() {
		return helmMajor, helmOperatorUpgradeArgs(helmMajor, disableCSI)
	}
	return helmMajor, helmOperatorArgs(helmMajor, disableCSI)
}

// ensureHelmOperator installs Helm if absent, then installs or upgrades the
// dynatrace-operator Helm chart. Helm version detection runs after installation
// so the correct args are always used.
func ensureHelmOperator(disableCSI bool) error {
	if !isHelmInstalled() {
		if err := installHelm(); err != nil {
			return fmt.Errorf("helm installation failed: %w", err)
		}
	}
	helmMajor, helmArgs := helmInstallArgs(disableCSI)
	if isOperatorInstalled() {
		display.Println("Dynatrace Operator already installed — upgrading (helm v%d)...", helmMajor)
	} else {
		display.Println("Installing Dynatrace Operator via Helm (helm v%d)...", helmMajor)
	}
	if err := RunCommandQuiet("helm", helmArgs...); err != nil {
		return fmt.Errorf("Helm operator install failed: %w", err) //nolint:staticcheck // ST1005: keep brand capitalization
	}
	display.Println("Helm chart deployed.")
	return nil
}

// handleK8sDryRun prints what the Kubernetes installation would do without executing anything.
func handleK8sDryRun(apiURL, clusterName, distroDisplay, helmArgsStr string) {
	fmt.Println("[dry-run] Would deploy Dynatrace Operator on Kubernetes")
	display.PrintAlignedStatusLines(display.ColorDefault,
		"API URL", apiURL,
		"DynaKube", clusterName,
		"Distribution", distroDisplay,
	)
	display.PrintSteps(
		"Ensure Helm is installed",
		"helm "+helmArgsStr,
		fmt.Sprintf("kubectl apply Secret + DynaKube CRs (cluster: %s)", clusterName),
		"Wait for pods to become ready",
	)
}

// printK8sPreview prints the full installation preview: cluster metadata,
// the rendered DynaKube manifest, and the Helm command to be executed.
func printK8sPreview(clusterName, distroDisplay, apiURL, manifest, helmCmd string) {
	fmt.Println()
	display.PrintlnColored(display.ColorMessage, "Dynatrace Kubernetes Integration")
	display.PrintSectionDivider()
	fmt.Println()
	display.PrintAlignedStatusLines(display.ColorDefault,
		"Cluster name", clusterName,
		"Distribution", distroDisplay,
		"API URL", apiURL,
	)
	fmt.Println()
	display.PrintSectionDivider()
	display.PrintlnColored(display.ColorMessage, "dynakube.yaml manifest to be applied:")
	display.PrintSectionDivider()
	for _, line := range strings.Split(strings.TrimRight(manifest, "\n"), "\n") {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println()
	display.PrintSectionDivider()
	display.PrintlnColored(display.ColorMessage, "Commands to be executed:")
	display.PrintSectionDivider()
	display.PrintStepsColored(display.ColorMessage,
		helmCmd,
		"kubectl apply -f dynakube.yaml  # manifest shown above",
	)
	display.PrintSectionDivider()
	fmt.Println()
}

// InstallKubernetes deploys the Dynatrace Operator on a Kubernetes cluster.
//
//   - envURL:      Dynatrace environment URL
//   - token:       platform token used for both apiToken and dataIngestToken in the DynaKube Secret
//   - clusterName: DynaKube CR name; when empty, derived from the kubectl context or envURL
//   - distro:      detected Kubernetes distribution (e.g. "GKE", "EKS"); empty falls back to defaults
//   - dryRun:      when true, only print what would be done
func InstallKubernetes(envURL, token, clusterName, distro string, dryRun bool) error {
	if err := refreshWindowsPath(); err != nil {
		display.PrintWarning("kubernetes", err)
	}

	apiURL := APIURL(envURL)
	clusterName = resolveClusterName(clusterName, envURL)

	manifest, err := buildDynakubeManifest(apiURL, token, clusterName, distro)
	if err != nil {
		return err
	}

	disableCSI := distro == "GKE-Autopilot"
	_, helmArgs := helmInstallArgs(disableCSI)
	helmArgsStr := strings.Join(helmArgs, " ")
	helmCmd := "helm " + helmArgsStr

	if dryRun {
		handleK8sDryRun(apiURL, clusterName, distro, helmArgsStr)
		return nil
	}

	printK8sPreview(clusterName, distro, apiURL, manifest, helmCmd)

	ok, err := confirmProceed("  Proceed with installation?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		display.Println("Installation cancelled.")
		return ErrInstallCancelled
	}
	fmt.Println()

	if err := ensureHelmOperator(disableCSI); err != nil {
		return err
	}

	display.Println("Applying DynaKube manifests (Secret + DynaKube CRs)...")
	if err := applyDynakube(manifest); err != nil {
		return fmt.Errorf("applying DynaKube manifests: %w", err)
	}

	if err := waitForPods(10 * time.Minute); err != nil {
		return err
	}

	display.Println("Dynatrace Operator installed successfully.")
	return nil
}
