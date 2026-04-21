package cmd

import (
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
