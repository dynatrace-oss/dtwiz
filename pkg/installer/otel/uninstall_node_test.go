package otel

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeProcessDescription(t *testing.T) {
	t.Parallel()

	projectDir := filepath.Join("tmp", "shop-api")
	tests := []struct {
		name string
		info otelProcessInfo
		want string
	}{
		{
			name: "working directory with relative entrypoint",
			info: otelProcessInfo{workingDir: filepath.Join(projectDir, ".otel"), command: "node ../services/frontend/server.js"},
			want: filepath.Join(projectDir) + "  frontend",
		},
		{
			name: "framework command falls back to project name",
			info: otelProcessInfo{workingDir: projectDir, command: "next-server"},
			want: projectDir + "  shop-api",
		},
		{
			name: "command fallback without working directory",
			info: otelProcessInfo{command: "node app.js", binaryPath: "/usr/bin/node"},
			want: "node app.js",
		},
		{
			name: "binary fallback without working directory or command",
			info: otelProcessInfo{binaryPath: "/usr/bin/node"},
			want: "/usr/bin/node",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := nodeProcessDescription(tt.info); got != tt.want {
				t.Fatalf("nodeProcessDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNodeServiceNameFromCommand(t *testing.T) {
	t.Parallel()

	projectDir := filepath.Join("tmp", "shop-api")
	if got := nodeServiceNameFromCommand(projectDir, "node ../workers/payment.js"); got != "workers" {
		t.Fatalf("nodeServiceNameFromCommand() = %q, want workers", got)
	}
	if got := nodeServiceNameFromCommand(projectDir, "next-server"); !strings.Contains(got, "shop-api") {
		t.Fatalf("nodeServiceNameFromCommand() = %q, want project-derived name", got)
	}
}
