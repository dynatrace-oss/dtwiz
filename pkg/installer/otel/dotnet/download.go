package dotnet

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

// maxTextDownload bounds small text downloads such as the checksum file.
const maxTextDownload = 1 << 20

// Download streams url into destPath. On any failure, including a response that
// ends early or a cancelled context, the partial file is removed.
func Download(ctx context.Context, client *http.Client, url, destPath string) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: HTTP %d", url, resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(destPath)
		}
	}()
	if _, err = io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	return nil
}

// fetchText downloads a small text file.
func fetchText(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: HTTP %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxTextDownload))
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", url, err)
	}
	return string(b), nil
}
