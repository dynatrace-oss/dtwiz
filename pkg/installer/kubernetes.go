package installer

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/fatih/color"
)

//go:embed dynakube.tmpl
var dynakubeTemplateText string

// dynakubeTemplateData holds the values substituted into dynakube.tmpl.
type dynakubeTemplateData struct {
	ClusterName     string // sanitised Kubernetes resource name
	APIURL          string // full Dynatrace API URL incl. /api suffix
	APIToken        string // raw API token
	DataIngestToken string // raw data-ingest token
	EECRepository   string // OCI repository for the EEC image
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

// eecRepositoryFromAPIURL derives the EEC OCI repository host path from the
// Dynatrace API URL, e.g.
//
//	"https://abc123.live.dynatracelabs.com/api" → "abc123.live.dynatracelabs.com/linux/dynatrace-eec"
func eecRepositoryFromAPIURL(apiURL string) string {
	u, err := url.Parse(apiURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Host + "/linux/dynatrace-eec"
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
	// Kubernetes names must be at most 63 characters.
	if len(name) > 63 {
		name = name[:63]
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
func helmOperatorArgs(helmMajor int) []string {
	rollbackFlag := "--atomic"
	if helmMajor >= 4 {
		rollbackFlag = "--rollback-on-failure"
	}
	return []string{
		"install", "dynatrace-operator",
		"oci://public.ecr.aws/dynatrace/dynatrace-operator",
		"--create-namespace",
		"--namespace", "dynatrace",
		rollbackFlag,
	}
}

// helmOperatorUpgradeArgs builds the `helm upgrade` argument slice used when
// the dynatrace-operator release already exists.
func helmOperatorUpgradeArgs(helmMajor int) []string {
	rollbackFlag := "--atomic"
	if helmMajor >= 4 {
		rollbackFlag = "--rollback-on-failure"
	}
	return []string{
		"upgrade", "dynatrace-operator",
		"oci://public.ecr.aws/dynatrace/dynatrace-operator",
		"--namespace", "dynatrace",
		rollbackFlag,
	}
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

// InstallKubernetes deploys the Dynatrace Operator on a Kubernetes cluster.
//
// Parameters:
//   - envURL:    Dynatrace environment URL
//   - token:     data-ingest token (used as dataIngestToken in the Secret)
//   - apiToken:  Classic Dynatrace access token (dt0c01.*) used as apiToken in the
//     DynaKube Secret; falls back to token when empty
//   - name:      DynaKube CR name (auto-derived from envURL if empty)
//   - dryRun:    when true, only print what would be done
func InstallKubernetes(envURL, token, apiToken, name string, dryRun bool) error {
	if apiToken == "" {
		apiToken = token
	}
	apiURL := APIURL(envURL)

	if name == "" {
		name = fetchClusterName(sanitizeK8sName(ExtractTenantID(envURL)))
		if name == "" {
			name = "dynakube"
		}
	}

	// --- Build manifest ---
	tmplData := dynakubeTemplateData{
		ClusterName:     name,
		APIURL:          apiURL + "/api",
		APIToken:        apiToken,
		DataIngestToken: apiToken,
		EECRepository:   eecRepositoryFromAPIURL(apiURL + "/api"),
	}
	manifest, err := renderDynakubeTemplate(tmplData)
	if err != nil {
		return fmt.Errorf("rendering DynaKube manifest: %w", err)
	}

	// --- Determine Helm command ---
	var helmArgs []string
	helmMajor := 3 // sensible default for display; re-detected before execution
	if isHelmInstalled() {
		if v, err := helmMajorVersion(); err == nil {
			helmMajor = v
		}
	}
	if isOperatorInstalled() {
		helmArgs = helmOperatorUpgradeArgs(helmMajor)
	} else {
		helmArgs = helmOperatorArgs(helmMajor)
	}
	helmCmd := "helm " + strings.Join(helmArgs, " ")

	if dryRun {
		fmt.Println("[dry-run] Would deploy Dynatrace Operator on Kubernetes")
		fmt.Printf("  API URL:    %s\n", apiURL)
		fmt.Printf("  DynaKube:   %s\n", name)
		fmt.Println("  Steps:")
		fmt.Println("    1. Ensure Helm is installed")
		fmt.Printf("    2. %s\n", helmCmd)
		fmt.Printf("    3. kubectl apply Secret + DynaKube CRs (cluster: %s)\n", name)
		fmt.Println("    4. Wait for pods to become ready")
		return nil
	}

	// --- Preview ---
	cyan := color.New(color.FgMagenta)
	sep := strings.Repeat("─", 60)

	fmt.Println()
	cyan.Println("  Dynatrace Kubernetes Integration")
	fmt.Println()
	fmt.Printf("  Cluster name:  %s\n", name)
	fmt.Printf("  API URL:       %s\n\n", apiURL)
	fmt.Printf("  %s\n", sep)
	cyan.Println("  dynakube.yaml manifest to be applied:")
	fmt.Printf("  %s\n", sep)
	for _, line := range strings.Split(strings.TrimRight(manifest, "\n"), "\n") {
		fmt.Printf("    %s\n", line)
	}
	fmt.Printf("\n  %s\n", sep)
	cyan.Printf("  Commands to be executed:\n")
	fmt.Printf("  %s\n", sep)
	cyan.Printf("    1. %s\n", helmCmd)
	cyan.Printf("    2. kubectl apply -f dynakube.yaml  # manifest shown above\n")
	fmt.Printf("  %s\n\n", sep)

	// --- Confirm ---
	ok, err := confirmProceed("  Proceed with installation?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		fmt.Println("  Installation cancelled.")
		return nil
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
				helmArgs = helmOperatorUpgradeArgs(helmMajor)
			} else {
				helmArgs = helmOperatorArgs(helmMajor)
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
		return fmt.Errorf("Helm operator install failed: %w", err)  //nolint:ST1005 to keep brand capitalization
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
