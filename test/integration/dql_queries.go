package integration

import "fmt"

// tracesByServiceQuery returns a DQL query that fetches spans for the given service name.
func tracesByServiceQuery(serviceName string) string {
	return fmt.Sprintf(
		`fetch spans | filter service.name == "%s" | fields service.name, span_id, trace_id`,
		serviceName,
	)
}
