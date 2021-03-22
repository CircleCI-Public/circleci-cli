package pipeline

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/pipelines"
	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/git"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

func NewCommand(config *settings.Config, preRunE validator) *cobra.Command {
	p := pipelines.New(rest.New(config.Host, config.RestEndpoint, config.Token))
	cmd := &cobra.Command{
		Use:     "pipeline",
		Short:   "Operate on pipelines",
		PreRunE: preRunE,
	}

	cmd.AddCommand(&cobra.Command{
		Use:     "trigger",
		Short:   "Trigger a pipeline for the current project",
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			remote, err := git.InferProjectFromGitRemotes()
			if err != nil {
				return errors.Wrap(err, "this command must be run from inside a git repository")
			}

			fmt.Printf("Triggering pipeline for: VCS=%q organization=%q project=%q\n", remote.VcsType, remote.Organization, remote.Project)

			pipe, err := p.Trigger(*remote, &pipelines.TriggerParameters{})
			if err != nil {
				return err
			}

			table := newPipelineTable()
			defer table.Render()
			appendPipeline(table, *pipe)

			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all pipelines for the current project",
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			remote, err := git.InferProjectFromGitRemotes()
			if err != nil {
				return errors.Wrap(err, "this command must be run from inside a git repository")
			}

			pipes, err := p.Get(*remote)
			if err != nil {
				return err
			}

			table := newPipelineTable()
			defer table.Render()
			for _, pipe := range pipes {
				appendPipeline(table, pipe)
			}

			return nil
		},
	})

	return cmd
}

func newPipelineTable() *tablewriter.Table {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{
		"ID",
		"Number",
		"Created At",
		"Updated At",
		"State",
		"Trigger Type",
		"Actor Login",
	})
	return table
}

func appendPipeline(table *tablewriter.Table, pipe pipelines.Pipeline) {
	table.Append([]string{
		pipe.ID,
		strconv.Itoa(pipe.Number),
		pipe.CreatedAt.Format(time.RFC3339),
		pipe.UpdatedAt.Format(time.RFC3339),
		pipe.State,
		pipe.Trigger.Type,
		pipe.Trigger.Actor.Login,
	})
}

type validator func(cmd *cobra.Command, args []string) error
