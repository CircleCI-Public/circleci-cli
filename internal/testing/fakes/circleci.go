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
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder/chirecorder"
)

// CircleCI is a fake CircleCI API server.
type CircleCI struct {
	*httprecorder.RequestRecorder

	server *httptest.Server

	// ExtraHeaders are added to every response via middleware. Use to inject
	// Deprecation/Sunset headers without changing individual handler logic.
	ExtraHeaders http.Header

	mu                                sync.RWMutex
	pipelines                         map[string]any
	projects                          map[string][]any  // project slug → ordered list of pipelines
	workflowJobs                      map[string][]any  // workflow id → jobs
	jobArtifacts                      map[string][]any  // "slug/jobNumber" → artifacts
	jobArtifactsV3                    map[string][]any  // job UUID → V3 artifact data items
	staticFiles                       map[string]string // path → body content, for artifact downloads
	jobs                              map[string]any    // "slug/jobNumber" → job detail response (v2)
	jobsV1                            map[string]any    // "vcs/org/repo/jobNumber" → job detail response (v1.1)
	rawStepOutputs                    map[string]string // "slug/number/taskIndex/stepID" → plain text output
	rawStepErrors                     map[string]string // "slug/number/taskIndex/stepID" → plain text error
	triggerResponses                  map[string]any    // project slug → trigger response body
	triggerPipelineRunResponses       map[string]any    // project slug → trigger run response body
	triggerPipelineRunStatuses        map[string]int    // project slug → HTTP status (default 201)
	pipelineDefinitions               map[string][]any  // projectID → list of pipeline definition objects
	createPipelineDefinitionResponses map[string]any    // projectID → response body
	createTriggerResponses            map[string]any    // "projectID/pipelineDefinitionID" → response body
	listTriggerResponses              map[string][]any  // "projectID/pipelineDefinitionID" → list of triggers

	// GitHub App state.
	githubAppInstalled      map[string]bool   // orgID → app installed (200) vs not (404)
	githubAppRepos          map[string][]any  // orgID → repositories the app can access
	githubAppInstallResp    any               // response body for POST /github-app/install
	rerunResponses          map[string]int    // workflow id → HTTP status to return
	rerunNewIDs             map[string]string // workflow id → id of the workflow its rerun creates
	rerunFromFailed         map[string]bool   // workflow id → is_from_failed as the request actually set it
	cancelResponses         map[string]int    // workflow id → HTTP status to return
	pipelineCancelResponses map[string]int    // pipeline id → HTTP status to return

	// Job (v3) state.
	jobsV3             map[string]any    // job UUID → V3 response body
	workflowJobsV3     map[string][]any  // workflow id → V3 job list items
	jobStdout          map[string][]byte // "jobID/index/stepNum" → plain text stdout
	jobStderr          map[string][]byte // "jobID/index/stepNum" → plain text stderr
	jobStdoutCondensed map[string][]byte // "jobID/index/stepNum" → raw condensed text
	jobTests           map[string][]any  // job UUID → test result objects (served as JSONL)

	// Run (v3) state.
	runsV3          map[string]any   // run UUID → V3 response data (inner, not wrapped)
	runsV3ByProject map[string][]any // project UUID → ordered V3 run data items
	userRunsV3      []any            // ordered V3 run data items for GET /runs?filter[user_id]=me

	// Workflow (v3) state.
	workflowsV3         map[string]any   // workflow UUID → V3 workflow data (inner, not wrapped)
	workflowsV3ByRun    map[string][]any // run UUID → V3 workflow data items
	workflowsV3NotFound map[string]bool  // run UUID → workflows list returns 404

	// Runner (v3) state.
	resourceClasses []any            // all resource classes
	runnerTokens    map[string][]any // resource class → tokens
	runnerInstances []any            // all instances

	runnerTokenCreateStatus int // 0 = default success response
	runnerTokenCreateBody   any
	deletedTokens           map[string]bool // token id → deleted
	deletedRCs              map[string]bool // resource class → deleted

	// Project / env-var state.
	followedProjects    []any            // list of project objects for GET /api/v1.1/projects
	followedSlugs       map[string]bool  // vcs+org+repo → true (for follow idempotency)
	envVars             map[string][]any // project slug → env vars
	deletedEnvVars      map[string]bool  // "slug/name" → deleted
	projectInfos        map[string]any   // project slug → project info response
	projectsByID        map[string]any   // project UUID → V3 project response (GET /api/v3/projects/{id})
	projectsBySlug      map[string]any   // project slug → V3 project entity (GET /api/v3/projects?filter[slug]=)
	projectSettings     map[string]any   // project UUID → advanced settings attributes
	createProjectResp   any              // preset response for POST /organization/{vcs}/{org}/project
	createProjectStatus int              // HTTP status for that POST (0 → 201 Created)
	createOrgResp       any              // preset response for POST /organization

	// Context state.
	contexts                   map[string]any   // context id → context object
	contextsByOrg              map[string][]any // org slug → ordered context objects
	contextEnvVars             map[string][]any // context id → env var objects
	contextRestrictions        map[string][]any // context id → restriction objects
	deletedContexts            map[string]bool  // context id → deleted
	deletedContextVars         map[string]bool  // "contextID/name" → deleted
	deletedContextRestrictions map[string]bool  // "contextID/restrictionID" → deleted

	// Deploy state. Each slice holds every stored entity; the list handlers
	// filter it by the org/project/component the request names.
	deployments    []Deployment              // filtered by project_id
	environments   []DeployEnvironment       // filtered by org_id
	components     []DeployComponent         // filtered by org_id and (optionally) project_id
	compVersions   []DeployComponentVersion  // filtered by component id and (optionally) environment_id
	deploySettings map[string]DeploySettings // project id → settings entity

	// Policy state.
	policyBundles   map[string]map[string]string // "ownerID/ctx" → bundle
	decisionLogs    map[string][]any             // "ownerID/ctx" → logs
	decisionResults map[string]any               // "ownerID/ctx" → decision response
	policySettings  map[string]bool              // "ownerID/ctx" → enabled

	// iOS code signing state.
	iosCerts          map[string][]any // org id → certificate objects
	iosBundles        map[string][]any // org id → signing bundle objects
	deletedIOSCerts   map[string]bool  // cert id → deleted
	deletedIOSBundles map[string]bool  // bundle id → deleted
	iosCertCounter    int              // monotonic ID generator for uploaded certs
	iosBundleCounter  int              // monotonic ID generator for created bundles

	// Auth state.
	tokens             map[string]bool // accepted bearer tokens; a request whose Authorization: Bearer <token> is absent from this set is rejected 401 on every non-exempt route
	me                 any             // response for GET /api/v3/users?filter[user_id]=me
	collaborations     []any           // response for GET /api/v2/me/collaborations
	oauthTokenResponse any             // response body for POST /oauth/token
	oauthTokenStatus   int             // HTTP status for POST /oauth/token (0 → 200 OK)
	parRequests        []url.Values    // recorded POST /oauth/par request bodies, in order
	parCounter         int             // monotonic ID generator for request_uri values

	// Orb state (v3).
	orbPackages        map[string]map[string]any // id → package object
	orbPackagesByName  map[string]string         // "ns/name" → id
	orbVersions        map[string]map[string]any // id → version object
	orbVersionsByRef   map[string]string         // "ns/name@version" → id
	orbVersionsByOrbID map[string][]string       // orbID → ordered version IDs
	orbCategories      map[string]map[string]any // id → category object
	// orbAddCategoryStatus, when non-zero, is the HTTP status returned for every
	// POST /api/v3/orb/packages/{id}/add-category, so a test can exercise how a
	// caller copes with the registry refusing a category.
	orbAddCategoryStatus int
	orbCategoriesByName  map[string]string        // name → id
	orbValidateResponse  *orbFakeValidateResponse // override for validate/process responses
	orbCreatedPackages   []map[string]any         // packages created via POST
	orbCreatedVersions   []map[string]any         // versions created via POST
	orbUnlistedPackages  map[string]bool          // id → unlisted
	orbCategoryMembers   map[string][]string      // packageID → []categoryID

	// Namespace state (served via /graphql-unstable).
	namespaces        map[string]any    // namespace id → {id, name}
	namespacesByName  map[string]string // namespace name → id
	deletedNamespaces map[string]bool   // namespace id → deleted

	// DLC state.
	dlcPurgeStatus map[string]int // projectID → HTTP status to return (default 204)
	// Config compile state.
	compileValid       bool
	compileOutputYAML  string
	compileErrors      []string
	lastCompileOwnerID string

	// Org state.
	orgs        map[string]map[string]any
	orgsByUUID  map[string]bool // org UUID → true
	orgSettings map[string]any  // org UUID → attributes map

	// Release state (GET /api/v3/tool/releases).
	releaseTool        string    // tool name the fake answers for (default "circleci-cli")
	releaseVersion     string    // version returned in the 200 response
	releasePublishedAt time.Time // published_at returned in the 200 response
	releaseStatus      int       // 0 → 200; otherwise the status to return
}

// orbFakeValidateResponse holds a preset validate/process response for testing.
type orbFakeValidateResponse struct {
	yaml       string
	valid      bool
	errors     []string
	outputYAML string
}

// DefaultToken is the bearer token the fake accepts when NewCircleCI is called
// without an explicit token list. It matches the token acceptance tests inject
// via env.Token, so the common case needs no wiring.
const DefaultToken = "test-token"

// NewCircleCI starts a fake CircleCI API server and registers t.Cleanup to close it.
//
// The fake enforces authentication: every request to a non-exempt route must
// carry Authorization: Bearer <token> with a token in the accepted set, or it is
// rejected with 401. The accepted set defaults to DefaultToken; pass one or more
// tokens to override it, or adjust it later with RequireTokens/AllowToken. The
// login/pre-auth routes (/oauth/*, /artifacts/*, config compile, tool releases)
// are exempt — see authExempt.
func NewCircleCI(t *testing.T, tokens ...string) *CircleCI {
	t.Helper()
	if len(tokens) == 0 {
		tokens = []string{DefaultToken}
	}
	tokenSet := make(map[string]bool, len(tokens))
	for _, tok := range tokens {
		tokenSet[tok] = true
	}
	f := &CircleCI{
		RequestRecorder: httprecorder.New(),

		tokens: tokenSet,

		pipelines:                         map[string]any{},
		projects:                          map[string][]any{},
		workflowJobs:                      map[string][]any{},
		jobArtifacts:                      map[string][]any{},
		jobArtifactsV3:                    map[string][]any{},
		staticFiles:                       map[string]string{},
		jobs:                              map[string]any{},
		jobsV1:                            map[string]any{},
		rawStepOutputs:                    map[string]string{},
		rawStepErrors:                     map[string]string{},
		triggerResponses:                  map[string]any{},
		triggerPipelineRunResponses:       map[string]any{},
		triggerPipelineRunStatuses:        map[string]int{},
		pipelineDefinitions:               map[string][]any{},
		createPipelineDefinitionResponses: map[string]any{},
		createTriggerResponses:            map[string]any{},
		listTriggerResponses:              map[string][]any{},
		githubAppInstalled:                map[string]bool{},
		githubAppRepos:                    map[string][]any{},
		rerunResponses:                    map[string]int{},
		rerunNewIDs:                       map[string]string{},
		rerunFromFailed:                   map[string]bool{},
		cancelResponses:                   map[string]int{},
		pipelineCancelResponses:           map[string]int{},
		jobsV3:                            map[string]any{},
		workflowJobsV3:                    map[string][]any{},
		jobStdout:                         map[string][]byte{},
		jobStderr:                         map[string][]byte{},
		jobStdoutCondensed:                map[string][]byte{},
		jobTests:                          map[string][]any{},
		runsV3:                            map[string]any{},
		runsV3ByProject:                   map[string][]any{},
		workflowsV3:                       map[string]any{},
		workflowsV3ByRun:                  map[string][]any{},
		workflowsV3NotFound:               map[string]bool{},
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
		projectsByID:                      map[string]any{},
		projectsBySlug:                    map[string]any{},
		projectSettings:                   map[string]any{},
		deploySettings:                    map[string]DeploySettings{},
		policyBundles:                     make(map[string]map[string]string),
		decisionLogs:                      make(map[string][]any),
		decisionResults:                   make(map[string]any),
		policySettings:                    make(map[string]bool),
		namespaces:                        map[string]any{},
		namespacesByName:                  map[string]string{},
		deletedNamespaces:                 map[string]bool{},
		iosCerts:                          map[string][]any{},
		iosBundles:                        map[string][]any{},
		deletedIOSCerts:                   map[string]bool{},
		deletedIOSBundles:                 map[string]bool{},
		orbPackages:                       map[string]map[string]any{},
		orbPackagesByName:                 map[string]string{},
		orbVersions:                       map[string]map[string]any{},
		orbVersionsByRef:                  map[string]string{},
		orbVersionsByOrbID:                map[string][]string{},
		orbCategories:                     map[string]map[string]any{},
		orbCategoriesByName:               map[string]string{},
		orbUnlistedPackages:               map[string]bool{},
		orbCategoryMembers:                map[string][]string{},
		dlcPurgeStatus:                    map[string]int{},
		compileValid:                      true,
		compileOutputYAML:                 "# compiled output\nversion: \"2.1\"\n",
		orgs:                              map[string]map[string]any{},
		orgsByUUID:                        map[string]bool{},
		orgSettings:                       map[string]any{},
	}

	r := newRouter()
	r.Use(chirecorder.Middleware(f.RequestRecorder))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			f.mu.RLock()
			for k, vals := range f.ExtraHeaders {
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
			f.mu.RUnlock()
			next.ServeHTTP(w, req)
		})
	})
	r.Use(f.authMiddleware)
	r.Get("/api/v2/pipeline/{id}", f.handleGetPipeline)
	r.Post("/api/v2/pipeline/{id}/cancel", f.handleCancelPipeline)
	r.Post("/api/v3/workflows/{id}/rerun", f.handleRerunWorkflow)
	r.Post("/api/v3/workflows/{id}/cancel", f.handleCancelWorkflow)
	r.Get("/api/v2/project/{vcs}/{org}/{repo}/pipeline", f.handleListProjectPipelines)
	r.Get("/api/v2/project/{vcs}/{org}/{repo}/pipeline/{number}", f.handleGetPipelineByNumber)
	r.Get("/api/v2/workflow/{id}/job", f.handleGetWorkflowJobs)
	r.Get("/api/v2/project/{vcs}/{org}/{repo}/{jobNumber}/artifacts", f.handleGetJobArtifacts)
	r.Get("/api/v2/project/{vcs}/{org}/{repo}/job/{jobNumber}", f.handleGetJob)
	r.Post("/api/v2/project/{vcs}/{org}/{repo}/pipeline", f.handleTriggerPipeline)
	r.Post("/api/v2/project/{vcs}/{org}/{repo}/pipeline/run", f.handleTriggerPipelineRun)
	r.Get("/api/v1.1/project/{vcs}/{org}/{repo}/{jobNumber}", f.handleGetJobV1)
	// Project / env-var routes. These API calls do not URL-encode slashes in the
	// project slug, so we match three separate path segments rather than {slug}.
	r.Get("/api/v1.1/projects", f.handleListProjects)
	r.Post("/api/v1.1/project/{vcs}/{org}/{repo}/follow", f.handleFollowProject)
	r.Post("/api/v2/organization", f.handleCreateOrg)
	r.Post("/api/v2/organization/{vcs}/{org}/project", f.handleCreateProject)
	r.Get("/api/v3/users", f.handleGetMe)
	r.Get("/api/v2/me/collaborations", f.handleGetCollaborations)
	r.Post("/oauth/par", f.handleOAuthPAR)
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
	// GitHub App routes.
	r.Get("/api/v2/github-app/organization/{orgID}/installation", f.handleGetGitHubAppInstallation)
	r.Post("/api/v2/github-app/install", f.handleInstallGitHubApp)
	r.Get("/api/v2/github-app/organization/{orgID}/repositories", f.handleListGitHubAppRepositories)
	// Policy routes.
	r.Route("/api/v2/owner/{ownerID}/context/{policyCtx}", func(r chi.Router) {
		r.Post("/policy-bundle", f.handleCreatePolicyBundle)
		r.Get("/policy-bundle", f.handleFetchPolicyBundle)
		r.Get("/policy-bundle/{name}", f.handleFetchPolicyBundleByName)
		r.Get("/decision", f.handleGetDecisionLogs)
		r.Post("/decision", f.handleMakeDecision)
		r.Get("/decision/settings", f.handleGetPolicySettings)
		r.Patch("/decision/settings", f.handleSetPolicySettings)
		r.Get("/decision/{id}", f.handleGetDecisionLog)
	})
	// Deploy routes. Static sub-paths must be registered before the {id} catch-alls.
	r.Get("/api/v3/deploy/deployments", f.handleListDeployments)
	r.Get("/api/v3/deploy/environments", f.handleListEnvironments)
	r.Get("/api/v3/deploy/environments/{id}", f.handleGetEnvironment)
	r.Get("/api/v3/deploy/components", f.handleListComponents)
	r.Get("/api/v3/deploy/components/{id}/versions", f.handleListComponentVersions)
	r.Get("/api/v3/deploy/components/{id}", f.handleGetComponent)
	r.Get("/api/v3/deploy/settings", f.handleGetDeploySettings)
	// iOS code signing routes (V3).
	r.Post("/api/v3/signing/certificates", f.handleUploadIOSCert)
	r.Get("/api/v3/signing/certificates", f.handleListIOSCerts)
	r.Delete("/api/v3/signing/certificates/{id}", f.handleDeleteIOSCert)
	r.Post("/api/v3/signing/configs", f.handleCreateIOSBundle)
	r.Get("/api/v3/signing/configs", f.handleListIOSBundles)
	r.Delete("/api/v3/signing/configs/{id}", f.handleDeleteIOSBundle)
	// Config compile + org routes.
	r.Post("/api/v2/compile-config-with-defaults", f.handleCompileConfig)
	r.Get("/api/v3/tool/releases", f.handleGetReleases)
	r.Get("/api/v3/orgs", f.handleResolveOrg)
	r.Get("/api/v3/orgs/{id}/settings", f.handleGetOrgSettingsV3)
	r.Post("/api/v3/orgs/{id}/update-settings", f.handleUpdateOrgSettingsV3)
	// Job (v3) routes.
	r.Get("/api/v3/jobs", f.handleListWorkflowJobsV3)
	r.Get("/api/v3/jobs/{id}", f.handleGetJobV3)
	r.Get("/api/v3/jobs/{id}/artifacts", f.handleGetJobArtifactsV3)
	r.Get("/api/v3/jobs/{id}/stdout", f.handleGetJobStdout)
	r.Get("/api/v3/jobs/{id}/stdout/condensed", f.handleGetJobStdoutCondensed)
	r.Get("/api/v3/jobs/{id}/stderr", f.handleGetJobStderr)
	r.Get("/api/v3/jobs/{id}/tests", f.handleGetJobTests)
	// Workflow (v3) routes.
	r.Get("/api/v3/workflows/{id}", f.handleGetWorkflowV3ByID)
	r.Get("/api/v3/workflows", f.handleGetWorkflowsV3)
	// Project (v3) routes.
	r.Get("/api/v3/projects", f.handleResolveProjectBySlug)
	r.Get("/api/v3/projects/{id}", f.handleGetProjectV3)
	r.Get("/api/v3/projects/{id}/settings", f.handleGetProjectSettingsV3)
	r.Post("/api/v3/projects/{id}/update-settings", f.handleUpdateProjectSettingsV3)
	// Run (v3) routes.
	r.Get("/api/v3/runs", f.handleListMyRunsV3)
	r.Get("/api/v3/runs/{id}", f.handleGetRunV3)
	r.Post("/api/v3/runs/search", f.handleSearchRunsV3)
	// Runner (v3) routes. GET /runner lists instances (scoped by ?org-id= and/or
	// ?resource-class=); GET /runner/resource lists resource classes (scoped by
	// ?org-id= and/or ?namespace=). GET /runner also still accepts ?namespace=
	// for the legacy dispatch path.
	r.Get("/api/v3/runner", f.handleRunnerList)
	r.Get("/api/v3/runner/resource", f.handleListResourceClasses)
	r.Post("/api/v3/runner/resource", f.handleCreateResourceClass)
	r.Delete("/api/v3/runner/resource/{id}", f.handleDeleteResourceClass)
	r.Delete("/api/v3/runner/resource/{id}/force", f.handleForceDeleteResourceClass)
	r.Get("/api/v3/runner/token", f.handleListRunnerTokens)
	r.Post("/api/v3/runner/token", f.handleCreateRunnerToken)
	r.Delete("/api/v3/runner/token/{id}", f.handleDeleteRunnerToken)
	// Namespace (v3) routes.
	r.Get("/api/v3/namespaces", f.handleRESTGetNamespaceByName)
	r.Get("/api/v3/namespaces/{id}", f.handleRESTGetNamespaceByID)
	r.Post("/api/v3/namespaces", f.handleRESTCreateNamespace)
	r.Post("/api/v3/namespaces/{id}/rename", f.handleRESTRenameNamespace)
	r.Delete("/api/v3/namespaces/{id}", f.handleRESTDeleteNamespace)
	// Orb (v3) routes. Static paths must be registered before the {id} catch-all.
	r.Get("/api/v3/orb/packages", f.handleOrbListPackages)
	r.Post("/api/v3/orb/packages", f.handleOrbCreatePackage)
	r.Post("/api/v3/orb/packages/validate", f.handleOrbValidate)
	r.Post("/api/v3/orb/packages/process", f.handleOrbProcess)
	r.Get("/api/v3/orb/packages/{id}", f.handleOrbGetPackage)
	r.Post("/api/v3/orb/packages/{id}/set-listed", f.handleOrbSetListed)
	r.Post("/api/v3/orb/packages/{id}/add-category", f.handleOrbAddCategory)
	r.Post("/api/v3/orb/packages/{id}/remove-category", f.handleOrbRemoveCategory)
	r.Get("/api/v3/orb/versions", f.handleOrbListVersions)
	r.Post("/api/v3/orb/versions", f.handleOrbCreateVersion)
	r.Get("/api/v3/orb/versions/{id}", f.handleOrbGetVersion)
	r.Get("/api/v3/orb/versions/{id}/source", f.handleOrbGetVersionSource)
	r.Post("/api/v3/orb/versions/{id}/promote", f.handleOrbPromoteVersion)
	r.Get("/api/v3/orb/categories", f.handleOrbListCategories)
	r.Delete("/api/v3/projects/{projectID}/dlc", f.handleDLCPurge)
	// Wildcard route for artifact downloads — populated via AddStaticFile before requests.
	r.Get("/artifacts/*", f.handleStaticFile)
	// Raw step output/error routes for the private output API.
	r.Get("/api/private/output/raw/{vcs}/{org}/{repo}/{number}/output/{taskIndex}/{stepID}", f.handleRawStepOutput)
	r.Get("/api/private/output/raw/{vcs}/{org}/{repo}/{number}/error/{taskIndex}/{stepID}", f.handleRawStepError)
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

// RequireTokens replaces the accepted-token set, so a request must carry one of
// these as its Bearer token to reach any non-exempt route. Use it to pin the
// exact token a test expects, or to exercise the 401 path with a token the fake
// will reject.
func (f *CircleCI) RequireTokens(tokens ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens = make(map[string]bool, len(tokens))
	for _, tok := range tokens {
		f.tokens[tok] = true
	}
}

// AllowToken adds a token to the accepted-token set without disturbing the
// tokens already there. The OAuth token endpoint calls this for every token it
// mints, so a login flow's follow-up authenticated calls are accepted.
func (f *CircleCI) AllowToken(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[token] = true
}

// authMiddleware rejects any request to a non-exempt route that does not carry
// Authorization: Bearer <token> with an accepted token, mirroring how the real
// API refuses unauthenticated calls.
func (f *CircleCI) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authExempt(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
			f.mu.RLock()
			accepted := f.tokens[tok]
			f.mu.RUnlock()
			if accepted {
				next.ServeHTTP(w, r)
				return
			}
		}
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]any{"message": "Unauthorized"})
	})
}

// authExempt reports whether a path is reachable without authentication. These
// are the routes the CLI legitimately calls with no (or not-yet-issued) token:
// the OAuth login flow, signed artifact downloads, the optional-auth config
// compile endpoint, and the public tool-releases feed.
func authExempt(path string) bool {
	if strings.HasPrefix(path, "/oauth/") || strings.HasPrefix(path, "/artifacts/") {
		return true
	}
	switch path {
	case "/api/v2/compile-config-with-defaults", "/api/v3/tool/releases":
		return true
	}
	return false
}

// SetPipelineCancelResponse sets the HTTP status code returned for POST /api/v2/pipeline/<id>/cancel.
// Use http.StatusAccepted (202) for success.
func (f *CircleCI) SetPipelineCancelResponse(pipelineID string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pipelineCancelResponses[pipelineID] = status
}

// SetOrbAddCategoryStatus makes every POST
// /api/v3/orb/packages/<id>/add-category fail with the given status, carrying the
// message the real registry returns when an orb is already at its category limit.
func (f *CircleCI) SetOrbAddCategoryStatus(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orbAddCategoryStatus = status
}

// DefaultRerunWorkflowID is the id of the workflow a rerun creates, unless a test
// overrides it with SetRerunNewWorkflowID.
//
// It is deliberately not the id being rerun. A rerun creates a *new* workflow, and
// echoing the old id back would let a caller that mistakenly reports the id it was
// given pass its tests.
const DefaultRerunWorkflowID = "11111111-1111-4111-8111-111111111111"

// RerunWasFromFailed reports whether the rerun request for workflowID actually set
// is_from_failed, as decoded from the wire.
//
// Assert on this rather than on the raw request body: a body comparison passes as
// long as client and fake agree, which is how sending the v2 field name to a v3
// endpoint went unnoticed. This reports what the *endpoint* would have acted on, so
// a wrong field name reads as false.
func (f *CircleCI) RerunWasFromFailed(workflowID string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.rerunFromFailed[workflowID]
}

// SetRerunNewWorkflowID sets the id that rerunning workflowID reports, for a test
// that needs to distinguish several reruns.
func (f *CircleCI) SetRerunNewWorkflowID(workflowID, newID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rerunNewIDs[workflowID] = newID
}

// SetRerunResponse sets the HTTP status code returned for POST /api/v2/workflow/<id>/rerun.
// Use http.StatusAccepted (202) for success.
func (f *CircleCI) SetRerunResponse(workflowID string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rerunResponses[workflowID] = status
}

// SetCancelResponse sets the HTTP status code returned for POST /api/v3/workflows/<id>/cancel.
// Use http.StatusAccepted (202) for success.
func (f *CircleCI) SetCancelResponse(workflowID string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelResponses[workflowID] = status
}

// SetDLCPurgeStatus sets the HTTP status returned for DELETE /private/output/project/{id}/dlc.
// Default is 204 (success). Use 410 to simulate the gone/deprecated response.
func (f *CircleCI) SetDLCPurgeStatus(projectID string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dlcPurgeStatus[projectID] = status
}

func (f *CircleCI) handleDLCPurge(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	f.mu.RLock()
	status, ok := f.dlcPurgeStatus[projectID]
	f.mu.RUnlock()
	if !ok {
		status = http.StatusNoContent
	}
	w.WriteHeader(status)
}

// SetLatestRelease registers the release returned by GET /api/v3/tool/releases
// for the given tool. Clears any previously set non-200 status.
func (f *CircleCI) SetLatestRelease(tool, version string, publishedAt time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseTool = tool
	f.releaseVersion = version
	f.releasePublishedAt = publishedAt
	f.releaseStatus = http.StatusOK
}

// SetReleaseStatus makes GET /api/v3/tool/releases answer with the given HTTP
// status (e.g. 503 or 400) instead of a 200.
func (f *CircleCI) SetReleaseStatus(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseStatus = status
}

// handleGetReleases serves GET /api/v3/tool/releases, a required-filter single
// lookup modelled as a one-element collection. It honours filter[tool] and the
// status override set via SetReleaseStatus.
func (f *CircleCI) handleGetReleases(w http.ResponseWriter, r *http.Request) {
	tool := r.URL.Query().Get("filter[tool]")

	f.mu.RLock()
	status := f.releaseStatus
	wantTool := f.releaseTool
	version := f.releaseVersion
	publishedAt := f.releasePublishedAt
	f.mu.RUnlock()

	if wantTool == "" {
		wantTool = "circleci-cli"
	}

	writeReleaseError := func(code int, title, detail string) {
		render.Status(r, code)
		render.JSON(w, r, map[string]any{"error": map[string]any{"title": title, "detail": detail}})
	}

	if tool == "" {
		writeReleaseError(http.StatusBadRequest, "Missing Required Filter", "Query parameter 'filter[tool]' is required.")
		return
	}
	if status != 0 && status != http.StatusOK {
		writeReleaseError(status, http.StatusText(status), "release lookup failed")
		return
	}
	if tool != wantTool {
		writeReleaseError(http.StatusBadRequest, "Unknown Tool", "Unknown tool: "+tool)
		return
	}

	w.Header().Set("Cache-Control", "private, max-age=3600")
	render.JSON(w, r, map[string]any{
		"data": []any{map[string]any{
			"id": "b0f8c1e2-4d3a-5f6b-8c7d-9e0f1a2b3c4d",
			"attributes": map[string]any{
				"tool":         tool,
				"version":      version,
				"published_at": publishedAt.UTC().Format(time.RFC3339Nano),
			},
		}},
	})
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

// AddWorkflowJobs registers job responses for a workflow.
func (f *CircleCI) AddWorkflowJobs(workflowID string, jobs ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workflowJobs[workflowID] = jobs
}

// AddWorkflowJobsV3 registers V3 job list items for a workflow.
func (f *CircleCI) AddWorkflowJobsV3(workflowID string, jobs ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workflowJobsV3[workflowID] = jobs
}

// AddJobArtifacts registers artifact responses for a job.
// slug should be in "vcs/org/repo" form; jobNumber is the integer job number.
func (f *CircleCI) AddJobArtifacts(slug string, jobNumber int64, artifactItems ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fmt.Sprintf("%s/%d", slug, jobNumber)
	f.jobArtifacts[key] = artifactItems
}

// AddJobArtifactsV3 registers V3 artifact data items for a job UUID.
// Each item should be a V3 data entity with "attributes" containing path, url, execution.
func (f *CircleCI) AddJobArtifactsV3(jobID string, items ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobArtifactsV3[jobID] = items
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

// AddJobV3 registers a V3 job detail response for GET /api/v3/jobs/<id>.
func (f *CircleCI) AddJobV3(id string, job any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobsV3[id] = job
}

// AddJobStdout registers plain-text stdout for a step, served at
// GET /api/v3/jobs/<id>/stdout?filter[execution]=<execution>&filter[step_num]=<stepNum>.
func (f *CircleCI) AddJobStdout(id string, execution, stepNum int, content []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobStdout[fmt.Sprintf("%s/%d/%d", id, execution, stepNum)] = content
}

// AddJobStdoutCondensed registers the condensed stdout for a step, served at
// GET /api/v3/jobs/<id>/stdout/condensed?filter[execution]=<execution>&filter[step_num]=<stepNum>.
func (f *CircleCI) AddJobStdoutCondensed(id string, execution, stepNum int, content []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobStdoutCondensed[fmt.Sprintf("%s/%d/%d", id, execution, stepNum)] = content
}

// AddJobStderr registers plain-text stderr for a step, served at
// GET /api/v3/jobs/<id>/stderr?filter[execution]=<execution>&filter[step_num]=<stepNum>.
func (f *CircleCI) AddJobStderr(id string, execution, stepNum int, content []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobStderr[fmt.Sprintf("%s/%d/%d", id, execution, stepNum)] = content
}

// SetTriggerResponse registers the response body returned when POST
// /api/v2/project/<slug>/pipeline is called. slug should be in "vcs/org/repo" form.
func (f *CircleCI) SetTriggerResponse(slug string, resp any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.triggerResponses[slug] = resp
}

// SetTriggerPipelineRunResponse registers the response body returned when POST
// /api/v2/project/<slug>/pipeline/run is called with a 201 status.
// slug should be in "vcs/org/repo" form.
func (f *CircleCI) SetTriggerPipelineRunResponse(slug string, resp any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.triggerPipelineRunResponses[slug] = resp
	f.triggerPipelineRunStatuses[slug] = http.StatusCreated
}

// SetTriggerPipelineRunSkipped registers a "not triggered" response (200) for
// POST /api/v2/project/<slug>/pipeline/run.
func (f *CircleCI) SetTriggerPipelineRunSkipped(slug, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.triggerPipelineRunResponses[slug] = map[string]any{"message": message}
	f.triggerPipelineRunStatuses[slug] = http.StatusOK
}

// AddJob registers a job detail response for GET /api/v2/project/<slug>/job/<number>.
// slug should be in "vcs/org/repo" form; jobNumber is the integer job number.
func (f *CircleCI) AddJob(slug string, jobNumber int64, job any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fmt.Sprintf("%s/%d", slug, jobNumber)
	f.jobs[key] = job
}

// AddStepOutput registers plain-text output content for a step action, served
// at GET /api/private/output/raw/{slug}/{number}/output/{taskIndex}/{stepID}.
// taskIndex is action.Index and stepID is action.Step from the job response.
func (f *CircleCI) AddStepOutput(slug string, jobNumber int64, taskIndex, stepID int, content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fmt.Sprintf("%s/%d/%d/%d", slug, jobNumber, taskIndex, stepID)
	f.rawStepOutputs[key] = content
}

// AddStepError registers plain-text error content for a step action, served
// at GET /api/private/output/raw/{slug}/{number}/error/{taskIndex}/{stepID}.
func (f *CircleCI) AddStepError(slug string, jobNumber int64, taskIndex, stepID int, content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fmt.Sprintf("%s/%d/%d/%d", slug, jobNumber, taskIndex, stepID)
	f.rawStepErrors[key] = content
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

func (f *CircleCI) handleGetJobArtifactsV3(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	items := f.jobArtifactsV3[id]
	f.mu.RUnlock()

	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{"data": items})
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

func (f *CircleCI) handleTriggerPipelineRun(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	f.mu.RLock()
	resp, ok := f.triggerPipelineRunResponses[slug]
	status := f.triggerPipelineRunStatuses[slug]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "project not found"})
		return
	}
	render.Status(r, status)
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

func (f *CircleCI) handleGetJobV3(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	job, ok := f.jobsV3[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, job)
}

func (f *CircleCI) handleGetJobStdout(w http.ResponseWriter, r *http.Request) {
	key := jobStepKey(r)
	f.mu.RLock()
	content, ok := f.jobStdout[key]
	f.mu.RUnlock()
	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	// Honor a "Range: bytes=X-" resume offset and report the stream complete via
	// X-Terminal (the fake's stdout is always whole), so the pager fetches once
	// and stops polling.
	content = content[rangeOffset(r, len(content)):]
	w.Header().Set("X-Terminal", "true")
	render.Data(w, r, content)
}

// rangeOffset parses the resume offset from a "Range: bytes=X-" header, clamped
// to [0, n]. Missing or malformed ranges start at 0.
func rangeOffset(r *http.Request, n int) int {
	const prefix = "bytes="
	v := r.Header.Get("Range")
	i := strings.Index(v, prefix)
	if i < 0 {
		return 0
	}
	v = v[i+len(prefix):]
	if j := strings.IndexByte(v, '-'); j >= 0 {
		v = v[:j]
	}
	off, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || off < 0 {
		return 0
	}
	if off > n {
		off = n
	}
	return off
}

func (f *CircleCI) handleGetJobStdoutCondensed(w http.ResponseWriter, r *http.Request) {
	key := jobStepKey(r)
	f.mu.RLock()
	content, ok := f.jobStdoutCondensed[key]
	f.mu.RUnlock()
	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.Data(w, r, content)
}

func (f *CircleCI) handleGetJobStderr(w http.ResponseWriter, r *http.Request) {
	key := jobStepKey(r)
	f.mu.RLock()
	content, ok := f.jobStderr[key]
	f.mu.RUnlock()
	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.Data(w, r, content)
}

// AddJobTests registers test-result records for a job UUID, served as
// newline-delimited JSON (JSONL) at GET /api/v3/jobs/<id>/tests. Each record
// should be a map with classname, name, result, run_time and message fields.
func (f *CircleCI) AddJobTests(id string, tests ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobTests[id] = tests
}

// handleGetJobTests serves a job's test metadata as JSONL — one JSON object per
// line — mirroring the real endpoint. A job with no registered tests returns an
// empty 200 body.
func (f *CircleCI) handleGetJobTests(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	tests := f.jobTests[id]
	f.mu.RUnlock()

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	for _, tc := range tests {
		b, err := json.Marshal(tc)
		if err != nil {
			continue
		}
		_, _ = w.Write(b)
		_, _ = w.Write([]byte("\n"))
	}
}

// jobStepKey builds the "jobID/execution/stepNum" lookup key from the request,
// reading the execution and step_num from the filter[...] query params.
func jobStepKey(r *http.Request) string {
	execution := r.URL.Query().Get("filter[execution]")
	if execution == "" {
		execution = "0"
	}
	stepNum := r.URL.Query().Get("filter[step_num]")
	return fmt.Sprintf("%s/%s/%s", chi.URLParam(r, "id"), execution, stepNum)
}

func (f *CircleCI) handleListWorkflowJobsV3(w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Query().Get("filter[workflow_id]")
	if workflowID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"error": map[string]any{
			"type":   "validation_error",
			"title":  "Missing Required Filter",
			"detail": "Query parameter 'filter[workflow_id]' is required.",
		}})
		return
	}

	f.mu.RLock()
	jobs := f.workflowJobsV3[workflowID]
	f.mu.RUnlock()

	if jobs == nil {
		jobs = []any{}
	}
	render.JSON(w, r, map[string]any{"data": jobs})
}

// AddWorkflowV3 registers a single V3 workflow response for GET /api/v3/workflows/<id>.
func (f *CircleCI) AddWorkflowV3(id string, workflow any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workflowsV3[id] = workflow
}

// AddRunWorkflowsV3 registers V3 workflow responses for a run.
func (f *CircleCI) AddRunWorkflowsV3(runID string, workflows ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workflowsV3ByRun[runID] = workflows
}

// SetRunWorkflowsV3NotFound makes GET /api/v3/workflows?filter[run_id]=<runID>
// return 404 for the given run, mirroring the real API for runs whose
// workflows have not materialised.
func (f *CircleCI) SetRunWorkflowsV3NotFound(runID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workflowsV3NotFound[runID] = true
}

func (f *CircleCI) handleGetWorkflowV3ByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	wf, ok := f.workflowsV3[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, map[string]any{"data": wf})
}

func (f *CircleCI) handleGetWorkflowsV3(w http.ResponseWriter, r *http.Request) {
	runID := r.URL.Query().Get("filter[run_id]")
	f.mu.RLock()
	workflows := f.workflowsV3ByRun[runID]
	notFound := f.workflowsV3NotFound[runID]
	f.mu.RUnlock()

	if notFound {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{
			"error": map[string]any{"type": "not_found", "detail": "run not found", "id": "fake-error-id"},
		})
		return
	}
	if workflows == nil {
		workflows = []any{}
	}
	render.JSON(w, r, map[string]any{"data": workflows})
}

// AddRunV3 registers a V3 run response and associates it with a project.
// The run must have an "id" and "references.project.id" field.
func (f *CircleCI) AddRunV3(id, projectID string, run any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runsV3[id] = run
	f.runsV3ByProject[projectID] = append(f.runsV3ByProject[projectID], run)
}

// SetUserRuns registers the V3 run data items returned by
// GET /api/v3/runs?filter[user_id]=me (i.e. "circleci my runs").
func (f *CircleCI) SetUserRuns(runs ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.userRunsV3 = runs
}

func (f *CircleCI) handleListMyRunsV3(w http.ResponseWriter, r *http.Request) {
	if got := r.URL.Query().Get("filter[user_id]"); got != "me" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "expected filter[user_id]=me, got " + got})
		return
	}

	// The my-runs endpoint has no status filter; it filters on the run's own
	// phase and current_outcome (see apiclient.StatusPhaseOutcome).
	phase := r.URL.Query().Get("filter[phase]")
	currentOutcome := r.URL.Query().Get("filter[current_outcome]")

	f.mu.RLock()
	var results []any
	for _, run := range f.userRunsV3 {
		if phase != "" && runAttr(run, "phase") != phase {
			continue
		}
		if currentOutcome != "" && runAttr(run, "current_outcome") != currentOutcome {
			continue
		}
		results = append(results, run)
	}
	f.mu.RUnlock()

	offset, _ := strconv.Atoi(r.URL.Query().Get("page[cursor]"))
	if offset > len(results) {
		offset = len(results)
	}
	page := results[offset:]
	if size, err := strconv.Atoi(r.URL.Query().Get("page[limit]")); err == nil && size > 0 && len(page) > size {
		page = page[:size]
	}

	var next any
	if nextOff := offset + len(page); nextOff < len(results) {
		next = strconv.Itoa(nextOff)
	}

	render.JSON(w, r, map[string]any{
		"data": page,
		"page": map[string]any{"next": next, "prev": nil},
	})
}

func (f *CircleCI) handleGetRunV3(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	run, ok := f.runsV3[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, map[string]any{"data": run})
}

func (f *CircleCI) handleSearchRunsV3(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Scope struct {
			ProjectIDs []string `json:"project_ids"`
		} `json:"scope"`
		Filter string `json:"filter"`
		Page   struct {
			Cursor string `json:"cursor"`
			Limit  int    `json:"limit"`
		} `json:"page"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "bad request"})
		return
	}

	branch := runBranchFilter(body.Filter)
	status := runStatusFilterExpr(body.Filter)

	f.mu.RLock()
	var all []any
	for _, pid := range body.Scope.ProjectIDs {
		for _, run := range f.runsV3ByProject[pid] {
			if branch != "" && runBranch(run) != branch {
				continue
			}
			if status != "" && runStatus(run) != status {
				continue
			}
			all = append(all, run)
		}
	}
	f.mu.RUnlock()

	offset, _ := strconv.Atoi(body.Page.Cursor)
	if offset > len(all) {
		offset = len(all)
	}
	page := all[offset:]
	if body.Page.Limit > 0 && len(page) > body.Page.Limit {
		page = page[:body.Page.Limit]
	}

	var next any
	if nextOff := offset + len(page); nextOff < len(all) {
		s := strconv.Itoa(nextOff)
		next = s
	}

	render.JSON(w, r, map[string]any{
		"data": page,
		"page": map[string]any{"next": next, "prev": nil},
	})
}

// runBranchFilter extracts the branch pinned by a V3 search filter expression
// like `pipeline.git.branch == "main"`. It returns "" when no branch is pinned,
// meaning "match every branch".
func runBranchFilter(filter string) string {
	const key = `pipeline.git.branch == "`
	i := strings.Index(filter, key)
	if i < 0 {
		return ""
	}
	rest := filter[i+len(key):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// runAttr reads a top-level attributes field (e.g. "phase", "current_outcome")
// from a stored fake run as a string, or "" if absent.
func runAttr(run any, key string) string {
	m, ok := run.(map[string]any)
	if !ok {
		return ""
	}
	attrs, _ := m["attributes"].(map[string]any)
	s, _ := attrs[key].(string)
	return s
}

// runBranch reads references.event.attributes.vcs.branch from a stored fake
// run, or "" if absent.
func runBranch(run any) string {
	m, ok := run.(map[string]any)
	if !ok {
		return ""
	}
	refs, _ := m["references"].(map[string]any)
	event, _ := refs["event"].(map[string]any)
	attrs, _ := event["attributes"].(map[string]any)
	vcs, _ := attrs["vcs"].(map[string]any)
	branch, _ := vcs["branch"].(string)
	return branch
}

// runStatusFilterExpr extracts the pipeline status pinned by a V3 search filter
// expression like `pipeline.status == "failed"`. It returns "" when no status is
// pinned, meaning "match every status".
func runStatusFilterExpr(filter string) string {
	const key = `pipeline.status == "`
	i := strings.Index(filter, key)
	if i < 0 {
		return ""
	}
	rest := filter[i+len(key):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// runStatus derives a stored fake run's pipeline status token (as filtered on by
// the search endpoint's pipeline.status and the my-runs filter[status] param)
// from its phase and current_outcome. An ended run maps its outcome
// ("succeeded" → "success", others pass through); a non-ended run reports its
// phase (e.g. "running").
func runStatus(run any) string {
	m, ok := run.(map[string]any)
	if !ok {
		return ""
	}
	attrs, _ := m["attributes"].(map[string]any)
	phase, _ := attrs["phase"].(string)
	if phase != "ended" {
		return phase
	}
	outcome, _ := attrs["current_outcome"].(string)
	if outcome == "succeeded" {
		return "success"
	}
	return outcome
}

// handleRerunWorkflow mirrors POST /api/v3/workflows/:id/rerun as the real service
// implements it, which matters more than usual here: this fake used to reply with a
// "workflow_id" field the API does not have, and to ignore the request body
// entirely. Because the client made the matching mistakes, assertions round-tripped
// through agreeing errors and passed.
//
// So: decode *only* is_from_failed, tolerating unknown fields exactly as the real
// handler does. A caller sending the wrong field name gets false recorded here and
// fails a test, rather than being quietly accepted.
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

	// An absent body is a valid full rerun, so a decode failure is not an error.
	var req struct {
		IsFromFailed bool `json:"is_from_failed"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	f.mu.Lock()
	newID, override := f.rerunNewIDs[id]
	f.rerunFromFailed[id] = req.IsFromFailed
	f.mu.Unlock()
	if !override {
		newID = DefaultRerunWorkflowID
	}

	// The real endpoint identifies the new workflow at data.id, and sets Location.
	w.Header().Set("Location", "/api/v3/workflows/"+newID)
	render.Status(r, status)
	render.JSON(w, r, map[string]any{"data": map[string]any{"id": newID}})
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
	render.JSON(w, r, map[string]any{"data": map[string]any{"id": id}})
}

func (f *CircleCI) handleRawStepOutput(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	key := fmt.Sprintf("%s/%s/%s/%s", slug, chi.URLParam(r, "number"), chi.URLParam(r, "taskIndex"), chi.URLParam(r, "stepID"))
	f.mu.RLock()
	content := f.rawStepOutputs[key]
	f.mu.RUnlock()
	render.PlainText(w, r, content)
}

func (f *CircleCI) handleRawStepError(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	key := fmt.Sprintf("%s/%s/%s/%s", slug, chi.URLParam(r, "number"), chi.URLParam(r, "taskIndex"), chi.URLParam(r, "stepID"))
	f.mu.RLock()
	content := f.rawStepErrors[key]
	f.mu.RUnlock()
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

// handleRunnerList serves GET /api/v3/runner, returning runner instances scoped
// by ?resource-class=, ?org-id= or ?namespace=.
func (f *CircleCI) handleRunnerList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("resource-class") != "" || q.Get("org-id") != "" || q.Get("namespace") != "" {
		f.handleListRunnerInstances(w, r)
		return
	}
	render.Status(r, http.StatusBadRequest)
	render.JSON(w, r, map[string]any{"message": "must specify one of org-id, resource-class, or namespace"})
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

// handleDeleteResourceClass serves DELETE /api/v3/runner/resource/{id}, which
// refuses with 409 while the resource class still has tokens.
func (f *CircleCI) handleDeleteResourceClass(w http.ResponseWriter, r *http.Request) {
	f.deleteResourceClass(w, r, false)
}

// handleForceDeleteResourceClass serves DELETE /api/v3/runner/resource/{id}/force,
// which deletes the resource class and its tokens.
func (f *CircleCI) handleForceDeleteResourceClass(w http.ResponseWriter, r *http.Request) {
	f.deleteResourceClass(w, r, true)
}

func (f *CircleCI) deleteResourceClass(w http.ResponseWriter, r *http.Request, force bool) {
	id := chi.URLParam(r, "id")
	f.mu.Lock()
	found, hasTokens := false, false
	for _, rc := range f.resourceClasses {
		m, ok := rc.(map[string]any)
		if !ok {
			continue
		}
		if m["id"] == id {
			found = true
			slug, _ := m["resource_class"].(string)
			hasTokens = len(f.runnerTokens[slug]) > 0
			if force || !hasTokens {
				f.deletedRCs[slug] = true
				delete(f.runnerTokens, slug)
			}
			break
		}
	}
	f.mu.Unlock()

	switch {
	case !found:
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
	case !force && hasTokens:
		render.Status(r, http.StatusConflict)
		render.JSON(w, r, map[string]any{"message": "resource class still has tokens"})
	default:
		render.JSON(w, r, map[string]any{"message": "Deleted."})
	}
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

// SetRunnerTokenCreateResponse overrides the response to POST /runner/token, which
// otherwise answers 201 with a token whose value is "fake-runner-token-value". Use
// it to force a failure, or a 201 whose payload omits or lengthens the token value.
// A nil body sends a generic error message.
func (f *CircleCI) SetRunnerTokenCreateResponse(status int, body any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runnerTokenCreateStatus = status
	f.runnerTokenCreateBody = body
}

func (f *CircleCI) handleCreateRunnerToken(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}

	f.mu.RLock()
	status, override := f.runnerTokenCreateStatus, f.runnerTokenCreateBody
	f.mu.RUnlock()
	if status != 0 {
		render.Status(r, status)
		if override == nil {
			override = map[string]any{"message": "runner token creation failed"}
		}
		render.JSON(w, r, override)
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
	ns := r.URL.Query().Get("namespace")
	f.mu.RLock()
	all := f.runnerInstances
	f.mu.RUnlock()

	var items []any
	for _, inst := range all {
		if rc == "" && ns == "" {
			items = append(items, inst)
			continue
		}
		m, ok := inst.(map[string]any)
		if !ok {
			items = append(items, inst)
			continue
		}
		slug, _ := m["resource_class"].(string)
		if rc != "" && slug != rc {
			continue
		}
		if ns != "" && !strings.HasPrefix(slug, ns+"/") {
			continue
		}
		items = append(items, inst)
	}
	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{"items": items})
}

// --- Auth helpers ---

// SetMe sets the data element for GET /api/v3/users?filter[user_id]=me.
// Pass a DataEntity-shaped map: {"id": "...", "attributes": {"name": "...", "login": "...", "avatar_url": "..."}}.
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
	render.JSON(w, r, map[string]any{
		"data": []any{me},
		"page": map[string]any{"next": nil, "prev": nil},
	})
}

// SetCollaborations sets the response for GET /api/v2/me/collaborations.
func (f *CircleCI) SetCollaborations(collabs []any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.collaborations = collabs
}

func (f *CircleCI) handleGetCollaborations(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	collabs := f.collaborations
	f.mu.RUnlock()

	if collabs == nil {
		collabs = []any{}
	}
	render.JSON(w, r, collabs)
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

// LastPARRequest returns the form parameters of the most recent
// POST /oauth/par (RFC 9126), or nil if none has been received. Acceptance
// tests use it to recover the redirect_uri and state, which no longer travel
// in the browser-facing authorize URL.
func (f *CircleCI) LastPARRequest() url.Values {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if len(f.parRequests) == 0 {
		return nil
	}
	return f.parRequests[len(f.parRequests)-1]
}

// handleOAuthPAR implements the pushed-authorization-request endpoint
// (RFC 9126). It records the pushed parameters and returns a fresh
// request_uri with the mandatory 201 Created status.
func (f *CircleCI) handleOAuthPAR(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"error": "invalid_request"})
		return
	}

	f.mu.Lock()
	f.parRequests = append(f.parRequests, r.PostForm)
	f.parCounter++
	id := f.parCounter
	f.mu.Unlock()

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]any{
		"request_uri": fmt.Sprintf("urn:ietf:params:oauth:request_uri:fake-%d", id),
		"expires_in":  int64(90),
	})
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
	// A successful exchange issues a token the CLI then uses to validate itself
	// against /api/v3/users. Accept that minted token so the login flow's
	// follow-up authenticated call is not rejected by authMiddleware.
	if status == 0 || status < http.StatusBadRequest {
		if m, ok := resp.(map[string]any); ok {
			if at, ok := m["access_token"].(string); ok && at != "" {
				f.AllowToken(at)
			}
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

// defaultProjectSettingsAttrs returns an all-false v3 attributes payload.
func defaultProjectSettingsAttrs() map[string]any {
	return map[string]any{
		"enable_ai_error_summarization":          false,
		"enable_auto_cancel_redundant_workflows": false,
		"enable_building_fork_prs":               false,
		"is_build_prs_only":                      false,
		"can_pass_secrets_to_fork_pr_jobs":       false,
		"can_set_github_status":                  false,
		"is_running_disabled":                    false,
		"is_ssh_disabled":                        false,
		"enable_dynamic_config":                  false,
		"is_admin_required_for_writing_settings": false,
		"is_oss":                                 false,
		"pr_only_branch_overrides":               []string{},
		"enable_unversioned_config":              false,
	}
}

// SetProjectSettings registers advanced settings for GET /api/v3/projects/:id/settings.
// projectID should be the project UUID string.
func (f *CircleCI) SetProjectSettings(projectID string, settings any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projectSettings[projectID] = settings
}

func (f *CircleCI) handleGetProjectSettingsV3(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	settings, hasSettings := f.projectSettings[id]
	_, hasProject := f.projectsBySlug[id]
	if !hasProject {
		// also check by UUID in projectsByID
		_, hasProject = f.projectsByID[id]
	}
	f.mu.RUnlock()

	if !hasSettings && !hasProject {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	if !hasSettings {
		settings = defaultProjectSettingsAttrs()
	}
	render.JSON(w, r, map[string]any{
		"data": map[string]any{"attributes": settings},
	})
}

func (f *CircleCI) handleUpdateProjectSettingsV3(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var patch map[string]any
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid JSON"})
		return
	}

	f.mu.Lock()
	existing, hasSettings := f.projectSettings[id]
	_, hasProject := f.projectsByID[id]
	if !hasProject {
		_, hasProject = f.projectsBySlug[id]
	}
	if !hasSettings && !hasProject {
		f.mu.Unlock()
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}

	attrs, _ := existing.(map[string]any)
	if attrs == nil {
		attrs = defaultProjectSettingsAttrs()
	}
	for k, v := range patch {
		attrs[k] = v
	}
	f.projectSettings[id] = attrs
	f.mu.Unlock()

	render.JSON(w, r, map[string]any{
		"data": map[string]any{"attributes": attrs},
	})
}

// AddProjectV3 registers a project returned by GET /api/v3/projects/<id>,
// keyed by the project's UUID. The response is wrapped as {"data": project}.
func (f *CircleCI) AddProjectV3(id string, project any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projectsByID[id] = project
}

func (f *CircleCI) handleGetProjectV3(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	project, ok := f.projectsByID[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, map[string]any{"data": project})
}

// AddProjectBySlug registers a project resolved by GET
// /api/v3/projects?filter[slug]=<slug>, returning its UUID, name, and org UUID.
func (f *CircleCI) AddProjectBySlug(slug, id, name, orgID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projectsBySlug[slug] = map[string]any{
		"id":         id,
		"attributes": map[string]any{"name": name},
		"references": map[string]any{"org": map[string]any{"id": orgID}},
	}
}

func (f *CircleCI) handleResolveProjectBySlug(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("filter[slug]")
	if slug == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"error": map[string]any{"title": "Bad Request", "detail": "filter[slug] is required"}})
		return
	}
	f.mu.RLock()
	project, ok := f.projectsBySlug[slug]
	f.mu.RUnlock()

	// The endpoint is a collection: an unmatched slug is an empty list, not a 404.
	data := []any{}
	if ok {
		data = append(data, project)
	}
	render.JSON(w, r, map[string]any{"data": data, "page": map[string]any{"next": nil, "prev": nil}})
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

// SetCreateOrgResponse registers the response body returned when
// POST /api/v2/organization is called. Pass nil to simulate a 422 error.
func (f *CircleCI) SetCreateOrgResponse(resp any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createOrgResp = resp
}

// SetCreateProjectResponse registers the response body returned when
// POST /api/v2/organization/{vcs}/{org}/project is called.
// Pass nil to simulate a 422 error.
func (f *CircleCI) SetCreateProjectResponse(resp any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createProjectResp = resp
}

// SetCreateProjectConflict makes POST /api/v2/organization/{vcs}/{org}/project
// answer 409, as the real API does when the organization already has a project
// with that name.
func (f *CircleCI) SetCreateProjectConflict() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createProjectStatus = http.StatusConflict
	f.createProjectResp = map[string]any{"message": "A project with this name already exists"}
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

func (f *CircleCI) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	resp := f.createOrgResp
	f.mu.RUnlock()

	if resp == nil {
		render.Status(r, http.StatusUnprocessableEntity)
		render.JSON(w, r, map[string]any{"message": "org creation not configured"})
		return
	}
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, resp)
}

func (f *CircleCI) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	resp := f.createProjectResp
	status := f.createProjectStatus
	f.mu.RUnlock()

	if status != 0 {
		render.Status(r, status)
		render.JSON(w, r, resp)
		return
	}
	if resp == nil {
		render.Status(r, http.StatusUnprocessableEntity)
		render.JSON(w, r, map[string]any{"message": "project creation not configured"})
		return
	}
	// Echo the requested name, as the real API does. Returning a canned name
	// regardless of the request would let a caller send the wrong one and still
	// look correct in the output a test asserts on.
	if body, ok := resp.(map[string]any); ok {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Name != "" {
			echoed := make(map[string]any, len(body))
			for k, v := range body {
				echoed[k] = v
			}
			echoed["name"] = req.Name
			resp = echoed
		}
	}
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, resp)
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

// SetGitHubAppInstalled controls whether GET
// /api/v2/github-app/organization/{orgID}/installation returns 200 (installed)
// or 404 (not installed) for the org.
func (f *CircleCI) SetGitHubAppInstalled(orgID string, installed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.githubAppInstalled[orgID] = installed
}

// AddGitHubAppRepository registers a repository returned by GET
// /api/v2/github-app/organization/{orgID}/repositories.
func (f *CircleCI) AddGitHubAppRepository(orgID string, repo any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.githubAppRepos[orgID] = append(f.githubAppRepos[orgID], repo)
}

// SetGitHubAppInstallResponse registers the body returned by POST
// /api/v2/github-app/install.
func (f *CircleCI) SetGitHubAppInstallResponse(resp any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.githubAppInstallResp = resp
}

func (f *CircleCI) handleGetGitHubAppInstallation(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	f.mu.RLock()
	installed := f.githubAppInstalled[orgID]
	f.mu.RUnlock()

	if !installed {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, map[string]any{
		"id":                   12345678,
		"target_type":          "Organization",
		"login":                "my-org",
		"repository_selection": "all",
	})
}

func (f *CircleCI) handleInstallGitHubApp(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	resp := f.githubAppInstallResp
	f.mu.RUnlock()

	if resp == nil {
		resp = map[string]any{"redirect_url": "https://github.com/apps/circleci/installations/new?state=test"}
	}
	render.JSON(w, r, resp)
}

func (f *CircleCI) handleListGitHubAppRepositories(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	f.mu.RLock()
	repos := f.githubAppRepos[orgID]
	f.mu.RUnlock()

	if repos == nil {
		repos = []any{}
	}
	render.JSON(w, r, map[string]any{"items": repos, "total_count": len(repos)})
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

// Deployment is a stored deploy served by GET /api/v3/deploy/deployments,
// filtered by ProjectID. Add deployments newest-first — the endpoint returns
// them in insertion order. Optional string fields (FailureReason, EndedAt,
// PipelineID, WorkflowID) are omitted from the wire entity when empty, matching
// the real API.
type Deployment struct {
	ID            string
	ProjectID     string // list filter key; not rendered
	ComponentID   string
	ComponentName string
	EnvironmentID string
	PipelineID    string
	WorkflowID    string
	Type          string
	Status        string
	FailureReason string
	Version       string // rendered as target_version.name
	IsRollback    bool
	CreatedAt     string
	EndedAt       string
}

// DeployEnvironment is a stored deploy environment served by the environment
// list/get endpoints. OrgID is the list filter key.
type DeployEnvironment struct {
	ID    string
	OrgID string
	Name  string
}

// DeployComponent is a stored deploy component served by the component list/get
// endpoints. OrgID is the list filter key; ProjectID narrows the list.
type DeployComponent struct {
	ID        string
	OrgID     string
	ProjectID string
	Name      string
	Type      string
}

// DeployComponentVersion is a stored version served by the component-versions
// endpoint. ComponentID is the list key; EnvironmentID optionally narrows it.
type DeployComponentVersion struct {
	ID            string
	ComponentID   string
	EnvironmentID string
	Name          string
	CreatedAt     string
}

// DeploySettings is a stored deploy settings entity for a project.
type DeploySettings struct {
	ID                         string
	ProjectID                  string
	AutoCancelRedundantDeploys bool
}

// AddDeployment registers a deployment, returned by GET /api/v3/deploy/deployments
// for requests whose filter[project_id] matches its ProjectID.
func (f *CircleCI) AddDeployment(deployment Deployment) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deployments = append(f.deployments, deployment)
}

func (f *CircleCI) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("filter[project_id]")
	f.mu.RLock()
	items := []any{}
	for _, d := range f.deployments {
		if projectID == "" || d.ProjectID == projectID {
			items = append(items, deploymentEntity(d))
		}
	}
	f.mu.RUnlock()

	render.JSON(w, r, map[string]any{"data": items, "page": map[string]any{}})
}

// deploymentEntity renders a stored Deployment as the V3 entity the deploy
// client decodes, omitting the optional fields that are empty.
func deploymentEntity(d Deployment) map[string]any {
	attrs := map[string]any{
		"type":           d.Type,
		"status":         d.Status,
		"target_version": map[string]any{"name": d.Version},
		"is_rollback":    d.IsRollback,
		"created_at":     d.CreatedAt,
	}
	if d.EndedAt != "" {
		attrs["ended_at"] = d.EndedAt
	}
	if d.FailureReason != "" {
		attrs["failure_reason"] = d.FailureReason
	}
	refs := map[string]any{
		"deploy_component": map[string]any{
			"id":         d.ComponentID,
			"attributes": map[string]any{"name": d.ComponentName},
		},
		"deploy_environment": map[string]any{"id": d.EnvironmentID},
	}
	if d.PipelineID != "" {
		refs["pipeline"] = map[string]any{"id": d.PipelineID}
	}
	if d.WorkflowID != "" {
		refs["workflow"] = map[string]any{"id": d.WorkflowID}
	}
	return map[string]any{"id": d.ID, "attributes": attrs, "references": refs}
}

// AddEnvironment registers a deploy environment, listed for requests whose
// filter[org_id] matches its OrgID and fetched by its ID.
func (f *CircleCI) AddEnvironment(environment DeployEnvironment) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.environments = append(f.environments, environment)
}

func (f *CircleCI) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("filter[org_id]")
	f.mu.RLock()
	items := []any{}
	for _, e := range f.environments {
		if orgID == "" || e.OrgID == orgID {
			items = append(items, environmentEntity(e))
		}
	}
	f.mu.RUnlock()

	render.JSON(w, r, map[string]any{"data": items, "page": map[string]any{}})
}

func (f *CircleCI) handleGetEnvironment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, e := range f.environments {
		if e.ID == id {
			render.JSON(w, r, map[string]any{"data": environmentEntity(e)})
			return
		}
	}
	render.Status(r, http.StatusNotFound)
	render.JSON(w, r, map[string]any{"message": "not found"})
}

// environmentEntity renders a stored DeployEnvironment as its V3 entity.
func environmentEntity(e DeployEnvironment) map[string]any {
	return map[string]any{
		"id":         e.ID,
		"attributes": map[string]any{"name": e.Name},
		"references": map[string]any{"org": map[string]any{"id": e.OrgID}},
	}
}

// AddComponent registers a deploy component, listed for requests whose
// filter[org_id] matches its OrgID (and filter[project_id] its ProjectID) and
// fetched by its ID.
func (f *CircleCI) AddComponent(component DeployComponent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.components = append(f.components, component)
}

func (f *CircleCI) handleListComponents(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("filter[org_id]")
	projectID := r.URL.Query().Get("filter[project_id]")
	f.mu.RLock()
	items := []any{}
	for _, c := range f.components {
		if orgID != "" && c.OrgID != orgID {
			continue
		}
		if projectID != "" && c.ProjectID != projectID {
			continue
		}
		items = append(items, componentEntity(c))
	}
	f.mu.RUnlock()

	render.JSON(w, r, map[string]any{"data": items, "page": map[string]any{}})
}

func (f *CircleCI) handleGetComponent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, c := range f.components {
		if c.ID == id {
			render.JSON(w, r, map[string]any{"data": componentEntity(c)})
			return
		}
	}
	render.Status(r, http.StatusNotFound)
	render.JSON(w, r, map[string]any{"message": "not found"})
}

// componentEntity renders a stored DeployComponent as its V3 entity.
func componentEntity(c DeployComponent) map[string]any {
	return map[string]any{
		"id":         c.ID,
		"attributes": map[string]any{"name": c.Name, "type": c.Type},
		"references": map[string]any{"project": map[string]any{"id": c.ProjectID}},
	}
}

// AddComponentVersion registers a version for a deploy component, listed under
// its ComponentID and (optionally) narrowed by filter[environment_id].
func (f *CircleCI) AddComponentVersion(version DeployComponentVersion) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.compVersions = append(f.compVersions, version)
}

func (f *CircleCI) handleListComponentVersions(w http.ResponseWriter, r *http.Request) {
	componentID := chi.URLParam(r, "id")
	envID := r.URL.Query().Get("filter[environment_id]")
	f.mu.RLock()
	items := []any{}
	for _, v := range f.compVersions {
		if v.ComponentID != componentID {
			continue
		}
		if envID != "" && v.EnvironmentID != envID {
			continue
		}
		items = append(items, componentVersionEntity(v))
	}
	f.mu.RUnlock()

	render.JSON(w, r, map[string]any{"data": items, "page": map[string]any{}})
}

// componentVersionEntity renders a stored DeployComponentVersion as its V3 entity.
func componentVersionEntity(v DeployComponentVersion) map[string]any {
	return map[string]any{
		"id":         v.ID,
		"attributes": map[string]any{"name": v.Name, "created_at": v.CreatedAt},
		"references": map[string]any{"component": map[string]any{"id": v.ComponentID}},
	}
}

// SetDeploySettings registers deploy settings for a project.
func (f *CircleCI) SetDeploySettings(settings DeploySettings) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deploySettings[settings.ProjectID] = settings
}

func (f *CircleCI) handleGetDeploySettings(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("filter[project_id]")
	f.mu.RLock()
	settings, ok := f.deploySettings[projectID]
	f.mu.RUnlock()

	if !ok {
		// No settings registered: an entity with empty attributes, which the CLI
		// treats as "not configured".
		render.JSON(w, r, map[string]any{"data": map[string]any{
			"id":         projectID,
			"attributes": map[string]any{},
			"references": map[string]any{"project": map[string]any{"id": projectID}},
		}})
		return
	}
	render.JSON(w, r, map[string]any{"data": map[string]any{
		"id":         settings.ID,
		"attributes": map[string]any{"auto_cancel_redundant_deploys": settings.AutoCancelRedundantDeploys},
		"references": map[string]any{"project": map[string]any{"id": settings.ProjectID}},
	}})
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
// GET /api/v3/signing/certificates?filter[org_id]=<orgID>. The cert is stored
// in its flat fixture shape and wrapped in the V3 entity envelope on read.
func (f *CircleCI) AddIOSCert(orgID string, cert any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.iosCerts[orgID] = append(f.iosCerts[orgID], cert)
}

// AddIOSBundle registers an iOS signing bundle for an org, returned by
// GET /api/v3/signing/configs?filter[org_id]=<orgID>.
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
	// V3 data envelope: file_name/cert_blob/cert_password in attributes, org in references.
	var body struct {
		Data struct {
			Attributes struct {
				FileName string `json:"file_name"`
				Blob     string `json:"cert_blob"`
				Password string `json:"cert_password"`
			} `json:"attributes"`
			References struct {
				Org struct {
					ID string `json:"id"`
				} `json:"org"`
			} `json:"references"`
		} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": err.Error()})
		return
	}
	attrs := body.Data.Attributes
	orgID := body.Data.References.Org.ID
	if orgID == "" || attrs.FileName == "" || attrs.Blob == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "missing required fields"})
		return
	}
	f.mu.Lock()
	f.iosCertCounter++
	certID := fmt.Sprintf("00000000-0000-0000-0000-%012d", f.iosCertCounter)
	f.iosCerts[orgID] = append(f.iosCerts[orgID], map[string]any{
		"id":        certID,
		"file_name": attrs.FileName,
		"cert_type": "distribution",
	})
	f.mu.Unlock()
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]any{"data": map[string]any{"id": certID}})
}

func (f *CircleCI) handleListIOSCerts(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("filter[org_id]")
	f.mu.RLock()
	all := f.iosCerts[orgID]
	deleted := make(map[string]bool, len(f.deletedIOSCerts))
	for k, v := range f.deletedIOSCerts {
		deleted[k] = v
	}
	f.mu.RUnlock()

	// Wrap each stored flat cert in the V3 entity envelope: {id, attributes:{...}}.
	items := make([]any, 0, len(all))
	for _, c := range all {
		m, ok := c.(map[string]any)
		if !ok {
			items = append(items, c)
			continue
		}
		if id, _ := m["id"].(string); id != "" && deleted[id] {
			continue
		}
		items = append(items, map[string]any{
			"id": m["id"],
			"attributes": map[string]any{
				"file_name": m["file_name"],
				"cert_type": m["cert_type"],
			},
		})
	}
	render.JSON(w, r, map[string]any{"data": items})
}

func (f *CircleCI) handleDeleteIOSCert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
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
	// V3 data envelope: name/provisioning_profiles in attributes, org and
	// signing_certificate in references.
	var body struct {
		Data struct {
			Attributes struct {
				Name                 string           `json:"name"`
				ProvisioningProfiles []map[string]any `json:"provisioning_profiles"`
			} `json:"attributes"`
			References struct {
				Org struct {
					ID string `json:"id"`
				} `json:"org"`
				Certificate struct {
					ID string `json:"id"`
				} `json:"signing_certificate"`
			} `json:"references"`
		} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": err.Error()})
		return
	}
	name := body.Data.Attributes.Name
	orgID := body.Data.References.Org.ID
	certID := body.Data.References.Certificate.ID
	if name == "" || orgID == "" || certID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "missing required fields"})
		return
	}
	f.mu.Lock()

	// Reject if no live cert with the given id exists in this org.
	var certRef map[string]any
	for _, c := range f.iosCerts[orgID] {
		m, ok := c.(map[string]any)
		if !ok || m["id"] != certID {
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
	for _, b := range f.iosBundles[orgID] {
		m, ok := b.(map[string]any)
		if !ok || m["name"] != name {
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
	id := fmt.Sprintf("10000000-0000-0000-0000-%012d", f.iosBundleCounter)

	// Provisioning-profile list response echoes only file_name, not the blob.
	profiles := make([]map[string]any, len(body.Data.Attributes.ProvisioningProfiles))
	for i, p := range body.Data.Attributes.ProvisioningProfiles {
		profiles[i] = map[string]any{"file_name": p["file_name"]}
	}

	stored := map[string]any{
		"id":                    id,
		"name":                  name,
		"certificate":           certRef,
		"provisioning_profiles": profiles,
		// Internal-only — used by handleDeleteIOSCert's in-use check; not
		// part of the real API response shape but harmless extras for the
		// CLI, which only decodes documented fields.
		"_cert_id": certID,
		"_org_id":  orgID,
	}
	f.iosBundles[orgID] = append(f.iosBundles[orgID], stored)
	f.mu.Unlock()
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]any{"data": map[string]any{"id": id}})
}

func (f *CircleCI) handleListIOSBundles(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("filter[org_id]")
	f.mu.RLock()
	all := f.iosBundles[orgID]
	deleted := make(map[string]bool, len(f.deletedIOSBundles))
	for k, v := range f.deletedIOSBundles {
		deleted[k] = v
	}
	f.mu.RUnlock()

	// Wrap each stored flat bundle in the V3 entity envelope: name and profiles
	// in attributes, the certificate carried as a reference with its attributes.
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
		items = append(items, map[string]any{
			"id": m["id"],
			"attributes": map[string]any{
				"name":                  m["name"],
				"provisioning_profiles": m["provisioning_profiles"],
			},
			"references": map[string]any{
				"signing_certificate": map[string]any{
					"attributes": m["certificate"],
				},
			},
		})
	}
	render.JSON(w, r, map[string]any{"data": items})
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

// --- Orb helpers ---

// AddOrbPackage registers an orb package in the fake server.
func (f *CircleCI) AddOrbPackage(id, nsID, nsName, orbName string, isPrivate, isListed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fullName := nsName + "/" + orbName
	pkg := map[string]any{
		"id": id,
		"attributes": map[string]any{
			"name":                       fullName,
			"is_private":                 isPrivate,
			"is_listed":                  isListed,
			"created_at":                 "2026-01-01T00:00:00.000Z",
			"last_30_days_build_count":   int64(0),
			"last_30_days_project_count": int64(0),
			"last_30_days_org_count":     int64(0),
		},
		"references": map[string]any{
			"namespace": map[string]any{
				"id":         nsID,
				"attributes": map[string]any{"name": nsName},
			},
		},
	}
	f.orbPackages[id] = pkg
	f.orbPackagesByName[fullName] = id
}

// AddOrbVersion registers an orb version in the fake server.
// createdAt can be empty (will default to a fixed timestamp).
func (f *CircleCI) AddOrbVersion(id, orbID, orbName, version, source, createdAt string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if createdAt == "" {
		createdAt = "2026-01-15T10:30:00.000Z"
	}
	ver := map[string]any{
		"id": id,
		"attributes": map[string]any{
			"version":    version,
			"source":     source,
			"created_at": createdAt,
		},
		"references": map[string]any{
			"orb_package": map[string]any{
				"id":         orbID,
				"attributes": map[string]any{"name": orbName},
			},
		},
	}
	f.orbVersions[id] = ver
	ref := orbName + "@" + version
	f.orbVersionsByRef[ref] = id
	// Also register @volatile pointing to this version (last registered wins).
	volatileRef := orbName + "@volatile"
	f.orbVersionsByRef[volatileRef] = id
	// Add to orb's version list (for list by orb_id)
	f.orbVersionsByOrbID[orbID] = append([]string{id}, f.orbVersionsByOrbID[orbID]...)

	// Update the package's orb/versions reference
	if pkg, ok := f.orbPackages[orbID]; ok {
		if refs, ok := pkg["references"].(map[string]any); ok {
			refs["orb_versions"] = []any{map[string]any{
				"id": id,
				"attributes": map[string]any{
					"version":    version,
					"created_at": createdAt,
				},
			}}
		}
	}
}

// AddOrbCategory registers an orb category in the fake server.
func (f *CircleCI) AddOrbCategory(id, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orbCategories[id] = map[string]any{
		"id":         id,
		"attributes": map[string]any{"name": name},
	}
	f.orbCategoriesByName[name] = id
}

// SetOrbValidationResponse configures the validate/process endpoints to return
// the given result when the request YAML matches yaml. Pass "" for yaml to
// match any request.
func (f *CircleCI) SetOrbValidationResponse(yaml string, valid bool, errors []string, outputYAML string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orbValidateResponse = &orbFakeValidateResponse{
		yaml:       yaml,
		valid:      valid,
		errors:     errors,
		outputYAML: outputYAML,
	}
}

// --- Orb handlers ---

func orbPackageResponse(pkg map[string]any) map[string]any {
	return map[string]any{"data": pkg}
}

func orbVersionResponse(ver map[string]any) map[string]any {
	// Return a shallow copy of ver with source stripped from attributes —
	// source is only served via the dedicated /source endpoint.
	attrs, _ := ver["attributes"].(map[string]any)
	filteredAttrs := make(map[string]any, len(attrs))
	for k, v := range attrs {
		if k != "source" {
			filteredAttrs[k] = v
		}
	}
	filtered := make(map[string]any, len(ver))
	for k, v := range ver {
		filtered[k] = v
	}
	filtered["attributes"] = filteredAttrs
	return map[string]any{"data": filtered}
}

func (f *CircleCI) handleOrbListPackages(w http.ResponseWriter, r *http.Request) {
	nsID := r.URL.Query().Get("namespace_id")
	nameFilter := r.URL.Query().Get("filter[name]")
	f.mu.RLock()
	pkgs := f.orbPackages
	unlisted := f.orbUnlistedPackages
	catMembers := f.orbCategoryMembers
	cats := f.orbCategories
	f.mu.RUnlock()

	var items []any
	for _, pkg := range pkgs {
		attrs, _ := pkg["attributes"].(map[string]any)
		name, _ := attrs["name"].(string)

		if nameFilter != "" && name != nameFilter {
			continue
		}
		refs, _ := pkg["references"].(map[string]any)
		ns, _ := refs["namespace"].(map[string]any)
		nsIDVal, _ := ns["id"].(string)
		if nsID != "" && nsIDVal != nsID {
			continue
		}
		id, _ := pkg["id"].(string)
		// Build categories list for this package.
		catIDs := catMembers[id]
		catList := make([]any, 0, len(catIDs))
		for _, cid := range catIDs {
			if c, ok := cats[cid]; ok {
				catList = append(catList, c)
			}
		}
		// Clone pkg with updated listed state and categories.
		pkgCopy := cloneMap(pkg)
		if attrsCopy, ok := pkgCopy["attributes"].(map[string]any); ok {
			attrsCopy["is_listed"] = !unlisted[id]
		}
		if refsCopy, ok := pkgCopy["references"].(map[string]any); ok {
			if len(catList) > 0 {
				refsCopy["orb_categories"] = catList
			}
		}
		items = append(items, pkgCopy)
	}
	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{
		"data": items,
		"page": map[string]any{"next": nil, "prev": nil},
	})
}

func (f *CircleCI) handleOrbCreatePackage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Data struct {
			Attributes struct {
				Name      string `json:"name"`
				IsPrivate bool   `json:"is_private"`
			} `json:"attributes"`
			References struct {
				Namespace struct {
					ID string `json:"id"`
				} `json:"namespace"`
			} `json:"references"`
		} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}

	nsID := body.Data.References.Namespace.ID
	f.mu.Lock()
	nsData, ok := f.namespaces[nsID]
	f.mu.Unlock()
	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "namespace not found"})
		return
	}
	nsName, _ := nsData.(map[string]any)["name"].(string)

	id := uuid.New().String()
	pkg := map[string]any{
		"id": id,
		"attributes": map[string]any{
			"name":                       body.Data.Attributes.Name,
			"is_private":                 body.Data.Attributes.IsPrivate,
			"is_listed":                  true,
			"created_at":                 "2026-01-01T00:00:00.000Z",
			"last_30_days_build_count":   int64(0),
			"last_30_days_project_count": int64(0),
			"last_30_days_org_count":     int64(0),
		},
		"references": map[string]any{
			"namespace": map[string]any{
				"id":         nsID,
				"attributes": map[string]any{"name": nsName},
			},
		},
	}
	f.mu.Lock()
	f.orbPackages[id] = pkg
	f.orbPackagesByName[body.Data.Attributes.Name] = id
	f.orbCreatedPackages = append(f.orbCreatedPackages, pkg)
	f.mu.Unlock()

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, orbPackageResponse(pkg))
}

func (f *CircleCI) handleOrbGetPackage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	pkg, ok := f.orbPackages[id]
	catIDs := f.orbCategoryMembers[id]
	cats := f.orbCategories
	unlisted := f.orbUnlistedPackages[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}

	catList := make([]any, 0, len(catIDs))
	for _, cid := range catIDs {
		if c, ok := cats[cid]; ok {
			catList = append(catList, c)
		}
	}
	pkgCopy := cloneMap(pkg)
	if attrsCopy, ok := pkgCopy["attributes"].(map[string]any); ok {
		attrsCopy["is_listed"] = !unlisted
	}
	if refsCopy, ok := pkgCopy["references"].(map[string]any); ok {
		if len(catList) > 0 {
			refsCopy["orb_categories"] = catList
		}
	}
	render.JSON(w, r, orbPackageResponse(pkgCopy))
}

func (f *CircleCI) handleOrbSetListed(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Listed bool `json:"is_listed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}

	f.mu.Lock()
	pkg, ok := f.orbPackages[id]
	if ok {
		if !body.Listed {
			f.orbUnlistedPackages[id] = true
		} else {
			delete(f.orbUnlistedPackages, id)
		}
	}
	f.mu.Unlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, orbPackageResponse(pkg))
}

func (f *CircleCI) handleOrbAddCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	f.mu.RLock()
	forced := f.orbAddCategoryStatus
	f.mu.RUnlock()
	if forced != 0 {
		render.Status(r, forced)
		render.JSON(w, r, map[string]any{
			"message": "Orbs may only belong to 2 category(s). This orb has already been " +
				"placed under the following category(s): Build,Deployment.",
		})
		return
	}
	var body struct {
		CategoryID string `json:"category_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}

	f.mu.Lock()
	pkg, ok := f.orbPackages[id]
	if ok {
		// Avoid duplicates
		found := false
		for _, cid := range f.orbCategoryMembers[id] {
			if cid == body.CategoryID {
				found = true
				break
			}
		}
		if !found {
			f.orbCategoryMembers[id] = append(f.orbCategoryMembers[id], body.CategoryID)
		}
	}
	f.mu.Unlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, orbPackageResponse(pkg))
}

func (f *CircleCI) handleOrbRemoveCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		CategoryID string `json:"category_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}

	f.mu.Lock()
	pkg, ok := f.orbPackages[id]
	if ok {
		var remaining []string
		for _, cid := range f.orbCategoryMembers[id] {
			if cid != body.CategoryID {
				remaining = append(remaining, cid)
			}
		}
		f.orbCategoryMembers[id] = remaining
	}
	f.mu.Unlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, orbPackageResponse(pkg))
}

func (f *CircleCI) handleOrbValidate(w http.ResponseWriter, r *http.Request) {
	f.handleOrbValidateOrProcess(w, r)
}

func (f *CircleCI) handleOrbProcess(w http.ResponseWriter, r *http.Request) {
	f.handleOrbValidateOrProcess(w, r)
}

func (f *CircleCI) handleOrbValidateOrProcess(w http.ResponseWriter, r *http.Request) {
	var body struct {
		YAML  string `json:"yaml"`
		OrgID string `json:"org_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}

	f.mu.RLock()
	override := f.orbValidateResponse
	f.mu.RUnlock()

	valid := true
	outputYAML := body.YAML
	var errors []string

	if override != nil && (override.yaml == "" || override.yaml == body.YAML) {
		valid = override.valid
		errors = override.errors
		outputYAML = override.outputYAML
	}

	response := map[string]any{
		"data": map[string]any{
			"id": "00000000-0000-0000-0000-000000000000",
			"attributes": map[string]any{
				"is_valid":    valid,
				"output_yaml": outputYAML,
				"errors":      errors,
			},
		},
	}
	render.JSON(w, r, response)
}

func (f *CircleCI) handleOrbListVersions(w http.ResponseWriter, r *http.Request) {
	// If filter[ref] is given, dispatch to ref-based lookup
	if refFilter := r.URL.Query().Get("filter[ref]"); refFilter != "" {
		f.handleOrbListVersionsByRefInternal(w, r, refFilter)
		return
	}

	orbID := r.URL.Query().Get("filter[orb_id]")
	channel := r.URL.Query().Get("filter[channel]")
	pageSizeStr := r.URL.Query().Get("page[limit]")

	f.mu.RLock()
	versionIDs := f.orbVersionsByOrbID[orbID]
	allVersions := f.orbVersions
	f.mu.RUnlock()

	var items []any
	pageSize := len(versionIDs)
	if pageSizeStr != "" {
		if n, err := fmt.Sscanf(pageSizeStr, "%d", &pageSize); n != 1 || err != nil {
			pageSize = len(versionIDs)
		}
	}

	count := 0
	for _, id := range versionIDs {
		if count >= pageSize {
			break
		}
		ver, ok := allVersions[id]
		if !ok {
			continue
		}
		if channel != "" {
			attrs, _ := ver["attributes"].(map[string]any)
			version, _ := attrs["version"].(string)
			isDev := len(version) > 4 && version[:4] == "dev:"
			if channel == "stable" && isDev {
				continue
			}
			if channel == "dev" && !isDev {
				continue
			}
		}
		items = append(items, ver)
		count++
	}
	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{
		"data": items,
		"page": map[string]any{"next": nil, "prev": nil},
	})
}

func (f *CircleCI) handleOrbCreateVersion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Data struct {
			Attributes struct {
				OrbID   string `json:"orb_id"`
				YAML    string `json:"yaml"`
				Version string `json:"version"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}

	orbID := body.Data.Attributes.OrbID
	f.mu.RLock()
	pkg, pkgOK := f.orbPackages[orbID]
	f.mu.RUnlock()

	if !pkgOK {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "orb not found"})
		return
	}

	attrs, _ := pkg["attributes"].(map[string]any)
	orbName, _ := attrs["name"].(string)
	version := body.Data.Attributes.Version

	id := uuid.New().String()
	ver := map[string]any{
		"id": id,
		"attributes": map[string]any{
			"version":    version,
			"source":     body.Data.Attributes.YAML,
			"created_at": "2026-01-15T10:30:00.000Z",
		},
		"references": map[string]any{
			"orb_package": map[string]any{
				"id":         orbID,
				"attributes": map[string]any{"name": orbName},
			},
		},
	}

	f.mu.Lock()
	f.orbVersions[id] = ver
	ref := orbName + "@" + version
	f.orbVersionsByRef[ref] = id
	f.orbVersionsByRef[orbName+"@volatile"] = id
	f.orbVersionsByOrbID[orbID] = append([]string{id}, f.orbVersionsByOrbID[orbID]...)
	if refs, ok := pkg["references"].(map[string]any); ok {
		refs["orb_versions"] = []any{map[string]any{
			"id": id,
			"attributes": map[string]any{
				"version":    version,
				"created_at": "2026-01-15T10:30:00.000Z",
			},
		}}
	}
	f.orbCreatedVersions = append(f.orbCreatedVersions, ver)
	f.mu.Unlock()

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, orbVersionResponse(ver))
}

func (f *CircleCI) handleOrbGetVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	ver, ok := f.orbVersions[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, orbVersionResponse(ver))
}

func (f *CircleCI) handleOrbGetVersionSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	ver, ok := f.orbVersions[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
		return
	}
	attrs, _ := ver["attributes"].(map[string]any)
	source, _ := attrs["source"].(string)
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(source))
}

func (f *CircleCI) handleOrbPromoteVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Segment string `json:"segment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid body"})
		return
	}

	f.mu.RLock()
	ver, ok := f.orbVersions[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}

	refs, _ := ver["references"].(map[string]any)
	orb, _ := refs["orb_package"].(map[string]any)
	orbID, _ := orb["id"].(string)
	orbName, _ := orb["attributes"].(map[string]any)["name"].(string)

	// Find the latest stable version to increment from
	f.mu.RLock()
	versionIDs := f.orbVersionsByOrbID[orbID]
	allVersions := f.orbVersions
	f.mu.RUnlock()

	latestStable := "0.0.0"
	for _, vid := range versionIDs {
		v, ok := allVersions[vid]
		if !ok {
			continue
		}
		attrs, _ := v["attributes"].(map[string]any)
		ver2, _ := attrs["version"].(string)
		if len(ver2) > 4 && ver2[:4] == "dev:" {
			continue
		}
		latestStable = ver2
		break
	}

	// Increment version
	newVersion := incrementFakeVersion(latestStable, body.Segment)

	attrs, _ := ver["attributes"].(map[string]any)
	newID := uuid.New().String()
	newVer := map[string]any{
		"id": newID,
		"attributes": map[string]any{
			"version":    newVersion,
			"source":     attrs["source"],
			"created_at": "2026-01-15T10:30:00.000Z",
		},
		"references": map[string]any{
			"orb_package": map[string]any{
				"id":         orbID,
				"attributes": map[string]any{"name": orbName},
			},
		},
	}

	f.mu.Lock()
	f.orbVersions[newID] = newVer
	f.orbVersionsByRef[orbName+"@"+newVersion] = newID
	f.orbVersionsByRef[orbName+"@volatile"] = newID
	f.orbVersionsByOrbID[orbID] = append([]string{newID}, f.orbVersionsByOrbID[orbID]...)
	f.mu.Unlock()

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, orbVersionResponse(newVer))
}

func (f *CircleCI) handleOrbListCategories(w http.ResponseWriter, r *http.Request) {
	nameFilter := r.URL.Query().Get("filter[name]")
	f.mu.RLock()
	cats := f.orbCategories
	byName := f.orbCategoriesByName
	f.mu.RUnlock()

	var items []any
	if nameFilter != "" {
		if id, ok := byName[nameFilter]; ok {
			if c, ok := cats[id]; ok {
				items = append(items, c)
			}
		}
	} else {
		for _, c := range cats {
			items = append(items, c)
		}
	}
	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{
		"data": items,
		"page": map[string]any{"next": nil, "prev": nil},
	})
}

// handleOrbListVersions handles GET /api/v3/orb/versions with filter[ref] support.
// The existing handleOrbListVersions is extended to handle filter[ref].
func (f *CircleCI) handleOrbListVersionsByRefInternal(w http.ResponseWriter, r *http.Request, refFilter string) {
	f.mu.RLock()
	verID, ok := f.orbVersionsByRef[refFilter]
	allVersions := f.orbVersions
	f.mu.RUnlock()

	if !ok {
		render.JSON(w, r, map[string]any{
			"data": []any{},
			"page": map[string]any{"next": nil, "prev": nil},
		})
		return
	}
	ver, ok := allVersions[verID]
	if !ok {
		render.JSON(w, r, map[string]any{
			"data": []any{},
			"page": map[string]any{"next": nil, "prev": nil},
		})
		return
	}
	render.JSON(w, r, map[string]any{
		"data": []any{ver},
		"page": map[string]any{"next": nil, "prev": nil},
	})
}

// cloneMap does a shallow clone of a map[string]any.
func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// incrementFakeVersion increments a semver string.
func incrementFakeVersion(version, segment string) string {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "0.0.1"
	}
	major := parseIntOrZero(parts[0])
	minor := parseIntOrZero(parts[1])
	patch := parseIntOrZero(parts[2])
	switch segment {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	default:
		patch++
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}

func parseIntOrZero(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

// --- Policy helpers ---

// AddPolicyBundle registers a policy bundle for the given owner and context.
func (f *CircleCI) AddPolicyBundle(ownerID, policyCtx string, bundle map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := ownerID + "/" + policyCtx
	f.policyBundles[key] = bundle
}

// AddDecisionLog appends a decision log entry for the given owner and context.
func (f *CircleCI) AddDecisionLog(ownerID, policyCtx string, log any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := ownerID + "/" + policyCtx
	f.decisionLogs[key] = append(f.decisionLogs[key], log)
}

// SetDecisionResult sets the response returned by MakeDecision for the given owner and context.
func (f *CircleCI) SetDecisionResult(ownerID, policyCtx string, result any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := ownerID + "/" + policyCtx
	f.decisionResults[key] = result
}

// SetPolicyEnabled sets the policy enforcement enabled flag for the given owner and context.
func (f *CircleCI) SetPolicyEnabled(ownerID, policyCtx string, enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := ownerID + "/" + policyCtx
	f.policySettings[key] = enabled
}

// --- Policy handlers ---

func (f *CircleCI) handleCreatePolicyBundle(w http.ResponseWriter, r *http.Request) {
	ownerID := chi.URLParam(r, "ownerID")
	policyCtx := chi.URLParam(r, "policyCtx")
	isDry := r.URL.Query().Get("dry") == "true"

	var body struct {
		Policies map[string]string `json:"policies"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	f.mu.Lock()
	key := ownerID + "/" + policyCtx
	if !isDry {
		f.policyBundles[key] = body.Policies
	}
	f.mu.Unlock()

	render.JSON(w, r, map[string]any{"created": []string{}, "deleted": []string{}, "updated": []string{}})
}

func (f *CircleCI) handleFetchPolicyBundle(w http.ResponseWriter, r *http.Request) {
	ownerID := chi.URLParam(r, "ownerID")
	policyCtx := chi.URLParam(r, "policyCtx")
	f.mu.RLock()
	bundle := f.policyBundles[ownerID+"/"+policyCtx]
	f.mu.RUnlock()
	if bundle == nil {
		bundle = map[string]string{}
	}
	render.JSON(w, r, bundle)
}

func (f *CircleCI) handleFetchPolicyBundleByName(w http.ResponseWriter, r *http.Request) {
	ownerID := chi.URLParam(r, "ownerID")
	policyCtx := chi.URLParam(r, "policyCtx")
	name := chi.URLParam(r, "name")
	f.mu.RLock()
	bundle := f.policyBundles[ownerID+"/"+policyCtx]
	f.mu.RUnlock()
	if bundle == nil {
		http.NotFound(w, r)
		return
	}
	content, ok := bundle[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	render.JSON(w, r, map[string]string{name: content})
}

func (f *CircleCI) handleGetDecisionLogs(w http.ResponseWriter, r *http.Request) {
	ownerID := chi.URLParam(r, "ownerID")
	policyCtx := chi.URLParam(r, "policyCtx")
	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		offset, _ = strconv.Atoi(offsetStr)
	}
	f.mu.RLock()
	all := f.decisionLogs[ownerID+"/"+policyCtx]
	f.mu.RUnlock()
	if offset >= len(all) {
		render.JSON(w, r, []any{})
		return
	}
	render.JSON(w, r, all[offset:])
}

func (f *CircleCI) handleGetDecisionLog(w http.ResponseWriter, r *http.Request) {
	ownerID := chi.URLParam(r, "ownerID")
	policyCtx := chi.URLParam(r, "policyCtx")
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	all := f.decisionLogs[ownerID+"/"+policyCtx]
	f.mu.RUnlock()
	for _, l := range all {
		if m, ok := l.(map[string]any); ok {
			if m["id"] == id {
				render.JSON(w, r, l)
				return
			}
		}
	}
	http.NotFound(w, r)
}

func (f *CircleCI) handleMakeDecision(w http.ResponseWriter, r *http.Request) {
	ownerID := chi.URLParam(r, "ownerID")
	policyCtx := chi.URLParam(r, "policyCtx")
	f.mu.RLock()
	result := f.decisionResults[ownerID+"/"+policyCtx]
	f.mu.RUnlock()
	if result == nil {
		result = map[string]any{"status": "PASS"}
	}
	render.JSON(w, r, result)
}

func (f *CircleCI) handleGetPolicySettings(w http.ResponseWriter, r *http.Request) {
	ownerID := chi.URLParam(r, "ownerID")
	policyCtx := chi.URLParam(r, "policyCtx")
	f.mu.RLock()
	enabled := f.policySettings[ownerID+"/"+policyCtx]
	f.mu.RUnlock()
	render.JSON(w, r, map[string]any{"enabled": enabled})
}

func (f *CircleCI) handleSetPolicySettings(w http.ResponseWriter, r *http.Request) {
	ownerID := chi.URLParam(r, "ownerID")
	policyCtx := chi.URLParam(r, "policyCtx")
	var body struct {
		Enabled bool `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.mu.Lock()
	f.policySettings[ownerID+"/"+policyCtx] = body.Enabled
	f.mu.Unlock()
	render.JSON(w, r, map[string]any{"enabled": body.Enabled})
}

// --- Config compile + org helpers ---

// SetCompileResponse configures what the fake returns for POST /compile-config-with-defaults.
// Pass valid=false and one or more error messages to simulate a compilation failure.
func (f *CircleCI) SetCompileResponse(valid bool, outputYAML string, errors ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.compileValid = valid
	f.compileOutputYAML = outputYAML
	f.compileErrors = errors
}

// LastCompileOwnerID returns the owner_id sent on the most recent
// POST /compile-config-with-defaults request (empty if none yet). Tests use it
// to assert that --org resolved to the expected organization UUID.
func (f *CircleCI) LastCompileOwnerID() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.lastCompileOwnerID
}

// AddOrg registers an org resolvable by slug via GET /api/v3/orgs.
func (f *CircleCI) AddOrg(id, slug, name, vcsType string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orgs[slug] = map[string]any{
		"id":       id,
		"name":     name,
		"slug":     slug,
		"vcs_type": vcsType,
	}
	f.orgsByUUID[id] = true
}

func (f *CircleCI) handleCompileConfig(w http.ResponseWriter, r *http.Request) {
	// Capture the resolved owner_id so tests can assert that --org (slug or UUID)
	// resolved to the expected organization UUID before the compile call.
	var body struct {
		Options struct {
			OwnerID string `json:"owner_id"`
		} `json:"options"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	f.mu.Lock()
	f.lastCompileOwnerID = body.Options.OwnerID
	valid := f.compileValid
	outputYAML := f.compileOutputYAML
	errs := f.compileErrors
	f.mu.Unlock()

	apiErrors := make([]map[string]any, len(errs))
	for i, e := range errs {
		apiErrors[i] = map[string]any{"message": e}
	}

	render.JSON(w, r, map[string]any{
		"valid":       valid,
		"source-yaml": outputYAML,
		"output-yaml": outputYAML,
		"errors":      apiErrors,
	})
}

// handleResolveOrg serves GET /api/v3/orgs?filter[slug]=<slug>, resolving a
// single org by its slug. An unknown slug returns 200 with an empty data
// array (not a 404), matching the real API.
func (f *CircleCI) handleResolveOrg(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("filter[slug]")
	f.mu.RLock()
	org, ok := f.orgs[slug]
	f.mu.RUnlock()

	data := []map[string]any{}
	if ok {
		data = append(data, map[string]any{"id": org["id"]})
	}
	render.JSON(w, r, map[string]any{
		"data": data,
		"page": map[string]any{"next": nil, "prev": nil},
	})
}

// defaultOrgSettingsAttrs returns an all-false v3 attributes payload for org settings.
func defaultOrgSettingsAttrs() map[string]any {
	return map[string]any{
		"is_runner_terms_of_service_accepted":      false,
		"enable_ai_error_summarization":            false,
		"enable_ai_agents":                         false,
		"enable_unversioned_config":                false,
		"enable_certified_public_orbs":             false,
		"enable_chunk_ip_ranges":                   false,
		"enable_minor_ai_features":                 false,
		"enable_private_orbs":                      false,
		"enable_uncertified_public_orbs":           false,
		"is_bitbucket_workspace_member_org_member": false,
		"is_user_checkout_keys_disabled":           false,
		"is_running_disabled":                      false,
		"enable_image_brownouts":                   false,
		"is_context_group_restriction_required":    false,
		"enable_resource_class_brownouts":          false,
	}
}

// SetOrgSettings registers advanced settings for GET /api/v3/orgs/:id/settings.
// orgUUID should be the org UUID string.
func (f *CircleCI) SetOrgSettings(orgUUID string, settings any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orgSettings[orgUUID] = settings
}

func (f *CircleCI) handleGetOrgSettingsV3(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	settings, hasSettings := f.orgSettings[id]
	hasOrg := f.orgsByUUID[id]
	f.mu.RUnlock()

	if !hasSettings && !hasOrg {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	if !hasSettings {
		settings = defaultOrgSettingsAttrs()
	}
	render.JSON(w, r, map[string]any{
		"data": map[string]any{"attributes": settings},
	})
}

func (f *CircleCI) handleUpdateOrgSettingsV3(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var patch map[string]any
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid JSON"})
		return
	}

	f.mu.Lock()
	existing, hasSettings := f.orgSettings[id]
	hasOrg := f.orgsByUUID[id]
	if !hasSettings && !hasOrg {
		f.mu.Unlock()
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}

	attrs, _ := existing.(map[string]any)
	if attrs == nil {
		attrs = defaultOrgSettingsAttrs()
	}
	for k, v := range patch {
		attrs[k] = v
	}
	f.orgSettings[id] = attrs
	f.mu.Unlock()

	render.JSON(w, r, map[string]any{
		"data": map[string]any{"attributes": attrs},
	})
}
