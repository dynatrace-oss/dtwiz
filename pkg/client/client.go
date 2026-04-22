package client

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/version"
	"github.com/go-resty/resty/v2"
)

// Client is the top-level HTTP client for Dynatrace API calls.
// It exposes a ClassicClient (Classic API, DT_ACCESS_TOKEN) and a
// PlatformClient (Platform/Apps API, DT_PLATFORM_TOKEN).
type Client struct {
	Classic  *ClassicClient
	Platform *PlatformClient
}

// ClassicClient calls the Classic Dynatrace API (no .apps. in URL)
// authenticated with DT_ACCESS_TOKEN.
type ClassicClient struct {
	http    *resty.Client
	baseURL string
}

// HTTP returns the underlying resty client for direct request building.
func (c *ClassicClient) HTTP() *resty.Client { return c.http }

// BaseURL returns the Classic API base URL.
func (c *ClassicClient) BaseURL() string { return c.baseURL }

// PlatformClient calls the Dynatrace Platform API (.apps. URL)
// authenticated with DT_PLATFORM_TOKEN.
type PlatformClient struct {
	http    *resty.Client
	baseURL string
}

// HTTP returns the underlying resty client for direct request building.
func (c *PlatformClient) HTTP() *resty.Client { return c.http }

// BaseURL returns the Platform API base URL.
func (c *PlatformClient) BaseURL() string { return c.baseURL }

var sensitiveHTTPHeaders = map[string]bool{
	"authorization": true,
	"x-api-key":     true,
	"cookie":        true,
	"set-cookie":    true,
}

// New builds a Client with a ClassicClient and a PlatformClient.
// classicURL and platformURL should already be in the correct URL families
// (use installer.APIURL / installer.AppsURL to convert from the raw env URL).
func New(classicURL, accessToken, platformURL, platformToken string, verbosityLevel int) (*Client, error) {
	if classicURL == "" {
		return nil, fmt.Errorf("classic API URL is required")
	}
	if platformURL == "" {
		return nil, fmt.Errorf("platform URL is required")
	}

	classic := &ClassicClient{
		baseURL: classicURL,
		http:    newRestyClient(classicURL, installer.AuthHeader(accessToken), verbosityLevel),
	}

	platform := &PlatformClient{
		baseURL: platformURL,
		http:    newRestyClient(platformURL, "Bearer "+platformToken, verbosityLevel),
	}

	return &Client{Classic: classic, Platform: platform}, nil
}

// newRestyClient creates a resty client with shared settings.
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
		SetHeader("User-Agent", "dtwiz/"+version.Version).
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
			fmt.Fprintf(os.Stderr, "===> RESPONSE <===\nSTATUS: %s\nTIME: %s\n",
				resp.Status(), resp.Time())
			if verbosityLevel >= 2 {
				const maxBodyBytes = 2048
				body := resp.String()
				if len(body) > maxBodyBytes {
					fmt.Fprintf(os.Stderr, "BODY (first %d of %d bytes):\n%s\n[... truncated]\n", maxBodyBytes, len(body), body[:maxBodyBytes])
				} else {
					fmt.Fprintf(os.Stderr, "BODY:\n%s\n", body)
				}
			}
			return nil
		})
	}
	return rc
}
