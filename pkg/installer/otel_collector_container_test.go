package installer

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/testutil"
)

// TestOtelContainerImagePattern verifies the regex that identifies OTel Collector
// container images and names across common registries, tags, and naming conventions.
func TestOtelContainerImagePattern(t *testing.T) {
	tests := []struct {
		input string
		match bool
	}{
		// Standard upstream images
		{"otel/opentelemetry-collector:0.153.0", true},
		{"otel/opentelemetry-collector:latest", true},
		{"otel/opentelemetry-collector-contrib:latest", true},
		{"docker.io/otel/opentelemetry-collector:0.153.0", true},
		// GitHub Container Registry
		{"ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:v0.100.0", true},
		// Dynatrace distribution
		{"dynatrace/dynatrace-otel-collector:latest", true},
		// Custom names
		{"custom-otel-collector", true},
		{"otel-collector-dev", true},
		{"my-org/otel-collector:1.0", true},
		// Case-insensitive
		{"OTEL/OpenTelemetry-Collector:latest", true},
		// Container names (not images)
		{"otel-collector-1", true},
		// Should NOT match — no separator between "otel" and "collector"
		{"otelcol", false},
		{"otelcol-contrib", false},
		// Unrelated images
		{"nginx:latest", false},
		{"prometheus/prometheus:v2.50.0", false},
		{"grafana/grafana:10.0.0", false},
		{"redis:7-alpine", false},
	}

	for _, tt := range tests {
		got := otelContainerImagePattern.MatchString(tt.input)
		if got != tt.match {
			t.Errorf("otelContainerImagePattern.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
		}
	}
}

// TestCollectorInstance_DisplayName_ContainerName verifies that containerName takes
// priority over the binary path basename when set.
func TestCollectorInstance_DisplayName_ContainerName(t *testing.T) {
	c := collectorInstance{
		containerRuntime: "podman",
		containerName:    "competent_shirley",
		binaryPath:       "docker.io/otel/opentelemetry-collector:0.153.0",
	}
	if got := c.displayName(); got != "competent_shirley" {
		t.Errorf("displayName() = %q, want %q", got, "competent_shirley")
	}
}

// TestCollectorInstance_DisplayName_BinaryFallback verifies that the binary basename
// is used when no container name is set.
func TestCollectorInstance_DisplayName_BinaryFallback(t *testing.T) {
	c := collectorInstance{binaryPath: "/usr/local/bin/otelcol-contrib"}
	if got := c.displayName(); got != "otelcol-contrib" {
		t.Errorf("displayName() = %q, want %q", got, "otelcol-contrib")
	}
}

// TestCollectorInstance_DisplayName_Unknown verifies the fallback when both
// containerName and binaryPath are empty.
func TestCollectorInstance_DisplayName_Unknown(t *testing.T) {
	c := collectorInstance{}
	if got := c.displayName(); got != "(unknown)" {
		t.Errorf("displayName() = %q, want %q", got, "(unknown)")
	}
}

// TestSelectCollector_ContainerDisplay verifies that selectCollector prints
// "container (<runtime>)" as the status for container-based instances and shows
// the container name as the label.
//
// Note: indented detail lines (image, config path) are printed via the color
// library which bypasses the captured pipe; only the main status line from
// fmt.Printf is asserted here.
func TestSelectCollector_ContainerDisplay(t *testing.T) {
	instances := []collectorInstance{
		{
			containerRuntime:    "podman",
			containerName:       "competent_shirley",
			binaryPath:          "docker.io/otel/opentelemetry-collector:0.153.0",
			containerConfigPath: "/etc/otelcol/config.yaml",
		},
	}

	// Feed "0\n" to stdin so selectCollector cancels immediately after printing the list.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(w, "0")
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = origStdin
		r.Close()
	}()

	output := testutil.CaptureStdout(t, func() {
		_, _ = selectCollector(instances)
	})

	if !strings.Contains(output, "container (podman)") {
		t.Errorf("expected 'container (podman)' status in output, got:\n%s", output)
	}
	if !strings.Contains(output, "competent_shirley") {
		t.Errorf("expected container name 'competent_shirley' in output, got:\n%s", output)
	}
}

// TestSelectCollector_HostMountedConfigDisplay verifies that a container with a
// host-mounted config shows the correct runtime label.
func TestSelectCollector_HostMountedConfigDisplay(t *testing.T) {
	instances := []collectorInstance{
		{
			containerRuntime: "docker",
			containerName:    "my-collector",
			binaryPath:       "otel/opentelemetry-collector:latest",
			configPath:       "/host/otel/config.yaml",
		},
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(w, "0")
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = origStdin
		r.Close()
	}()

	output := testutil.CaptureStdout(t, func() {
		_, _ = selectCollector(instances)
	})

	if !strings.Contains(output, "container (docker)") {
		t.Errorf("expected 'container (docker)' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "my-collector") {
		t.Errorf("expected container name 'my-collector' in output, got:\n%s", output)
	}
}

// TestSelectCollector_NativeProcessStatus verifies that native processes still show
// their PID in the status column (regression guard for the container status change).
func TestSelectCollector_NativeProcessStatus(t *testing.T) {
	instances := []collectorInstance{
		{
			pid:        12345,
			binaryPath: "/usr/local/bin/otelcol",
			configPath: "/etc/otel/config.yaml",
		},
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(w, "0")
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = origStdin
		r.Close()
	}()

	output := testutil.CaptureStdout(t, func() {
		_, _ = selectCollector(instances)
	})

	if !strings.Contains(output, "PID 12345") {
		t.Errorf("expected 'PID 12345' in output, got:\n%s", output)
	}
}
