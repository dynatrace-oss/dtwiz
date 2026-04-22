package cmd

import (
	"fmt"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func init() {
	statusCmd.Flags().BoolVar(&clientFlag, "extensions", false, "probe Classic and Platform Extensions APIs using the HTTP client")
}

var (
	statusOK    = color.New(color.FgGreen, color.Bold)
	statusError = color.New(color.FgRed, color.Bold)
	statusLabel = color.New()
	statusMuted = color.New()
	statusHead  = color.New(color.FgMagenta, color.Bold)
)

var clientFlag bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show connection status and system state",
	Long:  `Verifies connectivity to Dynatrace and displays the current system analysis.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		statusHead.Println("  Connection Status")
		statusMuted.Println("  " + "──────────────────────────────────────────")

		envURL := environmentHint()
		aTok := accessToken()
		pTok := platformToken()

		if envURL == "" {
			fmt.Printf("  %s  %s\n", statusLabel.Sprint("Environment:"), statusError.Sprint("✗ not set (use --environment or DT_ENVIRONMENT)"))
		} else {
			fmt.Printf("  %s  %s\n", statusLabel.Sprint("Environment:"), statusOK.Sprintf("✓ %s", envURL))
		}

		if aTok == "" {
			fmt.Printf("  %s  %s\n", statusLabel.Sprint("Access Token:"), statusError.Sprint("✗ not set (use --access-token or DT_ACCESS_TOKEN)"))
		} else if envURL != "" {
			if err := checkAccessToken(envURL, aTok); err != nil {
				fmt.Printf("  %s  %s\n", statusLabel.Sprint("Access Token:"), statusError.Sprintf("✗ %s", err))
			} else {
				fmt.Printf("  %s  %s\n", statusLabel.Sprint("Access Token:"), statusOK.Sprintf("✓ valid (%s)", installer.APIURL(envURL)))
			}
		} else {
			fmt.Printf("  %s  %s\n", statusLabel.Sprint("Access Token:"), statusOK.Sprint("✓ configured (skipped validation — no environment URL)"))
		}

		if pTok == "" {
			fmt.Printf("  %s  %s\n\n", statusLabel.Sprint("Platform Token:"), statusError.Sprint("✗ not set (use --platform-token or DT_PLATFORM_TOKEN)"))
		} else if envURL != "" {
			if err := checkPlatformToken(envURL, pTok); err != nil {
				fmt.Printf("  %s  %s\n\n", statusLabel.Sprint("Platform Token:"), statusError.Sprintf("✗ %s", err))
			} else {
				fmt.Printf("  %s  %s\n\n", statusLabel.Sprint("Platform Token:"), statusOK.Sprintf("✓ valid (%s)", installer.AppsURL(envURL)))
			}
		} else {
			fmt.Printf("  %s  %s\n\n", statusLabel.Sprint("Platform Token:"), statusOK.Sprint("✓ configured (skipped validation — no environment URL)"))
		}

		if clientFlag {
			printExtensionsStatus()
		}

		statusHead.Println("  System Analysis")
		statusMuted.Println("  " + "──────────────────────────────────────────")
		info, err := analyzer.AnalyzeSystem()
		if err != nil {
			fmt.Printf("  %s\n", statusError.Sprintf("✗ system analysis failed: %v", err))
			return nil
		}
		fmt.Println(info.Summary())
		return nil
	},
}

func printExtensionsStatus() {
	statusHead.Println("  Extensions API")
	statusMuted.Println("  " + "──────────────────────────────────────────")

	c, err := setupClient()
	if err != nil {
		fmt.Printf("  %s  %s\n\n", statusLabel.Sprint("Setup:"), statusError.Sprintf("✗ %v", err))
		return
	}

	// Classic: GET /api/v2/extensions
	var classicResp struct {
		TotalResults int `json:"totalResults"`
	}
	resp, err := c.Classic.HTTP().R().SetResult(&classicResp).Get("/api/v2/extensions")
	if err != nil {
		fmt.Printf("  %s  %s\n", statusLabel.Sprint("Classic Extensions:"), statusError.Sprintf("✗ %v", err))
	} else if resp.StatusCode() >= 400 {
		fmt.Printf("  %s  %s\n", statusLabel.Sprint("Classic Extensions:"), statusError.Sprintf("✗ HTTP %d", resp.StatusCode()))
	} else {
		fmt.Printf("  %s  %s\n", statusLabel.Sprint("Classic Extensions:"), statusOK.Sprintf("✓ reachable (%d extensions)", classicResp.TotalResults))
	}

	// Platform: GET /platform/extensions/v2/extensions
	var platformResp struct {
		TotalCount int `json:"totalCount"`
	}
	resp, err = c.Platform.HTTP().R().SetResult(&platformResp).Get("/platform/extensions/v2/extensions")
	if err != nil {
		fmt.Printf("  %s  %s\n\n", statusLabel.Sprint("Platform Extensions:"), statusError.Sprintf("✗ %v", err))
	} else if resp.StatusCode() >= 400 {
		fmt.Printf("  %s  %s\n\n", statusLabel.Sprint("Platform Extensions:"), statusError.Sprintf("✗ HTTP %d", resp.StatusCode()))
	} else {
		fmt.Printf("  %s  %s\n\n", statusLabel.Sprint("Platform Extensions:"), statusOK.Sprintf("✓ reachable (%d packages)", platformResp.TotalCount))
	}
}
