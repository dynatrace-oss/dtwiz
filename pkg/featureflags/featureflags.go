package featureflags

import (
	"os"
	"strings"
	"sync"

	"github.com/spf13/pflag"
)

// Flag identifies a feature flag.
type Flag int

const (
	AllRuntimes Flag = iota
)

// CLIFeatureFlag defines a single feature flag with its metadata.
type CLIFeatureFlag struct {
	flag       Flag
	name       string // kebab-case, used as cobra flag name: --all-runtimes
	envVar     string // env var name: DTWIZ_ALL_RUNTIMES
	defaultVal bool   // default value
	desc       string // cobra flag description
	bound      bool   // bound variable for cobra BoolVar
}

var registry = []CLIFeatureFlag{
	{AllRuntimes, "all-runtimes", "DTWIZ_ALL_RUNTIMES", false, "enable all runtimes including experimental (Java, Node.js, Go)", false},
}

var (
	cliOverrides = map[Flag]bool{}
	mu           sync.Mutex
)

// FlagState represents the resolved state of a feature flag.
type FlagState struct {
	Name    string // "all-runtimes"
	EnvVar  string // "DTWIZ_ALL_RUNTIMES"
	Enabled bool   // resolved value
	Source  string // "cli", "env", or "default"
}

// RegisterFlags registers the custom CLI Feature Flags to be bound to the Cobra instance.
func RegisterFlags(flags *pflag.FlagSet) {
	for i := range registry {
		cliFlag := &registry[i]
		flags.BoolVar(&cliFlag.bound, cliFlag.name, cliFlag.defaultVal, cliFlag.desc)
	}
}

// ApplyCLIOverrides populates the cliOverrides map in case any flags were set at runtime.
func ApplyCLIOverrides(flags *pflag.FlagSet) {
	for i := range registry {
		cliFlag := &registry[i]
		if flags.Changed(cliFlag.name) {
			cliOverrides[cliFlag.flag] = cliFlag.bound
		}
	}
}

// IsEnabled returns whether the given feature flag is enabled.
// Resolution order: CLI override → env var ("true"/"1") → default.
func IsEnabled(flag Flag) bool {
	featureFlag := getFlag(flag)
	if featureFlag == nil {
		return false
	}
	val, _ := resolveFlag(featureFlag)
	return val
}

// List returns all registered flags with their current resolved value and source.
func List() []FlagState {
	flatStates := make([]FlagState, 0, len(registry))
	for i := range registry {
		r := &registry[i]
		enabled, source := resolveFlag(r)
		flatStates = append(flatStates, FlagState{
			Name:    r.name,
			EnvVar:  r.envVar,
			Enabled: enabled,
			Source:  source,
		})
	}
	return flatStates
}

// getFlag returns a flag from the registry based on its enum value
func getFlag(flag Flag) *CLIFeatureFlag {
	for i := range registry {
		if registry[i].flag == flag {
			return &registry[i]
		}
	}
	return nil
}

// resolveFlag determines the value and source for a single flag entry.
func resolveFlag(r *CLIFeatureFlag) (bool, string) {
	mu.Lock()
	val, ok := cliOverrides[r.flag]
	mu.Unlock()
	if ok {
		return val, "cli"
	}

	env := os.Getenv(r.envVar)
	if env != "" {
		return strings.EqualFold(env, "true") || env == "1", "env"
	}

	return r.defaultVal, "default"
}
