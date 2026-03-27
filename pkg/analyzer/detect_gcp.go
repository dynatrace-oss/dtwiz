package analyzer

import (
	"encoding/json"
	"strings"
	"sync"
)

// detectGCP checks for a gcloud CLI with an active account.
func detectGCP() *GCPInfo {
	info := &GCPInfo{}

	ok, out := runCmd("gcloud", "config", "get-value", "project")
	if !ok || strings.TrimSpace(out) == "" || strings.Contains(out, "(unset)") {
		return info
	}
	info.Available = true
	info.ProjectID = strings.TrimSpace(out)

	_, acct := runCmd("gcloud", "config", "get-value", "account")
	info.Account = strings.TrimSpace(acct)

	info.Services, info.ServicesAuthError = detectGCPServices()
	return info
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
