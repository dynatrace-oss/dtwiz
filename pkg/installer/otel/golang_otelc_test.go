package otel

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoVersionSupportsOtelc(t *testing.T) {
	tests := []struct {
		out  string
		want bool
	}{
		{"go version go1.26.2 darwin/arm64", true},
		{"go version go1.25.0 linux/amd64", true},
		{"go version go1.24.9 linux/amd64", false},
		{"go version go1.21 windows/amd64", false},
		{"go version go2.0.1 linux/amd64", true},
		{"garbage", false},
	}
	for _, tt := range tests {
		if got := goVersionSupportsOtelc(tt.out); got != tt.want {
			t.Errorf("goVersionSupportsOtelc(%q) = %v, want %v", tt.out, got, tt.want)
		}
	}
}

func TestOtelcBuildCmd(t *testing.T) {
	mp := goMainPackage{ModuleDir: filepath.Join("proj", "mod"), PkgDir: "cmd/api"}
	cmd := otelcBuildCmd(context.Background(), "otelc", mp, "out", "work")
	args := strings.Join(cmd.Args, " ")
	for _, want := range []string{"--work-dir", "go build", "-o out", "./cmd/api"} {
		if !strings.Contains(args, want) {
			t.Errorf("args %q missing %q", args, want)
		}
	}
	if strings.Contains(args, "-modfile") {
		t.Errorf("otelc rejects -modfile, got %q", args)
	}
	hasGowork := false
	for _, e := range cmd.Env {
		hasGowork = hasGowork || e == "GOWORK=off"
	}
	if !hasGowork {
		t.Error("expected GOWORK=off so a go.work with an older go version does not reject otelc's go bump")
	}
	if cmd.Dir != mp.ModuleDir {
		t.Errorf("Dir = %q, want %q", cmd.Dir, mp.ModuleDir)
	}
}
