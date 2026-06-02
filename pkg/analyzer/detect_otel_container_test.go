package analyzer

import "testing"

func TestOtelContainerPattern(t *testing.T) {
	tests := []struct {
		input string
		match bool
	}{
		// image-based matches
		{"otel/opentelemetry-collector:0.153.0", true},
		{"otel/opentelemetry-collector-contrib:latest", true},
		{"ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector:v0.100.0", true},
		{"dynatrace/dynatrace-otel-collector:latest", true},
		{"custom-otel-collector", true},
		// container name-based matches
		{"otelcollector", false},     // no separator between otel and collector
		{"otel-collector-dev", true}, // "otel" then "collector" with separator
		{"opentelemetry-collector", true},
		// non-matching
		{"otelcol", false},
		{"nginx:latest", false},
		{"prometheus/prometheus", false},
		{"grafana/grafana", false},
		// case-insensitive
		{"OTEL/OpenTelemetry-Collector:latest", true},
	}

	for _, tt := range tests {
		got := otelContainerPattern.MatchString(tt.input)
		if got != tt.match {
			t.Errorf("otelContainerPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
		}
	}
}
