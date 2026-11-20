//go:build !windows

package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
)

// UninstallSelf removes the dtwiz binary from disk and strips the PATH
// entry that the install script added to the user's shell profile.
func UninstallSelf() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}
	// Follow any symlinks so we operate on the real file.
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}
	installDir := filepath.Dir(exePath)

	// Locate the shell profile that likely has the PATH export.
	profilePath := detectShellProfile()

	// Show preview and confirm.
	fmt.Println()
	display.Header("dtwiz · uninstall dtwiz")
	rows := []string{"Removes", exePath}
	if profilePath != "" {
		rows = append(rows, "Updates", "remove PATH entry from "+profilePath)
	}
	display.PrintAlignedStatusLines(display.ColorDefault, rows...)
	fmt.Println()
	ok, err := ConfirmQuestion("  Apply?")
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		return fmt.Errorf("uninstall aborted")
	}
	fmt.Println()

	// 1. Remove the PATH block from the shell profile.
	if profilePath != "" {
		if err := removeInstallerPathBlock(profilePath, installDir); err != nil {
			display.StepWarn("Could not update "+profilePath, err.Error())
		} else {
			display.StepDone("Removed PATH entry", profilePath)
		}
	}

	// 2. Delete the binary (last — doing this first would prevent profile cleanup).
	if err := os.Remove(exePath); err != nil {
		return fmt.Errorf("removing binary %s: %w", exePath, err)
	}
	display.StepDone("Removed dtwiz binary", exePath)
	return nil
}

// detectShellProfile returns the path of the shell RC/profile file that the
// install script would have written to, based on SHELL and OS.
func detectShellProfile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	shell := os.Getenv("SHELL")
	switch {
	case strings.HasSuffix(shell, "zsh"):
		return filepath.Join(home, ".zshrc")
	case strings.HasSuffix(shell, "bash"):
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, ".bash_profile")
		}
		return filepath.Join(home, ".bashrc")
	default:
		return filepath.Join(home, ".profile")
	}
}

// removeInstallerPathBlock removes the two-line block the installer wrote:
//
//	# Added by dtwiz installer
//	export PATH="<installDir>:$PATH"
//
// If the block is not found the file is left unchanged (not an error).
func removeInstallerPathBlock(profilePath, installDir string) error {
	data, err := os.ReadFile(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		line := lines[i]
		// Match the comment marker the installer writes.
		if strings.TrimSpace(line) == "# Added by dtwiz installer" {
			// Peek at the next non-empty line to confirm it's our export.
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			if j < len(lines) && strings.Contains(lines[j], installDir) {
				// Skip the comment, any blank lines between, and the export line.
				i = j + 1
				continue
			}
		}
		// Also catch just the export line in case only that was written.
		if strings.Contains(line, `export PATH=`) && strings.Contains(line, installDir) {
			i++
			continue
		}
		out = append(out, line)
		i++
	}

	return os.WriteFile(profilePath, []byte(strings.Join(out, "\n")), 0o644)
}
