package job

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/slug"
	"github.com/spf13/cobra"
)

const defaultImageTagRegex = `(?m)\b([a-zA-Z0-9][a-zA-Z0-9./_-]*:[a-zA-Z0-9][a-zA-Z0-9._-]*)\b`

func newImageTagCommand(ops *jobOpts, preRunE validator.Validator) *cobra.Command {
	var stepSelectors []string
	var regexStr string

	cmd := &cobra.Command{
		Use:   "image-tag <project-slug> <job-number>",
		Short: "Extract image tag(s) from job step output.",
		Long: `Extract image tag(s) from job step output.

By default, this searches the "Build Docker image" and "Push Docker images" steps.
You can use --step to override step selection and --regex to customize extraction.

Examples:
  circleci job image-tag gh/GetJobber/lakehouse-event-collector 12345
  circleci job image-tag gh/GetJobber/lakehouse-event-collector 12345 --step "Build Docker image"
  circleci job image-tag gh/GetJobber/lakehouse-event-collector 12345 --regex 'IMAGE_TAG=([^\n]+)'`,
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

			selectors := stepSelectors
			if len(selectors) == 0 {
				selectors = []string{"Build Docker image", "Push Docker images"}
			}

			selected, err := selectSteps(details.Steps, selectors)
			if err != nil {
				return err
			}

			var logText strings.Builder
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
						logText.WriteString(line.Message)
					}
				}
			}

			pattern := regexStr
			if pattern == "" {
				pattern = defaultImageTagRegex
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return err
			}

			matches := re.FindAllStringSubmatch(logText.String(), -1)
			if len(matches) == 0 {
				return fmt.Errorf("no image tags matched regex")
			}

			seen := map[string]struct{}{}
			out := make([]string, 0, len(matches))
			for _, m := range matches {
				value := firstNonEmpty(m[1:])
				if value == "" {
					value = m[0]
				}
				value = strings.TrimSpace(value)
				if value == "" {
					continue
				}
				if _, ok := seen[value]; ok {
					continue
				}
				seen[value] = struct{}{}
				out = append(out, value)
			}

			if len(out) == 0 {
				return fmt.Errorf("no image tags found")
			}

			for _, v := range out {
				cmd.Println(v)
			}

			return nil
		},
		Args: cobra.ExactArgs(2),
	}

	cmd.Flags().StringArrayVar(&stepSelectors, "step", nil, "Step name or 1-based index (repeatable)")
	cmd.Flags().StringVar(&regexStr, "regex", "", "Regex used to extract image tags (first capturing group is preferred)")

	return cmd
}

func firstNonEmpty(values []string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
