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

package run

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// TestFailureReportNextSteps verifies the report's closing section lists what
// fits the failures it found, and caps the per-job and per-workflow lines.
func TestFailureReportNextSteps(t *testing.T) {
	runID := uuid.MustParse("e0000000-0000-4000-8000-0000000000f1")
	wfID := uuid.MustParse("b0000000-0000-4000-8000-0000000000f1")
	jobID := uuid.MustParse("d0000000-0000-4000-8000-0000000000f1")
	run := &apiclient.RunV3{ID: runID}

	t.Run("failed steps with failed tests", func(t *testing.T) {
		got := failureReportNextSteps(run,
			[]apiclient.WorkflowV3{{ID: wfID, Name: "build"}},
			[]reportJob{
				{name: "run-tests", id: jobID, hasFailedTest: true},
				{name: "lint", id: uuid.MustParse("d0000000-0000-4000-8000-0000000000f2")},
			})

		assert.Check(t, is.Equal(got, "## next steps\n\n"+
			`- Failed tests in job "run-tests": circleci testresult list `+jobID.String()+"\n"+
			`- If "build" failed transiently (network timeout, lost executor), rerun its failed jobs: circleci workflow rerun `+wfID.String()+" --from-failed\n"+
			`- Push the fix and watch it: git push && circleci run watch --sha "$(git rev-parse HEAD)"`+"\n"))
	})

	t.Run("no failed tests leaves out the testresult line", func(t *testing.T) {
		got := failureReportNextSteps(run,
			[]apiclient.WorkflowV3{{ID: wfID, Name: "build"}},
			[]reportJob{{name: "lint", id: jobID}})

		assert.Check(t, !strings.Contains(got, "testresult"), got)
	})

	t.Run("config error alone gets validate and push only", func(t *testing.T) {
		got := failureReportNextSteps(&apiclient.RunV3{ID: runID, Errors: []apiclient.RunError{{Type: "config", Message: "bad"}}}, nil, nil)

		assert.Check(t, is.Equal(got, "## next steps\n\n"+
			"- Validate the config locally: circleci config validate .circleci/config.yml\n"+
			`- Push the fix and watch it: git push && circleci run watch --sha "$(git rev-parse HEAD)"`+"\n"))
	})

	t.Run("lists the first three jobs and workflows and counts the rest", func(t *testing.T) {
		names := []string{"a", "b", "c", "d", "e"}
		jobs := make([]reportJob, 0, len(names))
		workflows := make([]apiclient.WorkflowV3, 0, len(names))
		for _, name := range names {
			jobs = append(jobs, reportJob{name: name, id: jobID, hasFailedTest: true})
			workflows = append(workflows, apiclient.WorkflowV3{ID: wfID, Name: name})
		}

		got := failureReportNextSteps(run, workflows, jobs)

		assert.Check(t, is.Equal(strings.Count(got, "Failed tests in job"), 3))
		assert.Check(t, is.Equal(strings.Count(got, "failed transiently"), 3))
		assert.Check(t, is.Contains(got, "- 2 more jobs with failed tests; see all with: circleci run get "+runID.String()+"\n"))
		assert.Check(t, is.Contains(got, "- 2 more failed workflows; see all with: circleci run get "+runID.String()+"\n"))
	})
	t.Run("counts only jobs with failed tests past the cap", func(t *testing.T) {
		// 6 failed jobs, 4 with failed tests: 3 listed, 1 more with failed tests.
		// The 2 without failed tests are not part of the count.
		jobs := make([]reportJob, 0, 6)
		for i, name := range []string{"a", "b", "c", "d", "e", "f"} {
			jobs = append(jobs, reportJob{name: name, id: jobID, hasFailedTest: i < 4})
		}

		got := failureReportNextSteps(run, nil, jobs)

		assert.Check(t, is.Equal(strings.Count(got, "Failed tests in job"), 3))
		assert.Check(t, is.Contains(got, "- 1 more job with failed tests; see all with: circleci run get "+runID.String()+"\n"))
	})
}

// TestMarkFailedTests verifies which jobs get the failed-tests hint, and that a
// failing or hanging tests endpoint only drops the hint, within the budget.
func TestMarkFailedTests(t *testing.T) {
	failing := uuid.MustParse("d0000000-0000-4000-8000-0000000000a1")
	passing := uuid.MustParse("d0000000-0000-4000-8000-0000000000a2")
	erroring := uuid.MustParse("d0000000-0000-4000-8000-0000000000a3")
	hanging := uuid.MustParse("d0000000-0000-4000-8000-0000000000a4")
	slow := uuid.MustParse("d0000000-0000-4000-8000-0000000000a5")

	const slowDelay = 200 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/jobs/" + failing.String() + "/tests":
			_, _ = w.Write([]byte(`{"name":"a","result":"success"}` + "\n" + `{"name":"b","result":"failure"}` + "\n"))
		case "/api/v3/jobs/" + passing.String() + "/tests":
			_, _ = w.Write([]byte(`{"name":"a","result":"success"}` + "\n" + `{"name":"b","result":"skipped"}` + "\n"))
		case "/api/v3/jobs/" + erroring.String() + "/tests":
			w.WriteHeader(http.StatusForbidden)
		case "/api/v3/jobs/" + hanging.String() + "/tests":
			<-r.Context().Done()
		case "/api/v3/jobs/" + slow.String() + "/tests":
			time.Sleep(slowDelay)
			_, _ = w.Write([]byte(`{"name":"a","result":"failure"}` + "\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	client := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "testtoken"})
	ctx := iostream.Testing(t.Context())

	t.Run("marks only jobs with a failed test", func(t *testing.T) {
		jobs := []reportJob{{id: failing}, {id: passing}, {id: erroring}}

		markFailedTests(ctx, client, jobs)

		assert.Check(t, jobs[0].hasFailedTest, "job with a failure")
		assert.Check(t, !jobs[1].hasFailedTest, "job with no failure")
		assert.Check(t, !jobs[2].hasFailedTest, "job whose tests endpoint errors")
	})

	t.Run("reads concurrently", func(t *testing.T) {
		jobs := make([]reportJob, 5)
		for i := range jobs {
			jobs[i] = reportJob{id: slow}
		}

		start := time.Now()
		markFailedTests(ctx, client, jobs)
		elapsed := time.Since(start)

		// One at a time takes at least 5*slowDelay, so anything under it shows
		// the reads overlapped, with room for a slow CI machine.
		assert.Check(t, elapsed < 5*slowDelay, "5 reads took %s; one at a time takes at least %s", elapsed, 5*slowDelay)
		for i, j := range jobs {
			assert.Check(t, j.hasFailedTest, "job %d", i)
		}
	})

	t.Run("a hanging endpoint is cut off at the budget", func(t *testing.T) {
		budget := testReadBudget
		testReadBudget = 300 * time.Millisecond
		t.Cleanup(func() { testReadBudget = budget })
		jobs := []reportJob{{id: hanging}, {id: failing}}

		start := time.Now()
		markFailedTests(ctx, client, jobs)
		elapsed := time.Since(start)

		// Without the budget the hanging read would run to httpcl's 30s timeout.
		assert.Check(t, elapsed < 10*testReadBudget, "took %s with a %s budget", elapsed, testReadBudget)
		assert.Check(t, !jobs[0].hasFailedTest, "hanging job")
		assert.Check(t, jobs[1].hasFailedTest, "job read before the budget ran out")
	})
}

// TestWriteNextSteps verifies the section is only added to a report that found
// run errors or failed steps, and that a report without failed jobs reads no
// tests.
func TestWriteNextSteps(t *testing.T) {
	var testsReads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testsReads.Add(1)
	}))
	t.Cleanup(srv.Close)
	client := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "testtoken"})
	ctx := iostream.Testing(t.Context())
	runID := uuid.MustParse("e0000000-0000-4000-8000-0000000000f1")

	t.Run("nothing failed writes nothing", func(t *testing.T) {
		testsReads.Store(0)
		var out strings.Builder

		writeNextSteps(ctx, client, &out, &apiclient.RunV3{ID: runID}, nil, nil)

		assert.Check(t, is.Equal(out.String(), ""))
		assert.Check(t, is.Equal(testsReads.Load(), int32(0)))
	})

	t.Run("warnings alone write nothing", func(t *testing.T) {
		testsReads.Store(0)
		var out strings.Builder
		run := &apiclient.RunV3{ID: runID, Warnings: []apiclient.RunWarning{{Message: "deprecated image"}}}

		writeNextSteps(ctx, client, &out, run, nil, nil)

		assert.Check(t, is.Equal(out.String(), ""))
		assert.Check(t, is.Equal(testsReads.Load(), int32(0)))
	})

	t.Run("a run error alone writes the section without reading tests", func(t *testing.T) {
		testsReads.Store(0)
		var out strings.Builder
		run := &apiclient.RunV3{ID: runID, Errors: []apiclient.RunError{{Type: "config", Message: "bad"}}}

		writeNextSteps(ctx, client, &out, run, nil, nil)

		assert.Check(t, is.Contains(out.String(), "## next steps\n"))
		assert.Check(t, is.Equal(testsReads.Load(), int32(0)))
	})
}
