package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api/header"
	"github.com/CircleCI-Public/circleci-cli/cmd/policy"
	"github.com/CircleCI-Public/circleci-cli/cmd/runner"
	"github.com/CircleCI-Public/circleci-cli/data"
	"github.com/CircleCI-Public/circleci-cli/md_docs"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var defaultEndpoint = "graphql-unstable"
var defaultHost = "https://circleci.com"
var defaultRestEndpoint = "api/v2"

// const trueString = "true" //this constant makes go lint happy

// rootCmd is used internally and global to the package but not exported
// therefore we can use it in other commands, like `usage`
// it should be set once when Execute is first called
var rootCmd *cobra.Command

// rootOptions is used internally for preparing CLI and passed to sub-commands
var rootOptions *settings.Config

// rootTokenFromFlag stores the value passed in through the flag --token
var rootTokenFromFlag string

// Execute adds all child commands to rootCmd and
// sets flags appropriately. This function is called
// by main.main(). It only needs to happen once to
// the rootCmd.
func Execute() {
	header.SetCommandStr(CommandStr())
	command := MakeCommands()
	if err := command.Execute(); err != nil {
		os.Exit(-1)
	}
}

// Returns a string (e.g. "circleci context list") indicating what
// subcommand is being called, without any args or flags,
// for API headers.
func CommandStr() string {
	command := MakeCommands()
	subCmd, _, err := command.Find(os.Args[1:])
	if err != nil {
		return "unknown"
	}
	parentNames := []string{subCmd.Name()}
	subCmd.VisitParents(func(parent *cobra.Command) {
		parentNames = append([]string{parent.Name()}, parentNames...)
	})
	return strings.Join(parentNames, " ")
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
		Debug:        false,
		Token:        "",
		Host:         defaultHost,
		RestEndpoint: defaultRestEndpoint,
		Endpoint:     defaultEndpoint,
		GitHubAPI:    "https://api.github.com/",
	}

	if err := rootOptions.Load(); err != nil {
		panic(err)
	}

	rootOptions.Data = &data.Data

	rootCmd = &cobra.Command{
		Use:   "circleci",
		Long:  rootHelpLong(),
		Short: rootHelpShort(rootOptions),
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return rootCmdPreRun(rootOptions)
		},
	}

	// For supporting "Args" in command usage help
	cobra.AddTemplateFunc("HasAnnotations", hasAnnotations)
	cobra.AddTemplateFunc("PositionalArgs", md_docs.PositionalArgs)
	cobra.AddTemplateFunc("FormatPositionalArg", md_docs.FormatPositionalArg)

	helpCmd := helpCmd{cmd: rootCmd}
	rootCmd.SetHelpTemplate(helpCmd.helpTemplate())
	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.DisableAutoGenTag = true

	validator := func(_ *cobra.Command, _ []string) error {
		return validateToken(rootOptions)
	}

	rootCmd.AddCommand(newOpenCommand())
	rootCmd.AddCommand(newTestsCommand())
	rootCmd.AddCommand(newContextCommand(rootOptions))
	rootCmd.AddCommand(newQueryCommand(rootOptions))
	rootCmd.AddCommand(newConfigCommand(rootOptions))
	rootCmd.AddCommand(newOrbCommand(rootOptions))
	rootCmd.AddCommand(runner.NewCommand(rootOptions, validator))
	rootCmd.AddCommand(newLocalCommand(rootOptions))
	rootCmd.AddCommand(newBuildCommand(rootOptions))
	rootCmd.AddCommand(newVersionCommand(rootOptions))
	rootCmd.AddCommand(newDiagnosticCommand(rootOptions))
	rootCmd.AddCommand(newSetupCommand(rootOptions))

	rootCmd.AddCommand(followProjectCommand(rootOptions))
	rootCmd.AddCommand(policy.NewCommand(rootOptions, validator))

	if isUpdateIncluded(version.PackageManager()) {
		rootCmd.AddCommand(newUpdateCommand(rootOptions))
	} else {
		rootCmd.AddCommand(newDisabledCommand(rootOptions, "update"))
	}

	rootCmd.AddCommand(newNamespaceCommand(rootOptions))
	rootCmd.AddCommand(newUsageCommand(rootOptions))
	rootCmd.AddCommand(newStepCommand(rootOptions))
	rootCmd.AddCommand(newSwitchCommand(rootOptions))
	rootCmd.AddCommand(newAdminCommand(rootOptions))
	rootCmd.AddCommand(newCompletionCommand())

	flags := rootCmd.PersistentFlags()

	flags.BoolVar(&rootOptions.Debug, "debug", rootOptions.Debug, "Enable debug logging.")
	flags.StringVar(&rootTokenFromFlag, "token", "", "your token for using CircleCI, also CIRCLECI_CLI_TOKEN")
	flags.StringVar(&rootOptions.Host, "host", rootOptions.Host, "URL to your CircleCI host, also CIRCLECI_CLI_HOST")
	flags.StringVar(&rootOptions.Endpoint, "endpoint", rootOptions.Endpoint, "URI to your CircleCI GraphQL API endpoint")
	flags.StringVar(&rootOptions.GitHubAPI, "github-api", "https://api.github.com/", "Change the default endpoint to GitHub API for retrieving updates")
	flags.BoolVar(&rootOptions.SkipUpdateCheck, "skip-update-check", skipUpdateByDefault(), "Skip the check for updates check run before every command.")

	hidden := []string{"github-api", "debug", "endpoint"}

	for _, f := range hidden {
		if err := flags.MarkHidden(f); err != nil {
			panic(err)
		}
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

func rootCmdPreRun(rootOptions *settings.Config) error {
	// If an error occurs checking for updates, we should print the error but
	// not break the CLI entirely.
	err := checkForUpdates(rootOptions)
	if err != nil {
		fmt.Printf("Error checking for updates: %s\n", err)
		fmt.Printf("Please contact support.\n\n")
	}
	return nil
}

func validateToken(rootOptions *settings.Config) error {
	var (
		err error
		url string
	)

	if rootOptions.Host == defaultHost {
		url = rootOptions.Data.Links.NewAPIToken
	} else {
		url = fmt.Sprintf("%s/account/api", rootOptions.Host)
	}

	if rootOptions.Token == "token" || rootOptions.Token == "" {
		err = fmt.Errorf(`please set a token with 'circleci setup'
You can create a new personal API token here:
%s`, url)
	}

	return err
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

func isUpdateIncluded(packageManager string) bool {
	switch packageManager {
	case "homebrew", "snap":
		return false
	default:
		return true
	}
}

func skipUpdateByDefault() bool {
	return os.Getenv("CI") == "true" || os.Getenv("CIRCLECI_CLI_SKIP_UPDATE_CHECK") == "true"
}

/**************** Help Menu Functions ****************/

//rootHelpLong creates content for the long field in the command
func rootHelpLong() string {
	long := `
                          /??                     /??                         /??
                         |__/                    | ??                        |__/
  /????????      /??????? /??  /??????   /???????| ??  /??????       /??????? /??
 /_______/??    /??_____/| ?? /??__  ?? /??_____/| ?? /??__  ??     /??_____/| ??
    /?? | ??   | ??      | ??| ??  \__/| ??      | ??| ????????    | ??      | ??
   |__/ | ??   | ??      | ??| ??      | ??      | ??| ??_____/    | ??      | ??
  /????????    |  ???????| ??| ??      |  ???????| ??|  ???????    |  ???????| ??
 /________/     \_______/|__/|__/       \_______/|__/ \_______/     \_______/|__/`
	return long
}

// rootHelpShort creates content for the short feild in the command
func rootHelpShort(config *settings.Config) string {
	short := `Use CircleCI from the command line.
This project is the seed for CircleCI's new command-line application.`

	// We should only print this for cloud users
	if config.Host != defaultHost {
		return short
	}

	return fmt.Sprintf(`%s

For more help, see the documentation here: %s`, short, config.Data.Links.CLIDocs)
}

const (
	defaultWidth = 100

	//default colors
	purple    = `#7e2fcc`
	darkGrey  = `#353C3B`
	lightTeal = `#03DAC5`
	darkTeal  = `#01A299`
	white     = `#e5e5e5`
	red       = `#FF3333`
)

type helpCmd struct {
	cmd *cobra.Command
}

// helpTemplate Building a custom help template with more finess and pizazz
func (helpCmd *helpCmd) helpTemplate() string {
	cmd := helpCmd.cmd
	log.Println(cmd)
	usageText := strings.Builder{}

	//Title styles
	titleStyle := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: darkTeal, Dark: lightTeal}).
		Underline(true).
		BorderBottom(true).
		Margin(1, 0, 1, 0).
		Padding(0, 1, 0, 1).Align(lipgloss.Center)

	//get command name and style it
	cmdName := cmd.Long
	cmdName = titleStyle.Render(cmdName)
	usageText.WriteString(cmdName + "\n")

	// cmdLong := m.styles.SubTitle.Render(m.cmd.Long)
	// cmdTitle = m.styles.Title.Foreground(lipgloss.AdaptiveColor{Light: darkGrey, Dark: white}).
	// 	Render(lipgloss.JoinVertical(lipgloss.Center, cmdName, cmdLong))
	// usageText.WriteString(cmdTitle + "\n")

	// cmdSection := m.styles.Section.Render("Cmd Description:")
	// short := m.styles.Text.Render(m.cmd.Short)

	// usageText.WriteString(lipgloss.JoinVertical(lipgloss.Left, cmdSection, short) + "\n")

	// if m.cmd.Runnable() {
	// 	usage := m.styles.Section.Render("Usage:")
	// 	useLine := m.styles.Text.Render(m.cmd.UseLine())
	// 	usageText.WriteString(lipgloss.JoinVertical(lipgloss.Left, usage, useLine) + "\n")
	// commandPath := m.styles.Text.Render(m.cmd.CommandPath() + " [command]")
	// usageText.WriteString(lipgloss.JoinVertical(lipgloss.Left, cmd.cmd.CommandPath()) + "\n")
	// }

	// if len(m.cmd.Aliases) > 0 {
	// 	aliases := m.styles.Section.Render("Aliases:")
	// 	nameAndAlias := m.styles.Text.Render(m.cmd.NameAndAliases())
	// 	usageText.WriteString(lipgloss.JoinVertical(lipgloss.Left, aliases, nameAndAlias) + "\n")
	// }
	// if m.cmd.HasExample() {
	// 	examples := m.styles.Section.Render("Examples:")
	// 	example := m.styles.Text.Render(m.cmd.Example)
	// 	usageText.WriteString(lipgloss.JoinVertical(lipgloss.Left, examples, example) + "\n")
	// }

	// if m.cmd.HasAvailableSubCommands() {
	// 	usageText.WriteString(lipgloss.JoinVertical(lipgloss.Left, m.list.View()))
	// }

	// if m.cmd.HasAvailableLocalFlags() {
	// 	localFlags := m.styles.Section.Render("Flags:")
	// 	flagUsage := m.styles.Text.Render(strings.TrimRightFunc(m.cmd.LocalFlags().FlagUsages(), unicode.IsSpace))
	// 	usageText.WriteString(lipgloss.JoinVertical(lipgloss.Left, localFlags, flagUsage) + "\n")
	// }
	// if m.cmd.HasAvailableInheritedFlags() {
	// 	globalFlags := m.styles.Section.Render("Global Flags:")
	// 	flagUsage := m.styles.Text.Render(strings.TrimRightFunc(m.cmd.InheritedFlags().FlagUsages(), unicode.IsSpace))
	// 	usageText.WriteString(lipgloss.JoinVertical(lipgloss.Left, globalFlags, flagUsage) + "\n")
	// }

	// 	var ht2 = `
	// {{if .HasAvailableSubCommands}}{{.CommandPath}} [command]{{end}}
	// {{if gt (len .Aliases) 0}}
	// Aliases: {{.NameAndAliases}}{{end}}
	// {{if .Long}}{{.Long}}{{else}}{{.Short}}{{end}}
	// {{if .HasExample}}
	// Examples:
	// {{.Example}}{{end}}{{if .HasAvailableSubCommands}}

	// Available Commands: {{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
	//   {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if (HasAnnotations .)}}
	// {{$cmd := .}}
	// Args:
	// {{range (PositionalArgs .)}}  {{(FormatPositionalArg $cmd .)}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

	// Flags:
	// {{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

	// Global Flags:
	// {{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}
	// Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
	//   {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}

	// `
	//Border styles
	borderStyle := lipgloss.NewStyle().
		Padding(0, 1, 0, 1).
		Width(defaultWidth).
		BorderForeground(lipgloss.AdaptiveColor{Light: darkTeal, Dark: lightTeal}).
		Border(lipgloss.ThickBorder())

	return borderStyle.Render(usageText.String() + "\n")
}
