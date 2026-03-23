package analyzer

import (
	"strconv"
	"strings"
	"sync"
)

// detectAzure checks for an Azure CLI with an active account.
func detectAzure() *AzureInfo {
	info := &AzureInfo{}

	ok, out := runCmd("az", "account", "show", "--query", "id", "-o", "tsv")
	if !ok {
		return info
	}
	info.Available = true
	info.SubscriptionID = strings.TrimSpace(out)

	_, tenant := runCmd("az", "account", "show", "--query", "tenantId", "-o", "tsv")
	info.TenantID = strings.TrimSpace(tenant)

	info.Services, info.ServicesAuthError = detectAzureServices()
	return info
}

type azureServiceProbe struct {
	name    string
	cmd     []string
	countFn func(out string) int
}

func parseAzureCount(out string) int {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var n int
	for _, line := range lines {
		if t := strings.TrimSpace(line); t != "" && t != "[]" {
			n++
		}
	}
	return n
}

func parseAzureInt(out string) int {
	f := strings.Fields(strings.TrimSpace(out))
	if len(f) == 0 {
		return 0
	}
	n, err := strconv.Atoi(f[0])
	if err != nil {
		return 0
	}
	return n
}

var azureProbes = []azureServiceProbe{
	{
		name:    "VMs",
		cmd:     []string{"az", "vm", "list", "--query", "length(@)", "-o", "tsv"},
		countFn: parseAzureInt,
	},
	{
		name:    "AKS",
		cmd:     []string{"az", "aks", "list", "--query", "length(@)", "-o", "tsv"},
		countFn: parseAzureInt,
	},
	{
		name:    "Functions",
		cmd:     []string{"az", "functionapp", "list", "--query", "length(@)", "-o", "tsv"},
		countFn: parseAzureInt,
	},
	{
		name:    "App Services",
		cmd:     []string{"az", "webapp", "list", "--query", "length(@)", "-o", "tsv"},
		countFn: parseAzureInt,
	},
	{
		name:    "SQL DBs",
		cmd:     []string{"az", "sql", "db", "list", "--query", "length(@)", "-o", "tsv"},
		countFn: parseAzureInt,
	},
	{
		name:    "Storage",
		cmd:     []string{"az", "storage", "account", "list", "--query", "length(@)", "-o", "tsv"},
		countFn: parseAzureInt,
	},
}

func detectAzureServices() ([]AzureService, bool) {
	type result struct {
		svc       AzureService
		valid     bool
		authError bool
	}
	results := make([]result, len(azureProbes))
	var wg sync.WaitGroup
	for i, probe := range azureProbes {
		wg.Add(1)
		go func(i int, probe azureServiceProbe) {
			defer wg.Done()
			ok, out := runCmd(probe.cmd[0], probe.cmd[1:]...)
			if !ok {
				if strings.Contains(out, "AADSTS50078") || strings.Contains(out, "Interactive authentication is needed") {
					results[i] = result{authError: true}
				}
				return
			}
			count := probe.countFn(out)
			if count > 0 {
				results[i] = result{svc: AzureService{Name: probe.name, Count: count}, valid: true}
			}
		}(i, probe)
	}
	wg.Wait()
	var services []AzureService
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
