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

package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gofrs/flock"
	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/clikit/closer"
)

// StateDir returns the absolute path to the CLI state directory. Resolution
// mirrors the GitHub CLI's config.StateDir (github.com/cli/go-gh), in priority
// order:
//   - $XDG_STATE_HOME/circleci    (when XDG_STATE_HOME is set, any platform)
//   - %LocalAppData%\circleci     (Windows — LocalAppData, not roaming AppData,
//     because state is machine-local bookkeeping that should not roam)
//   - ~/.local/state/circleci     (default)
//
// State is machine-managed bookkeeping (e.g. the last update-check timestamp),
// deliberately kept out of config.yml so it never churns a file users hand-edit.
func StateDir() (string, error) {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "circleci"), nil
	}
	if runtime.GOOS == "windows" {
		if base := os.Getenv("LocalAppData"); base != "" {
			return filepath.Join(base, "circleci"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "circleci"), nil
}

// StatePath returns the absolute path to the CLI state file (state.yml).
func StatePath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.yml"), nil
}

// LockPath returns the advisory-lock companion path for a config or state file
// (path + ".lock"). Exported so callers outside this package can take the same
// lock LoadState / SaveState use — e.g. tests that pre-acquire it.
func LockPath(path string) string {
	return lockPath(path)
}

// stateLockRetry is how often a contended state lock is retried while waiting
// for it. Kept short: state writes are tiny, and the read-modify-write is
// bounded by the caller's context rather than this interval.
const stateLockRetry = 50 * time.Millisecond

// State holds the CLI's machine-managed bookkeeping, persisted to state.yml.
// Unlike Config it is never hand-edited — it records things the CLI tracks for
// itself, such as when it last checked for a newer release. Read it with
// LoadState and mutate it with SaveState; both take the same advisory lock the
// config read-modify-write uses, so concurrent circleci invocations (parallel
// shells, CI matrices, an agent firing several commands) can't corrupt it.
type State struct {
	data stateData
}

type stateData struct {
	CheckedForUpdateAt time.Time `yaml:"checked_for_update_at,omitempty"`
	LatestRelease      Release   `yaml:"latest_release,omitempty"`
}

// Release is the newest release recorded by the last successful update check.
type Release struct {
	Version     string    `yaml:"version,omitempty"`
	PublishedAt time.Time `yaml:"published_at,omitempty"`
}

// CheckedForUpdateAt reports when the last update check refreshed from the
// network. The zero value means no check has ever succeeded.
func (s *State) CheckedForUpdateAt() time.Time { return s.data.CheckedForUpdateAt }

// SetCheckedForUpdateAt records when an update check last refreshed.
func (s *State) SetCheckedForUpdateAt(t time.Time) { s.data.CheckedForUpdateAt = t }

// LatestRelease returns the newest release the last update check saw.
func (s *State) LatestRelease() Release { return s.data.LatestRelease }

// SetLatestRelease records the newest release seen (pass the zero Release to
// clear it).
func (s *State) SetLatestRelease(r Release) { s.data.LatestRelease = r }

// LoadState reads the state file at path under a shared advisory lock, mirroring
// the lock Config's read path takes. A missing file yields a zero State. path is
// required (callers pass StatePath()).
func LoadState(ctx context.Context, path string) (_ *State, err error) {
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o700); mkErr != nil {
		return nil, fmt.Errorf("creating state directory: %w", mkErr)
	}

	fl := flock.New(lockPath(path))
	defer closer.ErrorHandler(fl, &err)

	rlock, err := fl.TryRLockContext(ctx, stateLockRetry)
	switch {
	case err != nil:
		return nil, fmt.Errorf("error opening state lock file: %w", err)
	case !rlock:
		return nil, fmt.Errorf("could not lock state file %s", path)
	}

	return readStateFile(path)
}

// SaveState performs a locked read-modify-write of the state file at path: it
// loads the current state (zero if absent) under an exclusive advisory lock,
// passes it to mutate, then writes the result back. This mirrors the
// read-modify-write Config uses for its own writes, so the whole read → mutate →
// write is atomic against other circleci invocations.
func SaveState(ctx context.Context, path string, mutate func(*State) error) (err error) {
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o700); mkErr != nil {
		return fmt.Errorf("creating state directory: %w", mkErr)
	}

	fl := flock.New(lockPath(path))
	defer closer.ErrorHandler(fl, &err)

	lock, err := fl.TryLockContext(ctx, stateLockRetry)
	switch {
	case err != nil:
		return fmt.Errorf("error opening state lock file: %w", err)
	case !lock:
		return fmt.Errorf("could not lock state file %s", path)
	}

	st, err := readStateFile(path)
	if err != nil {
		return err
	}

	if err = mutate(st); err != nil {
		return err
	}

	data, err := yaml.Marshal(st.data)
	if err != nil {
		return fmt.Errorf("serialising state: %w", err)
	}
	if err = os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}
	return nil
}

// readStateFile reads and parses the state file. The caller must already hold
// the advisory lock. A missing file yields a zero State, not an error.
func readStateFile(path string) (*State, error) {
	var st State
	data, err := os.ReadFile(path) //#nosec:G304 // path is the XDG state file path, not arbitrary input
	switch {
	case os.IsNotExist(err):
		return &st, nil
	case err != nil:
		return nil, fmt.Errorf("reading state file: %w", err)
	}
	if err := yaml.Unmarshal(data, &st.data); err != nil {
		return nil, fmt.Errorf("parsing state file %s: %w", path, err)
	}
	return &st, nil
}
