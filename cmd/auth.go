package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// environmentHint returns the Dynatrace environment URL from the --environment
// flag or the DT_ENVIRONMENT env var (flag takes precedence).
func environmentHint() string {
	if environmentFlag != "" {
		return environmentFlag
	}
	return os.Getenv("DT_ENVIRONMENT")
}

// accessToken returns the Dynatrace API access token from the --access-token
// flag or the DT_ACCESS_TOKEN env var (flag takes precedence).
// Returns an empty string when neither is set.
func accessToken() string {
	if accessTokenFlag != "" {
		return accessTokenFlag
	}
	return os.Getenv("DT_ACCESS_TOKEN")
}

// platformToken returns a Dynatrace platform token (dt0s16.*) from the
// --platform-token flag or the DT_PLATFORM_TOKEN env var (flag takes precedence).
// Returns an empty string when neither is set.
func platformToken() string {
	if platformTokenFlag != "" {
		return platformTokenFlag
	}
	return os.Getenv("DT_PLATFORM_TOKEN")
}

// getDtEnvironment resolves the environment URL and raw tokens from flags/env vars.
// platformTok is required. accessTok is optional (empty string when not set).
func getDtEnvironment() (envURL, accessTok, platformTok string, err error) {
	envURL = environmentHint()
	if envURL == "" {
		return "", "", "", fmt.Errorf(
			"no Dynatrace environment URL configured\n\n" +
				"Set one with --environment or the DT_ENVIRONMENT env var:\n" +
				"  export DT_ENVIRONMENT=https://<your-env>.dynatracelabs.com/",
		)
	}

	platformTok = platformToken()
	if platformTok == "" {
		return "", "", "", fmt.Errorf(
			"no Dynatrace platform token configured\n\n" +
				"Set one with --platform-token or the DT_PLATFORM_TOKEN env var:\n" +
				"  export DT_PLATFORM_TOKEN=dt0s16.****",
		)
	}

	return envURL, accessToken(), platformTok, nil
}

var credentialHTTPClient = &http.Client{Timeout: 5 * time.Second}

// checkClassicAccess probes the Classic API to determine whether token can
// authenticate. Returns nil if any non-401/403 response is received.
func checkClassicAccess(envURL, token string) error {
	classicURL := strings.TrimRight(installer.APIURL(envURL), "/")
	req, err := http.NewRequest(http.MethodGet, classicURL+"/api/v2/settings/schemas", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", installer.AuthHeader(token))
	resp, err := credentialHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("Classic API not reachable (%s)", classicURL)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed")
	}
	return nil
}

// validateCredentials validates the platform token via DQL (required) and
// determines the Classic API token. Tries the platform token against the Classic
// API first; falls back to the access token if authentication fails there.
// Returns the classicTok to use for Classic API calls.
func validateCredentials(envURL, accessTok, platformTok string) (classicTok string, err error) {
	if err := checkPlatformToken(envURL, platformTok); err != nil {
		return "", err
	}
	if err := checkClassicAccess(envURL, platformTok); err == nil {
		logger.Debug("classic API auth: platform token accepted")
		return platformTok, nil
	}
	logger.Debug("classic API auth: platform token rejected, trying access token fallback")
	if accessTok != "" {
		logger.Debug("classic API auth: using access token as fallback")
		return accessTok, nil
	}
	logger.Debug("classic API auth: no access token configured, proceeding with platform token")
	return platformTok, nil
}

// checkAccessToken validates an access token via POST /api/v2/apiTokens/lookup.
func checkAccessToken(envURL, token string) error {
	classicURL := strings.TrimRight(installer.APIURL(envURL), "/")
	lookupURL := classicURL + "/api/v2/apiTokens/lookup"

	payload, _ := json.Marshal(map[string]string{"token": token})
	req, err := http.NewRequest(http.MethodPost, lookupURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("✗ Access token: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", installer.AuthHeader(token))

	resp, err := credentialHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("✗ Access token: environment not reachable (%s)", classicURL)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("✗ Access token: authentication failed")
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("✗ Access token: unexpected response %d from %s", resp.StatusCode, lookupURL)
	}
	return nil
}

// checkPlatformToken validates the platform token via a minimal DQL query.
func checkPlatformToken(envURL, token string) error {
	appsURL := strings.TrimRight(installer.AppsURL(envURL), "/")
	queryURL := appsURL + "/platform/storage/query/v1/query:execute"

	payload, _ := json.Marshal(map[string]interface{}{
		"query":                      "fetch dt.system.events | limit 1",
		"requestTimeoutMilliseconds": 4000,
		"maxResultRecords":           1,
	})
	req, err := http.NewRequest(http.MethodPost, queryURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("✗ Platform token: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := credentialHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("✗ Platform token: environment not reachable (%s)", appsURL)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("✗ Platform token: authentication failed")
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("✗ Platform token: unexpected response %d from %s", resp.StatusCode, queryURL)
	}
	return nil
}
