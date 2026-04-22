package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

type CredentialToken struct {
	value         string
	cliName       string
	envName       string
	tokenVerifyFn func(envURL, token string) error
	getUrlFn      func(envURL string) string
}

var clientFlag bool

func init() {
	statusCmd.Flags().BoolVar(&clientFlag, "extensions", false, "probe Classic and Platform Extensions APIs using the HTTP client")
}

const (
	statusLabel        = "status"
	envLabel           = "Environment"
	classicExtensions  = "Classic Extensions"
	platformExtensions = "Platform Extensions"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show connection status and system state",
	Long:  `Verifies connectivity to Dynatrace and displays the current system analysis.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		display.Header("Connection Status")

		envURL := environmentHint()
		accessTok := accessToken()
		platformTok := platformToken()

		if envURL == "" {
			display.PrintStatusLine(envLabel, "✗ not set (use --environment or DT_ENVIRONMENT)", display.ColorError)
		} else {
			display.PrintStatusLine(envLabel, fmt.Sprintf("✓ %s", envURL), display.ColorOK)
		}

		printCredentialStatus("Access Token", envURL, CredentialToken{
			value:         accessTok,
			cliName:       "access-token",
			envName:       "DT_ACCESS_TOKEN",
			tokenVerifyFn: checkAccessToken,
			getUrlFn:      installer.APIURL,
		})

		printCredentialStatus("Platform Token", envURL, CredentialToken{
			value:         platformTok,
			cliName:       "platform-token",
			envName:       "DT_PLATFORM_TOKEN",
			tokenVerifyFn: checkPlatformToken,
			getUrlFn:      installer.AppsURL,
		})

		if clientFlag {
			printExtensionsStatus()
		}

		display.Header("System Analysis")
		info, err := analyzer.AnalyzeSystem()
		if err != nil {
			fmt.Printf("  %s\n", display.ColorError.Sprintf("✗ system analysis failed: %v", err))
			return err
		}
		fmt.Println(info.Summary())

		printFeatureFlags()

		return nil
	},
}

func printExtensionsStatus() {
	display.Header("Extensions API")

	c, err := setupClient()
	if err != nil {
		display.PrintError(statusLabel, fmt.Errorf("setup: %s", err))
		return
	}

	// Classic: GET /api/v2/extensions
	var classicResp struct {
		TotalResults int `json:"totalResults"`
	}
	resp, err := c.Classic.HTTP().R().SetResult(&classicResp).Get("/api/v2/extensions")

	if err != nil {
		display.PrintError(classicExtensions, err)
	} else if resp.StatusCode() >= 400 {
		display.PrintError(classicExtensions, fmt.Errorf("HTTP %d", resp.StatusCode()))
	} else {
		display.PrintStatusLine(classicExtensions, fmt.Sprintf("✓ reachable (%d extensions)", classicResp.TotalResults), display.ColorOK)
	}

	// Platform: GET /platform/extensions/v2/extensions
	var platformResp struct {
		TotalCount int `json:"totalCount"`
	}
	resp, err = c.Platform.HTTP().R().SetResult(&platformResp).Get("/platform/extensions/v2/extensions")
	if err != nil {
		display.PrintError(platformExtensions, err)
	} else if resp.StatusCode() >= 400 {
		display.PrintError(platformExtensions, fmt.Errorf("HTTP %d", resp.StatusCode()))
	} else {
		display.PrintStatusLine(platformExtensions, fmt.Sprintf("✓ reachable (%d packages)", platformResp.TotalCount), display.ColorOK)
	}
	fmt.Println()
}

func printCredentialStatus(label, envURL string, token CredentialToken) {
	if token.value == "" {
		display.PrintStatusLine(label, fmt.Sprintf("✗ not set (use --%s or %s)", token.cliName, token.envName), display.ColorError)
		return
	}
	if envURL != "" {
		if err := token.tokenVerifyFn(envURL, token.value); err != nil {
			display.PrintStatusLine(label, fmt.Sprintf("✗ %s", err), display.ColorError)
		} else {
			display.PrintStatusLine(label, fmt.Sprintf("✓ valid (%s)", token.getUrlFn(envURL)), display.ColorOK)
		}
	} else {
		display.PrintStatusLine(label, "✓ configured (skipped validation — no environment URL)", display.ColorOK)
	}
}

func printFeatureFlags() {
	var enabledFlags []featureflags.FlagState
	for _, f := range featureflags.List() {
		if f.Enabled {
			enabledFlags = append(enabledFlags, f)
		}
	}
	if len(enabledFlags) > 0 {
		fmt.Println()
		display.Header("Feature Flags")
		for _, f := range enabledFlags {
			display.PrintFlagLine(f.EnvVar, fmt.Sprintf("✓ enabled (%s)", f.Source), display.ColorOK)
		}
	}
}
