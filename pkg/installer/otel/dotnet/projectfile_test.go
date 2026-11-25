package dotnet_test

import (
	"reflect"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

func TestParseProjectFile(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		wantService     bool
		wantFrameworks  []string
		wantAssembly    string
		wantFramework   bool
		wantSelfContain bool
		wantTest        bool
		wantLegacy      bool
	}{
		{name: "web", content: sdkProject("Microsoft.NET.Sdk.Web", "<TargetFramework>net8.0</TargetFramework>"), wantService: true, wantFrameworks: []string{"net8.0"}},
		{name: "console", content: sdkProject("Microsoft.NET.Sdk", "<OutputType>Exe</OutputType><TargetFramework>net8.0</TargetFramework><AssemblyName>Tool</AssemblyName>"), wantService: true, wantFrameworks: []string{"net8.0"}, wantAssembly: "Tool"},
		{name: "worker", content: sdkProject("Microsoft.NET.Sdk.Worker", "<TargetFramework>net9.0</TargetFramework>"), wantService: true, wantFrameworks: []string{"net9.0"}},
		{name: "library", content: sdkProject("Microsoft.NET.Sdk", "<TargetFramework>net8.0</TargetFramework>"), wantFrameworks: []string{"net8.0"}},
		{name: "test project by property", content: sdkProject("Microsoft.NET.Sdk", "<OutputType>Exe</OutputType><IsTestProject>true</IsTestProject><TargetFramework>net8.0</TargetFramework>"), wantTest: true, wantFrameworks: []string{"net8.0"}},
		{name: "test project by package", content: `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup><ItemGroup><PackageReference Include="Microsoft.NET.Test.Sdk" Version="17"/></ItemGroup></Project>`, wantTest: true, wantFrameworks: []string{"net8.0"}},
		{name: "multi-target", content: sdkProject("Microsoft.NET.Sdk.Web", "<TargetFrameworks>net8.0; net9.0</TargetFrameworks>"), wantService: true, wantFrameworks: []string{"net8.0", "net9.0"}},
		{name: ".NET Framework", content: sdkProject("Microsoft.NET.Sdk", "<OutputType>Exe</OutputType><TargetFramework>net48</TargetFramework>"), wantService: true, wantFrameworks: []string{"net48"}, wantFramework: true},
		{name: "mixed targets are not .NET Framework", content: sdkProject("Microsoft.NET.Sdk", "<OutputType>Exe</OutputType><TargetFrameworks>net8.0;net48</TargetFrameworks>"), wantService: true, wantFrameworks: []string{"net8.0", "net48"}},
		{name: "self-contained", content: sdkProject("Microsoft.NET.Sdk.Web", "<TargetFramework>net8.0</TargetFramework><SelfContained>true</SelfContained>"), wantService: true, wantFrameworks: []string{"net8.0"}, wantSelfContain: true},
		{name: "framework missing is unknown", content: sdkProject("Microsoft.NET.Sdk", "<OutputType>Exe</OutputType>"), wantService: true},
		{name: "conditional property ignored", content: `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup Condition="'$(Configuration)'=='Debug'"><OutputType>Exe</OutputType></PropertyGroup></Project>`},
		{name: "legacy non-SDK file", content: `<Project ToolsVersion="15.0" xmlns="http://schemas.microsoft.com/developer/msbuild/2003"><PropertyGroup><OutputType>Exe</OutputType><TargetFrameworkVersion>v4.8</TargetFrameworkVersion></PropertyGroup></Project>`, wantService: true, wantFrameworks: []string{"net48"}, wantFramework: true, wantLegacy: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			path := writeFile(t, t.TempDir(), "App.csproj", tt.content)

			// Act
			got, err := dotnet.ParseProjectFile(path)

			// Assert
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.IsService != tt.wantService || got.IsTest != tt.wantTest || got.LegacyFormat != tt.wantLegacy ||
				got.DotNetFramework != tt.wantFramework || got.SelfContained != tt.wantSelfContain ||
				got.AssemblyName != tt.wantAssembly || !reflect.DeepEqual(got.Frameworks, tt.wantFrameworks) {
				t.Errorf("got %+v", got)
			}
			if got.Name != "App" {
				t.Errorf("Name = %q, want App", got.Name)
			}
		})
	}
}

func TestParseProjectFile_MalformedXML(t *testing.T) {
	// Arrange
	path := writeFile(t, t.TempDir(), "Bad.csproj", "<Project><PropertyGroup>")

	// Act
	_, err := dotnet.ParseProjectFile(path)

	// Assert
	if err == nil {
		t.Error("expected an error for malformed XML")
	}
}
