//go:build windows

package installer

import (
	"fmt"
	"os"
	"path/filepath"
)

// UninstallSelf prints ready-to-paste PowerShell commands to remove dtwiz
// on Windows. Executing the binary after it deletes itself is problematic on
// Windows, so we print instructions instead.
func UninstallSelf() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}
	installDir := filepath.Dir(exePath)

	fmt.Println()
	fmt.Printf("  dtwiz is installed at: %s\n", exePath)
	fmt.Println()
	fmt.Println("  To uninstall, paste the following into PowerShell:")
	fmt.Println()
	fmt.Printf("    $dir = %q\n", installDir)
	fmt.Printf("    Remove-Item \"$dir\\dtwiz.exe\"\n")
	fmt.Println(`    $path = [Environment]::GetEnvironmentVariable("PATH", "User")`)
	fmt.Println(`    [Environment]::SetEnvironmentVariable("PATH", (($path -split ";") -ne $dir -join ";"), "User")`)
	fmt.Println(`    Remove-Item -Recurse $dir`)
	fmt.Println()
	return nil
}
