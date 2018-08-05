package cmd

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newUsageCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "usage [PATH] (default is \"docs\")",
		Short:  "Generate usage documentation in markdown for the CLI.",
		Hidden: true,
		RunE:   usage,
		Args:   cobra.MaximumNArgs(1),
	}
}

var defaultDocsPath = "docs"

func usage(cmd *cobra.Command, args []string) error {
	docsPath := defaultDocsPath
	if len(args) > 0 {
		docsPath = args[0]
	}

	if err := os.MkdirAll(docsPath, 0700); err != nil {
		return errors.Wrap(err, "Could not create usage docs directory")
	}

	out, err := filepath.Abs(docsPath)
	if err != nil {
		return err
	}

	// generate markdown to out
	emptyStr := func(s string) string { return "" }
	return doc.GenMarkdownTreeCustom(rootCmd, out, emptyStr, func(name string) string {
		base := strings.TrimSuffix(name, path.Ext(name))
		return base + ".html"
	})
}
