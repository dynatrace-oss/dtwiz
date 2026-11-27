package dotnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Upstream release locations.
const (
	releaseRepo         = "open-telemetry/opentelemetry-dotnet-instrumentation"
	defaultAPIURL       = "https://api.github.com/repos/" + releaseRepo + "/releases/latest"
	defaultLatestURL    = "https://github.com/" + releaseRepo + "/releases/latest"
	defaultDownloadBase = "https://github.com/" + releaseRepo + "/releases/download"
	checksumsAsset      = "checksums.txt"
)

// ErrRateLimited marks a release lookup refused because of API rate limiting.
var ErrRateLimited = errors.New("GitHub API rate limit exceeded")

// Source says where releases are looked up and downloaded from. The zero value
// is not usable; use DefaultSource. Tests point it at a fake server.
type Source struct {
	// APIURL answers with the latest release as GitHub's JSON ("tag_name").
	APIURL string
	// LatestURL redirects to the latest release's tag page; it is the fallback
	// when the API is rate limited.
	LatestURL string
	// DownloadBase is the prefix of release asset URLs: <base>/<tag>/<asset>.
	DownloadBase string
}

// DefaultSource returns the upstream GitHub release locations.
func DefaultSource() Source {
	return Source{APIURL: defaultAPIURL, LatestURL: defaultLatestURL, DownloadBase: defaultDownloadBase}
}

// Release is one upstream release.
type Release struct {
	// Tag is the release tag, for example "v1.20.0".
	Tag          string
	downloadBase string
}

// Version is the tag without the leading "v", as recorded in the archive's
// VERSION file.
func (r Release) Version() string { return strings.TrimPrefix(r.Tag, "v") }

// AssetURL returns the download URL of a release asset.
func (r Release) AssetURL(name string) string {
	return strings.TrimRight(r.downloadBase, "/") + "/" + r.Tag + "/" + name
}

// ChecksumsURL returns the download URL of the release's checksum file.
func (r Release) ChecksumsURL() string { return r.AssetURL(checksumsAsset) }

// LatestRelease resolves the latest upstream release. When the API is rate
// limited it falls back to the redirect of the "latest" page.
func (s Source) LatestRelease(ctx context.Context, client *http.Client) (Release, error) {
	tag, err := s.tagFromAPI(ctx, client)
	if errors.Is(err, ErrRateLimited) && s.LatestURL != "" {
		if fallbackTag, fbErr := s.tagFromRedirect(ctx, client); fbErr == nil {
			tag, err = fallbackTag, nil
		}
	}
	if err != nil {
		return Release{}, err
	}
	return Release{Tag: tag, downloadBase: s.DownloadBase}, nil
}

func (s Source) tagFromAPI(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.APIURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("querying the latest release: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
		return "", ErrRateLimited
	case resp.StatusCode != http.StatusOK:
		return "", fmt.Errorf("querying the latest release: HTTP %d", resp.StatusCode)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", fmt.Errorf("reading the latest release: %w", err)
	}
	if body.TagName == "" {
		return "", errors.New("the latest release has no tag")
	}
	return body.TagName, nil
}

func (s Source) tagFromRedirect(ctx context.Context, client *http.Client) (string, error) {
	noRedirect := *client
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.LatestURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := noRedirect.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	i := strings.LastIndex(loc, "/tag/")
	if resp.StatusCode < 300 || resp.StatusCode > 399 || i < 0 {
		return "", fmt.Errorf("unexpected response %d from %s", resp.StatusCode, s.LatestURL)
	}
	return loc[i+len("/tag/"):], nil
}
