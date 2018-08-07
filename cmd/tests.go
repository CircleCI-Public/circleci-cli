package cmd

import (
	"fmt"
	"os"

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

	testsCmd.AddCommand(globCmd)

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
