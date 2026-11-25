package dotnet_test

import (
	"reflect"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

func selectedPIDs(procs []dotnet.Process) []int {
	pids := make([]int, 0, len(procs))
	for _, p := range procs {
		pids = append(pids, p.PID)
	}
	return pids
}

func TestSelectProcesses_DotnetInvocations(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"dotnet run", "dotnet run --project Api", true},
		{"dotnet watch", "/usr/bin/dotnet watch run", true},
		{"dotnet app dll", "dotnet /src/Api/bin/Debug/net8.0/Api.dll", true},
		{"dotnet exec app dll", "dotnet exec --runtimeconfig x.json /src/Api.dll", true},
		{"host option before dll", "dotnet --roll-forward Major /src/Api.dll", true},
		{"windows dotnet.exe", `"C:\Program Files\dotnet\dotnet.exe" C:\src\Api\bin\Debug\Api.dll`, true},
		{"bare dotnet", "dotnet", false},
		{"dotnet build", "dotnet build", false},
		{"dotnet restore", "dotnet restore src", false},
		{"build server", "dotnet build-server", false},
		{"msbuild node", "dotnet /usr/share/dotnet/sdk/8.0.100/MSBuild.dll /nodemode:1", false},
		{"compiler server", "dotnet /usr/share/dotnet/sdk/8.0.100/Roslyn/bincore/VBCSCompiler.dll", false},
		{"test host", "dotnet exec --runtimeconfig t.json /src/bin/testhost.dll --port 1", false},
		{"ide language service", "dotnet /home/u/.vscode/extensions/ms-dotnettools.csharp-1/.roslyn/Microsoft.CodeAnalysis.LanguageServer.dll", false},
		{"dtwiz itself", "/usr/local/bin/dtwiz install otel", false},
		{"IIS worker", `c:\windows\system32\inetsrv\w3wp.exe -ap DefaultAppPool`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			procs := []dotnet.Process{{PID: 1, Command: tt.command}}

			// Act
			got := dotnet.SelectProcesses(procs)

			// Assert
			if (len(got) == 1) != tt.want {
				t.Errorf("selected = %v, want selected=%v", got, tt.want)
			}
		})
	}
}

func TestSelectProcesses_AppHostRequiresManagedLibraryAndRuntimeConfig(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	good := writeFile(t, dir, "good/MyApp", "")
	writeFile(t, dir, "good/MyApp.dll", "")
	writeFile(t, dir, "good/MyApp.runtimeconfig.json", "{}")
	noConfig := writeFile(t, dir, "noconfig/Other", "")
	writeFile(t, dir, "noconfig/Other.dll", "")
	winExe := writeFile(t, dir, "win/WinApp.exe", "")
	writeFile(t, dir, "win/WinApp.dll", "")
	writeFile(t, dir, "win/WinApp.runtimeconfig.json", "{}")
	testhost := writeFile(t, dir, "th/testhost", "")
	writeFile(t, dir, "th/testhost.dll", "")
	writeFile(t, dir, "th/testhost.runtimeconfig.json", "{}")
	procs := []dotnet.Process{
		{PID: 1, Command: good + " --urls http://localhost:5000"},
		{PID: 2, Command: noConfig},
		{PID: 3, Command: `"` + winExe + `"`},
		{PID: 4, Command: testhost},
		{PID: 5, Command: "/bin/bash"},
	}

	// Act
	got := dotnet.SelectProcesses(procs)

	// Assert
	if want := []int{1, 3}; !reflect.DeepEqual(selectedPIDs(got), want) {
		t.Errorf("selected PIDs = %v, want %v", selectedPIDs(got), want)
	}
}

func TestSelectProcesses_RelativeAppHostUsesWorkingDirectory(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, "bin/App", "")
	writeFile(t, dir, "bin/App.dll", "")
	writeFile(t, dir, "bin/App.runtimeconfig.json", "{}")
	procs := []dotnet.Process{
		{PID: 1, Command: "./bin/App", WorkingDirectory: dir},
		{PID: 2, Command: "./bin/App"},
	}

	// Act
	got := dotnet.SelectProcesses(procs)

	// Assert
	if want := []int{1}; !reflect.DeepEqual(selectedPIDs(got), want) {
		t.Errorf("selected PIDs = %v, want %v", selectedPIDs(got), want)
	}
}

func TestMatchPIDs(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		proc    dotnet.Process
		matches bool
	}{
		{"working directory inside project", "/src/api", dotnet.Process{PID: 1, WorkingDirectory: "/src/api/sub"}, true},
		{"dll in bin", "/src/api", dotnet.Process{PID: 1, Command: "dotnet /src/api/bin/Debug/net8.0/Api.dll"}, true},
		{"case difference", "/src/Api", dotnet.Process{PID: 1, Command: "dotnet /SRC/api/bin/Api.dll"}, true},
		{"windows quoted path with spaces", `C:\My Src\Api`, dotnet.Process{PID: 1, Command: `"C:\My Src\Api\bin\Debug\Api.exe"`}, true},
		{"other separator and case", `C:\Src\Api`, dotnet.Process{PID: 1, Command: "dotnet c:/src/api/bin/Api.dll"}, true},
		{"unix path with spaces via working directory", "/home/u/My Proj", dotnet.Process{PID: 1, Command: "dotnet run", WorkingDirectory: "/home/u/My Proj"}, true},
		{"unreadable working directory uses command line", "/src/api", dotnet.Process{PID: 1, Command: "/src/api/bin/App"}, true},
		{"relative executable resolved by working directory", "/src/api", dotnet.Process{PID: 1, Command: "./bin/App", WorkingDirectory: "/src/api"}, true},
		{"sibling directory with common prefix", "/src/api", dotnet.Process{PID: 1, Command: "dotnet /src/api2/App.dll", WorkingDirectory: "/src/api2"}, false},
		{"path embedded in a longer path", "/src/api", dotnet.Process{PID: 1, Command: "dotnet /other/src/api/App.dll"}, false},
		{"outside every project", "/src/api", dotnet.Process{PID: 1, Command: "dotnet /opt/x/App.dll", WorkingDirectory: "/opt/x"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := dotnet.MatchPIDs(tt.dir, []dotnet.Process{tt.proc})

			// Assert
			if (len(got) == 1) != tt.matches {
				t.Errorf("MatchPIDs(%q) = %v, want match=%v", tt.dir, got, tt.matches)
			}
		})
	}
}

func TestMatchPIDs_NoProcessesReturnsEmptySlice(t *testing.T) {
	// Act
	got := dotnet.MatchPIDs("/src/api", nil)

	// Assert
	if got == nil || len(got) != 0 {
		t.Errorf("MatchPIDs = %#v, want empty non-nil slice", got)
	}
}

func TestSplitCommandLine_QuotedPath(t *testing.T) {
	// Act
	got := dotnet.SplitCommandLine(`"C:\Program Files\dotnet\dotnet.exe" run`)

	// Assert
	if want := []string{`C:\Program Files\dotnet\dotnet.exe`, "run"}; !reflect.DeepEqual(got, want) {
		t.Errorf("fields = %v, want %v", got, want)
	}
}
