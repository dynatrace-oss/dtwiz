package oneagent

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// dtRootCertURL is the published Dynatrace root CA used to verify the
// signature of downloaded installers. Overridable in tests.
var dtRootCertURL = "https://ca.dynatrace.com/dt-root.cert.pem"

// openSSLMissingError is the user-facing message returned when openssl is not
// found on a Linux host where signature verification is required.
const openSSLMissingError = "openssl is required to verify the installer signature. Install openssl or pass --no-verify-signature to skip."

// rootCAFetchTimeout caps the time allowed to download the Dynatrace root CA
// certificate. Kept short because the file is tiny (~2 KB); a stalled
// connection should not block the install indefinitely.
const rootCAFetchTimeout = 30 * time.Second

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

	resp, err := resty.New().SetTimeout(rootCAFetchTimeout).R().SetOutput(certPath).Get(caURL)
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
