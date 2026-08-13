//go:build !windows

package otel

import (
	"errors"
	"net"
	"strconv"
	"testing"
)

func TestIsTransientDialError_ConnectionRefused(t *testing.T) {
	port := findFreePort(45300)
	_, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err == nil {
		t.Fatal("expected a dial error against a port nothing listens on")
	}
	if !isTransientDialError(err) {
		t.Errorf("isTransientDialError(%v) = false, want true for a connection-refused error", err)
	}
}

func TestIsTransientDialError_UnrelatedErrorsAreNotTransient(t *testing.T) {
	if isTransientDialError(errors.New("some unrelated error")) {
		t.Error("isTransientDialError should not treat an arbitrary error as transient")
	}
}
