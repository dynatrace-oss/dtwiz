package featureflags

// testCleaner is a minimal interface satisfied by *testing.T and *testing.B,
// allowing SetCLIOverrideForTest to avoid importing the testing package in production code.
type testCleaner interface {
	Cleanup(func())
}

// SetCLIOverrideForTest injects a CLI-scoped override for the given flag,
// equivalent to the user having passed the flag explicitly on the command line.
// Use this when testing that CLI overrides take precedence over env vars or
// defaults. The override is automatically removed via t.Cleanup.
func SetCLIOverrideForTest(t testCleaner, flag Flag, val bool) {
	mu.Lock()
	prev, hadPrev := cliOverrides[flag]
	cliOverrides[flag] = val
	mu.Unlock()

	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		if hadPrev {
			cliOverrides[flag] = prev
		} else {
			delete(cliOverrides, flag)
		}
	})
}
