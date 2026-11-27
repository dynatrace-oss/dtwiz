package dotnet

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// Verification failures.
var (
	// ErrChecksumMissing marks a checksum file without an entry for the archive.
	ErrChecksumMissing = errors.New("no checksum published for the archive")
	// ErrChecksumMismatch marks an archive that does not match its checksum.
	ErrChecksumMismatch = errors.New("archive does not match the published checksum")
	// ErrAttestationFailed marks a failed release or attestation check.
	ErrAttestationFailed = errors.New("release attestation verification failed")
)

// ParseChecksums reads `sha256sum` style lines ("<hex>  <name>") into a map
// from file name to lowercase hex digest.
func ParseChecksums(text string) map[string]string {
	sums := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		sums[strings.TrimPrefix(fields[1], "*")] = strings.ToLower(fields[0])
	}
	return sums
}

// VerifyChecksum compares the SHA-256 of the file at path with the entry for
// name in the checksum file text. It fails with ErrChecksumMissing when there
// is no entry and with ErrChecksumMismatch on a different digest.
func VerifyChecksum(path, name, checksums string) error {
	want, ok := ParseChecksums(checksums)[name]
	if !ok {
		return fmt.Errorf("%w: %s", ErrChecksumMissing, name)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("%w: %s", ErrChecksumMismatch, name)
	}
	return nil
}

// Attestation verifies release attestations with the GitHub CLI. It is an
// optional extra layer: a missing or signed-out CLI skips it silently, but once
// it runs, a failure is final.
type Attestation struct {
	LookPath func(file string) (string, error)
	Run      CommandRunner
}

// Verify checks the archive's release and build attestation for the release
// tag. It returns nil without checking when the GitHub CLI is not installed or
// not signed in, and an error wrapping ErrAttestationFailed when a check fails.
func (a Attestation) Verify(archivePath, tag string) error {
	gh, err := a.LookPath("gh")
	if err != nil {
		logger.Debug("gh not found, skipping attestation check")
		return nil
	}
	if _, err := a.Run(gh, "auth", "status"); err != nil {
		logger.Debug("gh is not signed in, skipping attestation check")
		return nil
	}
	if out, err := a.Run(gh, "release", "verify-asset", tag, archivePath, "--repo", releaseRepo); err != nil {
		return fmt.Errorf("%w: %s", ErrAttestationFailed, strings.TrimSpace(out+" "+err.Error()))
	}
	if out, err := a.Run(gh, "attestation", "verify", archivePath,
		"--repo", releaseRepo,
		"--signer-workflow", releaseRepo+"/.github/workflows/release.yml",
		"--source-ref", "refs/tags/"+tag); err != nil {
		return fmt.Errorf("%w: %s", ErrAttestationFailed, strings.TrimSpace(out+" "+err.Error()))
	}
	return nil
}
