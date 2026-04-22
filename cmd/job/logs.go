package job

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	jobapi "github.com/CircleCI-Public/circleci-cli/api/job"
	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/spf13/cobra"
)

var ansiRe = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func cleanLog(s string) string {
	return strings.ReplaceAll(stripANSI(s), "\r", "")
}

type stepLogJSON struct {
	Step           string `json:"step"`
	ContainerIndex int    `json:"container_index"`
	Output         string `json:"output,omitempty"`
	Error          string `json:"error,omitempty"`
}

type stepInfoJSON struct {
	Step             string `json:"step"`
	AnyFailed        bool   `json:"any_failed"`
	FailedContainers []int  `json:"failed_containers,omitempty"`
}

type jobLogsJSON struct {
	JobNumber int           `json:"job_number"`
	JobName   string        `json:"job_name"`
	Status    string        `json:"status"`
	Steps     []stepLogJSON `json:"steps"`
}

func isNotFound(err error) bool {
	var httpErr *rest.HTTPError
	return errors.As(err, &httpErr) && httpErr.Code == 404
}

func newLogsCommand(jos *jobOpts, preRunE validator.Validator) *cobra.Command {
	var projectSlug string
	var failedOnly bool
	var stepFilter string
	var listSteps bool
	var jsonOut bool
	var tailLines int
	var containerIndex int

	cmd := &cobra.Command{
		Use:   "logs <job-number>",
		Short: "Fetch step logs for a job",
		Long: `Fetch step logs for a job.

Shows log output and error streams for each step in a job. By default shows all steps.
Use --step to filter to a specific step, --failed-only for just failed steps, or --list-steps to see available steps.`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunE(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			jobNumber, err := strconv.Atoi(args[0])
			if err != nil {
				return reportErr(cmd, jsonOut, "invalid_argument", "job-number must be an integer")
			}

			details, err := jos.client.GetJobSteps(projectSlug, jobNumber)
			if err != nil {
				return reportAPIErr(cmd, jsonOut, "Error fetching job steps", err)
			}

			if listSteps {
				infos := collectStepInfos(details.Steps)
				if jsonOut {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(infos)
				}
				for _, info := range infos {
					if info.AnyFailed {
						cmd.Printf("%s [failed]\n", info.Step)
					} else {
						cmd.Println(info.Step)
					}
				}
				return nil
			}

			allStepNames := []string{}
			stepFound := false
			results := []stepLogJSON{}
			for _, step := range details.Steps {
				allStepNames = append(allStepNames, step.Name)
				if stepFilter != "" && step.Name != stepFilter {
					continue
				}
				stepFound = true
				for _, action := range step.Actions {
					if containerIndex >= 0 && action.Index != containerIndex {
						continue
					}
					if failedOnly && (action.Failed == nil || !*action.Failed) {
						continue
					}
					sl := stepLogJSON{Step: step.Name, ContainerIndex: action.Index}
					out, err := jos.client.GetStepLog(projectSlug, jobNumber, action.Index, action.Step, "output")
					if err != nil && !isNotFound(err) {
						return err
					}
					sl.Output = truncateLog(cleanLog(out), tailLines)
					errLog, err := jos.client.GetStepLog(projectSlug, jobNumber, action.Index, action.Step, "error")
					if err != nil && !isNotFound(err) {
						return err
					}
					sl.Error = truncateLog(cleanLog(errLog), tailLines)
					results = append(results, sl)
				}
			}

			if stepFilter != "" && !stepFound {
				msg := fmt.Sprintf("step %q not found; available steps: [%s]", stepFilter, strings.Join(allStepNames, ", "))
				return reportErr(cmd, jsonOut, "step_not_found", msg)
			}

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(jobLogsJSON{
					JobNumber: details.BuildNum,
					JobName:   details.Workflows.JobName,
					Status:    details.Status,
					Steps:     results,
				})
			}

			for _, r := range results {
				if r.ContainerIndex > 0 {
					cmd.Printf("Step: %s [container %d]\n", r.Step, r.ContainerIndex)
				} else {
					cmd.Printf("Step: %s\n", r.Step)
				}
				if r.Output != "" {
					cmd.Printf("  stdout:\n%s\n", indent(r.Output))
				}
				if r.Error != "" {
					cmd.Printf("  stderr:\n%s\n", indent(r.Error))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project-slug", "", "Project slug (e.g. gh/org/repo)")
	_ = cmd.MarkFlagRequired("project-slug")
	cmd.Flags().BoolVar(&failedOnly, "failed-only", false, "Only show logs for failed steps")
	cmd.Flags().StringVar(&stepFilter, "step", "", "Filter to a single step by name")
	cmd.Flags().BoolVar(&listSteps, "list-steps", false, "List available step names and exit")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().IntVar(&tailLines, "tail", 100, "Show the last N lines of each step log (0 = show all)")
	cmd.Flags().IntVar(&containerIndex, "container-index", -1, "Fetch logs for a specific container (0-based); default fetches all")
	cmd.MarkFlagsMutuallyExclusive("list-steps", "step")
	cmd.MarkFlagsMutuallyExclusive("list-steps", "failed-only")
	cmd.MarkFlagsMutuallyExclusive("list-steps", "tail")
	cmd.MarkFlagsMutuallyExclusive("list-steps", "container-index")

	return cmd
}

func truncateLog(s string, n int) string {
	if n == 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return fmt.Sprintf("[truncated — showing last %d of %d lines]\n%s",
		n, len(lines),
		strings.Join(lines[len(lines)-n:], "\n"))
}

func writeJSONError(cmd *cobra.Command, errType, message string) {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]string{"error": errType, "message": message})
}

func reportErr(cmd *cobra.Command, jsonOut bool, errType, msg string) error {
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if jsonOut {
		writeJSONError(cmd, errType, msg)
	} else {
		cmd.PrintErrln("Error: " + msg)
	}
	return errors.New(msg)
}

func reportAPIErr(cmd *cobra.Command, jsonOut bool, humanPrefix string, err error) error {
	if jsonOut {
		writeJSONError(cmd, "api_error", err.Error())
	} else {
		cmd.PrintErrf("%s: %s\n", humanPrefix, err)
	}
	return err
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n")
}

func collectStepInfos(steps []jobapi.JobStep) []stepInfoJSON {
	infos := []stepInfoJSON{}
	seen := map[string]bool{}
	for _, step := range steps {
		if seen[step.Name] {
			continue
		}
		seen[step.Name] = true
		info := stepInfoJSON{Step: step.Name}
		for _, action := range step.Actions {
			if action.Failed != nil && *action.Failed {
				info.AnyFailed = true
				info.FailedContainers = append(info.FailedContainers, action.Index)
			}
		}
		infos = append(infos, info)
	}
	return infos
}
