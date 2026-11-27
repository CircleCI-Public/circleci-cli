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

package api

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestEndpointTemplate(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"api/v2/me", "v2/me"},
		{"/api/v2/me/", "v2/me"},
		{"me", "v3/me"},
		{"api/v1.1/me", "v1.1/me"},
		{"api/v2/me/collaborations", "v2/me/collaborations"},
		{"api/v2/workflow/b9c14502-dae8-4e4e-8bfa-105b1340a286/job", "v2/workflow/{id}/job"},
		{"api/v2/workflow//job", "v2/workflow/{empty}/job"},
		{"api/v2/workflow/null/job", "v2/workflow/{null}/job"},
		{"api/v2/pipeline//workflow", "v2/pipeline/{empty}/workflow"},
		{"api/v2/pipeline/08bebb06-08c5-4848-a5f6-34b3941fbb37/workflow", "v2/pipeline/{id}/workflow"},
		{"api/v2/project/gh/EQTPartners/ventures-funnel/pipeline?branch=main", "v2/project/{slug}/pipeline"},
		{"api/v2/project/github/org/repo/pipeline/run", "v2/project/{slug}/pipeline/run"},
		{"api/v2/insights/gh/org/repo/workflows/ci/jobs", "v2/insights/{slug}/workflows/{id}/jobs"},
		{"api/v1.1/project/gh/org/repo", "v1.1/project/{slug}"},
		{"api/v2/context?owner-slug=gh/org", "v2/context"},
		{"projects/{project-id}/runs", "v3/projects/{id}/runs"},
		{"runs?filter[user_id]=me", "v3/runs"},
		{"api/v3/runs/b9c14502-dae8-4e4e-8bfa-105b1340a286", "v3/runs/{id}"},
		{"api/v2/some-new-endpoint/abc", "v2/{id}/{id}"},
		{"api/me", "me"},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			assert.Check(t, cmp.Equal(endpointTemplate(tc.path), tc.want))
		})
	}
}

// What a user types into a path — org, repo and branch names, IDs — must never
// reach telemetry through the template.
func TestEndpointTemplate_NeverEchoesNames(t *testing.T) {
	secrets := []string{"AcmeCorp", "payments-service", "feature-secret-branch", "0f1e2d3c"}
	paths := []string{
		"api/v2/project/gh/AcmeCorp/payments-service/pipeline?branch=feature-secret-branch",
		"api/v2/insights/gh/AcmeCorp/payments-service/workflows/0f1e2d3c/jobs",
		"api/v1.1/project/bb/AcmeCorp/payments-service/tree/feature-secret-branch",
		"api/v2/workflow/0f1e2d3c/job",
		"api/v2/project/circleci/AcmeCorp/payments-service/envvar",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			got := endpointTemplate(p)
			for _, secret := range secrets {
				assert.Check(t, !strings.Contains(got, secret), "template %q echoes %q", got, secret)
			}
		})
	}
}
