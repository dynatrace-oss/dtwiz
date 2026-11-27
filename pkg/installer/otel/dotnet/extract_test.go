package dotnet_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

func TestExtract_WritesFilesWithoutExecutableBits(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.zip")
	buildZip(t, archive,
		zipEntry{Name: "net/", Mode: os.ModeDir | 0o755},
		zipEntry{Name: "net/app.dll", Content: "dll", Mode: 0o755},
		zipEntry{Name: "VERSION", Content: "1.0.0\n"},
	)
	dest := filepath.Join(dir, "out")

	// Act
	err := dotnet.Extract(archive, dest)

	// Assert
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dest, "net", "app.dll"))
	if string(got) != "dll" {
		t.Errorf("app.dll content = %q", got)
	}
	if info, _ := os.Stat(filepath.Join(dest, "net", "app.dll")); info.Mode().Perm()&0o111 != 0 && !isWindows() {
		t.Errorf("file mode = %v, want no executable bit", info.Mode())
	}
}

func TestExtract_RejectsEntriesOutsideTarget(t *testing.T) {
	tests := []struct {
		name  string
		entry string
	}{
		{"parent traversal", "../evil.txt"},
		{"nested traversal", "net/../../evil.txt"},
		{"absolute path", "/etc/evil.txt"},
		{"backslash traversal", `..\evil.txt`},
		{"drive letter", `C:\evil.txt`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			archive := filepath.Join(root, "a.zip")
			buildZip(t, archive, zipEntry{Name: tt.entry, Content: "x"})
			dest := filepath.Join(root, "out", "content")

			// Act
			err := dotnet.Extract(archive, dest)

			// Assert
			if !errors.Is(err, dotnet.ErrUnsafeArchive) {
				t.Fatalf("Extract() error = %v, want ErrUnsafeArchive", err)
			}
			if _, statErr := os.Stat(filepath.Join(root, "evil.txt")); statErr == nil {
				t.Error("a file was written outside the target directory")
			}
		})
	}
}

func TestExtract_RejectsSymlinkEntries(t *testing.T) {
	// Arrange
	root := t.TempDir()
	archive := filepath.Join(root, "a.zip")
	buildZip(t, archive, zipEntry{Name: "link", Content: "/etc/passwd", Mode: os.ModeSymlink | 0o777})

	// Act
	err := dotnet.Extract(archive, filepath.Join(root, "out"))

	// Assert
	if !errors.Is(err, dotnet.ErrUnsafeArchive) {
		t.Errorf("Extract() error = %v, want ErrUnsafeArchive", err)
	}
}

func TestExtract_RejectsOversizedArchive(t *testing.T) {
	// Arrange: a highly compressible entry just over the limit.
	root := t.TempDir()
	archive := filepath.Join(root, "bomb.zip")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.CreateHeader(&zip.FileHeader{Name: "big.bin", Method: zip.Deflate})
	chunk := []byte(strings.Repeat("\x00", 1<<20))
	for i := int64(0); i <= dotnet.MaxExtractedSize>>20; i++ {
		_, _ = w.Write(chunk)
	}
	_ = zw.Close()
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "out")

	// Act
	err := dotnet.Extract(archive, dest)

	// Assert
	if !errors.Is(err, dotnet.ErrUnsafeArchive) {
		t.Errorf("Extract() error = %v, want ErrUnsafeArchive", err)
	}
}

func TestExtract_NotAZip(t *testing.T) {
	// Arrange
	root := t.TempDir()
	archive := writeFile(t, root, "a.zip", "not a zip")

	// Act
	err := dotnet.Extract(archive, filepath.Join(root, "out"))

	// Assert
	if err == nil {
		t.Error("expected an error for a file that is not a zip")
	}
}
