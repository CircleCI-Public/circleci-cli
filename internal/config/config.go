// Package config manages the CLI's user-level configuration file.
//
// Config file location follows XDG Base Directory Specification:
//   - $XDG_CONFIG_HOME/circleci/config.yml  (when XDG_CONFIG_HOME is set)
//   - ~/.config/circleci/config.yml          (default)
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all persisted CLI settings.
type Config struct {
	Token string `yaml:"token,omitempty"`
	Host  string `yaml:"host,omitempty"`
}

// DefaultHost is the CircleCI API host used when none is configured.
const DefaultHost = "https://circleci.com"

// Load reads the config file, returning an empty Config if the file does not exist.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}
	return &cfg, nil
}

// Save writes cfg to the config file, creating parent directories as needed.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serialising config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

// Path returns the resolved path to the config file.
func Path() (string, error) {
	return configPath()
}

// EffectiveHost returns the host, checked in priority order:
// CIRCLECI_HOST env var → config file value → DefaultHost.
func (c *Config) EffectiveHost() string {
	if h := os.Getenv("CIRCLECI_HOST"); h != "" {
		return h
	}
	if c.Host != "" {
		return c.Host
	}
	return DefaultHost
}

// EffectiveToken returns the token from the config, falling back to the
// CIRCLECI_TOKEN environment variable (with CIRCLECI_CLI_TOKEN as a legacy alias).
func (c *Config) EffectiveToken() string {
	if t := os.Getenv("CIRCLECI_TOKEN"); t != "" {
		return t
	}
	if t := os.Getenv("CIRCLECI_CLI_TOKEN"); t != "" {
		return t
	}
	return c.Token
}

func configPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "circleci", "config.yml"), nil
}
