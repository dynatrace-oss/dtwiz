package otel

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitCommandLine(t *testing.T) {
	got := splitCommandLine(`"C:\Program Files\app\svc.exe" --port 80`)
	want := []string{`C:\Program Files\app\svc.exe`, "--port", "80"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitCommandLine = %v, want %v", got, want)
	}
	if got := splitCommandLine("   "); len(got) != 0 {
		t.Fatalf("expected no fields, got %v", got)
	}
}

func TestSelectGoProcesses(t *testing.T) {
	abs := func(parts ...string) string {
		return filepath.Join(append([]string{string(filepath.Separator) + "work"}, parts...)...)
	}
	svc := abs("shop", ".otel", "go", "bin", "api")
	isGo := func(path string) bool { return path == svc || path == filepath.Join(abs("shop"), "server") }

	all := []DetectedProcess{
		{PID: 1, Command: "go run ./cmd/api", WorkingDirectory: abs("shop")},
		{PID: 2, Command: svc + " --flag", WorkingDirectory: abs("shop")},
		{PID: 3, Command: "./server", WorkingDirectory: abs("shop")},
		{PID: 4, Command: "/usr/bin/python3 app.py"},
		{PID: 5, Command: "go build ./..."},
		{PID: 6, Command: abs("bin", "gopls") + " serve"},
		{PID: 7, Command: "server", WorkingDirectory: abs("shop")},
	}

	got := selectGoProcesses(all, isGo)
	var pids []int
	for _, p := range got {
		pids = append(pids, p.PID)
	}
	if want := []int{1, 2, 3}; !reflect.DeepEqual(pids, want) {
		t.Fatalf("selected PIDs = %v, want %v", pids, want)
	}
}

func TestSelectGoProcesses_NoProcesses(t *testing.T) {
	if got := selectGoProcesses(nil, func(string) bool { return true }); len(got) != 0 {
		t.Fatalf("expected empty selection, got %v", got)
	}
}

func TestGoProcessAssociation_ScanFailure(t *testing.T) {
	projects := []ScannedProject{{Path: "/work/shop"}}
	matchProcessesToProjects(projects, selectGoProcesses(nil, isGoBinary))
	if len(projects[0].RunningProcessIDs) != 0 {
		t.Fatalf("expected no running processes, got %v", projects[0].RunningProcessIDs)
	}
}

func TestGoProcessAssociation_MatchesProjectByWorkingDirectory(t *testing.T) {
	projects := []ScannedProject{{Path: "/work/shop"}, {Path: "/work/other"}}
	procs := selectGoProcesses([]DetectedProcess{
		{PID: 10, Command: "go run .", WorkingDirectory: "/work/shop"},
		{PID: 11, Command: "/work/shop/bin/api", WorkingDirectory: "/work/shop/bin"},
	}, func(string) bool { return true })
	matchProcessesToProjects(projects, procs)
	if !reflect.DeepEqual(projects[0].RunningProcessIDs, []int{10, 11}) {
		t.Fatalf("shop PIDs = %v", projects[0].RunningProcessIDs)
	}
	if len(projects[1].RunningProcessIDs) != 0 {
		t.Fatalf("other PIDs = %v", projects[1].RunningProcessIDs)
	}
}
