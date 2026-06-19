//go:build windows

package oneagent

import "golang.org/x/sys/windows"

// isElevated reports whether the current process has Administrator privileges.
func isElevated() bool {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer token.Close()

	var sid *windows.SID
	if err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY, 2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0, &sid,
	); err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	member, err := token.IsMember(sid)
	return err == nil && member
}
