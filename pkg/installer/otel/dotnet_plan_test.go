package otel

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/test/helpers"
)

func TestShowDotNetStep1Outcome_ListsServicesAndEndsInManualSteps(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Api/Api.csproj", `<Project Sdk="Microsoft.NET.Sdk.Web"><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>`)
	writeDotNetFile(t, dir, "Legacy/Legacy.csproj", `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><OutputType>Exe</OutputType><TargetFramework>net48</TargetFramework></PropertyGroup></Project>`)
	proj := detectedProject{ScannedProject: ScannedProject{Path: dir}, Runtime: dotNetRuntimeName}
	before := listTree(t, dir)

	// Act
	var slug string
	out := helpers.CaptureStdout(t, func() { slug = showDotNetStep1Outcome(proj) })

	// Assert
	if slug != dotNetManualSlug {
		t.Errorf("slug = %q, want %q", slug, dotNetManualSlug)
	}
	for _, want := range []string{"Api  (net8.0)", "Legacy  (net48)  [.NET Framework, not supported yet]"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if after := listTree(t, dir); after != before {
		t.Errorf("files changed:\nbefore %s\nafter  %s", before, after)
	}
}

func TestShowDotNetStep1Outcome_NoServiceReported(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeDotNetFile(t, dir, "Lib.csproj", dotNetLib)
	proj := detectedProject{ScannedProject: ScannedProject{Path: dir, Markers: []string{"Lib.csproj"}}, Runtime: dotNetRuntimeName}

	// Act
	var slug string
	out := helpers.CaptureStdout(t, func() { slug = showDotNetStep1Outcome(proj) })

	// Assert
	if !strings.Contains(out, "No runnable .NET service found") || slug != dotNetManualSlug {
		t.Errorf("slug=%q output:\n%s", slug, out)
	}
}

func listTree(t *testing.T, dir string) string {
	t.Helper()
	var sb strings.Builder
	_ = filepath.Walk(dir, func(p string, info fs.FileInfo, err error) error {
		if err == nil {
			sb.WriteString(p + "|" + info.ModTime().String() + ";")
		}
		return nil
	})
	return sb.String()
}
