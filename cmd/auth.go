package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtctl/sdk/httpclient"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/azure"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/gcp"
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
// flag. Access-token auth is opt-in and activates only when the flag is
// passed explicitly on the command line.
func accessToken() string {
	return accessTokenFlag
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
// platformTok is required. accessTok may be empty when not configured.
func getDtEnvironment() (envURL, accessTok, platformTok string, err error) {
	envURL = environmentHint()
	platformTok = platformToken()

	var missing []string
	if envURL == "" {
		missing = append(missing, "DT_ENVIRONMENT")
	}
	if platformTok == "" {
		missing = append(missing, "DT_PLATFORM_TOKEN")
	}

	if envURL == "" {
		return "", "", "", fmt.Errorf(
			"no Dynatrace environment URL configured\n\n"+
				"Set one with --environment or the DT_ENVIRONMENT env var:\n"+
				"  export DT_ENVIRONMENT=https://<your-env>.dynatracelabs.com/: %w",
			&installer.ConfigError{MissingFields: missing},
		)
	}

	if platformTok == "" {
		return "", "", "", fmt.Errorf(
			"no Dynatrace platform token configured\n\n"+
				"Set one with --platform-token or the DT_PLATFORM_TOKEN env var:\n"+
				"  export DT_PLATFORM_TOKEN=dt0s16.****: %w",
			&installer.ConfigError{MissingFields: missing},
		)
	}

	accessTok = accessToken()

	return envURL, accessTok, platformTok, nil
}

// credentialClientOpts configures the one-shot credential probes: a short timeout so an
// unreachable environment fails fast, and a brief retry to ride out a transient 429/5xx.
// Tests override it to disable retries.
var credentialClientOpts = []httpclient.Option{
	httpclient.WithTimeout(5 * time.Second),
	httpclient.WithRetry(2, 500*time.Millisecond, 2*time.Second),
}

// newCredentialClient returns an SDK client for a credential probe against baseURL.
// The auth scheme (Bearer vs Api-Token) is derived from the token prefix.
func newCredentialClient(baseURL, token string) (*httpclient.Client, error) {
	opts := append([]httpclient.Option{httpclient.WithToken(token)}, credentialClientOpts...)
	return httpclient.New(baseURL, opts...)
}

// checkPlatformTokenClassicAccess probes the Classic API to determine whether token can
// authenticate. Returns nil if any non-401/403 response is received.
func checkPlatformTokenClassicAccess(envURL, token string) error {
	classicURL := strings.TrimRight(installer.APIURL(envURL), "/")
	c, err := newCredentialClient(classicURL, token)
	if err != nil {
		return err
	}
	resp, err := c.HTTP().R().Get("/api/v2/settings/schemas")
	if err != nil {
		return fmt.Errorf("classic API not reachable (%s)", classicURL)
	}
	apiErr := httpclient.CheckResponse(resp)
	if errors.Is(apiErr, httpclient.ErrUnauthorized) || errors.Is(apiErr, httpclient.ErrForbidden) {
		return fmt.Errorf("authentication failed")
	}
	return nil
}

// validateCredentials validates the platform token via DQL (required) and
// determines the Classic API token. When an explicit access token is provided
// (old customers), it takes precedence for Classic API calls. Otherwise the
// platform token is used for both Platform and Classic APIs.
// Returns the classicTok to use for Classic API calls.
func validateCredentials(envURL, accessTok, platformTok string) (classicTok string, err error) {
	if err := checkPlatformToken(envURL, platformTok); err != nil {
		return "", err
	}
	// When an explicit access token is set (different from platform token),
	// it takes precedence for Classic API calls.
	if accessTok != "" && accessTok != platformTok {
		logger.Debug("classic API auth: using explicit access token")
		return accessTok, nil
	}
	if err := checkPlatformTokenClassicAccess(envURL, platformTok); err == nil {
		logger.Debug("classic API auth: platform token accepted")
		return platformTok, nil
	}
	logger.Debug("classic API auth: platform token rejected by Classic API, proceeding anyway")
	return platformTok, nil
}

// checkAccessToken validates an access token via POST /api/v2/apiTokens/lookup.
func checkAccessToken(envURL, token string) error {
	classicURL := strings.TrimRight(installer.APIURL(envURL), "/")
	lookupURL := classicURL + "/api/v2/apiTokens/lookup"

	c, err := newCredentialClient(classicURL, token)
	if err != nil {
		return fmt.Errorf("✗ Access token: %v", err)
	}
	resp, err := c.HTTP().R().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]string{"token": token}).
		Post("/api/v2/apiTokens/lookup")
	if err != nil {
		return fmt.Errorf("✗ Access token: environment not reachable (%s): %w",
			classicURL, &installer.NetworkError{Reason: "environment_not_reachable", URL: classicURL})
	}

	apiErr := httpclient.CheckResponse(resp)
	switch {
	case apiErr == nil:
		return nil
	case errors.Is(apiErr, httpclient.ErrUnauthorized):
		return fmt.Errorf("✗ Access token: authentication failed: %w",
			&installer.AuthError{Reason: "authentication_failed"})
	case errors.Is(apiErr, httpclient.ErrForbidden):
		return fmt.Errorf("✗ Access token: insufficient permissions")
	default:
		return fmt.Errorf("✗ Access token: unexpected response %d from %s", resp.StatusCode(), lookupURL)
	}
}

// analyzeSystem runs AnalyzeSystem and enriches the result with cloud connection
// status. All cmd callers should use this instead of calling AnalyzeSystem directly.
func analyzeSystem() (*analyzer.SystemInfo, error) {
	info, err := analyzer.AnalyzeSystem()
	if err != nil {
		return nil, err
	}
	if envURL, _, platformTok, credErr := getDtEnvironment(); credErr == nil {
		var checks []func() error
		if info.Azure != nil && info.Azure.Available {
			checks = append(checks, func() error {
				exists, err := azure.ConnectionExists(envURL, platformTok)
				info.AzureConfigured = exists
				return err
			})
		}
		if info.GCP != nil && info.GCP.Available {
			checks = append(checks, func() error {
				exists, err := gcp.ConnectionExists(envURL, platformTok)
				info.GCPConfigured = exists
				return err
			})
		}
		if len(checks) > 0 {
			if err := installer.RunConcurrently(checks...); err != nil {
				logger.Debug("connection existence check failed, assuming not configured", "err", err)
			}
		}
	}
	return info, nil
}

// checkPlatformToken validates the platform token via a minimal DQL query.
func checkPlatformToken(envURL, token string) error {
	appsURL := strings.TrimRight(installer.AppsURL(envURL), "/")
	queryURL := appsURL + "/platform/storage/query/v1/query:execute"

	c, err := newCredentialClient(appsURL, token)
	if err != nil {
		return fmt.Errorf("✗ Platform token: %v", err)
	}
	resp, err := c.HTTP().R().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]interface{}{
			"query":                      "fetch dt.system.events | limit 1",
			"requestTimeoutMilliseconds": 4000,
			"maxResultRecords":           1,
		}).
		Post("/platform/storage/query/v1/query:execute")
	if err != nil {
		return fmt.Errorf("✗ Platform token: environment not reachable (%s): %w",
			appsURL, &installer.NetworkError{Reason: "environment_not_reachable", URL: appsURL})
	}

	apiErr := httpclient.CheckResponse(resp)
	switch {
	case apiErr == nil:
		return nil
	case errors.Is(apiErr, httpclient.ErrUnauthorized):
		return fmt.Errorf("✗ Platform token: authentication failed: %w",
			&installer.AuthError{Reason: "authentication_failed"})
	case errors.Is(apiErr, httpclient.ErrForbidden):
		return fmt.Errorf("✗ Platform token: insufficient permissions: %w",
			&installer.AuthError{Reason: "invalid_token"})
	default:
		return fmt.Errorf("✗ Platform token: unexpected response %d from %s: %w",
			resp.StatusCode(), queryURL, &installer.NetworkError{Reason: "environment_not_reachable", URL: appsURL})
	}
}
