package dotnet_test

import (
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

func TestActivationEnv(t *testing.T) {
	root := filepath.Join("home", "u", ".opentelemetry", "dotnet")
	hook := filepath.Join(root, "net", "OpenTelemetry.AutoInstrumentation.StartupHook.dll")
	common := func(extra map[string]string) map[string]string {
		env := map[string]string{
			"OTEL_DOTNET_AUTO_HOME":    root,
			"DOTNET_STARTUP_HOOKS":     hook,
			"CORECLR_ENABLE_PROFILING": "1",
			"CORECLR_PROFILER":         "{918728DD-259F-4A6A-AC2B-B85E1B658318}",
		}
		for k, v := range extra {
			env[k] = v
		}
		return env
	}
	tests := []struct {
		name             string
		goos, arch, libc string
		want             map[string]string
	}{
		{"linux glibc x64", "linux", "amd64", "glibc",
			common(map[string]string{"CORECLR_PROFILER_PATH": filepath.Join(root, "linux-x64", "OpenTelemetry.AutoInstrumentation.Native.so")})},
		{"linux musl arm64", "linux", "arm64", "musl",
			common(map[string]string{"CORECLR_PROFILER_PATH": filepath.Join(root, "linux-musl-arm64", "OpenTelemetry.AutoInstrumentation.Native.so")})},
		{"macos", "darwin", "arm64", "",
			common(map[string]string{"CORECLR_PROFILER_PATH": filepath.Join(root, "osx-arm64", "OpenTelemetry.AutoInstrumentation.Native.dylib")})},
		{"windows", "windows", "amd64", "",
			common(map[string]string{
				"CORECLR_PROFILER_PATH_64": filepath.Join(root, "win-x64", "OpenTelemetry.AutoInstrumentation.Native.dll"),
				"CORECLR_PROFILER_PATH_32": filepath.Join(root, "win-x86", "OpenTelemetry.AutoInstrumentation.Native.dll"),
			})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			archive, err := dotnet.SelectArchive(tt.goos, tt.arch, tt.libc)
			if err != nil {
				t.Fatal(err)
			}

			// Act
			got := dotnet.ActivationEnv(root, archive)

			// Assert
			if len(got) != len(tt.want) {
				t.Errorf("got %d variables %v, want %d %v", len(got), got, len(tt.want), tt.want)
			}
			for k, want := range tt.want {
				if got[k] != want {
					t.Errorf("%s = %q, want %q", k, got[k], want)
				}
			}
		})
	}
}
