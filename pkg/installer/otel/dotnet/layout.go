package dotnet

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	versionFile      = "VERSION"
	startupHookPath  = "net/OpenTelemetry.AutoInstrumentation.StartupHook.dll"
	versionFileLimit = 64
)

// ErrLayoutInvalid marks an extracted archive that lacks an expected file.
var ErrLayoutInvalid = errors.New("unexpected archive layout")

// ExpectedFiles lists the files, relative to the install directory, that an
// extracted archive for the platform must contain.
func ExpectedFiles(a Archive) []string {
	files := []string{versionFile, startupHookPath}
	for _, dir := range a.NativeDirs {
		files = append(files, dir+"/"+a.NativeFile)
	}
	return files
}

// ValidateLayout checks that dir holds every expected file and names the first
// missing one.
func ValidateLayout(dir string, a Archive) error {
	for _, rel := range ExpectedFiles(a) {
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil || info.IsDir() {
			return fmt.Errorf("%w: %s is missing", ErrLayoutInvalid, rel)
		}
	}
	return nil
}

// InstalledVersion reads the version from the VERSION file of an installation
// (its first line, for example "1.20.0").
func InstalledVersion(dir string) (string, error) {
	f, err := os.Open(filepath.Join(dir, versionFile))
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, versionFileLimit), versionFileLimit)
	if sc.Scan() {
		if v := strings.TrimSpace(sc.Text()); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("%w: %s is empty", ErrLayoutInvalid, versionFile)
}
