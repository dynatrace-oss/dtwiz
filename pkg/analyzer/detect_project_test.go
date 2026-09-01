package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

// chdirTemp creates a temp dir, changes into it, and registers a cleanup to
// restore the original working directory before t.TempDir cleans up the dir.
// On Windows, the process must release the directory before it can be deleted.
func chdirTemp(t *testing.T) string {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// Register AFTER TempDir so this cleanup runs FIRST (LIFO), releasing the
	// directory handle before TempDir's own cleanup tries to remove it.
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	return dir
}

func TestShortenPath_HomePrefix(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	path := filepath.Join(home, "Code", "dtwiz", "go.mod")
	got := shortenPath(path)
	want := "~" + string(filepath.Separator) + filepath.Join("Code", "dtwiz", "go.mod")
	if got != want {
		t.Errorf("shortenPath(%q) = %q, want %q", path, got, want)
	}
}

func TestShortenPath_OutsideHome(t *testing.T) {
	path := string(filepath.Separator) + filepath.Join("tmp", "work", "go.mod")
	got := shortenPath(path)
	if got != path {
		t.Errorf("shortenPath(%q) = %q, want unchanged %q", path, got, path)
	}
}

func TestShortenPath_SiblingDirectory(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	// /home/alice2 must not be shortened when home is /home/alice.
	sibling := home + "2"
	got := shortenPath(filepath.Join(sibling, "go.mod"))
	if got == "~"+string(filepath.Separator)+filepath.Join("2", "go.mod") {
		t.Errorf("shortenPath incorrectly shortened sibling path %q", sibling)
	}
}

func TestShortenPath_HomeItself(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	got := shortenPath(home)
	if got != "~" {
		t.Errorf("shortenPath(home) = %q, want ~", got)
	}
}

func TestDetectProject_GoProject(t *testing.T) {
	dir := chdirTemp(t)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0600); err != nil {
		t.Fatal(err)
	}

	_, techs := detectProject()
	found := false
	for _, tech := range techs {
		if tech.Name == "Go" {
			found = true
		}
	}
	if !found {
		t.Error("expected Go tech to be detected")
	}
}

func TestDetectProject_NodeProject(t *testing.T) {
	dir := chdirTemp(t)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	_, techs := detectProject()
	found := false
	for _, tech := range techs {
		if tech.Name == "Node.js" {
			found = true
		}
	}
	if !found {
		t.Error("expected Node.js tech to be detected")
	}
}

func TestDetectProject_PythonProject(t *testing.T) {
	dir := chdirTemp(t)
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask"), 0600); err != nil {
		t.Fatal(err)
	}

	_, techs := detectProject()
	found := false
	for _, tech := range techs {
		if tech.Name == "Python" {
			found = true
		}
	}
	if !found {
		t.Error("expected Python tech to be detected")
	}
}

func TestDetectProject_JavaProject(t *testing.T) {
	dir := chdirTemp(t)
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0600); err != nil {
		t.Fatal(err)
	}

	_, techs := detectProject()
	found := false
	for _, tech := range techs {
		if tech.Name == "Java" {
			found = true
		}
	}
	if !found {
		t.Error("expected Java tech to be detected")
	}
}

func TestDetectProject_EmptyDirectory(t *testing.T) {
	chdirTemp(t)

	_, techs := detectProject()
	if len(techs) != 0 {
		t.Errorf("expected no techs in empty directory, got %v", techs)
	}
}

func TestDetectProject_MultipleTechs(t *testing.T) {
	dir := chdirTemp(t)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	_, techs := detectProject()
	if len(techs) < 2 {
		t.Errorf("expected at least 2 techs, got %d", len(techs))
	}
}

func TestDetectProject_DuplicateTechDetectedOnce(t *testing.T) {
	dir := chdirTemp(t)
	// Both requirements.txt and pyproject.toml → should produce exactly one Python entry.
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool]"), 0600); err != nil {
		t.Fatal(err)
	}

	_, techs := detectProject()
	count := 0
	for _, tech := range techs {
		if tech.Name == "Python" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 Python entry, got %d", count)
	}
}
