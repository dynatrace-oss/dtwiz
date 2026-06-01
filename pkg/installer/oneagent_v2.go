package installer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-resty/resty/v2"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

type InstallOptions struct {
	DryRun                bool
	MonitoringMode        string
	NoVerifySignature     bool
	SkipConnectivityCheck bool
	ConnectivityCheckOnly bool
	PrintEndpoints        bool
	Quiet                 bool
}

type AgentConfig struct {
	MonitoringMode string
}

// Environment identifies the target OS and CPU architecture used to select the
// correct OneAgent installer. OS values: "linux", "windows". Arch values:
// "x86", "arm".
type Environment struct {
	OS   string
	Arch string
}

func DefaultAgentConfig() AgentConfig {
	return AgentConfig{MonitoringMode: string(InstallModeFullStack)}
}

func ResolveAgentConfig(opts InstallOptions) AgentConfig {
	cfg := DefaultAgentConfig()
	if opts.MonitoringMode != "" {
		cfg.MonitoringMode = opts.MonitoringMode
	}
	logger.Debug("resolved agent config",
		"monitoring-mode", cfg.MonitoringMode,
		"override_set", cfg.MonitoringMode != string(InstallModeFullStack),
	)
	return cfg
}

func InstallOneAgentV2(c *client.Client, opts InstallOptions) error {
	display.PrintStatusLine("oneagent", fmt.Sprintf("PoC flow (monitoring-mode=%s)", opts.MonitoringMode), display.ColorWarning)

	env, err := detectRuntimeEnvironment()
	if err != nil {
		return err
	}
	logger.Debug("detected environment", "os", env.OS, "arch", env.Arch)

	cfg := ResolveAgentConfig(opts)
	logger.Debug("install options",
		"dry_run", opts.DryRun,
		"no_verify_signature", opts.NoVerifySignature,
		"monitoring_mode", cfg.MonitoringMode,
	)

	installerPath, err := DownloadInstaller(c.Classic, env)
	if err != nil {
		return err
	}
	defer os.Remove(installerPath)

	if err := VerifyInstallerSignature(env, installerPath, opts.NoVerifySignature); err != nil {
		return err
	}

	display.PrintStatusLine("oneagent", "download and verification complete; install execution not yet implemented (Task 6)", display.ColorWarning)
	logger.Debug("install execution not yet implemented", "installer_path", installerPath)
	return nil
}

// detectRuntimeEnvironment returns an Environment based on the current host
// OS and architecture. Used as a stand-in until DetectEnvironment (Task 2) is
// implemented.
func detectRuntimeEnvironment() (Environment, error) {
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x86"
	case "arm64":
		arch = "arm"
	default:
		return Environment{}, fmt.Errorf("unsupported architecture for OneAgent: %s", runtime.GOARCH)
	}

	switch runtime.GOOS {
	case "linux":
		return Environment{OS: "linux", Arch: arch}, nil
	case "windows":
		return Environment{OS: "windows", Arch: arch}, nil
	case "darwin":
		return Environment{}, fmt.Errorf("OneAgent direct install is not supported on macOS; use Docker or Linux")
	default:
		return Environment{}, fmt.Errorf("unsupported OS for OneAgent: %s", runtime.GOOS)
	}
}

// readErrorBody reads up to 2 KB from a resty response body. Used on non-200
// responses where SetDoNotParseResponse(true) is active. Go's net/http
// transport handles transparent gzip decompression at the transport level, so
// no manual decompression is needed here.
func readErrorBody(resp *resty.Response) string {
	body, _ := io.ReadAll(io.LimitReader(resp.RawBody(), 2048))
	return strings.TrimSpace(string(body))
}

// installerOSSegment maps Environment.OS to the URL path segment used by the
// Dynatrace installer download API. Linux maps to "unix"; Windows maps to
// "windows".
func installerOSSegment(os string) (string, error) {
	switch os {
	case "linux":
		return "unix", nil
	case "windows":
		return "windows", nil
	default:
		return "", fmt.Errorf("unsupported installer OS: %q", os)
	}
}

// DownloadInstaller streams the OneAgent installer to a temporary file using
// the credentials already embedded in the ClassicClient (set upstream by
// validateCredentials). On Unix the file permissions are tightened to 0o700.
// The caller owns the returned path and is responsible for removing it.
func DownloadInstaller(c *client.ClassicClient, env Environment) (string, error) {
	osSeg, err := installerOSSegment(env.OS)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("/api/v1/deployment/installer/agent/%s/default/latest?arch=%s", osSeg, env.Arch)
	downloadURL := strings.TrimRight(c.BaseURL(), "/") + path

	// Log auth scheme (Bearer vs Api-Token) without exposing the token value.
	authScheme := strings.SplitN(c.HTTP().Header.Get("Authorization"), " ", 2)[0]
	logger.Debug("downloading installer",
		"url", downloadURL,
		"os", env.OS,
		"arch", env.Arch,
		"auth_scheme", authScheme,
	)

	resp, err := c.HTTP().R().SetDoNotParseResponse(true).Get(path)
	if err != nil {
		return "", fmt.Errorf("downloading OneAgent installer: %w", err)
	}
	defer resp.RawBody().Close()

	if resp.StatusCode() != http.StatusOK {
		body := readErrorBody(resp)
		logger.Debug("installer download failed",
			"status", resp.StatusCode(),
			"auth_scheme", authScheme,
			"url", downloadURL,
			"response_body", body,
		)
		if resp.StatusCode() == http.StatusForbidden || resp.StatusCode() == http.StatusUnauthorized {
			return "", fmt.Errorf( //nolint:staticcheck // ST1005: user-facing remediation hint with sentence structure
				"installer download failed (%d) — if using a platform token, ensure it has the InstallerDownload scope; "+
					"or pass a dt0c01.* access token with --access-token or DT_ACCESS_TOKEN.\n"+
					"Run with --debug for the API error detail.",
				resp.StatusCode(),
			)
		}
		return "", fmt.Errorf("installer download failed with status %d: %s", resp.StatusCode(), body)
	}

	tmpFile, err := os.CreateTemp("", "dynatrace-oneagent-*"+installerExtension(env.OS))
	if err != nil {
		return "", fmt.Errorf("creating temp file for installer: %w", err)
	}
	defer tmpFile.Close()

	n, err := io.Copy(tmpFile, resp.RawBody())
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("writing installer to disk: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpFile.Name(), 0o700); err != nil {
			_ = os.Remove(tmpFile.Name())
			return "", fmt.Errorf("setting installer permissions: %w", err)
		}
	}

	logger.Verbose("installer downloaded", "path", tmpFile.Name(), "size_bytes", n)
	display.PrintStatusLine("installer",
		fmt.Sprintf("%s (%s)", filepath.Base(tmpFile.Name()), humanBytes(n)),
		display.ColorOK)
	return tmpFile.Name(), nil
}

func installerExtension(os string) string {
	if os == "windows" {
		return ".exe"
	}
	return ".sh"
}

func humanBytes(n int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%dGB", n/gb)
	case n >= mb:
		return fmt.Sprintf("%dMB", n/mb)
	case n >= kb:
		return fmt.Sprintf("%dKB", n/kb)
	default:
		return fmt.Sprintf("%dB", n)
	}
}
