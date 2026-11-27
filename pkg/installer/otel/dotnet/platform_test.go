package dotnet_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

func TestSelectArchive_SupportedPlatforms(t *testing.T) {
	tests := []struct {
		name           string
		goos, arch     string
		libc           string
		wantName       string
		wantNativeDirs []string
		wantNativeFile string
	}{
		{"linux glibc x64", "linux", "amd64", "glibc", "opentelemetry-dotnet-instrumentation-linux-glibc-x64.zip", []string{"linux-x64"}, "OpenTelemetry.AutoInstrumentation.Native.so"},
		{"linux glibc arm64", "linux", "arm64", "glibc", "opentelemetry-dotnet-instrumentation-linux-glibc-arm64.zip", []string{"linux-arm64"}, "OpenTelemetry.AutoInstrumentation.Native.so"},
		{"linux musl x64", "linux", "amd64", "musl", "opentelemetry-dotnet-instrumentation-linux-musl-x64.zip", []string{"linux-musl-x64"}, "OpenTelemetry.AutoInstrumentation.Native.so"},
		{"linux musl arm64", "linux", "arm64", "musl", "opentelemetry-dotnet-instrumentation-linux-musl-arm64.zip", []string{"linux-musl-arm64"}, "OpenTelemetry.AutoInstrumentation.Native.so"},
		{"macos apple silicon", "darwin", "arm64", "", "opentelemetry-dotnet-instrumentation-macos.zip", []string{"osx-arm64"}, "OpenTelemetry.AutoInstrumentation.Native.dylib"},
		{"windows x64", "windows", "amd64", "", "opentelemetry-dotnet-instrumentation-windows.zip", []string{"win-x64", "win-x86"}, "OpenTelemetry.AutoInstrumentation.Native.dll"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := dotnet.SelectArchive(tt.goos, tt.arch, tt.libc)

			// Assert
			if err != nil {
				t.Fatalf("SelectArchive() error = %v", err)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if !reflect.DeepEqual(got.NativeDirs, tt.wantNativeDirs) {
				t.Errorf("NativeDirs = %v, want %v", got.NativeDirs, tt.wantNativeDirs)
			}
			if got.NativeFile != tt.wantNativeFile {
				t.Errorf("NativeFile = %q, want %q", got.NativeFile, tt.wantNativeFile)
			}
		})
	}
}

func TestSelectArchive_UnsupportedPlatforms(t *testing.T) {
	tests := []struct {
		name       string
		goos, arch string
		libc       string
	}{
		{"macos intel", "darwin", "amd64", ""},
		{"windows arm64", "windows", "arm64", ""},
		{"linux unknown libc", "linux", "amd64", ""},
		{"linux 386", "linux", "386", "glibc"},
		{"freebsd", "freebsd", "amd64", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			_, err := dotnet.SelectArchive(tt.goos, tt.arch, tt.libc)

			// Assert
			var unsupported *dotnet.UnsupportedPlatformError
			if !errors.As(err, &unsupported) {
				t.Fatalf("error = %v, want *UnsupportedPlatformError", err)
			}
			if got := err.Error(); !containsAll(got, "not supported", "macOS", "Windows", "Linux") {
				t.Errorf("message should name the supported platforms, got %q", got)
			}
		})
	}
}

func TestDetectLibc(t *testing.T) {
	tests := []struct {
		name string
		out  string
		err  error
		want string
	}{
		{"glibc", "\tlinux-vdso.so.1 (0x00007ffc)\n\tlibc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007f)\n", nil, "glibc"},
		{"musl exits non-zero", "\t/lib/ld-musl-x86_64.so.1 (0x7f)\n\tlibc.musl-x86_64.so.1 => /lib/ld-musl-x86_64.so.1\n", errors.New("exit status 1"), "musl"},
		{"static binary", "\tnot a dynamic executable\n", errors.New("exit status 1"), ""},
		{"ldd missing", "", errors.New("not found"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			run := func(name string, args ...string) (string, error) { return tt.out, tt.err }

			// Act
			got := dotnet.DetectLibc(run)

			// Assert
			if got != tt.want {
				t.Errorf("DetectLibc() = %q, want %q", got, tt.want)
			}
		})
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
