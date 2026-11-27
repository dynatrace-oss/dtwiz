package dotnet_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

// writeFile creates dir/rel (with parents) containing content and returns its path.
func writeFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func sdkProject(sdk, props string) string {
	return `<Project Sdk="` + sdk + `"><PropertyGroup>` + props + `</PropertyGroup></Project>`
}

// zipEntry is one entry of a test archive; a trailing "/" in Name makes a directory.
type zipEntry struct {
	Name    string
	Content string
	Mode    os.FileMode // zero means 0644
}

// buildZip writes a zip with the entries to path.
func buildZip(t *testing.T, path string, entries ...zipEntry) {
	t.Helper()
	data := zipBytes(t, entries...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func zipBytes(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.Name, Method: zip.Deflate}
		if e.Mode != 0 {
			h.SetMode(e.Mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.Content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// validEntries returns the archive content of a release with the given version
// for the archive's platform: every file the layout check expects.
func validEntries(version string, a dotnet.Archive) []zipEntry {
	entries := []zipEntry{
		{Name: "VERSION", Content: version + "\nv" + version + "@abc123\n"},
		{Name: "net/OpenTelemetry.AutoInstrumentation.StartupHook.dll", Content: "hook"},
		{Name: "instrument.sh", Content: "#!/bin/sh"},
	}
	for _, d := range a.NativeDirs {
		entries = append(entries, zipEntry{Name: d + "/" + a.NativeFile, Content: "native"})
	}
	return entries
}
