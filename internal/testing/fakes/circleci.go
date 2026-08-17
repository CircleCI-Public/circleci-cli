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
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
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
	pipelines                         map[string]PipelineV2
	projects                          map[string][]PipelineV2 // project slug → ordered pipelines
	jobArtifacts                      map[string][]any        // "slug/jobNumber" → artifacts
	jobArtifactsV3                    map[string][]Artifact   // job UUID → V3 artifacts
	staticFiles                       map[string]string       // path → body content, for artifact downloads
	triggerResponses                  map[string]any          // project slug → trigger response body
	triggerPipelineRunResponses       map[string]any          // project slug → trigger run response body
	triggerPipelineRunStatuses        map[string]int          // project slug → HTTP status (default 201)
	pipelineDefinitions               map[string][]any        // projectID → list of v3 pipeline entities
	createPipelineDefinitionResponses map[string]any          // projectID → v3 pipeline entity
	createTriggerResponses            map[string]any          // "projectID/pipelineID" → v3 trigger entity
	listTriggerResponses              map[string][]any        // "projectID/pipelineID" → list of v3 trigger entities

	// GitHub App state.
	providerConnections     map[string][]string        // orgID → connected providers
	githubAppRepos          map[string][]GitHubAppRepo // orgID → repositories the app can access
	githubAppInstallResp    any                        // response body for POST /github-app/install
	rerunResponses          map[string]int             // workflow id → HTTP status to return
	rerunNewIDs             map[string]string          // workflow id → id of the workflow its rerun creates
	rerunFromFailed         map[string]bool            // workflow id → is_from_failed as the request actually set it
	cancelResponses         map[string]int             // workflow id → HTTP status to return
	pipelineCancelResponses map[string]int             // pipeline id → HTTP status to return

	// Job (v3) state.
	jobsV3             map[string]JobV3         // job UUID → job detail entity
	workflowJobsV3     map[string][]JobV3       // workflow id → job list entities
	jobStdout          map[string][]byte        // "jobID/index/stepNum" → plain text stdout
	jobStderr          map[string][]byte        // "jobID/index/stepNum" → plain text stderr
	jobStdoutCondensed map[string][]byte        // "jobID/index/stepNum" → raw condensed text
	jobTests           map[string][]TestResult  // job UUID → test result objects (served as JSONL)
	jobResourceUsage   map[string]ResourceUsage // job UUID → sampled CPU/memory usage

	// Run (v3) state.
	runsV3          map[string]RunV3   // run UUID → stored run
	runsV3ByProject map[string][]RunV3 // project UUID → ordered runs (for search)
	userRunsV3      []RunV3            // ordered runs for GET /runs?filter[user_id]=me

	// Workflow (v3) state.
	workflowsV3         map[string]WorkflowV3   // workflow UUID → stored workflow
	workflowsV3ByRun    map[string][]WorkflowV3 // run UUID → ordered workflows
	workflowsV3NotFound map[string]bool         // run UUID → workflows list returns 404

	// Runner (v3) state.
	resourceClasses []ResourceClass          // all resource classes
	runnerTokens    map[string][]RunnerToken // resource class slug → tokens
	runnerInstances []RunnerInstance         // all instances

	runnerTokenCreateStatus int // 0 = default success response
	runnerTokenCreateBody   any
	deletedTokens           map[string]bool // token id → deleted
	deletedRCs              map[string]bool // resource class → deleted

	// Project / env-var state.
	followedProjects    []FollowedProject      // projects for GET /api/v1.1/projects
	followedSlugs       map[string]bool        // vcs+org+repo → true (for follow idempotency)
	envVars             map[string][]EnvVar    // project slug → env vars
	deletedEnvVars      map[string]bool        // "slug/name" → deleted
	projectInfos        map[string]ProjectInfo // project slug → project info response
	projectsByID        map[string]any         // project UUID → V3 project response (GET /api/v3/projects/{id})
	projectsBySlug      map[string]ProjectV3   // project slug → resolved project (GET /api/v3/projects?filter[slug]=)
	projectSettings     map[string]any         // project UUID → advanced settings attributes
	createProjectResp   any                    // preset response for POST /organization/{vcs}/{org}/project
	createProjectStatus int                    // HTTP status for that POST (0 → 201 Created)
	createOrgResp       any                    // preset response for POST /organization

	// Context state.
	contexts                   map[string]Context              // context id → context
	contextsByOrg              map[string][]Context            // org slug → ordered contexts
	contextEnvVars             map[string][]ContextEnvVar      // context id → env vars
	contextRestrictions        map[string][]ContextRestriction // context id → restrictions
	deletedContexts            map[string]bool                 // context id → deleted
	deletedContextVars         map[string]bool                 // "contextID/name" → deleted
	deletedContextRestrictions map[string]bool                 // "contextID/restrictionID" → deleted

	// Deploy state. Each slice holds every stored entity; the list handlers
	// filter it by the org/project/component the request names.
	deployments    []Deployment              // filtered by project_id
	environments   []DeployEnvironment       // filtered by org_id
	components     []DeployComponent         // filtered by org_id and (optionally) project_id
	compVersions   []DeployComponentVersion  // filtered by component id and (optionally) environment_id
	deploySettings map[string]DeploySettings // project id → settings entity
	rollback       *RollbackResult           // response for POST /api/v3/projects/{id}/rollback (nil → 404)

	// Policy state.
	policyBundles   map[string]map[string]string // "ownerID/ctx" → bundle
	decisionLogs    map[string][]DecisionLog     // "ownerID/ctx" → logs
	decisionResults map[string]DecisionResult    // "ownerID/ctx" → decision response
	policySettings  map[string]bool              // "ownerID/ctx" → enabled

	// iOS code signing state.
	iosCerts          map[string][]IOSCert          // org id → certificates
	iosBundles        map[string][]IOSSigningConfig // org id → signing configs
	deletedIOSCerts   map[string]bool               // cert id → deleted
	deletedIOSBundles map[string]bool               // bundle id → deleted
	iosCertCounter    int                           // monotonic ID generator for uploaded certs
	iosBundleCounter  int                           // monotonic ID generator for created bundles
	iosProfileCounter int                           // monotonic ID generator for provisioning profiles

	// Auth state.
	tokens             map[string]bool // accepted bearer tokens; a request whose Authorization: Bearer <token> is absent from this set is rejected 401 on every non-exempt route
	me                 *User           // authenticated user for GET /api/v3/users?filter[user_id]=me (nil → 401)
	collaborations     []Collaboration // response for GET /api/v2/me/collaborations
	oauthTokenResponse any             // response body for POST /oauth/token
	oauthTokenStatus   int             // HTTP status for POST /oauth/token (0 → 200 OK)
	parRequests        []url.Values    // recorded POST /oauth/par request bodies, in order
	parCounter         int             // monotonic ID generator for request_uri values

	// Orb state (v3).
	orbPackages        map[string]Orb         // id → package
	orbPackagesByName  map[string]string      // "ns/name" → id
	orbVersions        map[string]OrbVersion  // id → version
	orbVersionsByRef   map[string]string      // "ns/name@version" → id
	orbVersionsByOrbID map[string][]string    // orbID → ordered version IDs (newest first)
	orbCategories      map[string]OrbCategory // id → category
	// orbAddCategoryStatus, when non-zero, is the HTTP status returned for every
	// POST /api/v3/orb/packages/{id}/add-category, so a test can exercise how a
	// caller copes with the registry refusing a category.
	orbAddCategoryStatus int
	orbCategoriesByName  map[string]string        // name → id
	orbValidateResponse  *orbFakeValidateResponse // override for validate/process responses
	orbCreatedPackages   []Orb                    // packages created via POST
	orbCreatedVersions   []OrbVersion             // versions created via POST
	orbUnlistedPackages  map[string]bool          // id → unlisted
	orbCategoryMembers   map[string][]string      // packageID → []categoryID

	// Namespace state (served via /graphql-unstable).
	namespaces        map[string]Namespace // namespace id → namespace
	namespacesByName  map[string]string    // namespace name → id
	deletedNamespaces map[string]bool      // namespace id → deleted

	// DLC state.
	dlcPurgeStatus map[string]int // projectID → HTTP status to return (default 204)
	// Config compile state.
	compileValid       bool
	compileOutputYAML  string
	compileErrors      []string
	lastCompileOwnerID string

	// Org state.
	orgs        map[string]Org  // org slug → resolved org
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

		pipelines:                         map[string]PipelineV2{},
		projects:                          map[string][]PipelineV2{},
		jobArtifacts:                      map[string][]any{},
		jobArtifactsV3:                    map[string][]Artifact{},
		staticFiles:                       map[string]string{},
		triggerResponses:                  map[string]any{},
		triggerPipelineRunResponses:       map[string]any{},
		triggerPipelineRunStatuses:        map[string]int{},
		pipelineDefinitions:               map[string][]any{},
		createPipelineDefinitionResponses: map[string]any{},
		createTriggerResponses:            map[string]any{},
		listTriggerResponses:              map[string][]any{},
		providerConnections:               map[string][]string{},
		githubAppRepos:                    map[string][]GitHubAppRepo{},
		rerunResponses:                    map[string]int{},
		rerunNewIDs:                       map[string]string{},
		rerunFromFailed:                   map[string]bool{},
		cancelResponses:                   map[string]int{},
		pipelineCancelResponses:           map[string]int{},
		jobsV3:                            map[string]JobV3{},
		workflowJobsV3:                    map[string][]JobV3{},
		jobStdout:                         map[string][]byte{},
		jobStderr:                         map[string][]byte{},
		jobStdoutCondensed:                map[string][]byte{},
		jobTests:                          map[string][]TestResult{},
		jobResourceUsage:                  map[string]ResourceUsage{},
		runsV3:                            map[string]RunV3{},
		runsV3ByProject:                   map[string][]RunV3{},
		workflowsV3:                       map[string]WorkflowV3{},
		workflowsV3ByRun:                  map[string][]WorkflowV3{},
		workflowsV3NotFound:               map[string]bool{},
		resourceClasses:                   []ResourceClass{},
		runnerTokens:                      map[string][]RunnerToken{},
		runnerInstances:                   []RunnerInstance{},
		deletedTokens:                     map[string]bool{},
		deletedRCs:                        map[string]bool{},
		followedProjects:                  []FollowedProject{},
		followedSlugs:                     map[string]bool{},
		envVars:                           map[string][]EnvVar{},
		deletedEnvVars:                    map[string]bool{},
		contexts:                          map[string]Context{},
		contextsByOrg:                     map[string][]Context{},
		contextEnvVars:                    map[string][]ContextEnvVar{},
		contextRestrictions:               map[string][]ContextRestriction{},
		deletedContexts:                   map[string]bool{},
		deletedContextVars:                map[string]bool{},
		deletedContextRestrictions:        map[string]bool{},
		projectInfos:                      map[string]ProjectInfo{},
		projectsByID:                      map[string]any{},
		projectsBySlug:                    map[string]ProjectV3{},
		projectSettings:                   map[string]any{},
		deploySettings:                    map[string]DeploySettings{},
		policyBundles:                     make(map[string]map[string]string),
		decisionLogs:                      make(map[string][]DecisionLog),
		decisionResults:                   make(map[string]DecisionResult),
		policySettings:                    make(map[string]bool),
		namespaces:                        map[string]Namespace{},
		namespacesByName:                  map[string]string{},
		deletedNamespaces:                 map[string]bool{},
		iosCerts:                          map[string][]IOSCert{},
		iosBundles:                        map[string][]IOSSigningConfig{},
		deletedIOSCerts:                   map[string]bool{},
		deletedIOSBundles:                 map[string]bool{},
		orbPackages:                       map[string]Orb{},
		orbPackagesByName:                 map[string]string{},
		orbVersions:                       map[string]OrbVersion{},
		orbVersionsByRef:                  map[string]string{},
		orbVersionsByOrbID:                map[string][]string{},
		orbCategories:                     map[string]OrbCategory{},
		orbCategoriesByName:               map[string]string{},
		orbUnlistedPackages:               map[string]bool{},
		orbCategoryMembers:                map[string][]string{},
		dlcPurgeStatus:                    map[string]int{},
		compileValid:                      true,
		compileOutputYAML:                 "# compiled output\nversion: \"2.1\"\n",
		orgs:                              map[string]Org{},
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
	r.Get("/api/v2/project/{vcs}/{org}/{repo}/{jobNumber}/artifacts", f.handleGetJobArtifacts)
	r.Post("/api/v2/project/{vcs}/{org}/{repo}/pipeline", f.handleTriggerPipeline)
	r.Post("/api/v2/project/{vcs}/{org}/{repo}/pipeline/run", f.handleTriggerPipelineRun)
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
	r.Get("/api/v3/pipelines", f.handleListPipelineDefinitions)
	r.Post("/api/v3/pipelines", f.handleCreatePipelineDefinition)
	r.Get("/api/v3/triggers", f.handleListTriggers)
	r.Post("/api/v3/triggers", f.handleCreateTrigger)
	// GitHub App routes.
	r.Get("/api/v3/provider/connections", f.handleListProviderConnections)
	r.Post("/api/v2/github-app/install", f.handleInstallGitHubApp)
	r.Get("/api/v2/github-app/organization/{orgID}/repositories", f.handleListGitHubAppRepositories)
	// Policy routes.
	r.Post("/api/v2/owner/{ownerID}/context/{policyCtx}/policy-bundle", f.handleCreatePolicyBundle)
	r.Get("/api/v2/owner/{ownerID}/context/{policyCtx}/policy-bundle", f.handleFetchPolicyBundle)
	r.Get("/api/v2/owner/{ownerID}/context/{policyCtx}/policy-bundle/{name}", f.handleFetchPolicyBundleByName)
	r.Get("/api/v2/owner/{ownerID}/context/{policyCtx}/decision", f.handleGetDecisionLogs)
	r.Post("/api/v2/owner/{ownerID}/context/{policyCtx}/decision", f.handleMakeDecision)
	r.Get("/api/v2/owner/{ownerID}/context/{policyCtx}/decision/settings", f.handleGetPolicySettings)
	r.Patch("/api/v2/owner/{ownerID}/context/{policyCtx}/decision/settings", f.handleSetPolicySettings)
	r.Get("/api/v2/owner/{ownerID}/context/{policyCtx}/decision/{id}", f.handleGetDecisionLog)
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
	r.Post("/api/v3/signing/configs/{id}/update-profile", f.handleUpdateIOSBundleProfile)
	r.Post("/api/v3/signing/configs/{id}/remove-profile", f.handleRemoveIOSBundleProfile)
	// Config compile + org routes.
	r.Post("/api/v3/configs/compile", f.handleCompileConfig)
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
	r.Get("/api/v3/jobs/{id}/resource-usage", f.handleGetJobResourceUsage)
	// Workflow (v3) routes.
	r.Get("/api/v3/workflows/{id}", f.handleGetWorkflowV3ByID)
	r.Get("/api/v3/workflows", f.handleGetWorkflowsV3)
	// Project (v3) routes.
	r.Get("/api/v3/projects", f.handleResolveProjectBySlug)
	r.Get("/api/v3/projects/{id}", f.handleGetProjectV3)
	r.Get("/api/v3/projects/{id}/settings", f.handleGetProjectSettingsV3)
	r.Post("/api/v3/projects/{id}/update-settings", f.handleUpdateProjectSettingsV3)
	r.Post("/api/v3/projects/{id}/rollback", f.handleRollbackProject)
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
	case "/api/v3/configs/compile", "/api/v3/tool/releases":
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

// PipelineV2 is a stored v2 pipeline served by the pipeline get/get-by-number
// and project pipeline-list endpoints. The structural fixture fields (trigger
// type, actor, repository URLs) are constant across tests and supplied by the
// renderer; only the fields below vary.
type PipelineV2 struct {
	ID          string
	Number      int
	State       string
	ProjectSlug string
	Branch      string
	Revision    string
	CreatedAt   string
	UpdatedAt   string
}

// AddRun registers a run response for GET /api/v2/pipeline/<id>.
func (f *CircleCI) AddRun(id string, run PipelineV2) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pipelines[id] = run
}

// AddProjectRuns registers runs for GET /api/v2/project/<slug>/pipeline.
// slug should be in "vcs/org/repo" form, e.g. "gh/myorg/myrepo".
func (f *CircleCI) AddProjectRuns(slug string, runs ...PipelineV2) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projects[slug] = runs
}

// pipelineV2Entity renders a stored PipelineV2 as its v2 wire object, filling in
// the constant trigger/actor/vcs scaffold every fixture shares.
func pipelineV2Entity(p PipelineV2) map[string]any {
	return map[string]any{
		"id":           p.ID,
		"state":        p.State,
		"number":       p.Number,
		"project_slug": p.ProjectSlug,
		"created_at":   p.CreatedAt,
		"updated_at":   p.UpdatedAt,
		"trigger": map[string]any{
			"type":        "webhook",
			"received_at": p.CreatedAt,
			"actor":       map[string]any{"login": "testuser", "avatar_url": ""},
		},
		"vcs": map[string]any{
			"provider_name":         "GitHub",
			"origin_repository_url": "https://github.com/testorg/testrepo",
			"target_repository_url": "https://github.com/testorg/testrepo",
			"revision":              p.Revision,
			"branch":                p.Branch,
		},
	}
}

// JobV3 is a stored job served by both the workflow-jobs list
// (GET /api/v3/workflows/{id}/jobs) and the job-detail endpoint
// (GET /api/v3/jobs/{id}). The list exposes the summary attributes and
// references; detail adds the nested parallel executions. Optional fields
// (Outcome, StartedAt, EndedAt, the reference ids) are omitted from the wire
// entity when empty, and Executions is omitted when nil — matching how the real
// API reports a queued job that has not run yet.
type JobV3 struct {
	ID         string
	Name       string
	Type       string
	Phase      string
	Outcome    string
	StartedAt  string
	EndedAt    string
	ProjectID  string
	WorkflowID string
	PipelineID string
	UserID     string
	Executions [][]JobStep // parallel_executions, one inner slice of steps per execution
}

// JobStep is a single step within a JobV3 execution. ExitCode is a pointer so a
// step that never ran a command (spin-up, checkout) omits it, distinct from an
// explicit exit code of 0; Command is omitted when empty.
type JobStep struct {
	Name      string
	Type      string
	Num       int
	Phase     string
	Outcome   string
	ExitCode  *int
	Command   string
	StartedAt string
	EndedAt   string
}

// AddWorkflowJobsV3 registers the jobs a workflow lists.
func (f *CircleCI) AddWorkflowJobsV3(workflowID string, jobs ...JobV3) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workflowJobsV3[workflowID] = jobs
}

// jobV3Entity renders a stored JobV3 as its V3 entity, omitting the optional
// attributes and references that are empty.
func jobV3Entity(j JobV3) map[string]any {
	attrs := map[string]any{"name": j.Name, "type": j.Type, "phase": j.Phase}
	if j.Outcome != "" {
		attrs["outcome"] = j.Outcome
	}
	if j.StartedAt != "" {
		attrs["started_at"] = j.StartedAt
	}
	if j.EndedAt != "" {
		attrs["ended_at"] = j.EndedAt
	}
	if len(j.Executions) > 0 {
		execs := make([]any, 0, len(j.Executions))
		for _, steps := range j.Executions {
			rendered := make([]any, 0, len(steps))
			for _, s := range steps {
				rendered = append(rendered, jobStepEntity(s))
			}
			execs = append(execs, map[string]any{"steps": rendered})
		}
		attrs["parallel_executions"] = execs
	}
	refs := map[string]any{}
	for key, id := range map[string]string{
		"project":  j.ProjectID,
		"workflow": j.WorkflowID,
		"pipeline": j.PipelineID,
		"user":     j.UserID,
	} {
		if id != "" {
			refs[key] = map[string]any{"id": id}
		}
	}
	return map[string]any{"id": j.ID, "attributes": attrs, "references": refs}
}

// jobStepEntity renders a single JobStep, omitting exit_code when unset and
// command when empty.
func jobStepEntity(s JobStep) map[string]any {
	step := map[string]any{
		"name":       s.Name,
		"type":       s.Type,
		"num":        s.Num,
		"phase":      s.Phase,
		"outcome":    s.Outcome,
		"started_at": s.StartedAt,
		"ended_at":   s.EndedAt,
	}
	if s.ExitCode != nil {
		step["exit_code"] = *s.ExitCode
	}
	if s.Command != "" {
		step["command"] = s.Command
	}
	return step
}

// AddJobArtifacts registers artifact responses for a job.
// slug should be in "vcs/org/repo" form; jobNumber is the integer job number.
func (f *CircleCI) AddJobArtifacts(slug string, jobNumber int64, artifactItems ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fmt.Sprintf("%s/%d", slug, jobNumber)
	f.jobArtifacts[key] = artifactItems
}

// Artifact is a stored job artifact served by GET /api/v3/jobs/<id>/artifacts.
// Execution is the parallel-run index (0-based) the artifact came from.
type Artifact struct {
	Path      string
	URL       string
	Execution int
}

// AddJobArtifactsV3 registers V3 artifacts for a job UUID.
func (f *CircleCI) AddJobArtifactsV3(jobID string, items ...Artifact) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobArtifactsV3[jobID] = items
}

// artifactEntity renders a stored Artifact as its V3 entity. The id is a stable
// UUIDv5 of the artifact's URL and execution — the CLI ignores it, but the
// envelope requires one.
func artifactEntity(a Artifact) map[string]any {
	return map[string]any{
		"id": uuid.NewSHA1(uuid.NameSpaceURL, fmt.Appendf(nil, "%s#%d", a.URL, a.Execution)).String(),
		"attributes": map[string]any{
			"path":      a.Path,
			"url":       a.URL,
			"execution": a.Execution,
		},
	}
}

// AddJobV3 registers a job's detail, served by GET /api/v3/jobs/<id> keyed on
// the job's ID.
func (f *CircleCI) AddJobV3(job JobV3) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobsV3[job.ID] = job
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
	render.JSON(w, r, pipelineV2Entity(p))
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
		if strconv.Itoa(p.Number) == numberStr {
			render.JSON(w, r, pipelineV2Entity(p))
			return
		}
	}
	render.Status(r, http.StatusNotFound)
	render.JSON(w, r, map[string]any{"message": "not found"})
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
	items := make([]any, 0, len(f.jobArtifactsV3[id]))
	for _, a := range f.jobArtifactsV3[id] {
		items = append(items, artifactEntity(a))
	}
	f.mu.RUnlock()

	render.JSON(w, r, map[string]any{"data": items})
}

func (f *CircleCI) handleListProjectPipelines(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	f.mu.RLock()
	items := make([]any, 0, len(f.projects[slug]))
	for _, p := range f.projects[slug] {
		items = append(items, pipelineV2Entity(p))
	}
	f.mu.RUnlock()

	render.JSON(w, r, map[string]any{"items": items, "next_page_token": nil})
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
	render.JSON(w, r, map[string]any{"data": jobV3Entity(job)})
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

// TestResult is a stored test result served by the fake job-tests endpoint,
// which streams these as newline-delimited JSON (JSONL). The JSON tags match
// the wire fields the CLI decodes.
type TestResult struct {
	Classname string  `json:"classname"`
	Name      string  `json:"name"`
	Result    string  `json:"result"`
	RunTime   float64 `json:"run_time"`
	Message   string  `json:"message"`
}

// AddJobTests registers test-result records for a job UUID, served as
// newline-delimited JSON (JSONL) at GET /api/v3/jobs/<id>/tests.
func (f *CircleCI) AddJobTests(id string, tests ...TestResult) {
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

// ResourceUsage is a job's stored resource usage, served by
// GET /api/v3/jobs/<id>/resource-usage. It is named for the endpoint rather than
// the resource class it carries, since ResourceClass is already taken by the
// runner fixtures.
type ResourceUsage struct {
	ClassName        string
	CPUCount         float64
	MemoryLimitBytes int64
	Executions       []ResourceUsageExecution
}

// ResourceUsageExecution is one parallel execution's usage series. Index is the
// execution index the entity reports; the caller sets it explicitly rather than
// relying on slice position, so a test can register a non-zero index on its own.
type ResourceUsageExecution struct {
	Index          int
	IntervalMS     int
	CPUCores       []float64
	MemoryBytes    []int64
	NetworkRxBytes int64
	NetworkTxBytes int64
}

// AddJobResourceUsage registers a job's sampled resource usage, served by
// GET /api/v3/jobs/<id>/resource-usage. A job with none registered returns 404,
// matching the real endpoint's answer for a job that never ran an executor.
func (f *CircleCI) AddJobResourceUsage(id string, usage ResourceUsage) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobResourceUsage[id] = usage
}

func (f *CircleCI) handleGetJobResourceUsage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	usage, ok := f.jobResourceUsage[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}

	execs := make([]map[string]any, len(usage.Executions))
	for i, e := range usage.Executions {
		execs[i] = map[string]any{
			"execution":        e.Index,
			"interval_ms":      e.IntervalMS,
			"cpu_cores":        e.CPUCores,
			"memory_bytes":     e.MemoryBytes,
			"network_rx_bytes": e.NetworkRxBytes,
			"network_tx_bytes": e.NetworkTxBytes,
		}
	}
	render.JSON(w, r, map[string]any{"data": map[string]any{
		"id": id,
		"attributes": map[string]any{
			"resource_class": map[string]any{
				"name":               usage.ClassName,
				"cpu_count":          usage.CPUCount,
				"memory_limit_bytes": usage.MemoryLimitBytes,
			},
			"parallel_executions": execs,
		},
	}})
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
	jobs := []any{}
	for _, j := range f.workflowJobsV3[workflowID] {
		jobs = append(jobs, jobV3Entity(j))
	}
	f.mu.RUnlock()

	render.JSON(w, r, map[string]any{"data": jobs})
}

// WorkflowV3 is a stored workflow served by the workflow-detail
// (GET /api/v3/workflows/{id}) and run-workflows list endpoints. An ended
// workflow reports outcome + ended_at; a still-running one reports
// current_outcome and no ended_at instead — mirroring the real API.
type WorkflowV3 struct {
	ID        string
	Name      string
	RunID     string
	ProjectID string
	UserID    string
	Phase     string
	Outcome   string
	CreatedAt string
	EndedAt   string
}

// AddWorkflowV3 registers a workflow served by GET /api/v3/workflows/<id>.
func (f *CircleCI) AddWorkflowV3(id string, workflow WorkflowV3) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workflowsV3[id] = workflow
}

// AddRunWorkflowsV3 registers the workflows a run lists.
func (f *CircleCI) AddRunWorkflowsV3(runID string, workflows ...WorkflowV3) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workflowsV3ByRun[runID] = workflows
}

// workflowV3Entity renders a stored WorkflowV3 as its V3 entity, choosing the
// outcome/ended_at vs current_outcome shape from the phase.
func workflowV3Entity(wf WorkflowV3) map[string]any {
	attrs := map[string]any{
		"name":       wf.Name,
		"phase":      wf.Phase,
		"created_at": wf.CreatedAt,
	}
	if wf.Phase == "ended" {
		attrs["outcome"] = wf.Outcome
		attrs["ended_at"] = wf.EndedAt
	} else {
		attrs["current_outcome"] = wf.Outcome
	}
	return map[string]any{
		"id":         wf.ID,
		"attributes": attrs,
		"references": map[string]any{
			"run":     map[string]any{"id": wf.RunID},
			"project": map[string]any{"id": wf.ProjectID},
			"user":    map[string]any{"id": wf.UserID},
		},
	}
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
	render.JSON(w, r, map[string]any{"data": workflowV3Entity(wf)})
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
	items := make([]any, 0, len(workflows))
	for _, wf := range workflows {
		items = append(items, workflowV3Entity(wf))
	}
	render.JSON(w, r, map[string]any{"data": items})
}

// RunV3 is a stored run served by the run-detail, run-search, and my-runs
// endpoints. A run that resolved a revision renders a commit block; Tag,
// OriginRepoURL and Errors render only when set. CurrentOutcome is omitted when
// empty, matching a run that has not finished.
type RunV3 struct {
	ID             string
	ProjectID      string
	UserID         string
	Phase          string
	CurrentOutcome string
	CreatedAt      string
	Branch         string
	Tag            string
	Revision       string
	OriginRepoURL  string
	Errors         []RunError
}

// RunError is a config/setup error attached to a run, surfaced by run get.
type RunError struct {
	Type    string
	Message string
}

// AddRunV3 registers a run served by GET /api/v3/runs/<id> and included in the
// search results for its project.
func (f *CircleCI) AddRunV3(id, projectID string, run RunV3) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runsV3[id] = run
	f.runsV3ByProject[projectID] = append(f.runsV3ByProject[projectID], run)
}

// SetUserRuns registers the runs returned by
// GET /api/v3/runs?filter[user_id]=me (i.e. "circleci my runs").
func (f *CircleCI) SetUserRuns(runs ...RunV3) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.userRunsV3 = runs
}

// runV3Entity renders a stored RunV3 as its V3 entity. The VCS block always
// carries branch and revision; tag, origin_repository_url and a commit
// sub-object render only when the run has them.
func runV3Entity(run RunV3) map[string]any {
	attrs := map[string]any{
		"phase":      run.Phase,
		"created_at": run.CreatedAt,
	}
	if run.CurrentOutcome != "" {
		attrs["current_outcome"] = run.CurrentOutcome
	}
	if len(run.Errors) > 0 {
		errs := make([]any, 0, len(run.Errors))
		for _, e := range run.Errors {
			errs = append(errs, map[string]any{"type": e.Type, "message": e.Message})
		}
		attrs["errors"] = errs
	}
	vcs := map[string]any{
		"branch":   run.Branch,
		"revision": run.Revision,
	}
	if run.Tag != "" {
		vcs["tag"] = run.Tag
	}
	if run.OriginRepoURL != "" {
		vcs["origin_repository_url"] = run.OriginRepoURL
	}
	if run.Revision != "" {
		vcs["commit"] = map[string]any{
			"subject": "Fix the widget",
			"url":     "https://github.com/testorg/testrepo/commit/" + run.Revision,
			"author":  map[string]any{"name": "Ada Lovelace", "login": "ada"},
		}
	}
	return map[string]any{
		"id":         run.ID,
		"attributes": attrs,
		"references": map[string]any{
			"event":   map[string]any{"attributes": map[string]any{"vcs": vcs}},
			"trigger": map[string]any{"attributes": map[string]any{"event_source": map[string]any{"type": "webhook"}}},
			"project": map[string]any{"id": run.ProjectID},
			"user":    map[string]any{"id": run.UserID},
		},
	}
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
		if phase != "" && run.Phase != phase {
			continue
		}
		if currentOutcome != "" && run.CurrentOutcome != currentOutcome {
			continue
		}
		results = append(results, runV3Entity(run))
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
	render.JSON(w, r, map[string]any{"data": runV3Entity(run)})
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
			if branch != "" && run.Branch != branch {
				continue
			}
			if status != "" && runStatus(run) != status {
				continue
			}
			all = append(all, runV3Entity(run))
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
func runStatus(run RunV3) string {
	if run.Phase != "ended" {
		return run.Phase
	}
	if run.CurrentOutcome == "succeeded" {
		return "success"
	}
	return run.CurrentOutcome
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

// --- Runner helpers ---

// ResourceClass is a stored runner resource class served by the runner
// resource-class list and create endpoints. Slug is the "resource_class" wire
// field (e.g. "my-org/linux-runner"); the list filters on its namespace prefix.
type ResourceClass struct {
	ID          string
	Slug        string
	Description string
}

// RunnerToken is a stored runner token served by the runner token list and
// create endpoints. Token carries the secret value, which only the create
// response includes — it renders when set.
type RunnerToken struct {
	ID            string
	ResourceClass string
	Nickname      string
	CreatedAt     string
	Token         string
}

// RunnerInstance is a stored runner instance served by the runner instance
// list, filtered by resource-class or namespace prefix.
type RunnerInstance struct {
	ResourceClass  string
	Hostname       string
	Name           string
	Version        string
	IP             string
	FirstConnected string
	LastConnected  string
	LastUsed       string
}

// AddResourceClass registers a runner resource class.
func (f *CircleCI) AddResourceClass(rc ResourceClass) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resourceClasses = append(f.resourceClasses, rc)
}

// AddRunnerToken adds a token to the fake server for the given resource class.
func (f *CircleCI) AddRunnerToken(resourceClass string, token RunnerToken) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runnerTokens[resourceClass] = append(f.runnerTokens[resourceClass], token)
}

// AddRunnerInstance adds a runner instance to the fake server's list.
func (f *CircleCI) AddRunnerInstance(instance RunnerInstance) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runnerInstances = append(f.runnerInstances, instance)
}

// resourceClassEntity renders a stored ResourceClass as its wire object.
func resourceClassEntity(rc ResourceClass) map[string]any {
	return map[string]any{
		"id":             rc.ID,
		"resource_class": rc.Slug,
		"description":    rc.Description,
	}
}

// runnerTokenEntity renders a stored RunnerToken, including the secret value
// only when set (create responses carry it; the list does not).
func runnerTokenEntity(t RunnerToken) map[string]any {
	m := map[string]any{
		"id":             t.ID,
		"resource_class": t.ResourceClass,
		"nickname":       t.Nickname,
		"created_at":     t.CreatedAt,
	}
	if t.Token != "" {
		m["token"] = t.Token
	}
	return m
}

// runnerInstanceEntity renders a stored RunnerInstance as its wire object.
func runnerInstanceEntity(i RunnerInstance) map[string]any {
	return map[string]any{
		"resource_class":  i.ResourceClass,
		"hostname":        i.Hostname,
		"name":            i.Name,
		"version":         i.Version,
		"ip":              i.IP,
		"first_connected": i.FirstConnected,
		"last_connected":  i.LastConnected,
		"last_used":       i.LastUsed,
	}
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

	items := []any{}
	for _, rc := range all {
		if deleted[rc.Slug] {
			continue
		}
		if ns != "" && !strings.HasPrefix(rc.Slug, ns+"/") {
			continue
		}
		items = append(items, resourceClassEntity(rc))
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
	rc := ResourceClass{ID: fmt.Sprintf("rc-%s", slug), Slug: slug, Description: desc}
	f.mu.Lock()
	f.resourceClasses = append(f.resourceClasses, rc)
	f.mu.Unlock()
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, resourceClassEntity(rc))
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
		if rc.ID == id {
			found = true
			hasTokens = len(f.runnerTokens[rc.Slug]) > 0
			if force || !hasTokens {
				f.deletedRCs[rc.Slug] = true
				delete(f.runnerTokens, rc.Slug)
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

	items := []any{}
	for _, tok := range tokens {
		if !deleted[tok.ID] {
			items = append(items, runnerTokenEntity(tok))
		}
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
	tok := RunnerToken{
		ID:            fmt.Sprintf("tok-%s", rc),
		ResourceClass: rc,
		Nickname:      nickname,
		CreatedAt:     "2026-01-01T00:00:00Z",
		Token:         "fake-runner-token-value",
	}
	f.mu.Lock()
	f.runnerTokens[rc] = append(f.runnerTokens[rc], tok)
	f.mu.Unlock()
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, runnerTokenEntity(tok))
}

func (f *CircleCI) handleDeleteRunnerToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.Lock()
	found := false
	for _, tokens := range f.runnerTokens {
		for _, tok := range tokens {
			if tok.ID == id {
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

	items := []any{}
	for _, inst := range all {
		if rc != "" && inst.ResourceClass != rc {
			continue
		}
		if ns != "" && !strings.HasPrefix(inst.ResourceClass, ns+"/") {
			continue
		}
		items = append(items, runnerInstanceEntity(inst))
	}
	render.JSON(w, r, map[string]any{"items": items})
}

// --- Auth helpers ---

// User is the authenticated user served by GET /api/v3/users?filter[user_id]=me.
// AvatarURL renders only when set.
type User struct {
	ID        string
	Name      string
	Login     string
	AvatarURL string
}

// Collaboration is an org the authenticated user collaborates on, served by
// GET /api/v2/me/collaborations.
type Collaboration struct {
	ID      string
	Name    string
	Slug    string
	VCSType string
}

// SetMe sets the authenticated user returned by the users?filter[user_id]=me
// endpoint. When unset, that endpoint answers 401.
func (f *CircleCI) SetMe(me User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.me = &me
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
		"data": []any{userEntity(*me)},
		"page": map[string]any{"next": nil, "prev": nil},
	})
}

// userEntity renders a stored User as its V3 entity, including avatar_url only
// when set.
func userEntity(u User) map[string]any {
	attrs := map[string]any{"name": u.Name, "login": u.Login}
	if u.AvatarURL != "" {
		attrs["avatar_url"] = u.AvatarURL
	}
	return map[string]any{"id": u.ID, "attributes": attrs}
}

// SetCollaborations sets the response for GET /api/v2/me/collaborations.
func (f *CircleCI) SetCollaborations(collabs ...Collaboration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.collaborations = collabs
}

func (f *CircleCI) handleGetCollaborations(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	items := make([]any, 0, len(f.collaborations))
	for _, c := range f.collaborations {
		items = append(items, collaborationEntity(c))
	}
	f.mu.RUnlock()

	render.JSON(w, r, items)
}

// collaborationEntity renders a stored Collaboration as its wire object.
func collaborationEntity(c Collaboration) map[string]any {
	return map[string]any{"id": c.ID, "name": c.Name, "slug": c.Slug, "vcs_type": c.VCSType}
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

// ProjectInfo is a stored v1.1/v2 project record served by
// GET /api/v2/project/<slug>. Only ID and Slug are always present; the org
// fields and VCSInfo render only when set.
type ProjectInfo struct {
	ID               string
	Slug             string
	Name             string
	OrganizationName string
	OrganizationSlug string
	OrganizationID   string
	VCSInfo          *VCSInfo
}

// VCSInfo is the nested vcs_info block on a ProjectInfo.
type VCSInfo struct {
	Provider      string
	DefaultBranch string
	VCSURL        string
}

// AddProjectInfo registers a project info response for GET /api/v2/project/<slug>.
// slug should be in "vcs/org/repo" form.
func (f *CircleCI) AddProjectInfo(slug string, info ProjectInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projectInfos[slug] = info
}

// projectInfoEntity renders a stored ProjectInfo, omitting the optional fields
// that are empty.
func projectInfoEntity(p ProjectInfo) map[string]any {
	m := map[string]any{"id": p.ID, "slug": p.Slug}
	for k, v := range map[string]string{
		"name":              p.Name,
		"organization_name": p.OrganizationName,
		"organization_slug": p.OrganizationSlug,
		"organization_id":   p.OrganizationID,
	} {
		if v != "" {
			m[k] = v
		}
	}
	if p.VCSInfo != nil {
		m["vcs_info"] = map[string]any{
			"provider":       p.VCSInfo.Provider,
			"default_branch": p.VCSInfo.DefaultBranch,
			"vcs_url":        p.VCSInfo.VCSURL,
		}
	}
	return m
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

// ProjectV3 is a stored project resolved by
// GET /api/v3/projects?filter[slug]=<slug>. The entity carries its UUID, name,
// and owning org UUID — enough for the CLI to map a slug to its project.
type ProjectV3 struct {
	ID    string
	Name  string
	OrgID string
}

// AddProjectBySlug registers a project resolved by GET
// /api/v3/projects?filter[slug]=<slug>, returning its UUID, name, and org UUID.
func (f *CircleCI) AddProjectBySlug(slug, id, name, orgID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projectsBySlug[slug] = ProjectV3{ID: id, Name: name, OrgID: orgID}
}

// projectV3Entity renders a stored ProjectV3 as its V3 entity.
func projectV3Entity(p ProjectV3) map[string]any {
	return map[string]any{
		"id":         p.ID,
		"attributes": map[string]any{"name": p.Name},
		"references": map[string]any{"org": map[string]any{"id": p.OrgID}},
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
		data = append(data, projectV3Entity(project))
	}
	render.JSON(w, r, map[string]any{"data": data, "page": map[string]any{"next": nil, "prev": nil}})
}

// AddPipelineDefinition registers a pipeline entity for a project, returned by
// GET /api/v3/pipelines?filter[project_id]=. def is a v3 data entity
// ({id, attributes, references}); the fake supplies the collection envelope.
func (f *CircleCI) AddPipelineDefinition(projectID string, def any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pipelineDefinitions[projectID] = append(f.pipelineDefinitions[projectID], def)
}

// SetCreatePipelineDefinitionResponse registers the entity returned when
// POST /api/v3/pipelines is called with this project in its references.
// Pass nil to simulate a 404.
func (f *CircleCI) SetCreatePipelineDefinitionResponse(projectID string, resp any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createPipelineDefinitionResponses[projectID] = resp
}

// AddTrigger registers a trigger entity returned by GET /api/v3/triggers when
// filtered by this project and pipeline.
func (f *CircleCI) AddTrigger(projectID, pipelineDefinitionID string, trigger any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := projectID + "/" + pipelineDefinitionID
	f.listTriggerResponses[key] = append(f.listTriggerResponses[key], trigger)
}

// SetCreateTriggerResponse registers the entity returned when POST
// /api/v3/triggers is called for this project and pipeline. Pass nil to simulate
// a 404.
func (f *CircleCI) SetCreateTriggerResponse(projectID, pipelineDefinitionID string, resp any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createTriggerResponses[projectID+"/"+pipelineDefinitionID] = resp
}

// FollowedProject is a stored project served by GET /api/v1.1/projects.
type FollowedProject struct {
	Slug     string
	Username string
	Reponame string
	VCSType  string
	Name     string
}

// AddFollowedProject registers a project returned by GET /api/v1.1/projects.
func (f *CircleCI) AddFollowedProject(proj FollowedProject) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.followedProjects = append(f.followedProjects, proj)
}

// followedProjectEntity renders a stored FollowedProject as its v1.1 object.
func followedProjectEntity(p FollowedProject) map[string]any {
	return map[string]any{
		"slug":     p.Slug,
		"username": p.Username,
		"reponame": p.Reponame,
		"vcs_type": p.VCSType,
		"name":     p.Name,
	}
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

// EnvVar is a stored project environment variable served by the project env-var
// list/set endpoints. CreatedAt is a pointer so a variable with no timestamp
// renders created_at as null, matching the real API.
type EnvVar struct {
	Name      string
	Value     string
	CreatedAt *time.Time
}

// AddEnvVar registers an env var for a project.
// slug should be in "vcs/org/repo" form.
func (f *CircleCI) AddEnvVar(slug, name, value string, createdAt *time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envVars[slug] = append(f.envVars[slug], EnvVar{Name: name, Value: value, CreatedAt: createdAt})
}

// envVarEntity renders a stored EnvVar as its wire object; created_at is always
// present (null when unset).
func envVarEntity(v EnvVar) map[string]any {
	return map[string]any{"name": v.Name, "value": v.Value, "created_at": v.CreatedAt}
}

// --- Project / env-var handlers ---

func (f *CircleCI) handleListProjects(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	projects := make([]any, 0, len(f.followedProjects))
	for _, p := range f.followedProjects {
		projects = append(projects, followedProjectEntity(p))
	}
	f.mu.RUnlock()

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
		f.followedProjects = append(f.followedProjects, FollowedProject{
			Slug:     slug,
			Username: org,
			Reponame: repo,
			VCSType:  vcs,
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

	items := []any{}
	for _, v := range vars {
		if !deleted[slug+"/"+v.Name] {
			items = append(items, envVarEntity(v))
		}
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

	ev := EnvVar{Name: name, Value: value}
	f.mu.Lock()
	// Remove any existing var with this name.
	var kept []EnvVar
	for _, v := range f.envVars[slug] {
		if v.Name == name {
			continue
		}
		kept = append(kept, v)
	}
	f.envVars[slug] = append(kept, ev)
	delete(f.deletedEnvVars, slug+"/"+name)
	f.mu.Unlock()

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, envVarEntity(ev))
}

// --- Context helpers ---

// Context is a stored context served by the context list/get/create endpoints.
type Context struct {
	ID        string
	Name      string
	CreatedAt string
}

// ContextEnvVar is a stored context environment variable. TruncatedValue is the
// masked value the list/get surface; a freshly-set variable has none, so it
// renders only when set.
type ContextEnvVar struct {
	Variable       string
	TruncatedValue string
	ContextID      string
	CreatedAt      string
	UpdatedAt      string
}

// ContextRestriction is a stored context restriction. ContextID renders only
// when set — the create response omits it, matching the real API.
type ContextRestriction struct {
	ContextID        string
	ID               string
	RestrictionType  string
	RestrictionValue string
	Name             string
}

// AddContext registers a context served by GET /api/v2/context/{id} and
// indexed by org slug for list responses.
func (f *CircleCI) AddContext(orgSlug string, ctx Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ctx.ID != "" {
		f.contexts[ctx.ID] = ctx
	}
	f.contextsByOrg[orgSlug] = append(f.contextsByOrg[orgSlug], ctx)
}

// AddContextEnvVar registers an environment variable for a context.
func (f *CircleCI) AddContextEnvVar(contextID string, envVar ContextEnvVar) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.contextEnvVars[contextID] = append(f.contextEnvVars[contextID], envVar)
}

// AddContextRestriction registers a restriction for a context.
func (f *CircleCI) AddContextRestriction(contextID string, restriction ContextRestriction) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.contextRestrictions[contextID] = append(f.contextRestrictions[contextID], restriction)
}

// contextEntity renders a stored Context as its wire object.
func contextEntity(c Context) map[string]any {
	return map[string]any{"id": c.ID, "name": c.Name, "created_at": c.CreatedAt}
}

// contextEnvVarEntity renders a stored ContextEnvVar, including the masked value
// only when set.
func contextEnvVarEntity(v ContextEnvVar) map[string]any {
	m := map[string]any{
		"variable":   v.Variable,
		"context_id": v.ContextID,
		"created_at": v.CreatedAt,
		"updated_at": v.UpdatedAt,
	}
	if v.TruncatedValue != "" {
		m["truncated_value"] = v.TruncatedValue
	}
	return m
}

// contextRestrictionEntity renders a stored ContextRestriction, including
// context_id only when set.
func contextRestrictionEntity(rr ContextRestriction) map[string]any {
	m := map[string]any{
		"id":                rr.ID,
		"name":              rr.Name,
		"restriction_type":  rr.RestrictionType,
		"restriction_value": rr.RestrictionValue,
	}
	if rr.ContextID != "" {
		m["context_id"] = rr.ContextID
	}
	return m
}

// --- Context handlers ---

func (f *CircleCI) handleListContexts(w http.ResponseWriter, r *http.Request) {
	ownerSlug := r.URL.Query().Get("owner-slug")
	nameFilter := r.URL.Query().Get("name")
	f.mu.RLock()
	items := f.contextsByOrg[ownerSlug]
	deleted := f.deletedContexts
	f.mu.RUnlock()

	result := []any{}
	for _, ctx := range items {
		if deleted[ctx.ID] {
			continue
		}
		if nameFilter != "" && !strings.Contains(strings.ToLower(ctx.Name), strings.ToLower(nameFilter)) {
			continue
		}
		result = append(result, contextEntity(ctx))
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
	ctx := Context{ID: id, Name: name, CreatedAt: "2026-01-01T00:00:00Z"}
	f.mu.Lock()
	f.contexts[id] = ctx
	if orgSlug != "" {
		f.contextsByOrg[orgSlug] = append(f.contextsByOrg[orgSlug], ctx)
	}
	f.mu.Unlock()
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, contextEntity(ctx))
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
	liveVars := []any{}
	for _, v := range vars {
		if deletedVars[id+"/"+v.Variable] {
			continue
		}
		liveVars = append(liveVars, contextEnvVarEntity(v))
	}
	liveRestrictions := []any{}
	for _, restr := range restrictions {
		if deletedRestrictions[id+"/"+restr.ID] {
			continue
		}
		liveRestrictions = append(liveRestrictions, contextRestrictionEntity(restr))
	}

	detail := map[string]any{
		"id":                    ctx.ID,
		"name":                  ctx.Name,
		"created_at":            ctx.CreatedAt,
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

	items := []any{}
	for _, v := range vars {
		if deleted[id+"/"+v.Variable] {
			continue
		}
		items = append(items, contextEnvVarEntity(v))
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
	ev := ContextEnvVar{
		Variable:  name,
		ContextID: id,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}
	f.mu.Lock()
	// Remove existing var with same name.
	var kept []ContextEnvVar
	for _, v := range f.contextEnvVars[id] {
		if v.Variable == name {
			continue
		}
		kept = append(kept, v)
	}
	f.contextEnvVars[id] = append(kept, ev)
	delete(f.deletedContextVars, id+"/"+name)
	f.mu.Unlock()
	render.JSON(w, r, contextEnvVarEntity(ev))
}

func (f *CircleCI) handleDeleteContextEnvVar(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	name := chi.URLParam(r, "name")
	key := id + "/" + name

	f.mu.Lock()
	found := false
	for _, v := range f.contextEnvVars[id] {
		if v.Variable == name {
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
	restr := ContextRestriction{
		ID:               "c0000003-0000-4000-8000-000000000003",
		RestrictionType:  restrictionType,
		RestrictionValue: restrictionValue,
	}
	f.mu.Lock()
	f.contextRestrictions[id] = append(f.contextRestrictions[id], restr)
	f.mu.Unlock()
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, contextRestrictionEntity(restr))
}

func (f *CircleCI) handleDeleteContextRestriction(w http.ResponseWriter, r *http.Request) {
	contextID := chi.URLParam(r, "id")
	restrictionID := chi.URLParam(r, "restriction_id")
	key := contextID + "/" + restrictionID

	f.mu.Lock()
	found := false
	for _, restr := range f.contextRestrictions[contextID] {
		if restr.ID == restrictionID {
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
	projectID := r.URL.Query().Get("filter[project_id]")
	f.mu.RLock()
	items := f.pipelineDefinitions[projectID]
	f.mu.RUnlock()

	renderV3Collection(w, r, items)
}

func (f *CircleCI) handleCreatePipelineDefinition(w http.ResponseWriter, r *http.Request) {
	// The v3 create takes the owning project in the body's references rather than
	// the path, so the registered response is keyed off what the client sent.
	projectID := v3BodyRefID(r, "project")
	f.mu.RLock()
	resp, ok := f.createPipelineDefinitionResponses[projectID]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]any{"data": resp})
}

func (f *CircleCI) handleListTriggers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	key := q.Get("filter[project_id]") + "/" + q.Get("filter[pipeline_id]")
	f.mu.RLock()
	items := f.listTriggerResponses[key]
	f.mu.RUnlock()

	renderV3Collection(w, r, items)
}

func (f *CircleCI) handleCreateTrigger(w http.ResponseWriter, r *http.Request) {
	// The project stays a query filter on the v3 create, but the parent pipeline
	// moves into the body's references.
	key := r.URL.Query().Get("filter[project_id]") + "/" + v3BodyRefID(r, "pipeline")
	f.mu.RLock()
	resp, ok := f.createTriggerResponses[key]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]any{"data": resp})
}

// renderV3Collection writes items as a v3 collection, so an empty set is an empty
// data array rather than a null.
func renderV3Collection(w http.ResponseWriter, r *http.Request, items []any) {
	if items == nil {
		items = []any{}
	}
	render.JSON(w, r, map[string]any{
		"data": items,
		"meta": map[string]any{"total_count": len(items)},
	})
}

// v3BodyRefID reads data.references.<name>.id out of a v3 create body, restoring
// the request body for any later reader. It returns "" when the body is absent or
// not shaped that way, which lands the caller on the fake's 404.
func v3BodyRefID(r *http.Request, name string) string {
	if r.Body == nil {
		return ""
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))

	var body struct {
		Data struct {
			References map[string]struct {
				ID string `json:"id"`
			} `json:"references"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	return body.Data.References[name].ID
}

// SetProviderConnected controls whether GET /api/v3/provider/connections lists a
// connection for the org and provider. An org with no connection for a provider
// reads as not connected, which is how the GitHub App install check spells "not
// installed".
func (f *CircleCI) SetProviderConnected(orgID, provider string, connected bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	providers := slices.DeleteFunc(f.providerConnections[orgID], func(p string) bool {
		return p == provider
	})
	if connected {
		providers = append(providers, provider)
	}
	f.providerConnections[orgID] = providers
}

// GitHubAppRepo is a stored repository the CircleCI GitHub App can access,
// served by the org repositories endpoint. DefaultBranch renders only when set.
type GitHubAppRepo struct {
	ID            int
	RepoFullName  string
	RepoName      string
	Owner         string
	DefaultBranch string
	Private       bool
}

// AddGitHubAppRepository registers a repository returned by GET
// /api/v2/github-app/organization/{orgID}/repositories.
func (f *CircleCI) AddGitHubAppRepository(orgID string, repo GitHubAppRepo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.githubAppRepos[orgID] = append(f.githubAppRepos[orgID], repo)
}

// gitHubAppRepoEntity renders a stored GitHubAppRepo as its wire object,
// including default_branch only when set.
func gitHubAppRepoEntity(repo GitHubAppRepo) map[string]any {
	m := map[string]any{
		"id":             repo.ID,
		"repo_full_name": repo.RepoFullName,
		"repo_name":      repo.RepoName,
		"owner":          repo.Owner,
		"private":        repo.Private,
	}
	if repo.DefaultBranch != "" {
		m["default_branch"] = repo.DefaultBranch
	}
	return m
}

// SetGitHubAppInstallResponse registers the body returned by POST
// /api/v2/github-app/install.
func (f *CircleCI) SetGitHubAppInstallResponse(resp any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.githubAppInstallResp = resp
}

func (f *CircleCI) handleListProviderConnections(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("filter[org_id]")
	f.mu.RLock()
	providers := slices.Clone(f.providerConnections[orgID])
	f.mu.RUnlock()

	// An org with no connections is an empty collection, not a 404.
	data := make([]any, 0, len(providers))
	for i, p := range providers {
		data = append(data, map[string]any{
			"id": fmt.Sprintf("conn-uuid-%d", i+1),
			"attributes": map[string]any{
				"provider":             p,
				"external_id":          "12345678",
				"login":                "my-org",
				"repository_selection": "all",
			},
		})
	}
	renderV3Collection(w, r, data)
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

	items := make([]any, 0, len(repos))
	for _, repo := range repos {
		items = append(items, gitHubAppRepoEntity(repo))
	}
	render.JSON(w, r, map[string]any{"items": items, "total_count": len(items)})
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
	render.JSON(w, r, projectInfoEntity(info))
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

// RollbackResult is what POST /api/v3/projects/{id}/rollback reports. ID is the
// pipeline run or release-agent command carrying the rollback out and
// RollbackType ("pipeline" or "agent") says which. Set Status to return that
// status with a v3 error envelope instead of the entity, so a test can exercise
// a rejected or conflicting rollback.
type RollbackResult struct {
	ID           string
	RollbackType string
	Status       int
	Title        string
	Detail       string
}

// SetRollback registers the result of POST /api/v3/projects/{id}/rollback. Until
// it is called the route answers 404, matching a project with no rollback.
func (f *CircleCI) SetRollback(result RollbackResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rollback = &result
}

func (f *CircleCI) handleRollbackProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	// Only the ids are read back: they are echoed in the references, and the rest
	// of the body is asserted through the request recorder.
	var body struct {
		ComponentID   string `json:"component_id"`
		EnvironmentID string `json:"environment_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "invalid JSON"})
		return
	}

	f.mu.RLock()
	result := f.rollback
	f.mu.RUnlock()

	if result == nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	if result.Status != 0 {
		title := result.Title
		if title == "" {
			title = http.StatusText(result.Status)
		}
		render.Status(r, result.Status)
		render.JSON(w, r, map[string]any{"error": map[string]any{
			"title":  title,
			"detail": result.Detail,
		}})
		return
	}

	render.JSON(w, r, map[string]any{"data": map[string]any{
		"id":         result.ID,
		"attributes": map[string]any{"rollback_type": result.RollbackType},
		"references": map[string]any{
			"project":            map[string]any{"id": projectID},
			"deploy_component":   map[string]any{"id": body.ComponentID},
			"deploy_environment": map[string]any{"id": body.EnvironmentID},
		},
	}})
}

func (f *CircleCI) handleDeleteEnvVar(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "vcs") + "/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "repo")
	name := chi.URLParam(r, "name")
	key := slug + "/" + name

	f.mu.Lock()
	found := false
	for _, v := range f.envVars[slug] {
		if v.Name == name {
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

// Namespace is a stored namespace served by the REST namespace endpoints.
type Namespace struct {
	ID   string
	Name string
}

// AddNamespace registers a namespace for REST API queries.
// id and name form the namespace record returned by the /api/v3/namespaces endpoints.
func (f *CircleCI) AddNamespace(id, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.namespaces[id] = Namespace{ID: id, Name: name}
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
	render.JSON(w, r, namespaceDataResponse(id, ns.Name))
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
	f.namespaces[id] = Namespace{ID: id, Name: body.Name}
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
		delete(f.namespacesByName, ns.Name)
		f.namespaces[id] = Namespace{ID: id, Name: body.Name}
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

// IOSCert is a stored iOS signing certificate served by the signing
// certificate list endpoint and referenced by signing configs.
type IOSCert struct {
	ID       string
	FileName string
	CertType string
}

// IOSProfile holds the ID and file name of a provisioning profile for the
// remove-profile endpoint
// BundleID/ProfileType mirror the real server's replace-match key (bundle
// identifier + profile type embedded in the file, not the file name); the
// fake reads them from a stand-in content convention instead of a real plist
// parser (see fakeMobileProvisionContent in acceptance/certificate_test.go).
type IOSProfile struct {
	ID          string
	FileName    string
	BundleID    string
	ProfileType string
}

// parseFakeMobileProvisionBlob extracts the bundle ID and profile type
// fakeMobileProvisionContent encoded into a base64-encoded blob. Content that
// doesn't follow that convention yields empty values.
func parseFakeMobileProvisionBlob(blob string) (bundleID, profileType string) {
	decoded, err := base64.StdEncoding.DecodeString(blob)
	if err != nil {
		return "", ""
	}
	for _, field := range strings.Split(string(decoded), ";") {
		k, v, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch k {
		case "bundle-id":
			bundleID = v
		case "profile-type":
			profileType = v
		}
	}
	return bundleID, profileType
}

// IOSSigningConfig is a stored iOS signing config (bundle) served by the
// signing config list endpoint. CertID links it to the IOSCert it uses (the
// cert-in-use check on delete); CertFileName/CertType are the certificate
// reference echoed on the wire.
type IOSSigningConfig struct {
	ID                   string
	Name                 string
	CertID               string
	CertFileName         string
	CertType             string
	ProvisioningProfiles []IOSProfile
}

// AddIOSCert registers an iOS certificate for an org, returned by
// GET /api/v3/signing/certificates?filter[org_id]=<orgID>.
func (f *CircleCI) AddIOSCert(orgID string, cert IOSCert) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.iosCerts[orgID] = append(f.iosCerts[orgID], cert)
}

// AddIOSBundle registers an iOS signing config for an org, returned by
// GET /api/v3/signing/configs?filter[org_id]=<orgID>.
func (f *CircleCI) AddIOSBundle(orgID string, bundle IOSSigningConfig) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.iosBundles[orgID] = append(f.iosBundles[orgID], bundle)
}

// iosCertEntity renders a stored IOSCert as its V3 entity.
func iosCertEntity(c IOSCert) map[string]any {
	return map[string]any{
		"id":         c.ID,
		"attributes": map[string]any{"file_name": c.FileName, "cert_type": c.CertType},
	}
}

// iosSigningConfigEntity renders a stored IOSSigningConfig as its V3 entity,
// with the certificate carried as a reference.
func iosSigningConfigEntity(b IOSSigningConfig) map[string]any {
	profiles := make([]map[string]any, len(b.ProvisioningProfiles))
	for i, p := range b.ProvisioningProfiles {
		profiles[i] = map[string]any{"id": p.ID, "file_name": p.FileName}
	}
	return map[string]any{
		"id": b.ID,
		"attributes": map[string]any{
			"name":                  b.Name,
			"provisioning_profiles": profiles,
		},
		"references": map[string]any{
			"signing_certificate": map[string]any{
				"attributes": map[string]any{"file_name": b.CertFileName, "cert_type": b.CertType},
			},
		},
	}
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
	f.iosCerts[orgID] = append(f.iosCerts[orgID], IOSCert{
		ID:       certID,
		FileName: attrs.FileName,
		CertType: "distribution",
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

	items := make([]any, 0, len(all))
	for _, c := range all {
		if c.ID != "" && deleted[c.ID] {
			continue
		}
		items = append(items, iosCertEntity(c))
	}
	render.JSON(w, r, map[string]any{"data": items})
}

func (f *CircleCI) handleDeleteIOSCert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.Lock()
	found := false
	for _, certs := range f.iosCerts {
		for _, c := range certs {
			if c.ID == id {
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
				if b.CertID != id {
					continue
				}
				if b.ID != "" && !f.deletedIOSBundles[b.ID] {
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
	var cert *IOSCert
	for _, c := range f.iosCerts[orgID] {
		if c.ID != certID || f.deletedIOSCerts[c.ID] {
			continue
		}
		cert = &c
		break
	}
	if cert == nil {
		f.mu.Unlock()
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "certificate not found"})
		return
	}

	// Reject if a live signing config already uses this name in this org.
	for _, b := range f.iosBundles[orgID] {
		if b.Name != name {
			continue
		}
		if b.ID != "" && f.deletedIOSBundles[b.ID] {
			continue
		}
		f.mu.Unlock()
		render.Status(r, http.StatusConflict)
		render.JSON(w, r, map[string]any{"message": "a signing configuration with this name already exists"})
		return
	}

	f.iosBundleCounter++
	id := fmt.Sprintf("10000000-0000-0000-0000-%012d", f.iosBundleCounter)

	// Provisioning-profile list response echoes the id and file_name, not the blob.
	profiles := make([]IOSProfile, len(body.Data.Attributes.ProvisioningProfiles))
	for i, p := range body.Data.Attributes.ProvisioningProfiles {
		f.iosProfileCounter++
		fileName, _ := p["file_name"].(string)
		blob, _ := p["blob"].(string)
		bundleID, profileType := parseFakeMobileProvisionBlob(blob)
		profiles[i] = IOSProfile{
			ID:          fmt.Sprintf("20000000-0000-0000-0000-%012d", f.iosProfileCounter),
			FileName:    fileName,
			BundleID:    bundleID,
			ProfileType: profileType,
		}
	}

	f.iosBundles[orgID] = append(f.iosBundles[orgID], IOSSigningConfig{
		ID:                   id,
		Name:                 name,
		CertID:               certID,
		CertFileName:         cert.FileName,
		CertType:             cert.CertType,
		ProvisioningProfiles: profiles,
	})
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

	items := make([]any, 0, len(all))
	for _, b := range all {
		if b.ID != "" && deleted[b.ID] {
			continue
		}
		items = append(items, iosSigningConfigEntity(b))
	}
	render.JSON(w, r, map[string]any{"data": items})
}

func (f *CircleCI) handleDeleteIOSBundle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.Lock()
	found := false
	for _, bundles := range f.iosBundles {
		for _, b := range bundles {
			if b.ID == id {
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

// findIOSBundle returns the mutable fixture map for the signing config with
// the given id. The returned struct is the same one stored in f.iosBundles
func (f *CircleCI) findIOSBundle(id string) *IOSSigningConfig {
	for i := range f.iosBundles {
		for j := range f.iosBundles[i] {
			if f.iosBundles[i][j].ID == id {
				return &f.iosBundles[i][j]
			}
		}
	}
	return nil
}

func (f *CircleCI) handleUpdateIOSBundleProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Blob     string `json:"blob"`
		FileName string `json:"file_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": err.Error()})
		return
	}
	if body.Blob == "" || body.FileName == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": "missing required fields"})
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	bundle := f.findIOSBundle(id)
	if bundle == nil || f.deletedIOSBundles[id] {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}

	// Same bundle ID + profile type replaces in place, regardless of file name.
	bundleID, profileType := parseFakeMobileProvisionBlob(body.Blob)
	replaced := false
	for i, p := range bundle.ProvisioningProfiles {
		if p.BundleID == bundleID && p.ProfileType == profileType {
			bundle.ProvisioningProfiles[i].FileName = body.FileName
			replaced = true
			break
		}
	}
	if !replaced {
		f.iosProfileCounter++
		bundle.ProvisioningProfiles = append(bundle.ProvisioningProfiles, IOSProfile{
			ID:          fmt.Sprintf("20000000-0000-0000-0000-%012d", f.iosProfileCounter),
			FileName:    body.FileName,
			BundleID:    bundleID,
			ProfileType: profileType,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (f *CircleCI) handleRemoveIOSBundleProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		ProfileID string `json:"profile_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]any{"message": err.Error()})
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	bundle := f.findIOSBundle(id)
	if bundle == nil || f.deletedIOSBundles[id] {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}

	kept := make([]IOSProfile, 0, len(bundle.ProvisioningProfiles))
	for _, p := range bundle.ProvisioningProfiles {
		if p.ID == body.ProfileID {
			continue
		}
		kept = append(kept, p)
	}
	bundle.ProvisioningProfiles = kept

	// Removing an already-absent profile is idempotent: still 204.
	w.WriteHeader(http.StatusNoContent)
}

// --- Orb helpers ---

// AddOrbPackage registers an orb package in the fake server.
func (f *CircleCI) AddOrbPackage(id, nsID, nsName, orbName string, isPrivate, isListed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fullName := nsName + "/" + orbName
	f.orbPackages[id] = Orb{
		ID:        id,
		Name:      fullName,
		NsID:      nsID,
		NsName:    nsName,
		IsPrivate: isPrivate,
		IsListed:  isListed,
		CreatedAt: "2026-01-01T00:00:00.000Z",
	}
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
	f.storeOrbVersionLocked(OrbVersion{
		ID:        id,
		OrbID:     orbID,
		OrbName:   orbName,
		Version:   version,
		Source:    source,
		CreatedAt: createdAt,
	})
}

// AddOrbCategory registers an orb category in the fake server.
func (f *CircleCI) AddOrbCategory(id, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orbCategories[id] = OrbCategory{ID: id, Name: name}
	f.orbCategoriesByName[name] = id
}

// Orb is a stored orb package served by the orb package endpoints. is_listed,
// orb_versions and orb_categories are derived at render time from the unlisted
// set, version list and category membership respectively.
type Orb struct {
	ID        string
	Name      string // full "namespace/orb"
	NsID      string
	NsName    string
	IsPrivate bool
	IsListed  bool
	CreatedAt string
}

// OrbVersion is a stored orb version. Source is served only by the dedicated
// /source endpoint and by the version list — the get/create/promote responses
// strip it.
type OrbVersion struct {
	ID        string
	OrbID     string
	OrbName   string // full "namespace/orb"
	Version   string
	Source    string
	CreatedAt string
}

// OrbCategory is a stored orb category.
type OrbCategory struct {
	ID   string
	Name string
}

// storeOrbVersionLocked records a version and its ref/volatile/order indexes.
// Callers must hold the write lock.
func (f *CircleCI) storeOrbVersionLocked(v OrbVersion) {
	f.orbVersions[v.ID] = v
	f.orbVersionsByRef[v.OrbName+"@"+v.Version] = v.ID
	f.orbVersionsByRef[v.OrbName+"@volatile"] = v.ID
	f.orbVersionsByOrbID[v.OrbID] = append([]string{v.ID}, f.orbVersionsByOrbID[v.OrbID]...)
}

// orbEntity renders a stored Orb as its V3 entity. isListed, cats and latest
// are the render-time derived values; orb_versions and orb_categories are
// omitted when empty, matching the real API.
func orbEntity(o Orb, isListed bool, cats []OrbCategory, latest *OrbVersion) map[string]any {
	refs := map[string]any{
		"namespace": map[string]any{"id": o.NsID, "attributes": map[string]any{"name": o.NsName}},
	}
	if latest != nil {
		refs["orb_versions"] = []any{map[string]any{
			"id":         latest.ID,
			"attributes": map[string]any{"version": latest.Version, "created_at": latest.CreatedAt},
		}}
	}
	if len(cats) > 0 {
		catList := make([]any, 0, len(cats))
		for _, c := range cats {
			catList = append(catList, orbCategoryEntity(c))
		}
		refs["orb_categories"] = catList
	}
	return map[string]any{
		"id": o.ID,
		"attributes": map[string]any{
			"name":                       o.Name,
			"is_private":                 o.IsPrivate,
			"is_listed":                  isListed,
			"created_at":                 o.CreatedAt,
			"last_30_days_build_count":   int64(0),
			"last_30_days_project_count": int64(0),
			"last_30_days_org_count":     int64(0),
		},
		"references": refs,
	}
}

// orbVersionEntity renders a stored OrbVersion. Source is included only when
// requested (the version list serves it; get/create/promote do not).
func orbVersionEntity(v OrbVersion, includeSource bool) map[string]any {
	attrs := map[string]any{"version": v.Version, "created_at": v.CreatedAt}
	if includeSource {
		attrs["source"] = v.Source
	}
	return map[string]any{
		"id":         v.ID,
		"attributes": attrs,
		"references": map[string]any{
			"orb_package": map[string]any{"id": v.OrbID, "attributes": map[string]any{"name": v.OrbName}},
		},
	}
}

// orbCategoryEntity renders a stored OrbCategory.
func orbCategoryEntity(c OrbCategory) map[string]any {
	return map[string]any{"id": c.ID, "attributes": map[string]any{"name": c.Name}}
}

// orbDerivedLocked gathers the render-time derived values for an orb: its listed
// state, attached categories, and latest version. Callers must hold the lock.
func (f *CircleCI) orbDerivedLocked(orbID string) (isListed bool, cats []OrbCategory, latest *OrbVersion) {
	isListed = !f.orbUnlistedPackages[orbID]
	for _, cid := range f.orbCategoryMembers[orbID] {
		if c, ok := f.orbCategories[cid]; ok {
			cats = append(cats, c)
		}
	}
	if ids := f.orbVersionsByOrbID[orbID]; len(ids) > 0 {
		if v, ok := f.orbVersions[ids[0]]; ok {
			latest = &v
		}
	}
	return isListed, cats, latest
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

func (f *CircleCI) handleOrbListPackages(w http.ResponseWriter, r *http.Request) {
	nsID := r.URL.Query().Get("namespace_id")
	nameFilter := r.URL.Query().Get("filter[name]")
	f.mu.RLock()
	defer f.mu.RUnlock()

	items := []any{}
	for _, o := range f.orbPackages {
		if nameFilter != "" && o.Name != nameFilter {
			continue
		}
		if nsID != "" && o.NsID != nsID {
			continue
		}
		isListed, cats, latest := f.orbDerivedLocked(o.ID)
		items = append(items, orbEntity(o, isListed, cats, latest))
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
	defer f.mu.Unlock()
	nsData, ok := f.namespaces[nsID]
	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "namespace not found"})
		return
	}

	id := uuid.New().String()
	o := Orb{
		ID:        id,
		Name:      body.Data.Attributes.Name,
		NsID:      nsID,
		NsName:    nsData.Name,
		IsPrivate: body.Data.Attributes.IsPrivate,
		IsListed:  true,
		CreatedAt: "2026-01-01T00:00:00.000Z",
	}
	f.orbPackages[id] = o
	f.orbPackagesByName[o.Name] = id
	f.orbCreatedPackages = append(f.orbCreatedPackages, o)

	render.Status(r, http.StatusCreated)
	isListed, cats, latest := f.orbDerivedLocked(id)
	render.JSON(w, r, map[string]any{"data": orbEntity(o, isListed, cats, latest)})
}

func (f *CircleCI) handleOrbGetPackage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	defer f.mu.RUnlock()
	o, ok := f.orbPackages[id]
	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	isListed, cats, latest := f.orbDerivedLocked(id)
	render.JSON(w, r, map[string]any{"data": orbEntity(o, isListed, cats, latest)})
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
	defer f.mu.Unlock()
	o, ok := f.orbPackages[id]
	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	if !body.Listed {
		f.orbUnlistedPackages[id] = true
	} else {
		delete(f.orbUnlistedPackages, id)
	}
	isListed, cats, latest := f.orbDerivedLocked(id)
	render.JSON(w, r, map[string]any{"data": orbEntity(o, isListed, cats, latest)})
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
	defer f.mu.Unlock()
	o, ok := f.orbPackages[id]
	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	// Avoid duplicates.
	if !slices.Contains(f.orbCategoryMembers[id], body.CategoryID) {
		f.orbCategoryMembers[id] = append(f.orbCategoryMembers[id], body.CategoryID)
	}
	isListed, cats, latest := f.orbDerivedLocked(id)
	render.JSON(w, r, map[string]any{"data": orbEntity(o, isListed, cats, latest)})
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
	defer f.mu.Unlock()
	o, ok := f.orbPackages[id]
	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	var remaining []string
	for _, cid := range f.orbCategoryMembers[id] {
		if cid != body.CategoryID {
			remaining = append(remaining, cid)
		}
	}
	f.orbCategoryMembers[id] = remaining
	isListed, cats, latest := f.orbDerivedLocked(id)
	render.JSON(w, r, map[string]any{"data": orbEntity(o, isListed, cats, latest)})
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
	defer f.mu.RUnlock()
	versionIDs := f.orbVersionsByOrbID[orbID]

	pageSize := len(versionIDs)
	if pageSizeStr != "" {
		if n, err := fmt.Sscanf(pageSizeStr, "%d", &pageSize); n != 1 || err != nil {
			pageSize = len(versionIDs)
		}
	}

	items := []any{}
	for _, id := range versionIDs {
		if len(items) >= pageSize {
			break
		}
		v, ok := f.orbVersions[id]
		if !ok {
			continue
		}
		isDev := strings.HasPrefix(v.Version, "dev:")
		if channel == "stable" && isDev {
			continue
		}
		if channel == "dev" && !isDev {
			continue
		}
		items = append(items, orbVersionEntity(v, true)) // the list serves source
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
	f.mu.Lock()
	defer f.mu.Unlock()
	pkg, pkgOK := f.orbPackages[orbID]
	if !pkgOK {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "orb not found"})
		return
	}

	v := OrbVersion{
		ID:        uuid.New().String(),
		OrbID:     orbID,
		OrbName:   pkg.Name,
		Version:   body.Data.Attributes.Version,
		Source:    body.Data.Attributes.YAML,
		CreatedAt: "2026-01-15T10:30:00.000Z",
	}
	f.storeOrbVersionLocked(v)
	f.orbCreatedVersions = append(f.orbCreatedVersions, v)

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]any{"data": orbVersionEntity(v, false)})
}

func (f *CircleCI) handleOrbGetVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	v, ok := f.orbVersions[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}
	render.JSON(w, r, map[string]any{"data": orbVersionEntity(v, false)})
}

func (f *CircleCI) handleOrbGetVersionSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	v, ok := f.orbVersions[id]
	f.mu.RUnlock()

	if !ok {
		render.Status(r, http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(v.Source))
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

	f.mu.Lock()
	defer f.mu.Unlock()
	ver, ok := f.orbVersions[id]
	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]any{"message": "not found"})
		return
	}

	// Find the latest stable version to increment from.
	latestStable := "0.0.0"
	for _, vid := range f.orbVersionsByOrbID[ver.OrbID] {
		v, ok := f.orbVersions[vid]
		if !ok || strings.HasPrefix(v.Version, "dev:") {
			continue
		}
		latestStable = v.Version
		break
	}

	newVer := OrbVersion{
		ID:        uuid.New().String(),
		OrbID:     ver.OrbID,
		OrbName:   ver.OrbName,
		Version:   incrementFakeVersion(latestStable, body.Segment),
		Source:    ver.Source,
		CreatedAt: "2026-01-15T10:30:00.000Z",
	}
	f.storeOrbVersionLocked(newVer)

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]any{"data": orbVersionEntity(newVer, false)})
}

func (f *CircleCI) handleOrbListCategories(w http.ResponseWriter, r *http.Request) {
	nameFilter := r.URL.Query().Get("filter[name]")
	f.mu.RLock()
	defer f.mu.RUnlock()

	items := []any{}
	if nameFilter != "" {
		if id, ok := f.orbCategoriesByName[nameFilter]; ok {
			if c, ok := f.orbCategories[id]; ok {
				items = append(items, orbCategoryEntity(c))
			}
		}
	} else {
		for _, c := range f.orbCategories {
			items = append(items, orbCategoryEntity(c))
		}
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
	v, vOK := f.orbVersions[verID]
	f.mu.RUnlock()

	data := []any{}
	if ok && vOK {
		data = append(data, orbVersionEntity(v, true)) // the list serves source
	}
	render.JSON(w, r, map[string]any{
		"data": data,
		"page": map[string]any{"next": nil, "prev": nil},
	})
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

// DecisionLog is a stored policy decision-log entry served by the decision log
// list/get endpoints.
type DecisionLog struct {
	ID     string
	Status string
}

// DecisionResult is the decision returned by the make-decision endpoint.
type DecisionResult struct {
	Status string
}

func (f *CircleCI) AddDecisionLog(ownerID, policyCtx string, log DecisionLog) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := ownerID + "/" + policyCtx
	f.decisionLogs[key] = append(f.decisionLogs[key], log)
}

// SetDecisionResult sets the response returned by MakeDecision for the given owner and context.
func (f *CircleCI) SetDecisionResult(ownerID, policyCtx string, result DecisionResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := ownerID + "/" + policyCtx
	f.decisionResults[key] = result
}

// decisionLogEntity renders a stored DecisionLog as its wire object.
func decisionLogEntity(l DecisionLog) map[string]any {
	return map[string]any{"id": l.ID, "status": l.Status}
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
	tail := all[min(offset, len(all)):]
	items := make([]any, 0, len(tail))
	for _, l := range tail {
		items = append(items, decisionLogEntity(l))
	}
	render.JSON(w, r, items)
}

func (f *CircleCI) handleGetDecisionLog(w http.ResponseWriter, r *http.Request) {
	ownerID := chi.URLParam(r, "ownerID")
	policyCtx := chi.URLParam(r, "policyCtx")
	id := chi.URLParam(r, "id")
	f.mu.RLock()
	all := f.decisionLogs[ownerID+"/"+policyCtx]
	f.mu.RUnlock()
	for _, l := range all {
		if l.ID == id {
			render.JSON(w, r, decisionLogEntity(l))
			return
		}
	}
	http.NotFound(w, r)
}

func (f *CircleCI) handleMakeDecision(w http.ResponseWriter, r *http.Request) {
	ownerID := chi.URLParam(r, "ownerID")
	policyCtx := chi.URLParam(r, "policyCtx")
	f.mu.RLock()
	result, ok := f.decisionResults[ownerID+"/"+policyCtx]
	f.mu.RUnlock()
	if !ok {
		result = DecisionResult{Status: "PASS"}
	}
	render.JSON(w, r, map[string]any{"status": result.Status})
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

// SetCompileResponse configures what the compile route returns. Pass
// valid=false and one or more error messages to simulate a compilation failure.
func (f *CircleCI) SetCompileResponse(valid bool, outputYAML string, errors ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.compileValid = valid
	f.compileOutputYAML = outputYAML
	f.compileErrors = errors
}

// LastCompileOwnerID returns the owning org UUID sent on the most recent compile
// request (empty if none yet). Tests use it to assert that --org resolved to the
// expected organization UUID.
func (f *CircleCI) LastCompileOwnerID() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.lastCompileOwnerID
}

// Org is a stored organization resolved by
// GET /api/v3/orgs?filter[slug]=<slug>. The resolve endpoint surfaces only the
// id; Slug, Name and VCSType round out the record for completeness.
type Org struct {
	ID      string
	Slug    string
	Name    string
	VCSType string
}

// AddOrg registers an org resolvable by slug via GET /api/v3/orgs.
func (f *CircleCI) AddOrg(id, slug, name, vcsType string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orgs[slug] = Org{ID: id, Slug: slug, Name: name, VCSType: vcsType}
	f.orgsByUUID[id] = true
}

// handleCompileConfig serves POST /api/v3/configs/compile. A config that fails
// to compile is still a 200: outcome is "failed" and the reasons ride in
// meta.messages, mirroring the real endpoint.
func (f *CircleCI) handleCompileConfig(w http.ResponseWriter, r *http.Request) {
	// Capture the referenced org so tests can assert that --org (slug or UUID)
	// resolved to the expected organization UUID before the compile call.
	var body struct {
		Data struct {
			References struct {
				Org struct {
					ID string `json:"id"`
				} `json:"org"`
			} `json:"references"`
		} `json:"data"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	f.mu.Lock()
	f.lastCompileOwnerID = body.Data.References.Org.ID
	valid := f.compileValid
	outputYAML := f.compileOutputYAML
	errs := f.compileErrors
	f.mu.Unlock()

	attrs := map[string]any{"phase": "ended", "outcome": "succeeded"}
	if !valid {
		attrs["outcome"] = "failed"
	} else {
		attrs["compiled_config"] = outputYAML
	}

	resp := map[string]any{"data": map[string]any{
		"id":         "00000000-0000-0000-0000-00000000c0de",
		"attributes": attrs,
	}}
	if !valid {
		messages := make([]map[string]any, len(errs))
		for i, e := range errs {
			messages[i] = map[string]any{"title": e}
		}
		resp["meta"] = map[string]any{"messages": messages}
	}

	render.JSON(w, r, resp)
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
		data = append(data, map[string]any{"id": org.ID})
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
