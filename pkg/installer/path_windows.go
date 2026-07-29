//go:build windows

package installer

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// RefreshWindowsPath appends any user registry PATH entries missing from the
// current process's PATH.
func RefreshWindowsPath() error {
	currentPath := os.Getenv("PATH")
	registryPath, err := windowsUserPath()
	if err != nil {
		return err
	}
	mergedPath := mergePathEntries(currentPath, registryPath)
	logger.Debug("refreshing Windows PATH", "current_entries", pathEntryCount(currentPath), "registry_entries", pathEntryCount(registryPath), "merged_entries", pathEntryCount(mergedPath))
	return os.Setenv("PATH", mergedPath)
}

func pathEntryCount(pathValue string) int {
	return len(pathEntries(pathValue))
}

func windowsUserPath() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("opening user environment registry key: %w", err)
	}
	defer key.Close()

	pathValue, _, err := key.GetStringValue("Path")
	if err != nil {
		if err == registry.ErrNotExist {
			return "", nil
		}
		return "", fmt.Errorf("reading PATH from registry: %w", err)
	}
	return pathValue, nil
}
