package azure

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// azurePreflightChecks runs all pre-mutation checks and returns the subscription
// scope used for the integration:
//  1. az on PATH
//  2. az account show — must succeed (user is logged in)
//  3. RBAC checkAccess hard gate at subscription scope
//
// The integration always operates at subscription scope (/subscriptions/<id>).
func azurePreflightChecks(runner cmdRunner, envURL, platformToken string) (subscriptionID, tenantID string, err error) {
	// 1. az must be on PATH
	if _, err = execLookPath("az"); err != nil {
		return "", "", fmt.Errorf("Azure CLI (az) not found — install it from https://aka.ms/installazurecliwindows") //nolint:staticcheck // ST1005: "Azure CLI" is a product name
	}

	// 2. Check Azure login
	accountJSON, err := runner("az", []string{"account", "show", "-o", "json"}, nil)
	if err != nil {
		return "", "", fmt.Errorf("Not logged in to Azure — run `az login` and retry") //nolint:staticcheck // ST1005: user-facing message
	}

	var account struct {
		ID     string `json:"id"`
		Tenant string `json:"tenantId"`
	}
	if err = json.Unmarshal([]byte(accountJSON), &account); err != nil {
		return "", "", fmt.Errorf("parsing az account show output: %w", err)
	}
	subscriptionID = account.ID
	tenantID = account.Tenant
	logger.Debug("az account show", "subscriptionID", subscriptionID, "tenantID", tenantID)

	// 3. RBAC checkAccess at subscription scope
	if err = azureCheckRBAC(runner, "/subscriptions/"+subscriptionID); err != nil {
		return "", "", err
	}
	logger.Debug("RBAC check passed", "scope", "/subscriptions/"+subscriptionID)

	return subscriptionID, tenantID, nil
}

// azureCheckRBAC verifies the current principal has Microsoft.Authorization/roleAssignments/write
// at the given subscription scope. Aborts if access is not allowed.
func azureCheckRBAC(runner cmdRunner, subscriptionScope string) error {
	url := fmt.Sprintf(
		"https://management.azure.com%s/providers/Microsoft.Authorization/checkAccess?api-version=2022-04-01",
		subscriptionScope,
	)
	body := `{"actions":[{"id":"Microsoft.Authorization/roleAssignments/write"}]}`
	logger.Debug("checking RBAC", "url", url)

	out, err := runner("az", []string{"rest", "--method", "POST", "--url", url, "--body", body}, nil)
	if err != nil {
		return fmt.Errorf("RBAC check failed: %w", err)
	}
	if !strings.Contains(out, `"accessDecision":"Allowed"`) {
		return fmt.Errorf("insufficient Azure RBAC permissions — your account needs Microsoft.Authorization/roleAssignments/write at subscription scope (e.g. the Owner or User Access Administrator role) to assign Monitoring Reader")
	}
	return nil
}
