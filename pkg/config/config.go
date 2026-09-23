// Package config manages per-project persistent configuration stored in
// {project-dir}/.dtwiz/config.yaml, keyed by Dynatrace environment URL.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	configDirName  = ".dtwiz"
	configFileName = "config.yaml"
)

// ProjectConfig is the top-level structure for a project's dtwiz configuration.
// It is keyed by Dynatrace environment URL so that a single project directory
// can hold independent configuration for multiple environments.
type ProjectConfig struct {
	Frontends map[string]FrontendConfig `yaml:"frontends,omitempty"`
}

// FrontendConfig stores the Dynatrace RUM frontend application ID for one environment.
type FrontendConfig struct {
	ID           string `yaml:"id"`
	FrontendName string `yaml:"frontendName"`
}

// Load reads the project config from {projectDir}/.dtwiz/config.yaml.
// If the file does not exist, an empty config is returned with no error.
func Load(projectDir string) (*ProjectConfig, error) {
	data, err := os.ReadFile(configPath(projectDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &ProjectConfig{}, nil
		}
		return nil, fmt.Errorf("read project config: %w", err)
	}
	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}
	return &cfg, nil
}

// Save writes cfg to {projectDir}/.dtwiz/config.yaml, creating the directory
// if it does not already exist. The file is written with 0o600 permissions.
// All existing environment entries in cfg are preserved; callers must Load
// before modifying a single entry to avoid overwriting other environments.
func Save(projectDir string, cfg *ProjectConfig) error {
	dir := filepath.Join(projectDir, configDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal project config: %w", err)
	}
	if err := os.WriteFile(configPath(projectDir), data, 0o600); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}
	return nil
}

func configPath(projectDir string) string {
	return filepath.Join(projectDir, configDirName, configFileName)
}
