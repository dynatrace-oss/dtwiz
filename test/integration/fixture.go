package integration

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// PrepareFixture copies test/fixtures/<fixture> into a new sub-directory of
// env.TempDir named after svcName and returns the destination path.
func PrepareFixture(t *testing.T, env *TestEnv, fixture, svcName string) string {
	t.Helper()
	fixtureDir := filepath.Join("..", "fixtures", fixture)
	appDir := filepath.Join(env.TempDir, svcName)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("PrepareFixture: mkdir %s: %v", appDir, err)
	}
	CopyFixture(t, fixtureDir, appDir)
	return appDir
}

// CopyFixture copies all files from fixtureDir into destDir recursively.
func CopyFixture(t *testing.T, fixtureDir, destDir string) {
	t.Helper()

	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("CopyFixture: read dir %s: %v", fixtureDir, err)
	}

	for _, entry := range entries {
		src := filepath.Join(fixtureDir, entry.Name())
		dst := filepath.Join(destDir, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dst, 0755); err != nil {
				t.Fatalf("CopyFixture: mkdir %s: %v", dst, err)
			}
			CopyFixture(t, src, dst)
			continue
		}

		if err := copyFile(src, dst); err != nil {
			t.Fatalf("CopyFixture: copy %s → %s: %v", src, dst, err)
		}
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
