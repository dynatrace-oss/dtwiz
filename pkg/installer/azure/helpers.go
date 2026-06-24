package azure

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// azureIssuerURL derives the federated identity issuer URL from the Dynatrace environment URL.
//
// The issuer mirrors the apps URL structure:
//   - *.apps.dynatrace.com            → https://token.dynatrace.com
//   - *.dev.apps.dynatracelabs.com    → https://dev.token.dynatracelabs.com
//   - *.sprint.apps.dynatracelabs.com → https://sprint.token.dynatracelabs.com
func azureIssuerURL(envURL string) string {
	appsURL := installer.AppsURL(envURL)
	host := strings.TrimPrefix(appsURL, "https://")
	host = strings.TrimRight(host, "/")

	appsIdx := strings.Index(host, ".apps.")
	if appsIdx < 0 {
		return "https://token.dynatrace.com"
	}
	domain := host[appsIdx+6:] // e.g. "dynatrace.com" or "dynatracelabs.com"
	beforeApps := host[:appsIdx] // e.g. "rrx28105" or "rrx28105.dev"

	parts := strings.Split(beforeApps, ".")
	if len(parts) >= 2 {
		qualifier := parts[len(parts)-1]
		return "https://" + qualifier + ".token." + domain
	}
	return "https://token." + domain
}

// azureBuildFedCredJSON builds the JSON body for the federated credential creation.
func azureBuildFedCredJSON(connID, envURL string) (string, error) {
	appsURL := installer.AppsURL(envURL)
	audience := strings.TrimPrefix(appsURL, "https://") + "/svc-id/com.dynatrace.da"
	issuer := azureIssuerURL(envURL)
	logger.Debug("federated credential", "connID", connID, "audience", audience, "issuer", issuer)

	payload := map[string]interface{}{
		"name":      fedCredName,
		"issuer":    issuer,
		"subject":   "dt:connection-id/" + connID,
		"audiences": []string{audience},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("building federated credential JSON: %w", err)
	}
	return string(b), nil
}

// azureDeleteFedCred deletes the dtwiz-managed federated credential from an App Registration.
// A "not found" error is treated as success — the goal is already achieved.
func azureDeleteFedCred(runner cmdRunner, clientID string) error {
	_, err := runner("az", []string{"ad", "app", "federated-credential", "delete",
		"--id", clientID, "--federated-credential-id", fedCredName}, nil)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "not found") {
		return nil
	}
	return err
}

// azureGetSPObjectID retrieves the Service Principal object ID for a given
// application client ID. It retries up to 5 times with a 3-second sleep
// between attempts to handle Entra eventual consistency after SP creation.
func azureGetSPObjectID(runner cmdRunner, clientID string, sleeper func(time.Duration)) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			sleeper(3 * time.Second)
		}
		out, err := runner("az", []string{"ad", "sp", "show", "--id", clientID, "-o", "json"}, nil)
		if err != nil {
			msg := strings.ToLower(err.Error() + out)
			if strings.Contains(msg, "403") || strings.Contains(msg, "forbidden") {
				return "", fmt.Errorf("az ad sp show: %w", err)
			}
			if strings.Contains(msg, "not found") || strings.Contains(msg, "does not exist") ||
				strings.Contains(msg, "resource was not found") {
				lastErr = err
				logger.Debug("SP not yet propagated, retrying", "attempt", attempt+1, "clientID", clientID)
				continue
			}
			return "", fmt.Errorf("az ad sp show: %w", err)
		}
		var sp struct {
			ID string `json:"id"`
		}
		if err = json.Unmarshal([]byte(out), &sp); err != nil {
			return "", fmt.Errorf("parsing az ad sp show output: %w", err)
		}
		if sp.ID == "" {
			lastErr = fmt.Errorf("empty object ID in az ad sp show response")
			continue
		}
		return sp.ID, nil
	}
	if lastErr != nil {
		return "", fmt.Errorf("az ad sp show (exhausted retries): %w", lastErr)
	}
	return "", fmt.Errorf("az ad sp show: failed to get object ID after 5 attempts")
}
