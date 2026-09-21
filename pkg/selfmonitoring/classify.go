package selfmonitoring

import (
	"errors"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// ClassifyError maps err to an ErrorType and optional telemetry attributes.
// Priority order: user_cancelled → auth_error → config_error → dependency_missing
// → network_error → platform_unsupported → install_failed → fallback install_failed.
func ClassifyError(err error) (installer.ErrorType, map[string]string) {
	if err == nil {
		return installer.ErrTypeInstallFailed, nil
	}

	if errors.Is(err, installer.ErrInstallCancelled) {
		return installer.ErrTypeUserCancelled, nil
	}

	var authErr *installer.AuthError
	if errors.As(err, &authErr) {
		return installer.ErrTypeAuthError, map[string]string{"auth.failure_reason": authErr.Reason}
	}

	var cfgErr *installer.ConfigError
	if errors.As(err, &cfgErr) {
		return installer.ErrTypeConfigError, map[string]string{
			"config.missing_fields": strings.Join(cfgErr.MissingFields, ","),
		}
	}

	var depErr *installer.DependencyMissingError
	if errors.As(err, &depErr) {
		return installer.ErrTypeDependencyMissing, map[string]string{"dependency.name": depErr.Name}
	}

	var netErr *installer.NetworkError
	if errors.As(err, &netErr) {
		attrs := map[string]string{"network.failure_reason": netErr.Reason}
		if netErr.URL != "" {
			attrs["network.url"] = netErr.URL
		}
		return installer.ErrTypeNetworkError, attrs
	}

	if errors.Is(err, installer.ErrPlatformUnsupported) {
		return installer.ErrTypePlatformUnsupported, nil
	}

	var installErr *installer.InstallFailedError
	if errors.As(err, &installErr) {
		return installer.ErrTypeInstallFailed, map[string]string{"install.step": installErr.Step}
	}

	return installer.ErrTypeInstallFailed, nil
}
