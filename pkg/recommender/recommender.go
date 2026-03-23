// Package recommender generates ranked Dynatrace ingestion recommendations
// based on the system analysis produced by the analyzer package.
package recommender

import (
	"fmt"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/fatih/color"
)

// IngestMethod identifies a Dynatrace ingestion approach.
type IngestMethod string

const (
	MethodOneAgent         IngestMethod = "oneagent"
	MethodKubernetes       IngestMethod = "kubernetes"
	MethodDocker           IngestMethod = "docker"
	MethodOtelCollector    IngestMethod = "otel"
	MethodOtelUpdate       IngestMethod = "otel-update"
	MethodAWS              IngestMethod = "aws"
	MethodAlreadyInstalled IngestMethod = "already-installed"
	MethodNotSupported     IngestMethod = "not-supported"
)

// Recommendation describes a single suggested ingestion method.
type Recommendation struct {
	Method        IngestMethod `json:"method"`
	Priority      int          `json:"priority"`
	Title         string       `json:"title"`
	Description   string       `json:"description"`
	Prerequisites []string     `json:"prerequisites,omitempty"`
	Steps         []string     `json:"steps,omitempty"`
	Done          bool         `json:"done,omitempty"`
	// ConfigPath carries the detected config file path for methods that need
	// it (e.g. MethodOtelUpdate).  Empty when not relevant.
	ConfigPath string `json:"config_path,omitempty"`
}

// GenerateRecommendations returns a ranked list of recommendations based on
// the given system analysis.  The list is ordered from highest to lowest
// priority.
func GenerateRecommendations(system *analyzer.SystemInfo) []Recommendation {
	var recs []Recommendation

	// 1. OneAgent already running — nothing to do.
	if system.OneAgentRunning {
		recs = append(recs, Recommendation{
			Method:      MethodAlreadyInstalled,
			Priority:    0,
			Title:       "Dynatrace OneAgent is already running",
			Description: "OneAgent is detected on this host.  No additional installation is needed.",
			Done:        true,
		})
		return recs
	}

	// 2. OTel Collector found → configure existing exporter (highest priority).
	if system.OtelCollector {
		configHint := ""
		if system.OtelConfigPath != "" {
			configHint = fmt.Sprintf(" (config: %s)", system.OtelConfigPath)
		}
		recs = append(recs, Recommendation{
			Method:   MethodOtelUpdate,
			Priority: 0,
			Title:    "Configure existing OpenTelemetry Collector",
			Description: fmt.Sprintf(
				"An OpenTelemetry Collector is running%s. Add the Dynatrace OTLP exporter to send telemetry to Dynatrace.",
				configHint,
			),
			Prerequisites: []string{"Access to OTel Collector configuration"},
			Steps: []string{
				"dtwiz update otel",
			},
			ConfigPath: system.OtelConfigPath,
		})
	}

	// 3. Always offer installing a new OTel Collector (even if one is already
	//    running — the user may want a separate Dynatrace-managed collector).
	recs = append(recs, Recommendation{
		Method:   MethodOtelCollector,
		Priority: 0,
		Title:    "Install new OpenTelemetry Collector and instrument apps",
		Description: "Deploy the Dynatrace OpenTelemetry Collector to ingest traces, metrics, and logs via OTLP.",
		Prerequisites: []string{"Dynatrace API token with ingest scopes"},
		Steps: []string{
			"dtwiz install otel",
		},
	})

	// 4. Kubernetes → Dynatrace Operator.
	if system.Orchestrator == analyzer.OrchestratorKubernetes && system.Kubernetes != nil && system.Kubernetes.Available {
		recs = append(recs, Recommendation{
			Method:   MethodKubernetes,
			Priority: 10,
			Title:    "Deploy Dynatrace Operator on Kubernetes",
			Description: fmt.Sprintf(
				"A Kubernetes cluster (%s) is detected. The Dynatrace Operator provides full-stack observability for all workloads.",
				system.Kubernetes.Distribution,
			),
			Prerequisites: []string{"kubectl access to the cluster", "Dynatrace API token with required scopes"},
			Steps: []string{
				"dtwiz install kubernetes",
			},
		})
	}

	// 5. Docker without Kubernetes → Docker OneAgent.
	if system.ContainerRuntime == analyzer.ContainerRuntimeDocker &&
		system.Orchestrator != analyzer.OrchestratorKubernetes {
		recs = append(recs, Recommendation{
			Method:      MethodDocker,
			Priority:    20,
			Title:       "Install Dynatrace OneAgent for Docker",
			Description: "Docker is running without Kubernetes orchestration. Deploy OneAgent as a container for host + container monitoring.",
			Prerequisites: []string{"Docker daemon access", "Dynatrace API token"},
			Steps: []string{
				"dtwiz install docker",
			},
		})
	}

	// 6. Bare metal / VM (Linux or Windows, no containers) → host OneAgent.
	if system.ContainerRuntime == analyzer.ContainerRuntimeNone &&
		system.Orchestrator == analyzer.OrchestratorNone &&
		(system.Platform == analyzer.PlatformLinux || system.Platform == analyzer.PlatformWindows) {
		recs = append(recs, Recommendation{
			Method:      MethodOneAgent,
			Priority:    40,
			Title:       "Install Dynatrace OneAgent on this host",
			Description: "No container runtime detected. Install OneAgent directly for full-stack host monitoring.",
			Prerequisites: []string{"Root/Administrator privileges", "Dynatrace API token"},
			Steps: []string{
				"dtwiz install oneagent",
			},
		})
	}

	// 7. AWS detected → CloudFormation integration.
	if system.AWS != nil && system.AWS.Available {
		recs = append(recs, Recommendation{
			Method:      MethodAWS,
			Priority:    50,
			Title:       "Set up Dynatrace AWS CloudFormation integration",
			Description: fmt.Sprintf("AWS credentials detected (account: %s). Deploy the Dynatrace ActiveGate via CloudFormation for cloud-level monitoring.", system.AWS.AccountID),
			Prerequisites: []string{"AWS CLI with sufficient permissions", "Dynatrace API token"},
			Steps: []string{
				"dtwiz install aws",
			},
		})
	}

	return recs
}

var (
	recHeader      = color.New(color.FgMagenta, color.Bold)
	recTitleDone   = color.New(color.FgGreen, color.Bold)
	recTitleActive = color.New()
	recTitleWarn   = color.New(color.FgYellow, color.Bold)
	recMuted       = color.New()
	recBadgeDone   = color.New(color.FgGreen, color.Bold)
	recBadgeNum    = color.New(color.FgMagenta, color.Bold)
	recBadgeWarn   = color.New(color.FgYellow, color.Bold)
)

// FormatRecommendations returns a human-readable string of recommendations.
func FormatRecommendations(recs []Recommendation) string {
	if len(recs) == 0 {
		return recMuted.Sprint("No recommendations generated.")
	}

	var sb strings.Builder
	sb.WriteString(recHeader.Sprint("  Recommendations") + "\n")
	sb.WriteString(recMuted.Sprint("  "+strings.Repeat("─", 42)) + "\n\n")

	for i, r := range recs {
		if r.Done {
			badge := recBadgeDone.Sprint(" ✓ ")
			title := recTitleDone.Sprint(r.Title)
			sb.WriteString(fmt.Sprintf("  %s  %s\n", badge, title))
		} else if r.Method == MethodNotSupported {
			badge := recBadgeWarn.Sprint(" ! ")
			title := recTitleWarn.Sprint(r.Title)
			sb.WriteString(fmt.Sprintf("  %s  %s\n", badge, title))
		} else {
			badge := recBadgeNum.Sprintf(" %d ", i+1)
			title := recTitleActive.Sprint(r.Title)
			install := recMuted.Sprintf("dtwiz install %s", r.Method)
			sb.WriteString(fmt.Sprintf("  %s  %s  →  %s\n", badge, title, install))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}
