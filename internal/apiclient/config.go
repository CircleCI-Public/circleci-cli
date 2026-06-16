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

package apiclient

import "context"

// CompileConfigRequest is sent to POST /api/v2/compile-config-with-defaults.
type CompileConfigRequest struct {
	ConfigYAML string               `json:"config_yaml"`
	Options    CompileConfigOptions `json:"options"`
}

// CompileConfigOptions controls org ownership and pipeline context for compilation.
type CompileConfigOptions struct {
	OwnerID            string         `json:"owner_id,omitempty"`
	Next               bool           `json:"next,omitempty"`
	PipelineValues     map[string]any `json:"pipeline_values,omitempty"`
	PipelineParameters map[string]any `json:"pipeline_parameters,omitempty"`
}

// CompileConfigResponse is returned by /api/v2/compile-config-with-defaults.
type CompileConfigResponse struct {
	Valid      bool                 `json:"valid"`
	SourceYAML string               `json:"source-yaml"`
	OutputYAML string               `json:"output-yaml"`
	Errors     []CompileConfigError `json:"errors"`
}

// CompileConfigError is one entry in a compile response's errors array.
type CompileConfigError struct {
	Message string `json:"message"`
}

// CompileConfig sends a config YAML to the compilation API and returns the result.
// Transport failures are returned as errors; API-level validation errors are in the response.
func (c *Client) CompileConfig(ctx context.Context, configYAML, orgID string, previewNext bool, pipelineValues, pipelineParams map[string]any) (*CompileConfigResponse, error) {
	req := CompileConfigRequest{
		ConfigYAML: configYAML,
		Options: CompileConfigOptions{
			OwnerID:            orgID,
			Next:               previewNext,
			PipelineValues:     pipelineValues,
			PipelineParameters: pipelineParams,
		},
	}
	var resp CompileConfigResponse
	if err := c.post(ctx, "/compile-config-with-defaults", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
