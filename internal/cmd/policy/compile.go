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

package policy

import (
	"context"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/configcmd"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
)

// resolveOwnerID resolves the organization reference to its UUID for config
// compilation. An empty ref is inferred best-effort from the current project or
// git remote and yields "" (not an error) when it cannot be determined, since
// the org is only needed to resolve private and namespaced orbs. A supplied ref
// (slug or UUID) is resolved strictly.
func resolveOwnerID(ctx context.Context, client *apiclient.Client, org, cmdName string) (string, error) {
	if org == "" {
		return cmdutil.InferOrgID(ctx, client), nil
	}
	id, err := cmdutil.ResolveOrgSlugOrID(ctx, client, org, cmdName)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// compileConfig compiles the config YAML against the CircleCI compile endpoint
// and returns the source config with its compiled form nested under a
// "_compiled_" key, so policies can inspect both. ownerID may be empty; params
// are injected at << pipeline.parameters.* >>.
func compileConfig(ctx context.Context, client *apiclient.Client, input []byte, ownerID string, params map[string]any) ([]byte, error) {
	resp, err := client.CompileConfig(ctx, string(input), ownerID, false, configcmd.LocalPipelineValues(params), params)
	if err != nil {
		return nil, cmdutil.APIErr(err, "", "policy.compile_failed", "Config compilation request failed")
	}

	if !resp.Valid || len(resp.Errors) > 0 {
		msgs := make([]string, 0, len(resp.Errors))
		for _, e := range resp.Errors {
			msgs = append(msgs, e.Message)
		}
		detail := "config compilation failed"
		if len(msgs) > 0 {
			detail = strings.Join(msgs, "; ")
		}
		return nil, clierrors.New("policy.compile_invalid", "Config compilation failed", detail).
			WithSuggestions("Validate the config first: circleci config validate",
				"Or skip compilation with --no-compile to evaluate the raw config").
			WithExitCode(clierrors.ExitValidationFail)
	}

	var compiledConfigMap, sourceConfigMap map[string]any
	if err := yaml.Unmarshal([]byte(resp.OutputYAML), &compiledConfigMap); err != nil {
		return nil, clierrors.New("policy.compile_parse_failed", "Could not parse compiled config", err.Error()).
			WithExitCode(clierrors.ExitAPIError)
	}
	if err := yaml.Unmarshal([]byte(resp.SourceYAML), &sourceConfigMap); err != nil {
		return nil, clierrors.New("policy.compile_parse_failed", "Could not parse source config", err.Error()).
			WithExitCode(clierrors.ExitAPIError)
	}
	if sourceConfigMap == nil {
		sourceConfigMap = map[string]any{}
	}
	sourceConfigMap["_compiled_"] = compiledConfigMap

	merged, err := yaml.Marshal(sourceConfigMap)
	if err != nil {
		return nil, clierrors.New("policy.compile_parse_failed", "Could not serialize merged config", err.Error()).
			WithExitCode(clierrors.ExitAPIError)
	}
	return merged, nil
}
