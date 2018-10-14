package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/proxy"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/bmatcuk/doublestar"
	"github.com/spf13/cobra"
)

type testsOptions struct {
	*settings.Config
	args []string
}

func newTestsCommand(config *settings.Config) *cobra.Command {
	opts := testsOptions{
		Config: config,
	}

	testsCmd := &cobra.Command{
		Use:    "tests",
		Short:  "Collect and split files with tests",
		Hidden: true,
	}

	globCmd := &cobra.Command{
		Use:   "glob",
		Short: "Glob files using pattern",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return globRun(opts)
		},
		Hidden: true,
	}

	splitCmd := &cobra.Command{
		Use:   "split",
		Short: "Return a split batch of provided files",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return splitRunE(opts)
		},
		Hidden: true,
	}
	splitCmd.Flags().Uint("index", 0, "index of node.")
	splitCmd.Flags().Uint("total", 1, "number of nodes.")
	splitCmd.Flags().String("split-by", "name", `how to weight the split, allowed values are "name", "filesize", and "timings".`)
	splitCmd.Flags().String("timings-type", "filename", `lookup historical timing data by: "classname", "filename", or "testname".`)
	splitCmd.Flags().Bool("show-counts", false, `print test file or test class counts to stderr (default false).`)
	splitCmd.Flags().String("timings-file", "", "JSON file containing historical timing data.")

	testsCmd.AddCommand(globCmd)
	testsCmd.AddCommand(splitCmd)

	return testsCmd
}

func expandGlobs(opts testsOptions) ([]string, error) {
	result := []string{}

	for _, arg := range opts.args {
		matches, err := doublestar.Glob(arg)
		if err != nil {
			return nil, err
		}

		result = append(result, matches...)
	}

	return result, nil
}

func globRun(opts testsOptions) error {
	allfiles, err := expandGlobs(opts)

	if err != nil {
		return err
	}

	for _, filename := range allfiles {
		opts.Logger.Infoln(filename)
	}

	return nil
}

func splitRunE(opts testsOptions) error {
	return proxy.Exec([]string{"tests", "split"}, opts.args)
}
