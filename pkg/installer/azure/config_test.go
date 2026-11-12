package azure

import (
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

func TestAzureMaskToken(t *testing.T) {
	const secret = "dt0s16.verysecrettoken.abc"
	// Commands are only listed with -v, which is where masking matters.
	logger.Init(false, 1)
	t.Cleanup(func() { logger.Init(false, 0) })
	preview := captureStdout(t, func() {
		azurePrintPreview(azureConfig{
			ConnectionName:    "dtwiz-azure",
			ConfigurationName: "dtwiz-azure",
			EnvURL:            "https://abc.live.dynatrace.com",
			PlatformToken:     secret,
			TenantID:          "tenant-id-123",
			SubscriptionID:    "sub-abc123",
			Scope:             "/subscriptions/sub-abc123",
		}, false)
	})
	if strings.Contains(preview, secret) {
		t.Errorf("platform token must not appear in preview output; got:\n%s", preview)
	}
	if !strings.Contains(preview, "***") {
		t.Errorf("expected *** placeholder in preview output; got:\n%s", preview)
	}
}

func TestAzurePreviewHidesCommandsByDefault(t *testing.T) {
	logger.Init(false, 0)
	preview := captureStdout(t, func() {
		azurePrintPreview(azureConfig{
			ConnectionName:    "dtwiz-azure",
			ConfigurationName: "dtwiz-azure",
			EnvURL:            "https://abc.live.dynatrace.com",
			PlatformToken:     "dt0s16.token.abc",
			TenantID:          "tenant-id-123",
			SubscriptionID:    "sub-abc123",
			Scope:             "/subscriptions/sub-abc123",
		}, false)
	})
	if strings.Contains(preview, "az ad") || strings.Contains(preview, "Commands to be executed") {
		t.Errorf("commands should only be listed with -v; got:\n%s", preview)
	}
}
