package runner

import (
	"net/url"
	"time"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
)

type Runner struct {
	rc *rest.Client
}

func New(rc *rest.Client) *Runner {
	return &Runner{rc: rc}
}

type ResourceClass struct {
	ID            string `json:"id"`
	ResourceClass string `json:"resource_class"`
	Description   string `json:"description"`
}

func (r *Runner) CreateResourceClass(resourceClass, desc string) (rc *ResourceClass, err error) {
	req, err := r.rc.NewRequest("POST", &url.URL{Path: "runner/resource"}, struct {
		ResourceClass string `json:"resource_class"`
		Description   string `json:"description"`
	}{
		ResourceClass: resourceClass,
		Description:   desc,
	})
	if err != nil {
		return nil, err
	}

	rc = &ResourceClass{}
	_, err = r.rc.DoRequest(req, rc)
	return rc, err
}

func (r *Runner) GetResourceClassesByNamespace(namespace string) ([]ResourceClass, error) {
	query := url.Values{}
	query.Add("namespace", namespace)
	req, err := r.rc.NewRequest("GET", &url.URL{Path: "runner/resource", RawQuery: query.Encode()}, nil)
	if err != nil {
		return nil, err
	}

	resp := struct {
		Items []ResourceClass `json:"items"`
	}{}
	_, err = r.rc.DoRequest(req, &resp)
	return resp.Items, err
}

func (r *Runner) DeleteResourceClass(id string) error {
	req, err := r.rc.NewRequest("DELETE", &url.URL{Path: "runner/resource/" + url.PathEscape(id)}, nil)
	if err != nil {
		return err
	}

	_, err = r.rc.DoRequest(req, nil)
	return err
}

type Token struct {
	ID            string    `json:"id"`
	Token         string    `json:"token"`
	ResourceClass string    `json:"resource_class"`
	Nickname      string    `json:"nickname"`
	CreatedAt     time.Time `json:"created_at"`
}

func (r *Runner) CreateToken(resourceClass, nickname string) (token *Token, err error) {
	t := struct {
		ResourceClass string `json:"resource_class"`
		Nickname      string `json:"nickname"`
	}{
		ResourceClass: resourceClass,
		Nickname:      nickname,
	}

	req, err := r.rc.NewRequest("POST", &url.URL{Path: "runner/token"}, t)
	if err != nil {
		return nil, err
	}

	token = &Token{}
	_, err = r.rc.DoRequest(req, token)
	return token, err
}

func (r *Runner) GetRunnerTokensByResourceClass(resourceClass string) ([]Token, error) {
	query := url.Values{}
	query.Add("resource-class", resourceClass)
	req, err := r.rc.NewRequest("GET", &url.URL{Path: "runner/token", RawQuery: query.Encode()}, nil)
	if err != nil {
		return nil, err
	}

	resp := struct {
		Items []Token `json:"items"`
	}{}
	_, err = r.rc.DoRequest(req, &resp)
	return resp.Items, err
}

func (r *Runner) DeleteToken(id string) error {
	req, err := r.rc.NewRequest("DELETE", &url.URL{Path: "runner/token/" + url.PathEscape(id)}, nil)
	if err != nil {
		return err
	}

	_, err = r.rc.DoRequest(req, nil)
	return err
}