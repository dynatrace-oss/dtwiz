package oneagent

import (
	"net/http"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// installerScopeHint is appended to 401/403 errors from the installer APIs so the user
// knows which permission to add.
const installerScopeHint = "if using a platform token, ensure it has the InstallerDownload scope; " +
	"or pass a dt0c01.* access token with --access-token"

// classifiedError keeps a user-facing message while exposing a typed installer error (and
// the underlying cause) to errors.As. selfmonitoring.ClassifyError relies on that to report
// auth and network failures with their own error type instead of install_failed.
type classifiedError struct {
	msg   string
	class error
	cause error
}

func (e *classifiedError) Error() string { return e.msg }

func (e *classifiedError) Unwrap() []error {
	if e.cause == nil {
		return []error{e.class}
	}
	return []error{e.class, e.cause}
}

// installerAuthError returns msg as an installer.AuthError. A 403 means the token
// authenticated but lacks a permission; anything else is an authentication failure.
func installerAuthError(msg string, status int) error {
	reason := "authentication_failed"
	if status == http.StatusForbidden {
		reason = "insufficient_permissions"
	}
	return &classifiedError{msg: msg, class: &installer.AuthError{Reason: reason}}
}

// installerNetworkError returns msg as an installer.NetworkError for url, keeping cause.
func installerNetworkError(msg, reason, url string, cause error) error {
	return &classifiedError{msg: msg, class: &installer.NetworkError{Reason: reason, URL: url}, cause: cause}
}
