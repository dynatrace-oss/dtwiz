package otel

import (
	"net/http"
	"sync"
	"time"
)

const imdsTimeout = 150 * time.Millisecond

const (
	// AWS: PUT to the IMDSv2 token endpoint — returns 200 only on real EC2.
	// Azure's 169.254.169.254 does not expose this path, so no false positives.
	awsIMDSURL   = "http://169.254.169.254/latest/api/token"
	azureIMDSURL = "http://169.254.169.254/metadata/instance?api-version=2021-02-01"
	gcpIMDSURL   = "http://metadata.google.internal/computeMetadata/v1/"
)

func detectIMDSCloudProvider() string {
	return detectIMDS(&http.Client{Timeout: imdsTimeout}, awsIMDSURL, azureIMDSURL, gcpIMDSURL)
}

// detectIMDS probes the three cloud IMDS endpoints in parallel and returns
// "aws", "azure", or "gcp" if the machine is a cloud VM, or "" if no endpoint
// responds within imdsTimeout. URLs are parameters so tests can substitute
// httptest server addresses without mutating package-level state.
func detectIMDS(client *http.Client, awsURL, azureURL, gcpURL string) string {
	type probe struct {
		provider string
		method   string
		url      string
		header   http.Header
	}

	probes := []probe{
		{
			provider: "aws",
			method:   http.MethodPut,
			url:      awsURL,
			header:   http.Header{"X-aws-ec2-metadata-token-ttl-seconds": {"21600"}},
		},
		{
			provider: "azure",
			method:   http.MethodGet,
			url:      azureURL,
			header:   http.Header{"Metadata": {"true"}},
		},
		{
			provider: "gcp",
			method:   http.MethodGet,
			url:      gcpURL,
			header:   http.Header{"Metadata-Flavor": {"Google"}},
		},
	}

	resultCh := make(chan string, len(probes))
	var once sync.Once
	var wg sync.WaitGroup

	for _, p := range probes {
		wg.Add(1)
		go func(p probe) {
			defer wg.Done()
			req, err := http.NewRequest(p.method, p.url, nil)
			if err != nil {
				return
			}
			for k, vals := range p.header {
				req.Header.Set(k, vals[0])
			}
			resp, err := client.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
			if err != nil {
				return
			}
			if resp.StatusCode == http.StatusOK {
				once.Do(func() { resultCh <- p.provider })
			}
		}(p)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	return <-resultCh
}
