package installer

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorType classifies a command failure for self-monitoring telemetry.
type ErrorType string

const (
	ErrTypeUserCancelled       ErrorType = "user_cancelled"
	ErrTypeAuthError           ErrorType = "auth_error"
	ErrTypeConfigError         ErrorType = "config_error"
	ErrTypeDependencyMissing   ErrorType = "dependency_missing"
	ErrTypeNetworkError        ErrorType = "network_error"
	ErrTypeInstallFailed       ErrorType = "install_failed"
	ErrTypePlatformUnsupported ErrorType = "platform_unsupported"
)

// ErrPlatformUnsupported is returned when the current OS or architecture is not supported.
var ErrPlatformUnsupported = errors.New("platform not supported")

// AuthError indicates authentication failure.
type AuthError struct {
	// Reason is one of: invalid_token, environment_not_reachable, authentication_failed.
	Reason string
}

func (e *AuthError) Error() string { return fmt.Sprintf("auth error: %s", e.Reason) }
func (e *AuthError) Unwrap() error { return nil }

// ConfigError indicates missing or invalid configuration before the command could start.
type ConfigError struct {
	// MissingFields lists the names of missing environment variables or config values.
	MissingFields []string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error: missing %s", strings.Join(e.MissingFields, ", "))
}
func (e *ConfigError) Unwrap() error { return nil }

// DependencyMissingError indicates a required external binary was not found.
type DependencyMissingError struct {
	Name string
}

func (e *DependencyMissingError) Error() string { return fmt.Sprintf("%s not found", e.Name) }
func (e *DependencyMissingError) Unwrap() error { return nil }

// NetworkError indicates a network-level failure.
type NetworkError struct {
	Reason string
	URL    string
}

func (e *NetworkError) Error() string {
	if e.URL != "" {
		return fmt.Sprintf("network error: %s (%s)", e.Reason, e.URL)
	}
	return fmt.Sprintf("network error: %s", e.Reason)
}
func (e *NetworkError) Unwrap() error { return nil }

// InstallFailedError indicates an install step failed.
type InstallFailedError struct {
	Step string
}

func (e *InstallFailedError) Error() string { return fmt.Sprintf("install failed at %s", e.Step) }
func (e *InstallFailedError) Unwrap() error { return nil }
