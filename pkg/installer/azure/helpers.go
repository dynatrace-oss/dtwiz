package azure

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// azureBuildFedCredJSON builds the JSON body for the federated credential creation.
func azureBuildFedCredJSON(connID, envURL string) (string, error) {
	appsURL := installer.AppsURL(envURL)
	audience := strings.TrimPrefix(appsURL, "https://") + "/svc-id/com.dynatrace.da"
	logger.Debug("federated credential", "connID", connID, "audience", audience)

	payload := map[string]interface{}{
		"name":      fedCredName,
		"issuer":    "https://token.dynatrace.com",
		"subject":   "dt:connection-id/" + connID,
		"audiences": []string{audience},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("building federated credential JSON: %w", err)
	}
	return string(b), nil
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
