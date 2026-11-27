package otel

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
)

func TestResolveProjectRuntime(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		noProject   bool
		runtimeFlag string
		dotNet      bool
		want        RuntimeID
		wantErr     error
		wantErrText string
	}{
		{name: "no runtime flag", files: map[string]string{"pom.xml": ""}, runtimeFlag: "", want: ""},
		{name: "runtime without project", noProject: true, runtimeFlag: "java", wantErr: ErrRuntimeRequiresProject},
		{name: "wrong case", files: map[string]string{"requirements.txt": ""}, runtimeFlag: "Python", wantErrText: `unknown runtime "Python" (valid: python, java, nodejs, go)`},
		{name: "more than one value", files: map[string]string{"pom.xml": ""}, runtimeFlag: "java,go", wantErr: ErrRuntimeSingleValue},
		{name: "dotnet while flag off", files: map[string]string{"App.csproj": ""}, runtimeFlag: "dotnet", wantErrText: `unknown runtime "dotnet" (valid: python, java, nodejs, go)`},
		{name: "two runtimes resolved to python", files: map[string]string{"package.json": "{}", "requirements.txt": ""}, runtimeFlag: "python", want: RuntimePython},
		{name: "dotnet overrides node precedence", files: map[string]string{"package.json": "{}", "App.csproj": ""}, runtimeFlag: "dotnet", dotNet: true, want: RuntimeDotNet},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			featureflags.SetCLIOverrideForTest(t, featureflags.DotNetOtel, tt.dotNet)
			project := ""
			if !tt.noProject {
				project = t.TempDir()
				writeTree(t, project, tt.files)
			}

			// Act
			got, err := ResolveProjectRuntime(project, tt.runtimeFlag)

			// Assert
			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
			case tt.wantErrText != "":
				if err == nil || err.Error() != tt.wantErrText {
					t.Errorf("error = %v, want %q", err, tt.wantErrText)
				}
			default:
				if err != nil || got != tt.want {
					t.Errorf("ResolveProjectRuntime() = %q, %v; want %q, nil", got, err, tt.want)
				}
			}
		})
	}
}

func TestResolveProjectRuntime_NoMarkersForRuntime(t *testing.T) {
	// Arrange
	web := filepath.Join(t.TempDir(), "web")
	writeTree(t, web, map[string]string{"package.json": "{}"})

	// Act
	_, err := ResolveProjectRuntime(web, "python")

	// Assert
	if want := "no python project markers in " + web; err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestResolveProjectRuntime_MissingProject(t *testing.T) {
	// Arrange
	missing := filepath.Join(t.TempDir(), "nope")

	// Act
	_, err := ResolveProjectRuntime(missing, "python")

	// Assert
	if want := "project path not found: " + missing; err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}
