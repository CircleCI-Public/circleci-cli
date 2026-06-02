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
	"time"

	"github.com/gofrs/flock"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/closer"
	"github.com/CircleCI-Public/circleci-cli/internal/keyring"
)

// Config holds all persisted CLI settings.
type Config struct {
	state state
}

type state struct {
	Token     string     `yaml:"token,omitempty"`
	Host      string     `yaml:"host,omitempty"`
	DeviceID  *uuid.UUID `yaml:"device_id,omitempty"`
	UserID    *uuid.UUID `yaml:"user_id,omitempty"`
	Telemetry *bool      `yaml:"telemetry,omitempty"`
}

// DefaultHost is the CircleCI API host used when none is configured.
const DefaultHost = "https://circleci.com"

// noTelemetryEnvVars is the set of environment variables that disable telemetry
// regardless of the stored config preference.
var noTelemetryEnvVars = []string{"CIRCLECI_NO_TELEMETRY", "NO_ANALYTICS", "DO_NOT_TRACK", "CI"}

// ActiveTelemetryOverrides returns the names of environment variables that are
// currently set and override the stored telemetry preference.
func ActiveTelemetryOverrides() []string {
	var active []string
	for _, env := range noTelemetryEnvVars {
		if os.Getenv(env) != "" {
			active = append(active, env)
		}
	}
	return active
}

// Load reads the config file from the given path. If path is empty the
// default XDG path is used. Returns an empty Config if the file does not exist.
func Load(ctx context.Context, path string, secureStorage bool) (*Config, error) {
	cfg, err := load(ctx, path, secureStorage)
	if err != nil {
		return nil, err
	}

	if cfg.DeviceID() == uuid.Nil {
		deviceID, err := ensureDeviceID(ctx, path)
		if err != nil {
			return nil, err
		}

		cfg.state.DeviceID = &deviceID
	}

	return cfg, nil
}

func load(ctx context.Context, path string, secureStorage bool) (*Config, error) {
	resolved, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	err = mkConfigDir(resolved)
	if err != nil {
		return nil, err
	}

	fl := flock.New(lockPath(resolved))
	defer closer.ErrorHandler(fl, &err)

	rlock, err := fl.TryRLockContext(ctx, time.Second)
	switch {
	case err != nil:
		return nil, fmt.Errorf("error opening lock file: %w", err)
	default:
		if !rlock {
			return nil, fmt.Errorf("could not lock config file %s", resolved)
		}
	}

	var cfg Config

	data, err := os.ReadFile(resolved) //#nosec:G304 // resolved is derived from XDG config path or an explicit user-supplied flag, not arbitrary input
	if os.IsNotExist(err) {
		if secureStorage {
			err = cfg.loadToken(ctx)
			if err != nil {
				return nil, err
			}
		}
		return &cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg.state); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", resolved, err)
	}

	if secureStorage {
		err = cfg.loadToken(ctx)
		if err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

func mkConfigDir(resolved string) error {
	err := os.MkdirAll(filepath.Dir(resolved), 0o700)
	if err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	return nil
}

func lockPath(path string) string {
	return path + ".lock"
}

func SetLogin(ctx context.Context, host, token string, userID uuid.UUID, secureStorage bool) error {
	return saveTo(ctx, "", secureStorage, func(cfg *Config) error {
		cfg.state.Host = host
		cfg.state.Token = token
		cfg.state.UserID = &userID
		return nil
	})
}

func SetLogout(ctx context.Context, secureStorage bool) error {
	return saveTo(ctx, "", secureStorage, func(cfg *Config) error {
		cfg.state.Token = ""
		cfg.state.UserID = nil
		return nil
	})
}

func SetToken(ctx context.Context, token string, secureStorage bool) error {
	return saveTo(ctx, "", secureStorage, func(cfg *Config) error {
		cfg.state.Token = token
		return nil
	})
}

func SetHost(ctx context.Context, host string, secureStorage bool) error {
	return saveTo(ctx, "", secureStorage, func(cfg *Config) error {
		cfg.state.Host = host
		return nil
	})
}

// SetTelemetry persists the telemetry opt-in/opt-out preference.
// path follows the same convention as Load (empty → XDG default).
func SetTelemetry(ctx context.Context, enabled bool, path string) error {
	return saveTo(ctx, path, false, func(cfg *Config) error {
		cfg.state.Telemetry = &enabled
		return nil
	})
}

// IsTelemetry returns true when telemetry should be collected.
// Environment variables always take precedence over the stored config value.
// When no preference has been set, telemetry is enabled by default.
func (c *Config) IsTelemetry() bool {
	for _, env := range noTelemetryEnvVars {
		if os.Getenv(env) != "" {
			return false
		}
	}
	if c.state.Telemetry != nil {
		return *c.state.Telemetry
	}
	return true
}

func (c *Config) UserID() uuid.UUID {
	if c.state.UserID == nil {
		return uuid.Nil
	}
	return *c.state.UserID
}

func ensureDeviceID(ctx context.Context, path string) (id uuid.UUID, err error) {
	err = saveTo(ctx, path, false, func(cfg *Config) error {
		if cfg.state.DeviceID == nil || *cfg.state.DeviceID == uuid.Nil {
			cfg.state.DeviceID = new(uuid.New())
		}
		id = *cfg.state.DeviceID
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (c *Config) DeviceID() uuid.UUID {
	if c.state.DeviceID == nil {
		return uuid.Nil
	}
	return *c.state.DeviceID
}

// saveTo writes cfg to the given path, creating parent directories as needed.
// If path is empty the default XDG path is used.
func saveTo(ctx context.Context, path string, secureStorage bool, cb func(config *Config) error) error {
	resolved, err := resolvePath(path)
	if err != nil {
		return err
	}

	err = mkConfigDir(resolved)
	if err != nil {
		return err
	}

	fl := flock.New(lockPath(resolved))
	defer closer.ErrorHandler(fl, &err)

	lock, err := fl.TryLockContext(ctx, time.Second)
	if err != nil {
		return err
	}
	if !lock {
		return fmt.Errorf("could not lock config file %s", resolved)
	}

	var cfg Config

	data, err := os.ReadFile(resolved) //#nosec:G304 // resolved is derived from XDG config path or an explicit user-supplied flag, not arbitrary input
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg.state); err != nil {
		return fmt.Errorf("parsing config file %s: %w", resolved, err)
	}

	err = cb(&cfg)
	if err != nil {
		return err
	}

	cp := cfg.state
	if secureStorage {
		if cp.Token == "" {
			err = cfg.deleteToken(ctx)
			if err != nil {
				return err
			}
		} else {
			cp.Token = ""
			err = cfg.storeToken(ctx)
			if err != nil {
				return err
			}
		}
	}

	data, err = yaml.Marshal(cp)
	if err != nil {
		return fmt.Errorf("serialising config: %w", err)
	}

	if err := os.WriteFile(resolved, data, 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

// Path returns the resolved path to the default config file.
func Path() (string, error) {
	return configPath()
}

// EffectiveHost returns the host, checked in priority order:
// CIRCLECI_HOST env var → config file value → DefaultHost.
func (c *Config) EffectiveHost() string {
	if h := os.Getenv("CIRCLECI_HOST"); h != "" {
		return h
	}
	if c.state.Host != "" {
		return c.state.Host
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
	return c.state.Token
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
		// Keyring unavailable (no secret service, D-Bus not running, etc.).
		// Treat as "no stored token" — the user can still authenticate via
		// CIRCLECI_TOKEN env var or by passing --insecure-storage.
		return nil
	}

	c.state.Token = password
	return nil
}

func (c *Config) storeToken(ctx context.Context) error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	return keyring.Set(ctx, c.EffectiveHost(), u.Username, c.state.Token)
}

func (c *Config) deleteToken(ctx context.Context) error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	return keyring.Delete(ctx, c.EffectiveHost(), u.Username)
}

// resolvePath returns override if non-empty, otherwise the default XDG path.
func resolvePath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return configPath()
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
