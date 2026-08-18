//go:build windows

package otel

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isTransientDialError reports whether err is a connection-refused or
// connection-reset error: the OTel Collector's HTTP port can accept a TCP
// connection before its HTTP handler is fully initialized, causing the first
// request to be refused or reset.
//
// The WinSock error codes checked here are distinct from the "invented"
// syscall.ECONNREFUSED/ECONNRESET constants the stdlib syscall package
// defines on Windows (those exist only for POSIX-style os-package checks
// such as errors.Is(err, fs.ErrNotExist) and never match a real WinSock
// error), so they must be matched explicitly per platform.
func isTransientDialError(err error) bool {
	return errors.Is(err, windows.WSAECONNREFUSED) || errors.Is(err, windows.WSAECONNRESET)
}
