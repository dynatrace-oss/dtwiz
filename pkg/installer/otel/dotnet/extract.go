package dotnet

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MaxExtractedSize bounds the total uncompressed size of an archive. Real
// releases are under 100 MB; the limit guards against decompression bombs.
const MaxExtractedSize int64 = 512 << 20

// ErrUnsafeArchive marks an archive that cannot be extracted safely.
var ErrUnsafeArchive = errors.New("unsafe archive")

// Extract unpacks the zip at archivePath into destDir (created when missing).
// Entries that would land outside destDir, symbolic links, and archives that
// expand beyond MaxExtractedSize are rejected with ErrUnsafeArchive. Files are
// written as 0644 and directories as 0755: nothing in the instrumentation
// needs an executable bit.
func Extract(archivePath, destDir string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("opening archive: %w", err)
	}
	defer zr.Close()

	var declared uint64
	for _, f := range zr.File {
		declared += f.UncompressedSize64
	}
	if declared > uint64(MaxExtractedSize) {
		return fmt.Errorf("%w: expands to %d bytes, limit is %d", ErrUnsafeArchive, declared, MaxExtractedSize)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	remaining := MaxExtractedSize
	for _, f := range zr.File {
		target, err := entryPath(destDir, f.Name)
		if err != nil {
			return err
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: entry %q is a symbolic link", ErrUnsafeArchive, f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		n, err := extractFile(f, target, remaining)
		if err != nil {
			return err
		}
		remaining -= n
	}
	return nil
}

// entryPath resolves a zip entry name below destDir or fails when it escapes.
func entryPath(destDir, name string) (string, error) {
	slashed := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(slashed, "/") || (len(slashed) >= 2 && slashed[1] == ':') {
		return "", fmt.Errorf("%w: entry %q has an absolute path", ErrUnsafeArchive, name)
	}
	target := filepath.Join(destDir, filepath.FromSlash(slashed))
	rel, err := filepath.Rel(destDir, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: entry %q escapes the target directory", ErrUnsafeArchive, name)
	}
	return target, nil
}

func extractFile(f *zip.File, target string, limit int64) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}
	// Read one byte past the limit to tell "exactly at the limit" from "over".
	n, err := io.Copy(out, io.LimitReader(rc, limit+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, fmt.Errorf("%w: archive expands beyond %d bytes", ErrUnsafeArchive, MaxExtractedSize)
	}
	return n, nil
}
