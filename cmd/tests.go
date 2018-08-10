package cmd

import (
	"fmt"
	"os"

	"github.com/CircleCI-Public/circleci-cli/proxy"
	"github.com/bmatcuk/doublestar"
	"github.com/spf13/cobra"
)

func newTestsCommand() *cobra.Command {
	testsCmd := &cobra.Command{
		Use:    "tests",
		Short:  "Collect and split files with tests",
		Hidden: true,
	}

	globCmd := &cobra.Command{
		Use:    "glob",
		Short:  "glob files using pattern",
		Run:    globRun,
		Hidden: true,
	}

	splitCmd := &cobra.Command{
		Use:    "split",
		Short:  "return a split batch of provided files",
		RunE:   splitRunE,
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

func expandGlobs(args []string) ([]string, error) {
	result := []string{}

	for _, arg := range args {
		matches, err := doublestar.Glob(arg)
		if err != nil {
			return nil, err
		}

		result = append(result, matches...)
	}

	return result, nil
}

func globRun(cmd *cobra.Command, args []string) {
	allfiles, err := expandGlobs(args)

	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	for _, filename := range allfiles {
		fmt.Println(filename)
	}
}

func splitRunE(cmd *cobra.Command, args []string) error {
	return proxy.Exec("tests split", args)
}
