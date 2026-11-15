package otel

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestMain keeps tests that create instrumentation directories from writing to
// the real registry in the user's home directory.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "dtwiz-registry-test")
	if err != nil {
		panic(err)
	}
	otelRegistryPath = func() (string, error) { return filepath.Join(dir, "instrumented-dirs.json"), nil }
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func useTempRegistry(t *testing.T) {
	t.Helper()
	orig := otelRegistryPath
	path := filepath.Join(t.TempDir(), "reg", "instrumented-dirs.json")
	otelRegistryPath = func() (string, error) { return path, nil }
	t.Cleanup(func() { otelRegistryPath = orig })
}

func TestOtelRegistry_RegisterListAndPrune(t *testing.T) {
	useTempRegistry(t)
	root := t.TempDir()
	a := filepath.Join(root, "a", ".otel")
	b := filepath.Join(root, "b", ".otel")
	for _, d := range []string{a, b} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	registerOtelDir(a)
	registerOtelDir(b)
	registerOtelDir(a) // duplicate
	if got := registeredOtelDirs(); !reflect.DeepEqual(got, []string{a, b}) {
		t.Fatalf("registered = %v, want [%s %s]", got, a, b)
	}

	if err := os.RemoveAll(a); err != nil {
		t.Fatal(err)
	}
	if got := registeredOtelDirs(); !reflect.DeepEqual(got, []string{b}) {
		t.Fatalf("missing dirs must be skipped, got %v", got)
	}
	pruneOtelRegistry()
	if got := readOtelRegistry(); !reflect.DeepEqual(got, []string{b}) {
		t.Fatalf("after prune = %v, want [%s]", got, b)
	}

	if err := os.RemoveAll(b); err != nil {
		t.Fatal(err)
	}
	pruneOtelRegistry()
	if got := readOtelRegistry(); len(got) != 0 {
		t.Fatalf("empty registry expected, got %v", got)
	}
	path, _ := otelRegistryPath()
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("empty registry directory should be removed, stat err = %v", err)
	}
}

func TestOtelRegistry_KeepsNonEmptyDirectory(t *testing.T) {
	useTempRegistry(t)
	registerOtelDir(t.TempDir())
	path, _ := otelRegistryPath()
	other := filepath.Join(filepath.Dir(path), "other.txt")
	if err := os.WriteFile(other, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	// The registered directory is removed with t.TempDir cleanup only at the end,
	// so drop the entry by pointing the registry at a missing path.
	if err := writeOtelRegistry([]string{filepath.Join(t.TempDir(), "gone")}); err != nil {
		t.Fatal(err)
	}
	pruneOtelRegistry()
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("foreign file must survive: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("registry file should be gone, stat err = %v", err)
	}
}

func TestOtelRegistry_CorruptFileIsIgnored(t *testing.T) {
	useTempRegistry(t)
	path, _ := otelRegistryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := registeredOtelDirs(); len(got) != 0 {
		t.Fatalf("corrupt registry must read as empty, got %v", got)
	}
	registerOtelDir(t.TempDir()) // must recover by rewriting
	if got := readOtelRegistry(); len(got) != 1 {
		t.Fatalf("registry should be rewritten, got %v", got)
	}
}
