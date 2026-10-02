package selfmonitoring

import (
	"runtime"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/installer"
)

// cmdShortMap and subShortMap abbreviate natural command names for the User-Agent header.
var cmdShortMap = map[string]string{
	"install":   "ins",
	"uninstall": "uni",
	"update":    "upd",
	"analyze":   "ana",
	"recommend": "rec",
	"status":    "sta",
	"watch":     "wch",
	"setup":     "set",
	"version":   "ver",
	"help":      "hlp",
}

// errShortMap abbreviates error taxonomy values for the User-Agent header.
// The event body always carries the full name.
var errShortMap = map[string]string{
	string(installer.ErrTypeUserCancelled):       "ucl",
	string(installer.ErrTypeAuthError):           "aut",
	string(installer.ErrTypeConfigError):         "cfg",
	string(installer.ErrTypeDependencyMissing):   "dep",
	string(installer.ErrTypeNetworkError):        "net",
	string(installer.ErrTypeInstallFailed):       "ifl",
	string(installer.ErrTypePlatformUnsupported): "plt",
}

// distroShortMap abbreviates Kubernetes distribution names for the User-Agent header.
// All codes are 3 chars to fit the 64-char capture budget alongside other fields.
// The event body always carries the full name from K8sDistro.
var distroShortMap = map[string]string{
	"GKE":              "gke",
	"EKS":              "eks",
	"AKS":              "aks",
	"IKS":              "iks",
	"OpenShift":        "ocp",
	"k3s":              "k3s",
	"RKE":              "rke",
	"kubernetes":       "k8s",
	"GKE-Autopilot":    "gka",
	"EKS-Bottlerocket": "ekb",
	"minikube":         "mnk",
	"kind":             "knd",
	"TKGI":             "tkg",
}

var subShortMap = map[string]string{
	"otel":           "otel",
	"otel-collector": "otlc",
	"otel-python":    "otlp",
	"otel-node":      "otln",
	"otel-java":      "otlj",
	"kubernetes":     "k8s",
	"oneagent":       "oa",
	"gcp":            "gcp",
	"azure":          "az",
	"aws":            "aws",
	"aws-lambda":     "awsl",
	"docker":         "dock",
	"demo":           "demo",
	"self":           "self",
	"uninstall":      "uni",
	"otel-update":    "otlu",
	"azure-update":   "azu",
	"gcp-update":     "gcpu",
}

// optShortMap abbreviates setup menu options for the User-Agent header.
var optShortMap = map[string]string{
	"oneagent":     "oa",
	"kubernetes":   "k8s",
	"docker":       "dock",
	"otel":         "otel",
	"otel-update":  "otlu",
	"aws":          "aws",
	"azure":        "az",
	"azure-update": "azu",
	"gcp":          "gcp",
	"gcp-update":   "gcpu",
	"uninstall":    "uni",
	"demo":         "demo",
}

// cloudProviderShortMap abbreviates cloud provider names for the User-Agent header.
var cloudProviderShortMap = map[string]string{
	"aws":   "aws",
	"azure": "az",
	"gcp":   "gcp",
}

func shortCmd(name string) string {
	if s, ok := cmdShortMap[name]; ok {
		return s
	}
	return name
}

func shortSub(name string) string {
	if s, ok := subShortMap[name]; ok {
		return s
	}
	return name
}

func shortOpt(name string) string {
	if s, ok := optShortMap[name]; ok {
		return s
	}
	return name
}

func shortErr(name string) string {
	if s, ok := errShortMap[name]; ok {
		return s
	}
	return name
}

func shortDistro(name string) string {
	if s, ok := distroShortMap[name]; ok {
		return s
	}
	return name
}

func shortCloudProvider(name string) string {
	parts := strings.Split(name, ",")
	for i, p := range parts {
		parts[i] = cloudProviderShortMap[p]
	}
	return strings.Join(parts, ",")
}

func shortMode(m Mode) string {
	switch m {
	case ModeDebug:
		return "deb"
	case ModeNonTTY:
		return "ntt"
	default:
		return string(m)
	}
}

func resolveOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "mac"
	case "linux":
		return "lin"
	case "windows":
		return "win"
	default:
		return runtime.GOOS
	}
}
