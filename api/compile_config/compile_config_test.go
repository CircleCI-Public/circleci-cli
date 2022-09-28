package compile_config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
)

var source_config = `version: 2.1
workflows:
  bar:
	jobs:
	  - foo

jobs:
  foo:
	machine: true
	steps:
	  - run: echo Hello World`

var compiled_config = `version: 2.1
	  workflows:
		version: 2
		bar:
		  jobs:
			- foo
	  
	  jobs:
		foo:
		  machine: true
		  steps:
		  - run:
			  command: echo Hello World`

var options = &Options{OwnerId: "123"}

func Test_CompileConfigWithDefaults(t *testing.T) {
	fix := fixture{}
	compile_config, cleanup := fix.Run(
		http.StatusOK,
		fmt.Sprintf(`{"valid": true,"output_yaml": "%s", "source_yaml": "%s"}`, source_config, compiled_config),
	)
	defer cleanup()

	t.Run("Check config is compiled correctly", func(t *testing.T) {
		rc, err := compile_config.CompileConfigWithDefaults(
			&CompileConfigRequest{
				ConfigYml: source_config,
				Options:   *options,
			})
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(rc, &CompileConfigResult{
			Valid:      true,
			OutputYaml: source_config,
			SourceYaml: compiled_config,
			Errors:     nil,
		}))
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/compile-config-with-defaults"}))
		assert.Check(t, cmp.Equal(fix.method, "POST"))
		assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
			"Accept-Encoding": {"gzip"},
			"Accept":          {"application/json"},
			"Circle-Token":    {"fake-token"},
			"Content-Length":  {"173"},
			"Content-Type":    {"application/json"},
			"User-Agent":      {version.UserAgent()},
		}))
		body, err := json.Marshal(&CompileConfigRequest{ConfigYml: source_config, Options: *options})
		assert.NilError(t, err)
		assert.Equal(t, strings.TrimSuffix(fix.Body(), "\n"), string(body))
	})
}

func Test_CompileConfig(t *testing.T) {
	fix := fixture{}
	compile_config, cleanup := fix.Run(
		http.StatusOK,
		`{
			"valid": true,
			"output_yaml": "version 2.1",
			"source_yaml": "version 2.1"
		}`,
	)
	defer cleanup()

	t.Run("Check config is compiled with org slug correctly", func(t *testing.T) {
		rc, err := compile_config.CompileConfig(&CompileConfigRequest{
			ConfigYml: source_config,
			Options:   *options,
		}, "gh/circleci")

		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(rc, &CompileConfigResult{
			Valid:      true,
			OutputYaml: "version 2.1",
			SourceYaml: "version 2.1",
			Errors:     nil,
		}))
	})
}

func Test_GetCollaborations(t *testing.T) {
	fix := fixture{}
	compileConfig, cleanup := fix.Run(
		http.StatusOK,
		`[{ "vcs_type": "github",
			"slug": "gh/circleci",
			"name": "circleci",
			"id": "org-id",
			"avatar_url": "image.png"
		}]`,
	)
	defer cleanup()

	t.Run("Check instance list", func(t *testing.T) {
		collaborations, err := compileConfig.GetOrgCollaborations()

		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(collaborations, []CollaborationResult{
			{
				VcsTye:    "github",
				OrgSlug:   "gh/circleci",
				OrgName:   "circleci",
				OrgId:     "org-id",
				AvatarUrl: "image.png",
			},
		}))
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/me/collaborations"}))
		assert.Check(t, cmp.Equal(fix.method, http.MethodGet))
		assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
			"Accept-Encoding": {"gzip"},
			"Accept":          {"application/json"},
			"Circle-Token":    {"fake-token"},
			"User-Agent":      {version.UserAgent()},
		}))
		assert.Check(t, cmp.Equal(fix.Body(), ""))
	})
}

func Test_GetOrgCollaborations(t *testing.T) {
	fix := fixture{}
	compileConfig, cleanup := fix.Run(
		http.StatusOK,
		`[{
			"vcs_type": "github",
			"slug": "gh/circleci",
			"name": "circleci",
			"id": "org-id",
			"avatar_url": "image.png"
		}]`,
	)
	defer cleanup()

	t.Run("Check collborations", func(t *testing.T) {
		collaborations, err := compileConfig.GetOrgCollaborations()
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(collaborations, []CollaborationResult{
			{
				VcsTye:    "github",
				OrgSlug:   "gh/circleci",
				OrgName:   "circleci",
				OrgId:     "org-id",
				AvatarUrl: "image.png",
			},
		}))
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/me/collaborations"}))
	})
}

type fixture struct {
	mu     sync.Mutex
	url    url.URL
	method string
	header http.Header
	body   bytes.Buffer
}

func (f *fixture) URL() url.URL {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.url
}

func (f *fixture) Method() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.method
}

func (f *fixture) Header() http.Header {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.header
}

func (f *fixture) Body() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.body.String()
}

func (f *fixture) Run(statusCode int, respBody string) (r *CompileConfig, cleanup func()) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		defer r.Body.Close()
		_, _ = io.Copy(&f.body, r.Body)
		f.url = *r.URL
		f.header = r.Header
		f.method = r.Method

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = io.WriteString(w, respBody)
	})
	server := httptest.NewServer(mux)

	cfg := &settings.Config{
		Debug:        false,
		Token:        "fake-token",
		RestEndpoint: "api/v2",
		Endpoint:     "api/v2",
		HTTPClient:   http.DefaultClient,
	}

	return New(rest.New(server.URL, cfg), rest.New(server.URL, cfg)), server.Close
}
