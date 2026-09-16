package otel

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestMain stubs all IMDS URLs before any test runs. Without this, tests that
// call generateOtelConfig() trigger live IMDS probes — on Azure-hosted CI
// runners the Azure endpoint responds, injecting an unexpected CloudProvider
// into config snapshot tests.
func TestMain(m *testing.M) {
	awsIMDSURL = "http://localhost:0/"
	azureIMDSURL = "http://localhost:0/"
	gcpIMDSURL = "http://localhost:0/"
	os.Exit(m.Run())
}

func TestDetectIMDSCloudProvider(t *testing.T) {
	tests := []struct {
		name       string
		urlVar     *string
		wantMethod string
		statusCode int
		want       string
	}{
		{
			name:       "aws",
			urlVar:     &awsIMDSURL,
			wantMethod: http.MethodPut,
			statusCode: 200,
			want:       "aws",
		},
		{
			name:       "azure",
			urlVar:     &azureIMDSURL,
			wantMethod: http.MethodGet,
			statusCode: 200,
			want:       "azure",
		},
		{
			name:       "gcp",
			urlVar:     &gcpIMDSURL,
			wantMethod: http.MethodGet,
			statusCode: 200,
			want:       "gcp",
		},
		{
			name:       "no imds",
			urlVar:     &awsIMDSURL,
			wantMethod: http.MethodPut,
			statusCode: 404,
			want:       "",
		},
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

			orig := *tc.urlVar
			*tc.urlVar = srv.URL + "/"
			defer func() { *tc.urlVar = orig }()

			got := detectIMDSCloudProvider()
			if got != tc.want {
				t.Errorf("detectIMDSCloudProvider() = %q, want %q", got, tc.want)
			}
		})
	}
}
