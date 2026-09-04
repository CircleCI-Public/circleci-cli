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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// PipelineDefinitionRepo holds repository info for a pipeline definition source.
type PipelineDefinitionRepo struct {
	FullName   string `json:"full_name,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
}

// PipelineDefinitionSource describes a config or checkout source.
type PipelineDefinitionSource struct {
	Provider string                  `json:"provider,omitempty"`
	Repo     *PipelineDefinitionRepo `json:"repo,omitempty"`
	FilePath string                  `json:"file_path,omitempty"`
}

// PipelineDefinition represents a CircleCI pipeline definition.
type PipelineDefinition struct {
	ID             string                    `json:"id"`
	Name           string                    `json:"name"`
	Description    string                    `json:"description,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	ConfigSource   *PipelineDefinitionSource `json:"config_source,omitempty"`
	CheckoutSource *PipelineDefinitionSource `json:"checkout_source,omitempty"`
}

// hostedConfigProvider is the one provider whose config lives outside a repo, so
// it is the only one that maps to a "hosted" rather than a "vcs" config source.
const hostedConfigProvider = "circleci"

// config.type / checkout.type values on the v3 pipelines endpoints.
const (
	sourceTypeVCS    = "vcs"
	sourceTypeHosted = "hosted"
)

// vcsAttrs identifies a VCS integration and repository. repo_id is the external
// repository id as an opaque string — numeric for the GitHub providers, text for
// providers on the generic schema. repo_full_name is decorative for the GitHub
// providers but load-bearing for those that address a repo by owner and name.
type vcsAttrs struct {
	Provider     string `json:"provider"`
	RepoID       string `json:"repo_id,omitempty"`
	RepoFullName string `json:"repo_full_name,omitempty"`
}

// hostedAttrs identifies a config hosted outside a VCS repo.
type hostedAttrs struct {
	Provider string `json:"provider"`
}

// pipelineConfigAttrs is the config half of a pipeline: a tagged union on Type,
// where exactly one of VCS/Hosted is populated. FilePath and FileType are common to both.
type pipelineConfigAttrs struct {
	Type     string       `json:"type"`
	FilePath string       `json:"file_path"`
	FileType string       `json:"file_type,omitempty"`
	VCS      *vcsAttrs    `json:"vcs,omitempty"`
	Hosted   *hostedAttrs `json:"hosted,omitempty"`
}

// pipelineCheckoutAttrs is the checkout half: a union that today only ever
// carries the vcs variant.
type pipelineCheckoutAttrs struct {
	Type string    `json:"type,omitempty"`
	VCS  *vcsAttrs `json:"vcs,omitempty"`
}

// pipelineAttrs is the attributes object of a v3 pipeline entity, used for both
// the create body and the response.
type pipelineAttrs struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	CreatedAt   *time.Time            `json:"created_at,omitempty"`
	Config      pipelineConfigAttrs   `json:"config"`
	Checkout    pipelineCheckoutAttrs `json:"checkout"`
}

// pipelineRefs carries the owning project. On create it is where the project
// travels; the endpoint takes no project in the path.
type pipelineRefs struct {
	Project entityRef `json:"project"`
}

// entityRef is a reference to another entity by id.
type entityRef struct {
	ID string `json:"id"`
}

// pipelineEntity is the data entity of GET/POST /api/v3/pipelines.
type pipelineEntity struct {
	ID         string        `json:"id"`
	Attributes pipelineAttrs `json:"attributes"`
	References pipelineRefs  `json:"references"`
}

// The create body is a narrower document than the entity: the endpoint rejects
// unknown members, so it must carry no id and no created_at.
type pipelineCreateAttrs struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Config      pipelineConfigAttrs   `json:"config"`
	Checkout    pipelineCheckoutAttrs `json:"checkout"`
}

type pipelineCreateData struct {
	Attributes pipelineCreateAttrs `json:"attributes"`
	References pipelineRefs        `json:"references"`
}

// ListPipelineDefinitions returns all pipeline definitions for a project via
// GET /api/v3/pipelines.
func (c *Client) ListPipelineDefinitions(ctx context.Context, projectID string) ([]PipelineDefinition, error) {
	var resp v3List[pipelineEntity]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/pipelines",
		filterParam("project_id", projectID),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}

	defs := make([]PipelineDefinition, 0, len(resp.Data))
	for _, e := range resp.Data {
		defs = append(defs, e.toPipelineDefinition())
	}
	return defs, nil
}

// CreatePipelineDefinitionInput contains all fields for creating a pipeline definition.
// The RepoFullName fields are required by providers that address a repository by
// owner and name rather than by id, and ignored by the rest.
// ConfigFileType is optional; pass "github-actions" for a GitHub Actions workflow file.
type CreatePipelineDefinitionInput struct {
	Name                 string
	Description          string
	ConfigProvider       string
	ConfigRepoID         string
	ConfigRepoFullName   string
	ConfigFilePath       string
	ConfigFileType       string
	CheckoutProvider     string
	CheckoutRepoID       string
	CheckoutRepoFullName string
}

// TriggerPipelineRunInput contains the options for triggering a pipeline run.
type TriggerPipelineRunInput struct {
	DefinitionID   string
	ConfigBranch   string
	ConfigTag      string
	CheckoutBranch string
	CheckoutTag    string
	Parameters     map[string]any
}

// TriggerPipelineRunResult holds the response from triggering a pipeline run.
// When Triggered is false the pipeline was skipped (e.g. due to a CI skip
// commit message) and Message describes why.
type TriggerPipelineRunResult struct {
	Triggered bool
	ID        string
	State     string
	Number    int
	CreatedAt time.Time
	Message   string
}

// TriggerPipelineRun triggers a pipeline run via the recommended v2 endpoint.
// projectSlug must be in "vcs/org/repo" form (e.g. "gh/myorg/myrepo").
func (c *Client) TriggerPipelineRun(ctx context.Context, projectSlug string, input TriggerPipelineRunInput) (*TriggerPipelineRunResult, error) {
	// The endpoint path is /project/{provider}/{organization}/{project}/pipeline/run.
	// We split the slug into three separate route params so each segment is
	// individually percent-encoded. Passing the full slug as one param would
	// encode the slashes as %2F, producing a single path segment that the server
	// cannot route — resulting in a 404.
	parts := strings.SplitN(projectSlug, "/", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid project slug %q: expected provider/organization/project", projectSlug)
	}

	body := map[string]any{}
	if input.DefinitionID != "" {
		body["definition_id"] = input.DefinitionID
	}

	cfg := map[string]any{}
	if input.ConfigBranch != "" {
		cfg["branch"] = input.ConfigBranch
	} else if input.ConfigTag != "" {
		cfg["tag"] = input.ConfigTag
	}
	if len(cfg) > 0 {
		body["config"] = cfg
	}

	checkout := map[string]any{}
	if input.CheckoutBranch != "" {
		checkout["branch"] = input.CheckoutBranch
	} else if input.CheckoutTag != "" {
		checkout["tag"] = input.CheckoutTag
	}
	if len(checkout) > 0 {
		body["checkout"] = checkout
	}

	if len(input.Parameters) > 0 {
		body["parameters"] = input.Parameters
	}

	var raw struct {
		ID        string    `json:"id"`
		State     string    `json:"state"`
		Number    int       `json:"number"`
		CreatedAt time.Time `json:"created_at"`
		Message   string    `json:"message"`
	}
	status, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v2/project/%s/%s/%s/pipeline/run",
		httpcl.RouteParams(parts[0], parts[1], parts[2]),
		httpcl.Body(body),
		httpcl.JSONDecoder(&raw),
	))
	if err != nil {
		return nil, err
	}
	return &TriggerPipelineRunResult{
		Triggered: status == http.StatusCreated,
		ID:        raw.ID,
		State:     raw.State,
		Number:    raw.Number,
		CreatedAt: raw.CreatedAt,
		Message:   raw.Message,
	}, nil
}

// CreatePipelineDefinition creates a new pipeline definition for a project via
// POST /api/v3/pipelines. The owning project travels in the body's references
// rather than the path.
func (c *Client) CreatePipelineDefinition(ctx context.Context, projectID string, input CreatePipelineDefinitionInput) (*PipelineDefinition, error) {
	attrs := pipelineCreateAttrs{
		Name:        input.Name,
		Description: input.Description,
		Config: pipelineConfigAttrs{
			Type:     sourceTypeVCS,
			FilePath: input.ConfigFilePath,
			FileType: input.ConfigFileType,
			VCS: &vcsAttrs{
				Provider:     input.ConfigProvider,
				RepoID:       input.ConfigRepoID,
				RepoFullName: input.ConfigRepoFullName,
			},
		},
		Checkout: pipelineCheckoutAttrs{
			VCS: &vcsAttrs{
				Provider:     input.CheckoutProvider,
				RepoID:       input.CheckoutRepoID,
				RepoFullName: input.CheckoutRepoFullName,
			},
		},
	}
	// A config not backed by a repository is the "hosted" variant, which carries a
	// provider and no repo. The checkout stays a VCS repo either way.
	if input.ConfigProvider == hostedConfigProvider {
		attrs.Config = pipelineConfigAttrs{
			Type:     sourceTypeHosted,
			FilePath: input.ConfigFilePath,
			FileType: input.ConfigFileType,
			Hosted:   &hostedAttrs{Provider: input.ConfigProvider},
		}
	}

	body := v3Entity[pipelineCreateData]{Data: pipelineCreateData{
		Attributes: attrs,
		References: pipelineRefs{Project: entityRef{ID: projectID}},
	}}

	var resp v3Entity[pipelineEntity]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/pipelines",
		httpcl.Body(body),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	def := resp.Data.toPipelineDefinition()
	return &def, nil
}

// toPipelineDefinition flattens a v3 pipeline entity into the definition shape the
// commands render: the config and checkout unions become one source struct each,
// so a hosted config reports its provider with no repo.
func (e pipelineEntity) toPipelineDefinition() PipelineDefinition {
	def := PipelineDefinition{
		ID:          e.ID,
		Name:        e.Attributes.Name,
		Description: e.Attributes.Description,
	}
	if e.Attributes.CreatedAt != nil {
		def.CreatedAt = *e.Attributes.CreatedAt
	}

	cfg := e.Attributes.Config
	switch {
	case cfg.Hosted != nil:
		def.ConfigSource = &PipelineDefinitionSource{Provider: cfg.Hosted.Provider, FilePath: cfg.FilePath}
	case cfg.VCS != nil:
		def.ConfigSource = sourceFromVCS(cfg.VCS)
		def.ConfigSource.FilePath = cfg.FilePath
	}

	if vcs := e.Attributes.Checkout.VCS; vcs != nil {
		def.CheckoutSource = sourceFromVCS(vcs)
	}
	return def
}

// sourceFromVCS maps a v3 vcs object onto the provider/repo source shape.
func sourceFromVCS(vcs *vcsAttrs) *PipelineDefinitionSource {
	src := &PipelineDefinitionSource{Provider: vcs.Provider}
	if vcs.RepoID != "" || vcs.RepoFullName != "" {
		src.Repo = &PipelineDefinitionRepo{ExternalID: vcs.RepoID, FullName: vcs.RepoFullName}
	}
	return src
}
