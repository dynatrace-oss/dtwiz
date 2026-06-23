package azure

import (
	"strings"
	"testing"
)

func TestAzureMaskToken(t *testing.T) {
	const secret = "dt0s16.verysecrettoken.abc"
	preview := captureStdout(t, func() {
		azurePrintPreview(azureConfig{
			ConnectionName:    "dtwiz-azure",
			ConfigurationName: "dtwiz-azure",
			EnvURL:            "https://abc.live.dynatrace.com",
			PlatformToken:     secret,
			TenantID:          "tenant-id-123",
			ManagementGroupID: "/providers/Microsoft.Management/managementGroups/tenant-id-123",
		})
	})
	if strings.Contains(preview, secret) {
		t.Errorf("platform token must not appear in preview output; got:\n%s", preview)
	}
	if !strings.Contains(preview, "***") {
		t.Errorf("expected *** placeholder in preview output; got:\n%s", preview)
	}
}
