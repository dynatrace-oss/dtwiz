package otel

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// ProjectQuery selects what DetectProjects scans.
type ProjectQuery struct {
	Dir      string      // scan root; relative paths resolve against the working directory; "" means the working directory
	Runtimes []RuntimeID // runtimes in scope; empty means all active runtimes
}

// ProjectStatus describes what `dtwiz install otel` does for a project.
type ProjectStatus int

const (
	// Instrumentable: the install instruments the application.
	Instrumentable ProjectStatus = iota
	// ManualSteps: the install ends in the manual-steps outcome.
	ManualSteps
	// RuntimeUnavailable: the runtime's toolchain is not usable on this machine.
	RuntimeUnavailable
)

// RuntimeStatus reports whether a runtime in scope is usable on this machine.
type RuntimeStatus struct {
	ID        RuntimeID
	Available bool
	Reason    string // set when not available
}

// InventoryProject is one detected project of one runtime.
type InventoryProject struct {
	Runtime RuntimeID
	Path    string // absolute
	Markers []string
	PIDs    []int
	Module  string
	Status  ProjectStatus
}

// ProjectInventory is the result of DetectProjects.
type ProjectInventory struct {
	Roots    []string        // absolute
	Filter   []RuntimeID     // runtimes the query asked for; empty when unfiltered
	Runtimes []RuntimeStatus // runtimes in scope, in identifier order
	Projects []InventoryProject
}

// DetectProjects lists the projects under the query's root, without prompting
// and without writing to stdout. Unlike the install flow it scans runtimes whose
// toolchain is unavailable too, and reports them as such.
func DetectProjects(query ProjectQuery) (ProjectInventory, error) {
	root, err := resolveInventoryRoot(query.Dir)
	if err != nil {
		return ProjectInventory{}, err
	}
	runtimes, err := runtimesInScope(query.Runtimes)
	if err != nil {
		return ProjectInventory{}, err
	}

	type result struct {
		status   RuntimeStatus
		projects []InventoryProject
	}
	results := make([]result, len(runtimes))
	var wg sync.WaitGroup
	for i, rt := range runtimes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			availability := checkRuntimeAvailability(rt)
			status := Instrumentable
			switch {
			case !availability.available:
				status = RuntimeUnavailable
			case rt.endsInManualSteps:
				status = ManualSteps
			}
			detected := rt.detect([]string{root})
			projects := make([]InventoryProject, 0, len(detected))
			for _, p := range detected {
				projects = append(projects, InventoryProject{
					Runtime: rt.id,
					Path:    p.Path,
					Markers: p.Markers,
					PIDs:    p.RunningProcessIDs,
					Module:  p.ModuleName,
					Status:  status,
				})
			}
			results[i] = result{
				status:   RuntimeStatus{ID: rt.id, Available: availability.available, Reason: availability.reason},
				projects: projects,
			}
		}()
	}
	wg.Wait()

	inventory := ProjectInventory{Roots: []string{root}, Filter: query.Runtimes, Projects: []InventoryProject{}}
	for _, r := range results {
		inventory.Runtimes = append(inventory.Runtimes, r.status)
		inventory.Projects = append(inventory.Projects, r.projects...)
	}
	slices.SortStableFunc(inventory.Projects, func(a, b InventoryProject) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return strings.Compare(string(a.Runtime), string(b.Runtime))
	})
	return inventory, nil
}

// resolveInventoryRoot returns the absolute, symlink-resolved scan root, so
// it compares equal to the resolved project paths the scanner records.
func resolveInventoryRoot(dir string) (string, error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("determining working directory: %w", err)
		}
		dir = wd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving directory %s: %w", dir, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("directory not found: %s", dir)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", dir)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return abs, nil
}

// runtimesInScope returns the active runtimes selected by ids (all active
// runtimes when ids is empty), in identifier order.
func runtimesInScope(ids []RuntimeID) ([]runtimeInfo, error) {
	for _, id := range ids {
		if _, ok := activeRuntime(id); !ok {
			return nil, &UnknownRuntimeError{Value: string(id), Valid: ActiveRuntimeIDs()}
		}
	}
	var scoped []runtimeInfo
	for _, rt := range detectAvailableRuntimes() {
		if rt.enabled && (len(ids) == 0 || slices.Contains(ids, rt.id)) {
			scoped = append(scoped, rt)
		}
	}
	return scoped, nil
}
