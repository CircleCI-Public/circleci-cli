package cmd

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/md_docs"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type usageOptions struct {
	cfg  *settings.Config
	tree bool
	args []string
}

func newUsageCommand(config *settings.Config) *cobra.Command {
	opts := usageOptions{
		cfg: config,
	}

	usageCmd := &cobra.Command{
		Use:    "usage <path> (default is \"docs\")",
		Short:  "Generate usage documentation in markdown for the CLI.",
		Hidden: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return usage(opts)
		},
		Args: cobra.MaximumNArgs(1),
	}

	usageCmd.PersistentFlags().BoolVarP(&opts.tree, "tree", "", false, "generate a single file for the command tree")
	if err := usageCmd.PersistentFlags().MarkHidden("tree"); err != nil {
		panic(err)
	}
}

var defaultDocsPath = "docs"

func usage(opts usageOptions) error {
	docsPath := defaultDocsPath
	if len(opts.args) > 0 {
		docsPath = opts.args[0]
	}

	if err := os.MkdirAll(docsPath, 0700); err != nil {
		return errors.Wrap(err, "Could not create usage docs directory")
	}

	out, err := filepath.Abs(docsPath)
	if err != nil {
		return err
	}

	if opts.tree {
		return md_docs.GenMarkdownTreeSingle(rootCmd, out)
	}

	// generate markdown to out
	emptyStr := func(s string) string { return "" }
	return md_docs.GenMarkdownTreeCustom(rootCmd, out, emptyStr, func(name string) string {
		base := strings.TrimSuffix(name, path.Ext(name))
		return base + ".html"
	})
}
