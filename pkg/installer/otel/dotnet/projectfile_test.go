package dotnet_test

import (
	"fmt"
	"path/filepath"
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

const propsFile = `<Project><PropertyGroup>%s</PropertyGroup></Project>`

func TestParseProjectFile_SharedBuildProperties(t *testing.T) {
	const exeProject = `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><OutputType>Exe</OutputType>%s</PropertyGroup></Project>`
	tests := []struct {
		name            string
		files           map[string]string // relative to the temp root; the project is src/App/App.csproj
		wantFrameworks  []string
		wantSelfContain bool
		wantFramework   bool
	}{
		{name: "framework inherited from the nearest props",
			files: map[string]string{
				"Directory.Build.props": fmt.Sprintf(propsFile, "<TargetFramework>net8.0</TargetFramework>"),
				"src/App/App.csproj":    fmt.Sprintf(exeProject, ""),
			},
			wantFrameworks: []string{"net8.0"}},
		{name: "project value wins over the shared value",
			files: map[string]string{
				"Directory.Build.props": fmt.Sprintf(propsFile, "<TargetFramework>net8.0</TargetFramework>"),
				"src/App/App.csproj":    fmt.Sprintf(exeProject, "<TargetFramework>net9.0</TargetFramework>"),
			},
			wantFrameworks: []string{"net9.0"}},
		{name: "nearest props wins over a farther one without import",
			files: map[string]string{
				"Directory.Build.props":     fmt.Sprintf(propsFile, "<TargetFramework>net6.0</TargetFramework>"),
				"src/Directory.Build.props": fmt.Sprintf(propsFile, "<TargetFramework>net8.0</TargetFramework>"),
				"src/App/App.csproj":        fmt.Sprintf(exeProject, ""),
			},
			wantFrameworks: []string{"net8.0"}},
		{name: "chained to the parent props through an import",
			files: map[string]string{
				"Directory.Build.props": fmt.Sprintf(propsFile, "<TargetFramework>net8.0</TargetFramework>"),
				"src/Directory.Build.props": `<Project><Import Project="$([MSBuild]::GetPathOfFileAbove('Directory.Build.props', '$(MSBuildThisFileDirectory)../'))" />` +
					`<PropertyGroup><Nullable>enable</Nullable></PropertyGroup></Project>`,
				"src/App/App.csproj": fmt.Sprintf(exeProject, ""),
			},
			wantFrameworks: []string{"net8.0"}},
		{name: "nearer props overrides the chained parent",
			files: map[string]string{
				"Directory.Build.props": fmt.Sprintf(propsFile, "<TargetFramework>net6.0</TargetFramework>"),
				"src/Directory.Build.props": `<Project><Import Project="$([MSBuild]::GetPathOfFileAbove('Directory.Build.props', '$(MSBuildThisFileDirectory)../'))" />` +
					`<PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>`,
				"src/App/App.csproj": fmt.Sprintf(exeProject, ""),
			},
			wantFrameworks: []string{"net8.0"}},
		{name: "shared self-contained",
			files: map[string]string{
				"Directory.Build.props": fmt.Sprintf(propsFile, "<TargetFramework>net8.0</TargetFramework><SelfContained>true</SelfContained>"),
				"src/App/App.csproj":    fmt.Sprintf(exeProject, ""),
			},
			wantFrameworks: []string{"net8.0"}, wantSelfContain: true},
		{name: "shared .NET Framework",
			files: map[string]string{
				"Directory.Build.props": fmt.Sprintf(propsFile, "<TargetFramework>net48</TargetFramework>"),
				"src/App/App.csproj":    fmt.Sprintf(exeProject, ""),
			},
			wantFrameworks: []string{"net48"}, wantFramework: true},
		{name: "property reference stays unknown",
			files: map[string]string{
				"Directory.Build.props": fmt.Sprintf(propsFile, "<TargetFramework>$(DefaultTfm)</TargetFramework>"),
				"src/App/App.csproj":    fmt.Sprintf(exeProject, ""),
			}},
		{name: "conditional shared property ignored",
			files: map[string]string{
				"Directory.Build.props": `<Project><PropertyGroup Condition="'$(CI)'=='true'"><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>`,
				"src/App/App.csproj":    fmt.Sprintf(exeProject, ""),
			}},
		{name: "malformed props is ignored",
			files: map[string]string{
				"Directory.Build.props": "<Project>",
				"src/App/App.csproj":    fmt.Sprintf(exeProject, "<TargetFramework>net8.0</TargetFramework>"),
			},
			wantFrameworks: []string{"net8.0"}},
		{name: "no shared file",
			files: map[string]string{"src/App/App.csproj": fmt.Sprintf(exeProject, "")}},
		{name: "props below the project is not applied",
			files: map[string]string{
				"src/App/App.csproj":                fmt.Sprintf(exeProject, ""),
				"src/App/sub/Directory.Build.props": fmt.Sprintf(propsFile, "<TargetFramework>net8.0</TargetFramework>"),
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			for rel, content := range tt.files {
				writeFile(t, root, rel, content)
			}

			// Act
			got, err := dotnet.ParseProjectFile(filepath.Join(root, "src", "App", "App.csproj"))

			// Assert
			if err != nil {
				t.Fatalf("ParseProjectFile() error = %v", err)
			}
			if !reflect.DeepEqual(got.Frameworks, tt.wantFrameworks) {
				t.Errorf("Frameworks = %v, want %v", got.Frameworks, tt.wantFrameworks)
			}
			if got.SelfContained != tt.wantSelfContain || got.DotNetFramework != tt.wantFramework {
				t.Errorf("SelfContained=%v DotNetFramework=%v, want %v %v", got.SelfContained, got.DotNetFramework, tt.wantSelfContain, tt.wantFramework)
			}
		})
	}
}

func TestParseProjectFile_SharedOutputTypeMakesServices(t *testing.T) {
	// Arrange
	root := t.TempDir()
	writeFile(t, root, "Directory.Build.props", fmt.Sprintf(propsFile, "<OutputType>Exe</OutputType><TargetFramework>net8.0</TargetFramework>"))
	path := writeFile(t, root, "App/App.csproj", `<Project Sdk="Microsoft.NET.Sdk"/>`)

	// Act
	got, err := dotnet.ParseProjectFile(path)

	// Assert
	if err != nil || !got.IsService {
		t.Errorf("IsService = %v, err = %v; want a service from the shared OutputType", got.IsService, err)
	}
}
