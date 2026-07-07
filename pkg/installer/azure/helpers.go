package azure

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// azureIssuerURL maps the env URL to the federated identity issuer:
// *.apps.dynatrace.com → token.dynatrace.com; *.dev.apps.dynatracelabs.com → dev.token.dynatracelabs.com; etc.
func azureIssuerURL(envURL string) (string, error) {
	appsURL := installer.AppsURL(envURL)
	host := strings.TrimPrefix(appsURL, "https://")
	host = strings.TrimPrefix(host, "http://")
	if slash := strings.Index(host, "/"); slash >= 0 {
		host = host[:slash]
	}
	host = strings.TrimRight(host, "/")

	appsIdx := strings.Index(host, ".apps.")
	if appsIdx < 0 {
		return "", fmt.Errorf("unsupported Dynatrace environment URL for Azure federated credential issuer: %s", envURL)
	}
	domain := host[appsIdx+6:]   // e.g. "dynatrace.com" or "dynatracelabs.com"
	beforeApps := host[:appsIdx] // e.g. "rrx28105" or "rrx28105.dev"

	parts := strings.Split(beforeApps, ".")
	if len(parts) >= 2 {
		qualifier := parts[len(parts)-1]
		return "https://" + qualifier + ".token." + domain, nil
	}
	return "https://token." + domain, nil
}

func azureBuildFedCredJSON(connID, envURL string) (string, error) {
	appsURL := installer.AppsURL(envURL)
	audience := strings.TrimPrefix(appsURL, "https://") + "/svc-id/com.dynatrace.da"
	issuer, err := azureIssuerURL(envURL)
	if err != nil {
		return "", err
	}
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

func azureDeleteFedCred(runner cmdRunner, clientID string) error {
	_, err := runner("az", []string{"ad", "app", "federated-credential", "delete",
		"--id", clientID, "--federated-credential-id", fedCredName}, nil)
	if err != nil && installer.IsNotFoundErr(err) {
		return nil
	}
	return err
}

// azureListAppIDsByName finds leftover App Registrations from interrupted installs.
// `az ad sp create-for-rbac --name X` reuses an existing app with that name, so a leftover
// keeps the same appId across runs and triggers the DT "Constraints violated" error on reinstall.
func azureListAppIDsByName(runner cmdRunner, name string) ([]string, error) {
	out, err := runner("az", []string{"ad", "app", "list", "--display-name", name, "-o", "json"}, nil)
	if err != nil {
		return nil, fmt.Errorf("az ad app list: %w", err)
	}
	var apps []struct {
		AppID string `json:"appId"`
	}
	if err := json.Unmarshal([]byte(out), &apps); err != nil {
		return nil, fmt.Errorf("parsing az ad app list output: %w", err)
	}
	ids := make([]string, 0, len(apps))
	for _, a := range apps {
		if a.AppID != "" {
			ids = append(ids, a.AppID)
		}
	}
	logger.Debug("listed app registrations by display name", "name", name, "count", len(ids))
	return ids, nil
}

// azureAppHasDtwizFedCred checks ownership before deleting an app found only by display name.
// Entra display names are not unique, so a name match alone is not enough.
func azureAppHasDtwizFedCred(runner cmdRunner, clientID, issuer string) (bool, error) {
	out, err := runner("az", []string{"ad", "app", "federated-credential", "list", "--id", clientID, "-o", "json"}, nil)
	if err != nil {
		return false, fmt.Errorf("az ad app federated-credential list: %w", err)
	}
	var creds []struct {
		Name   string `json:"name"`
		Issuer string `json:"issuer"`
	}
	if err := json.Unmarshal([]byte(out), &creds); err != nil {
		return false, fmt.Errorf("parsing federated-credential list: %w", err)
	}
	for _, c := range creds {
		if c.Name == fedCredName && c.Issuer == issuer {
			logger.Debug("federated credential ownership verified", "clientID", clientID, "credName", c.Name)
			return true, nil
		}
	}
	logger.Debug("federated credential not found on app", "clientID", clientID, "wantName", fedCredName, "wantIssuer", issuer, "credCount", len(creds))
	return false, nil
}

func azureDeleteRoleAssignment(runner cmdRunner, clientID string) error {
	_, err := runner("az", []string{"role", "assignment", "delete",
		"--assignee", clientID,
		"--role", "Monitoring Reader"}, nil)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "no matched assignments") || installer.IsNotFoundErr(err) {
			return nil
		}
		return err
	}
	return nil
}

// azureDeleteApp deletes an App Registration; Azure cascades this to its Service Principal and federated credentials.
func azureDeleteApp(runner cmdRunner, clientID string) error {
	_, err := runner("az", []string{"ad", "app", "delete", "--id", clientID}, nil)
	if err != nil && installer.IsNotFoundErr(err) {
		return nil
	}
	return err
}

// azureSPObjectIDRetryable reports whether err signals that a just-created SP is not yet
// visible in Entra (worth retrying), as opposed to a permanent failure (403, bad JSON, ...).
func azureSPObjectIDRetryable(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "resource was not found") || strings.Contains(msg, "empty object id")
}

// azureGetSPObjectID retries up to 5 times (3s apart) because a newly created SP is not immediately visible in Entra.
func azureGetSPObjectID(runner cmdRunner, clientID string, sleeper func(time.Duration)) (string, error) {
	var id string
	err := installer.Retry(sleeper, installer.RetryConfig{
		MaxAttempts: 5,
		Delay:       func(int) time.Duration { return installer.Jitter(3 * time.Second) },
		Retryable:   azureSPObjectIDRetryable,
		OnRetry: func(attempt int, _ time.Duration, _ error) {
			logger.Debug("SP not yet propagated, retrying", "attempt", attempt, "clientID", clientID)
		},
	}, func() error {
		out, cmdErr := runner("az", []string{"ad", "sp", "show", "--id", clientID, "-o", "json"}, nil)
		if cmdErr != nil {
			// Fold stdout into the error text up front so every downstream classification
			// (the 403 check below, and azureSPObjectIDRetryable via Retry's Retryable
			// callback and the final check after retries are exhausted) sees the same
			// combined signal — some az CLI error shapes put the useful detail in stdout
			// rather than in the Go/stderr-derived error.
			err := cmdErr
			if out != "" {
				err = fmt.Errorf("%w: %s", cmdErr, out)
			}
			if msg := strings.ToLower(err.Error()); strings.Contains(msg, "403") || strings.Contains(msg, "forbidden") {
				return fmt.Errorf("az ad sp show: %w", err)
			}
			if azureSPObjectIDRetryable(err) {
				return err
			}
			return fmt.Errorf("az ad sp show: %w", err)
		}
		var sp struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(out), &sp); err != nil {
			return fmt.Errorf("parsing az ad sp show output: %w", err)
		}
		if sp.ID == "" {
			return fmt.Errorf("empty object ID in az ad sp show response")
		}
		id = sp.ID
		return nil
	})
	if err == nil {
		return id, nil
	}
	if azureSPObjectIDRetryable(err) {
		return "", fmt.Errorf("az ad sp show (exhausted retries): %w", err)
	}
	return "", err
}
