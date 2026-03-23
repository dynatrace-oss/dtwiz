// Package analyzer detects the current system's platform, container runtime,
// orchestration layer, existing Dynatrace agents, cloud providers, and running services.
package analyzer

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

// runCmd executes a command and returns (success, combined output).
func runCmd(cmd string, args ...string) (bool, string) {
	c := exec.Command(cmd, args...)
	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = &buf

	// Use a timeout so a slow/hanging command doesn't block the whole analysis.
	done := make(chan error, 1)
	go func() { done <- c.Run() }()

	select {
	case err := <-done:
		return err == nil, strings.TrimSpace(buf.String())
	case <-time.After(20 * time.Second):
		if c.Process != nil {
			_ = c.Process.Kill()
		}
		return false, ""
	}
}

// Platform describes the operating system / architecture.
type Platform string

const (
	PlatformLinux   Platform = "linux"
	PlatformDarwin  Platform = "darwin"
	PlatformWindows Platform = "windows"
	PlatformUnknown Platform = "unknown"
)

// ContainerRuntime describes detected container engines.
type ContainerRuntime string

const (
	ContainerRuntimeDocker ContainerRuntime = "docker"
	ContainerRuntimeNone   ContainerRuntime = "none"
)

// Orchestrator describes the container orchestration layer.
type Orchestrator string

const (
	OrchestratorKubernetes Orchestrator = "kubernetes"
	OrchestratorNone       Orchestrator = "none"
)

// DockerInfo holds details about a detected Docker installation.
type DockerInfo struct {
	Available      bool   `json:"available"`
	ServerVersion  string `json:"server_version,omitempty"`
	RunningContainerCount int `json:"running_containers"`
}

// KubernetesInfo holds details about a detected Kubernetes cluster.
type KubernetesInfo struct {
	Available    bool   `json:"available"`
	Context      string `json:"context,omitempty"`
	Cluster      string `json:"cluster,omitempty"`
	Distribution string `json:"distribution,omitempty"`
	NodeCount    int    `json:"node_count"`
	ServerVersion string `json:"server_version,omitempty"`
}

// AWSService represents a detected AWS service and its resource count.
type AWSService struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// AWSInfo holds details about a detected AWS environment.
type AWSInfo struct {
	Available bool         `json:"available"`
	AccountID string       `json:"account_id,omitempty"`
	Region    string       `json:"region,omitempty"`
	Services  []AWSService `json:"services,omitempty"`
}

// AzureService represents a detected Azure service and its resource count.
type AzureService struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// AzureInfo holds details about a detected Azure environment.
type AzureInfo struct {
	Available         bool           `json:"available"`
	SubscriptionID    string         `json:"subscription_id,omitempty"`
	TenantID          string         `json:"tenant_id,omitempty"`
	Services          []AzureService `json:"services,omitempty"`
	ServicesAuthError bool           `json:"services_auth_error,omitempty"`
}

// SystemInfo is the top-level result of AnalyzeSystem.
type SystemInfo struct {
	Hostname         string           `json:"hostname"`
	Platform         Platform         `json:"platform"`
	Arch             string           `json:"arch"`
	ContainerRuntime ContainerRuntime `json:"container_runtime"`
	Orchestrator     Orchestrator     `json:"orchestrator"`
	Docker           *DockerInfo      `json:"docker,omitempty"`
	Kubernetes       *KubernetesInfo  `json:"kubernetes,omitempty"`
	OneAgentRunning  bool             `json:"oneagent_running"`
	OtelCollector    bool             `json:"otel_collector"`
	OtelBinaryPath   string           `json:"otel_binary_path,omitempty"`
	OtelConfigPath   string           `json:"otel_config_path,omitempty"`
	AWS              *AWSInfo         `json:"aws,omitempty"`
	Azure            *AzureInfo       `json:"azure,omitempty"`
	Services         []string         `json:"services"`
}

var (
	colorHeader = color.New(color.FgMagenta, color.Bold)
	colorMuted  = color.New()
)

const (
	labelWidth  = 18
)

func label(s string) string {
	return fmt.Sprintf("%-*s", labelWidth, s+":")
}

// Summary returns a human-readable summary of the system analysis.
func (s *SystemInfo) Summary() string {
	var sb strings.Builder

	sb.WriteString(colorHeader.Sprint("  System Analysis") + "\n")
	sb.WriteString(colorMuted.Sprint("  " + strings.Repeat("─", 42)) + "\n")

	osName := map[Platform]string{
		PlatformLinux:   "Linux",
		PlatformWindows: "Windows",
		PlatformDarwin:  "macOS",
	}[s.Platform]
	if osName == "" {
		osName = string(s.Platform)
	}
	sb.WriteString(fmt.Sprintf("  %s %s  %s  (%s)\n",
		label("Platform"), osName, s.Arch, s.Hostname))

	if s.OtelCollector {
		var line string
		if s.OtelBinaryPath != "" {
			line = s.OtelBinaryPath
			if s.OtelConfigPath != "" {
				line += "  config=" + s.OtelConfigPath
			}
			line += "  (running)"
		} else if s.OtelConfigPath != "" {
			line = s.OtelConfigPath + "  (running)"
		} else {
			line = "running"
		}
		sb.WriteString(fmt.Sprintf("  %s %s\n", label("OpenTelemetry"), line))
	} else {
		sb.WriteString(fmt.Sprintf("  %s %s\n",
			label("OpenTelemetry"),
			colorMuted.Sprint("none")))
	}

	if s.Docker != nil && s.Docker.Available {
		sb.WriteString(fmt.Sprintf("  %s version %s, %d containers running\n",
			label("Docker"),
			s.Docker.ServerVersion,
			s.Docker.RunningContainerCount))
	} else {
		sb.WriteString(fmt.Sprintf("  %s %s\n",
			label("Docker"),
			colorMuted.Sprint("none")))
	}

	if s.Kubernetes != nil && s.Kubernetes.Available {
		sb.WriteString(fmt.Sprintf("  %s dist=%s  context=%s  nodes=%d\n",
			label("Kubernetes"),
			s.Kubernetes.Distribution,
			s.Kubernetes.Context,
			s.Kubernetes.NodeCount))
	} else {
		sb.WriteString(fmt.Sprintf("  %s %s\n",
			label("Kubernetes"),
			colorMuted.Sprint("none")))
	}

	if s.AWS != nil && s.AWS.Available {
		awsLine := fmt.Sprintf("  %s account=%s  region=%s",
			label("AWS"),
			s.AWS.AccountID,
			s.AWS.Region)
		if len(s.AWS.Services) > 0 {
			parts := make([]string, len(s.AWS.Services))
			for i, svc := range s.AWS.Services {
				parts[i] = fmt.Sprintf("%s (%d)", svc.Name, svc.Count)
			}
			awsLine += "  services: " + strings.Join(parts, ", ")
		}
		sb.WriteString(awsLine + "\n")
	} else {
		sb.WriteString(fmt.Sprintf("  %s %s\n",
			label("AWS"),
			colorMuted.Sprint("none")))
	}

	if s.Azure != nil && s.Azure.Available {
		azureLine := fmt.Sprintf("  %s subscription=%s",
			label("Azure"),
			s.Azure.SubscriptionID)
		if s.Azure.ServicesAuthError {
			azureLine += "  services: MFA expired, run 'az login'"
		} else if len(s.Azure.Services) > 0 {
			parts := make([]string, len(s.Azure.Services))
			for i, svc := range s.Azure.Services {
				parts[i] = fmt.Sprintf("%s (%d)", svc.Name, svc.Count)
			}
			azureLine += "  services: " + strings.Join(parts, ", ")
		}
		sb.WriteString(azureLine + "\n")
	} else {
		sb.WriteString(fmt.Sprintf("  %s %s\n",
			label("Azure"),
			colorMuted.Sprint("none")))
	}

	sb.WriteString(fmt.Sprintf("  %s %s\n",
		label("GCP"),
		colorMuted.Sprint("none")))

	if s.OneAgentRunning {
		sb.WriteString(fmt.Sprintf("  %s running\n",
			label("OneAgent")))
	} else if s.Platform == PlatformDarwin {
		sb.WriteString(fmt.Sprintf("  %s %s\n",
			label("OneAgent"),
			colorMuted.Sprint("none")+colorMuted.Sprint(" (macOS not supported)")))
	} else {
		sb.WriteString(fmt.Sprintf("  %s %s\n",
			label("OneAgent"),
			colorMuted.Sprint("none")))
	}

	if len(s.Services) > 0 {
		sb.WriteString(fmt.Sprintf("  %s %s\n",
			label("Services"),
			strings.Join(s.Services, ", ")))
	} else {
		sb.WriteString(fmt.Sprintf("  %s %s\n",
			label("Services"),
			colorMuted.Sprint("none")))
	}

	return strings.TrimRight(sb.String(), "\n")
}

// AnalyzeSystem runs all detectors concurrently and returns a populated SystemInfo.
func AnalyzeSystem() (*SystemInfo, error) {
	info := &SystemInfo{}

	var wg sync.WaitGroup
	var mu sync.Mutex

	run := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				// Non-fatal: detectors may fail gracefully (missing binaries etc.)
				_ = err
			}
		}()
	}

	// Platform (synchronous, no subprocess needed)
	info.Hostname, _ = os.Hostname()
	info.Arch = runtime.GOARCH
	switch runtime.GOOS {
	case "linux":
		info.Platform = PlatformLinux
	case "darwin":
		info.Platform = PlatformDarwin
	case "windows":
		info.Platform = PlatformWindows
	default:
		info.Platform = PlatformUnknown
	}
	info.ContainerRuntime = ContainerRuntimeNone
	info.Orchestrator = OrchestratorNone

	run(func() error {
		d := detectDocker()
		mu.Lock()
		info.Docker = d
		if d.Available {
			info.ContainerRuntime = ContainerRuntimeDocker
		}
		mu.Unlock()
		return nil
	})

	run(func() error {
		k := detectKubernetes()
		mu.Lock()
		info.Kubernetes = k
		if k.Available {
			info.Orchestrator = OrchestratorKubernetes
		}
		mu.Unlock()
		return nil
	})

	run(func() error {
		running := detectOneAgent()
		mu.Lock()
		info.OneAgentRunning = running
		mu.Unlock()
		return nil
	})

	run(func() error {
		running, binaryPath, configPath := detectOtelCollector()
		mu.Lock()
		info.OtelCollector = running
		info.OtelBinaryPath = binaryPath
		info.OtelConfigPath = configPath
		mu.Unlock()
		return nil
	})

	run(func() error {
		a := detectAWS()
		mu.Lock()
		info.AWS = a
		mu.Unlock()
		return nil
	})

	run(func() error {
		az := detectAzure()
		mu.Lock()
		info.Azure = az
		mu.Unlock()
		return nil
	})

	run(func() error {
		svcs := detectServices()
		mu.Lock()
		info.Services = svcs
		mu.Unlock()
		return nil
	})

	wg.Wait()
	return info, nil
}
