package selfmonitoring

import (
	"errors"

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
		return installer.ErrTypeAuthError, map[string]string{"reason": authErr.Reason}
	}

	var cfgErr *installer.ConfigError
	if errors.As(err, &cfgErr) {
		attrs := make(map[string]string, len(cfgErr.MissingFields))
		for i, f := range cfgErr.MissingFields {
			if i == 0 {
				attrs["missing"] = f
			} else {
				attrs["missing"] += "," + f
			}
		}
		return installer.ErrTypeConfigError, attrs
	}

	var depErr *installer.DependencyMissingError
	if errors.As(err, &depErr) {
		return installer.ErrTypeDependencyMissing, map[string]string{"dependency": depErr.Name}
	}

	var netErr *installer.NetworkError
	if errors.As(err, &netErr) {
		attrs := map[string]string{"reason": netErr.Reason}
		if netErr.URL != "" {
			attrs["url"] = netErr.URL
		}
		return installer.ErrTypeNetworkError, attrs
	}

	if errors.Is(err, installer.ErrPlatformUnsupported) {
		return installer.ErrTypePlatformUnsupported, nil
	}

	var installErr *installer.InstallFailedError
	if errors.As(err, &installErr) {
		return installer.ErrTypeInstallFailed, map[string]string{"step": installErr.Step}
	}

	return installer.ErrTypeInstallFailed, nil
}
