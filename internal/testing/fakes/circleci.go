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

// Package fakes provides fake HTTP servers for acceptance testing.
package fakes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/uuid"
)

// CircleCI is a fake CircleCI API server.
type CircleCI struct {
	server *httptest.Server

	mu                                sync.RWMutex
	pipelines                         map[string]any
	projects                          map[string][]any  // project slug → ordered list of pipelines
	workflows                         map[string][]any  // pipeline id → workflows
	workflowDetails                   map[string]any    // workflow id → workflow detail response
	workflowJobs                      map[string][]any  // workflow id → jobs
	jobArtifacts                      map[string][]any  // "slug/jobNumber" → artifacts
	staticFiles                       map[string]string // path → body content, for artifact downloads
	jobs                              map[string]any    // "slug/jobNumber" → job detail response (v2)
	jobsV1                            map[string]any    // "vcs/org/repo/jobNumber" → job detail response (v1.1)
	stepOutputs                       map[string]string // path → JSON log lines content
	triggerResponses                  map[string]any    // project slug → trigger response body
	pipelineDefinitions               map[string][]any  // projectID → list of pipeline definition objects
	createPipelineDefinitionResponses map[string]any    // projectID → response body
	createTriggerResponses            map[string]any    // "projectID/pipelineDefinitionID" → response body
	listTriggerResponses              map[string][]any  // "projectID/pipelineDefinitionID" → list of triggers
	rerunResponses                    map[string]int    // workflow id → HTTP status to return
	cancelResponses                   map[string]int    // workflow id → HTTP status to return
	pipelineCancelResponses           map[string]int    // pipeline id → HTTP status to return

	// Runner (v3) state.
	resourceClasses []any            // all resource classes
	runnerTokens    map[string][]any // resource class → tokens
	runnerInstances []any            // all instances
	deletedTokens   map[string]bool  // token id → deleted
	deletedRCs      map[string]bool  // resource class → deleted

	// Project / env-var state.
	followedProjects []any            // list of project objects for GET /api/v1.1/projects
	followedSlugs    map[string]bool  // vcs+org+repo → true (for follow idempotency)
	envVars          map[string][]any // project slug → env vars
	deletedEnvVars   map[string]bool  // "slug/name" → deleted
	projectInfos     map[string]any   // project slug → project info response

	// Context state.
	contexts                   map[string]any   // context id → context object
	contextsByOrg              map[string][]any // org slug → ordered context objects
	contextEnvVars             map[string][]any // context id → env var objects
	contextRestrictions        map[string][]any // context id → restriction objects
	deletedContexts            map[string]bool  // context id → deleted
	deletedContextVars         map[string]bool  // "contextID/name" → deleted
	deletedContextRestrictions map[string]bool  // "contextID/restrictionID" → deleted

	// Deploy state.
	deploys map[string][]any // project id → deploys

	// iOS code signing state.
	iosCerts          map[string][]any // org id → certificate objects
	iosBundles        map[string][]any // org id → signing bundle objects
	deletedIOSCerts   map[string]bool  // cert id → deleted
	deletedIOSBundles map[string]bool  // bundle id → deleted
	iosCertCounter    int              // monotonic ID generator for uploaded certs
	iosBundleCounter  int              // monotonic ID generator for created bundles

	// Auth state.
	me                 any // response for GET /api/v2/me
	oauthTokenResponse any // response body for POST /oauth/token
	oauthTokenStatus   int // HTTP status for POST /oauth/token (0 → 200 OK)

	// Namespace state (served via /graphql-unstable).
	namespaces        map[string]any    // namespace id → {id, name}
	namespacesByName  map[string]string // namespace name → id
	deletedNamespaces map[string]bool   // namespace id → deleted
}

// NewCircleCI starts a fake CircleCI API server and registers t.Cleanup to close it.
func NewCircleCI(t *testing.T) *CircleCI {
	t.Helper()
	f := &CircleCI{
		pipelines:                         map[string]any{},
		projects:                          map[string][]any{},
		workflows:                         map[string][]any{},
		workflowDetails:                   map[string]any{},
		workflowJobs:                      map[string][]any{},
		jobArtifacts:                      map[string][]any{},
		staticFiles:                       map[string]string{},
		jobs:                              map[string]any{},
		jobsV1:                            map[string]any{},
		stepOutputs:                       map[string]string{},
		triggerResponses:                  map[string]any{},
		pipelineDefinitions:               map[string][]any{},
		createPipelineDefinitionResponses: map[string]any{},
		createTriggerResponses:            map[string]any{},
		listTriggerResponses:              map[string][]any{},
		rerunResponses:                    map[string]int{},
		cancelResponses:                   map[string]int{},
		pipelineCancelResponses:           map[string]int{},
		resourceClasses:                   []any{},
		runnerTokens:                      map[string][]any{},
		runnerInstances:                   []any{},
		deletedTokens:                     map[string]bool{},
		deletedRCs:                        map[string]bool{},
		followedProjects:                  []any{},
		followedSlugs:                     map[string]bool{},
		envVars:                           map[string][]any{},
		deletedEnvVars:                    map[string]bool{},
		contexts:                          map[string]any{},
		contextsByOrg:                     map[string][]any{},
		contextEnvVars:                    map[string][]any{},
		contextRestrictions:               map[string][]any{},
		deletedContexts:                   map[string]bool{},
		deletedContextVars:                map[string]bool{},
		deletedContextRestrictions:        map[string]bool{},
		projectInfos:                      map[string]any{},
		deploys:                           map[string][]any{},
		namespaces:                        map[string]any{},
		namespacesByName:                  map[string]string{},
		deletedNamespaces:                 map[string]bool{},
		iosCerts:                          map[string][]any{},
		iosBundles:                        map[string][]any{},
		deletedIOSCerts:                   map[string]bool{},
		deletedIOSBundles:                 map[string]bool{},
	}

	r := newRouter()
	r.Get("/api/v2/pipeline/{id}", f.handleGetPipeline)
	r.Post("/api/v2/pipeline/{id}/cancel", f.handleCancelPipeline)
	r.Get("/api/v2/pipeline/{id}/workflow", f.handleGetPipelineWorkflows)
	r.Get("/api/v2/workflow/{id}", f.handleGetWorkflowDetail)
	r.Post("/api/v2/workflow/{id}/rerun", f.handleRerunWorkflow)
	r.Post("/api/v2/workflow/{id}/cancel", f.handleCancelWorkflow)
	r.Get("/api/v2/project/{vcs}/{org}/{repo}/pipeline", f.handleListProjectPipelines)
	r.Get("/api/v2/project/{vcs}/{org}/{repo}/pipeline/{number}", f.handleGetPipelineByNumber)
	r.Get("/api/v2/workflow/{id}/job", f.handleGetWorkflowJobs)
	r.Get("/api/v2/project/{vcs}/{org}/{repo}/{jobNumber}/artifacts", f.handleGetJobArtifacts)
	r.Get("/api/v2/project/{vcs}/{org}/{repo}/job/{jobNumber}", f.handleGetJob)
	r.Post("/api/v2/project/{vcs}/{org}/{repo}/pipeline", f.handleTriggerPipeline)
	r.Get("/api/v1.1/project/{vcs}/{org}/{repo}/{jobNumber}", f.handleGetJobV1)
	// Project / env-var routes. These API calls do not URL-encode slashes in the
	// project slug, so we match three separate path segments rather than {slug}.
	r.Get("/api/v1.1/projects", f.handleListProjects)
	r.Post("/api/v1.1/project/{vcs}/{org}/{repo}/follow", f.handleFollowProject)
	r.Get("/api/v2/me", f.handleGetMe)
	r.Post("/oauth/token", f.handleOAuthToken)
	// Context routes.
	r.Get("/api/v2/context", f.handleListContexts)
	r.Post("/api/v2/context", f.handleCreateContext)
	r.Get("/api/v2/context/{id}", f.handleGetContext)
	r.Delete("/api/v2/context/{id}", f.handleDeleteContext)
	r.Get("/api/v2/context/{id}/environment-variable", f.handleListContextEnvVars)
	r.Put("/api/v2/context/{id}/environment-variable/{name}", f.handleSetContextEnvVar)
	r.Delete("/api/v2/context/{id}/environment-variable/{name}", f.handleDeleteContextEnvVar)
	r.Post("/api/v2/context/{id}/restrictions", f.handleCreateContextRestriction)
	r.Delete("/api/v2/context/{id}/restrictions/{restriction_id}", f.handleDeleteContextRestriction)
	r.Get("/api/v2/project/{vcs}/{org}/{repo}/envvar", f.handleListEnvVars)
	r.Post("/api/v2/project/{vcs}/{org}/{repo}/envvar", f.handleSetEnvVar)
	r.Delete("/api/v2/project/{vcs}/{org}/{repo}/envvar/{name}", f.handleDeleteEnvVar)
	r.Get("/api/v2/project/{vcs}/{org}/{repo}", f.handleGetProjectInfo)
	r.Get("/api/v2/projects/{projectID}/pipeline-definitions", f.handleListPipelineDefinitions)
	r.Post("/api/v2/projects/{projectID}/pipeline-definitions", f.handleCreatePipelineDefinition)
	r.Get("/api/v2/projects/{projectID}/pipeline-definitions/{pipelineDefinitionID}/triggers", f.handleListTriggers)
	r.Post("/api/v2/projects/{projectID}/pipeline-definitions/{pipelineDefinitionID}/triggers", f.handleCreateTrigger)
	// Deploy routes.
	r.Get("/api/v2/deploy/projects/{project_id}/releases", f.handleListDeploys)
	// iOS code signing routes.
	r.Post("/api/v2/certificates", f.handleUploadIOSCert)
	r.Get("/api/v2/certificates", f.handleListIOSCerts)
	r.Delete("/api/v2/certificates/{cert_id}", f.handleDeleteIOSCert)
	r.Post("/api/v2/signing-configs", f.handleCreateIOSBundle)
	r.Get("/api/v2/signing-configs", f.handleListIOSBundles)
	r.Delete("/api/v2/signing-configs/{id}", f.handleDeleteIOSBundle)
	// Runner (v3) routes. GET /runner dispatches on query param:
	// ?namespace=  → resource classes, ?resource-class= → instances.
	r.Get("/api/v3/runner", f.handleRunnerList)
	r.Post("/api/v3/runner/resource", f.handleCreateResourceClass)
	r.Delete("/api/v3/runner/resource/{namespace}/{name}", f.handleDeleteResourceClass)
	r.Get("/api/v3/runner/token", f.handleListRunnerTokens)
	r.Post("/api/v3/runner/token", f.handleCreateRunnerToken)
	r.Delete("/api/v3/runner/token/{id}", f.handleDeleteRunnerToken)
	// Namespace (v3) routes.
	r.Get("/api/v3/namespaces", f.handleRESTGetNamespaceByName)
	r.Get("/api/v3/namespaces/{id}", f.handleRESTGetNamespaceByID)
	r.Post("/api/v3/namespaces", f.handleRESTCreateNamespace)
	r.Post("/api/v3/namespaces/{id}/rename", f.handleRESTRenameNamespace)
	r.Delete("/api/v3/namespaces/{id}", f.handleRESTDeleteNamespace)
	// Wildcard routes for downloads and step output — populated via helpers before requests.
	r.Get("/artifacts/*", f.handleStaticFile)
	r.Get("/output/*", f.handleStepOutput)
	// GraphQL endpoint — dispatches by operation within the request body.
	r.Post("/graphql-unstable", f.handleGraphQL)

	f.server = httptest.NewServer(r)
	t.Cleanup(f.server.Close)
	return f
}

// URL returns the base URL of the fake server.
func (f *CircleCI) URL() string {
	return f.server.URL
}

// AddWorkflowDetail registers a workflow detail response for GET /api/v2/workflow/<id>.
func (f *CircleCI) AddWorkflowDetail(id string, detail any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workflowDetails[id] = detail
}

// SetPipelineCancelResponse sets the HTTP status code returned for POST /api/v2/pipeline/<id>/cancel.
// Use http.StatusAccepted (202) for success.
func (f *CircleCI) SetPipelineCancelResponse(pipelineID string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pipelineCancelResponses[pipelineID] = status
}

// SetRerunResponse sets the HTTP status code returned for POST /api/v2/workflow/<id>/rerun.
// Use http.StatusAccepted (202) for success.
func (f *CircleCI) SetRerunResponse(workflowID string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rerunResponses[workflowID] = status
}

// SetCancelResponse sets the HTTP status code returned for POST /api/v2/workflow/<id>/cancel.
// Use http.StatusAccepted (202) for success.
func (f *CircleCI) SetCancelResponse(workflowID string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelResponses[workflowID] = status
}

// AddRun registers a run response for GET /api/v2/pipeline/<id>.
func (f *CircleCI) AddRun(id string, run any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pipelines[id] = run
}

// AddProjectRuns registers runs for GET /api/v2/project/<slug>/pipeline.
// slug should be in "vcs/org/repo" form, e.g. "gh/myorg/myrepo".
func (f *CircleCI) AddProjectRuns(slug string, runs ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projects[slug] = runs
}

// AddRunWorkflows registers workflow responses for a run.
func (f *CircleCI) AddRunWorkflows(runID string, workflows ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workflows[runID] = workflows
}

// AddWorkflowJobs registers job responses for a workflow.
func (f *CircleCI) AddWorkflowJobs(workflowID string, jobs ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workflowJobs[workflowID] = jobs
}

// AddJobArtifacts registers artifact responses for a job.
// slug should be in "vcs/org/repo" form; jobNumber is the integer job number.
func (f *CircleCI) AddJobArtifacts(slug string, jobNumber int64, artifactItems ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fmt.Sprintf("%s/%d", slug, jobNumber)
	f.jobArtifacts[key] = artifactItems
}

// AddJobV1 registers a v1.1 job detail response. Use this alongside AddJob
// (with a job body that has no steps) to exercise the v2→v1.1 fallback path.
// slug should be in the v1.1 form, e.g. "github/org/repo".
func (f *CircleCI) AddJobV1(slug string, jobNumber int64, job any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fmt.Sprintf("%s/%d", slug, jobNumber)
	f.jobsV1[key] = job
}

// SetTriggerResponse registers the response body returned when POST
// /api/v2/project/<slug>/pipeline is called. slug should be in "vcs/org/repo" form.
func (f *CircleCI) SetTriggerResponse(slug string, resp any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.triggerResponses[slug] = resp
}

// AddJob registers a job detail response for GET /api/v2/project/<slug>/job/<number>.
// slug should be in "vcs/org/repo" form; jobNumber is the integer job number.
func (f *CircleCI) AddJob(slug string, jobNumber int64, job any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fmt.Sprintf("%s/%d", slug, jobNumber)
	f.jobs[key] = job
}

// AddStepOutput registers JSON log-line content served at the given path,
// e.g. "/output/job/99/step/0". The path must match the output_url set on the
// fake job's step actions (relative to the fake server URL).
func (f *CircleCI) AddStepOutput(path, content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stepOutputs[path] = content
}

// AddStaticFile registers a path that serves static content for artifact
// download tests. Must be called before any requests are made to the server
// (i.e. before RunCLI). The path should be relative, e.g. "/artifacts/foo.html".
func (f *CircleCI) AddStaticFile(path, content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.staticFiles[path] = content
}

func (f *CircleCI) handleStaticFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	f.mu.RLock()
	content, ok := f.staticFiles[path]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.PlainText(w, r, content)
}

func (f *CircleCI) handleGetPipeline(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	p, ok := f.pipelines[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, p)
}

func (f *CircleCI) handleCancelPipeline(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	status, ok := f.pipelineCancelResponses[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.Status(r, status)
	render.JSON(w, r, map[string]any{"message": "Accepted."})
}

func (f *CircleCI) handleGetPipelineByNumber(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	numberStr := chi.URLParam(r, "number")
	f.mu.RLock()
	pipelines := f.projects[slug]
	f.mu.RUnlock()

	for _, p := range pipelines {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		num := m["number"]
		// number may be int, int64, float64, or json.Number depending on how it was stored
		var numStr string
		switch v := num.(type) {
		case int:
			numStr = strconv.Itoa(v)
		case int64:
			numStr = strconv.FormatInt(v, 10)
		case float64:
			numStr = strconv.FormatInt(int64(v), 10)
		case json.Number:
			numStr = v.String()
		}
		if numStr == numberStr {
			render.JSON(w, r, p)
			return
		}
	}
	render.Status(r, http.StatusNotFound)
	render.JSON(w, r, map[string]any{"message": "not found"})
}

func (f *CircleCI) handleGetPipelineWorkflows(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	_, pipelineExists := f.pipelines[id]
	workflows := f.workflows[id]
	f.mu.RUnlock()

	if !pipelineExists {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	if workflows == nil {
		workflows = []any{}
	}
	render.JSON(w, r, map[string]any{"items": workflows, "next_page_token": nil})
}

func (f *CircleCI) handleGetWorkflowJobs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	jobs := f.workflowJobs[id]
	f.mu.RUnlock()

	if jobs == nil {
		jobs = []any{}
	}
	render.JSON(w, r, map[string]any{"items": jobs, "next_page_token": nil})
}

func (f *CircleCI) handleGetJobArtifacts(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	key := slug + "/" + chi.URLParam(r, "jobNumber")
	f.mu.RLock()
	items := f.jobArtifacts[key]
	f.mu.RUnlock()

	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{"items": items, "next_page_token": nil})
}

func (f *CircleCI) handleListProjectPipelines(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	f.mu.RLock()
	pipelines := f.projects[slug]
	f.mu.RUnlock()

	if pipelines == nil {
		pipelines = []any{}
	}
	render.JSON(w, r, map[string]any{"items": pipelines, "next_page_token": nil})
}

func (f *CircleCI) handleGetJobV1(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	key := slug + "/" + chi.URLParam(r, "jobNumber")
	f.mu.RLock()
	job, ok := f.jobsV1[key]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, job)
}

func (f *CircleCI) handleTriggerPipeline(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	f.mu.RLock()
	resp, ok := f.triggerResponses[slug]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "project not found"})
		return
	}
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, resp)
}

func (f *CircleCI) handleGetJob(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	key := slug + "/" + chi.URLParam(r, "jobNumber")
	f.mu.RLock()
	job, ok := f.jobs[key]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, job)
}

func (f *CircleCI) handleGetWorkflowDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	detail, ok := f.workflowDetails[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, detail)
}

func (f *CircleCI) handleRerunWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	status, ok := f.rerunResponses[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.Status(r, status)
	render.JSON(w, r, map[string]any{"message": "Accepted."})
}

func (f *CircleCI) handleCancelWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	status, ok := f.cancelResponses[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.Status(r, status)
	render.JSON(w, r, map[string]any{"message": "Accepted."})
}

func (f *CircleCI) handleStepOutput(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	f.mu.RLock()
	content, ok := f.stepOutputs[path]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.PlainText(w, r, content)
}

// --- Runner helpers ---

// AddResourceClass adds a resource class to the fake server's list.
func (f *CircleCI) AddResourceClass(rc any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resourceClasses = append(f.resourceClasses, rc)
}

// AddRunnerToken adds a token to the fake server for the given resource class.
func (f *CircleCI) AddRunnerToken(resourceClass string, token any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runnerTokens[resourceClass] = append(f.runnerTokens[resourceClass], token)
}

// AddRunnerInstance adds a runner instance to the fake server's list.
func (f *CircleCI) AddRunnerInstance(instance any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runnerInstances = append(f.runnerInstances, instance)
}

// --- Runner handlers ---

// handleRunnerList dispatches GET /api/v3/runner based on query params:
// ?namespace=  → list resource classes; ?resource-class= → instances.
func (f *CircleCI) handleRunnerList(w http.ResponseWriter, r *http.Request) {
	if rc := r.URL.Query().Get("resource-class"); rc != "" {
		f.handleListRunnerInstances(w, r)
		return
	}
	if ns := r.URL.Query().Get("namespace"); ns != "" {
		f.handleListResourceClasses(w, r)
		return
	}
	render.Status(r, http.StatusBadRequest)
	render.JSON(w, r, map[string]any{"message": "must specify exactly one of resource-class or namespace"})
}

func (f *CircleCI) handleListResourceClasses(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	f.mu.RLock()
	all := f.resourceClasses
	deleted := f.deletedRCs
	f.mu.RUnlock()

	var items []any
	for _, rc := range all {
		m, ok := rc.(map[string]any)
		if !ok {
			items = append(items, rc)
			continue
		}
		slug, _ := m["resource_class"].(string)
		if deleted[slug] {
			continue
		}
		if ns != "" {
			if len(slug) <= len(ns)+1 || slug[:len(ns)+1] != ns+"/" {
				continue
			}
		}
		items = append(items, rc)
	}
	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{"items": items})
}

func (f *CircleCI) handleCreateResourceClass(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}
	slug, _ := body["resource_class"].(string)
	desc, _ := body["description"].(string)
	rc := map[string]any{
		"id":             fmt.Sprintf("rc-%s", slug),
		"resource_class": slug,
		"description":    desc,
	}
	f.mu.Lock()
	f.resourceClasses = append(f.resourceClasses, rc)
	f.mu.Unlock()
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, rc)
}

func (f *CircleCI) handleDeleteResourceClass(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "namespace") + "/" + chi.URLParam(r, "name")
	f.mu.Lock()
	found := false
	for _, rc := range f.resourceClasses {
		m, ok := rc.(map[string]any)
		if !ok {
			continue
		}
		if m["resource_class"] == slug {
			found = true
			break
		}
	}
	if found {
		f.deletedRCs[slug] = true
	}
	f.mu.Unlock()

	if !found {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, map[string]any{"message": "Deleted."})
}

func (f *CircleCI) handleListRunnerTokens(w http.ResponseWriter, r *http.Request) {
	rc := r.URL.Query().Get("resource-class")
	f.mu.RLock()
	tokens := f.runnerTokens[rc]
	deleted := f.deletedTokens
	f.mu.RUnlock()

	var items []any
	for _, tok := range tokens {
		m, ok := tok.(map[string]any)
		if !ok {
			items = append(items, tok)
			continue
		}
		id, _ := m["id"].(string)
		if !deleted[id] {
			items = append(items, tok)
		}
	}
	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{"items": items})
}

func (f *CircleCI) handleCreateRunnerToken(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}
	rc, _ := body["resource_class"].(string)
	nickname, _ := body["nickname"].(string)
	tok := map[string]any{
		"id":             fmt.Sprintf("tok-%s", rc),
		"resource_class": rc,
		"nickname":       nickname,
		"created_at":     "2026-01-01T00:00:00Z",
		"token":          "fake-runner-token-value",
	}
	f.mu.Lock()
	f.runnerTokens[rc] = append(f.runnerTokens[rc], tok)
	f.mu.Unlock()
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, tok)
}

func (f *CircleCI) handleDeleteRunnerToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.Lock()
	found := false
	for _, tokens := range f.runnerTokens {
		for _, tok := range tokens {
			m, ok := tok.(map[string]any)
			if !ok {
				continue
			}
			if m["id"] == id {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if found {
		f.deletedTokens[id] = true
	}
	f.mu.Unlock()

	if !found {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, map[string]any{"message": "Deleted."})
}

func (f *CircleCI) handleListRunnerInstances(w http.ResponseWriter, r *http.Request) {
	rc := r.URL.Query().Get("resource-class")
	f.mu.RLock()
	all := f.runnerInstances
	f.mu.RUnlock()

	var items []any
	for _, inst := range all {
		if rc == "" {
			items = append(items, inst)
			continue
		}
		m, ok := inst.(map[string]any)
		if !ok {
			items = append(items, inst)
			continue
		}
		if m["resource_class"] == rc {
			items = append(items, inst)
		}
	}
	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{"items": items})
}

// --- Auth helpers ---

// SetMe sets the response body for GET /api/v2/me.
func (f *CircleCI) SetMe(me any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.me = me
}

func (f *CircleCI) handleGetMe(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	me := f.me
	f.mu.RUnlock()

	if me == nil {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]any{"message": "unauthorized"})
		return
	}
	render.JSON(w, r, me)
}

// SetOAuthTokenResponse sets the response body for POST /oauth/token.
func (f *CircleCI) SetOAuthTokenResponse(resp any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.oauthTokenResponse = resp
}

// SetOAuthTokenError configures POST /oauth/token to return the given status
// and JSON body. Use for testing token-exchange failure paths.
func (f *CircleCI) SetOAuthTokenError(status int, resp any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.oauthTokenStatus = status
	f.oauthTokenResponse = resp
}

func (f *CircleCI) handleOAuthToken(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	resp := f.oauthTokenResponse
	status := f.oauthTokenStatus
	f.mu.RUnlock()

	if resp == nil {
		resp = map[string]any{
			"access_token":  "fake-access-token",
			"token_type":    "Bearer",
			"expires_in":    int64(3600),
			"refresh_token": "fake-refresh-token",
		}
	}
	if status != 0 {
		render.Status(r, status)
	}
	render.JSON(w, r, resp)
}

// --- Project / env-var helpers ---

// AddProjectInfo registers a project info response for GET /api/v2/project/<slug>.
// slug should be in "vcs/org/repo" form.
func (f *CircleCI) AddProjectInfo(slug string, info any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projectInfos[slug] = info
}

// AddPipelineDefinition registers a pipeline definition for a project, returned by
// GET /api/v2/projects/{projectID}/pipeline-definitions.
func (f *CircleCI) AddPipelineDefinition(projectID string, def any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pipelineDefinitions[projectID] = append(f.pipelineDefinitions[projectID], def)
}

// SetCreatePipelineDefinitionResponse registers the response body returned when
// POST /api/v2/projects/{projectID}/pipeline-definitions is called.
// Pass nil to simulate a 404.
func (f *CircleCI) SetCreatePipelineDefinitionResponse(projectID string, resp any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createPipelineDefinitionResponses[projectID] = resp
}

// AddTrigger registers a trigger returned by GET
// /api/v2/projects/{projectID}/pipeline-definitions/{pipelineDefinitionID}/triggers.
func (f *CircleCI) AddTrigger(projectID, pipelineDefinitionID string, trigger any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := projectID + "/" + pipelineDefinitionID
	f.listTriggerResponses[key] = append(f.listTriggerResponses[key], trigger)
}

// SetCreateTriggerResponse registers the response body returned when POST
// /api/v2/projects/{projectID}/pipeline-definitions/{pipelineDefinitionID}/triggers
// is called. Pass nil to simulate a 404.
func (f *CircleCI) SetCreateTriggerResponse(projectID, pipelineDefinitionID string, resp any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createTriggerResponses[projectID+"/"+pipelineDefinitionID] = resp
}

// AddFollowedProject registers a project returned by GET /api/v1.1/projects.
// proj should be a map or struct with at least "slug", "username", and "reponame" fields.
func (f *CircleCI) AddFollowedProject(proj any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.followedProjects = append(f.followedProjects, proj)
}

// AddEnvVar registers an env var for a project.
// slug should be in "vcs/org/repo" form.
func (f *CircleCI) AddEnvVar(slug, name, value string, createdAt *time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envVars[slug] = append(f.envVars[slug], map[string]any{
		"name":       name,
		"value":      value,
		"created_at": createdAt,
	})
}

// --- Project / env-var handlers ---

func (f *CircleCI) handleListProjects(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	projects := f.followedProjects
	f.mu.RUnlock()

	if projects == nil {
		projects = []any{}
	}
	render.JSON(w, r, projects)
}

func (f *CircleCI) handleFollowProject(w http.ResponseWriter, r *http.Request) {
	vcs := chi.URLParam(r, "vcs")
	org := chi.URLParam(r, "org")
	repo := chi.URLParam(r, "repo")
	slug := vcs + "/" + org + "/" + repo

	f.mu.Lock()
	if !f.followedSlugs[slug] {
		f.followedSlugs[slug] = true
		f.followedProjects = append(f.followedProjects, map[string]any{
			"slug":     slug,
			"username": org,
			"reponame": repo,
			"vcs_type": vcs,
		})
	}
	f.mu.Unlock()

	render.JSON(w, r, map[string]any{"following": true})
}

func (f *CircleCI) handleListEnvVars(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	f.mu.RLock()
	vars := f.envVars[slug]
	deleted := f.deletedEnvVars
	f.mu.RUnlock()

	var items []any
	for _, v := range vars {
		m, ok := v.(map[string]any)
		if !ok {
			items = append(items, v)
			continue
		}
		name, _ := m["name"].(string)
		key := slug + "/" + name
		if !deleted[key] {
			items = append(items, v)
		}
	}
	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{"items": items, "next_page_token": nil})
}

func (f *CircleCI) handleSetEnvVar(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}
	name, _ := body["name"].(string)
	value, _ := body["value"].(string)

	ev := map[string]any{"name": name, "value": value}
	f.mu.Lock()
	// Remove any existing var with this name.
	var kept []any
	for _, v := range f.envVars[slug] {
		m, ok := v.(map[string]any)
		if ok && m["name"] == name {
			continue
		}
		kept = append(kept, v)
	}
	f.envVars[slug] = append(kept, ev)
	delete(f.deletedEnvVars, slug+"/"+name)
	f.mu.Unlock()

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, ev)
}

// --- Context helpers ---

// AddContext registers a context object for GET /api/v2/context/{id}.
// ctx should be a map with at least "id", "name", and "created_at" fields.
// It is also indexed by org slug for list responses.
func (f *CircleCI) AddContext(orgSlug string, ctx any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := ctx.(map[string]any)
	if ok {
		if id, _ := m["id"].(string); id != "" {
			f.contexts[id] = ctx
		}
	}
	f.contextsByOrg[orgSlug] = append(f.contextsByOrg[orgSlug], ctx)
}

// AddContextEnvVar registers an environment variable for a context.
func (f *CircleCI) AddContextEnvVar(contextID string, envVar any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.contextEnvVars[contextID] = append(f.contextEnvVars[contextID], envVar)
}

// AddContextRestriction registers a restriction for a context.
func (f *CircleCI) AddContextRestriction(contextID string, restriction any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.contextRestrictions[contextID] = append(f.contextRestrictions[contextID], restriction)
}

// --- Context handlers ---

func (f *CircleCI) handleListContexts(w http.ResponseWriter, r *http.Request) {
	ownerSlug := r.URL.Query().Get("owner-slug")
	nameFilter := r.URL.Query().Get("name")
	f.mu.RLock()
	items := f.contextsByOrg[ownerSlug]
	deleted := f.deletedContexts
	f.mu.RUnlock()

	var result []any
	for _, ctx := range items {
		m, ok := ctx.(map[string]any)
		if ok {
			if id, _ := m["id"].(string); deleted[id] {
				continue
			}
			if nameFilter != "" {
				if name, _ := m["name"].(string); !strings.Contains(strings.ToLower(name), strings.ToLower(nameFilter)) {
					continue
				}
			}
		}
		result = append(result, ctx)
	}
	if result == nil {
		result = []any{}
	}
	render.JSON(w, r, map[string]any{"items": result, "next_page_token": nil})
}

func (f *CircleCI) handleCreateContext(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}
	name, _ := body["name"].(string)
	var orgSlug string
	if owner, ok := body["owner"].(map[string]any); ok {
		orgSlug, _ = owner["slug"].(string)
	}
	id := "c0000099-0000-4000-8000-000000000099"
	ctx := map[string]any{
		"id":         id,
		"name":       name,
		"created_at": "2026-01-01T00:00:00Z",
	}
	f.mu.Lock()
	f.contexts[id] = ctx
	if orgSlug != "" {
		f.contextsByOrg[orgSlug] = append(f.contextsByOrg[orgSlug], ctx)
	}
	f.mu.Unlock()
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, ctx)
}

func (f *CircleCI) handleGetContext(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	ctx, ok := f.contexts[id]
	deleted := f.deletedContexts[id]
	vars := f.contextEnvVars[id]
	restrictions := f.contextRestrictions[id]
	deletedVars := f.deletedContextVars
	deletedRestrictions := f.deletedContextRestrictions
	f.mu.RUnlock()

	if !ok || deleted {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}

	// Build a ContextDetail-shaped response with env vars embedded.
	var liveVars []any
	for _, v := range vars {
		m, ok := v.(map[string]any)
		if ok {
			name, _ := m["variable"].(string)
			if deletedVars[id+"/"+name] {
				continue
			}
		}
		liveVars = append(liveVars, v)
	}
	if liveVars == nil {
		liveVars = []any{}
	}
	var liveRestrictions []any
	for _, restr := range restrictions {
		m, ok := restr.(map[string]any)
		if ok {
			rid, _ := m["id"].(string)
			if deletedRestrictions[id+"/"+rid] {
				continue
			}
		}
		liveRestrictions = append(liveRestrictions, restr)
	}
	if liveRestrictions == nil {
		liveRestrictions = []any{}
	}

	m, _ := ctx.(map[string]any)
	detail := map[string]any{
		"id":                    m["id"],
		"name":                  m["name"],
		"created_at":            m["created_at"],
		"org_id":                "00000000-0000-0000-0000-000000000000",
		"environment_variables": liveVars,
		"restrictions":          liveRestrictions,
	}
	render.JSON(w, r, detail)
}

func (f *CircleCI) handleDeleteContext(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.Lock()
	_, ok := f.contexts[id]
	if ok {
		f.deletedContexts[id] = true
	}
	f.mu.Unlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, map[string]any{"message": "Deleted."})
}

func (f *CircleCI) handleListContextEnvVars(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	vars := f.contextEnvVars[id]
	deleted := f.deletedContextVars
	f.mu.RUnlock()

	var items []any
	for _, v := range vars {
		m, ok := v.(map[string]any)
		if ok {
			name, _ := m["variable"].(string)
			if deleted[id+"/"+name] {
				continue
			}
		}
		items = append(items, v)
	}
	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{"items": items, "next_page_token": nil})
}

func (f *CircleCI) handleSetContextEnvVar(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	name := chi.URLParam(r, "name")
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}
	ev := map[string]any{
		"variable":   name,
		"context_id": id,
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z",
	}
	f.mu.Lock()
	// Remove existing var with same name.
	var kept []any
	for _, v := range f.contextEnvVars[id] {
		m, ok := v.(map[string]any)
		if ok && m["variable"] == name {
			continue
		}
		kept = append(kept, v)
	}
	f.contextEnvVars[id] = append(kept, ev)
	delete(f.deletedContextVars, id+"/"+name)
	f.mu.Unlock()
	render.JSON(w, r, ev)
}

func (f *CircleCI) handleDeleteContextEnvVar(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	name := chi.URLParam(r, "name")
	key := id + "/" + name

	f.mu.Lock()
	found := false
	for _, v := range f.contextEnvVars[id] {
		m, ok := v.(map[string]any)
		if ok && m["variable"] == name {
			found = true
			break
		}
	}
	if found {
		f.deletedContextVars[key] = true
	}
	f.mu.Unlock()

	if !found {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, map[string]any{"message": "Deleted."})
}

func (f *CircleCI) handleCreateContextRestriction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	_, ok := f.contexts[id]
	deleted := f.deletedContexts[id]
	f.mu.RUnlock()

	if !ok || deleted {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}
	restrictionType, _ := body["restriction_type"].(string)
	restrictionValue, _ := body["restriction_value"].(string)
	restr := map[string]any{
		"id":                "c0000003-0000-4000-8000-000000000003",
		"name":              "",
		"restriction_type":  restrictionType,
		"restriction_value": restrictionValue,
	}
	f.mu.Lock()
	f.contextRestrictions[id] = append(f.contextRestrictions[id], restr)
	f.mu.Unlock()
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, restr)
}

func (f *CircleCI) handleDeleteContextRestriction(w http.ResponseWriter, r *http.Request) {
	contextID := chi.URLParam(r, "id")
	restrictionID := chi.URLParam(r, "restriction_id")
	key := contextID + "/" + restrictionID

	f.mu.Lock()
	found := false
	for _, restr := range f.contextRestrictions[contextID] {
		m, ok := restr.(map[string]any)
		if ok && m["id"] == restrictionID {
			found = true
			break
		}
	}
	if found {
		f.deletedContextRestrictions[key] = true
	}
	f.mu.Unlock()

	if !found {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, map[string]any{"message": "Context restriction deleted."})
}

func (f *CircleCI) handleListPipelineDefinitions(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	f.mu.RLock()
	items := f.pipelineDefinitions[projectID]
	f.mu.RUnlock()

	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{"items": items})
}

func (f *CircleCI) handleCreatePipelineDefinition(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	f.mu.RLock()
	resp, ok := f.createPipelineDefinitionResponses[projectID]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, resp)
}

func (f *CircleCI) handleListTriggers(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	pipelineDefinitionID := chi.URLParam(r, "pipelineDefinitionID")
	key := projectID + "/" + pipelineDefinitionID
	f.mu.RLock()
	items := f.listTriggerResponses[key]
	f.mu.RUnlock()

	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{"items": items})
}

func (f *CircleCI) handleCreateTrigger(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	pipelineDefinitionID := chi.URLParam(r, "pipelineDefinitionID")
	key := projectID + "/" + pipelineDefinitionID
	f.mu.RLock()
	resp, ok := f.createTriggerResponses[key]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, resp)
}

func (f *CircleCI) handleGetProjectInfo(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	f.mu.RLock()
	info, ok := f.projectInfos[slug]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, info)
}

// --- Deploy helpers ---

// AddDeploy registers a deploy for a project, returned by
// GET /api/v2/deploy/projects/{project_id}/releases.
func (f *CircleCI) AddDeploy(projectID string, deploy any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deploys[projectID] = append(f.deploys[projectID], deploy)
}

func (f *CircleCI) handleListDeploys(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "project_id")
	f.mu.RLock()
	items := f.deploys[id]
	f.mu.RUnlock()

	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{"items": items, "next_page_token": nil})
}

func (f *CircleCI) handleDeleteEnvVar(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	name := chi.URLParam(r, "name")
	key := slug + "/" + name

	f.mu.Lock()
	found := false
	for _, v := range f.envVars[slug] {
		m, ok := v.(map[string]any)
		if ok && m["name"] == name {
			found = true
			break
		}
	}
	if found {
		f.deletedEnvVars[key] = true
	}
	f.mu.Unlock()

	if !found {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, map[string]any{"message": "Deleted."})
}

// AddNamespace registers a namespace for REST API queries.
// id and name form the namespace record returned by the /api/v3/namespaces endpoints.
func (f *CircleCI) AddNamespace(id, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.namespaces[id] = map[string]any{"id": id, "name": name}
	f.namespacesByName[name] = id
}

func namespaceDataResponse(id, name string) map[string]any {
	return map[string]any{
		"data": map[string]any{
			"id":         id,
			"attributes": map[string]any{"name": name},
		},
	}
}

func (f *CircleCI) handleRESTGetNamespaceByName(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("filter[name]")
	if name == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"error": map[string]any{"title": "Bad Request", "detail": "filter[name] is required"}})
		return
	}
	f.mu.RLock()
	id, ok := f.namespacesByName[name]
	var deleted bool
	if ok {
		deleted = f.deletedNamespaces[id]
	}
	f.mu.RUnlock()

	if !ok || deleted {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"error": map[string]any{"title": "Not Found", "detail": "namespace not found"}})
		return
	}
	render.JSON(w, r, namespaceDataResponse(id, name))
}

func (f *CircleCI) handleRESTGetNamespaceByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	ns, ok := f.namespaces[id]
	deleted := f.deletedNamespaces[id]
	f.mu.RUnlock()

	if !ok || deleted {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"error": map[string]any{"title": "Not Found", "detail": "namespace not found"}})
		return
	}
	name, _ := ns.(map[string]any)["name"].(string)
	render.JSON(w, r, namespaceDataResponse(id, name))
}

func (f *CircleCI) handleRESTCreateNamespace(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		OrgID string `json:"org_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"error": map[string]any{"title": "Bad Request", "detail": "name is required"}})
		return
	}
	id := uuid.New().String()
	f.mu.Lock()
	f.namespaces[id] = map[string]any{"id": id, "name": body.Name}
	f.namespacesByName[body.Name] = id
	f.mu.Unlock()

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, namespaceDataResponse(id, body.Name))
}

func (f *CircleCI) handleRESTRenameNamespace(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"error": map[string]any{"title": "Bad Request", "detail": "name is required"}})
		return
	}
	f.mu.Lock()
	ns, ok := f.namespaces[id]
	deleted := f.deletedNamespaces[id]
	if ok && !deleted {
		oldName, _ := ns.(map[string]any)["name"].(string)
		delete(f.namespacesByName, oldName)
		f.namespaces[id] = map[string]any{"id": id, "name": body.Name}
		f.namespacesByName[body.Name] = id
	}
	f.mu.Unlock()

	if !ok || deleted {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"error": map[string]any{"title": "Not Found", "detail": "namespace not found"}})
		return
	}
	render.JSON(w, r, namespaceDataResponse(id, body.Name))
}

func (f *CircleCI) handleRESTDeleteNamespace(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.Lock()
	_, ok := f.namespaces[id]
	alreadyDeleted := f.deletedNamespaces[id]
	if ok && !alreadyDeleted {
		f.deletedNamespaces[id] = true
	}
	f.mu.Unlock()

	if !ok || alreadyDeleted {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"error": map[string]any{"title": "Not Found", "detail": "namespace not found"}})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGraphQL dispatches GraphQL operations by the operationName field sent by the client.
func (f *CircleCI) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query         string         `json:"query"`
		OperationName string         `json:"operationName"`
		Variables     map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"errors": []any{map[string]any{"message": "invalid request body"}}})
		return
	}

	render.JSON(w, r, map[string]any{"errors": []any{map[string]any{"message": "unknown operation: " + body.OperationName}}})
}

// --- iOS code signing helpers ---

// AddIOSCert registers an iOS certificate for an org, returned by
// GET /api/v2/certificates?org-id=<orgID>.
func (f *CircleCI) AddIOSCert(orgID string, cert any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.iosCerts[orgID] = append(f.iosCerts[orgID], cert)
}

// AddIOSBundle registers an iOS signing bundle for an org, returned by
// GET /api/v2/signing-configs?org-id=<orgID>.
func (f *CircleCI) AddIOSBundle(orgID string, bundle any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.iosBundles[orgID] = append(f.iosBundles[orgID], bundle)
}

// DeletedIOSCert reports whether the given cert ID was deleted.
func (f *CircleCI) DeletedIOSCert(certID string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.deletedIOSCerts[certID]
}

// DeletedIOSBundle reports whether the given bundle ID was deleted.
func (f *CircleCI) DeletedIOSBundle(id string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.deletedIOSBundles[id]
}

func (f *CircleCI) handleUploadIOSCert(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OrgID    string `json:"org_id"`
		FileName string `json:"cert_file_name"`
		Blob     string `json:"cert_blob"`
		Password string `json:"cert_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": err.Error()})
		return
	}
	if body.OrgID == "" || body.FileName == "" || body.Blob == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "missing required fields"})
		return
	}
	f.mu.Lock()
	f.iosCertCounter++
	certID := fmt.Sprintf("cert-%05d", f.iosCertCounter)
	f.iosCerts[body.OrgID] = append(f.iosCerts[body.OrgID], map[string]any{
		"id":        certID,
		"file_name": body.FileName,
		"cert_type": "distribution",
	})
	f.mu.Unlock()
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]any{"id": certID})
}

func (f *CircleCI) handleListIOSCerts(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org-id")
	f.mu.RLock()
	all := f.iosCerts[orgID]
	deleted := make(map[string]bool, len(f.deletedIOSCerts))
	for k, v := range f.deletedIOSCerts {
		deleted[k] = v
	}
	f.mu.RUnlock()

	items := make([]any, 0, len(all))
	for _, c := range all {
		if m, ok := c.(map[string]any); ok {
			if id, _ := m["id"].(string); id != "" && deleted[id] {
				continue
			}
		}
		items = append(items, c)
	}
	render.JSON(w, r, map[string]any{"items": items})
}

func (f *CircleCI) handleDeleteIOSCert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "cert_id")
	f.mu.Lock()
	found := false
	for _, certs := range f.iosCerts {
		for _, c := range certs {
			if m, ok := c.(map[string]any); ok && m["id"] == id {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	// Reject if any live signing config references this cert.
	inUse := false
	if found {
		for _, bundles := range f.iosBundles {
			for _, b := range bundles {
				m, ok := b.(map[string]any)
				if !ok || m["_cert_id"] != id {
					continue
				}
				if bid, _ := m["id"].(string); bid != "" && !f.deletedIOSBundles[bid] {
					inUse = true
					break
				}
			}
			if inUse {
				break
			}
		}
	}
	if found && !inUse {
		f.deletedIOSCerts[id] = true
	}
	f.mu.Unlock()

	if !found {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	if inUse {
		render.Status(r, http.StatusConflict)
		render.JSON(w, r, map[string]any{"message": "certificate is in use by one or more signing configurations"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (f *CircleCI) handleCreateIOSBundle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name                 string           `json:"name"`
		OrgID                string           `json:"org_id"`
		CertID               string           `json:"cert_id"`
		ProvisioningProfiles []map[string]any `json:"provisioning_profiles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": err.Error()})
		return
	}
	if body.Name == "" || body.OrgID == "" || body.CertID == "" || len(body.ProvisioningProfiles) == 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "missing required fields"})
		return
	}
	f.mu.Lock()

	// Reject if no live cert with the given id exists in this org.
	var certRef map[string]any
	for _, c := range f.iosCerts[body.OrgID] {
		m, ok := c.(map[string]any)
		if !ok || m["id"] != body.CertID {
			continue
		}
		if id, _ := m["id"].(string); f.deletedIOSCerts[id] {
			continue
		}
		certRef = map[string]any{
			"file_name": m["file_name"],
			"cert_type": m["cert_type"],
		}
		break
	}
	if certRef == nil {
		f.mu.Unlock()
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "certificate not found"})
		return
	}

	// Reject if a live signing config already uses this name in this org.
	for _, b := range f.iosBundles[body.OrgID] {
		m, ok := b.(map[string]any)
		if !ok || m["name"] != body.Name {
			continue
		}
		if bid, _ := m["id"].(string); bid != "" && f.deletedIOSBundles[bid] {
			continue
		}
		f.mu.Unlock()
		render.Status(r, http.StatusConflict)
		render.JSON(w, r, map[string]any{"message": "a signing configuration with this name already exists"})
		return
	}

	f.iosBundleCounter++
	id := fmt.Sprintf("bundle-%05d", f.iosBundleCounter)

	// Provisioning-profile list response echoes only file_name, not the blob.
	profiles := make([]map[string]any, len(body.ProvisioningProfiles))
	for i, p := range body.ProvisioningProfiles {
		profiles[i] = map[string]any{"file_name": p["file_name"]}
	}

	stored := map[string]any{
		"id":                    id,
		"name":                  body.Name,
		"certificate":           certRef,
		"provisioning_profiles": profiles,
		// Internal-only — used by handleDeleteIOSCert's in-use check; not
		// part of the real API response shape but harmless extras for the
		// CLI, which only decodes documented fields.
		"_cert_id": body.CertID,
		"_org_id":  body.OrgID,
	}
	f.iosBundles[body.OrgID] = append(f.iosBundles[body.OrgID], stored)
	f.mu.Unlock()
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]any{"id": id})
}

func (f *CircleCI) handleListIOSBundles(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org-id")
	f.mu.RLock()
	all := f.iosBundles[orgID]
	deleted := make(map[string]bool, len(f.deletedIOSBundles))
	for k, v := range f.deletedIOSBundles {
		deleted[k] = v
	}
	f.mu.RUnlock()

	items := make([]any, 0, len(all))
	for _, b := range all {
		m, ok := b.(map[string]any)
		if !ok {
			items = append(items, b)
			continue
		}
		if id, _ := m["id"].(string); id != "" && deleted[id] {
			continue
		}
		// Strip internal-only fields (prefixed with "_") so the wire shape
		// matches what the real API returns.
		clean := make(map[string]any, len(m))
		for k, v := range m {
			if strings.HasPrefix(k, "_") {
				continue
			}
			clean[k] = v
		}
		items = append(items, clean)
	}
	render.JSON(w, r, map[string]any{"items": items})
}

func (f *CircleCI) handleDeleteIOSBundle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.Lock()
	found := false
	for _, bundles := range f.iosBundles {
		for _, b := range bundles {
			if m, ok := b.(map[string]any); ok && m["id"] == id {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if found {
		f.deletedIOSBundles[id] = true
	}
	f.mu.Unlock()

	if !found {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
