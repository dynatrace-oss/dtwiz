package otel

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// connectedService is an application process that has an active TCP connection
// to the OTel Collector's OTLP receiver ports.
type connectedService struct {
	pid           int
	name          string   // short display name (binary basename)
	command       string   // full command line
	workDir       string   // working directory at detection time
	collectorPort string   // OTLP receiver port this service sends to (e.g. "4317" or "4318")
	listenPorts   []string // TCP ports this process itself listens on (e.g. ["8080", "8001"])
}

// receiverPortsFromConfig parses the collector YAML config and returns the
// port numbers configured for the OTLP gRPC and HTTP receivers.  Falls back
// to the standard OTLP defaults (4317 and 4318) when the config cannot be
// parsed or the endpoint fields are absent.
func receiverPortsFromConfig(data []byte) []string {
	defaults := []string{"4317", "4318"}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return defaults
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return defaults
	}
	root := doc.Content[0]

	receivers := nodeMappingGet(root, "receivers")
	if receivers == nil {
		return defaults
	}
	otlp := nodeMappingGet(receivers, "otlp")
	if otlp == nil {
		return defaults
	}
	protocols := nodeMappingGet(otlp, "protocols")
	if protocols == nil {
		return defaults
	}

	var ports []string
	for _, proto := range []string{"grpc", "http"} {
		protoNode := nodeMappingGet(protocols, proto)
		if protoNode == nil {
			continue
		}
		endpointNode := nodeMappingGet(protoNode, "endpoint")
		if endpointNode == nil {
			continue
		}
		addr := endpointNode.Value
		if idx := strings.LastIndex(addr, ":"); idx >= 0 {
			if port := addr[idx+1:]; port != "" {
				ports = append(ports, port)
				logger.Debug("receiver port from config", "proto", proto, "port", port)
			}
		}
	}
	if len(ports) == 0 {
		return defaults
	}
	return ports
}

// detectConnectedServices returns the application processes that currently have
// established TCP connections to the collector's OTLP receiver ports, as
// derived from configData.  The collector's own listening connections are
// excluded.  Returns nil when nothing is found or detection is unavailable.
func detectConnectedServices(configData []byte) []connectedService {
	ports := receiverPortsFromConfig(configData)
	return detectServicesOnPorts(ports)
}

// serviceDisplayName returns a short human-readable label for a process given
// its full command line.
func serviceDisplayName(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "(unknown)"
	}
	name := filepath.Base(fields[0])
	// For interpreter-based services (node, python, java), append the first
	// non-flag argument so the display name is more informative.
	if isInterpreter(name) && len(fields) > 1 {
		for _, arg := range fields[1:] {
			if !strings.HasPrefix(arg, "-") {
				return name + " " + filepath.Base(arg)
			}
		}
	}
	return name
}

func isInterpreter(name string) bool {
	lower := strings.ToLower(name)
	for _, interp := range []string{"node", "python", "python3", "java", "ruby", "perl"} {
		if lower == interp || strings.HasPrefix(lower, interp+".") {
			return true
		}
	}
	return false
}

// printConnectedServices prints the list of detected connected services.
func printConnectedServices(svcs []connectedService) {
	display.ColorBold.Printf("  Application services connected to this collector (%d):\n", len(svcs))
	for _, svc := range svcs {
		portHint := ""
		if svc.collectorPort != "" {
			portHint = "  " + display.ColorMuted.Sprint("(→ port "+svc.collectorPort+")")
		}
		fmt.Printf("    • PID %-6d  %s%s\n", svc.pid, display.ColorDefault.Sprint(svc.name), portHint)
		if len(svc.listenPorts) > 0 {
			display.ColorMuted.Printf("              listening on: %s\n", strings.Join(svc.listenPorts, ", "))
		}
		if svc.command != "" {
			display.ColorMuted.Printf("              %s\n", truncateStr(svc.command, 80))
		}
	}
}

// restartConnectedServices terminates each detected service process so it can
// be restarted (by a process supervisor or the user) and reconnect cleanly to
// the updated collector.  It prints the outcome for every service.
func restartConnectedServices(svcs []connectedService) {
	if len(svcs) == 0 {
		return
	}

	display.ColorBold.Printf("  Restarting %d connected service(s):\n", len(svcs))
	for _, svc := range svcs {
		portLabel := ""
		if len(svc.listenPorts) > 0 {
			portLabel = " (ports: " + strings.Join(svc.listenPorts, ", ") + ")"
		}
		fmt.Printf("    • PID %-6d  %s%s  ", svc.pid, svc.name, display.ColorMuted.Sprint(portLabel))
		if err := terminateService(svc); err != nil {
			fmt.Println(display.ColorError.Sprint("failed: " + err.Error()))
			logger.Debug("terminateService failed", "pid", svc.pid, "name", svc.name, "err", err)
		} else {
			fmt.Println(display.ColorOK.Sprint("terminated"))
		}
	}
	fmt.Println()
	display.ColorDefault.Println("  Services managed by a supervisor (systemd, launchd, etc.) will restart")
	display.ColorDefault.Println("  automatically.  Others will need to be restarted manually.")
}

// truncateStr trims s to at most n characters, appending "…" when truncated.
func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
