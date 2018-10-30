package cmd

import (
	"fmt"
	"os"
	"strconv"

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

// rootOptions is used internally for preparing CLI and passed to sub-commands
var rootOptions *settings.Config

// rootTokenFromFlag stores the value passed in through the flag --token
var rootTokenFromFlag string

// AutoUpdate defines the default behavior to include `circleci update` command with update feature.
var AutoUpdate = "true"

// PackageManager defines the package manager which was used to install the CLI.
// You can override this value using -X flag to the compiler ldflags.
var PackageManager = "source"

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
	rootOptions = &settings.Config{
		Debug:    false,
		Token:    "",
		Host:     defaultHost,
		Endpoint: defaultEndpoint,
	}

	if err := rootOptions.Load(); err != nil {
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

	rootCmd.AddCommand(newTestsCommand(rootOptions))
	rootCmd.AddCommand(newQueryCommand(rootOptions))
	rootCmd.AddCommand(newConfigCommand(rootOptions))
	rootCmd.AddCommand(newOrbCommand(rootOptions))
	rootCmd.AddCommand(newLocalCommand(rootOptions))
	rootCmd.AddCommand(newBuildCommand(rootOptions))
	rootCmd.AddCommand(newVersionCommand(rootOptions))
	rootCmd.AddCommand(newDiagnosticCommand(rootOptions))
	rootCmd.AddCommand(newSetupCommand(rootOptions))

	if isUpdateIncluded(AutoUpdate) {
		rootCmd.AddCommand(newUpdateCommand(rootOptions))
	} else {
		rootCmd.AddCommand(newDisabledCommand(rootOptions, "update"))
	}

	rootCmd.AddCommand(newNamespaceCommand(rootOptions))
	rootCmd.AddCommand(newUsageCommand(rootOptions))
	rootCmd.AddCommand(newStepCommand(rootOptions))
	rootCmd.AddCommand(newSwitchCommand(rootOptions))

	rootCmd.PersistentFlags().BoolVar(&rootOptions.Debug,
		"debug", rootOptions.Debug, "Enable debug logging.")
	rootCmd.PersistentFlags().StringVar(&rootTokenFromFlag,
		"token", "", "your token for using CircleCI")
	rootCmd.PersistentFlags().StringVar(&rootOptions.Host,
		"host", rootOptions.Host, "URL to your CircleCI host")
	rootCmd.PersistentFlags().StringVar(&rootOptions.Endpoint,
		"endpoint", rootOptions.Endpoint, "URI to your CircleCI GraphQL API endpoint")
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

func init() {
	cobra.OnInitialize(prepare)
}

func prepare() {
	if rootTokenFromFlag != "" {
		rootOptions.Token = rootTokenFromFlag
	}
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

func isUpdateIncluded(flag string) bool {
	conv, err := strconv.ParseBool(flag)
	if err != nil {
		panic(err)
	}

	return conv
}
