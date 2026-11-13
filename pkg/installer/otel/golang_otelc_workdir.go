package otel

import (
	"fmt"
	"os"
)

// otelcWorkDirs hands out one otelc work directory per module for the length of
// a run. otelc keeps its analysis and generated files there, so services of the
// same module built one after another reuse it and the later builds are much
// faster. Nothing outlives the run: Close removes every directory.
type otelcWorkDirs struct {
	dirs map[string]string
}

// For returns the work directory of a module, creating it on first use.
func (w *otelcWorkDirs) For(module string) (string, error) {
	if dir, ok := w.dirs[module]; ok {
		return dir, nil
	}
	dir, err := os.MkdirTemp("", "dtwiz-otelc-*")
	if err != nil {
		return "", fmt.Errorf("create otelc work directory: %w", err)
	}
	if w.dirs == nil {
		w.dirs = map[string]string{}
	}
	w.dirs[module] = dir
	return dir, nil
}

// Reset discards a module's work directory, so the next build starts clean. It
// is called after a failed build, whose leftovers must not affect other services.
func (w *otelcWorkDirs) Reset(module string) {
	if dir, ok := w.dirs[module]; ok {
		_ = os.RemoveAll(dir)
		delete(w.dirs, module)
	}
}

// Close removes every work directory.
func (w *otelcWorkDirs) Close() {
	for module := range w.dirs {
		w.Reset(module)
	}
}
