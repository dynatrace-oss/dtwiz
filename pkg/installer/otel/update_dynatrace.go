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

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
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

	cfgData := otelConfigData{
		Endpoint:    strings.TrimRight(apiURL, "/"),
		AuthHeader:  installer.AuthHeader(token),
		GRPCPort:    grpcPort,
		HTTPPort:    httpPort,
		MetricsPort: metricsPort,
	}

	experimentalEnabled := featureflags.IsEnabled(featureflags.Experimental)
	if experimentalEnabled {
		cfgData.HostMonitoring = true
		cfgData.IncludeJournald = runtime.GOOS == "linux"
		cfgData.HealthCheckPort = healthCheckPort
	}

	freshConfig, err := renderOtelTemplate(cfgData)
	if err != nil {
		return fmt.Errorf("generating fresh collector config: %w", err)
	}

	updatedData := []byte(freshConfig)
	configChanged := !bytes.Equal(origData, updatedData)

	// When the config is already current and tenant-side prerequisites are not
	// in scope (no experimental flag or no platform token), there is nothing to do.
	if !configChanged && !(experimentalEnabled && platformTok != "") {
		display.ColorOK.Println("  Collector configuration is up to date.")
		return ErrUpToDate
	}

	if configChanged {
		display.Header(fmt.Sprintf("Preview: collector configuration changes to %s:", configPath))
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

	if experimentalEnabled && platformTok != "" {
		if status, err := buildExtensionActivationPreviewFn(envURL, platformTok); err != nil {
			fmt.Println()
			display.PrintWarning("OTel Host Monitoring extension", err)
		} else {
			fmt.Println()
			printExtensionActivationPreview(status)
		}
	}

	var grailC grailRouteClient
	var grailPlans []grailSignalPlan
	if experimentalEnabled && platformTok != "" {
		if c, plans, err := buildGrailRoutePlansFn(envURL, platformTok); err != nil {
			fmt.Println()
			display.PrintWarning("OpenPipeline routes", err)
		} else {
			grailC = c
			grailPlans = plans
			fmt.Println()
			printGrailPlan(plans)
		}
	}

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

	if experimentalEnabled && platformTok != "" {
		activateHostMonitoringExtensionFn(envURL, platformTok)
	}

	if grailC != nil {
		if err := waitForGrailPipelinesFn(context.Background(), grailC, time.Sleep); err != nil {
			logger.Debug("OTel host-monitoring pipelines did not appear within the wait bound", "error", err)
		}
		if freshPlans, err := buildGrailPlans(context.Background(), grailC); err != nil {
			logger.Debug("failed to rebuild Grail route plans after extension activation, applying preview snapshot", "error", err)
		} else {
			grailPlans = freshPlans
		}
		logger.Debug("applying Grail route plans", "count", len(grailPlans))
		applyErrs := make([]error, len(grailPlans))
		for i, p := range grailPlans {
			applyErrs[i] = applyGrailPlan(context.Background(), grailC, p)
		}
		printGrailApplyResults(grailPlans, applyErrs)
	}

	if !configChanged {
		display.ColorOK.Println("  Collector configuration is up to date.")
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
	fmt.Printf("  Config updated: %s\n", result.ConfigPath)
	fmt.Printf("  Backup created: %s\n", backupPath)
	fmt.Println()

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
		if p.containerCfgPath != "" {
			fmt.Printf("  Copying updated config into container %s...\n", p.containerName)
			if err := copyFileToContainer(p.containerRuntime, p.containerName, configPath, p.containerCfgPath); err != nil {
				return fmt.Errorf("copying config to container %s: %w", p.containerName, err)
			}
		}
		fmt.Printf("  Restarting container %s...\n", p.containerName)
		if err := restartContainer(p.containerRuntime, p.containerName); err != nil {
			return fmt.Errorf("restarting container %s: %w", p.containerName, err)
		}
		fmt.Printf("  Container %s restarted.\n", p.containerName)
	}

	if len(nativeProcs) > 0 {
		restartBinary := killCollectorProcesses(nativeProcs)
		fmt.Println()

		if restartBinary == "" {
			display.ColorDefault.Println("  Could not determine binary path — skipping restart.")
			display.ColorDefault.Println("  Start the collector manually with the updated config.")
			return nil
		}

		fmt.Printf("  Restarting collector with updated config...\n")
		crashed, err := startOtelCollector(restartBinary, configPath)
		if err != nil {
			return fmt.Errorf("restarting collector: %w", err)
		}
		if err := verifyOtelInstall(envURL, platformTok, token, otlpHTTPPortFromConfig(configPath), crashed); err != nil {
			fmt.Printf("\n  Warning: log verification failed: %v\n", err)
			fmt.Println("  The collector may still be working — check the Dynatrace UI.")
			return nil
		}
	}

	if len(containerProcs) > 0 {
		noCrash := make(chan error)
		if err := verifyOtelInstall(envURL, platformTok, token, otlpHTTPPortFromConfig(configPath), noCrash); err != nil {
			fmt.Printf("\n  Warning: log verification failed: %v\n", err)
			fmt.Println("  The collector may still be working — check the Dynatrace UI.")
			return nil
		}
	}

	display.ColorOK.Println("  ✓ Collector restarted and verified.")

	installer.WatchIngest(envURL, platformTok, startTime.UTC().Format(installer.IngestTimeFormat))
	return nil
}
