package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
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

// getDtEnvironment returns the environment URL, access token, and platform
// token. All three must be configured or an error is returned.
func getDtEnvironment() (environmentURL, accessTok, platformTok string, err error) {
	envURL := environmentHint()
	if envURL == "" {
		return "", "", "", fmt.Errorf(
			"no Dynatrace environment URL configured\n\n" +
				"Set one with --environment or the DT_ENVIRONMENT env var:\n" +
				"  export DT_ENVIRONMENT=https://<your-env>.dynatracelabs.com/",
		)
	}

	aTok := accessToken()
	if aTok == "" {
		return "", "", "", fmt.Errorf(
			"no Dynatrace access token configured\n\n" +
				"Set one with --access-token or the DT_ACCESS_TOKEN env var:\n" +
				"  export DT_ACCESS_TOKEN=dt0c01.****",
		)
	}

	pTok := platformToken()
	if pTok == "" {
		return "", "", "", fmt.Errorf(
			"no Dynatrace platform token configured\n\n" +
				"Set one with --platform-token or the DT_PLATFORM_TOKEN env var:\n" +
				"  export DT_PLATFORM_TOKEN=dt0s16.****",
		)
	}

	return envURL, aTok, pTok, nil
}

var credentialHTTPClient = &http.Client{Timeout: 5 * time.Second}

// validateCredentials checks that the Dynatrace environment is reachable and
// both the access token and platform token are valid. Both checks run in
// parallel. Returns a combined error listing all failures.
func validateCredentials(envURL, accessTok, platformTok string) error {
	var mu sync.Mutex
	var errs []string

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := checkAccessToken(envURL, accessTok); err != nil {
			mu.Lock()
			errs = append(errs, err.Error())
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		if err := checkPlatformToken(envURL, platformTok); err != nil {
			mu.Lock()
			errs = append(errs, err.Error())
			mu.Unlock()
		}
	}()

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

// checkAccessToken validates the access token via POST /api/v2/apiTokens/lookup.
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
