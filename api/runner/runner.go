package runner

import (
	"fmt"
	"net/url"
	"strings"
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

func (r *Runner) GetResourceClassByName(resourceClass string) (rc *ResourceClass, err error) {
	s := strings.SplitN(resourceClass, "/", 2)
	if len(s) != 2 {
		return nil, fmt.Errorf("bad resource class: %q", resourceClass)
	}

	namespace := s[0]
	rcs, err := r.GetResourceClassesByNamespace(namespace)
	if err != nil {
		return nil, err
	}

	for _, rc := range rcs {
		if rc.ResourceClass == resourceClass {
			return &rc, nil
		}
	}

	return nil, fmt.Errorf("resource class %q not found", resourceClass)
}

func (r *Runner) GetResourceClassesByNamespace(namespace string) ([]ResourceClass, error) {
	query := url.Values{}
	query.Set("namespace", namespace)
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
	query.Set("resource-class", resourceClass)
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

type RunnerInstance struct {
	ResourceClass  string     `json:"resource_class,omitempty"`
	Hostname       string     `json:"hostname"`
	Name           string     `json:"name"`
	FirstConnected *time.Time `json:"first_connected"`
	LastConnected  *time.Time `json:"last_connected"`
	LastUsed       *time.Time `json:"last_used"`
	IP             string     `json:"ip,omitempty"`
	Version        string     `json:"version"`
}

func runnerQueryFromString(query string) url.Values {
	switch {
	default:
		return url.Values{"namespace": []string{query}}
	case strings.Contains(query, "/"):
		return url.Values{"resource-class": []string{query}}
	}
}

func (r *Runner) GetRunnerInstances(query string) ([]RunnerInstance, error) {
	req, err := r.rc.NewRequest("GET", &url.URL{Path: "runner", RawQuery: runnerQueryFromString(query).Encode()}, nil)
	if err != nil {
		return nil, err
	}

	resp := struct {
		Items []RunnerInstance `json:"items"`
	}{}
	_, err = r.rc.DoRequest(req, &resp)
	return resp.Items, err
}
