package projectreport_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/fatih/color"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/projectreport"
)

const divider = "  ──────────────────────────────────────────────────\n"

// fakeHome points os.UserHomeDir at a fixed directory and disables colors.
func fakeHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(string(filepath.Separator)+"home", "me")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	old := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = old })
	return home
}

func renderText(t *testing.T, inv otel.ProjectInventory) string {
	t.Helper()
	var buf bytes.Buffer
	if err := projectreport.RenderText(&buf, inv); err != nil {
		t.Fatalf("RenderText() error = %v", err)
	}
	return buf.String()
}

func TestRenderText_DefaultListing(t *testing.T) {
	// Arrange
	home := fakeHome(t)
	shop := filepath.Join(home, "work", "shop")
	inv := otel.ProjectInventory{
		Roots: []string{shop},
		Runtimes: []otel.RuntimeStatus{
			{ID: otel.RuntimePython, Available: false, Reason: "python3 not found on PATH"},
			{ID: otel.RuntimeJava, Available: true},
			{ID: otel.RuntimeNodeJS, Available: true},
			{ID: otel.RuntimeGo, Available: true},
			{ID: otel.RuntimeDotNet, Available: true},
		},
		Projects: []otel.InventoryProject{
			{Runtime: otel.RuntimeNodeJS, Path: filepath.Join(shop, "api"), Markers: []string{"package.json"}, PIDs: []int{4121, 4122}},
			{Runtime: otel.RuntimePython, Path: filepath.Join(shop, "api"), Markers: []string{"requirements.txt", "pyproject.toml"}, Status: otel.RuntimeUnavailable},
			{Runtime: otel.RuntimeJava, Path: filepath.Join(shop, "billing"), Markers: []string{"pom.xml"}, PIDs: []int{5310}},
			{Runtime: otel.RuntimeGo, Path: filepath.Join(shop, "gateway"), Markers: []string{"go.mod"}, Module: "github.com/acme/gateway"},
			{Runtime: otel.RuntimeDotNet, Path: filepath.Join(shop, "legacy"), Markers: []string{"Legacy.csproj"}, Status: otel.ManualSteps},
			{Runtime: otel.RuntimeNodeJS, Path: filepath.Join(shop, "web"), Markers: []string{"package.json"}},
		},
	}
	p := func(rel string) string { return filepath.Join("~", "work", "shop", rel) }
	row := func(path, runtime, markers, notes string) string {
		return fmt.Sprintf("  %-21s%-9s%-34s%s", path, runtime, markers, notes) + "\n"
	}
	want := "  Detected projects\n" + divider +
		"  Scanned: " + filepath.Join("~", "work", "shop") + "\n\n" +
		row("PATH", "RUNTIME", "MARKERS", "NOTES") +
		row(p("api"), "nodejs", "package.json", "PIDs 4121, 4122") +
		row(p("api"), "python", "requirements.txt, pyproject.toml", "python3 not found on PATH") +
		row(p("billing"), "java", "pom.xml", "PID 5310") +
		row(p("gateway"), "go", "go.mod", "module github.com/acme/gateway") +
		row(p("legacy"), "dotnet", "Legacy.csproj", "manual steps after install") +
		"  " + fmt.Sprintf("%-21s%-9s", p("web"), "nodejs") + "package.json\n" +
		"\n  Instrument one:  dtwiz install otel --project <path> --runtime <runtime>\n"

	// Act
	got := renderText(t, inv)

	// Assert
	if got != want {
		t.Errorf("RenderText() mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderText_FilteredListing(t *testing.T) {
	// Arrange
	home := fakeHome(t)
	shop := filepath.Join(home, "work", "shop")
	inv := otel.ProjectInventory{
		Roots:    []string{shop},
		Filter:   []otel.RuntimeID{otel.RuntimeJava, otel.RuntimeGo},
		Runtimes: []otel.RuntimeStatus{{ID: otel.RuntimeJava, Available: true}, {ID: otel.RuntimeGo, Available: true}},
		Projects: []otel.InventoryProject{
			{Runtime: otel.RuntimeJava, Path: filepath.Join(shop, "billing"), Markers: []string{"pom.xml"}},
		},
	}
	want := "  Detected projects\n" + divider +
		"  Scanned: " + filepath.Join("~", "work", "shop") + "   Runtimes: java, go\n\n" +
		fmt.Sprintf("  %-21s%-9s%-9s%s\n", "PATH", "RUNTIME", "MARKERS", "NOTES") +
		fmt.Sprintf("  %-21s%-9s%s\n", filepath.Join("~", "work", "shop", "billing"), "java", "pom.xml") +
		"\n  Instrument one:  dtwiz install otel --project <path> --runtime <runtime>\n"

	// Act
	got := renderText(t, inv)

	// Assert
	if got != want {
		t.Errorf("RenderText() mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderText_PathOutsideHomeIsUnchanged(t *testing.T) {
	// Arrange
	fakeHome(t)
	root := filepath.Join(string(filepath.Separator)+"opt", "app")
	inv := otel.ProjectInventory{Roots: []string{root}, Projects: []otel.InventoryProject{}}
	want := "  Detected projects\n" + divider + "  Scanned: " + root + "\n\n  No projects found.\n"

	// Act
	got := renderText(t, inv)

	// Assert
	if got != want {
		t.Errorf("RenderText() mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}
