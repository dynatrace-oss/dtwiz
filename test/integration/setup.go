package integration

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// TestEnv holds shared state for an integration test run.
type TestEnv struct {
	Client        *client.Client
	TestID        string
	TempDir       string
	EnvURL        string
	PlatformToken string // required; used for Platform/DQL API calls
	AccessToken   string // optional; used as Classic API token when set
	ClassicToken  string // effective Classic API token: AccessToken if set, else PlatformToken
}

// SetupIntegration validates required environment variables, constructs a
// *client.Client, generates a unique test ID, and returns a TestEnv.
// TEST_DT_PLATFORM_TOKEN is required. TEST_DT_ACCESS_TOKEN is optional; when
// set it is used as the Classic API token, otherwise PlatformToken is used.
// It calls t.Fatal if requirements are not met.
func SetupIntegration(t *testing.T) *TestEnv {
	t.Helper()

	envUrl := requireEnv(t, "TEST_DT_ENVIRONMENT")
	platformToken := requireEnv(t, "TEST_DT_PLATFORM_TOKEN")
	accessToken := os.Getenv("TEST_DT_ACCESS_TOKEN")

	classicToken := platformToken
	if accessToken != "" {
		classicToken = accessToken
	}

	classicURL := installer.APIURL(envUrl)
	platformURL := installer.AppsURL(envUrl)

	c, err := client.New(classicURL, platformURL, classicToken, platformToken, 0)
	if err != nil {
		t.Fatalf("SetupIntegration: failed to create client: %v", err)
	}

	testID := fmt.Sprintf("dtwiz-test-%d-%s", time.Now().Unix(), randomString(6))

	return &TestEnv{
		Client:        c,
		TestID:        testID,
		TempDir:       t.TempDir(),
		EnvURL:        envUrl,
		PlatformToken: platformToken,
		AccessToken:   accessToken,
		ClassicToken:  classicToken,
	}
}

// Parallelize marks the test as parallel unless TEST_SEQUENTIAL is set to a
// non-empty value. Use it at the top of each top-level integration test so
// that `make test-integration` runs them concurrently by default; pass
// SEQUENTIAL=true to the make invocation to opt out.
func Parallelize(t *testing.T) {
	t.Helper()
	if os.Getenv("TEST_SEQUENTIAL") == "" {
		t.Parallel()
	}
}

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	val := os.Getenv(key)
	if val == "" {
		t.Fatalf("SetupIntegration: required environment variable %s is not set", key)
	}
	return val
}

func randomString(length int) string {
	b := make([]byte, length/2+1)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
	}
	return hex.EncodeToString(b)[:length]
}
