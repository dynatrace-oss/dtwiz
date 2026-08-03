//go:build windows

package installer

import (
	"golang.org/x/sys/windows"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// IsElevated reports whether the current process has Administrator privileges.
func IsElevated() bool {
	token := windows.GetCurrentProcessToken()

	var sid *windows.SID
	if err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY, 2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0, &sid,
	); err != nil {
		logger.Debug("IsElevated: could not initialize Administrators SID", "error", err)
		return false
	}
	defer windows.FreeSid(sid)

	member, err := token.IsMember(sid)
	return err == nil && member
}
