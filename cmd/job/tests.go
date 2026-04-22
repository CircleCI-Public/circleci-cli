package job

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/spf13/cobra"
)

var ErrTestsFailed = errors.New("tests failed")

type testResultJSON struct {
	Name      string  `json:"name"`
	Classname string  `json:"classname"`
	Result    string  `json:"result"`
	Message   string  `json:"message,omitempty"`
	RunTime   float64 `json:"run_time"`
}

type testSummaryJSON struct {
	Total   int              `json:"total"`
	Failed  int              `json:"failed"`
	Results []testResultJSON `json:"results"`
}

func newTestsCommand(jos *jobOpts, preRunE validator.Validator) *cobra.Command {
	var projectSlug string
	var showAll bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "tests <job-number>",
		Short: "Fetch test results for a job",
		Long: `Fetch test results for a job.

By default, only failed and errored tests are listed. Use --all to include passing tests.`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunE(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			jobNumber, err := strconv.Atoi(args[0])
			if err != nil {
				return reportErr(cmd, jsonOut, "invalid_argument", "job-number must be an integer")
			}

			results, err := jos.client.GetTestResults(projectSlug, jobNumber)
			if err != nil {
				return reportAPIErr(cmd, jsonOut, "Error fetching test results", err)
			}

			failures := 0
			for _, r := range results {
				if r.Result == "failure" || r.Result == "error" {
					failures++
				}
			}

			out := []testResultJSON{}
			for _, r := range results {
				if !showAll && r.Result != "failure" && r.Result != "error" {
					continue
				}
				out = append(out, testResultJSON{
					Name:      r.Name,
					Classname: r.Classname,
					Result:    r.Result,
					Message:   r.Message,
					RunTime:   r.RunTime,
				})
			}

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(testSummaryJSON{
					Total:   len(results),
					Failed:  failures,
					Results: out,
				}); err != nil {
					return err
				}
				if failures > 0 {
					cmd.SilenceErrors = true
					cmd.SilenceUsage = true
					return ErrTestsFailed
				}
				return nil
			}

			if len(results) == 0 {
				cmd.Println("No test results found for this job.")
				return nil
			}

			for _, r := range out {
				status := r.Result
				if r.Message != "" {
					cmd.Printf("  %s %s (%s): %s\n", status, r.Name, r.Classname, r.Message)
				} else {
					cmd.Printf("  %s %s (%s)\n", status, r.Name, r.Classname)
				}
			}
			passed := len(results) - failures
			cmd.Printf("%d passed, %d failed (total %d).\n", passed, failures, len(results))
			if failures > 0 {
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				return ErrTestsFailed
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project-slug", "", "Project slug (e.g. gh/org/repo)")
	_ = cmd.MarkFlagRequired("project-slug")
	cmd.Flags().BoolVar(&showAll, "all", false, "Include passing tests in output (default: only failed/errored)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}
