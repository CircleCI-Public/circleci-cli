package pipelines

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/CircleCI-Public/circleci-cli/git"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/version"
)

func TestPipelines_Trigger(t *testing.T) {
	fix := fixture{}
	p, cleanup := fix.Run(
		http.StatusOK,
		`
{
	"id": "2bc0df8e-d258-4ae8-9c2b-3793f004725f",
	"number": 123,
	"state": "created",
	"created_at": "2020-03-01T09:30:00Z",
	"updated_at": "2020-03-01T09:32:00Z",
	"trigger": {
		"type": "api",
		"received_at": "2020-03-01T09:30:00Z",
		"actor": {
			"login": "the-actor-login",
			"avatar_url": "the-actor-avatar"
		}
	}
}`)
	defer cleanup()

	t.Run("Check resource-class is created", func(t *testing.T) {
		pipe, err := p.Trigger(
			git.Remote{
				VcsType:      "github",
				Organization: "the-organization",
				Project:      "the-project",
			},
			&TriggerParameters{
				Branch: "the-branch",
			},
		)
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(pipe, &Pipeline{
			ID:        "2bc0df8e-d258-4ae8-9c2b-3793f004725f",
			Number:    123,
			State:     "created",
			CreatedAt: time.Date(2020, 3, 1, 9, 30, 0, 0, time.UTC),
			UpdatedAt: time.Date(2020, 3, 1, 9, 32, 0, 0, time.UTC),
			Trigger: Trigger{
				Type:       "api",
				ReceivedAt: time.Date(2020, 3, 1, 9, 30, 0, 0, time.UTC),
				Actor: Actor{
					Login:     "the-actor-login",
					AvatarURL: "the-actor-avatar",
				},
			},
		}))
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/project/github/the-organization/the-project/pipeline"}))
		assert.Check(t, cmp.Equal(fix.method, "POST"))
		assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
			"Accept-Encoding": {"gzip"},
			"Accept-Type":     {"application/json"},
			"Circle-Token":    {"fake-token"},
			"Content-Length":  {"24"},
			"Content-Type":    {"application/json"},
			"User-Agent":      {version.UserAgent()},
		}))
		assert.Check(t, cmp.Equal(fix.Body(), `{"branch":"the-branch"}`+"\n"))
	})
}

func TestPipelines_Get(t *testing.T) {
	fix := fixture{}
	p, cleanup := fix.Run(
		http.StatusOK,
		`
{
	"items": [
		{
			"id": "673b09d4-bb6f-41e0-8923-61c486376bff",
			"number": 123,
			"state": "created",
			"created_at": "2020-03-01T09:30:00Z",
			"updated_at": "2020-03-01T09:32:00Z",
			"trigger": {
				"type": "api",
				"received_at": "2020-03-01T09:30:00Z",
				"actor": {
					"login": "the-actor-login",
					"avatar_url": "the-actor-avatar"
				}
			}
		},
		{
			"id": "ba7fea2b-47a4-4213-8425-dfa37d900a62",
			"number": 234,
			"state": "created",
			"created_at": "2020-04-01T09:30:00Z",
			"updated_at": "2020-04-01T09:32:00Z",
			"trigger": {
				"type": "webhook",
				"received_at": "2020-04-01T09:30:00Z",
				"actor": {
					"login": "the-actor-login",
					"avatar_url": "the-actor-avatar"
				}
			}
		}
	]
}`,
	)
	defer cleanup()

	t.Run("Check pipeline list results", func(t *testing.T) {
		pipes, err := p.Get(git.Remote{
			VcsType:      "github",
			Organization: "the-organization",
			Project:      "the-project",
		})
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(pipes, []Pipeline{
			{
				ID:        "673b09d4-bb6f-41e0-8923-61c486376bff",
				Number:    123,
				State:     "created",
				CreatedAt: time.Date(2020, 3, 1, 9, 30, 0, 0, time.UTC),
				UpdatedAt: time.Date(2020, 3, 1, 9, 32, 0, 0, time.UTC),
				Trigger: Trigger{
					Type:       "api",
					ReceivedAt: time.Date(2020, 3, 1, 9, 30, 0, 0, time.UTC),
					Actor: Actor{
						Login:     "the-actor-login",
						AvatarURL: "the-actor-avatar",
					},
				},
			},
			{
				ID:        "ba7fea2b-47a4-4213-8425-dfa37d900a62",
				Number:    234,
				State:     "created",
				CreatedAt: time.Date(2020, 4, 1, 9, 30, 0, 0, time.UTC),
				UpdatedAt: time.Date(2020, 4, 1, 9, 32, 0, 0, time.UTC),
				Trigger: Trigger{
					Type:       "webhook",
					ReceivedAt: time.Date(2020, 4, 1, 9, 30, 0, 0, time.UTC),
					Actor: Actor{
						Login:     "the-actor-login",
						AvatarURL: "the-actor-avatar",
					},
				},
			},
		}))
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/project/github/the-organization/the-project/pipeline"}))
		assert.Check(t, cmp.Equal(fix.method, "GET"))
		assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
			"Accept-Encoding": {"gzip"},
			"Accept-Type":     {"application/json"},
			"Circle-Token":    {"fake-token"},
			"User-Agent":      {version.UserAgent()},
		}))
		assert.Check(t, cmp.Equal(fix.Body(), ``))
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

func (f *fixture) Run(statusCode int, respBody string) (p *Pipelines, cleanup func()) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		defer r.Body.Close()
		_, _ = io.Copy(&f.body, r.Body)
		f.url = *r.URL
		f.header = r.Header
		f.method = r.Method

		w.WriteHeader(statusCode)
		_, _ = io.WriteString(w, respBody)
	})
	server := httptest.NewServer(mux)

	return New(rest.New(server.URL, "api/v2", "fake-token")), server.Close
}
