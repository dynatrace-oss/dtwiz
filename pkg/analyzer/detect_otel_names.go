package analyzer

// otelCollectorNames lists known OTel Collector binary name patterns.
// Used for exact process matching (pgrep -x, Get-Process) and command-line
// substring searches (pgrep -f, PowerShell CommandLine).
var otelCollectorNames = []string{
	"otelcorecol",
	"otel-collector",
	"otelcol",
	"otelcol-contrib",
	"opentelemetry-collector",
	"dynatrace-otel-collector",
}
