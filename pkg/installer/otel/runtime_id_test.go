package otel

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
)

func TestActiveRuntimeIDs(t *testing.T) {
	tests := []struct {
		name   string
		dotNet bool
		want   []RuntimeID
	}{
		{"dotnet flag off", false, []RuntimeID{RuntimePython, RuntimeJava, RuntimeNodeJS, RuntimeGo}},
		{"dotnet flag on", true, []RuntimeID{RuntimePython, RuntimeJava, RuntimeNodeJS, RuntimeGo, RuntimeDotNet}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			featureflags.SetCLIOverrideForTest(t, featureflags.DotNetOtel, tt.dotNet)

			// Act
			got := ActiveRuntimeIDs()

			// Assert
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ActiveRuntimeIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRuntimeFilter(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		dotNet      bool
		want        []RuntimeID
		wantErrText string
	}{
		{name: "empty means no filter", value: "", want: nil},
		{name: "two runtimes", value: "java,go", want: []RuntimeID{RuntimeJava, RuntimeGo}},
		{name: "duplicates collapsed", value: "java,java", want: []RuntimeID{RuntimeJava}},
		{name: "wrong case rejected", value: "JAVA", wantErrText: `unknown runtime "JAVA" (valid: python, java, nodejs, go)`},
		{name: "alias rejected", value: "golang", wantErrText: `unknown runtime "golang" (valid: python, java, nodejs, go)`},
		{name: "empty element rejected", value: "java,", wantErrText: `unknown runtime "" (valid: python, java, nodejs, go)`},
		{name: "dotnet rejected while flag off", value: "dotnet", wantErrText: `unknown runtime "dotnet" (valid: python, java, nodejs, go)`},
		{name: "dotnet accepted while flag on", value: "dotnet", dotNet: true, want: []RuntimeID{RuntimeDotNet}},
		{name: "valid list includes dotnet while flag on", value: "ruby", dotNet: true, wantErrText: `unknown runtime "ruby" (valid: python, java, nodejs, go, dotnet)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			featureflags.SetCLIOverrideForTest(t, featureflags.DotNetOtel, tt.dotNet)

			// Act
			got, err := ParseRuntimeFilter(tt.value)

			// Assert
			if tt.wantErrText != "" {
				var unknown *UnknownRuntimeError
				if !errors.As(err, &unknown) || err.Error() != tt.wantErrText {
					t.Fatalf("ParseRuntimeFilter(%q) error = %v, want %q", tt.value, err, tt.wantErrText)
				}
				if strings.Contains(err.Error(), "dotnet-otel") || strings.Contains(err.Error(), "DTWIZ_DOTNET_OTEL") {
					t.Errorf("error must not mention the feature flag: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRuntimeFilter(%q) unexpected error: %v", tt.value, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRuntimeFilter(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseRuntime(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		want        RuntimeID
		wantErr     error
		wantErrText string
	}{
		{name: "single value", value: "python", want: RuntimePython},
		{name: "list rejected", value: "java,go", wantErr: ErrRuntimeSingleValue},
		{name: "wrong case rejected", value: "Python", wantErrText: `unknown runtime "Python" (valid: python, java, nodejs, go)`},
		{name: "empty rejected", value: "", wantErrText: `unknown runtime "" (valid: python, java, nodejs, go)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			featureflags.SetCLIOverrideForTest(t, featureflags.DotNetOtel, false)

			// Act
			got, err := ParseRuntime(tt.value)

			// Assert
			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ParseRuntime(%q) error = %v, want %v", tt.value, err, tt.wantErr)
				}
			case tt.wantErrText != "":
				if err == nil || err.Error() != tt.wantErrText {
					t.Errorf("ParseRuntime(%q) error = %v, want %q", tt.value, err, tt.wantErrText)
				}
			default:
				if err != nil || got != tt.want {
					t.Errorf("ParseRuntime(%q) = %q, %v; want %q, nil", tt.value, got, err, tt.want)
				}
			}
		})
	}
}
