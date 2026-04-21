package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
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

// NewHTTPClient builds a Client with a ClassicClient and a PlatformClient.
// Credentials and the environment URL are read from flags / env vars at call time.
func NewHTTPClient() (*Client, error) {
	envURL, aTok, pTok, err := getDtEnvironment()
	if err != nil {
		return nil, err
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
