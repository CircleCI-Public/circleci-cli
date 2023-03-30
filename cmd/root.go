package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api/header"
	"github.com/CircleCI-Public/circleci-cli/cmd/info"
	"github.com/CircleCI-Public/circleci-cli/cmd/policy"
	"github.com/CircleCI-Public/circleci-cli/cmd/project"
	"github.com/CircleCI-Public/circleci-cli/cmd/runner"
	"github.com/CircleCI-Public/circleci-cli/data"
	"github.com/CircleCI-Public/circleci-cli/md_docs"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var defaultEndpoint = "graphql-unstable"
var defaultHost = "https://circleci.com"
var defaultRestEndpoint = "api/v2"
var trueString = "true"

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

	helpWidth := getHelpWidth()
	// CircleCI Logo will only appear with enough window width
	longHelp := ""
	if helpWidth > 85 {
		longHelp = rootHelpLong()
	}

	rootCmd = &cobra.Command{
		Use:   "circleci",
		Long:  longHelp,
		Short: rootHelpShort(rootOptions),
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return rootCmdPreRun(rootOptions)
		},
	}

	// For supporting "Args" in command usage help
	cobra.AddTemplateFunc("HasAnnotations", hasAnnotations)
	cobra.AddTemplateFunc("PositionalArgs", md_docs.PositionalArgs)
	cobra.AddTemplateFunc("FormatPositionalArg", md_docs.FormatPositionalArg)

	if os.Getenv("TESTING") != trueString {
		helpCmd := helpCmd{width: helpWidth}
		rootCmd.SetHelpFunc(helpCmd.helpTemplate)
	}
	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.DisableAutoGenTag = true

	validator := func(_ *cobra.Command, _ []string) error {
		return validateToken(rootOptions)
	}

	rootCmd.AddCommand(newOpenCommand())
	rootCmd.AddCommand(newTestsCommand())
	rootCmd.AddCommand(newContextCommand(rootOptions))
	rootCmd.AddCommand(project.NewProjectCommand(rootOptions, validator))
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
	rootCmd.AddCommand(info.NewInfoCommand(rootOptions, validator))
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
	return os.Getenv("CI") == trueString || os.Getenv("CIRCLECI_CLI_SKIP_UPDATE_CHECK") == trueString
}

/**************** Help Menu Functions ****************/

// rootHelpLong creates content for the long field in the command
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

type helpCmd struct {
	width int
}

// helpTemplate Building a custom help template with more finesse and pizazz
func (helpCmd *helpCmd) helpTemplate(cmd *cobra.Command, s []string) {

	/***Styles ***/
	titleStyle := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: `#003740`, Dark: `#3B6385`}).
		BorderBottom(true).
		Margin(1, 0, 1, 0).
		Padding(0, 1, 0, 1).Align(lipgloss.Center)
	subCmdStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: `#161616`, Dark: `#FFFFFF`}).
		Padding(0, 4, 0, 4).Align(lipgloss.Left)
	subCmdInfoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: `#161616`, Dark: `#FFFFFF`}).Bold(true)
	textStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: `#161616`, Dark: `#FFFFFF`}).Align(lipgloss.Left).Margin(0).Padding(0)

	/** Building Usage String **/
	usageText := strings.Builder{}

	//get command path
	usageText.WriteString(titleStyle.Render(cmd.CommandPath()))

	//get command short or long
	cmdDesc := titleStyle.Render(cmd.Long)
	if strings.TrimSpace(cmdDesc) == "" || cmd.Name() == "circleci" {
		if cmd.Name() == "circleci" {
			cmdDesc += "\n\n" //add some spaces for circleci command
		}
		cmdDesc += subCmdStyle.Render(cmd.Short)
	}
	usageText.WriteString(cmdDesc + "\n")

	if len(cmd.Aliases) > 0 {
		aliases := titleStyle.Render("Aliases:")
		aliases += textStyle.Render(cmd.NameAndAliases())
		usageText.WriteString(aliases + "\n")
	}

	if cmd.Runnable() {
		usage := titleStyle.Render("Usage:")
		usage += textStyle.Render(cmd.UseLine())
		usageText.WriteString(usage + "\n")
	}

	if cmd.HasExample() {
		examples := titleStyle.Render("Example:")
		examples += textStyle.Render(cmd.Example)
		usageText.WriteString(examples + "\n")
	}

	if cmd.HasAvailableSubCommands() {
		subCmds := cmd.Commands()
		subTitle := titleStyle.Render("Available Commands:")
		subs := ""
		for i := range subCmds {
			if subCmds[i].IsAvailableCommand() {
				subs += subCmdStyle.Render(subCmds[i].Name()) + subCmdInfoStyle.
					PaddingLeft(subCmds[i].NamePadding()-len(subCmds[i].Name())+1).Render(subCmds[i].Short) + "\n"
			}
		}
		usageText.WriteString(lipgloss.JoinVertical(lipgloss.Left, subTitle, subs))
	}

	if cmd.HasAvailableLocalFlags() {
		flags := titleStyle.Render("Local Flags:")
		flags += textStyle.Render("\n" + cmd.LocalFlags().FlagUsages())
		usageText.WriteString(flags)
	}
	if cmd.HasAvailableInheritedFlags() {
		flags := titleStyle.Render("Global Flags:")
		flags += textStyle.Render("\n" + cmd.InheritedFlags().FlagUsages())
		usageText.WriteString(flags)
	}

	//Border styles
	borderStyle := lipgloss.NewStyle().
		Padding(0, 1, 0, 1).
		Width(helpCmd.width - 2).
		BorderForeground(lipgloss.AdaptiveColor{Light: `#3B6385`, Dark: `#47A359`}).
		Border(lipgloss.ThickBorder())

	log.Println("\n" + borderStyle.Render(usageText.String()+"\n"))
}

func getHelpWidth() int {
	const defaultHelpWidth = 122
	if !term.IsTerminal(0) {
		return defaultHelpWidth
	}
	w, _, err := term.GetSize(0)
	if err == nil && w < defaultHelpWidth {
		return w
	}
	return defaultHelpWidth
}
