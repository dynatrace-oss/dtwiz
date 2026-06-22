//go:build !windows

package oneagent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// TestLifecycle_InstallThenUninstall exercises the full OneAgent V2 lifecycle:
// InstallOneAgentV2 (with a fake HTTP server serving a stub installer) → oneAgentInstalled()
// returns true → UninstallOneAgentV2 → oneAgentInstalled() returns false.
//
// It is the only test that chains InstallOneAgentV2 and UninstallOneAgentV2 in a
// single sequence; the individual install/uninstall/detection branches are covered
// by the unit tests in oneagent_test.go, uninstall_test.go, and detect_unix_test.go.
func TestLifecycle_InstallThenUninstall(t *testing.T) {
	skipNonLinux(t)

	// Redirect install dir to a temp path that does not yet exist.
	base := t.TempDir()
	installDir := filepath.Join(base, "oneagent")
	withInstallDir(t, installDir)
	withNeedsSudo(t, false)

	// The stub installer script reads DTWIZ_TEST_INSTALL_DIR and creates the
	// directory structure that oneAgentInstalled() and UninstallOneAgentV2 expect.
	t.Setenv("DTWIZ_TEST_INSTALL_DIR", installDir)
	installerScript := fmt.Sprintf(`#!/bin/sh
mkdir -p "%s/agent"
printf '#!/bin/sh\nexit 0\n' > "%s/agent/uninstall.sh"
chmod +x "%s/agent/uninstall.sh"
exit 0
`, installDir, installDir, installDir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "installer/agent") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(installerScript))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := newMockClient(t, srv.URL)

	// Phase 1: install
	flush := captureStdout(t)
	err := InstallOneAgentV2(c, InstallOptions{
		MonitoringMode:        "fullstack",
		SkipConnectivityCheck: true,
		NoVerifySignature:     true,
		Quiet:                 true,
	})
	flush()

	if err != nil {
		t.Fatalf("InstallOneAgentV2 failed: %v", err)
	}
	if !oneAgentInstalled() {
		t.Error("expected oneAgentInstalled() true after InstallOneAgentV2")
	}

	// Phase 2: uninstall
	origAC := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = origAC })

	flush = captureStdout(t)
	err = UninstallOneAgentV2(UninstallOptions{DryRun: false})
	flush()

	if err != nil {
		t.Fatalf("UninstallOneAgentV2 failed: %v", err)
	}
	if oneAgentInstalled() {
		t.Error("expected oneAgentInstalled() false after UninstallOneAgentV2")
	}
}
