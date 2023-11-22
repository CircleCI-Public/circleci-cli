package pipelines

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/git"
)

type Pipelines struct {
	rc *rest.Client
}

func New(rc *rest.Client) *Pipelines {
	return &Pipelines{rc: rc}
}

type Pipeline struct {
	ID        string    `json:"id"`
	Number    int       `json:"number"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Trigger   Trigger   `json:"trigger"`
}

type Trigger struct {
	Type       string    `json:"type"`
	ReceivedAt time.Time `json:"received_at"`
	Actor      Actor     `json:"actor"`
}

type Actor struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

func (p *Pipelines) Get(remote git.Remote) ([]Pipeline, error) {
	req, err := p.rc.NewRequest("GET", pipelineSlug(remote), nil)
	if err != nil {
		return nil, err
	}

	resp := struct {
		Items []Pipeline `json:"items"`
	}{}
	_, err = p.rc.DoRequest(req, &resp)
	return resp.Items, err
}

type TriggerParameters struct {
	Branch string `json:"branch,omitempty"`
}

func (p *Pipelines) Trigger(remote git.Remote, params *TriggerParameters) (*Pipeline, error) {
	req, err := p.rc.NewRequest("POST", pipelineSlug(remote), params)
	if err != nil {
		return nil, err
	}

	resp := &Pipeline{}
	_, err = p.rc.DoRequest(req, resp)
	return resp, err
}

func pipelineSlug(remote git.Remote) *url.URL {
	return &url.URL{Path: fmt.Sprintf("project/%s/%s/%s/pipeline",
		strings.ToLower(string(remote.VcsType)),
		url.QueryEscape(remote.Organization),
		url.QueryEscape(remote.Project))}
}
