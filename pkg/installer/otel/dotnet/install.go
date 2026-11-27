package dotnet

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// DefaultRoot returns the dtwiz-owned directory the instrumentation is
// installed into: ~/.opentelemetry/dotnet (the user profile on Windows).
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine home directory: %w", err)
	}
	return filepath.Join(home, ".opentelemetry", "dotnet"), nil
}

// Installer installs the upstream instrumentation into Root. All environment
// access is injected so that tests need neither the network nor the real home
// directory.
type Installer struct {
	// Root is the install directory; the archive content lands directly in it.
	Root   string
	Source Source
	Client *http.Client
	// GOOS and GOARCH describe the machine (runtime.GOOS and runtime.GOARCH).
	GOOS, GOARCH string
	// Libc returns the Linux C library ("glibc", "musl" or ""); it is only
	// called on Linux.
	Libc        func() string
	Attestation Attestation
}

// Result describes the outcome of Install.
type Result struct {
	Dir     string
	Version string
	// Reused is true when a usable installation existed and nothing was
	// downloaded.
	Reused bool
}

// Archive returns the release archive for this machine; it fails with
// *UnsupportedPlatformError when the upstream release has none.
func (in *Installer) Archive() (Archive, error) {
	libc := ""
	if in.GOOS == "linux" && in.Libc != nil {
		libc = in.Libc()
	}
	return SelectArchive(in.GOOS, in.GOARCH, libc)
}

// Installed returns the version of a valid installation in Root for this
// platform, or ok == false when there is none.
func (in *Installer) Installed() (version string, ok bool) {
	a, err := in.Archive()
	if err != nil {
		return "", false
	}
	if ValidateLayout(in.Root, a) != nil {
		return "", false
	}
	v, err := InstalledVersion(in.Root)
	if err != nil {
		return "", false
	}
	return v, true
}

// Install makes the instrumentation available in Root. A usable installation
// is used as it is: nothing is looked up, downloaded or verified. Otherwise the
// latest upstream release is downloaded, verified, extracted and validated in a
// staging directory and swapped in. A failure leaves the previous content of
// Root, if any, untouched.
//
// An unsupported platform is returned as *UnsupportedPlatformError before
// anything is downloaded; every other failure is an *InstallError.
func (in *Installer) Install(ctx context.Context) (Result, error) {
	archive, err := in.Archive()
	if err != nil {
		return Result{}, err
	}
	logger.Debug("dotnet instrumentation archive selected", "archive", archive.Name, "root", in.Root)

	if current, ok := in.Installed(); ok {
		logger.Debug("dotnet instrumentation already installed, using it", "version", current)
		return Result{Dir: in.Root, Version: current, Reused: true}, nil
	}

	release, err := in.Source.LatestRelease(ctx, in.Client)
	if err != nil {
		return Result{}, in.classify(StageReleaseLookup, err)
	}
	logger.Debug("dotnet instrumentation latest release", "tag", release.Tag)

	parent := filepath.Dir(in.Root)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Result{}, in.classify(StageInstall, err)
	}
	staging, err := os.MkdirTemp(parent, ".dotnet-install-")
	if err != nil {
		return Result{}, in.classify(StageInstall, err)
	}
	defer os.RemoveAll(staging)

	zipPath := filepath.Join(staging, archive.Name)
	if err := in.fetchAndVerify(ctx, release, archive, zipPath); err != nil {
		return Result{}, err
	}

	content := filepath.Join(staging, "content")
	if err := Extract(zipPath, content); err != nil {
		return Result{}, in.classify(StageExtraction, err)
	}
	if err := ValidateLayout(content, archive); err != nil {
		return Result{}, in.classify(StageLayoutValidation, err)
	}
	version, err := InstalledVersion(content)
	if err != nil {
		return Result{}, in.classify(StageLayoutValidation, err)
	}
	if err := in.swap(content, staging); err != nil {
		return Result{}, in.classify(StageInstall, err)
	}
	logger.Debug("dotnet instrumentation installed", "version", version, "dir", in.Root)
	return Result{Dir: in.Root, Version: version}, nil
}

func (in *Installer) fetchAndVerify(ctx context.Context, rel Release, archive Archive, zipPath string) error {
	url := rel.AssetURL(archive.Name)
	logger.Debug("downloading dotnet instrumentation", "url", url)
	if err := Download(ctx, in.Client, url, zipPath); err != nil {
		return in.classify(StageDownload, err)
	}
	sums, err := fetchText(ctx, in.Client, rel.ChecksumsURL())
	if err != nil {
		return in.classify(StageDownload, err)
	}
	if err := VerifyChecksum(zipPath, archive.Name, sums); err != nil {
		return in.classify(StageVerification, err)
	}
	logger.Debug("dotnet instrumentation checksum verified", "archive", archive.Name)
	if err := in.Attestation.Verify(zipPath, rel.Tag); err != nil {
		return in.classify(StageVerification, err)
	}
	return nil
}

// swap moves the validated content into Root. The previous installation is
// moved aside first and restored when the final rename fails.
func (in *Installer) swap(content, staging string) error {
	previous := filepath.Join(staging, "previous")
	hadPrevious := false
	if _, err := os.Stat(in.Root); err == nil {
		if err := os.Rename(in.Root, previous); err != nil {
			return err
		}
		hadPrevious = true
	}
	if err := os.Rename(content, in.Root); err != nil {
		if hadPrevious {
			if rerr := os.Rename(previous, in.Root); rerr != nil {
				return fmt.Errorf("%w (restoring the previous installation also failed: %v)", err, rerr)
			}
		}
		return err
	}
	return nil
}

// classify wraps err in an InstallError for the stage and adds a hint for
// causes the user can fix.
func (in *Installer) classify(stage Stage, err error) error {
	e := stageErr(stage, err)
	var netErr net.Error
	switch {
	case errors.Is(err, ErrRateLimited):
		e.Hint = "GitHub rate-limited the request; try again later"
	case errors.Is(err, fs.ErrPermission):
		e.Hint = "check that you can write to " + filepath.Dir(in.Root)
	case errors.Is(err, syscall.ENOSPC):
		e.Hint = "free some disk space and try again"
	case errors.As(err, &netErr), errors.Is(err, context.DeadlineExceeded):
		e.Hint = "check your network connection"
	}
	if e.Hint == "" && (stage == StageReleaseLookup || stage == StageDownload) && !errors.Is(err, context.Canceled) {
		e.Hint = "check your network connection"
	}
	return e
}
