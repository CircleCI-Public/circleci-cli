// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

// Package config manages the CLI's user-level configuration file.
//
// Config file location follows XDG Base Directory Specification:
//   - $XDG_CONFIG_HOME/circleci/config.yml  (when XDG_CONFIG_HOME is set)
//   - ~/.config/circleci/config.yml          (default)
package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/keyring"
)

// Config holds all persisted CLI settings.
type Config struct {
	Token string `yaml:"token,omitempty"`
	Host  string `yaml:"host,omitempty"`
}

// DefaultHost is the CircleCI API host used when none is configured.
const DefaultHost = "https://circleci.com"

// Load reads the config file, returning an empty Config if the file does not exist.
func Load(ctx context.Context, secureStorage bool) (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := &Config{}
		if secureStorage {
			err = cfg.loadToken(ctx)
			if err != nil {
				return nil, err
			}
		}
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}

	if secureStorage {
		err = cfg.loadToken(ctx)
		if err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

// Save writes cfg to the config file, creating parent directories as needed.
func Save(ctx context.Context, cfg *Config, secureStorage bool) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	cp := *cfg
	if secureStorage {
		cp.Token = ""

		err = cfg.storeToken(ctx)
		if err != nil {
			return err
		}
	}

	data, err := yaml.Marshal(cp)
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

func (c *Config) loadToken(ctx context.Context) error {
	// At some point this should be the actual CircleCI username (when doing oauth login).
	u, err := user.Current()
	if err != nil {
		return err
	}

	password, err := keyring.Get(ctx, c.EffectiveHost(), u.Username)
	switch {
	case errors.Is(err, keyring.ErrNotFound):
		return nil
	case err != nil:
		return err
	}

	c.Token = password
	return nil
}

func (c *Config) storeToken(ctx context.Context) error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	return keyring.Set(ctx, c.EffectiveHost(), u.Username, c.Token)
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
