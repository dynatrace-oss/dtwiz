//go:build integration

package e2e_test

import (
	"os"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

func TestMain(m *testing.M) {
	// Set AutoConfirm once for the entire test binary. Per-test save/restore
	// is unsafe when tests run in parallel: a finishing test restores the
	// global to false while other tests are still in progress.
	installer.AutoConfirm = true
	os.Exit(m.Run())
}
