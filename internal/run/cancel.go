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

// Package run contains business logic for run-level operations.
package run

import (
	"context"
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// ErrNothingToCancel is returned by Cancel when the run exists but all
// of its workflows are already in a terminal state.
type ErrNothingToCancel struct {
	RunID string
}

func (e *ErrNothingToCancel) Error() string {
	return fmt.Sprintf("run %s has no active workflows to cancel", e.RunID)
}

// Cancel cancels a run by cancelling each of its active workflows.
// The CircleCI API has no run-level cancel endpoint; cancellation operates
// at the workflow level via POST /api/v3/workflows/{id}/cancel.
//
// Returns ErrNothingToCancel when the run exists but all workflows are
// already in a terminal state.
func Cancel(ctx context.Context, client *apiclient.Client, runID string) error {
	workflows, err := client.GetRunWorkflowsV3(ctx, runID)
	if err != nil {
		return err
	}

	cancelled := 0
	for _, wf := range workflows {
		if wf.Phase == "ended" {
			continue
		}
		if err := client.CancelWorkflow(ctx, wf.ID); err != nil {
			return err
		}
		cancelled++
	}

	if cancelled == 0 {
		return &ErrNothingToCancel{RunID: runID}
	}
	return nil
}
