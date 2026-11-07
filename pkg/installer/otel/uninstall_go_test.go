package otel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

// makeGoOtelDir creates <module>/.otel/go with the dtwiz marker and a binary.
func makeGoOtelDir(t *testing.T, module string) string {
	t.Helper()
	goDir := goOtelDir(module)
	writeTree(t, goDir, map[string]string{goMarkerFile: "{}\n", "bin/api": "x", "logs/api.log": "log"})
	return goDir
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	helpers.SetTestWorkingDir(t, dir)
}

func TestFindGoOtelDirs(t *testing.T) {
	setGoFlag(t, true)
	root := t.TempDir()
	a := makeGoOtelDir(t, filepath.Join(root, "a"))
	b := makeGoOtelDir(t, filepath.Join(root, "services", "b"))
	// Node-only .otel/ (package.json, no Go marker) and an unrelated .otel/.
	writeTree(t, root, map[string]string{
		"node/.otel/package.json":      `{"dependencies":{"@opentelemetry/sdk-node":"latest"}}`,
		"other/.otel/something.txt":    "x",
		"gomissing/.otel/go/bin/thing": "x", // go dir without marker
	})
	chdir(t, root)

	got := findGoOtelDirs()
	if len(got) != 2 {
		t.Fatalf("expected the two marked directories, got %v", got)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{filepath.Base(filepath.Dir(filepath.Dir(a))), filepath.Base(filepath.Dir(filepath.Dir(b)))} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %v", want, got)
		}
	}
	for _, unwanted := range []string{"node", "other", "gomissing"} {
		if strings.Contains(joined, string(filepath.Separator)+unwanted+string(filepath.Separator)) {
			t.Fatalf("%s must not be reported: %v", unwanted, got)
		}
	}
}

func TestFindGoOtelDirs_FlagOff(t *testing.T) {
	setGoFlag(t, false)
	root := t.TempDir()
	makeGoOtelDir(t, root)
	chdir(t, root)
	if got := findGoOtelDirs(); got != nil {
		t.Fatalf("flag off: no Go scan expected, got %v", got)
	}
	if tool := isolatedOtelcPath(); tool != "" {
		t.Fatalf("flag off: tool must not be reported, got %q", tool)
	}
}

func TestGoCleaner_FlagOffNothingDetected(t *testing.T) {
	setGoFlag(t, false)
	orig := goListProcessesFn
	goListProcessesFn = func() []DetectedProcess {
		t.Fatal("process scan must not run with the flag off")
		return nil
	}
	t.Cleanup(func() { goListProcessesFn = orig })
	if got := (goCleaner{}).DetectProcesses(); got == nil || len(got) != 0 {
		t.Fatalf("expected empty non-nil result, got %#v", got)
	}
}

func TestGoCleaner_DetectsOnlyDtwizBinaries(t *testing.T) {
	setGoFlag(t, true)
	root, err := filepath.EvalSymlinks(t.TempDir()) // macOS temp dirs sit behind a symlink
	if err != nil {
		t.Fatal(err)
	}
	shop := makeGoOtelDir(t, filepath.Join(root, "shop"))
	billing := makeGoOtelDir(t, filepath.Join(root, "billing"))
	chdir(t, t.TempDir()) // uninstall may run from anywhere
	api := filepath.Join(shop, "bin", "api")
	worker := filepath.Join(shop, "bin", "worker.exe")
	procs := []DetectedProcess{
		{PID: 1, Command: api + " --x"},
		{PID: 2, Command: "go run ./cmd/api"},
		{PID: 3, Command: "/work/shop/bin/api"},
		{PID: 4, Command: `"` + worker + `" -a`},
		{PID: 5, Command: "vim " + api},
		{PID: 6, Command: "go run " + api},
		{PID: 7, Command: "/elsewhere/other/.otel/go/bin/api"}, // no dtwiz marker
		{PID: 8, Command: filepath.Join(billing, "bin", "api")},
	}
	orig := goListProcessesFn
	goListProcessesFn = func() []DetectedProcess { return procs }
	t.Cleanup(func() { goListProcessesFn = orig })

	got := (goCleaner{}).DetectProcesses()
	var pids []int
	for _, p := range got {
		pids = append(pids, p.PID)
	}
	if len(pids) != 3 || pids[0] != 1 || pids[1] != 4 || pids[2] != 8 {
		t.Fatalf("PIDs = %v, want [1 4 8] (binaries under a marked .otel/go/bin, from any directory)", pids)
	}
}

func TestGoCleaner_ScanFailureSkipsRuntime(t *testing.T) {
	setGoFlag(t, true)
	orig := goListProcessesFn
	goListProcessesFn = func() []DetectedProcess { return nil }
	t.Cleanup(func() { goListProcessesFn = orig })
	if got := (goCleaner{}).DetectProcesses(); got != nil {
		t.Fatalf("scan failure should yield nil, got %v", got)
	}
}

func TestGoCleanerRegistered(t *testing.T) {
	for _, c := range runtimeCleaners {
		if c.Label() == "Go" {
			return
		}
	}
	t.Fatal("goCleaner must be registered in runtimeCleaners")
}

func TestRemoveGoOtelDir_RemovesDirAndEmptyOtel(t *testing.T) {
	module := t.TempDir()
	goDir := makeGoOtelDir(t, module)
	writeTree(t, module, map[string]string{"main.go": mainSrc, "go.mod": "module x\n", "go.sum": "s\n"})

	backup, err := removeGoOtelDir(goDir)
	if err != nil || backup != "" {
		t.Fatalf("backup=%q err=%v", backup, err)
	}
	if _, err := os.Stat(filepath.Join(module, ".otel")); !os.IsNotExist(err) {
		t.Fatal(".otel/ should be removed when left empty")
	}
	for _, f := range []string{"main.go", "go.mod", "go.sum"} {
		if _, err := os.Stat(filepath.Join(module, f)); err != nil {
			t.Fatalf("project file %s must be untouched", f)
		}
	}
}

func TestRemoveGoOtelDir_KeepsNodeFiles(t *testing.T) {
	module := t.TempDir()
	goDir := makeGoOtelDir(t, module)
	writeTree(t, module, map[string]string{".otel/package.json": `{"@opentelemetry":1}`})

	if _, err := removeGoOtelDir(goDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(module, ".otel", "package.json")); err != nil {
		t.Fatal("Node.js files in the same .otel/ must be kept")
	}
	if _, err := os.Stat(goDir); !os.IsNotExist(err) {
		t.Fatal("go dir should be removed")
	}
}

func TestRemoveGoOtelDir_PreservesBackup(t *testing.T) {
	module := t.TempDir()
	goDir := makeGoOtelDir(t, module)
	writeTree(t, goDir, map[string]string{"modguard/go.mod": "module orig\n"})

	backup, err := removeGoOtelDir(goDir)
	if err != nil {
		t.Fatal(err)
	}
	if backup != filepath.Join(goDir, "modguard") || readFile(t, filepath.Join(backup, "go.mod")) != "module orig\n" {
		t.Fatalf("backup must be preserved, got %q", backup)
	}
	if _, err := os.Stat(filepath.Join(goDir, "bin")); !os.IsNotExist(err) {
		t.Fatal("everything except the backup should be removed")
	}
}

func TestRemoveIsolatedOtelc_OnlyDtwizCopy(t *testing.T) {
	home := setHome(t)
	setGoFlag(t, true)
	binDir := filepath.Join(home, ".opentelemetry", "go", "bin")
	writeTree(t, binDir, map[string]string{otelcBinaryName(): "x"})
	// A user-owned copy in GOBIN must never be listed or deleted.
	gobin := filepath.Join(home, "go", "bin")
	writeTree(t, gobin, map[string]string{otelcBinaryName(): "user"})
	t.Setenv("GOBIN", gobin)

	path := isolatedOtelcPath()
	if path != filepath.Join(binDir, otelcBinaryName()) {
		t.Fatalf("isolatedOtelcPath = %q", path)
	}
	if err := removeIsolatedOtelc(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("dtwiz's otelc should be deleted")
	}
	if _, err := os.Stat(filepath.Join(home, ".opentelemetry")); !os.IsNotExist(err) {
		t.Fatal("empty tool directories should be cleaned up")
	}
	if readFile(t, filepath.Join(gobin, otelcBinaryName())) != "user" {
		t.Fatal("the user's own otelc must remain")
	}
}

func TestUninstallOtelCollector_GoSection(t *testing.T) {
	home := setHome(t)
	setGoFlag(t, true)
	root := t.TempDir()
	goDir := makeGoOtelDir(t, filepath.Join(root, "shop"))
	writeTree(t, goDir, map[string]string{"modguard/go.mod": "module orig\n"})
	writeTree(t, filepath.Join(home, ".opentelemetry", "go", "bin"), map[string]string{otelcBinaryName(): "x"})
	chdir(t, root)

	out := helpers.CaptureStdout(t, func() {
		printGoUninstallSection(findGoOtelDirs(), isolatedOtelcPath())
	})
	for _, want := range []string{"Go instrumentation directories", goDir, "interrupted build", "tool installed by dtwiz"} {
		if !strings.Contains(out, want) {
			t.Fatalf("Go section should contain %q:\n%s", want, out)
		}
	}
}

func runGoDryRunUninstall(t *testing.T, flag bool) (string, string) {
	t.Helper()
	setHome(t)
	setGoFlag(t, flag)
	origDetect, origEnrich := detectJavaProcessesFunc, enrichProcessesWithJPSFunc
	detectJavaProcessesFunc = func() []DetectedProcess { return []DetectedProcess{} }
	enrichProcessesWithJPSFunc = func(procs []DetectedProcess) []DetectedProcess { return procs }
	origCleaners := runtimeCleaners
	runtimeCleaners = []RuntimeCleaner{goCleaner{}}
	t.Cleanup(func() {
		detectJavaProcessesFunc, enrichProcessesWithJPSFunc, runtimeCleaners = origDetect, origEnrich, origCleaners
	})
	origList := goListProcessesFn
	goListProcessesFn = func() []DetectedProcess { return []DetectedProcess{} }
	t.Cleanup(func() { goListProcessesFn = origList })

	root := t.TempDir()
	goDir := makeGoOtelDir(t, filepath.Join(root, "shop"))
	chdir(t, root)

	out := stripANSI(helpers.CaptureStdout(t, func() {
		_ = UninstallOtelCollector("https://env.example.com", "dt0s16.test", true)
	}))
	return out, goDir
}

func TestUninstallOtelCollector_GoDryRun(t *testing.T) {
	out, goDir := runGoDryRunUninstall(t, true)
	if !strings.Contains(out, "Go instrumentation directories that will be removed") || !strings.Contains(out, goDir) {
		t.Fatalf("Go section expected in the preview:\n%s", out)
	}
	if _, err := os.Stat(goDir); err != nil {
		t.Fatal("dry-run must not delete anything")
	}
}

func TestUninstallOtelCollector_GoFlagOff(t *testing.T) {
	out, goDir := runGoDryRunUninstall(t, false)
	if strings.Contains(out, "Go instrumentation") {
		t.Fatalf("no Go section expected with the flag off:\n%s", out)
	}
	if _, err := os.Stat(goDir); err != nil {
		t.Fatal("nothing may be removed with the flag off")
	}
}

func TestFindGoOtelDirs_RegisteredFoundFromAnyDirectory(t *testing.T) {
	setGoFlag(t, true)
	useTempRegistry(t)
	project := t.TempDir()
	goDir := makeGoOtelDir(t, project)
	registerOtelDir(goDir)
	chdir(t, t.TempDir()) // unrelated working directory

	got := findGoOtelDirs()
	if len(got) != 1 || got[0] != goDir {
		t.Fatalf("findGoOtelDirs = %v, want [%s]", got, goDir)
	}

	// A registered directory without the dtwiz marker is not trusted.
	if err := os.Remove(filepath.Join(goDir, goMarkerFile)); err != nil {
		t.Fatal(err)
	}
	if got := findGoOtelDirs(); len(got) != 0 {
		t.Fatalf("directory without marker must be ignored, got %v", got)
	}
}
