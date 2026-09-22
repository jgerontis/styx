package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for Styx.
type Config struct {
	Provider             string `yaml:"provider"`
	Model                string `yaml:"model"`
	BaseURL              string `yaml:"base-url"`
	LogLevel             string `yaml:"log-level"`
	OutputFormat         string `yaml:"output-format"`
	SkillsDir            string `yaml:"skills-dir"`
	SessionsDir          string `yaml:"sessions-dir"`
	ApprovalAutoReadOnly bool   `yaml:"approval-auto-readonly"`
	ApprovalSystemPaths  bool   `yaml:"approval-system-paths"`
	LoopMaxSameToolCalls int    `yaml:"loop-max-same-tool-calls"`
	LoopMaxIterations    int    `yaml:"loop-max-iterations"`

	// System1Enabled turns on the optional fast-gate sidecar (e.g. Laya).
	// Styx runs fully without it; when false or unreachable, gating falls
	// back to the static permission rules and full LLM calls.
	System1Enabled          bool    `yaml:"system1-enabled"`
	System1Endpoint         string  `yaml:"system1-endpoint"`
	System1AutoApprove      bool    `yaml:"system1-auto-approve"`
	System1ConfidenceThresh float64 `yaml:"system1-confidence-threshold"`
}

// Load reads configuration with precedence: env > file > defaults.
// Env var format: STYX_PROVIDER, STYX_MODEL, etc. (UPPERCASE_UNDERSCORES)
// File location: ~/.styx/config.yaml
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Load from file if it exists
	configPath := configPath()
	if _, err := os.Stat(configPath); err == nil {
		if err := loadFromFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Override with environment variables
	if v := os.Getenv("STYX_PROVIDER"); v != "" {
		cfg.Provider = v
	}
	if v := os.Getenv("STYX_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("STYX_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("STYX_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("STYX_OUTPUT_FORMAT"); v != "" {
		cfg.OutputFormat = v
	}
	if v := os.Getenv("STYX_SKILLS_DIR"); v != "" {
		cfg.SkillsDir = v
	}
	if v := os.Getenv("STYX_SESSIONS_DIR"); v != "" {
		cfg.SessionsDir = v
	}

	// Set defaults for dirs if not specified
	if cfg.SkillsDir == "" {
		cfg.SkillsDir = filepath.Join(userConfigDir(), "skills")
	}
	if cfg.SessionsDir == "" {
		cfg.SessionsDir = filepath.Join(userConfigDir(), "sessions")
	}
	if cfg.ApprovalAutoReadOnly == false {
		cfg.ApprovalAutoReadOnly = true
	}
	if cfg.ApprovalSystemPaths == false {
		cfg.ApprovalSystemPaths = true
	}

	return cfg, nil
}

// loadFromFile reads config from a YAML file.
func loadFromFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, cfg)
}

// SaveToFile writes config to a YAML file.
func (c *Config) SaveToFile() error {
	path := configPath()
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// configPath returns the path to the config file.
func configPath() string {
	return filepath.Join(userConfigDir(), "config.yaml")
}

// userConfigDir returns ~/.styx directory.
func userConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".styx")
}
