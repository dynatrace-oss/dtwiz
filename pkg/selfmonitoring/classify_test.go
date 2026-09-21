package selfmonitoring

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// Classifier attributes are merged into the event body after the top-level properties, so an
// unnamespaced key silently overwrites one. Namespacing every attribute keeps them distinct.
func TestClassifyAttrsDoNotShadowBodyProperties(t *testing.T) {
	reserved := buildEventProps(EventParams{
		Cmd: "install", Sub: "oneagent", StepID: StepFailed, Mode: ModeTTY, Type: "x",
	})

	errs := []error{
		&installer.AuthError{Reason: "invalid_token"},
		&installer.ConfigError{MissingFields: []string{"DT_ENVIRONMENT"}},
		&installer.DependencyMissingError{Name: "helm"},
		&installer.NetworkError{Reason: "timeout", URL: "https://example.com"},
		&installer.InstallFailedError{Step: "download"},
	}

	for _, err := range errs {
		_, attrs := ClassifyError(err)
		for k := range attrs {
			if _, clash := reserved[k]; clash {
				t.Errorf("%T attribute %q shadows a top-level body property", err, k)
			}
		}
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantType  installer.ErrorType
		wantAttrs map[string]string
	}{
		{
			name:     "nil error → install_failed fallback",
			err:      nil,
			wantType: installer.ErrTypeInstallFailed,
		},
		{
			name:     "ErrInstallCancelled → user_cancelled",
			err:      installer.ErrInstallCancelled,
			wantType: installer.ErrTypeUserCancelled,
		},
		{
			name:      "AuthError → auth_error with reason",
			err:       &installer.AuthError{Reason: "authentication_failed"},
			wantType:  installer.ErrTypeAuthError,
			wantAttrs: map[string]string{"auth.failure_reason": "authentication_failed"},
		},
		{
			name:      "ConfigError single field → config_error",
			err:       &installer.ConfigError{MissingFields: []string{"DT_ENVIRONMENT"}},
			wantType:  installer.ErrTypeConfigError,
			wantAttrs: map[string]string{"config.missing_fields": "DT_ENVIRONMENT"},
		},
		{
			name:      "ConfigError multiple fields → config_error comma-joined",
			err:       &installer.ConfigError{MissingFields: []string{"DT_ENVIRONMENT", "DT_PLATFORM_TOKEN"}},
			wantType:  installer.ErrTypeConfigError,
			wantAttrs: map[string]string{"config.missing_fields": "DT_ENVIRONMENT,DT_PLATFORM_TOKEN"},
		},
		{
			name:      "DependencyMissingError → dependency_missing",
			err:       &installer.DependencyMissingError{Name: "az"},
			wantType:  installer.ErrTypeDependencyMissing,
			wantAttrs: map[string]string{"dependency.name": "az"},
		},
		{
			name:      "NetworkError with URL → network_error with url attr",
			err:       &installer.NetworkError{Reason: "environment_not_reachable", URL: "https://example.dynatracelabs.com"},
			wantType:  installer.ErrTypeNetworkError,
			wantAttrs: map[string]string{"network.failure_reason": "environment_not_reachable", "network.url": "https://example.dynatracelabs.com"},
		},
		{
			name:      "NetworkError without URL → network_error without url attr",
			err:       &installer.NetworkError{Reason: "timeout"},
			wantType:  installer.ErrTypeNetworkError,
			wantAttrs: map[string]string{"network.failure_reason": "timeout"},
		},
		{
			name:     "ErrPlatformUnsupported → platform_unsupported",
			err:      installer.ErrPlatformUnsupported,
			wantType: installer.ErrTypePlatformUnsupported,
		},
		{
			name:      "InstallFailedError → install_failed with step",
			err:       &installer.InstallFailedError{Step: "download"},
			wantType:  installer.ErrTypeInstallFailed,
			wantAttrs: map[string]string{"install.step": "download"},
		},
		{
			name:     "unknown error → install_failed fallback",
			err:      errors.New("some unexpected error"),
			wantType: installer.ErrTypeInstallFailed,
		},
		{
			name:      "wrapped AuthError is detected via errors.As",
			err:       fmt.Errorf("context: %w", &installer.AuthError{Reason: "invalid_token"}),
			wantType:  installer.ErrTypeAuthError,
			wantAttrs: map[string]string{"auth.failure_reason": "invalid_token"},
		},
		{
			name:     "ErrInstallCancelled wrapped → user_cancelled",
			err:      fmt.Errorf("setup: %w", installer.ErrInstallCancelled),
			wantType: installer.ErrTypeUserCancelled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotAttrs := ClassifyError(tt.err)
			if gotType != tt.wantType {
				t.Errorf("ClassifyError() type = %q, want %q", gotType, tt.wantType)
			}
			if len(tt.wantAttrs) == 0 && len(gotAttrs) == 0 {
				return
			}
			for k, wantV := range tt.wantAttrs {
				if gotV := gotAttrs[k]; gotV != wantV {
					t.Errorf("ClassifyError() attrs[%q] = %q, want %q", k, gotV, wantV)
				}
			}
			for k := range gotAttrs {
				if _, ok := tt.wantAttrs[k]; !ok {
					t.Errorf("ClassifyError() unexpected attrs key %q", k)
				}
			}
		})
	}
}
