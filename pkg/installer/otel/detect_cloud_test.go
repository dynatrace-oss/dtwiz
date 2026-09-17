package otel

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectIMDS(t *testing.T) {
	tests := []struct {
		name       string
		wantMethod string
		statusCode int
		argIdx     int // 0=aws, 1=azure, 2=gcp
		want       string
	}{
		{name: "aws", wantMethod: http.MethodPut, statusCode: 200, argIdx: 0, want: "aws"},
		{name: "azure", wantMethod: http.MethodGet, statusCode: 200, argIdx: 1, want: "azure"},
		{name: "gcp", wantMethod: http.MethodGet, statusCode: 200, argIdx: 2, want: "gcp"},
		{name: "no imds", wantMethod: http.MethodPut, statusCode: 404, argIdx: 0, want: ""},
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

			urls := [3]string{"http://localhost:0", "http://localhost:0", "http://localhost:0"}
			urls[tc.argIdx] = srv.URL

			got := detectIMDS(&http.Client{}, urls[0], urls[1], urls[2])
			if got != tc.want {
				t.Errorf("detectIMDS() = %q, want %q", got, tc.want)
			}
		})
	}
}
