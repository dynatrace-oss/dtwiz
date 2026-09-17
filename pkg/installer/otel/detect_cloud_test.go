package otel

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectIMDS(t *testing.T) {
	tests := []struct {
		name       string
		provider   string // which IMDS endpoint the server simulates
		wantMethod string
		statusCode int
		want       string
	}{
		{name: "aws", provider: "aws", wantMethod: http.MethodPut, statusCode: 200, want: "aws"},
		{name: "azure", provider: "azure", wantMethod: http.MethodGet, statusCode: 200, want: "azure"},
		{name: "gcp", provider: "gcp", wantMethod: http.MethodGet, statusCode: 200, want: "gcp"},
		{name: "no imds", provider: "aws", wantMethod: http.MethodPut, statusCode: 404, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.wantMethod {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				w.WriteHeader(tc.statusCode)
			}))
			defer srv.Close()

			awsURL, azureURL, gcpURL := "http://localhost:0", "http://localhost:0", "http://localhost:0"
			switch tc.provider {
			case "aws":
				awsURL = srv.URL
			case "azure":
				azureURL = srv.URL
			case "gcp":
				gcpURL = srv.URL
			}

			got := detectIMDS(&http.Client{}, awsURL, azureURL, gcpURL)
			if got != tc.want {
				t.Errorf("detectIMDS() = %q, want %q", got, tc.want)
			}
		})
	}
}
