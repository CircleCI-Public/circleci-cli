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

package cmdconfig

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/configgen"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

// scanFixtureEnv is an internal test hook: when set to a file path, the
// scanner reads its result from that JSON file instead of calling envbuilder.
// Acceptance tests use it to exercise the generate flow without hitting
// Docker Hub. A fixture with a non-empty top-level "error" field produces a
// scan failure.
const scanFixtureEnv = "CIRCLECI_SCAN_FIXTURE"

// scan is the package-level seam for repository scanning. In-process tests
// swap it via SetScanForTest; out-of-process acceptance tests use
// CIRCLECI_SCAN_FIXTURE because they can't reach into the binary.
var scan = func(ctx context.Context, dir string) (*reposcan.Result, error) {
	if path := os.Getenv(scanFixtureEnv); path != "" {
		return loadScanFixture(path)
	}
	return reposcan.NewDefaultScanner().Scan(ctx, dir)
}

func loadScanFixture(path string) (*reposcan.Result, error) {
	data, err := os.ReadFile(path) //#nosec:G304,G703 // test-only hook; path comes from CIRCLECI_SCAN_FIXTURE
	if err != nil {
		return nil, err
	}
	var probe struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &probe); err == nil && probe.Error != "" {
		return nil, stderrors.New(probe.Error)
	}
	var r reposcan.Result
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("decode scan fixture: %w", err)
	}
	return &r, nil
}

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate [path]",
		Short: "Generate .circleci/config.yml from a repository scan",
		Long: heredoc.Doc(`
			Detect the language stack, container image, and setup commands for a
			repository, then write a starter pipeline to <path>/.circleci/config.yml.

			If no supported stack is detected, a minimal cimg/base:stable template
			with a placeholder build step is written instead so you have something
			to iterate on.

			If a config file already exists at that path, generate does not overwrite
			it; it prints a confirmation and exits successfully. The .circleci/
			directory is created if needed, and the file is written atomically.
		`),
		Example: heredoc.Doc(`
			# Generate a config for the current directory
			$ circleci config generate

			# Generate a config for a specific project path
			$ circleci config generate ./my-app

			# Re-run is a no-op when a config already exists
			$ circleci config generate
			✓ Using existing config at .circleci/config.yml
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: runGenerate,
	}
	return cmd
}

func runGenerate(cmd *cobra.Command, args []string) error {
	ctx := iostream.FromCmd(cmd.Context(), cmd)

	dir := "."
	if len(args) == 1 {
		dir = args[0]
	}

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return clierrors.New(
			"config.path_not_found",
			"Path not found",
			fmt.Sprintf("No directory exists at %q.", dir),
		).WithSuggestions(
			"Check the path you passed and try again",
			"Omit the argument to scan the current directory",
		).WithExitCode(clierrors.ExitBadArguments)
	}

	// Skip-check before scan so a no-op re-run never makes a network call.
	configPath := filepath.Join(dir, ".circleci", "config.yml")
	if _, err := os.Stat(configPath); err == nil {
		iostream.ErrPrintf(ctx, "%s Using existing config at %s\n",
			iostream.SymbolOK(ctx), configPath)
		return nil
	}

	result, err := scan(ctx, dir)
	if err != nil {
		return clierrors.New(
			"config.scan_failed",
			"Repository scan failed",
			fmt.Sprintf("Could not detect the project stack: %s.", err),
		).WithSuggestions(
			"Re-run with --debug to see scan details",
			"Try again; image resolution requires network access",
		).WithExitCode(clierrors.ExitGeneralError)
	}

	return configgen.Generate(ctx, dir, result)
}
