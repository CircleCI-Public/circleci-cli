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

import (
	"context"
	"net/http"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

type ConfigPromptSetupStep struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

type ConfigPromptStack struct {
	Stack        string                  `json:"stack"`
	Image        string                  `json:"image,omitempty"`
	ImageVersion string                  `json:"image_version,omitempty"`
	Setup        []ConfigPromptSetupStep `json:"setup,omitempty"`
}

type ConfigPromptRequest struct {
	Stack             *ConfigPromptStack `json:"stack,omitempty"`
	ConfigPath        string             `json:"config_path"`
	HasExistingConfig bool               `json:"has_existing_config"`
}

type ConfigPrompt struct {
	Prompt        string
	PromptVersion string
}

// ConfigPrompt fetches the prompt a coding agent follows to write or improve a CircleCI config.
// It needs no token.
func (c *Client) ConfigPrompt(ctx context.Context, req ConfigPromptRequest) (*ConfigPrompt, error) {
	var resp v3Entity[struct {
		Attributes struct {
			Prompt        string `json:"prompt"`
			PromptVersion string `json:"prompt_version"`
		} `json:"attributes"`
	}]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/configs/generate-prompt",
		httpcl.Idempotent(),
		httpcl.Body(req),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	return &ConfigPrompt{
		Prompt:        resp.Data.Attributes.Prompt,
		PromptVersion: resp.Data.Attributes.PromptVersion,
	}, nil
}
