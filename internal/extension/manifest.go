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

package extension

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/projectref"
)

type Manifest struct {
	Name       string     `yaml:"name"`
	BinaryName string     `yaml:"binary_name"`
	Version    string     `yaml:"version"`
	Sha256     string     `yaml:"sha256"`
	URL        string     `yaml:"url"`
	Path       string     `yaml:"path"`
	Ref        *Reference `yaml:"reference,omitempty"`
}

// ReferenceAnnotation is the cobra command annotation key for when an
// extension provides reference documentation.
const ReferenceAnnotation = "extension:reference"

// Reference describes an extension command and, recursively, its subcommands.
// It is owned by the extension and fetched from the extension registry's
// release.json. When present, it enriches the CLI's generated reference docs;
// when absent, the CLI falls back to the base extension docs.
type Reference struct {
	Use         string          `json:"use,omitempty" yaml:"use,omitempty"`
	Short       string          `json:"short,omitempty" yaml:"short,omitempty"`
	Long        string          `json:"long,omitempty" yaml:"long,omitempty"`
	Args        []ReferenceArg  `json:"args,omitempty" yaml:"args,omitempty"`
	Flags       []ReferenceFlag `json:"flags,omitempty" yaml:"flags,omitempty"`
	Subcommands []ReferenceSub  `json:"subcommands,omitempty" yaml:"subcommands,omitempty"`
}

type ReferenceSub struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	Reference
}

// ReferenceFlag describes a single flag for Reference.
type ReferenceFlag struct {
	Name      string `json:"name" yaml:"name"`
	Shorthand string `json:"shorthand,omitempty" yaml:"shorthand,omitempty"`
	Usage     string `json:"usage,omitempty" yaml:"usage,omitempty"`
	// Type is the value placeholder shown after the flag name, e.g. "string".
	// Empty means the flag takes no value (boolean-style).
	Type    string `json:"type,omitempty" yaml:"type,omitempty"`
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
}

// ReferenceArg describes a single argument for Reference.
type ReferenceArg struct {
	Name string `json:"name" yaml:"name"`
	Help string `json:"help,omitempty" yaml:"help,omitempty"`
}

// Run executes the extension binary with args, injecting CircleCI environment
// variables.
//
// The current process is replaced by the extension via syscall exec on Unix;
// on Windows the extension is run as a child process and its exit code is
// propagated.
//
// If the extension binary is not found, ErrExtensionBinaryNotFound is returned
// and the caller should show prompt the user to reinstall the extension.
func (ext *Manifest) Run(ctx context.Context, client *apiclient.Client, args []string) error {
	path := ext.Path

	_, err := os.Stat(path)
	if err != nil {
		return &ErrExtensionBinaryNotFound{
			Name: ext.Name,
			Path: path,
		}
	}

	env := buildEnv(ctx, client)

	cmd := exec.CommandContext(ctx, path, args...) //#nosec:G204,G702 // path comes from LookPath, args are user-supplied CLI args for the extension
	cmd.Stdin = iostream.In(ctx)
	cmd.Stdout = iostream.Out(ctx)
	cmd.Stderr = iostream.Err(ctx)
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		if cmd.ProcessState != nil {
			return &ErrExited{Code: cmd.ProcessState.ExitCode()}
		}

		return err
	}
	return nil
}

// buildEnv constructs the environment for the extension process. It starts
// from the current process environment and overlays CIRCLE_* variables so
// extensions can call the CircleCI API without reimplementing auth.
func buildEnv(ctx context.Context, client *apiclient.Client) []string {
	env := os.Environ()

	cfg := cmdutil.GetConfig(ctx)

	type kv struct{ key, val string }
	overlays := []kv{
		{"CIRCLE_TOKEN", cfg.EffectiveToken()},
		{"CIRCLE_HOST", cfg.EffectiveHost()},
		{"CIRCLE_TELEMETRY_ENABLED", fmt.Sprintf("%t", cfg.IsTelemetry())},
	}

	// Best-effort: inject project metadata from git remote. Failures are
	// silently ignored — the extension is responsible for handling missing vars.
	if info, err := gitremote.Detect(); err == nil {
		parts := strings.SplitN(info.Slug, "/", 3)
		if len(parts) == 3 {
			overlays = append(overlays,
				kv{"CIRCLE_VCS_TYPE", vcsLong(parts[0])},
				kv{"CIRCLE_PROJECT_USERNAME", parts[1]},
				kv{"CIRCLE_PROJECT_REPONAME", parts[2]},
			)
		}
		if info.Branch != "" {
			overlays = append(overlays, kv{"CIRCLE_BRANCH", info.Branch})
		}
		if info.DefaultBranch != "" {
			overlays = append(overlays, kv{"CIRCLE_DEFAULT_BRANCH", info.DefaultBranch})
		}

		if id := resolveProjectID(ctx, client, cfg, info.Slug); id != "" {
			overlays = append(overlays, kv{"CIRCLE_PROJECT_ID", id})
		}
	}

	for _, o := range overlays {
		if o.val == "" {
			continue
		}
		prefix := o.key + "="
		replaced := false
		for i, e := range env {
			if strings.HasPrefix(e, prefix) {
				env[i] = prefix + o.val
				replaced = true
				break
			}
		}
		if !replaced {
			env = append(env, prefix+o.val)
		}
	}
	return env
}

// resolveProjectID returns the CircleCI project UUID for the current checkout,
// preferring the locally recorded value in .circleci/info.yml and falling back
// to an API lookup keyed by slug. Returns "" if no project ID can be determined.
func resolveProjectID(ctx context.Context, client *apiclient.Client, cfg *config.Config, slug string) string {
	if cwd, err := os.Getwd(); err == nil {
		if ref, err := projectref.Read(cwd); err == nil && ref.Project.ID != "" {
			return ref.Project.ID
		}
	}

	token := cfg.EffectiveToken()
	if token == "" || slug == "" || client == nil {
		return ""
	}

	info, err := client.GetProjectBySlug(ctx, slug)
	if err != nil {
		return ""
	}

	return info.ID.String()
}

func vcsLong(short string) string {
	switch short {
	case "gh":
		return "github"
	case "bb":
		return "bitbucket"
	case "gl":
		return "gitlab"
	default:
		return short
	}
}
