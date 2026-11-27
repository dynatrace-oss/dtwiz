package otel

import (
	"errors"
	"fmt"
	"os"
)

// ErrRuntimeRequiresProject is returned when --runtime is given without --project.
var ErrRuntimeRequiresProject = errors.New("--runtime requires --project")

// ResolveProjectRuntime validates the --runtime value of `install otel` against
// its --project path. An empty runtimeFlag returns "" (infer from markers). The
// runtime must be active and its project markers must be present in projectPath.
func ResolveProjectRuntime(projectPath, runtimeFlag string) (RuntimeID, error) {
	if runtimeFlag == "" {
		return "", nil
	}
	if projectPath == "" {
		return "", ErrRuntimeRequiresProject
	}
	id, err := ParseRuntime(runtimeFlag)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(projectPath); err != nil {
		return "", fmt.Errorf("project path not found: %s", projectPath)
	}
	if !hasRuntimeMarkers(projectPath, id) {
		return "", fmt.Errorf("no %s project markers in %s", id, projectPath)
	}
	return id, nil
}
