package dotnet_test

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

const (
	webProj  = "Microsoft.NET.Sdk.Web"
	libProj  = `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>`
	testProj = `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><OutputType>Exe</OutputType><IsTestProject>true</IsTestProject></PropertyGroup></Project>`
)

func serviceNames(services []dotnet.Service) []string {
	names := make([]string, 0, len(services))
	for _, s := range services {
		names = append(names, s.Name)
	}
	sort.Strings(names)
	return names
}

func TestDiscoverServices_SolutionWithLibraryTestAndTwoServices(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, "App.sln", slnHeader+
		`Project("{F}") = "Api", "src\Api\Api.csproj", "{1}"`+"\nEndProject\n"+
		`Project("{F}") = "Worker", "src\Worker\Worker.csproj", "{2}"`+"\nEndProject\n"+
		`Project("{F}") = "Lib", "src\Lib\Lib.csproj", "{3}"`+"\nEndProject\n"+
		`Project("{F}") = "Tests", "tests\Tests\Tests.csproj", "{4}"`+"\nEndProject\n"+
		`Project("{F}") = "Out", "..\Out\Out.csproj", "{5}"`+"\nEndProject\n")
	writeFile(t, dir, "src/Api/Api.csproj", sdkProject(webProj, "<TargetFramework>net8.0</TargetFramework>"))
	writeFile(t, dir, "src/Worker/Worker.csproj", sdkProject("Microsoft.NET.Sdk.Worker", "<TargetFramework>net8.0</TargetFramework>"))
	writeFile(t, dir, "src/Lib/Lib.csproj", libProj)
	writeFile(t, dir, "tests/Tests/Tests.csproj", testProj)

	// Act
	got := dotnet.DiscoverServices(dir, []string{"App.sln"})

	// Assert
	if want := []string{"Api", "Worker"}; !reflect.DeepEqual(serviceNames(got), want) {
		t.Errorf("services = %v, want %v", serviceNames(got), want)
	}
}

func TestDiscoverServices_UnreadableSolutionFallsBackToSearch(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, "App.sln", "garbage")
	writeFile(t, dir, "Api/Api.csproj", sdkProject(webProj, ""))
	writeFile(t, dir, "Api/bin/Debug/Copy.csproj", sdkProject(webProj, ""))
	writeFile(t, dir, "a/b/c/d/e/f/TooDeep.csproj", sdkProject(webProj, ""))

	// Act
	got := dotnet.DiscoverServices(dir, []string{"App.sln"})

	// Assert
	if want := []string{"Api"}; !reflect.DeepEqual(serviceNames(got), want) {
		t.Errorf("services = %v, want %v", serviceNames(got), want)
	}
}

func TestDiscoverServices_NameCollisionUsesRelativePath(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, "one/App.csproj", sdkProject(webProj, "<AssemblyName>Shared</AssemblyName>"))
	writeFile(t, dir, "two/App.csproj", sdkProject(webProj, "<AssemblyName>Shared</AssemblyName>"))

	// Act
	got := dotnet.DiscoverServices(dir, nil)

	// Assert
	if want := []string{"one", "two"}; !reflect.DeepEqual(serviceNames(got), want) {
		t.Errorf("services = %v, want %v", serviceNames(got), want)
	}
}

func TestDiscoverServices_LibraryOnlyProjectHasNoService(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, "Lib.csproj", libProj)

	// Act
	got := dotnet.DiscoverServices(dir, []string{"Lib.csproj"})

	// Assert
	if len(got) != 0 {
		t.Errorf("services = %v, want none", got)
	}
}

func TestDiscoverServices_ServiceDetails(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, "Api.csproj", sdkProject(webProj, "<TargetFrameworks>net8.0;net9.0</TargetFrameworks>"))

	// Act
	got := dotnet.DiscoverServices(dir, []string{"Api.csproj"})

	// Assert
	if len(got) != 1 || got[0].Name != "Api" || got[0].Dir != dir || got[0].ProjectFile != filepath.Join(dir, "Api.csproj") ||
		got[0].FrameworkLabel() != "net8.0, net9.0" || got[0].UnsupportedReason() != "" {
		t.Errorf("unexpected service %+v", got)
	}
}

func TestService_UnsupportedReason(t *testing.T) {
	tests := []struct {
		name string
		svc  dotnet.Service
		want string
	}{
		{"supported", dotnet.Service{}, ""},
		{".NET Framework", dotnet.Service{DotNetFramework: true}, ".NET Framework, not supported yet"},
		{"self-contained", dotnet.Service{SelfContained: true}, "self-contained, not supported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := tt.svc.UnsupportedReason()

			// Assert
			if got != tt.want {
				t.Errorf("UnsupportedReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestService_FrameworkLabelUnknown(t *testing.T) {
	// Act
	got := dotnet.Service{}.FrameworkLabel()

	// Assert
	if got != "unknown" {
		t.Errorf("FrameworkLabel() = %q, want unknown", got)
	}
}

func TestDiscoverServices_FrameworkFromDirectoryBuildProps(t *testing.T) {
	// Arrange: the layout of the coffee-shop examples; the framework is only in the shared props.
	root := t.TempDir()
	writeFile(t, root, "Shop.slnx", `<Solution><Project Path="s-frontend/Frontend.csproj" /><Project Path="s-order/Order.csproj" /></Solution>`)
	writeFile(t, root, "Directory.Build.props", `<Project><PropertyGroup><TargetFramework>net11.0</TargetFramework></PropertyGroup></Project>`)
	writeFile(t, root, "s-frontend/Frontend.csproj", `<Project Sdk="Microsoft.NET.Sdk.Web"/>`)
	writeFile(t, root, "s-order/Order.csproj", `<Project Sdk="Microsoft.NET.Sdk.Web"/>`)

	// Act
	services := dotnet.DiscoverServices(root, []string{"Shop.slnx"})

	// Assert
	if len(services) != 2 {
		t.Fatalf("got %d services, want 2: %+v", len(services), services)
	}
	for _, s := range services {
		if got := s.FrameworkLabel(); got != "net11.0" {
			t.Errorf("service %s framework = %q, want net11.0", s.Name, got)
		}
	}
}
