package otel

import (
	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

// listProcessesFn is overridable in tests.
var listProcessesFn = listProcesses

// detectDotNetProcesses returns the running .NET applications: `dotnet run`,
// `dotnet watch`, `dotnet <app>.dll` and application-host executables.
func detectDotNetProcesses() []DetectedProcess {
	all := listProcessesFn()
	in := make([]dotnet.Process, 0, len(all))
	for _, p := range all {
		in = append(in, dotnet.Process{PID: p.PID, Command: p.Command, WorkingDirectory: p.WorkingDirectory})
	}
	selected := dotnet.SelectProcesses(in)
	out := make([]DetectedProcess, 0, len(selected))
	for _, p := range selected {
		out = append(out, DetectedProcess{PID: p.PID, Command: p.Command, WorkingDirectory: p.WorkingDirectory})
	}
	return out
}

// matchDotNetProcessesToProjects annotates projects with the PIDs of .NET
// processes whose working directory, command line or executable lies inside the
// project. Unlike the other runtimes it ignores letter case and path separator style.
func matchDotNetProcessesToProjects(projects []ScannedProject, processes []DetectedProcess) {
	in := make([]dotnet.Process, 0, len(processes))
	for _, p := range processes {
		in = append(in, dotnet.Process{PID: p.PID, Command: p.Command, WorkingDirectory: p.WorkingDirectory})
	}
	for i := range projects {
		projects[i].RunningProcessIDs = dotnet.MatchPIDs(projects[i].Path, in)
	}
}
