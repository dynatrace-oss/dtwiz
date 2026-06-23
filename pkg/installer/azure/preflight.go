package azure

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// azurePreflightChecks runs all pre-mutation checks:
//  1. az on PATH
//  2. az account show — must succeed (user is logged in)
//  3. auto-detect management group
//  4. RBAC checkAccess hard gate
func azurePreflightChecks(runner cmdRunner, envURL, platformToken string) (subscriptionID, tenantID, mgmtGroupID string, err error) {
	// 1. az must be on PATH
	if _, err = execLookPath("az"); err != nil {
		return "", "", "", fmt.Errorf("Azure CLI (az) not found — install it from https://aka.ms/installazurecliwindows") //nolint:staticcheck // ST1005: "Azure CLI" is a product name
	}

	// 2. Check Azure login
	accountJSON, err := runner("az", []string{"account", "show", "-o", "json"}, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("Not logged in to Azure — run `az login` and retry") //nolint:staticcheck // ST1005: user-facing message
	}

	var account struct {
		ID     string `json:"id"`
		Tenant string `json:"tenantId"`
	}
	if err = json.Unmarshal([]byte(accountJSON), &account); err != nil {
		return "", "", "", fmt.Errorf("parsing az account show output: %w", err)
	}
	subscriptionID = account.ID
	tenantID = account.Tenant
	logger.Debug("az account show", "subscriptionID", subscriptionID, "tenantID", tenantID)

	// 3. Detect management group
	mgmtGroupID, err = azureDetectMgmtGroup(runner, subscriptionID, tenantID)
	if err != nil {
		return "", "", "", err
	}
	logger.Debug("management group selected", "scope", mgmtGroupID)

	// 4. RBAC checkAccess
	if err = azureCheckRBAC(runner, mgmtGroupID); err != nil {
		return "", "", "", err
	}
	logger.Debug("RBAC check passed", "scope", mgmtGroupID)

	return subscriptionID, tenantID, mgmtGroupID, nil
}

// azureDetectMgmtGroup auto-detects the management group scope to use.
// It tries to find the tenant root group (whose id ends with the tenant ID).
// Falls back to subscription scope if the az command fails.
func azureDetectMgmtGroup(runner cmdRunner, subscriptionID, tenantID string) (string, error) {
	out, err := runner("az", []string{"account", "management-group", "list", "-o", "json"}, nil)
	if err != nil {
		display.ColorWarning.Printf("  Warning: could not list management groups (%v); using subscription scope\n", err)
		return "/subscriptions/" + subscriptionID, nil
	}

	type mgGroup struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var groups []mgGroup
	if err = json.Unmarshal([]byte(out), &groups); err != nil {
		display.ColorWarning.Printf("  Warning: could not parse management groups output; using subscription scope\n")
		return "/subscriptions/" + subscriptionID, nil
	}

	if len(groups) == 0 {
		return "/subscriptions/" + subscriptionID, nil
	}

	// Look for the tenant root group (id ends with the tenant ID)
	for _, g := range groups {
		if strings.HasSuffix(g.ID, "/"+tenantID) {
			return g.ID, nil
		}
	}

	// Single group — use it
	if len(groups) == 1 {
		return groups[0].ID, nil
	}

	// Multiple non-root groups: prompt user to select
	fmt.Println("  Available management groups:")
	for i, g := range groups {
		fmt.Printf("    %d) %s  (%s)\n", i+1, g.Name, g.ID)
	}
	fmt.Printf("  Select a management group [1-%d]: ", len(groups))

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "", fmt.Errorf("reading management group selection: %w", scanner.Err())
	}
	text := strings.TrimSpace(scanner.Text())
	idx := 0
	if _, err = fmt.Sscanf(text, "%d", &idx); err != nil || idx < 1 || idx > len(groups) {
		return "", fmt.Errorf("invalid selection %q — expected 1-%d", text, len(groups))
	}
	return groups[idx-1].ID, nil
}

// azureCheckRBAC verifies the current principal has Microsoft.Authorization/roleAssignments/write
// at the management group scope. Aborts if access is not allowed.
func azureCheckRBAC(runner cmdRunner, mgmtGroupID string) error {
	// Extract the bare segment (group name) from the full resource path.
	mgSegment := mgmtGroupID
	if idx := strings.LastIndex(mgmtGroupID, "/"); idx >= 0 {
		mgSegment = mgmtGroupID[idx+1:]
	}

	// Subscription fallback scope — skip mgmt-group-specific checkAccess.
	if strings.HasPrefix(mgmtGroupID, "/subscriptions/") {
		return nil
	}

	url := fmt.Sprintf(
		"https://management.azure.com/providers/Microsoft.Management/managementGroups/%s/providers/Microsoft.Authorization/checkAccess?api-version=2022-04-01",
		mgSegment,
	)
	body := `{"actions":[{"id":"Microsoft.Authorization/roleAssignments/write"}]}`
	logger.Debug("checking RBAC", "url", url)

	out, err := runner("az", []string{"rest", "--method", "POST", "--url", url, "--body", body}, nil)
	if err != nil {
		return fmt.Errorf("RBAC check failed: %w", err)
	}
	if !strings.Contains(out, `"accessDecision":"Allowed"`) {
		return fmt.Errorf("insufficient Azure RBAC permissions — need Microsoft.Authorization/roleAssignments/write at management group scope")
	}
	return nil
}
