//go:build !windows

package installer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	fmt.Printf("  This will uninstall dtwiz:\n")
	fmt.Println()
	fmt.Printf("    Binary  : %s\n", exePath)
	if profilePath != "" {
		fmt.Printf("    Profile : remove PATH entry from %s\n", profilePath)
	}
	fmt.Println()
	fmt.Print("  Proceed? [Y/n]: ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "" && answer != "y" && answer != "yes" {
		return fmt.Errorf("uninstall aborted")
	}
	fmt.Println()

	// 1. Remove the PATH block from the shell profile.
	if profilePath != "" {
		if err := removeInstallerPathBlock(profilePath, installDir); err != nil {
			fmt.Printf("  Warning: could not update %s: %v\n", profilePath, err)
		} else {
			fmt.Printf("  Removed PATH entry from %s\n", profilePath)
		}
	}

	// 2. Delete the binary (last — doing this first would prevent profile cleanup).
	if err := os.Remove(exePath); err != nil {
		return fmt.Errorf("removing binary %s: %w", exePath, err)
	}
	fmt.Printf("  Removed %s\n", exePath)
	fmt.Println()
	fmt.Println("  dtwiz uninstalled.")
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
