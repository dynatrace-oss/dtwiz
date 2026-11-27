package otel

import (
	"errors"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

const runtimes8 = "Microsoft.AspNetCore.App 8.0.4 [/usr/share/dotnet/shared/Microsoft.AspNetCore.App]\n" +
	"Microsoft.NETCore.App 8.0.4 [/usr/share/dotnet/shared/Microsoft.NETCore.App]\n"

// stubDotNet replaces the dotnet lookup and runtime listing for one test.
func stubDotNet(t *testing.T, found bool, listOut string, listErr error) {
	t.Helper()
	oldLook, oldList := dotnetLookPathFn, dotnetListRuntimesFn
	t.Cleanup(func() { dotnetLookPathFn, dotnetListRuntimesFn = oldLook, oldList })
	dotnetLookPathFn = func(string) (string, error) {
		if !found {
			return "", errors.New("not found")
		}
		return "/usr/bin/dotnet", nil
	}
	dotnetListRuntimesFn = func() (string, error) { return listOut, listErr }
}

func TestCheckDotNetPrerequisites(t *testing.T) {
	tests := []struct {
		name        string
		found       bool
		listOut     string
		listErr     error
		service     dotnet.Service
		wantMissing bool
		wantInstr   bool
		wantNote    string
	}{
		{name: "dotnet missing", found: false,
			service: dotnet.Service{Name: "Api", Frameworks: []string{"net8.0"}}, wantMissing: true, wantNote: ".NET is not installed"},
		{name: "runtime present", found: true, listOut: runtimes8,
			service: dotnet.Service{Name: "Api", Frameworks: []string{"net8.0"}}, wantInstr: true},
		{name: "newer runtime accepted", found: true, listOut: runtimes8,
			service: dotnet.Service{Name: "Api", Frameworks: []string{"net6.0"}}, wantInstr: true},
		{name: "runtime missing", found: true, listOut: runtimes8,
			service: dotnet.Service{Name: "Api", Frameworks: []string{"net9.0"}}, wantNote: "requires .NET 9 runtime, not installed"},
		{name: "multi-target passes on any satisfied framework", found: true, listOut: runtimes8,
			service: dotnet.Service{Name: "Api", Frameworks: []string{"net10.0", "net8.0"}}, wantInstr: true},
		{name: "multi-target all missing", found: true, listOut: runtimes8,
			service: dotnet.Service{Name: "Api", Frameworks: []string{"net9.0", "net10.0"}}, wantNote: "requires .NET 9 or 10 runtime, not installed"},
		{name: "netcoreapp moniker", found: true, listOut: runtimes8,
			service: dotnet.Service{Name: "Api", Frameworks: []string{"netcoreapp3.1"}}, wantInstr: true},
		{name: "unknown framework is not blocked", found: true, listOut: runtimes8,
			service: dotnet.Service{Name: "Api"}, wantInstr: true, wantNote: "framework not found in the project or Directory.Build.props, runtime not checked"},
		{name: "unparseable output is not blocked", found: true, listOut: "something unexpected\n",
			service: dotnet.Service{Name: "Api", Frameworks: []string{"net8.0"}}, wantInstr: true,
			wantNote: "installed runtimes could not be determined, runtime not checked"},
		{name: "listing fails is not blocked", found: true, listErr: errors.New("exit 1"),
			service: dotnet.Service{Name: "Api", Frameworks: []string{"net8.0"}}, wantInstr: true,
			wantNote: "installed runtimes could not be determined, runtime not checked"},
		{name: ".NET Framework reason wins", found: true, listOut: runtimes8,
			service:  dotnet.Service{Name: "Legacy", Frameworks: []string{"net48"}, DotNetFramework: true},
			wantNote: ".NET Framework, not supported yet"},
		{name: "self-contained reason wins", found: true, listOut: runtimes8,
			service:  dotnet.Service{Name: "Sc", Frameworks: []string{"net8.0"}, SelfContained: true},
			wantNote: "self-contained, not supported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			stubDotNet(t, tt.found, tt.listOut, tt.listErr)

			// Act
			got := checkDotNetPrerequisites([]dotnet.Service{tt.service})

			// Assert
			if got.DotnetMissing != tt.wantMissing {
				t.Errorf("DotnetMissing = %v, want %v", got.DotnetMissing, tt.wantMissing)
			}
			if len(got.Services) != 1 {
				t.Fatalf("got %d service checks, want 1", len(got.Services))
			}
			if c := got.Services[0]; c.Instrumented != tt.wantInstr || c.Note != tt.wantNote {
				t.Errorf("check = {Instrumented:%v Note:%q}, want {%v %q}", c.Instrumented, c.Note, tt.wantInstr, tt.wantNote)
			}
		})
	}
}

func TestParseNetCoreRuntimeMajors(t *testing.T) {
	// Arrange
	out := "Microsoft.AspNetCore.App 9.0.1 [/x]\nMicrosoft.NETCore.App 6.0.30 [/x]\nMicrosoft.NETCore.App 8.0.4 [/x]\r\nMicrosoft.WindowsDesktop.App 8.0.4 [/x]\n"

	// Act
	got := parseNetCoreRuntimeMajors(out)

	// Assert
	if len(got) != 2 || got[0] != 6 || got[1] != 8 {
		t.Errorf("majors = %v, want [6 8]", got)
	}
}
