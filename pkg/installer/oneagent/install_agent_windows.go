//go:build windows

package oneagent

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"golang.org/x/sys/windows"
)

var (
	modShell32         = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteEx = modShell32.NewProc("ShellExecuteExW")
)

// shellExecuteInfo mirrors the Windows SHELLEXECUTEINFOW struct.
// Go's natural field alignment matches the C ABI on both 32-bit and 64-bit Windows,
// so no explicit padding fields are needed.
type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           uintptr
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       uintptr // Go inserts 4-byte padding before this on amd64
	lpIDList       uintptr
	lpClass        *uint16
	hkeyProgID     uintptr
	dwHotKey       uint32
	hIconOrMonitor uintptr // Go inserts 4-byte padding before this on amd64
	hProcess       uintptr
}

const seeMaskNoCloseProcess = 0x00000040

// runInstaller on Windows uses ShellExecuteEx with the "runas" verb when not
// already running as Administrator, so Windows can show the UAC consent dialog
// and grant Administrator privileges to the OneAgent installer process.
// exec.Command (CreateProcess) cannot launch processes that declare requireAdministrator
// in their manifest from a non-elevated caller — it returns "The handle is invalid".
func runInstaller(argv []string, quiet bool) (int, error) {
	if isElevatedFn() {
		return installer.RunCommandWithExitCode(argv, quiet)
	}
	return runElevatedInstaller(argv)
}

func runElevatedInstaller(argv []string) (int, error) {
	if len(argv) == 0 {
		return 1, fmt.Errorf("empty command")
	}

	exePtr, err := windows.UTF16PtrFromString(argv[0])
	if err != nil {
		return 1, fmt.Errorf("installer path: %w", err)
	}
	verbPtr, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return 1, fmt.Errorf("runas verb: %w", err)
	}

	info := shellExecuteInfo{
		fMask:  seeMaskNoCloseProcess,
		lpVerb: verbPtr,
		lpFile: exePtr,
		nShow:  1, // SW_SHOWNORMAL
	}
	info.cbSize = uint32(unsafe.Sizeof(info))

	if len(argv) > 1 {
		params := quoteWindowsArgs(argv[1:])
		info.lpParameters, err = windows.UTF16PtrFromString(params)
		if err != nil {
			return 1, fmt.Errorf("installer parameters: %w", err)
		}
	}

	logger.Debug("launching installer via ShellExecuteEx", "exe", argv[0])
	r, _, e := procShellExecuteEx.Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return 1, fmt.Errorf("ShellExecuteEx: %w", e)
	}

	if info.hProcess == 0 {
		return 1, fmt.Errorf("ShellExecuteEx returned no process handle")
	}
	defer windows.CloseHandle(windows.Handle(info.hProcess))

	if _, err := windows.WaitForSingleObject(windows.Handle(info.hProcess), windows.INFINITE); err != nil {
		return 1, fmt.Errorf("waiting for installer: %w", err)
	}

	var exitCode uint32
	if err := windows.GetExitCodeProcess(windows.Handle(info.hProcess), &exitCode); err != nil {
		return 1, fmt.Errorf("getting installer exit code: %w", err)
	}

	if exitCode != 0 {
		return int(exitCode), fmt.Errorf("exited with code %d", exitCode)
	}
	return 0, nil
}

// quoteWindowsArgs joins args into a parameter string for ShellExecuteEx
// following CommandLineToArgvW escaping rules.
func quoteWindowsArgs(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = quoteWindowsArg(a)
	}
	return strings.Join(parts, " ")
}

// quoteWindowsArg quotes a single argument per CommandLineToArgvW semantics:
// backslashes are only special immediately before a double-quote or at the end
// of a quoted string, where they must be doubled.
func quoteWindowsArg(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for _, c := range s {
		switch c {
		case '\\':
			slashes++
		case '"':
			// Double the preceding backslashes, then escape the quote.
			b.WriteString(strings.Repeat(`\`, slashes*2))
			slashes = 0
			b.WriteString(`\"`)
		default:
			b.WriteString(strings.Repeat(`\`, slashes))
			slashes = 0
			b.WriteRune(c)
		}
	}
	// Trailing backslashes precede the closing quote and must be doubled.
	b.WriteString(strings.Repeat(`\`, slashes*2))
	b.WriteByte('"')
	return b.String()
}
