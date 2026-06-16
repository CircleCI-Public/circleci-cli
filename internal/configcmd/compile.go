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

package configcmd

import (
	"context"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// ValidateResult holds the outcome of a config validation call.
type ValidateResult struct {
	Valid        bool     `json:"valid"`
	CompiledYAML string   `json:"compiled_yaml,omitempty"`
	Errors       []string `json:"errors,omitempty"`
}

// Validate compiles the config YAML against the CircleCI API and returns whether it
// is valid. orgID may be empty; pass one to enable private orb resolution.
func Validate(ctx context.Context, client *apiclient.Client, configYAML, orgID string, previewNext bool) (*ValidateResult, error) {
	resp, err := client.CompileConfig(ctx, configYAML, orgID, previewNext, LocalPipelineValues(nil), nil)
	if err != nil {
		return nil, err
	}
	result := &ValidateResult{Valid: resp.Valid, CompiledYAML: resp.OutputYAML}
	for _, e := range resp.Errors {
		result.Errors = append(result.Errors, e.Message)
	}
	// The API can return valid:false without explicit errors — treat that as invalid.
	if !resp.Valid && len(result.Errors) == 0 {
		result.Errors = []string{"config compilation failed"}
	}
	return result, nil
}

// Process compiles the config YAML and returns the fully expanded output YAML.
// params are pipeline parameters injected at << pipeline.parameters.* >>.
func Process(ctx context.Context, client *apiclient.Client, configYAML, orgID string, previewNext bool, params map[string]any) (*ValidateResult, error) {
	resp, err := client.CompileConfig(ctx, configYAML, orgID, previewNext, LocalPipelineValues(params), params)
	if err != nil {
		return nil, err
	}
	result := &ValidateResult{Valid: resp.Valid, CompiledYAML: resp.OutputYAML}
	for _, e := range resp.Errors {
		result.Errors = append(result.Errors, e.Message)
	}
	if !resp.Valid && len(result.Errors) == 0 {
		result.Errors = []string{"config compilation failed"}
	}
	return result, nil
}
