package projectreport

import (
	"encoding/json"
	"io"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel"
)

type jsonInventory struct {
	Roots    []string      `json:"roots"`
	Runtimes []jsonRuntime `json:"runtimes"`
	Projects []jsonProject `json:"projects"`
}

type jsonRuntime struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type jsonProject struct {
	Runtime        string   `json:"runtime"`
	Path           string   `json:"path"`
	Markers        []string `json:"markers"`
	PIDs           []int    `json:"pids,omitempty"`
	Module         string   `json:"module,omitempty"`
	Status         string   `json:"status"`
	InstallCommand string   `json:"install_command,omitempty"`
}

var statusNames = map[otel.ProjectStatus]string{
	otel.Instrumentable:     "instrumentable",
	otel.ManualSteps:        "manual_steps",
	otel.RuntimeUnavailable: "runtime_unavailable",
}

// RenderJSON writes inv to w as a single indented JSON document.
func RenderJSON(w io.Writer, inv otel.ProjectInventory) error {
	doc := jsonInventory{
		Roots:    append([]string{}, inv.Roots...),
		Runtimes: make([]jsonRuntime, 0, len(inv.Runtimes)),
		Projects: make([]jsonProject, 0, len(inv.Projects)),
	}
	for _, r := range inv.Runtimes {
		doc.Runtimes = append(doc.Runtimes, jsonRuntime{ID: string(r.ID), Available: r.Available, Reason: r.Reason})
	}
	for _, p := range inv.Projects {
		project := jsonProject{
			Runtime: string(p.Runtime),
			Path:    p.Path,
			Markers: append([]string{}, p.Markers...),
			PIDs:    p.PIDs,
			Module:  p.Module,
			Status:  statusNames[p.Status],
		}
		if p.Status != otel.RuntimeUnavailable {
			project.InstallCommand = installCommand(p)
		}
		doc.Projects = append(doc.Projects, project)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(doc)
}

// installCommand returns the command that installs exactly project p.
func installCommand(p otel.InventoryProject) string {
	return "dtwiz install otel --project " + shellQuote(p.Path) + " --runtime " + string(p.Runtime)
}

// shellQuote wraps path in double quotes when it contains characters a shell
// would split or interpret. Not a full escaping scheme: "$" and "`" still
// expand inside double quotes in POSIX shells.
func shellQuote(path string) string {
	safe := func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return true
		case strings.ContainsRune("/._-:+@,=~", r):
			return true
		case r == '\\' && filepath.Separator == '\\':
			return true
		}
		return false
	}
	for _, r := range path {
		if !safe(r) {
			return `"` + strings.ReplaceAll(path, `"`, `\"`) + `"`
		}
	}
	return path
}
