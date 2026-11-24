package gcp

import (
	"errors"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// assertInOrder fails unless every needle appears in out, each after the previous one.
func assertInOrder(t *testing.T, out string, needles ...string) {
	t.Helper()
	pos := 0
	for _, n := range needles {
		i := strings.Index(out[pos:], n)
		if i < 0 {
			t.Errorf("output missing %q (in order) in:\n%s", n, out)
			return
		}
		pos += i + len(n)
	}
}

// assertConfigErrorFor fails unless err is already-reported and classifies as a
// ConfigError naming exactly field.
func assertConfigErrorFor(t *testing.T, err error, field string) {
	t.Helper()
	if !errors.Is(err, installer.ErrAlreadyReported) {
		t.Errorf("err = %v, want ErrAlreadyReported", err)
	}
	var cfgErr *installer.ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("err = %v, want *installer.ConfigError", err)
	}
	if len(cfgErr.MissingFields) != 1 || cfgErr.MissingFields[0] != field {
		t.Errorf("MissingFields = %v, want [%s]", cfgErr.MissingFields, field)
	}
}

var allVerbs = []string{verbInstall, verbUpdate, verbUninstall}

func TestReportPreflight_NoProject(t *testing.T) {
	for _, verb := range allVerbs {
		t.Run(verb, func(t *testing.T) {
			// Arrange
			in := &noProjectError{account: "jane@example.com"}
			var err error

			// Act
			out := captureStdout(t, func() { err = reportPreflight(verb, in) })

			// Assert
			assertInOrder(t, out,
				"Google Cloud "+verb+" not run: no gcloud project is selected.",
				"Logged in as: jane@example.com",
				"gcloud projects list",
				"gcloud config set project <PROJECT_ID>",
				"dtwiz "+verb+" gcp",
			)
			leftBehind := strings.Contains(out, "would stay behind")
			if leftBehind != (verb == verbUninstall) {
				t.Errorf("left-behind paragraph shown = %v, want only for uninstall", leftBehind)
			}
			assertConfigErrorFor(t, err, "gcloud.project")
		})
	}
}

func TestReportPreflight_NoAccount(t *testing.T) {
	for _, verb := range allVerbs {
		t.Run(verb, func(t *testing.T) {
			// Arrange
			var err error

			// Act
			out := captureStdout(t, func() { err = reportPreflight(verb, errNoGCloudAccount) })

			// Assert
			assertInOrder(t, out,
				"Google Cloud "+verb+" not run: no gcloud account is logged in.",
				"gcloud auth login",
				"dtwiz "+verb+" gcp",
			)
			if strings.Contains(out, "gcloud projects list") || strings.Contains(out, "Logged in as") {
				t.Errorf("no-account box must not show project guidance:\n%s", out)
			}
			assertConfigErrorFor(t, err, "gcloud.account")
		})
	}
}

func TestReportPreflight_UnrelatedErrorPassesThrough(t *testing.T) {
	// Arrange
	in := errors.New("failed to read gcloud configuration: boom")
	var err error

	// Act
	out := captureStdout(t, func() { err = reportPreflight(verbInstall, in) })

	// Assert
	if err != in {
		t.Errorf("err = %v, want the input error unchanged", err)
	}
	if out != "" {
		t.Errorf("unexpected output for unrelated error:\n%s", out)
	}
}
