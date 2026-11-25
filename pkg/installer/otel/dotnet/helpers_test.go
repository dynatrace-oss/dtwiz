package dotnet_test

import (
	"os"
	"path/filepath"
	"testing"
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
