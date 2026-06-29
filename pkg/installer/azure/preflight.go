package azure

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// azurePreflightChecks runs all pre-mutation checks and returns the subscription
// scope used for the integration:
//  1. az on PATH
//  2. az account show — must succeed (user is logged in)
//  3. RBAC checkAccess at subscription scope (advisory only — warns, never blocks)
//
// The integration always operates at subscription scope (/subscriptions/<id>).
func azurePreflightChecks(runner cmdRunner, envURL, platformToken string) (subscriptionID, tenantID string, err error) {
	if _, err = execLookPath("az"); err != nil {
		return "", "", fmt.Errorf("Azure CLI (az) not found — install it from https://aka.ms/installazurecliwindows") //nolint:staticcheck // ST1005: "Azure CLI" is a product name
	}

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

	azureCheckRBAC(runner, "/subscriptions/"+subscriptionID)

	return subscriptionID, tenantID, nil
}

// azureCheckRBAC makes a best-effort check that the current principal can create
// role assignments at the given subscription scope. It is advisory only: if the
// check cannot be completed (e.g. the principal lacks rights to read
// authorization data) or reports insufficient access, it prints a warning and
// returns. It never blocks — the role-assignment step surfaces the definitive
// error if permissions are actually missing.
func azureCheckRBAC(runner cmdRunner, subscriptionScope string) {
	userJSON, err := runner("az", []string{"ad", "signed-in-user", "show", "-o", "json"}, nil)
	if err != nil {
		logger.Debug("could not resolve signed-in user for RBAC check, skipping", "err", err)
		return
	}
	var user struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal([]byte(userJSON), &user); err != nil || user.ID == "" {
		logger.Debug("could not parse signed-in user response for RBAC check, skipping", "err", err)
		return
	}

	url := fmt.Sprintf(
		"https://management.azure.com%s/providers/Microsoft.Authorization/checkAccess?api-version=2018-09-01-preview",
		subscriptionScope,
	)
	body := fmt.Sprintf(
		`{"subject":{"objectId":%q},"actions":[{"id":"Microsoft.Authorization/roleAssignments/write"}]}`,
		user.ID,
	)
	logger.Debug("checking RBAC", "url", url, "objectId", user.ID)

	out, err := runner("az", []string{"rest", "--method", "POST", "--url", url, "--body", body}, nil)
	if err != nil {
		display.ColorWarning.Printf("  Warning: could not validate Azure permissions (%v); continuing\n", err)
		return
	}
	if !strings.Contains(out, `"accessDecision":"Allowed"`) {
		display.ColorWarning.Println("  Warning: your account may lack Microsoft.Authorization/roleAssignments/write at subscription scope — you may need Owner or User Access Administrator role; continuing")
		return
	}
	logger.Debug("RBAC check passed", "scope", subscriptionScope)
}
