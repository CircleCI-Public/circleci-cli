package runner

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
)

func TestRunner_CreateResourceClass(t *testing.T) {
	fix := fixture{}
	runner, cleanup := fix.Run(
		http.StatusOK,
		`
{
	"id": "2bc0df8e-d258-4ae8-9c2b-3793f004725f",
	"resource_class": "the-namespace/the-resource-class",
	"description": "the-description"
}`,
	)
	defer cleanup()

	t.Run("Check resource-class is created", func(t *testing.T) {
		rc, err := runner.CreateResourceClass("the-namespace/the-resource-class", "the-description")
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(rc, &ResourceClass{
			ID:            "2bc0df8e-d258-4ae8-9c2b-3793f004725f",
			ResourceClass: "the-namespace/the-resource-class",
			Description:   "the-description",
		}))
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/runner/resource"}))
		assert.Check(t, cmp.Equal(fix.method, "POST"))
		assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
			"Accept-Encoding": {"gzip"},
			"Accept":          {"application/json"},
			"Circle-Token":    {"fake-token"},
			"Content-Length":  {"86"},
			"Content-Type":    {"application/json"},
			"User-Agent":      {version.UserAgent()},
		}))
		assert.Check(t, cmp.Equal(fix.Body(), `{"resource_class":"the-namespace/the-resource-class","description":"the-description"}`+"\n"))
	})
}

func TestRunner_GetResourceClassByName(t *testing.T) {
	fix := fixture{}
	runner, cleanup := fix.Run(
		http.StatusOK,
		`
{
	"items": [
		{"id": "7101f2a4-1617-4ef9-8fd4-f72de73896bd", "resource_class": "the-namespace/the-resource-class-1", "description": "the-description-1"},
		{"id": "b2713ad1-13b9-44f6-9b0d-1bf5f38571db", "resource_class": "the-namespace/the-resource-class-2", "description": "the-one-we-want"},
		{"id": "aa8cdb84-bc8e-4e42-a04a-8e719b586069", "resource_class": "the-namespace/the-resource-class-3", "description": "the-description-3"}
	]
}`,
	)
	defer cleanup()

	t.Run("Check resource-class list results", func(t *testing.T) {
		rc, err := runner.GetResourceClassByName("the-namespace/the-resource-class-2")
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(rc, &ResourceClass{
			ID:            "b2713ad1-13b9-44f6-9b0d-1bf5f38571db",
			ResourceClass: "the-namespace/the-resource-class-2",
			Description:   "the-one-we-want",
		}))
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/runner/resource", RawQuery: "namespace=the-namespace"}))
		assert.Check(t, cmp.Equal(fix.method, http.MethodGet))
		assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
			"Accept-Encoding": {"gzip"},
			"Accept":          {"application/json"},
			"Circle-Token":    {"fake-token"},
			"User-Agent":      {version.UserAgent()},
		}))
		assert.Check(t, cmp.Equal(fix.Body(), ``))
	})
}

func TestRunner_GetResourceClassByName_BadResourceClass(t *testing.T) {
	r := Runner{}
	rc, err := r.GetResourceClassByName("there-is-no-slash")
	assert.Check(t, cmp.Nil(rc))
	assert.ErrorContains(t, err, "bad resource class")
}

func TestRunner_GetResourceClassesByNamespace(t *testing.T) {
	fix := fixture{}
	runner, cleanup := fix.Run(
		http.StatusOK,
		`
{
	"items": [
		{"id": "7101f2a4-1617-4ef9-8fd4-f72de73896bd", "resource_class": "the-namespace/the-resource-class-1", "description": "the-description-1"},
		{"id": "b2713ad1-13b9-44f6-9b0d-1bf5f38571db", "resource_class": "the-namespace/the-resource-class-2", "description": "the-description-2"}
	]
}`,
	)
	defer cleanup()

	t.Run("Check resource-class list results", func(t *testing.T) {
		rcs, err := runner.GetResourceClassesByNamespace("the-namespace")
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(rcs, []ResourceClass{
			{
				ID:            "7101f2a4-1617-4ef9-8fd4-f72de73896bd",
				ResourceClass: "the-namespace/the-resource-class-1",
				Description:   "the-description-1",
			},
			{
				ID:            "b2713ad1-13b9-44f6-9b0d-1bf5f38571db",
				ResourceClass: "the-namespace/the-resource-class-2",
				Description:   "the-description-2",
			},
		}))
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/runner/resource", RawQuery: "namespace=the-namespace"}))
		assert.Check(t, cmp.Equal(fix.method, http.MethodGet))
		assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
			"Accept-Encoding": {"gzip"},
			"Accept":          {"application/json"},
			"Circle-Token":    {"fake-token"},
			"User-Agent":      {version.UserAgent()},
		}))
		assert.Check(t, cmp.Equal(fix.Body(), ``))
	})
}

func TestRunner_DeleteResourceClass(t *testing.T) {
	fix := fixture{}
	runner, cleanup := fix.Run(http.StatusOK, ``)
	defer cleanup()

	t.Run("Check resource-class is deleted", func(t *testing.T) {
		err := runner.DeleteResourceClass("51628548-4627-4813-9f9b-8cc9637ac879", false)
		assert.NilError(t, err)
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/runner/resource/51628548-4627-4813-9f9b-8cc9637ac879"}))
		assert.Check(t, cmp.Equal(fix.method, "DELETE"))
		assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
			"Accept-Encoding": {"gzip"},
			"Accept":          {"application/json"},
			"Circle-Token":    {"fake-token"},
			"User-Agent":      {version.UserAgent()},
		}))
		assert.Check(t, cmp.Equal(fix.Body(), ``))
	})
}

func TestRunner_DeleteResourceClass_Force(t *testing.T) {
	fix := fixture{}
	runner, cleanup := fix.Run(http.StatusOK, ``)
	defer cleanup()

	err := runner.DeleteResourceClass("5a1ef22d-444b-45db-8e98-21d7c42fb80b", true)
	assert.NilError(t, err)

	assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/runner/resource/5a1ef22d-444b-45db-8e98-21d7c42fb80b/force"}))
	assert.Check(t, cmp.Equal(fix.method, "DELETE"))
	assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
		"Accept-Encoding": {"gzip"},
		"Accept":          {"application/json"},
		"Circle-Token":    {"fake-token"},
		"User-Agent":      {version.UserAgent()},
	}))
	assert.Check(t, cmp.Equal(fix.Body(), ``))
}

func TestRunner_DeleteResourceClass_PathEscaping(t *testing.T) {
	fix := fixture{}
	runner, cleanup := fix.Run(http.StatusOK, ``)
	defer cleanup()

	t.Run("Check resource-class is deleted", func(t *testing.T) {
		err := runner.DeleteResourceClass("escape~,/;?~noescape~$&+:=@", false)
		assert.NilError(t, err)
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/runner/resource/escape~%2C%2F%3B%3F~noescape~$&+:=@"}))
	})
}

func TestRunner_CreateToken(t *testing.T) {
	fix := fixture{}
	runner, cleanup := fix.Run(
		http.StatusOK,
		`
{
	"id": "2bc0df8e-d258-4ae8-9c2b-3793f004725f",
	"resource_class": "the-namespace/the-resource-class",
	"nickname": "the-nickname",
	"created_at": "2020-10-01T09:55:00.000000Z"
}`,
	)
	defer cleanup()

	t.Run("Check token is created", func(t *testing.T) {
		token, err := runner.CreateToken("the-namespace/the-resource-class", "the-nickname")
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(token, &Token{
			ID:            "2bc0df8e-d258-4ae8-9c2b-3793f004725f",
			ResourceClass: "the-namespace/the-resource-class",
			Nickname:      "the-nickname",
			CreatedAt:     time.Date(2020, 10, 1, 9, 55, 0, 0, time.UTC),
		}))
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/runner/token"}))
		assert.Check(t, cmp.Equal(fix.method, "POST"))
		assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
			"Accept-Encoding": {"gzip"},
			"Accept":          {"application/json"},
			"Circle-Token":    {"fake-token"},
			"Content-Length":  {"80"},
			"Content-Type":    {"application/json"},
			"User-Agent":      {version.UserAgent()},
		}))
		assert.Check(t, cmp.Equal(fix.Body(), `{"resource_class":"the-namespace/the-resource-class","nickname":"the-nickname"}`+"\n"))
	})
}

func TestRunner_GetRunnerTokensByResourceClass(t *testing.T) {
	fix := fixture{}
	runner, cleanup := fix.Run(
		http.StatusOK,
		`
{
	"items": [
		{
			"id": "9e12ad09-527d-482c-b7ce-1a2fd20d1b9b",
			"resource_class": "the-namespace/the-resource-class",
			"nickname": "the-nickname-1",
			"created_at": "2020-10-01T09:55:00.000000Z"
		},
		{
			"id": "8618c56e-4abf-48a6-89fa-f75a845d1196",
			"resource_class": "the-namespace/the-resource-class",
			"nickname": "the-nickname-2",
			"created_at": "2020-10-01T09:55:00.000000Z"
		},
		{
			"id": "a3117e42-12ce-4027-8fde-5ce39e174dfe",
			"resource_class": "the-namespace/the-resource-class",
			"nickname": "the-nickname-3",
			"created_at": "2020-10-01T09:55:00.000000Z"
		}
	]
}`,
	)
	defer cleanup()

	t.Run("Check token list", func(t *testing.T) {
		token, err := runner.GetRunnerTokensByResourceClass("the-namespace/the-resource-class")
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(token, []Token{
			{
				ID:            "9e12ad09-527d-482c-b7ce-1a2fd20d1b9b",
				ResourceClass: "the-namespace/the-resource-class",
				Nickname:      "the-nickname-1",
				CreatedAt:     time.Date(2020, 10, 1, 9, 55, 0, 0, time.UTC),
			},
			{
				ID:            "8618c56e-4abf-48a6-89fa-f75a845d1196",
				ResourceClass: "the-namespace/the-resource-class",
				Nickname:      "the-nickname-2",
				CreatedAt:     time.Date(2020, 10, 1, 9, 55, 0, 0, time.UTC),
			},
			{
				ID:            "a3117e42-12ce-4027-8fde-5ce39e174dfe",
				ResourceClass: "the-namespace/the-resource-class",
				Nickname:      "the-nickname-3",
				CreatedAt:     time.Date(2020, 10, 1, 9, 55, 0, 0, time.UTC),
			},
		}))
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/runner/token", RawQuery: "resource-class=the-namespace%2Fthe-resource-class"}))
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

func TestRunner_DeleteToken(t *testing.T) {
	fix := fixture{}
	runner, cleanup := fix.Run(http.StatusOK, ``)
	defer cleanup()

	t.Run("Check token is deleted", func(t *testing.T) {
		err := runner.DeleteToken("ca5341fd-9b4d-4704-b16e-1b496d6012f2")
		assert.NilError(t, err)
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/runner/token/ca5341fd-9b4d-4704-b16e-1b496d6012f2"}))
		assert.Check(t, cmp.Equal(fix.method, "DELETE"))
		assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
			"Accept-Encoding": {"gzip"},
			"Accept":          {"application/json"},
			"Circle-Token":    {"fake-token"},
			"User-Agent":      {version.UserAgent()},
		}))
		assert.Check(t, cmp.Equal(fix.Body(), ``))
	})
}

func TestRunner_DeleteToken_PathEscaping(t *testing.T) {
	fix := fixture{}
	runner, cleanup := fix.Run(http.StatusOK, ``)
	defer cleanup()

	t.Run("Check token is deleted", func(t *testing.T) {
		err := runner.DeleteToken("escape~,/;?~noescape~$&+:=@")
		assert.NilError(t, err)
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/runner/token/escape~%2C%2F%3B%3F~noescape~$&+:=@"}))
	})
}

func TestRunner_GetRunnerInstances_ByNamespace(t *testing.T) {
	fix := fixture{}
	runner, cleanup := fix.Run(
		http.StatusOK,
		`
{
	"items": [
		{
			"resource_class": "the-namespace/the-resource-class",
			"hostname": "the-hostname-1",
			"name": "the-name-1",
			"first_connected": "2020-10-01T09:55:00.000000Z",
			"last_connected": "2020-10-01T09:55:00.000000Z",
			"last_used": "2020-10-01T09:55:00.000000Z",
			"ip": "1.2.3.4",
			"version": "2.10.32"
		},
		{
			"resource_class": "the-namespace/the-resource-class",
			"hostname": "the-hostname-2",
			"name": "the-name-2",
			"first_connected": "2020-10-01T09:55:00.000000Z",
			"last_connected": "2020-10-01T09:55:00.000000Z",
			"last_used": "2020-10-01T09:55:00.000000Z",
			"ip": "1.2.3.5",
			"version": "2.10.33"
		}
	]
}`,
	)
	defer cleanup()

	t.Run("Check instance list", func(t *testing.T) {
		token, err := runner.GetRunnerInstances("the-namespace")
		assert.NilError(t, err)
		d := time.Date(2020, 10, 1, 9, 55, 0, 0, time.UTC)
		assert.Check(t, cmp.DeepEqual(token, []RunnerInstance{
			{
				ResourceClass:  "the-namespace/the-resource-class",
				Hostname:       "the-hostname-1",
				Name:           "the-name-1",
				FirstConnected: &d,
				LastConnected:  &d,
				LastUsed:       &d,
				IP:             "1.2.3.4",
				Version:        "2.10.32",
			},
			{
				ResourceClass:  "the-namespace/the-resource-class",
				Hostname:       "the-hostname-2",
				Name:           "the-name-2",
				FirstConnected: &d,
				LastConnected:  &d,
				LastUsed:       &d,
				IP:             "1.2.3.5",
				Version:        "2.10.33",
			},
		}))
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/runner", RawQuery: "namespace=the-namespace"}))
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

func TestRunner_GetRunnerInstances_ByResourceClass(t *testing.T) {
	fix := fixture{}
	runner, cleanup := fix.Run(
		http.StatusOK,
		`{"items": []}`,
	)
	defer cleanup()

	t.Run("Check instance list", func(t *testing.T) {
		token, err := runner.GetRunnerInstances("the-namespace/the-resource-class")
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(token, []RunnerInstance{}))
	})

	t.Run("Check request", func(t *testing.T) {
		assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/runner", RawQuery: "resource-class=the-namespace%2Fthe-resource-class"}))
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

func (f *fixture) Run(statusCode int, respBody string) (r *Runner, cleanup func()) {
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

	return New(rest.NewFromConfig(server.URL, cfg)), server.Close
}
