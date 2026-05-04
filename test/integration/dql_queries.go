package integration

import "fmt"

// tracesByServiceQuery returns a DQL query that fetches spans for the given service name.
func tracesByServiceQuery(serviceName string) string {
	return fmt.Sprintf(
		`fetch spans, from:now()-30m | filter service.name == "%s"`,
		serviceName,
	)
}
