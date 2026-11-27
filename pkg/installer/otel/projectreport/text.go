// Package projectreport renders a project inventory for humans or machines.
package projectreport

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel"
)

const installHint = "Instrument one:  dtwiz install otel --project <path> --runtime <runtime>"

// RenderText writes the human-readable listing of inv to w.
func RenderText(w io.Writer, inv otel.ProjectInventory) error {
	home, _ := os.UserHomeDir()
	reasons := unavailableReasons(inv)

	var out bytes.Buffer
	out.WriteString(display.ColorMessage.Sprint("  Detected projects") + "\n")
	out.WriteString(display.ColorMuted.Sprint("  "+strings.Repeat("─", display.DividerLineLength)) + "\n")

	roots := make([]string, len(inv.Roots))
	for i, r := range inv.Roots {
		roots[i] = displayPath(r, home)
	}
	scanned := "  Scanned: " + strings.Join(roots, ", ")
	if len(inv.Filter) > 0 {
		scanned += "   Runtimes: " + otel.FormatRuntimeIDs(inv.Filter)
	}
	out.WriteString(scanned + "\n\n")

	if len(inv.Projects) == 0 {
		out.WriteString("  No projects found.\n")
		_, err := w.Write(out.Bytes())
		return err
	}

	var table bytes.Buffer
	tw := tabwriter.NewWriter(&table, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  PATH\tRUNTIME\tMARKERS\tNOTES")
	for _, p := range inv.Projects {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n",
			displayPath(p.Path, home), p.Runtime, strings.Join(p.Markers, ", "), notes(p, reasons))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	// tabwriter pads every cell followed by a tab, leaving trailing blanks when NOTES is empty.
	for _, line := range strings.SplitAfter(table.String(), "\n") {
		if line == "" {
			continue
		}
		out.WriteString(strings.TrimRight(line, " \n") + "\n")
	}

	out.WriteString("\n  " + installHint + "\n")
	_, err := w.Write(out.Bytes())
	return err
}

// notes joins, in order: unavailability reason, manual-steps note, PIDs, module.
func notes(p otel.InventoryProject, reasons map[otel.RuntimeID]string) string {
	var parts []string
	switch p.Status {
	case otel.RuntimeUnavailable:
		if reason := reasons[p.Runtime]; reason != "" {
			parts = append(parts, reason)
		}
	case otel.ManualSteps:
		parts = append(parts, "manual steps after install")
	}
	if len(p.PIDs) > 0 {
		pids := make([]string, len(p.PIDs))
		for i, pid := range p.PIDs {
			pids[i] = strconv.Itoa(pid)
		}
		label := "PID"
		if len(p.PIDs) > 1 {
			label = "PIDs"
		}
		parts = append(parts, label+" "+strings.Join(pids, ", "))
	}
	if p.Module != "" {
		parts = append(parts, "module "+p.Module)
	}
	return strings.Join(parts, "; ")
}

func unavailableReasons(inv otel.ProjectInventory) map[otel.RuntimeID]string {
	reasons := make(map[otel.RuntimeID]string, len(inv.Runtimes))
	for _, r := range inv.Runtimes {
		if !r.Available {
			reasons[r.ID] = r.Reason
		}
	}
	return reasons
}

// displayPath replaces the home-directory prefix with "~".
func displayPath(path, home string) string {
	if home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if rel, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rel
	}
	return path
}
