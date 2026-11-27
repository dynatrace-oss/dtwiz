package dotnet_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

func TestLatestRelease_ParsesTagAndBuildsAssetURLs(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v1.20.0"}`))
	}))
	defer srv.Close()
	src := dotnet.Source{APIURL: srv.URL + "/api", DownloadBase: srv.URL + "/download"}

	// Act
	rel, err := src.LatestRelease(context.Background(), srv.Client())

	// Assert
	if err != nil {
		t.Fatalf("LatestRelease() error = %v", err)
	}
	if rel.Tag != "v1.20.0" || rel.Version() != "1.20.0" {
		t.Errorf("Tag/Version = %q/%q, want v1.20.0/1.20.0", rel.Tag, rel.Version())
	}
	if got, want := rel.AssetURL("a.zip"), srv.URL+"/download/v1.20.0/a.zip"; got != want {
		t.Errorf("AssetURL = %q, want %q", got, want)
	}
	if got, want := rel.ChecksumsURL(), srv.URL+"/download/v1.20.0/checksums.txt"; got != want {
		t.Errorf("ChecksumsURL = %q, want %q", got, want)
	}
}

func TestLatestRelease_NotFound(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	src := dotnet.Source{APIURL: srv.URL, DownloadBase: srv.URL}

	// Act
	_, err := src.LatestRelease(context.Background(), srv.Client())

	// Assert
	if err == nil || errors.Is(err, dotnet.ErrRateLimited) {
		t.Errorf("error = %v, want a non-rate-limit error", err)
	}
}

func TestLatestRelease_RateLimitedWithoutFallback(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	src := dotnet.Source{APIURL: srv.URL, DownloadBase: srv.URL}

	// Act
	_, err := src.LatestRelease(context.Background(), srv.Client())

	// Assert
	if !errors.Is(err, dotnet.ErrRateLimited) {
		t.Errorf("error = %v, want ErrRateLimited", err)
	}
}

func TestLatestRelease_RateLimitedFallsBackToRedirect(t *testing.T) {
	// Arrange
	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTooManyRequests) })
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/v1.21.0", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	src := dotnet.Source{APIURL: srv.URL + "/api", LatestURL: srv.URL + "/latest", DownloadBase: srv.URL}

	// Act
	rel, err := src.LatestRelease(context.Background(), srv.Client())

	// Assert
	if err != nil {
		t.Fatalf("LatestRelease() error = %v", err)
	}
	if rel.Tag != "v1.21.0" {
		t.Errorf("Tag = %q, want v1.21.0", rel.Tag)
	}
}

func TestLatestRelease_NetworkFailure(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	src := dotnet.Source{APIURL: url, DownloadBase: url}

	// Act
	_, err := src.LatestRelease(context.Background(), http.DefaultClient)

	// Assert
	if err == nil {
		t.Error("expected an error when the server is unreachable")
	}
}
