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

package orb

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newPackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pack <path>",
		Short: "Pack a multi-file orb directory into a single YAML",
		Long: heredoc.Doc(`
			Pack an orb directory into a single YAML file.

			If the path is a file, reads and prints it to stdout.

			If the path is a directory, reads '@orb.yml' (or 'orb.yml') as the
			base YAML and merges 'commands/', 'jobs/', 'executors/', and 'examples/'
			subdirectories. Each .yml file in these directories is added as a named
			entry under the corresponding top-level key.

			The merged YAML is written to stdout.
		`),
		Example: heredoc.Doc(`
			# Pack a single orb file
			$ circleci orb pack orb.yml

			# Pack a multi-file orb directory
			$ circleci orb pack ./src

			# Pack and save to a file
			$ circleci orb pack ./src > orb.yml

			# Pack and immediately validate
			$ circleci orb pack ./src | circleci orb validate -
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "path"); err != nil {
				return err
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			return runOrbPack(ctx, args[0])
		},
	}
}

func runOrbPack(ctx context.Context, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return clierrors.New("orb.pack_error", "Cannot access path",
			err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	if !info.IsDir() {
		// Single file: read and print
		data, err := os.ReadFile(path) //#nosec:G304 // Intentionally reading a user-provided file
		if err != nil {
			return clierrors.New("orb.pack_error", "Failed to read orb file",
				err.Error()).
				WithExitCode(clierrors.ExitGeneralError)
		}
		iostream.Print(ctx, string(data))
		return nil
	}

	// Directory: find base file
	basePath := filepath.Join(path, "@orb.yml")
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		basePath = filepath.Join(path, "orb.yml")
	}

	baseData, err := os.ReadFile(basePath) //#nosec:G304 // Intentionally reading a user-provided file
	if err != nil {
		return clierrors.New("orb.pack_error", "Failed to read base orb file (@orb.yml or orb.yml)",
			err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	// Parse base YAML into a map
	var base map[string]any
	if err := yaml.Unmarshal(baseData, &base); err != nil {
		return clierrors.New("orb.pack_error", "Failed to parse base orb YAML",
			err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}
	if base == nil {
		base = map[string]any{}
	}

	// Merge subdirectories
	for _, section := range []string{"commands", "jobs", "executors", "examples"} {
		dirPath := filepath.Join(path, section)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return clierrors.New("orb.pack_error", "Failed to read directory "+section,
				err.Error()).
				WithExitCode(clierrors.ExitGeneralError)
		}

		sectionMap, _ := base[section].(map[string]any)
		if sectionMap == nil {
			sectionMap = map[string]any{}
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
				continue
			}
			key := strings.TrimSuffix(strings.TrimSuffix(name, ".yml"), ".yaml")
			filePath := filepath.Join(dirPath, name)
			fileData, err := os.ReadFile(filePath) //#nosec:G304 // Intentionally reading a user-provided file
			if err != nil {
				return clierrors.New("orb.pack_error", "Failed to read "+filePath,
					err.Error()).
					WithExitCode(clierrors.ExitGeneralError)
			}
			var content any
			if err := yaml.Unmarshal(fileData, &content); err != nil {
				return clierrors.New("orb.pack_error", "Failed to parse "+filePath,
					err.Error()).
					WithExitCode(clierrors.ExitGeneralError)
			}
			sectionMap[key] = content
		}

		if len(sectionMap) > 0 {
			base[section] = sectionMap
		}
	}

	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(base); err != nil {
		return clierrors.New("orb.pack_error", "Failed to render merged orb YAML",
			err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	iostream.Print(ctx, buf.String())
	return nil
}
