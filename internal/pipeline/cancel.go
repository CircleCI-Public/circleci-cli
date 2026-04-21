// Package pipeline contains business logic for pipeline-level operations.
package pipeline

import (
	"context"
	"fmt"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
)

// ErrNothingToCancel is returned by Cancel when the pipeline exists but all
// of its workflows are already in a terminal state.
type ErrNothingToCancel struct {
	PipelineID string
}

func (e *ErrNothingToCancel) Error() string {
	return fmt.Sprintf("pipeline %s has no active workflows to cancel", e.PipelineID)
}

// Cancel cancels a pipeline by cancelling each of its active workflows.
// The CircleCI API has no pipeline-level cancel endpoint; cancellation operates
// at the workflow level via POST /api/v2/workflow/{id}/cancel.
//
// Returns ErrNothingToCancel when the pipeline exists but all workflows are
// already in a terminal state.
func Cancel(ctx context.Context, client *apiclient.Client, pipelineID string) error {
	workflows, err := client.GetPipelineWorkflows(ctx, pipelineID)
	if err != nil {
		return err
	}

	active := map[string]bool{
		"running": true, "on_hold": true, "failing": true,
	}

	cancelled := 0
	for _, wf := range workflows {
		if !active[wf.Status] {
			continue
		}
		if err := client.CancelWorkflow(ctx, wf.ID); err != nil {
			return err
		}
		cancelled++
	}

	if cancelled == 0 && len(workflows) > 0 {
		return &ErrNothingToCancel{PipelineID: pipelineID}
	}
	return nil
}
