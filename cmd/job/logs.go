package job

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	jobapi "github.com/CircleCI-Public/circleci-cli/api/job"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/slug"
	"github.com/spf13/cobra"
)

type logsStep struct {
	Index   int          `json:"index"`
	Name    string       `json:"name"`
	Actions []logsAction `json:"actions"`
}

type logsAction struct {
	Index  int                 `json:"index"`
	Name   string              `json:"name"`
	Status string              `json:"status,omitempty"`
	Type   string              `json:"type,omitempty"`
	Output []jobapi.StepOutput `json:"output"`
}

type logsResponse struct {
	ProjectSlug string     `json:"project_slug"`
	JobNumber   int        `json:"job_number"`
	Steps       []logsStep `json:"steps"`
}

func newLogsCommand(ops *jobOpts, preRunE validator.Validator) *cobra.Command {
	var stepSelectors []string
	var jsonFormat bool

	cmd := &cobra.Command{
		Use:   "logs <project-slug> <job-number>",
		Short: "Fetch job logs by reading step outputs.",
		Long: `Fetch job logs by reading step outputs.

This command retrieves job step output via the job details endpoint (API v1.1) and
the presigned output URLs provided for each step action.

Examples:
  circleci job logs gh/GetJobber/lakehouse-event-collector 12345
  circleci job logs gh/GetJobber/lakehouse-event-collector 12345 --step "Build Docker image"`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectSlug := args[0]
			jobNumber, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid job-number %q: %w", args[1], err)
			}

			project, err := slug.ParseProject(projectSlug)
			if err != nil {
				return err
			}
			v1vcs, err := project.V1VCS()
			if err != nil {
				return err
			}

			details, err := ops.jobClient.GetJobDetails(v1vcs, project.Org, project.Repo, jobNumber)
			if err != nil {
				return err
			}

			selected, err := selectSteps(details.Steps, stepSelectors)
			if err != nil {
				return err
			}

			if jsonFormat {
				response := logsResponse{
					ProjectSlug: projectSlug,
					JobNumber:   jobNumber,
					Steps:       make([]logsStep, 0, len(selected)),
				}

				for _, step := range selected {
					stepPayload := logsStep{
						Index:   step.Index,
						Name:    step.Step.Name,
						Actions: make([]logsAction, 0, len(step.Step.Actions)),
					}

					for actionIndex, action := range step.Step.Actions {
						if action.OutputURL == "" {
							continue
						}

						output, err := ops.jobClient.GetStepOutput(action.OutputURL)
						if err != nil {
							return err
						}

						stepPayload.Actions = append(stepPayload.Actions, logsAction{
							Index:  actionIndex,
							Name:   action.Name,
							Status: action.Status,
							Type:   action.Type,
							Output: output,
						})
					}

					response.Steps = append(response.Steps, stepPayload)
				}

				payload, err := json.Marshal(response)
				if err != nil {
					return err
				}
				cmd.Println(string(payload))
				return nil
			}

			for _, step := range selected {
				for _, action := range step.Step.Actions {
					if action.OutputURL == "" {
						continue
					}
					output, err := ops.jobClient.GetStepOutput(action.OutputURL)
					if err != nil {
						return err
					}
					for _, line := range output {
						cmd.Print(line.Message)
					}
				}
			}

			return nil
		},
		Args: cobra.ExactArgs(2),
	}

	cmd.Flags().StringArrayVar(&stepSelectors, "step", nil, "Step name or 1-based index (repeatable)")
	cmd.Flags().BoolVar(&jsonFormat, "json", false, "Return output back in JSON format")

	return cmd
}

type selectedStep struct {
	Index int
	Step  jobapi.Step
}

func selectSteps(steps []jobapi.Step, selectors []string) ([]selectedStep, error) {
	if len(selectors) == 0 {
		all := make([]selectedStep, 0, len(steps))
		for i, s := range steps {
			all = append(all, selectedStep{Index: i, Step: s})
		}
		return all, nil
	}

	selected := make([]bool, len(steps))

	for _, rawSelector := range selectors {
		selector := strings.TrimSpace(rawSelector)
		if selector == "" {
			continue
		}

		if index, err := strconv.Atoi(selector); err == nil {
			if index <= 0 {
				return nil, fmt.Errorf("invalid step index %d (expected 1..%d)", index, len(steps))
			}
			zeroIdx := index - 1
			if zeroIdx >= len(steps) {
				return nil, fmt.Errorf("invalid step index %d (expected 1..%d)", index, len(steps))
			}
			selected[zeroIdx] = true
			continue
		}

		matched := false
		for i := range steps {
			if strings.EqualFold(steps[i].Name, selector) {
				selected[i] = true
				matched = true
			}
		}

		if matched {
			continue
		}

		needle := strings.ToLower(selector)
		for i := range steps {
			if strings.Contains(strings.ToLower(steps[i].Name), needle) {
				selected[i] = true
				matched = true
			}
		}

		if !matched {
			return nil, fmt.Errorf("no step matched %q", selector)
		}
	}

	out := make([]selectedStep, 0, len(steps))
	for i, isSelected := range selected {
		if isSelected {
			out = append(out, selectedStep{Index: i, Step: steps[i]})
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no steps selected")
	}

	return out, nil
}
