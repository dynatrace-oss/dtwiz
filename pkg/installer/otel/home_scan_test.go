package otel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

// ── path relationship helpers ─────────────────────────────────────────────────

func TestIsDescendant(t *testing.T) {
	base := t.TempDir()
	parent := filepath.Join(base, "parent")
	child := filepath.Join(parent, "child")
	sibling := filepath.Join(base, "parentother") // shares a name prefix but is disjoint

	cases := []struct {
		name          string
		child, parent string
		want          bool
	}{
		{"strict child", child, parent, true},
		{"same dir", parent, parent, false},
		{"parent is not descendant of child", parent, child, false},
		{"prefix sibling is not descendant", sibling, parent, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDescendant(tc.child, tc.parent); got != tc.want {
				t.Errorf("isDescendant(%q, %q) = %v, want %v", tc.child, tc.parent, got, tc.want)
			}
		})
	}
}

func TestPathsInSameLineage(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	under := filepath.Join(home, "projects", "foo")
	prefixSibling := filepath.Join(base, "homeother")
	if err := os.MkdirAll(under, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(prefixSibling, 0755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"equal", home, home, true},
		{"cwd under home", under, home, true},
		{"home under cwd", home, base, true},
		{"disjoint prefix sibling", prefixSibling, home, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathsInSameLineage(tc.a, tc.b); got != tc.want {
				t.Errorf("pathsInSameLineage(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// ── selectScanRoots decision logic ────────────────────────────────────────────
//
// Under `go test`, stdin is not a character device, so selectScanRoots never
// prompts: same-lineage cases return the working directory only, and the
// disjoint case falls through to the non-interactive default (cwd + home).

func rootsContain(roots []string, target string) bool {
	rt := resolvePath(target)
	for _, r := range roots {
		if resolvePath(r) == rt {
			return true
		}
	}
	return false
}

func TestSelectScanRoots_SkipsWhenSameLineage(t *testing.T) {
	t.Run("cwd is home", func(t *testing.T) {
		home := fakeHome(t)
		helpers.SetTestWorkingDir(t, home)

		roots, err := selectScanRoots()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(roots) != 1 {
			t.Fatalf("expected working directory only, got %v", roots)
		}
	})

	t.Run("cwd under home", func(t *testing.T) {
		home := fakeHome(t)
		cwd := filepath.Join(home, "projects", "foo")
		if err := os.MkdirAll(cwd, 0755); err != nil {
			t.Fatal(err)
		}
		helpers.SetTestWorkingDir(t, cwd)

		roots, err := selectScanRoots()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(roots) != 1 || !rootsContain(roots, cwd) {
			t.Fatalf("expected [cwd] only, got %v", roots)
		}
	})

	t.Run("home under cwd", func(t *testing.T) {
		home := fakeHome(t)
		parent := filepath.Dir(home) // ancestor of home
		helpers.SetTestWorkingDir(t, parent)

		roots, err := selectScanRoots()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(roots) != 1 {
			t.Fatalf("expected working directory only (home already covered), got %v", roots)
		}
	})
}

func TestSelectScanRoots_DisjointDefaultsToBoth(t *testing.T) {
	// A piped stdin is not a character device, so selectScanRoots treats the run
	// as non-interactive and applies the default without prompting.
	setTestStdin(t, "")

	home := fakeHome(t)
	cwd := t.TempDir() // sibling of home under the test temp root: disjoint
	if pathsInSameLineage(cwd, home) {
		t.Skipf("temp dirs unexpectedly share lineage (cwd=%q home=%q)", cwd, home)
	}
	helpers.SetTestWorkingDir(t, cwd)

	roots, err := selectScanRoots()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roots) != 2 || !rootsContain(roots, cwd) || !rootsContain(roots, home) {
		t.Fatalf("expected non-interactive default [cwd, home], got %v", roots)
	}
}

func TestSelectScanRoots_AutoConfirmDisjointScansBoth(t *testing.T) {
	old := installer.AutoConfirm
	installer.AutoConfirm = true
	t.Cleanup(func() { installer.AutoConfirm = old })

	home := fakeHome(t)
	cwd := t.TempDir()
	if pathsInSameLineage(cwd, home) {
		t.Skipf("temp dirs unexpectedly share lineage (cwd=%q home=%q)", cwd, home)
	}
	helpers.SetTestWorkingDir(t, cwd)

	roots, err := selectScanRoots()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roots) != 2 || !rootsContain(roots, home) {
		t.Fatalf("expected [cwd, home] under AutoConfirm, got %v", roots)
	}
}

// ── interactive prompt parsing ────────────────────────────────────────────────

func TestPromptHomeScanChoice(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  scanChoice
	}{
		{"enter defaults to both", "\n", scanChoiceBoth},
		{"y is both", "y\n", scanChoiceBoth},
		{"uppercase Y is both", "Y\n", scanChoiceBoth},
		{"c is cwd only", "c\n", scanChoiceCwdOnly},
		{"n aborts", "n\n", scanChoiceAbort},
		{"invalid then valid", "x\nc\n", scanChoiceCwdOnly},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setTestStdin(t, tc.input)
			if got := promptHomeScanChoice("/tmp/work", "/home/user"); got != tc.want {
				t.Errorf("promptHomeScanChoice() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ── multi-root dedup ──────────────────────────────────────────────────────────

// TestScanProjectDirs_MultiRootDedup scans [cwd, home] where a project lives
// under the always-on ~/.dtwiz/examples/ root (itself under home). It must be
// reported exactly once even though the home root and the examples root both
// reach it.
func TestScanProjectDirs_MultiRootDedup(t *testing.T) {
	home := fakeHome(t)
	bundled := filepath.Join(home, ".dtwiz", "examples", "schnitzel")
	if err := os.MkdirAll(bundled, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundled, "requirements.txt"), []byte("flask\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cwd := t.TempDir()
	helpers.SetTestWorkingDir(t, cwd)

	projects := scanProjectDirs([]string{"requirements.txt"}, nil, []string{cwd, home})

	count := 0
	for _, p := range projects {
		if strings.HasSuffix(filepath.ToSlash(p.Path), "schnitzel") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected schnitzel exactly once across [cwd, home] + examples; got %d in %v", count, projects)
	}
}
