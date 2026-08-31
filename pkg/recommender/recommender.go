// Package recommender generates ranked Dynatrace ingestion recommendations
// based on the system analysis produced by the analyzer package.
package recommender

import (
	"fmt"
	"strings"

	"github.com/fatih/color"

	"github.com/dynatrace-oss/dtwiz/pkg/analyzer"
	"github.com/dynatrace-oss/dtwiz/pkg/featureflags"
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
	MethodAzure            IngestMethod = "azure"
	MethodAzureUpdate      IngestMethod = "azure-update"
	MethodGCP              IngestMethod = "gcp"
	MethodGCPUpdate        IngestMethod = "gcp-update"
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
	ComingSoon    bool         `json:"coming_soon,omitempty"`
	// ConfigPath carries the detected config file path for methods that need
	// it (e.g. MethodOtelUpdate).  Empty when not relevant.
	ConfigPath string `json:"config_path,omitempty"`
	// DetectionInfo is a short inline summary of what was found (or not found)
	// for this recommendation — shown next to the title in the setup menu.
	DetectionInfo string `json:"detection_info,omitempty"`
	// Unavailable marks recommendations whose prerequisite was not detected.
	// They are rendered greyed and not numbered in the setup menu.
	Unavailable bool `json:"unavailable,omitempty"`
	// ShortTitle is a compact label used in the "Sign in to unlock" section
	// (e.g. "AWS" instead of "AWS cloud services").
	ShortTitle string `json:"short_title,omitempty"`
	// UnlockCommand is the CLI command the user should run to unlock this
	// unavailable recommendation (e.g. "aws configure").
	UnlockCommand string `json:"unlock_command,omitempty"`
}

// platformName returns a human-readable OS name for DetectionInfo labels.
func platformName(p analyzer.Platform) string {
	switch p {
	case analyzer.PlatformDarwin:
		return "macOS"
	case analyzer.PlatformLinux:
		return "Linux"
	case analyzer.PlatformWindows:
		return "Windows"
	default:
		return string(p)
	}
}

// hostDetectionInfo builds the inline detection summary for the OtelCollector
// (new install) option: hostname + OS/arch + current directory + project techs.
func hostDetectionInfo(system *analyzer.SystemInfo) string {
	hostname := system.Hostname
	if hostname == "" {
		hostname = "this host"
	}
	info := fmt.Sprintf("%s (%s %s)", hostname, platformName(system.Platform), system.Arch)
	if len(system.ProjectTechs) > 0 {
		var parts []string
		for _, t := range system.ProjectTechs {
			parts = append(parts, fmt.Sprintf("%s (%s)", t.Name, t.Path))
		}
		info += " · " + strings.Join(parts, " · ")
	}
	return info
}

// GenerateRecommendations returns a ranked list of recommendations based on
// the given system analysis.  The list is ordered from highest to lowest
// priority.  Recommendations for undetected infrastructure are appended at the
// end with Unavailable=true so the setup menu can render them greyed.
func GenerateRecommendations(system *analyzer.SystemInfo) []Recommendation {
	var recs []Recommendation

	// 1. OneAgent already running — mark as done but continue with other recommendations.
	if system.OneAgentRunning {
		recs = append(recs, Recommendation{
			Method:      MethodAlreadyInstalled,
			Priority:    0,
			Title:       "Dynatrace OneAgent is already running",
			Description: "OneAgent is detected on this host.  No additional installation is needed.",
			Done:        true,
		})
	}

	// 2. OTel Collector found → configure existing exporter (highest priority).
	if system.OtelCollector {
		configHint := ""
		detInfo := "running"
		if system.OtelBinaryPath != "" {
			detInfo = "running: " + system.OtelBinaryPath
		}
		if system.OtelConfigPath != "" {
			configHint = fmt.Sprintf(" (config: %s)", system.OtelConfigPath)
		}
		recs = append(recs, Recommendation{
			Method:   MethodOtelUpdate,
			Priority: 0,
			Title:    "This host and its services (via existing OpenTelemetry Collector)",
			Description: fmt.Sprintf(
				"An OpenTelemetry Collector is running%s. Add the Dynatrace OTLP exporter to send telemetry to Dynatrace.",
				configHint,
			),
			Prerequisites: []string{"Access to OTel Collector configuration"},
			Steps: []string{
				"dtwiz update otel",
			},
			ConfigPath:    system.OtelConfigPath,
			DetectionInfo: detInfo,
		})
	}

	// 3. Always offer installing a new OTel Collector (even if one is already
	//    running — the user may want a separate Dynatrace-managed collector).
	recs = append(recs, Recommendation{
		Method:        MethodOtelCollector,
		Priority:      0,
		Title:         "This host and its services (via new OpenTelemetry Collector)",
		Description:   "Deploy the Dynatrace OpenTelemetry Collector to ingest traces, metrics, and logs via OTLP.",
		Prerequisites: []string{"Dynatrace API token with ingest scopes"},
		Steps: []string{
			"dtwiz install otel",
		},
		DetectionInfo: hostDetectionInfo(system),
	})

	// 4. Kubernetes → Dynatrace Operator.
	k8sDetected := system.Orchestrator == analyzer.OrchestratorKubernetes && system.Kubernetes != nil && system.Kubernetes.Available
	if k8sDetected {
		detInfo := system.Kubernetes.Distribution
		if system.Kubernetes.Cluster != "" {
			detInfo = fmt.Sprintf("%s: %s (%d nodes)", system.Kubernetes.Distribution, system.Kubernetes.Cluster, system.Kubernetes.NodeCount)
		}
		recs = append(recs, Recommendation{
			Method:   MethodKubernetes,
			Priority: 10,
			Title:    "Kubernetes cluster",
			Description: fmt.Sprintf(
				"A Kubernetes cluster (%s) is detected. The Dynatrace Operator provides full-stack observability for all workloads.",
				system.Kubernetes.Distribution,
			),
			Prerequisites: []string{"kubectl access to the cluster", "Dynatrace API token with required scopes"},
			Steps: []string{
				"dtwiz install kubernetes",
			},
			DetectionInfo: detInfo,
		})
	}

	// 5. Docker without Kubernetes → Docker OneAgent (experimental).
	if featureflags.IsEnabled(featureflags.Experimental) &&
		system.ContainerRuntime == analyzer.ContainerRuntimeDocker &&
		system.Orchestrator != analyzer.OrchestratorKubernetes {
		recs = append(recs, Recommendation{
			Method:        MethodDocker,
			Priority:      20,
			Title:         "Docker host + containers (via OneAgent)",
			Description:   "Docker is running without Kubernetes orchestration. Deploy OneAgent as a container for host + container monitoring.",
			Prerequisites: []string{"Docker daemon access", "Dynatrace API token"},
			Steps: []string{
				"dtwiz install docker",
			},
		})
	}

	// 6. Bare metal / VM (Linux or Windows, no containers) → host OneAgent.
	if !system.OneAgentRunning &&
		system.ContainerRuntime == analyzer.ContainerRuntimeNone &&
		system.Orchestrator == analyzer.OrchestratorNone &&
		(system.Platform == analyzer.PlatformLinux || system.Platform == analyzer.PlatformWindows) {
		recs = append(recs, Recommendation{
			Method:        MethodOneAgent,
			Priority:      40,
			Title:         "This host and its services (via OneAgent)",
			Description:   "No container runtime detected. Install OneAgent directly for full-stack host monitoring.",
			Prerequisites: []string{"Root/Administrator privileges", "Dynatrace API token"},
			Steps: []string{
				"dtwiz install oneagent",
			},
			DetectionInfo: hostDetectionInfo(system),
		})
	}

	// 7. AWS detected → CloudFormation integration.
	awsDetected := system.AWS != nil && system.AWS.Available
	if awsDetected {
		detInfo := "account: " + system.AWS.AccountID
		if system.AWS.Region != "" {
			detInfo += ", " + system.AWS.Region
		}
		recs = append(recs, Recommendation{
			Method:        MethodAWS,
			Priority:      50,
			Title:         "AWS cloud services",
			Description:   fmt.Sprintf("AWS credentials detected (account: %s). Deploy the Dynatrace ActiveGate via CloudFormation for cloud-level monitoring.", system.AWS.AccountID),
			Prerequisites: []string{"AWS CLI with sufficient permissions", "Dynatrace API token"},
			Steps: []string{
				"dtwiz install aws",
			},
			DetectionInfo: detInfo,
		})
	}

	// 8. Azure detected.
	if system.AzureDetected() {
		method := MethodAzure
		title := "Azure cloud services"
		if system.AzureConfigured {
			method = MethodAzureUpdate
			title = "Azure cloud services (update)"
		}
		recs = append(recs, Recommendation{
			Method:        method,
			Priority:      50,
			Title:         title,
			Description:   fmt.Sprintf("Azure subscription detected (%s).", system.Azure.SubscriptionID),
			DetectionInfo: "subscription: " + system.Azure.SubscriptionID,
		})
	}

	// 9. GCP detected.
	if system.GCPDetected() {
		method := MethodGCP
		title := "GCP cloud services"
		if system.GCPConfigured {
			method = MethodGCPUpdate
			title = "GCP cloud services (update)"
		}
		recs = append(recs, Recommendation{
			Method:        method,
			Priority:      50,
			Title:         title,
			Description:   fmt.Sprintf("GCP project detected (%s).", system.GCP.ProjectID),
			DetectionInfo: "project: " + system.GCP.ProjectID,
		})
	}

	// Unavailable entries — shown greyed in the setup menu so users know what
	// monitoring they can unlock by signing in or connecting infrastructure.
	if !k8sDetected {
		recs = append(recs, Recommendation{
			Method:        MethodKubernetes,
			Priority:      100,
			Title:         "Kubernetes cluster",
			ShortTitle:    "Kubernetes",
			DetectionInfo: "not detected",
			UnlockCommand: "kubectl config use-context <name>",
			Unavailable:   true,
		})
	}
	if !awsDetected {
		recs = append(recs, Recommendation{
			Method:        MethodAWS,
			Priority:      100,
			Title:         "AWS cloud services",
			ShortTitle:    "AWS",
			DetectionInfo: "not signed in",
			UnlockCommand: "aws configure",
			Unavailable:   true,
		})
	}
	if !system.AzureDetected() {
		recs = append(recs, Recommendation{
			Method:        MethodAzure,
			Priority:      100,
			Title:         "Azure cloud services",
			ShortTitle:    "Azure",
			DetectionInfo: "not signed in",
			UnlockCommand: "az login",
			Unavailable:   true,
		})
	}
	if !system.GCPDetected() {
		recs = append(recs, Recommendation{
			Method:        MethodGCP,
			Priority:      100,
			Title:         "GCP cloud services",
			ShortTitle:    "GCP",
			DetectionInfo: "not signed in",
			UnlockCommand: "gcloud auth login",
			Unavailable:   true,
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
	recFaint       = color.New(color.Faint)
)

// FormatRecommendations returns a human-readable string of recommendations.
func FormatRecommendations(recs []Recommendation) string {
	if len(recs) == 0 {
		return recMuted.Sprint("No recommendations generated.")
	}

	var sb strings.Builder
	sb.WriteString(recHeader.Sprint("  Recommendations — What do you want to monitor?") + "\n")
	sb.WriteString(recMuted.Sprint("  "+strings.Repeat("─", 42)) + "\n")
	sb.WriteString("  Monitor Logs, Metrics, Traces of:\n\n")

	n := 0
	for _, r := range recs {
		if r.Unavailable {
			continue
		}
		switch {
		case r.Done:
			badge := recBadgeDone.Sprint(" ✓ ")
			title := recTitleDone.Sprint(r.Title)
			sb.WriteString(fmt.Sprintf("  %s  %s\n", badge, title))
		case r.Method == MethodNotSupported:
			badge := recBadgeWarn.Sprint(" ! ")
			title := recTitleWarn.Sprint(r.Title)
			sb.WriteString(fmt.Sprintf("  %s  %s\n", badge, title))
		case r.ComingSoon:
			// Coming-soon items are shown muted without a number.
			bullet := recMuted.Sprint(" · ")
			title := recMuted.Sprint(r.Title)
			sb.WriteString(fmt.Sprintf("  %s  %s\n", bullet, title))
		default:
			n++
			badge := recBadgeNum.Sprintf(" %d ", n)
			title := recTitleActive.Sprint(r.Title)
			sb.WriteString(fmt.Sprintf("  %s  %s\n", badge, title))
		}
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %s  %s\n", recMuted.Sprint("[d]"), recMuted.Sprint("Install demo app (schnitzel)")))
	return strings.TrimRight(sb.String(), "\n")
}

// ActionableItems returns the subset of recommendations that are numbered and
// selectable in the setup menu — excluding done, not-supported, coming-soon,
// unavailable, and experimental-gated entries.
func ActionableItems(recs []Recommendation, experimental bool) []Recommendation {
	var out []Recommendation
	for _, r := range recs {
		if r.Unavailable || r.Done || r.Method == MethodNotSupported || r.ComingSoon {
			continue
		}
		if !experimental && (r.Method == MethodDocker || r.Method == MethodOtelUpdate) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// FormatSetupMenu returns the formatted recommendation menu for the interactive
// setup command: done items, numbered actionable items with detection info,
// coming-soon items, the "Sign in to unlock" section, and the demo option.
func FormatSetupMenu(recs []Recommendation, demoRunning bool, experimental bool) string {
	var sb strings.Builder

	// Done items (e.g. OneAgent already running).
	for _, r := range recs {
		if r.Done {
			sb.WriteString(fmt.Sprintf("  %s  %s\n", recBadgeDone.Sprint(" ✓ "), recTitleDone.Sprint(r.Title)))
			ctx := r.DetectionInfo
			if ctx == "" {
				ctx = r.Description
			}
			if ctx != "" {
				sb.WriteString(fmt.Sprintf("         %s\n", recFaint.Sprint(ctx)))
			}
		}
	}

	// Numbered selectable items with detection info on a second line.
	for i, r := range ActionableItems(recs, experimental) {
		sb.WriteString(fmt.Sprintf("  %s  %s\n", recBadgeNum.Sprintf("[%d]", i+1), recTitleActive.Sprint(r.Title)))
		if r.DetectionInfo != "" {
			sb.WriteString(fmt.Sprintf("         %s\n", recFaint.Sprint(r.DetectionInfo)))
		}
	}

	// Coming-soon items (informational only, not selectable).
	for _, r := range recs {
		if r.ComingSoon {
			sb.WriteString(fmt.Sprintf("  %s  %s\n", recFaint.Sprint(" · "), recFaint.Sprint(r.Title)))
		}
	}

	// Unavailable items: compact "Sign in to unlock" section.
	var unavailable []Recommendation
	for _, r := range recs {
		if r.Unavailable {
			unavailable = append(unavailable, r)
		}
	}
	if len(unavailable) > 0 {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("  %s\n", recFaint.Sprint("Sign in to unlock:")))
		maxLen := 0
		for _, r := range unavailable {
			if l := len(r.ShortTitle); l > maxLen {
				maxLen = l
			}
		}
		for _, r := range unavailable {
			sb.WriteString(fmt.Sprintf("   %s  %s\n",
				recFaint.Sprint("·"),
				recFaint.Sprintf("%-*s  — run: %s", maxLen, r.ShortTitle, r.UnlockCommand),
			))
		}
	}

	// Demo option.
	if !demoRunning {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("  %s  %s\n", recMuted.Sprint("[d]"), recMuted.Sprint("Install demo app (schnitzel)")))
	}

	return sb.String()
}
