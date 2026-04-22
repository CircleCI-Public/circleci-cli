// Package fakes provides fake HTTP servers for acceptance testing.
package fakes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// CircleCI is a fake CircleCI API server.
type CircleCI struct {
	server *httptest.Server

	mu                      sync.RWMutex
	pipelines               map[string]any
	projects                map[string][]any  // project slug → ordered list of pipelines
	workflows               map[string][]any  // pipeline id → workflows
	workflowDetails         map[string]any    // workflow id → workflow detail response
	workflowJobs            map[string][]any  // workflow id → jobs
	jobArtifacts            map[string][]any  // "slug/jobNumber" → artifacts
	staticFiles             map[string]string // path → body content, for artifact downloads
	jobs                    map[string]any    // "slug/jobNumber" → job detail response (v2)
	jobsV1                  map[string]any    // "vcs/org/repo/jobNumber" → job detail response (v1.1)
	stepOutputs             map[string]string // path → JSON log lines content
	triggerResponses        map[string]any    // project slug → trigger response body
	rerunResponses          map[string]int    // workflow id → HTTP status to return
	cancelResponses         map[string]int    // workflow id → HTTP status to return
	pipelineCancelResponses map[string]int    // pipeline id → HTTP status to return

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
}

// NewCircleCI starts a fake CircleCI API server and registers t.Cleanup to close it.
func NewCircleCI(t *testing.T) *CircleCI {
	t.Helper()
	f := &CircleCI{
		pipelines:               map[string]any{},
		projects:                map[string][]any{},
		workflows:               map[string][]any{},
		workflowDetails:         map[string]any{},
		workflowJobs:            map[string][]any{},
		jobArtifacts:            map[string][]any{},
		staticFiles:             map[string]string{},
		jobs:                    map[string]any{},
		jobsV1:                  map[string]any{},
		stepOutputs:             map[string]string{},
		triggerResponses:        map[string]any{},
		rerunResponses:          map[string]int{},
		cancelResponses:         map[string]int{},
		pipelineCancelResponses: map[string]int{},
		resourceClasses:         []any{},
		runnerTokens:            map[string][]any{},
		runnerInstances:         []any{},
		deletedTokens:           map[string]bool{},
		deletedRCs:              map[string]bool{},
		followedProjects:        []any{},
		followedSlugs:           map[string]bool{},
		envVars:                 map[string][]any{},
		deletedEnvVars:          map[string]bool{},
	}

	r := newRouter()
	r.GET("/api/v2/pipeline/:id", f.handleGetPipeline)
	r.POST("/api/v2/pipeline/:id/cancel", f.handleCancelPipeline)
	r.GET("/api/v2/pipeline/:id/workflow", f.handleGetPipelineWorkflows)
	r.GET("/api/v2/workflow/:id", f.handleGetWorkflowDetail)
	r.POST("/api/v2/workflow/:id/rerun", f.handleRerunWorkflow)
	r.POST("/api/v2/workflow/:id/cancel", f.handleCancelWorkflow)
	r.GET("/api/v2/project/:vcs/:org/:repo/pipeline", f.handleListProjectPipelines)
	r.GET("/api/v2/project/:vcs/:org/:repo/pipeline/:number", f.handleGetPipelineByNumber)
	r.GET("/api/v2/workflow/:id/job", f.handleGetWorkflowJobs)
	r.GET("/api/v2/project/:vcs/:org/:repo/:jobNumber/artifacts", f.handleGetJobArtifacts)
	r.GET("/api/v2/project/:vcs/:org/:repo/job/:jobNumber", f.handleGetJob)
	r.POST("/api/v2/project/:vcs/:org/:repo/pipeline", f.handleTriggerPipeline)
	r.GET("/api/v1.1/project/:vcs/:org/:repo/:jobNumber", f.handleGetJobV1)
	// Project / env-var routes.
	r.GET("/api/v1.1/projects", f.handleListProjects)
	r.POST("/api/v1.1/project/:vcs/:org/:repo/follow", f.handleFollowProject)
	r.GET("/api/v2/project/:vcs/:org/:repo/envvar", f.handleListEnvVars)
	r.POST("/api/v2/project/:vcs/:org/:repo/envvar", f.handleSetEnvVar)
	r.DELETE("/api/v2/project/:vcs/:org/:repo/envvar/:name", f.handleDeleteEnvVar)
	// Runner (v3) routes. GET /runner dispatches on query param:
	// ?namespace=  → resource classes, ?resource-class= → instances.
	r.GET("/api/v3/runner", f.handleRunnerList)
	r.POST("/api/v3/runner/resource", f.handleCreateResourceClass)
	r.DELETE("/api/v3/runner/resource/:namespace/:name", f.handleDeleteResourceClass)
	r.GET("/api/v3/runner/token", f.handleListRunnerTokens)
	r.POST("/api/v3/runner/token", f.handleCreateRunnerToken)
	r.DELETE("/api/v3/runner/token/:id", f.handleDeleteRunnerToken)
	// Wildcard routes for downloads and step output — populated via helpers before requests.
	r.GET("/artifacts/*path", f.handleStaticFile)
	r.GET("/output/*path", f.handleStepOutput)

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

// AddPipeline registers a pipeline response for GET /api/v2/pipeline/<id>.
func (f *CircleCI) AddPipeline(id string, pipeline any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pipelines[id] = pipeline
}

// AddProjectPipelines registers pipelines for GET /api/v2/project/<slug>/pipeline.
// slug should be in "vcs/org/repo" form, e.g. "gh/myorg/myrepo".
func (f *CircleCI) AddProjectPipelines(slug string, pipelines ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projects[slug] = pipelines
}

// AddPipelineWorkflows registers workflow responses for a pipeline.
func (f *CircleCI) AddPipelineWorkflows(pipelineID string, workflows ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workflows[pipelineID] = workflows
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

func (f *CircleCI) handleStaticFile(c *gin.Context) {
	path := "/artifacts" + c.Param("path")
	f.mu.RLock()
	content, ok := f.staticFiles[path]
	f.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	c.String(http.StatusOK, content)
}

func (f *CircleCI) handleGetPipeline(c *gin.Context) {
	id := c.Param("id")
	f.mu.RLock()
	p, ok := f.pipelines[id]
	f.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (f *CircleCI) handleCancelPipeline(c *gin.Context) {
	id := c.Param("id")
	f.mu.RLock()
	status, ok := f.pipelineCancelResponses[id]
	f.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	c.JSON(status, gin.H{"message": "Accepted."})
}

func (f *CircleCI) handleGetPipelineByNumber(c *gin.Context) {
	slug := c.Param("vcs") + "/" + c.Param("org") + "/" + c.Param("repo")
	numberStr := c.Param("number")
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
			c.JSON(http.StatusOK, p)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
}

func (f *CircleCI) handleGetPipelineWorkflows(c *gin.Context) {
	id := c.Param("id")
	f.mu.RLock()
	wflows, pipelineExists := f.pipelines[id]
	workflows := f.workflows[id]
	f.mu.RUnlock()

	if !pipelineExists {
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	_ = wflows
	if workflows == nil {
		workflows = []any{}
	}
	c.JSON(http.StatusOK, gin.H{"items": workflows, "next_page_token": nil})
}

func (f *CircleCI) handleGetWorkflowJobs(c *gin.Context) {
	id := c.Param("id")
	f.mu.RLock()
	jobs := f.workflowJobs[id]
	f.mu.RUnlock()

	if jobs == nil {
		jobs = []any{}
	}
	c.JSON(http.StatusOK, gin.H{"items": jobs, "next_page_token": nil})
}

func (f *CircleCI) handleGetJobArtifacts(c *gin.Context) {
	slug := c.Param("vcs") + "/" + c.Param("org") + "/" + c.Param("repo")
	key := slug + "/" + c.Param("jobNumber")
	f.mu.RLock()
	items := f.jobArtifacts[key]
	f.mu.RUnlock()

	if items == nil {
		items = []any{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "next_page_token": nil})
}

func (f *CircleCI) handleListProjectPipelines(c *gin.Context) {
	slug := c.Param("vcs") + "/" + c.Param("org") + "/" + c.Param("repo")
	f.mu.RLock()
	pipelines := f.projects[slug]
	f.mu.RUnlock()

	if pipelines == nil {
		pipelines = []any{}
	}
	c.JSON(http.StatusOK, gin.H{"items": pipelines, "next_page_token": nil})
}

func (f *CircleCI) handleGetJobV1(c *gin.Context) {
	slug := c.Param("vcs") + "/" + c.Param("org") + "/" + c.Param("repo")
	key := slug + "/" + c.Param("jobNumber")
	f.mu.RLock()
	job, ok := f.jobsV1[key]
	f.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (f *CircleCI) handleTriggerPipeline(c *gin.Context) {
	slug := c.Param("vcs") + "/" + c.Param("org") + "/" + c.Param("repo")
	f.mu.RLock()
	resp, ok := f.triggerResponses[slug]
	f.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "project not found"})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (f *CircleCI) handleGetJob(c *gin.Context) {
	slug := c.Param("vcs") + "/" + c.Param("org") + "/" + c.Param("repo")
	key := slug + "/" + c.Param("jobNumber")
	f.mu.RLock()
	job, ok := f.jobs[key]
	f.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (f *CircleCI) handleGetWorkflowDetail(c *gin.Context) {
	id := c.Param("id")
	f.mu.RLock()
	detail, ok := f.workflowDetails[id]
	f.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (f *CircleCI) handleRerunWorkflow(c *gin.Context) {
	id := c.Param("id")
	f.mu.RLock()
	status, ok := f.rerunResponses[id]
	f.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	c.JSON(status, gin.H{"message": "Accepted."})
}

func (f *CircleCI) handleCancelWorkflow(c *gin.Context) {
	id := c.Param("id")
	f.mu.RLock()
	status, ok := f.cancelResponses[id]
	f.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	c.JSON(status, gin.H{"message": "Accepted."})
}

func (f *CircleCI) handleStepOutput(c *gin.Context) {
	path := "/output" + c.Param("path")
	f.mu.RLock()
	content, ok := f.stepOutputs[path]
	f.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	c.String(http.StatusOK, content)
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
// ?namespace=  → list resource classes; ?resource-class= → list instances.
func (f *CircleCI) handleRunnerList(c *gin.Context) {
	if rc := c.Query("resource-class"); rc != "" {
		f.handleListRunnerInstances(c)
		return
	}
	if ns := c.Query("namespace"); ns != "" {
		f.handleListResourceClasses(c)
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"message": "must specify exactly one of resource-class or namespace"})
}

func (f *CircleCI) handleListResourceClasses(c *gin.Context) {
	ns := c.Query("namespace")
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
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (f *CircleCI) handleCreateResourceClass(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid body"})
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
	c.JSON(http.StatusCreated, rc)
}

func (f *CircleCI) handleDeleteResourceClass(c *gin.Context) {
	slug := c.Param("namespace") + "/" + c.Param("name")
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
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted."})
}

func (f *CircleCI) handleListRunnerTokens(c *gin.Context) {
	rc := c.Query("resource-class")
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
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (f *CircleCI) handleCreateRunnerToken(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid body"})
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
	c.JSON(http.StatusCreated, tok)
}

func (f *CircleCI) handleDeleteRunnerToken(c *gin.Context) {
	id := c.Param("id")
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
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted."})
}

func (f *CircleCI) handleListRunnerInstances(c *gin.Context) {
	rc := c.Query("resource-class")
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
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// --- Project / env-var helpers ---

// AddFollowedProject registers a project returned by GET /api/v1.1/projects.
// proj should be a map or struct with at least "slug", "username", and "reponame" fields.
func (f *CircleCI) AddFollowedProject(proj any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.followedProjects = append(f.followedProjects, proj)
}

// AddEnvVar registers an env var for a project.
// slug should be in "vcs/org/repo" form.
func (f *CircleCI) AddEnvVar(slug, name, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envVars[slug] = append(f.envVars[slug], map[string]any{"name": name, "value": value})
}

// --- Project / env-var handlers ---

func (f *CircleCI) handleListProjects(c *gin.Context) {
	f.mu.RLock()
	projects := f.followedProjects
	f.mu.RUnlock()

	if projects == nil {
		projects = []any{}
	}
	c.JSON(http.StatusOK, projects)
}

func (f *CircleCI) handleFollowProject(c *gin.Context) {
	vcs := c.Param("vcs")
	org := c.Param("org")
	repo := c.Param("repo")
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

	c.JSON(http.StatusOK, gin.H{"following": true})
}

func (f *CircleCI) handleListEnvVars(c *gin.Context) {
	slug := c.Param("vcs") + "/" + c.Param("org") + "/" + c.Param("repo")
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
	c.JSON(http.StatusOK, gin.H{"items": items, "next_page_token": nil})
}

func (f *CircleCI) handleSetEnvVar(c *gin.Context) {
	slug := c.Param("vcs") + "/" + c.Param("org") + "/" + c.Param("repo")
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid body"})
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

	c.JSON(http.StatusCreated, ev)
}

func (f *CircleCI) handleDeleteEnvVar(c *gin.Context) {
	slug := c.Param("vcs") + "/" + c.Param("org") + "/" + c.Param("repo")
	name := c.Param("name")
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
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted."})
}
