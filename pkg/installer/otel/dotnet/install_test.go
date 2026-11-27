package dotnet_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

const glibcX64Archive = "opentelemetry-dotnet-instrumentation-linux-glibc-x64.zip"

// fakeRelease serves one upstream release and counts the requests it gets.
type fakeRelease struct {
	srv *httptest.Server

	mu       sync.Mutex
	tag      string
	archive  []byte
	checksum string // checksums.txt content; empty means "computed from archive"
	apiDown  bool
	hits     map[string]int
}

func newFakeRelease(t *testing.T, version string, entries ...zipEntry) *fakeRelease {
	t.Helper()
	f := &fakeRelease{tag: "v" + version, hits: map[string]int{}}
	f.setArchive(t, entries...)
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeRelease) setArchive(t *testing.T, entries ...zipEntry) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.archive = zipBytes(t, entries...)
}

func (f *fakeRelease) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hits[r.URL.Path]++
	switch {
	case r.URL.Path == "/api":
		if f.apiDown {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, `{"tag_name":%q}`, f.tag)
	case strings.HasSuffix(r.URL.Path, "/"+glibcX64Archive):
		_, _ = w.Write(f.archive)
	case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
		if f.checksum != "" {
			_, _ = w.Write([]byte(f.checksum))
			return
		}
		sum := sha256.Sum256(f.archive)
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), glibcX64Archive)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeRelease) totalHits() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.hits {
		n += c
	}
	return n
}

func (f *fakeRelease) installer(root string) *dotnet.Installer {
	return &dotnet.Installer{
		Root:   root,
		Source: dotnet.Source{APIURL: f.srv.URL + "/api", DownloadBase: f.srv.URL + "/download"},
		Client: f.srv.Client(),
		GOOS:   "linux", GOARCH: "amd64",
		Libc: func() string { return "glibc" },
		Attestation: dotnet.Attestation{
			LookPath: func(string) (string, error) { return "", errors.New("no gh") },
			Run:      func(string, ...string) (string, error) { return "", nil },
		},
	}
}

func glibcX64(t *testing.T) dotnet.Archive {
	t.Helper()
	a, err := dotnet.SelectArchive("linux", "amd64", "glibc")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// installRoot returns a root below a fresh parent directory, like ~/.opentelemetry/dotnet.
func installRoot(t *testing.T) string {
	return filepath.Join(t.TempDir(), ".opentelemetry", "dotnet")
}

// assertNoStaging fails when staging directories were left next to root.
func assertNoStaging(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(root))
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".dotnet-install-") {
			t.Errorf("staging directory %s was left behind", e.Name())
		}
	}
}

func TestInstall_FreshInstall(t *testing.T) {
	// Arrange
	rel := newFakeRelease(t, "1.20.0", validEntries("1.20.0", glibcX64(t))...)
	root := installRoot(t)

	// Act
	res, err := rel.installer(root).Install(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if res.Version != "1.20.0" || res.Reused || res.Dir != root {
		t.Errorf("Result = %+v", res)
	}
	if err := dotnet.ValidateLayout(root, glibcX64(t)); err != nil {
		t.Errorf("installed layout invalid: %v", err)
	}
	assertNoStaging(t, root)
}

func TestInstall_InstalledIsUsedWithoutAnyRequest(t *testing.T) {
	// Arrange
	rel := newFakeRelease(t, "1.20.0", validEntries("1.20.0", glibcX64(t))...)
	root := installRoot(t)
	if _, err := rel.installer(root).Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	hitsBefore := rel.totalHits()

	// Act
	res, err := rel.installer(root).Install(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if !res.Reused || res.Version != "1.20.0" {
		t.Errorf("Result = %+v, want reused 1.20.0", res)
	}
	if got := rel.totalHits(); got != hitsBefore {
		t.Errorf("server was contacted %d more times, want none", got-hitsBefore)
	}
}

func TestInstall_NewerReleaseDoesNotReplaceInstallation(t *testing.T) {
	// Arrange
	rel := newFakeRelease(t, "1.20.0", validEntries("1.20.0", glibcX64(t))...)
	root := installRoot(t)
	if _, err := rel.installer(root).Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	rel.mu.Lock()
	rel.tag = "v1.21.0"
	rel.mu.Unlock()
	rel.setArchive(t, validEntries("1.21.0", glibcX64(t))...)
	hitsBefore := rel.totalHits()

	// Act
	res, err := rel.installer(root).Install(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if v, _ := dotnet.InstalledVersion(root); v != "1.20.0" || res.Version != "1.20.0" || !res.Reused {
		t.Errorf("installed %q, Result %+v, want 1.20.0 reused", v, res)
	}
	if got := rel.totalHits(); got != hitsBefore {
		t.Errorf("server was contacted %d more times, want none", got-hitsBefore)
	}
}

func TestInstall_InstalledWorksWhenReleaseHostIsDown(t *testing.T) {
	// Arrange
	rel := newFakeRelease(t, "1.20.0", validEntries("1.20.0", glibcX64(t))...)
	root := installRoot(t)
	if _, err := rel.installer(root).Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	rel.mu.Lock()
	rel.apiDown = true
	rel.mu.Unlock()

	// Act
	res, err := rel.installer(root).Install(context.Background())

	// Assert
	if err != nil || !res.Reused || res.Version != "1.20.0" {
		t.Errorf("Install() = %+v, %v; want the installed 1.20.0 without error", res, err)
	}
}

// brokenInstallation creates a root whose layout is invalid and a marker file inside it.
func brokenInstallation(t *testing.T, root string) {
	t.Helper()
	writeFile(t, root, "VERSION", "0.9.0\n")
	writeFile(t, root, "keep.txt", "previous content")
}

func TestInstall_BrokenInstallationIsReplaced(t *testing.T) {
	// Arrange
	rel := newFakeRelease(t, "1.20.0", validEntries("1.20.0", glibcX64(t))...)
	root := installRoot(t)
	brokenInstallation(t, root)

	// Act
	res, err := rel.installer(root).Install(context.Background())

	// Assert
	if err != nil || res.Reused || res.Version != "1.20.0" {
		t.Fatalf("Install() = %+v, %v; want a fresh 1.20.0 install", res, err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "keep.txt")); !os.IsNotExist(statErr) {
		t.Errorf("unusable content should be replaced, stat err = %v", statErr)
	}
	assertNoStaging(t, root)
}

func TestInstall_FailedReinstallOfBrokenInstallationKeepsItsContent(t *testing.T) {
	tests := []struct {
		name      string
		entries   func(a dotnet.Archive) []zipEntry
		stage     dotnet.Stage
		checksums string
	}{
		{"extraction fails", func(a dotnet.Archive) []zipEntry {
			return append(validEntries("1.20.0", a), zipEntry{Name: "../evil", Content: "x"})
		}, dotnet.StageExtraction, ""},
		{"layout invalid", func(a dotnet.Archive) []zipEntry {
			return []zipEntry{{Name: "VERSION", Content: "1.20.0\n"}}
		}, dotnet.StageLayoutValidation, ""},
		{"checksum mismatch", func(a dotnet.Archive) []zipEntry { return validEntries("1.20.0", a) },
			dotnet.StageVerification, strings.Repeat("0", 64) + "  " + glibcX64Archive + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			rel := newFakeRelease(t, "1.20.0", tt.entries(glibcX64(t))...)
			rel.checksum = tt.checksums
			root := installRoot(t)
			brokenInstallation(t, root)

			// Act
			_, err := rel.installer(root).Install(context.Background())

			// Assert
			var ie *dotnet.InstallError
			if !errors.As(err, &ie) || ie.Stage != tt.stage {
				t.Fatalf("Install() error = %v, want InstallError at stage %q", err, tt.stage)
			}
			if got, _ := os.ReadFile(filepath.Join(root, "keep.txt")); string(got) != "previous content" {
				t.Errorf("previous content changed: %q", got)
			}
			assertNoStaging(t, root)
		})
	}
}

func TestInstall_ChecksumProblemsAbortBeforeExtraction(t *testing.T) {
	tests := []struct {
		name      string
		checksums string
		wantErr   error
	}{
		{"mismatch", strings.Repeat("0", 64) + "  " + glibcX64Archive + "\n", dotnet.ErrChecksumMismatch},
		{"no entry for the archive", strings.Repeat("0", 64) + "  other.zip\n", dotnet.ErrChecksumMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			rel := newFakeRelease(t, "1.20.0", validEntries("1.20.0", glibcX64(t))...)
			rel.checksum = tt.checksums
			root := installRoot(t)

			// Act
			_, err := rel.installer(root).Install(context.Background())

			// Assert
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Install() error = %v, want %v", err, tt.wantErr)
			}
			if !strings.Contains(err.Error(), "verification") {
				t.Errorf("error should name the verification stage: %v", err)
			}
			if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
				t.Errorf("nothing should be installed, stat err = %v", statErr)
			}
		})
	}
}

func TestInstall_AttestationFailureAbortsDespiteMatchingChecksum(t *testing.T) {
	// Arrange
	rel := newFakeRelease(t, "1.20.0", validEntries("1.20.0", glibcX64(t))...)
	root := installRoot(t)
	in := rel.installer(root)
	in.Attestation = dotnet.Attestation{
		LookPath: func(string) (string, error) { return "/usr/bin/gh", nil },
		Run: func(name string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "attestation" {
				return "no matching attestation", errors.New("exit status 1")
			}
			return "", nil
		},
	}

	// Act
	_, err := in.Install(context.Background())

	// Assert
	if !errors.Is(err, dotnet.ErrAttestationFailed) {
		t.Errorf("Install() error = %v, want ErrAttestationFailed", err)
	}
	if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
		t.Errorf("nothing should be installed, stat err = %v", statErr)
	}
}

func TestInstall_InterruptedFirstInstallLeavesNothing(t *testing.T) {
	// Arrange
	rel := newFakeRelease(t, "1.20.0", validEntries("1.20.0", glibcX64(t))...)
	root := installRoot(t)
	ctx, cancel := context.WithCancel(context.Background())
	in := rel.installer(root)
	in.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, ".zip") {
			cancel() // the user presses Ctrl+C when the download starts
		}
		return rel.srv.Client().Transport.RoundTrip(r)
	})}

	// Act
	_, err := in.Install(ctx)

	// Assert
	if err == nil {
		t.Fatal("expected an error after cancellation")
	}
	if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
		t.Errorf("no installation should remain, stat err = %v", statErr)
	}
	assertNoStaging(t, root)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestInstall_ReleaseUnreachableWithoutInstallation(t *testing.T) {
	// Arrange
	rel := newFakeRelease(t, "1.20.0", validEntries("1.20.0", glibcX64(t))...)
	rel.apiDown = true
	in := rel.installer(installRoot(t))

	// Act
	_, err := in.Install(context.Background())
	_, installed := in.Installed()

	// Assert
	var ie *dotnet.InstallError
	if !errors.As(err, &ie) || ie.Stage != dotnet.StageReleaseLookup || ie.Hint == "" {
		t.Errorf("Install() error = %v, want an actionable release lookup failure", err)
	}
	if installed {
		t.Error("Installed() = true, want false")
	}
}

func TestInstall_UnsupportedPlatformDownloadsNothing(t *testing.T) {
	// Arrange
	rel := newFakeRelease(t, "1.20.0", validEntries("1.20.0", glibcX64(t))...)
	in := rel.installer(installRoot(t))
	in.GOOS, in.GOARCH = "darwin", "amd64"

	// Act
	_, err := in.Install(context.Background())

	// Assert
	var unsupported *dotnet.UnsupportedPlatformError
	if !errors.As(err, &unsupported) {
		t.Errorf("Install() error = %v, want UnsupportedPlatformError", err)
	}
	if rel.totalHits() != 0 {
		t.Errorf("server was contacted %d times, want 0", rel.totalHits())
	}
}

func TestInstall_RootNotWritable(t *testing.T) {
	if isWindows() || os.Geteuid() == 0 {
		t.Skip("permission bits are not enforced here")
	}
	// Arrange
	rel := newFakeRelease(t, "1.20.0", validEntries("1.20.0", glibcX64(t))...)
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(locked, "dotnet")

	// Act
	_, err := rel.installer(root).Install(context.Background())

	// Assert
	var ie *dotnet.InstallError
	if !errors.As(err, &ie) || !strings.Contains(ie.Hint, locked) {
		t.Errorf("Install() error = %v, want a hint naming %s", err, locked)
	}
}
