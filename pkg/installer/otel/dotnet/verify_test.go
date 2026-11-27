package dotnet_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/installer/otel/dotnet"
)

func checksumLine(content, name string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:]) + "  " + name + "\n"
}

func TestVerifyChecksum(t *testing.T) {
	const name = "a.zip"
	tests := []struct {
		name      string
		checksums func() string
		wantErr   error
	}{
		{"match", func() string { return checksumLine("payload", name) + "deadbeef  other.zip\n" }, nil},
		{"mismatch", func() string { return checksumLine("different", name) }, dotnet.ErrChecksumMismatch},
		{"entry missing", func() string { return checksumLine("payload", "other.zip") }, dotnet.ErrChecksumMissing},
		{"empty file", func() string { return "" }, dotnet.ErrChecksumMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			path := writeFile(t, t.TempDir(), name, "payload")

			// Act
			err := dotnet.VerifyChecksum(path, name, tt.checksums())

			// Assert
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("VerifyChecksum() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseChecksums_IgnoresMalformedLines(t *testing.T) {
	// Arrange
	text := "ABC123  a.zip\nnot a checksum line\n\nff *b.zip\n"

	// Act
	got := dotnet.ParseChecksums(text)

	// Assert
	if got["a.zip"] != "abc123" || got["b.zip"] != "ff" || len(got) != 2 {
		t.Errorf("ParseChecksums() = %v", got)
	}
}

type ghCalls struct {
	calls []string
}

func (g *ghCalls) run(failOn string, authErr error) dotnet.CommandRunner {
	return func(name string, args ...string) (string, error) {
		line := strings.Join(args, " ")
		g.calls = append(g.calls, line)
		if strings.HasPrefix(line, "auth") {
			return "", authErr
		}
		if failOn != "" && strings.HasPrefix(line, failOn) {
			return "boom", errors.New("exit status 1")
		}
		return "", nil
	}
}

func lookPath(found bool) func(string) (string, error) {
	return func(file string) (string, error) {
		if !found {
			return "", errors.New("not found")
		}
		return "/usr/bin/gh", nil
	}
}

func TestAttestationVerify_GhMissingIsSilent(t *testing.T) {
	// Arrange
	g := &ghCalls{}
	a := dotnet.Attestation{LookPath: lookPath(false), Run: g.run("", nil)}

	// Act
	err := a.Verify(filepath.Join(t.TempDir(), "a.zip"), "v1.20.0")

	// Assert
	if err != nil || len(g.calls) != 0 {
		t.Errorf("err = %v, gh calls = %v, want none", err, g.calls)
	}
}

func TestAttestationVerify_NotSignedInIsSilent(t *testing.T) {
	// Arrange
	g := &ghCalls{}
	a := dotnet.Attestation{LookPath: lookPath(true), Run: g.run("", errors.New("not logged in"))}

	// Act
	err := a.Verify("a.zip", "v1.20.0")

	// Assert
	if err != nil {
		t.Errorf("Verify() error = %v, want nil", err)
	}
	if len(g.calls) != 1 {
		t.Errorf("only the auth check should run, calls = %v", g.calls)
	}
}

func TestAttestationVerify_RunsBothChecks(t *testing.T) {
	// Arrange
	g := &ghCalls{}
	a := dotnet.Attestation{LookPath: lookPath(true), Run: g.run("", nil)}

	// Act
	err := a.Verify("a.zip", "v1.20.0")

	// Assert
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	joined := strings.Join(g.calls, "\n")
	for _, want := range []string{
		"release verify-asset v1.20.0 a.zip --repo open-telemetry/opentelemetry-dotnet-instrumentation",
		"attestation verify a.zip --repo open-telemetry/opentelemetry-dotnet-instrumentation --signer-workflow open-telemetry/opentelemetry-dotnet-instrumentation/.github/workflows/release.yml --source-ref refs/tags/v1.20.0",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing gh call %q in:\n%s", want, joined)
		}
	}
}

func TestAttestationVerify_FailureIsFinal(t *testing.T) {
	for _, failOn := range []string{"release verify-asset", "attestation verify"} {
		t.Run(failOn, func(t *testing.T) {
			// Arrange
			g := &ghCalls{}
			a := dotnet.Attestation{LookPath: lookPath(true), Run: g.run(failOn, nil)}

			// Act
			err := a.Verify("a.zip", "v1.20.0")

			// Assert
			if !errors.Is(err, dotnet.ErrAttestationFailed) {
				t.Errorf("Verify() error = %v, want ErrAttestationFailed", err)
			}
		})
	}
}
