package runner

import (
	"encoding/json"
	"io"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/runner"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
)

func newResourceClassCommand(o *runnerOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource-class",
		Short: "Operate on runner resource-classes",
	}

	telemetryWrappedPreRunE := func(cmd *cobra.Command, args []string) error {
		telemetryClient, ok := telemetry.FromContext(cmd.Context())
		if ok {
			_ = telemetryClient.Track(telemetry.CreateRunnerResourceClassEvent(telemetry.GetCommandInformation(cmd, true)))
		}
		if preRunE != nil {
			return preRunE(cmd, args)
		}
		return nil
	}

	jsonFormat := false
	genToken := false
	createCmd := &cobra.Command{
		Use:     "create <resource-class> <description>",
		Short:   "Create a resource-class",
		Args:    cobra.ExactArgs(2),
		PreRunE: telemetryWrappedPreRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			cmd.PrintErr(terms)

			rc, err := o.r.CreateResourceClass(args[0], args[1])
			if err != nil {
				return err
			}

			var token *runner.Token
			if genToken {
				token, err = o.r.CreateToken(args[0], "default")
				if err != nil {
					return err
				}
			}

			if jsonFormat && !genToken {
				// return JSON formatted output for resource-class (without generated token)
				jsonRc, err := json.Marshal(rc)
				if err != nil {
					return err
				}
				jsonWriter := cmd.OutOrStdout()
				if _, err := jsonWriter.Write(jsonRc); err != nil {
					return err
				}
			} else if jsonFormat && genToken {
				// return JSON formatted output for token since it contains enough related resource-class info
				jsonToken, err := json.Marshal(token)
				if err != nil {
					return err
				}
				jsonWriter := cmd.OutOrStdout()
				if _, err := jsonWriter.Write(jsonToken); err != nil {
					return err
				}
			} else {
				// return default ASCII table format for output
				table := newResourceClassTable(cmd.OutOrStdout())
				defer table.Render()
				appendResourceClass(table, *rc)

				// check to conditionally return YAML formatted resource-class token
				if genToken {
					return generateConfig(*token, cmd.OutOrStdout())
				}
			}

			return nil
		},
	}
	createCmd.PersistentFlags().BoolVar(&jsonFormat, "json", false,
		"Return output back in JSON format")
	createCmd.PersistentFlags().BoolVar(&genToken, "generate-token", false,
		"Generate a default token")
	cmd.AddCommand(createCmd)

	forceDelete := false
	deleteCmd := &cobra.Command{
		Use:     "delete <resource-class>",
		Short:   "Delete a resource-class",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		PreRunE: telemetryWrappedPreRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			rc, err := o.r.GetResourceClassByName(args[0])
			if err != nil {
				return err
			}
			return o.r.DeleteResourceClass(rc.ID, forceDelete)
		},
	}
	deleteCmd.PersistentFlags().BoolVarP(&forceDelete, "force", "f", false,
		"Delete resource-class and any associated tokens")
	cmd.AddCommand(deleteCmd)

	listCmd := &cobra.Command{
		Use:     "list <namespace>",
		Short:   "List resource-classes for a namespace",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(1),
		PreRunE: telemetryWrappedPreRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			rcs, err := o.r.GetResourceClassesByNamespace(args[0])
			if err != nil {
				return err
			}

			if jsonFormat {
				// return JSON formatted for output
				jsonRcs, err := json.Marshal(rcs)
				if err != nil {
					return err
				}
				jsonWriter := cmd.OutOrStdout()
				if _, err := jsonWriter.Write(jsonRcs); err != nil {
					return err
				}
			} else {
				// return default ASCII table format for output
				table := newResourceClassTable(cmd.OutOrStdout())
				defer table.Render()
				for _, rc := range rcs {
					appendResourceClass(table, rc)
				}
			}

			return nil
		},
	}
	listCmd.PersistentFlags().BoolVar(&jsonFormat, "json", false,
		"Return output back in JSON format")
	cmd.AddCommand(listCmd)

	return cmd
}

func newResourceClassTable(writer io.Writer) *tablewriter.Table {
	table := tablewriter.NewWriter(writer)
	table.SetHeader([]string{"Resource Class", "Description"})
	return table
}

func appendResourceClass(table *tablewriter.Table, rc runner.ResourceClass) {
	table.Append([]string{rc.ResourceClass, rc.Description})
}

const terms = "If you have not already agreed to Runner Terms in a signed Order, " +
	"then by continuing to install Runner, " +
	"you are agreeing to CircleCI's Runner Terms which are found at: https://circleci.com/legal/runner-terms/.\n" +
	"If you already agreed to Runner Terms in a signed Order, " +
	"the Runner Terms in the signed Order supersede the Runner Terms in the web address above.\n" +
	"If you did not already agree to Runner Terms through a signed Order and do not agree to the Runner Terms in the web address above, " +
	"please do not install or use Runner.\n\n"
