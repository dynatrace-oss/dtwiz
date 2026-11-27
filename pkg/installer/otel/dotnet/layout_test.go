package dotnet_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

func isWindows() bool { return runtime.GOOS == "windows" }

func testArchives(t *testing.T) map[string]dotnet.Archive {
	t.Helper()
	cases := map[string][3]string{
		"linux-glibc-x64":  {"linux", "amd64", "glibc"},
		"linux-musl-arm64": {"linux", "arm64", "musl"},
		"macos":            {"darwin", "arm64", ""},
		"windows":          {"windows", "amd64", ""},
	}
	out := map[string]dotnet.Archive{}
	for name, c := range cases {
		a, err := dotnet.SelectArchive(c[0], c[1], c[2])
		if err != nil {
			t.Fatal(err)
		}
		out[name] = a
	}
	return out
}

func TestValidateLayout_ValidForEveryPlatform(t *testing.T) {
	for name, archive := range testArchives(t) {
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			for _, e := range validEntries("1.20.0", archive) {
				writeFile(t, dir, e.Name, e.Content)
			}

			// Act
			err := dotnet.ValidateLayout(dir, archive)

			// Assert
			if err != nil {
				t.Errorf("ValidateLayout() error = %v", err)
			}
		})
	}
}

func TestValidateLayout_NamesTheMissingFile(t *testing.T) {
	for name, archive := range testArchives(t) {
		for _, expected := range dotnet.ExpectedFiles(archive) {
			t.Run(name+"/"+expected, func(t *testing.T) {
				// Arrange
				dir := t.TempDir()
				for _, e := range validEntries("1.20.0", archive) {
					writeFile(t, dir, e.Name, e.Content)
				}
				if err := os.Remove(filepath.Join(dir, filepath.FromSlash(expected))); err != nil {
					t.Fatal(err)
				}

				// Act
				err := dotnet.ValidateLayout(dir, archive)

				// Assert
				if !errors.Is(err, dotnet.ErrLayoutInvalid) || !strings.Contains(err.Error(), expected) {
					t.Errorf("ValidateLayout() error = %v, want ErrLayoutInvalid naming %s", err, expected)
				}
			})
		}
	}
}

func TestInstalledVersion(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{"first line", "1.20.0\nv1.20.0@abc\n", "1.20.0", false},
		{"with carriage return", "1.20.0\r\n", "1.20.0", false},
		{"empty", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			writeFile(t, dir, "VERSION", tt.content)

			// Act
			got, err := dotnet.InstalledVersion(dir)

			// Assert
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("InstalledVersion() = %q, %v; want %q, wantErr=%v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestInstalledVersion_MissingFile(t *testing.T) {
	// Act
	_, err := dotnet.InstalledVersion(t.TempDir())

	// Assert
	if err == nil {
		t.Error("expected an error when VERSION does not exist")
	}
}
