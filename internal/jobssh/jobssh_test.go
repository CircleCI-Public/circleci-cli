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

package jobssh

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// proxyOutput is the Enable SSH step output of a real Docker job, CRLF line
// endings included.
const proxyOutput = "\r\nEnabling SSH access through ssh.circleci.com...\r\n\r\n" +
	"You can now SSH into this job if your SSH public key is added:\r\n" +
	"    $ ssh b0023961-07d3-4943-8342-a052e5e9307a-0@ssh.circleci.com\r\n\r\n" +
	"Use the same SSH public key that you've added in your User Settings or that you use for your VCS-provider (e.g., GitHub).\r\n\r\n" +
	"Job SSH keys can be added in your User Settings at https://app.circleci.com/settings/user/job-ssh-keys\r\n\r\n" +
	"SSH Session Activity Log (times in UTC):\r\n"

func TestParseOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   Conn
		ok     bool
	}{
		{
			name:   "proxy form",
			output: proxyOutput,
			want: Conn{
				Command: "ssh b0023961-07d3-4943-8342-a052e5e9307a-0@ssh.circleci.com",
				User:    "b0023961-07d3-4943-8342-a052e5e9307a-0",
				Host:    "ssh.circleci.com",
			},
			ok: true,
		},
		{
			name:   "host and port form",
			output: "You can now SSH into this box if your SSH public key is added:\n    $ ssh -p 54782 3.94.12.34\n",
			want:   Conn{Command: "ssh -p 54782 3.94.12.34", Host: "3.94.12.34", Port: 54782},
			ok:     true,
		},
		{
			name:   "not printed yet",
			output: "\r\nEnabling SSH access through ssh.circleci.com...\r\n",
		},
		{
			name:   "ssh mentioned outside a command line",
			output: "Use the same SSH public key, then run ssh to connect\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseOutput([]byte(tt.output))
			assert.Check(t, cmp.Equal(ok, tt.ok))
			assert.Check(t, cmp.DeepEqual(got, tt.want))
		})
	}
}

// TestWait_ParallelStatus checks the status is reported again after a command
// is found while others are still pending. The caller stops its spinner to
// print each command, so without this a parallel job shows nothing between its
// first command and its last.
func TestWait_ParallelStatus(t *testing.T) {
	const (
		workflowID = "11111111-1111-4111-8111-111111111111"
		jobID      = "d0000000-0000-4000-8000-000000000301"
	)
	steps := []fakes.JobStep{{Name: "Enable SSH", Type: StepType, Num: 101, Phase: "started", StartedAt: "2026-11-17T17:11:16.552Z"}}
	job := fakes.JobV3{
		ID: jobID, Name: "audit", Type: "build", Phase: "started",
		StartedAt:  "2026-11-17T17:11:12.621Z",
		Executions: [][]fakes.JobStep{steps, steps},
	}
	commandFor := func(execution int) []byte {
		return fmt.Appendf(nil, "    $ ssh %s-%d@ssh.circleci.com\r\n", jobID, execution)
	}

	fake := fakes.NewCircleCI(t)
	fake.AddWorkflowJobsV3(workflowID, job)
	fake.AddJobV3(job)
	// Both executions have started their SSH step but not printed the command.
	for i := range 2 {
		fake.AddJobStdout(jobID, i, 101, []byte("\r\nEnabling SSH access through ssh.circleci.com...\r\n"))
	}

	client := apiclient.New(apiclient.Config{BaseURL: fake.URL(), Token: "test-token"})
	var events []string
	released := 0
	conns, err := Wait(iostream.Testing(t.Context()), client, uuid.MustParse(workflowID), []string{"audit"}, Options{
		Timeout:  2 * time.Second,
		Interval: 10 * time.Millisecond,
		OnStatus: func(s string) {
			events = append(events, "status: "+s)
			// Each status report lets the next execution print its command.
			// Execution 1's comes only once the unchanged status is reported a
			// second time, so without that the wait times out.
			if released < 2 {
				fake.AddJobStdout(jobID, released, 101, commandFor(released))
				released++
			}
		},
		OnConn: func(c Conn) { events = append(events, fmt.Sprintf("conn: %d", c.Execution)) },
	})

	assert.NilError(t, err)
	assert.Check(t, cmp.Len(conns, 2))
	assert.Check(t, cmp.DeepEqual(events, []string{
		"status: Waiting for the SSH step of audit",
		"conn: 0",
		"status: Waiting for the SSH step of audit",
		"conn: 1",
	}))
}
