package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
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

// nolint: gosec
func isCmdAvailable(name string) bool {
	cmd := exec.Command("/bin/sh", "-c", "command", "-v", name)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func usage(cmd *cobra.Command, args []string) error {
	if !isCmdAvailable("pandoc") {
		return errors.New(`
Unable to execute pandoc, please install it before using this command:
    https://pandoc.org/installing.html`)
	}

	tmpDir, err := ioutil.TempDir("", "circleci-cli-usage-")
	if err != nil {
		return err
	}

	docsPath := defaultDocsPath
	if len(args) > 0 {
		docsPath = args[0]
	}

	if err = os.MkdirAll(docsPath, 0700); err != nil {
		return errors.Wrap(err, "Could not create usage docs directory")
	}

	out, err := filepath.Abs(docsPath)
	if err != nil {
		return err
	}

	// generate markdown in tmpDir
	emptyStr := func(s string) string { return "" }
	err = doc.GenMarkdownTreeCustom(rootCmd, tmpDir, emptyStr, func(name string) string {
		base := strings.TrimSuffix(name, path.Ext(name))
		return base + ".html"
	})
	if err != nil {
		return err
	}

	// pandoc markdown from tmpDir to html into docsPath
	scriptfmt := "for f in %s/*.md; do pandoc \"$f\" -s -o \"%s/$(basename ${f%%.md}.html)\"; done"
	script := fmt.Sprintf(scriptfmt, tmpDir, out)

	// nolint: gosec
	pandoc := exec.Command("/bin/sh", "-c", script)
	err = pandoc.Run()
	if err != nil {
		return err
	}

	return os.RemoveAll(tmpDir)
}
