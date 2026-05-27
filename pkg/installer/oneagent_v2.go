package installer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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

	// Task 6 — BuildInstallCommand + ExecuteInstallCommand — not yet implemented.
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
		body, _ := io.ReadAll(io.LimitReader(resp.RawBody(), 2048))
		logger.Debug("installer download failed",
			"status", resp.StatusCode(),
			"auth_scheme", authScheme,
			"url", downloadURL,
			"response_body", strings.TrimSpace(string(body)),
		)
		if resp.StatusCode() == http.StatusForbidden || resp.StatusCode() == http.StatusUnauthorized {
			return "", fmt.Errorf( //nolint:staticcheck // ST1005: user-facing remediation hint with sentence structure
				"installer download failed (%d) — if using a platform token, ensure it has the InstallerDownload scope; "+
					"or pass a dt0c01.* access token with --access-token or DT_ACCESS_TOKEN.\n"+
					"Run with --debug for the API error detail.",
				resp.StatusCode(),
			)
		}
		return "", fmt.Errorf("installer download failed with status %d: %s", resp.StatusCode(), strings.TrimSpace(string(body)))
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

// dtRootCertURL is the published Dynatrace root CA used to verify the
// signature of downloaded installers. Overridable in tests.
var dtRootCertURL = "https://ca.dynatrace.com/dt-root.cert.pem"

// openSSLMissingError is the user-facing message returned when openssl is not
// found on a Linux host where signature verification is required.
const openSSLMissingError = "openssl is required to verify the installer signature. Install openssl or pass --no-verify-signature to skip."

// VerifyInstallerSignature verifies a Linux installer's CMS signature against
// the published Dynatrace root CA. It returns nil when skip is true or
// env.OS != "linux". Missing openssl on Linux is a hard error; verification
// failure aborts the install with the captured openssl stderr.
func VerifyInstallerSignature(env Environment, installerPath string, skip bool) error {
	if skip || env.OS != "linux" {
		return nil
	}

	opensslPath, err := exec.LookPath("openssl")
	logger.Debug("openssl lookup", "path", opensslPath, "found", err == nil)
	if err != nil {
		return errors.New(openSSLMissingError) //nolint:staticcheck // ST1005: exact wording is required by spec (user-facing remediation hint)
	}

	certPath, err := fetchDynatraceRootCA(dtRootCertURL)
	if err != nil {
		return err
	}
	defer os.Remove(certPath)

	code, stderr, err := runOpensslVerify(opensslPath, installerPath, certPath)
	if err != nil {
		return fmt.Errorf("running openssl: %w", err)
	}
	if code != 0 {
		logger.Debug("signature verification failed", "exit_code", code, "stderr", stderr)
		return fmt.Errorf("installer signature verification failed (openssl exit %d): %s", code, stderr)
	}

	logger.Verbose("installer signature verified")
	display.PrintStatusLine("signature", "Installer signature verified", display.ColorOK)
	return nil
}

// fetchDynatraceRootCA downloads the published Dynatrace root certificate to a
// temp file and returns its path. The caller owns the file.
func fetchDynatraceRootCA(caURL string) (string, error) {
	tmp, err := os.CreateTemp("", "dt-root-cert-*.pem")
	if err != nil {
		return "", fmt.Errorf("creating temp file for root CA: %w", err)
	}
	certPath := tmp.Name()
	_ = tmp.Close()

	logger.Debug("fetching dynatrace root ca", "url", caURL, "path", certPath)

	resp, err := resty.New().R().SetOutput(certPath).Get(caURL)
	if err != nil {
		_ = os.Remove(certPath)
		return "", fmt.Errorf("downloading Dynatrace root CA: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		_ = os.Remove(certPath)
		return "", fmt.Errorf("downloading Dynatrace root CA: status %d", resp.StatusCode())
	}
	return certPath, nil
}

// runOpensslVerify runs the documented Dynatrace CMS verification pipeline:
//
//	( echo "Content-Type: multipart/signed; ... boundary=--SIGNED-INSTALLER" ;
//	  echo ; echo "----SIGNED-INSTALLER" ; cat <installer> )
//	| openssl cms -verify -CAfile <certPath>
//
// It returns the openssl exit code and captured stderr. The error return is
// non-nil only when the subprocess could not be started.
func runOpensslVerify(opensslPath, installerPath, certPath string) (int, string, error) {
	installer, err := os.Open(installerPath)
	if err != nil {
		return 0, "", fmt.Errorf("opening installer: %w", err)
	}
	defer installer.Close()

	header := strings.NewReader(
		"Content-Type: multipart/signed; protocol=\"application/x-pkcs7-signature\"; " +
			"micalg=\"sha-256\"; boundary=\"--SIGNED-INSTALLER\"\n\n" +
			"----SIGNED-INSTALLER\n",
	)

	cmd := exec.Command(opensslPath, "cms", "-verify", "-CAfile", certPath)
	cmd.Stdin = io.MultiReader(header, installer)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = io.Discard

	err = cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), strings.TrimSpace(stderr.String()), nil
		}
		return 0, strings.TrimSpace(stderr.String()), err
	}
	return 0, "", nil
}
