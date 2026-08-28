// Package rum implements Dynatrace agentless RUM frontend application provisioning
// for use by the otel installer.
package rum

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
	"github.com/dynatrace-oss/dtwiz/pkg/config"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const frontendsPath = "/platform/rum/v1/frontends"

// frontendRequest is the body sent to POST /platform/rum/v1/frontends.
type frontendRequest struct {
	DisplayName  string `json:"displayName"`
	FrontendName string `json:"frontendName"`
	Type         string `json:"type"`
	EnableRum    bool   `json:"enableRum"`
}

// frontendResponse is the body returned by a successful POST /platform/rum/v1/frontends.
type frontendResponse struct {
	ID           string `json:"id"`
	FrontendName string `json:"frontendName"`
	DisplayName  string `json:"displayName"`
	Type         string `json:"type"`
}

// EnsureFrontendApplication returns the Dynatrace RUM frontend application ID for
// the project, creating one on the tenant if none is stored yet for envURL.
// The result is persisted in {projectDir}/.dtwiz/config.yaml.
func EnsureFrontendApplication(ctx context.Context, platform *client.PlatformClient, envURL, projectDir string) (string, error) {
	cfg, err := config.Load(projectDir)
	if err != nil {
		return "", fmt.Errorf("load project config: %w", err)
	}
	if fe, ok := cfg.Frontends[envURL]; ok && fe.ID != "" {
		logger.Debug("reusing existing frontend application ID", "id", fe.ID, "env", envURL)
		return fe.ID, nil
	}

	resp, err := createFrontend(ctx, platform, projectDir)
	if err != nil {
		return "", err
	}
	logger.Debug("frontend application created", "id", resp.ID, "frontendName", resp.FrontendName)

	if cfg.Frontends == nil {
		cfg.Frontends = make(map[string]config.FrontendConfig)
	}
	cfg.Frontends[envURL] = config.FrontendConfig{
		ID:           resp.ID,
		FrontendName: resp.FrontendName,
	}
	if err := config.Save(projectDir, cfg); err != nil {
		return "", fmt.Errorf("save project config: %w", err)
	}
	return resp.ID, nil
}

// createFrontend calls POST /platform/rum/v1/frontends and returns the response.
func createFrontend(ctx context.Context, platform *client.PlatformClient, projectDir string) (frontendResponse, error) {
	name, err := FrontendName(projectDir)
	if err != nil {
		return frontendResponse{}, fmt.Errorf("generate frontend name: %w", err)
	}
	displayName := filepath.Base(projectDir) + " [dtwiz]"

	body := frontendRequest{
		DisplayName:  displayName,
		FrontendName: name,
		Type:         "WEB_AGENTLESS",
		EnableRum:    true,
	}
	logger.Debug("creating frontend application", "frontendName", name, "displayName", displayName)

	resp, err := platform.HTTP().R().
		SetContext(ctx).
		SetBody(body).
		Post(frontendsPath)
	if err != nil {
		return frontendResponse{}, fmt.Errorf("create frontend application: %w", err)
	}
	if resp.StatusCode() != http.StatusCreated {
		return frontendResponse{}, fmt.Errorf("create frontend application: unexpected status %d: %s", resp.StatusCode(), resp.Body())
	}

	var result frontendResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return frontendResponse{}, fmt.Errorf("decode frontend application response: %w", err)
	}
	return result, nil
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// FrontendName produces a stable, unique frontend name from the project directory.
// Format: dtwiz-{sanitized-basename}-{8 hex chars of sha256(abs path)}
func FrontendName(projectDir string) (string, error) {
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	base := strings.ToLower(filepath.Base(abs))
	sanitized := nonAlphanumeric.ReplaceAllString(base, "-")
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		sanitized = "project"
	}

	sum := sha256.Sum256([]byte(abs))
	hash := fmt.Sprintf("%x", sum[:4]) // 4 bytes = 8 hex chars

	name := "dtwiz-" + sanitized + "-" + hash
	if len(name) > 255 {
		maxSanitized := 255 - len("dtwiz-") - 1 - 8
		sanitized = sanitized[:maxSanitized]
		name = "dtwiz-" + sanitized + "-" + hash
	}
	return name, nil
}
