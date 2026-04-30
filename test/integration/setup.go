package integration

import (
	"fmt"
	"math/rand"
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
	AccessToken   string
	PlatformToken string
}

// SetupIntegration validates required environment variables, constructs a
// *client.Client, generates a unique test ID, and returns a TestEnv.
// It calls t.Fatal if any required variable is missing.
func SetupIntegration(t *testing.T) *TestEnv {
	t.Helper()

	envUrl := requireEnv(t, "TEST_DT_ENVIRONMENT")
	accessToken := requireEnv(t, "TEST_DT_ACCESS_TOKEN")
	platformToken := requireEnv(t, "TEST_DT_PLATFORM_TOKEN")

	classicURL := installer.APIURL(envUrl)
	platformURL := installer.AppsURL(envUrl)

	c, err := client.New(classicURL, accessToken, platformURL, platformToken, 0)
	if err != nil {
		t.Fatalf("SetupIntegration: failed to create client: %v", err)
	}

	testID := fmt.Sprintf("dtwiz-test-%d-%s", time.Now().Unix(), randomString(6))

	return &TestEnv{
		Client:        c,
		TestID:        testID,
		TempDir:       t.TempDir(),
		EnvURL:        envUrl,
		AccessToken:   accessToken,
		PlatformToken: platformToken,
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

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"

	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))] //nolint:gosec
	}
	return string(b)
}
