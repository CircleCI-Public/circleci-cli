package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/local"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

// These options are purely here to retain a mock of the structure of the flags used by `build`.
// They don't reflect the entire structure or available flags, only those which are public in the original command.
type proxyBuildArgs struct {
	configFilename string
	taskInfo       struct {
		NodeTotal     int
		NodeIndex     int
		Job           string
		SkipCheckout  bool
		Volumes       []string
		Revision      string
		Branch        string
		RepositoryURI string
	}
	checkoutKey string
	envVarArgs  []string
}

func newLocalExecuteCommand(config *settings.Config) *cobra.Command {
	opts := local.BuildOptions{
		Cfg: config,
	}

	buildCommand := &cobra.Command{
		Use:   "execute",
		Short: "Run a job in a container on the local machine",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.Args = args
			opts.Help = cmd.Help
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return local.Execute(opts)
		},
		DisableFlagParsing: true,
	}

	// Used as a convenience work-around when DisableFlagParsing is enabled
	// Allows help command to access the combined rollup of flags
	args := proxyBuildArgs{}
	flags := buildCommand.Flags()

	flags.StringVarP(&args.configFilename, "config", "c", local.DefaultConfigPath, "config file")
	flags.StringVar(&args.taskInfo.Job, "job", "build", "job to be executed")
	flags.IntVar(&args.taskInfo.NodeTotal, "node-total", 1, "total number of parallel nodes")
	flags.IntVar(&args.taskInfo.NodeIndex, "index", 0, "node index of parallelism")

	flags.BoolVar(&args.taskInfo.SkipCheckout, "skip-checkout", true, "use local path as-is")
	flags.StringSliceVarP(&args.taskInfo.Volumes, "volume", "v", nil, "Volume bind-mounting")

	flags.StringVar(&args.checkoutKey, "checkout-key", "~/.ssh/id_rsa", "Git Checkout key")
	flags.StringVar(&args.taskInfo.Revision, "revision", "", "Git Revision")
	flags.StringVar(&args.taskInfo.Branch, "branch", "", "Git branch")
	flags.StringVar(&args.taskInfo.RepositoryURI, "repo-url", "", "Git Url")

	flags.StringArrayVarP(&args.envVarArgs, "env", "e", nil, "Set environment variables, e.g. `-e VAR=VAL`")

	return buildCommand
}

func newBuildCommand(config *settings.Config) *cobra.Command {
	cmd := newLocalExecuteCommand(config)
	cmd.Hidden = true
	cmd.Use = "build"
	return cmd
}

func newLocalCommand(config *settings.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Debug jobs on the local machine",
	}
	cmd.AddCommand(newLocalExecuteCommand(config))
	return cmd
}
