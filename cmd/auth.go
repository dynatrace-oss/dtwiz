package cmd

import (
	"fmt"
	"os"
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

// getDtEnvironment returns the environment URL and token for installers that
// need raw credentials.
//
// Resolution order for the environment URL:
//  1. --environment flag
//  2. DT_ENVIRONMENT env var
//
// Token resolution:
//  1. --access-token flag
//  2. DT_ACCESS_TOKEN env var
func getDtEnvironment() (environmentURL, token string, err error) {
	envURL := environmentHint()
	if envURL == "" {
		return "", "", fmt.Errorf(
			"no Dynatrace environment URL configured\n\n" +
				"Set one with --environment or the DT_ENVIRONMENT env var:\n" +
				"  export DT_ENVIRONMENT=https://<your-env>.dynatracelabs.com/",
		)
	}

	tok := accessToken()
	if tok == "" {
		return "", "", fmt.Errorf(
			"no Dynatrace access token configured\n\n" +
				"Set one with --access-token or the DT_ACCESS_TOKEN env var:\n" +
				"  export DT_ACCESS_TOKEN=dt0c01.****",
		)
	}

	return envURL, tok, nil
}
