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
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// PendingApproval is an approval job that is on hold, waiting to be approved.
type PendingApproval struct {
	WorkflowID   uuid.UUID
	WorkflowName string
	JobID        uuid.UUID
	JobName      string
}

// Label names the approval for prompts and messages, e.g.
// "hold (workflow deploy)". Job names are only unique within a workflow, so the
// workflow is always included.
func (a PendingApproval) Label() string {
	return fmt.Sprintf("%s (workflow %s)", a.JobName, a.WorkflowName)
}

// PendingApprovals returns the approval jobs in a run that are on hold, in
// workflow order. Ended workflows are skipped: nothing in them can be approved.
//
// The V3 jobs API reports an approval job that is waiting for approval as
// phase "started". One whose required jobs have not finished yet is not
// started, and cannot be approved until they have.
func PendingApprovals(ctx context.Context, client *apiclient.Client, runID uuid.UUID) ([]PendingApproval, error) {
	workflows, err := client.GetRunWorkflowsV3(ctx, runID)
	if err != nil {
		return nil, err
	}

	var pending []PendingApproval
	for _, wf := range workflows {
		if wf.Phase == apiclient.PhaseEnded {
			continue
		}
		jobs, err := client.GetWorkflowJobsV3(ctx, wf.ID)
		if err != nil {
			return nil, err
		}
		for _, j := range jobs {
			if j.Type != apiclient.JobTypeApproval || j.Phase != apiclient.PhaseStarted {
				continue
			}
			pending = append(pending, PendingApproval{
				WorkflowID:   wf.ID,
				WorkflowName: wf.Name,
				JobID:        j.ID,
				JobName:      j.Name,
			})
		}
	}
	return pending, nil
}

// MatchApprovals returns the pending approvals that job identifies, by job name
// or job UUID. A name can match more than one approval when two workflows in
// the run have an approval job of the same name.
func MatchApprovals(pending []PendingApproval, job string) []PendingApproval {
	var matches []PendingApproval
	for _, a := range pending {
		if a.JobName == job || a.JobID.String() == job {
			matches = append(matches, a)
		}
	}
	return matches
}

// Approve approves a pending approval job.
func Approve(ctx context.Context, client *apiclient.Client, a PendingApproval) error {
	return client.ApproveJob(ctx, a.WorkflowID, a.JobID)
}
