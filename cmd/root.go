package cmd

import (
	"fmt"
	"os"

	"github.com/CircleCI-Public/circleci-cli/md_docs"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

var defaultEndpoint = "graphql-unstable"
var defaultHost = "https://circleci.com"

// rootCmd is used internally and global to the package but not exported
// therefore we can use it in other commands, like `usage`
// it should be set once when Execute is first called
var rootCmd *cobra.Command

// Execute adds all child commands to rootCmd and
// sets flags appropriately. This function is called
// by main.main(). It only needs to happen once to
// the rootCmd.
func Execute() {
	command := MakeCommands()
	if err := command.Execute(); err != nil {
		os.Exit(-1)
	}
}

func hasAnnotations(cmd *cobra.Command) bool {
	return len(cmd.Annotations) > 0
}

var usageTemplate = `
Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}
Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if (HasAnnotations .)}}
{{$cmd := .}}
Args:
{{range (PositionalArgs .)}}  {{(FormatPositionalArg $cmd .)}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}
Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}
Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

// MakeCommands creates the top level commands
func MakeCommands() *cobra.Command {
	config := &settings.Config{
		Debug:    false,
		Token:    "",
		Host:     defaultHost,
		Endpoint: defaultEndpoint,
	}

	if err := config.Load(); err != nil {
		panic(err)
	}

	rootCmd = &cobra.Command{
		Use:   "circleci",
		Short: `Use CircleCI from the command line.`,
		Long:  `This project is the seed for CircleCI's new command-line application.`,
	}

	// For supporting "Args" in command usage help
	cobra.AddTemplateFunc("HasAnnotations", hasAnnotations)
	cobra.AddTemplateFunc("PositionalArgs", md_docs.PositionalArgs)
	cobra.AddTemplateFunc("FormatPositionalArg", md_docs.FormatPositionalArg)
	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.DisableAutoGenTag = true

	rootCmd.AddCommand(newTestsCommand(config))
	rootCmd.AddCommand(newQueryCommand(config))
	rootCmd.AddCommand(newConfigCommand(config))
	rootCmd.AddCommand(newOrbCommand(config))
	rootCmd.AddCommand(newLocalCommand(config))
	rootCmd.AddCommand(newBuildCommand(config))
	rootCmd.AddCommand(newVersionCommand(config))
	rootCmd.AddCommand(newDiagnosticCommand(config))
	rootCmd.AddCommand(newSetupCommand(config))
	rootCmd.AddCommand(newUpdateCommand(config))
	rootCmd.AddCommand(newNamespaceCommand(config))
	rootCmd.AddCommand(newUsageCommand(config))
	rootCmd.AddCommand(newStepCommand(config))
	rootCmd.AddCommand(newSwitchCommand(config))

	rootCmd.PersistentFlags().BoolVar(&config.Debug, "debug", config.Debug, "Enable debug logging.")
	rootCmd.PersistentFlags().StringVar(&config.Token, "token", config.Token, "your token for using CircleCI")
	rootCmd.PersistentFlags().StringVar(&config.Host, "host", config.Host, "URL to your CircleCI host")
	rootCmd.PersistentFlags().StringVar(&config.Endpoint, "endpoint", config.Endpoint, "URI to your CircleCI GraphQL API endpoint")
	if err := rootCmd.PersistentFlags().MarkHidden("debug"); err != nil {
		panic(err)
	}

	if err := rootCmd.PersistentFlags().MarkHidden("endpoint"); err != nil {
		panic(err)
	}

	// Cobra has a peculiar default behaviour:
	// https://github.com/spf13/cobra/issues/340
	// If you expose a command with `RunE`, and return an error from your
	// command, then Cobra will print the error message, followed by the usage
	// information for the command. This makes it really difficult to see what's
	// gone wrong. It usually prints a one line error message followed by 15
	// lines of usage information.
	// This flag disables that behaviour, so that if a comment fails, it prints
	// just the error message.
	rootCmd.SilenceUsage = true

	setFlagErrorFuncAndValidateArgs(rootCmd)

	return rootCmd
}

func setFlagErrorFunc(cmd *cobra.Command, err error) error {
	if e := cmd.Help(); e != nil {
		return e
	}
	fmt.Println("")
	return err
}

func setFlagErrorFuncAndValidateArgs(command *cobra.Command) {
	visitAll(command, func(cmd *cobra.Command) {
		cmd.SetFlagErrorFunc(setFlagErrorFunc)

		if cmd.Args == nil {
			return
		}

		cmdArgs := cmd.Args
		cmd.Args = func(cccmd *cobra.Command, args []string) error {
			if err := cmdArgs(cccmd, args); err != nil {
				if e := cccmd.Help(); e != nil {
					return e
				}

				fmt.Println("")
				return err
			}

			return nil
		}
	})
}

func visitAll(root *cobra.Command, fn func(*cobra.Command)) {
	for _, cmd := range root.Commands() {
		visitAll(cmd, fn)
	}
	fn(root)
}
