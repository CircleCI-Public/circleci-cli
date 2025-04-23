package runner

import (
	"encoding/json"
	"io"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/runner"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
)

func newRunnerInstanceCommand(o *runnerOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instance",
		Short: "Operate on runner instances",
	}

	jsonFormat := false

	cmd.AddCommand(&cobra.Command{
		Use:   "list <namespace or resource-class>",
		Short: "List runner instances",
		Example: `  circleci runner instance ls my-namespace
  circleci runner instance ls my-namespace/my-resource-class`,
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(1),
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			telemetryClient, ok := telemetry.FromContext(cmd.Context())
			if ok {
				// We defer the call to be sure the `err` has been filled
				defer (func() {
					_ = telemetryClient.Track(telemetry.CreateRunnerInstanceEvent(telemetry.GetCommandInformation(cmd, true), err))
				})()
			}

			runners, err := o.r.GetRunnerInstances(args[0])
			if err != nil {
				return err
			}

			if jsonFormat {
				// return JSON formatted for output
				jsonRunners, err := json.Marshal(runners)
				if err != nil {
					return err
				}
				jsonWriter := cmd.OutOrStdout()
				if _, err := jsonWriter.Write(jsonRunners); err != nil {
					return err
				}
			} else {
				table := newRunnerInstanceTable(cmd.OutOrStdout())
				defer table.Render()
				for _, r := range runners {
					appendRunnerInstance(table, r)
				}
			}

			return nil
		},
	})

	cmd.PersistentFlags().BoolVar(&jsonFormat, "json", false,
		"Return output back in JSON format")

	return cmd
}

func newRunnerInstanceTable(writer io.Writer) *tablewriter.Table {
	table := tablewriter.NewWriter(writer)
	table.SetHeader([]string{
		"Name",
		"Resource Class",
		"Hostname",
		"First Connected",
		"Last Connected",
		"Last Used",
		"IP",
		"Version",
	})
	return table
}

func appendRunnerInstance(table *tablewriter.Table, r runner.RunnerInstance) {
	table.Append([]string{
		r.Name,
		r.ResourceClass,
		r.Hostname,
		formatOptionalTime(r.FirstConnected),
		formatOptionalTime(r.LastConnected),
		formatOptionalTime(r.LastUsed),
		r.IP,
		r.Version,
	})
}

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
