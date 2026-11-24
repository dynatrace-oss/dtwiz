package analyzer

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// gcloudReadTimeout bounds each gcloud config/auth read during detection.
const gcloudReadTimeout = 20 * time.Second

// detectGCP checks for a gcloud CLI with an active account and an active project.
func detectGCP() *GCPInfo {
	return detectGCPWith(runCmdWithTimeout, detectGCPServices)
}

// detectGCPWith resolves the active account first, then the project, mirroring the
// installer preflight. run must return stdout only: gcloud prints notices and warnings
// (e.g. "WARNING: The following filter keys were not present ...") to stderr, which
// must never be mistaken for an account or project value.
func detectGCPWith(
	run func(timeout time.Duration, command string, args ...string) (string, error),
	probeServices func() ([]GCPService, bool),
) *GCPInfo {
	info := &GCPInfo{}

	acctOut, err := run(gcloudReadTimeout, "gcloud", "auth", "list", "--filter=status:ACTIVE", "--format=value(account)")
	if err != nil {
		return info // gcloud missing or failing: treat as not signed in
	}
	account := firstLine(acctOut)
	if account == "" {
		return info
	}
	info.Authenticated = true
	info.Account = account

	projOut, err := run(gcloudReadTimeout, "gcloud", "config", "get-value", "project")
	projectID := CleanGCloudConfigValue(projOut)
	if err != nil || projectID == "" || strings.Contains(projectID, "(unset)") {
		return info
	}
	info.Available = true
	info.ProjectID = projectID

	info.Services, info.ServicesAuthError = probeServices()
	return info
}

// firstLine returns the first non-empty trimmed line of out.
func firstLine(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// CleanGCloudConfigValue extracts the value from `gcloud config get-value ...` output.
// In some environments (e.g. Cloud Shell with multiple named configurations) gcloud
// prefixes the value with a "Your active configuration is: [...]" notice line; this
// returns the last non-empty, non-notice line instead of the raw multi-line output.
// Exported so pkg/installer/gcp (which already depends on pkg/analyzer, same direction
// as pkg/installer/kubernetes.go) can parse the identical `gcloud config get-value`
// output without pkg/analyzer importing pkg/installer, which would create an import cycle.
func CleanGCloudConfigValue(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "Your active configuration is:") {
			continue
		}
		return line
	}
	return ""
}

type gcpServiceProbe struct {
	name    string
	cmd     []string
	countFn func(out string) int
}

// parseGCPJSONArray parses gcloud JSON list output and returns len(array).
func parseGCPJSONArray(out string) int {
	out = strings.TrimSpace(out)
	if out == "" || out == "[]" {
		return 0
	}
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		return 0
	}
	return len(arr)
}

var gcpProbes = []gcpServiceProbe{
	{
		name:    "Compute VMs",
		cmd:     []string{"gcloud", "compute", "instances", "list", "--format=json", "--verbosity=none"},
		countFn: parseGCPJSONArray,
	},
	{
		name:    "GKE",
		cmd:     []string{"gcloud", "container", "clusters", "list", "--format=json", "--verbosity=none"},
		countFn: parseGCPJSONArray,
	},
	{
		name:    "Cloud Functions",
		cmd:     []string{"gcloud", "functions", "list", "--format=json", "--verbosity=none"},
		countFn: parseGCPJSONArray,
	},
	{
		name:    "Cloud Run",
		cmd:     []string{"gcloud", "run", "services", "list", "--format=json", "--verbosity=none"},
		countFn: parseGCPJSONArray,
	},
	{
		name:    "Cloud SQL",
		cmd:     []string{"gcloud", "sql", "instances", "list", "--format=json", "--verbosity=none"},
		countFn: parseGCPJSONArray,
	},
	{
		name:    "GCS Buckets",
		cmd:     []string{"gcloud", "storage", "buckets", "list", "--format=json", "--verbosity=none"},
		countFn: parseGCPJSONArray,
	},
}

func detectGCPServices() ([]GCPService, bool) {
	type result struct {
		svc       GCPService
		valid     bool
		authError bool
	}
	results := make([]result, len(gcpProbes))
	var wg sync.WaitGroup
	for i, probe := range gcpProbes {
		wg.Add(1)
		go func(i int, probe gcpServiceProbe) {
			defer wg.Done()
			ok, out := runCmd(probe.cmd[0], probe.cmd[1:]...)
			if !ok {
				if strings.Contains(out, "UNAUTHENTICATED") || strings.Contains(out, "Reauthentication required") || strings.Contains(out, "access token is expired") {
					results[i] = result{authError: true}
				}
				return
			}
			count := probe.countFn(out)
			if count > 0 {
				results[i] = result{svc: GCPService{Name: probe.name, Count: count}, valid: true}
			}
		}(i, probe)
	}
	wg.Wait()
	var services []GCPService
	for _, r := range results {
		if r.authError {
			return nil, true
		}
		if r.valid {
			services = append(services, r.svc)
		}
	}
	return services, false
}
