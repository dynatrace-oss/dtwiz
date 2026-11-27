package projectreport_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/projectreport"
)

type decodedInventory struct {
	Roots    []string         `json:"roots"`
	Runtimes []map[string]any `json:"runtimes"`
	Projects []map[string]any `json:"projects"`
}

func renderJSON(t *testing.T, inv otel.ProjectInventory) (string, decodedInventory) {
	t.Helper()
	var buf bytes.Buffer
	if err := projectreport.RenderJSON(&buf, inv); err != nil {
		t.Fatalf("RenderJSON() error = %v", err)
	}
	var doc decodedInventory
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, buf.String())
	}
	if dec.More() {
		t.Fatalf("output contains more than one JSON value:\n%s", buf.String())
	}
	return buf.String(), doc
}

func TestRenderJSON_Document(t *testing.T) {
	// Arrange
	inv := otel.ProjectInventory{
		Roots: []string{"/Users/me/work/shop"},
		Runtimes: []otel.RuntimeStatus{
			{ID: otel.RuntimePython, Available: false, Reason: "python3 not found on PATH"},
			{ID: otel.RuntimeNodeJS, Available: true},
			{ID: otel.RuntimeGo, Available: true},
			{ID: otel.RuntimeDotNet, Available: true},
		},
		Projects: []otel.InventoryProject{
			{Runtime: otel.RuntimeNodeJS, Path: "/Users/me/work/shop/api", Markers: []string{"package.json"}, PIDs: []int{4121, 4122}},
			{Runtime: otel.RuntimePython, Path: "/Users/me/work/shop/api", Markers: []string{"requirements.txt"}, Status: otel.RuntimeUnavailable},
			{Runtime: otel.RuntimeGo, Path: "/Users/me/work/shop/gateway", Markers: []string{"go.mod"}, Module: "github.com/acme/gateway"},
			{Runtime: otel.RuntimeDotNet, Path: "/Users/me/work/shop/legacy", Markers: []string{"Legacy.csproj"}, Status: otel.ManualSteps},
		},
	}
	wantProjects := []map[string]any{
		{
			"runtime": "nodejs", "path": "/Users/me/work/shop/api", "markers": []any{"package.json"},
			"pids": []any{4121.0, 4122.0}, "status": "instrumentable",
			"install_command": "dtwiz install otel --project /Users/me/work/shop/api --runtime nodejs",
		},
		{
			"runtime": "python", "path": "/Users/me/work/shop/api", "markers": []any{"requirements.txt"},
			"status": "runtime_unavailable",
		},
		{
			"runtime": "go", "path": "/Users/me/work/shop/gateway", "markers": []any{"go.mod"},
			"module": "github.com/acme/gateway", "status": "instrumentable",
			"install_command": "dtwiz install otel --project /Users/me/work/shop/gateway --runtime go",
		},
		{
			"runtime": "dotnet", "path": "/Users/me/work/shop/legacy", "markers": []any{"Legacy.csproj"},
			"status":          "manual_steps",
			"install_command": "dtwiz install otel --project /Users/me/work/shop/legacy --runtime dotnet",
		},
	}
	wantRuntimes := []map[string]any{
		{"id": "python", "available": false, "reason": "python3 not found on PATH"},
		{"id": "nodejs", "available": true},
		{"id": "go", "available": true},
		{"id": "dotnet", "available": true},
	}

	// Act
	_, doc := renderJSON(t, inv)

	// Assert
	if !reflect.DeepEqual(doc.Roots, []string{"/Users/me/work/shop"}) {
		t.Errorf("roots = %v", doc.Roots)
	}
	if !reflect.DeepEqual(doc.Runtimes, wantRuntimes) {
		t.Errorf("runtimes =\n%v\nwant\n%v", doc.Runtimes, wantRuntimes)
	}
	if !reflect.DeepEqual(doc.Projects, wantProjects) {
		t.Errorf("projects =\n%v\nwant\n%v", doc.Projects, wantProjects)
	}
}

func TestRenderJSON_QuotesPathWithSpaces(t *testing.T) {
	// Arrange
	inv := otel.ProjectInventory{
		Roots:    []string{"/Users/me/My Apps"},
		Projects: []otel.InventoryProject{{Runtime: otel.RuntimeNodeJS, Path: "/Users/me/My Apps/web", Markers: []string{"package.json"}}},
	}

	// Act
	_, doc := renderJSON(t, inv)

	// Assert
	want := `dtwiz install otel --project "/Users/me/My Apps/web" --runtime nodejs`
	if got := doc.Projects[0]["install_command"]; got != want {
		t.Errorf("install_command = %v, want %s", got, want)
	}
}

func TestRenderJSON_EmptyResult(t *testing.T) {
	// Arrange
	inv := otel.ProjectInventory{Roots: []string{"/tmp/empty"}}

	// Act
	raw, doc := renderJSON(t, inv)

	// Assert
	if !strings.Contains(raw, `"projects": []`) || !strings.Contains(raw, `"runtimes": []`) {
		t.Errorf("expected empty arrays, not null:\n%s", raw)
	}
	if doc.Projects == nil || len(doc.Projects) != 0 {
		t.Errorf("projects = %#v, want empty array", doc.Projects)
	}
}

// splitCommand splits a rendered command line into arguments, honoring the
// double quotes RenderJSON puts around paths with spaces.
func splitCommand(line string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false
	for _, r := range line {
		switch {
		case r == '"':
			inQuotes = !inQuotes
		case r == ' ' && !inQuotes:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

func flagValue(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestRenderJSON_InstallCommandsRoundTripThroughInstallFlags(t *testing.T) {
	// Arrange
	root := t.TempDir()
	api := filepath.Join(root, "My Apps", "api")
	if err := os.MkdirAll(api, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"package.json", "requirements.txt"} {
		if err := os.WriteFile(filepath.Join(api, f), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	inv := otel.ProjectInventory{
		Roots: []string{root},
		Projects: []otel.InventoryProject{
			{Runtime: otel.RuntimeNodeJS, Path: api, Markers: []string{"package.json"}},
			{Runtime: otel.RuntimePython, Path: api, Markers: []string{"requirements.txt"}},
		},
	}

	// Act
	_, doc := renderJSON(t, inv)

	// Assert
	for _, p := range doc.Projects {
		command, _ := p["install_command"].(string)
		args := splitCommand(command)
		got, err := otel.ResolveProjectRuntime(flagValue(args, "--project"), flagValue(args, "--runtime"))
		if err != nil {
			t.Errorf("install_command %q rejected: %v", command, err)
			continue
		}
		if string(got) != p["runtime"] || flagValue(args, "--project") != p["path"] {
			t.Errorf("install_command %q selects %s at %s, want %v at %v", command, got, flagValue(args, "--project"), p["runtime"], p["path"])
		}
	}
}
