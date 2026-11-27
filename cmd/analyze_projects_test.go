package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel"
	"github.com/dynatrace-oss/dtwiz/pkg/selfmonitoring"
)

// setAnalyzeProjectsForTest stubs detection, sets the flag variables and
// captures command output; everything is restored on cleanup.
func setAnalyzeProjectsForTest(t *testing.T, runtime string, jsonOut bool, inv otel.ProjectInventory) (*bytes.Buffer, *otel.ProjectQuery) {
	t.Helper()
	origDetect, origRuntime, origDir, origJSON := detectProjectsFn, analyzeProjectsRuntime, analyzeProjectsDir, analyzeJSON
	origSilence := rootCmd.SilenceUsage
	t.Cleanup(func() {
		detectProjectsFn, analyzeProjectsRuntime, analyzeProjectsDir, analyzeJSON = origDetect, origRuntime, origDir, origJSON
		rootCmd.SilenceUsage = origSilence
		analyzeProjectsCmd.SetOut(nil)
	})
	var got otel.ProjectQuery
	detectProjectsFn = func(q otel.ProjectQuery) (otel.ProjectInventory, error) {
		got = q
		return inv, nil
	}
	analyzeProjectsRuntime, analyzeProjectsDir, analyzeJSON = runtime, "", jsonOut
	var out bytes.Buffer
	analyzeProjectsCmd.SetOut(&out)
	return &out, &got
}

func TestAnalyzeCmd_BareAndSubcommandResolveSeparately(t *testing.T) {
	// Act
	bare, _, bareErr := rootCmd.Find([]string{"analyze"})
	sub, _, subErr := rootCmd.Find([]string{"analyze", "projects"})

	// Assert
	if bareErr != nil || bare != analyzeCmd || analyzeCmd.RunE == nil {
		t.Errorf("bare analyze must resolve to analyzeCmd with its own handler, got %v (%v)", bare, bareErr)
	}
	if subErr != nil || sub != analyzeProjectsCmd {
		t.Errorf("analyze projects must resolve to analyzeProjectsCmd, got %v (%v)", sub, subErr)
	}
}

func TestAnalyzeProjectsCmd_JSONOutput(t *testing.T) {
	// Arrange
	captureEvents(t)
	inv := otel.ProjectInventory{
		Roots:    []string{"/work"},
		Runtimes: []otel.RuntimeStatus{{ID: otel.RuntimeGo, Available: true}},
		Projects: []otel.InventoryProject{{Runtime: otel.RuntimeGo, Path: "/work/gw", Markers: []string{"go.mod"}}},
	}
	out, query := setAnalyzeProjectsForTest(t, "go", true, inv)

	// Act
	err := analyzeProjectsCmd.RunE(analyzeProjectsCmd, nil)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not a JSON document: %v\n%s", err, out.String())
	}
	if len(query.Runtimes) != 1 || query.Runtimes[0] != otel.RuntimeGo {
		t.Errorf("query runtimes = %v, want [go]", query.Runtimes)
	}
}

func TestAnalyzeProjectsCmd_TextOutput(t *testing.T) {
	// Arrange
	captureEvents(t)
	out, _ := setAnalyzeProjectsForTest(t, "", false, otel.ProjectInventory{Roots: []string{"/work"}})

	// Act
	err := analyzeProjectsCmd.RunE(analyzeProjectsCmd, nil)

	// Assert
	if err != nil || !strings.Contains(out.String(), "No projects found.") {
		t.Errorf("err = %v, output = %q", err, out.String())
	}
}

func TestAnalyzeProjectsCmd_UnknownRuntime(t *testing.T) {
	// Arrange
	captureEvents(t)
	featureflags.SetCLIOverrideForTest(t, featureflags.DotNetOtel, false)
	setAnalyzeProjectsForTest(t, "dotnet", false, otel.ProjectInventory{})

	// Act
	err := analyzeProjectsCmd.RunE(analyzeProjectsCmd, nil)

	// Assert
	var unknown *otel.UnknownRuntimeError
	if !errors.As(err, &unknown) || err.Error() != `unknown runtime "dotnet" (valid: python, java, nodejs, go)` {
		t.Errorf("error = %v, want unknown runtime", err)
	}
}

func TestAnalyzeProjectsCmd_FiresCompletedEvent(t *testing.T) {
	// Arrange
	captured := captureEvents(t)
	setAnalyzeProjectsForTest(t, "", true, otel.ProjectInventory{})

	// Act
	err := analyzeProjectsCmd.RunE(analyzeProjectsCmd, nil)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*captured) != 1 {
		t.Fatalf("expected one event, got %v", *captured)
	}
	ev := (*captured)[0]
	if ev.Cmd != "analyze" || ev.Sub != "projects" || ev.StepID != selfmonitoring.StepCompleted {
		t.Errorf("event = %+v, want analyze/projects completed", ev)
	}
}

func TestAnalyzeProjectsCmd_HelpListsDotNetOnlyWithFlag(t *testing.T) {
	tests := []struct {
		name       string
		dotNet     bool
		wantDotNet bool
	}{
		{"flag off", false, false},
		{"flag on", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			captureEvents(t)
			featureflags.SetCLIOverrideForTest(t, featureflags.DotNetOtel, tt.dotNet)
			var buf bytes.Buffer
			analyzeProjectsCmd.SetOut(&buf)
			t.Cleanup(func() { analyzeProjectsCmd.SetOut(nil) })

			// Act
			analyzeProjectsCmd.HelpFunc()(analyzeProjectsCmd, nil)

			// Assert
			line := ""
			for _, l := range strings.Split(buf.String(), "\n") {
				if strings.Contains(l, "--runtime string") {
					line = l
				}
			}
			if !strings.Contains(line, "python, java, nodejs, go") {
				t.Fatalf("--runtime usage missing identifiers, help:\n%s", buf.String())
			}
			if strings.Contains(line, "dotnet") != tt.wantDotNet {
				t.Errorf("--runtime usage = %q, want dotnet listed = %v", line, tt.wantDotNet)
			}
		})
	}
}
