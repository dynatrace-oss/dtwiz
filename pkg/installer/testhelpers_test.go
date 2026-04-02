package installer

import (
	"os"
	"sync"
	"testing"
)

// cwdMu serializes tests that temporarily change the process working directory.
// os.Chdir mutates global process state; tests must hold this lock for the duration.
var cwdMu sync.Mutex

// setTestWorkingDir changes CWD to dir for the duration of the test, then restores it.
// Acquires cwdMu to prevent concurrent CWD mutation if t.Parallel() is ever added.
func setTestWorkingDir(t *testing.T, dir string) {
	t.Helper()
	cwdMu.Lock()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		cwdMu.Unlock()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chdir(orig) //nolint:errcheck
		cwdMu.Unlock()
	})
}
