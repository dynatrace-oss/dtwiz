package oneagent

import (
	"compress/gzip"
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

// readErrorBody reads up to 2 KB from a resty response body. Used on non-200
// responses where SetDoNotParseResponse(true) is active. Because the shared
// resty client sets Accept-Encoding: gzip explicitly, Go's transport does not
// decompress automatically — we must do it here when the server signals gzip.
func readErrorBody(resp *resty.Response) string {
	var r io.Reader = resp.RawBody()
	if resp.Header().Get("Content-Encoding") == "gzip" {
		if gz, err := gzip.NewReader(r); err == nil {
			defer gz.Close()
			r = gz
		}
	}
	body, _ := io.ReadAll(io.LimitReader(r, 2048))
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
			return "", fmt.Errorf(
				"installer download failed (%d) — if using a platform token, ensure it has the InstallerDownload scope; "+
					"or pass a dt0c01.* access token with --access-token or DT_ACCESS_TOKEN.\n"+
					"Run with --debug for the API error detail",
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

	totalBytes := resp.RawResponse.ContentLength // -1 when unknown
	src := display.NewProgressReader(resp.RawBody(), totalBytes)

	n, err := io.Copy(tmpFile, src)
	src.Clear()
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
		fmt.Sprintf("%s (%s)", filepath.Base(tmpFile.Name()), display.HumanBytes(n)),
		display.ColorOK)
	return tmpFile.Name(), nil
}

func installerExtension(os string) string {
	if os == "windows" {
		return ".exe"
	}
	return ".sh"
}
