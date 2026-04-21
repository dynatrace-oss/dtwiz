package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"github.com/fatih/color"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

var debugFlag bool
var verbosityFlag int
var environmentFlag string
var accessTokenFlag string
var platformTokenFlag string

var rootCmd = &cobra.Command{
	Use:   "dtwiz",
	Short: "Dynatrace Ingest CLI — analyze systems and deploy observability",
	Long: `dtwiz analyzes your system and deploys the best Dynatrace ingestion method.

Set your Dynatrace credentials via environment variables:

  export DT_ENVIRONMENT=https://<your-tenant-domain>
  export DT_ACCESS_TOKEN=dt0c01.****
  export DT_PLATFORM_TOKEN=dt0s16.****

Then use dtwiz commands to analyze and instrument your system.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.Init(debugFlag, verbosityFlag)
		logger.Verbose("logging: verbose")
		logger.Debug("logging: debug")
	},
}

func printBanner() {
	purple := color.New(color.FgMagenta, color.Bold)
	purple.Printf("  ____   _______   __        __ ___  ____\n")
	purple.Printf(" |  _ \\ |__   __| \\ \\      / /|_ _||_  /\n")
	purple.Printf(" | | | |   | |     \\ \\ /\\ / /  | |  / / \n")
	purple.Printf(" | |_| |   | |      \\ V  V /   | | / /_ \n")
	purple.Printf(" |____/    |_|       \\_/\\_/   |___|/____| %s\n", Version)
	fmt.Printf("\n HASTA LA VISTA - BLIND SPOTS!\n\n")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Show banner when no subcommand is given or --help is used on the root command.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd == rootCmd {
			printBanner()
		}
		defaultHelp(cmd, args)
	})
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	}

	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable debug logging")
	rootCmd.PersistentFlags().CountVarP(&verbosityFlag, "verbose", "v", "verbose output")
	rootCmd.PersistentFlags().StringVar(&environmentFlag, "environment", "", "Dynatrace environment URL (also read from DT_ENVIRONMENT)")
	rootCmd.PersistentFlags().StringVar(&accessTokenFlag, "access-token", "", "Dynatrace API access token (also read from DT_ACCESS_TOKEN)")
	rootCmd.PersistentFlags().StringVar(&platformTokenFlag, "platform-token", "", "Dynatrace platform token (dt0s16.*) for AWS installer (also read from DT_PLATFORM_TOKEN)")

	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(recommendCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
}

var sensitiveHTTPHeaders = map[string]bool{
	"authorization": true,
	"x-api-key":     true,
	"cookie":        true,
	"set-cookie":    true,
}

// newHTTPClient builds a Client with a ClassicClient and a PlatformClient.
// Credentials and the environment URL are read from flags / env vars at call time.
func newHTTPClient() (*Client, error) {
	envURL := environmentHint()
	if envURL == "" {
		return nil, fmt.Errorf("no Dynatrace environment URL configured\n\n" +
			"Set one with --environment or the DT_ENVIRONMENT env var:\n" +
			"  export DT_ENVIRONMENT=https://<your-env>.dynatracelabs.com/")
	}

	aTok := accessToken()
	if aTok == "" {
		return nil, fmt.Errorf("no Dynatrace access token configured\n\n" +
			"Set one with --access-token or the DT_ACCESS_TOKEN env var:\n" +
			"  export DT_ACCESS_TOKEN=dt0c01.****")
	}

	pTok := platformToken()
	if pTok == "" {
		return nil, fmt.Errorf("no Dynatrace platform token configured\n\n" +
			"Set one with --platform-token or the DT_PLATFORM_TOKEN env var:\n" +
			"  export DT_PLATFORM_TOKEN=dt0s16.****")
	}

	level := verbosityFlag
	if debugFlag {
		level = 2
	}

	classicURL := installer.APIURL(envURL)
	classic := &ClassicClient{
		baseURL: classicURL,
		http:    newRestyClient(classicURL, installer.AuthHeader(aTok), level),
	}

	appsURL := installer.AppsURL(envURL)
	platform := &PlatformClient{
		baseURL: appsURL,
		http:    newRestyClient(appsURL, "Bearer "+pTok, level),
	}

	return &Client{Classic: classic, Platform: platform}, nil
}

// newRestyClient creates a resty client with shared dtctl-equivalent settings.
func newRestyClient(baseURL, authHeader string, verbosityLevel int) *resty.Client {
	rc := resty.New().
		SetBaseURL(baseURL).
		SetHeader("Authorization", authHeader).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(10 * time.Second).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			sc := r.StatusCode()
			return sc == 429 || sc >= 500
		}).
		SetTimeout(6 * time.Minute).
		SetHeader("User-Agent", "dtwiz/"+Version).
		SetHeader("Accept-Encoding", "gzip")

	if verbosityLevel > 0 {
		rc.SetPreRequestHook(func(_ *resty.Client, req *http.Request) error {
			fmt.Fprintf(os.Stderr, "===> REQUEST <===\n%s %s\n", req.Method, req.URL)
			if verbosityLevel >= 2 {
				fmt.Fprintln(os.Stderr, "HEADERS:")
				for k, v := range req.Header {
					if sensitiveHTTPHeaders[strings.ToLower(k)] {
						fmt.Fprintf(os.Stderr, "    %s: [REDACTED]\n", k)
					} else {
						fmt.Fprintf(os.Stderr, "    %s: %s\n", k, strings.Join(v, ", "))
					}
				}
			}
			return nil
		})
		rc.OnAfterResponse(func(_ *resty.Client, resp *resty.Response) error {
			fmt.Fprintf(os.Stderr, "===> RESPONSE <===\nSTATUS: %d %s\nTIME: %s\n",
				resp.StatusCode(), resp.Status(), resp.Time())
			if verbosityLevel >= 2 {
				fmt.Fprintf(os.Stderr, "BODY:\n%s\n", resp.String())
			}
			return nil
		})
	}
	return rc
}
