package otel

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dynatrace-oss/dtctl/sdk/auth"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/installer"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

// portsFromConfig reads the OTLP receiver ports and internal telemetry ports from
// an existing OTel Collector config, falling back to canonical defaults for any
// field that cannot be parsed.
func portsFromConfig(configPath string) (grpc, http, metrics, healthCheck int) {
	grpc, http, metrics, healthCheck = 4317, 4318, 8888, 13133
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return
	}
	root := doc.Content[0]

	parsePort := func(endpoint string) (int, bool) {
		_, portStr, err := net.SplitHostPort(endpoint)
		if err != nil {
			return 0, false
		}
		p, err := strconv.Atoi(portStr)
		if err != nil || p <= 0 {
			return 0, false
		}
		return p, true
	}

	if ep := nodeGet(root, "receivers", "otlp", "protocols", "grpc", "endpoint"); ep != nil {
		if p, ok := parsePort(ep.Value); ok {
			grpc = p
		}
	}
	if ep := nodeGet(root, "receivers", "otlp", "protocols", "http", "endpoint"); ep != nil {
		if p, ok := parsePort(ep.Value); ok {
			http = p
		}
	}

	if readers := nodeGet(root, "service", "telemetry", "metrics", "readers"); readers != nil && readers.Kind == yaml.SequenceNode && len(readers.Content) > 0 {
		if portNode := nodeGet(readers.Content[0], "pull", "exporter", "prometheus", "port"); portNode != nil {
			if p, err := strconv.Atoi(portNode.Value); err == nil && p > 0 {
				metrics = p
			}
		}
	}

	if ep := nodeGet(root, "extensions", "health_check", "endpoint"); ep != nil {
		if p, ok := parsePort(ep.Value); ok {
			healthCheck = p
		}
	}

	return
}

// updateDynatraceCollector regenerates the Dynatrace OTel Collector config from
// the install template with new tenant credentials, preserving the existing
// receiver ports so connected app services keep their OTLP endpoint.
func updateDynatraceCollector(configPath string, runningProcs []otelProcessInfo, envURL, token, platformTok string, dryRun bool) error {
	startTime := time.Now()
	apiURL := installer.APIURL(envURL)

	origData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config file %s: %w", configPath, err)
	}

	// Preserve existing ports so connected app services keep their OTLP endpoint.
	grpcPort, httpPort, metricsPort, healthCheckPort := portsFromConfig(configPath)
	logger.Debug("existing collector ports", "grpc", grpcPort, "http", httpPort, "metrics", metricsPort, "health_check", healthCheckPort)

	hostname, _ := os.Hostname()
	cfgData := otelConfigData{
		Endpoint:    strings.TrimRight(apiURL, "/"),
		AuthHeader:  auth.AuthHeader(token),
		HostGroupID: hostname,
		GRPCPort:    grpcPort,
		HTTPPort:    httpPort,
		MetricsPort: metricsPort,
	}

	cfgData.IncludeJournald = runtime.GOOS == "linux"
	cfgData.HealthCheckPort = healthCheckPort

	freshConfig, err := renderOtelTemplate(cfgData)
	if err != nil {
		return fmt.Errorf("generating fresh collector config: %w", err)
	}

	updatedData := []byte(freshConfig)
	configChanged := !bytes.Equal(origData, updatedData)

	// When the config is already current and tenant-side prerequisites are not
	// in scope (no platform token), there is nothing to do.
	if !configChanged && platformTok == "" {
		display.StepDone("Collector configuration is up to date", "")
		return ErrUpToDate
	}

	if configChanged {
		display.Header(fmt.Sprintf("dtwiz · OpenTelemetry update — %s", configPath))
		fmt.Println()

		showConfigDiff(origData, updatedData)

		fmt.Println()
		display.PrintSectionDivider()
		fmt.Println()

		if len(runningProcs) > 0 {
			display.ColorBold.Println("  The following will be restarted:")
			fmt.Println()
			display.ColorBold.Println("  Collector")
			for _, p := range runningProcs {
				if p.containerRuntime != "" {
					fmt.Printf("    • %s  %s\n", p.containerName, display.ColorMuted.Sprint("("+p.containerRuntime+" restart)"))
				} else {
					name := filepath.Base(p.binaryPath)
					if name == "" || name == "." {
						name = "(unknown)"
					}
					fmt.Printf("    • %s (PID %d)\n", display.ColorDefault.Sprint(name), p.pid)
				}
			}
		} else {
			display.ColorDefault.Println("  No running collector found — config will be updated on disk only.")
		}

		excludePIDs := make(map[int]bool, len(runningProcs))
		for _, p := range runningProcs {
			excludePIDs[p.pid] = true
		}
		connectedSvcs := detectConnectedServices(origData, excludePIDs)
		if len(connectedSvcs) > 0 {
			fmt.Println()
			printConnectedServices(connectedSvcs)
			fmt.Println()
		}

		fmt.Println()
		display.PrintSectionDivider()
	}

	prereqs := buildTenantPrerequisitePreview(envURL, platformTok)

	if dryRun {
		display.ColorDefault.Println("  [dry-run] No changes made.")
		return nil
	}

	confirmText := "  Apply changes and restart collector?"
	if !configChanged {
		confirmText = "  Apply tenant-side prerequisite changes?"
	}

	ok, err := installer.ConfirmProceed(confirmText)
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	if !ok {
		display.ColorDefault.Println("  Cancelled — no changes written.")
		return installer.ErrInstallCancelled
	}
	fmt.Println()

	if platformTok != "" {
		activateHostMonitoringExtensionFn(envURL, platformTok)
	}

	applyGrailRoutes(prereqs.grailC, prereqs.grailPlans)
	applyOTLPMetricDimensions(context.Background(), prereqs.otlpDimensionsClient, prereqs.otlpDimensionsPlan)

	if !configChanged {
		// The config is already current, which counts as written.
		installer.RecordFeature(installer.FeatureOtelConfig, true)
		display.StepDone("Collector configuration is up to date", "")
		return nil
	}

	backupPath, err := backupFile(configPath, origData)
	if err != nil {
		return err
	}
	result, err := writeConfig(configPath, updatedData)
	if err != nil {
		return err
	}
	display.StepDone("Updated collector config", "backup created")
	display.Detail("config: %s", result.ConfigPath)
	display.Detail("backup: %s", backupPath)

	if len(runningProcs) == 0 {
		display.ColorDefault.Println("  No running collector to restart.")
		return nil
	}

	var nativeProcs, containerProcs []otelProcessInfo
	for _, p := range runningProcs {
		if p.containerRuntime != "" {
			containerProcs = append(containerProcs, p)
		} else {
			nativeProcs = append(nativeProcs, p)
		}
	}

	for _, p := range containerProcs {
		if err := display.RunStep("Restarted container "+p.containerName, func(*display.Step) error {
			if p.containerCfgPath != "" {
				if err := copyFileToContainer(p.containerRuntime, p.containerName, configPath, p.containerCfgPath); err != nil {
					return fmt.Errorf("copying config to container %s: %w", p.containerName, err)
				}
			}
			if err := restartContainer(p.containerRuntime, p.containerName); err != nil {
				return fmt.Errorf("restarting container %s: %w", p.containerName, err)
			}
			return nil
		}); err != nil {
			return err
		}
	}

	if len(nativeProcs) > 0 {
		var restartBinary string
		_ = display.RunStep("Stopped collector", func(*display.Step) error {
			restartBinary = killCollectorProcesses(nativeProcs)
			return nil
		})

		if restartBinary == "" {
			display.StepWarn("Could not determine binary path — skipping restart", "start the collector manually with the updated config")
			return nil
		}

		var crashed <-chan error
		if err := display.RunStep("Restarted collector", func(*display.Step) error {
			var serr error
			crashed, serr = startOtelCollector(restartBinary, configPath)
			if serr != nil {
				return fmt.Errorf("restarting collector: %w", serr)
			}
			return nil
		}); err != nil {
			return err
		}
		httpPort := otlpHTTPPortFromConfig(configPath)
		verified, err := verifyRestartedCollector(envURL, platformTok, token, httpPort, crashed)
		if err != nil {
			return err
		}
		if !verified {
			return nil
		}
	}

	if len(containerProcs) > 0 {
		httpPort := otlpHTTPPortFromConfig(configPath)
		// Best-effort verification for containers: httpPort may or may not be
		// exposed to the host depending on how the container was started.
		noCrash := make(chan error, 1) // never sends — no process to monitor
		verified, err := verifyRestartedCollector(envURL, platformTok, token, httpPort, noCrash)
		if err != nil {
			return err
		}
		if !verified {
			return nil
		}
	}

	installer.MarkInstallDone()
	installer.WatchIngest(envURL, platformTok, startTime.UTC().Format(installer.IngestTimeFormat))
	return nil
}
