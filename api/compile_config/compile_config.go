package compile_config

import (
	"fmt"
	"net/url"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
)

type CompileConfig struct {
	rc            *rest.Client
	compileClient *rest.Client
}

func New(rc *rest.Client, compileClient *rest.Client) *CompileConfig {
	return &CompileConfig{rc: rc, compileClient: compileClient}
}

type ConfigError struct {
	Message string `json:"message"`
}

type CompileConfigResult struct {
	Valid      bool          `json:"valid"`
	OutputYaml string        `json:"output_yaml"`
	SourceYaml string        `json:"source_yaml"`
	Errors     []ConfigError `json:"errors"`
}

type CollaborationResult struct {
	VcsTye    string `json:"vcs_type"`
	OrgSlug   string `json:"slug"`
	OrgName   string `json:"name"`
	OrgId     string `json:"id"`
	AvatarUrl string `json:"avatar_url"`
}

type Options struct {
	OwnerId            string            `json:"owner_id,omitempty"`
	PipelineParameters string            `json:"pipeline_parameters,omitempty"`
	PipelineValues     map[string]string `json:"pipeline_values,omitempty"`
}

type CompileConfigRequest struct {
	ConfigYml string  `json:"config_yaml"`
	Options   Options `json:"options,omitempty"`
}

func GetOrgIdFromSlug(slug string, collaborations []CollaborationResult) string {
	for _, v := range collaborations {
		if v.OrgSlug == slug {
			return v.OrgId
		}
	}
	return ""
}

func (r *CompileConfig) CompileConfig(request *CompileConfigRequest, orgSlug string) (rc *CompileConfigResult, err error) {
	var orgId string
	if orgSlug != "" {
		orgs, err := r.GetOrgCollaborations()
		if err != nil {
			return nil, err
		}
		orgId = GetOrgIdFromSlug(orgSlug, orgs)
	} else {
		orgId = request.Options.OwnerId
	}

	configOptions := &Options{OwnerId: orgId,
		PipelineParameters: request.Options.PipelineParameters,
		PipelineValues:     request.Options.PipelineValues}

	compileConfgRequest := &CompileConfigRequest{ConfigYml: request.ConfigYml,
		Options: *configOptions}

	rcs, err := r.CompileConfigWithDefaults(compileConfgRequest)
	if err != nil {
		return nil, err
	}

	return rcs, nil
}

func (r *CompileConfig) GetOrgCollaborations() ([]CollaborationResult, error) {
	req, err := r.rc.NewRequest("GET", &url.URL{Path: "me/collaborations"}, nil)
	if err != nil {
		return nil, err
	}

	var resp []CollaborationResult
	_, err = r.rc.DoRequest(req, &resp)
	return resp, err
}

func (r *CompileConfig) CompileConfigWithDefaults(compileConfigRequest *CompileConfigRequest) (*CompileConfigResult, error) {
	println("calling api****")
	req, err := r.compileClient.NewRequest("POST", &url.URL{Path: "compile-config-with-defaults"}, compileConfigRequest)
	fmt.Printf("calling request****: %+v", req)

	if err != nil {
		return nil, err
	}

	resp := &CompileConfigResult{}
	_, err = r.compileClient.DoRequest(req, resp)
	return resp, err
}
