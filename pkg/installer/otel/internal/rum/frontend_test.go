package rum

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/config"
)

// newTestPlatformClient returns a PlatformClient whose base URL points at srv.
func newTestPlatformClient(t *testing.T, srv *httptest.Server) *client.PlatformClient {
	t.Helper()
	c, err := client.New(srv.URL, srv.URL, "test-token", "test-token", 0)
	if err != nil {
		t.Fatalf("new test client: %v", err)
	}
	return c.Platform
}

// --- FrontendName tests ---

func TestFrontendName_StartsWithDtwiz(t *testing.T) {
	name, err := FrontendName(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(name, "dtwiz-") {
		t.Errorf("expected name to start with 'dtwiz-', got %q", name)
	}
}

func TestFrontendName_SamePathSameName(t *testing.T) {
	dir := t.TempDir()
	name1, err := FrontendName(dir)
	if err != nil {
		t.Fatal(err)
	}
	name2, err := FrontendName(dir)
	if err != nil {
		t.Fatal(err)
	}
	if name1 != name2 {
		t.Errorf("expected same name for same path, got %q and %q", name1, name2)
	}
}

func TestFrontendName_DifferentPathsDifferentNames(t *testing.T) {
	name1, err := FrontendName(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	name2, err := FrontendName(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if name1 == name2 {
		t.Errorf("expected different names for different paths, both got %q", name1)
	}
}

func TestFrontendName_OnlyAllowedChars(t *testing.T) {
	name, err := FrontendName(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range name {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz0123456789-", ch) {
			t.Errorf("unexpected character %q in name %q", ch, name)
		}
	}
}

func TestFrontendName_MaxLength(t *testing.T) {
	name, err := FrontendName(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(name) > 255 {
		t.Errorf("name length %d exceeds 255", len(name))
	}
}

// --- EnsureFrontendApplication tests ---

func TestEnsureFrontendApplication_ReusesExistingID(t *testing.T) {
	dir := t.TempDir()
	envURL := "https://abc.live.dynatrace.com"

	if err := config.Save(dir, &config.ProjectConfig{
		Frontends: map[string]config.FrontendConfig{
			envURL: {ID: "EXISTING-ID", FrontendName: "dtwiz-foo-abc12345"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API should not be called when ID is already stored")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	id, err := EnsureFrontendApplication(context.Background(), newTestPlatformClient(t, srv), envURL, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "EXISTING-ID" {
		t.Errorf("expected EXISTING-ID, got %q", id)
	}
}

func TestEnsureFrontendApplication_CreatesAndSavesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	envURL := "https://abc.live.dynatrace.com"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != frontendsPath {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(frontendResponse{
			ID:           "NEW-FRONTEND-ID",
			FrontendName: "dtwiz-myapp-aabbccdd",
			DisplayName:  "myapp [dtwiz]",
			Type:         "WEB_AGENTLESS",
		})
	}))
	defer srv.Close()

	id, err := EnsureFrontendApplication(context.Background(), newTestPlatformClient(t, srv), envURL, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "NEW-FRONTEND-ID" {
		t.Errorf("expected NEW-FRONTEND-ID, got %q", id)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Frontends[envURL].ID != "NEW-FRONTEND-ID" {
		t.Errorf("expected saved ID NEW-FRONTEND-ID, got %q", cfg.Frontends[envURL].ID)
	}
}

func TestEnsureFrontendApplication_APIErrorLeavesConfigUnchanged(t *testing.T) {
	dir := t.TempDir()
	envURL := "https://abc.live.dynatrace.com"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer srv.Close()

	_, err := EnsureFrontendApplication(context.Background(), newTestPlatformClient(t, srv), envURL, dir)
	if err == nil {
		t.Fatal("expected error from API failure")
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fe := cfg.Frontends[envURL]; fe.ID != "" {
		t.Errorf("expected config unchanged, got ID %q", fe.ID)
	}
}
