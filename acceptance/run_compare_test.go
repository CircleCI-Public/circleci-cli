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

package acceptance_test

import (
	"encoding/json"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	compareOldRunID = "11111111-0000-4000-8000-000000000001"
	compareNewRunID = "22222222-0000-4000-8000-000000000002"
)

// compareWorkflow is an ended workflow of runID, created and ended at the given
// offsets from noon on 2020-01-01.
func compareWorkflow(id, name, runID string, created, ended time.Duration) fakes.WorkflowV3 {
	wf := fakeWorkflowV3(id, name, runID, runTestProjectID, "ended", "succeeded")
	start := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	wf.CreatedAt = start.Add(created).Format(v3TimeFormat)
	wf.EndedAt = start.Add(ended).Format(v3TimeFormat)
	return wf
}

// newCompareFake serves two ended runs: the old one ran two workflows over
// 11m2s for 300 credits, the new one a single workflow over 8m40s for 330.
func newCompareFake(t *testing.T) *fakes.CircleCI {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	fake.AddRunV3(compareOldRunID, runTestProjectID, fakeRunV3(compareOldRunID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(compareOldRunID,
		compareWorkflow("b1000000-0000-4000-8000-000000000001", "build", compareOldRunID, 0, 6*time.Minute),
		compareWorkflow("b1000000-0000-4000-8000-000000000002", "deploy", compareOldRunID, 2*time.Second, 11*time.Minute+2*time.Second),
	)
	fake.AddRunCharges(compareOldRunID,
		fakes.RunCharge{ProjectID: runTestProjectID, Workflow: "build", Credits: 200},
		fakes.RunCharge{ProjectID: runTestProjectID, Workflow: "deploy", Credits: 100},
	)

	fake.AddRunV3(compareNewRunID, runTestProjectID, fakeRunV3(compareNewRunID, runTestProjectID, "ended", "succeeded", "my-change", "def5678abc1234"))
	fake.AddRunWorkflowsV3(compareNewRunID,
		compareWorkflow("b2000000-0000-4000-8000-000000000001", "build-and-deploy", compareNewRunID, 0, 8*time.Minute+40*time.Second),
	)
	return fake
}

func runCompare(t *testing.T, fake *fakes.CircleCI, args ...string) binary.CLIResult {
	t.Helper()
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"run", "compare"}, args...),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
}

func TestRunCompare(t *testing.T) {
	fake := newCompareFake(t)
	fake.AddRunCharges(compareNewRunID, fakes.RunCharge{ProjectID: runTestProjectID, Workflow: "build-and-deploy", Credits: 330})

	result := runCompare(t, fake, compareOldRunID, compareNewRunID)

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunCompare_JSON(t *testing.T) {
	fake := newCompareFake(t)
	fake.AddRunCharges(compareNewRunID, fakes.RunCharge{ProjectID: runTestProjectID, Workflow: "build-and-deploy", Credits: 330})

	result := runCompare(t, fake, "--json", compareOldRunID, compareNewRunID)

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, map[string]any{
		"old_run": map[string]any{
			"id":              compareOldRunID,
			"project_id":      runTestProjectID,
			"running_seconds": float64(662),
			"credits":         float64(300),
		},
		"new_run": map[string]any{
			"id":              compareNewRunID,
			"project_id":      runTestProjectID,
			"running_seconds": float64(520),
			"credits":         float64(330),
		},
		"delta": map[string]any{
			"running_seconds": float64(-142),
			"running_pct":     -21.5,
			"credits":         float64(30),
			"credits_pct":     float64(10),
		},
	}))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

func TestRunCompare_NoCreditsYet(t *testing.T) {
	// The new run has ended but none of its charges have been recorded.
	fake := newCompareFake(t)

	result := runCompare(t, fake, compareOldRunID, compareNewRunID)

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunCompare_Errors(t *testing.T) {
	const unknownRunID = "33333333-0000-4000-8000-000000000003"

	tests := []struct {
		name     string
		args     []string
		setup    func(*fakes.CircleCI)
		exitCode int
	}{
		{
			name:     "missing arguments",
			args:     []string{compareOldRunID},
			exitCode: 2,
		},
		{
			name:     "invalid run id",
			args:     []string{"not-a-uuid", compareNewRunID},
			exitCode: 2,
		},
		{
			name:     "same run twice",
			args:     []string{compareOldRunID, compareOldRunID},
			exitCode: 2,
		},
		{
			name: "run not ended",
			args: []string{compareOldRunID, unknownRunID},
			setup: func(f *fakes.CircleCI) {
				f.AddRunV3(unknownRunID, runTestProjectID, fakeRunV3(unknownRunID, runTestProjectID, "started", "", "my-change", "def5678abc1234"))
			},
			exitCode: 2,
		},
		{
			name:     "unknown run",
			args:     []string{compareOldRunID, unknownRunID},
			exitCode: 5,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := newCompareFake(t)
			if tc.setup != nil {
				tc.setup(fake)
			}

			result := runCompare(t, fake, tc.args...)

			assert.Check(t, cmp.Equal(result.ExitCode, tc.exitCode))
			assert.Check(t, cmp.Equal(result.Stdout, ""))
			assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
		})
	}
}
