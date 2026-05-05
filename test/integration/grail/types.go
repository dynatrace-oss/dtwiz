package grail

import "time"

// TraceRecord is a single record returned by a DQL trace query.
type TraceRecord map[string]interface{}

// PollOption configures the polling behavior of WaitForTraces.
type PollOption func(*pollConfig)

type pollConfig struct {
	timeout  time.Duration
	interval time.Duration
}

type grailResponse struct {
	RequestToken string `json:"requestToken"`
	State        string `json:"state"`
	TTLSeconds   int    `json:"ttlSeconds"`
	Result       struct {
		Records []TraceRecord `json:"records"`
	} `json:"result"`
}

const (
	grailExecutePath  = "/platform/storage/query/v1/query:execute"
	grailPollPath     = "/platform/storage/query/v1/query:poll"
	dqlPollMaxRetries = 10
	dqlPollInterval   = 1 * time.Second
)

// WithTimeout overrides the default 60s polling timeout.
func WithTimeout(d time.Duration) PollOption {
	return func(c *pollConfig) { c.timeout = d }
}

// WithInterval overrides the default 2s poll interval.
func WithInterval(d time.Duration) PollOption {
	return func(c *pollConfig) { c.interval = d }
}
