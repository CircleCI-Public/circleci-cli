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

// CompileInput holds everything a config compilation needs. It is the input to
// Compile, in the CLI's own terms rather than the wire shape.
type CompileInput struct {
	// ConfigYAML is the raw pipeline config to compile. Required.
	ConfigYAML string
	// OrgID is the owning organization UUID. Optional; supplying one enables
	// private and namespaced orb resolution.
	OrgID string
	// PreviewNext enables "config next", previewing upcoming and potentially
	// breaking config changes.
	PreviewNext bool
	// PipelineValues is the << pipeline.* >> template context. The CLI
	// fabricates it locally from git state (see configcmd.LocalPipelineValues).
	PipelineValues map[string]any
	// PipelineParameters are the user parameters injected at
	// << pipeline.parameters.* >>.
	PipelineParameters map[string]any
}

// CompileResult is the outcome of a config compilation. An invalid config is not
// an error: Valid is false and Errors holds the compilation messages.
type CompileResult struct {
	Valid        bool
	CompiledYAML string
	Errors       []string
}

// compileOutcomeSucceeded is the outcome the v3 compile endpoint reports for a
// config that compiled. Anything else — "failed", or an outcome this client does
// not know — is treated as invalid.
const compileOutcomeSucceeded = "succeeded"

// compileV3Request is the data-envelope body of POST /api/v3/configs/compile.
// The endpoint rejects unknown members, so nothing outside this shape may be sent.
type compileV3Request struct {
	Data compileV3Data `json:"data"`
}

type compileV3Data struct {
	Attributes compileV3Attributes  `json:"attributes"`
	References *compileV3References `json:"references,omitempty"`
}

type compileV3Attributes struct {
	Config             string         `json:"config"`
	EnableNextPreview  bool           `json:"enable_next_preview,omitempty"`
	PipelineParameters map[string]any `json:"pipeline_parameters,omitempty"`
	PipelineValues     map[string]any `json:"pipeline_values,omitempty"`
}

type compileV3References struct {
	Org compileV3OrgRef `json:"org"`
}

type compileV3OrgRef struct {
	ID string `json:"id"`
}

// compileV3Response is the entity returned by POST /api/v3/configs/compile.
// A config that fails to compile is still HTTP 200: outcome is "failed" and the
// reasons arrive in meta.messages.
//
// The entity also carries a phase, but it is always "ended" for a synchronous
// compile, so there is nothing to read it for; if a deferred compile path ever
// appears, that is the field to start honouring.
type compileV3Response struct {
	Data struct {
		Attributes struct {
			Outcome        string `json:"outcome"`
			CompiledConfig string `json:"compiled_config"`
		} `json:"attributes"`
	} `json:"data"`
	Meta struct {
		Messages []struct {
			Title string `json:"title"`
		} `json:"messages"`
	} `json:"meta"`
}

// Compile compiles a pipeline config via POST /api/v3/configs/compile and
// reports whether it is valid, along with the fully expanded YAML. `config
// validate` and `config process` are the same operation — compilation — and
// differ only in which fields of the result they read.
//
// Transport and request-level failures are returned as errors; a config that
// merely fails to compile comes back as a CompileResult with Valid false.
func (c *Client) Compile(ctx context.Context, in CompileInput) (*CompileResult, error) {
	req := compileV3Request{
		Data: compileV3Data{
			Attributes: compileV3Attributes{
				Config:             in.ConfigYAML,
				EnableNextPreview:  in.PreviewNext,
				PipelineParameters: in.PipelineParameters,
				PipelineValues:     in.PipelineValues,
			},
		},
	}
	if in.OrgID != "" {
		req.Data.References = &compileV3References{Org: compileV3OrgRef{ID: in.OrgID}}
	}

	var resp compileV3Response
	if err := c.postV3(ctx, "/configs/compile", req, &resp); err != nil {
		return nil, err
	}

	result := &CompileResult{Valid: resp.Data.Attributes.Outcome == compileOutcomeSucceeded}
	if result.Valid {
		result.CompiledYAML = resp.Data.Attributes.CompiledConfig
	}
	for _, msg := range resp.Meta.Messages {
		result.Errors = append(result.Errors, msg.Title)
	}
	return result, nil
}
