package rum

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/dynatrace-oss/dtctl/sdk/httpclient"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

const tagFetchStep = "rum_tag_fetch"

type rumClient interface {
	fetchJavaScriptTag(ctx context.Context, frontendID string) (string, error)
}

var _ rumClient = (*sdkRumClient)(nil)

type sdkRumClient struct {
	*installer.ExtensionClient
}

func newSDKRumClient(envURL, platformToken string) (*sdkRumClient, error) {
	ec, err := installer.NewExtensionClient(envURL, platformToken)
	if err != nil {
		return nil, err
	}
	return &sdkRumClient{ExtensionClient: ec}, nil
}

// fetchJavaScriptTag returns the OpenTelemetry RUM JavaScript tag for the given
// FRONTEND entity. An unknown entity ID is reported as an InstallFailedError so
// it is classified distinctly from generic API failures.
func (c *sdkRumClient) fetchJavaScriptTag(ctx context.Context, frontendID string) (string, error) {
	resp, err := c.C.HTTP().R().SetContext(ctx).
		Get("/platform/rum/v1/manualInsertionTags/javaScriptTag/" + url.PathEscape(frontendID))
	if err != nil {
		return "", fmt.Errorf("fetch RUM JavaScript tag for %q: %w", frontendID, err)
	}
	if resp.StatusCode() == http.StatusNotFound {
		return "", &installer.InstallFailedError{Step: tagFetchStep}
	}
	if checkErr := httpclient.CheckResponse(resp); checkErr != nil {
		return "", fmt.Errorf("fetch RUM JavaScript tag for %q: %w", frontendID, checkErr)
	}
	return strings.TrimSpace(resp.String()), nil
}

// FetchJavaScriptTag fetches the OpenTelemetry RUM JavaScript tag for the given
// FRONTEND entity using a freshly built client.
func FetchJavaScriptTag(ctx context.Context, envURL, platformToken, frontendID string) (string, error) {
	c, err := newSDKRumClient(envURL, platformToken)
	if err != nil {
		return "", fmt.Errorf("fetch RUM JavaScript tag: %w", err)
	}
	return c.fetchJavaScriptTag(ctx, frontendID)
}
