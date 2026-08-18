//go:build !windows

package otel

import (
	"errors"
	"syscall"
)

// isTransientDialError reports whether err is a connection-refused or
// connection-reset error: the OTel Collector's HTTP port can accept a TCP
// connection before its HTTP handler is fully initialized, causing the first
// request to be refused or reset.
func isTransientDialError(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET)
}
