package rum

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

const testFrontendID = "FRONTEND-0123456789ABCDEF"

func newTestRumClient(t *testing.T, serverURL string) *sdkRumClient {
	t.Helper()
	c, err := newSDKRumClient(serverURL, "dt0s16.test")
	if err != nil {
		t.Fatalf("create RUM client: %v", err)
	}
	c.C.HTTP().SetRetryCount(0)
	return c
}

func isTagNotFound(err error) bool {
	var ife *installer.InstallFailedError
	return errors.As(err, &ife) && ife.Step == tagFetchStep
}

func TestFetchJavaScriptTag(t *testing.T) {
	const tag = `<script src="https://example.invalid/rum.js"></script>`

	tests := []struct {
		name         string
		status       int
		body         string
		wantTag      string
		wantErr      bool
		wantNotFound bool
	}{
		{name: "success trims whitespace", status: http.StatusOK, body: "\n  " + tag + "\n\n", wantTag: tag},
		{name: "unknown entity", status: http.StatusNotFound, wantErr: true, wantNotFound: true},
		{name: "server error", status: http.StatusInternalServerError, body: "boom", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(srv.Close)

			got, err := newTestRumClient(t, srv.URL).fetchJavaScriptTag(context.Background(), testFrontendID)

			wantPath := "/platform/rum/v1/manualInsertionTags/javaScriptTag/" + testFrontendID
			if gotPath != wantPath {
				t.Errorf("path = %q, want %q", gotPath, wantPath)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if isTagNotFound(err) != tt.wantNotFound {
				t.Errorf("not-found classification = %v, want %v (err: %v)", isTagNotFound(err), tt.wantNotFound, err)
			}
			if got != tt.wantTag {
				t.Errorf("tag = %q, want %q", got, tt.wantTag)
			}
		})
	}
}

func TestFetchJavaScriptTagTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	c := newTestRumClient(t, srv.URL)
	srv.Close()

	_, err := c.fetchJavaScriptTag(context.Background(), testFrontendID)
	if err == nil {
		t.Fatal("expected transport error, got nil")
	}
	if isTagNotFound(err) {
		t.Errorf("transport error misclassified as not-found: %v", err)
	}
}
