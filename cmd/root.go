package cmd

import (
	"os"
	"path"
	"runtime"

	"github.com/circleci/circleci-cli/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Execute adds all child commands to rootCmd and
// sets flags appropriately. This function is called
// by main.main(). It only needs to happen once to
// the RootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(-1)
	}
}

// Logger is exposed here so we can access it from subcommands.
// This allows us to print to the log at anytime from within the `cmd` package.
var Logger *logger.Logger

var rootCmd = &cobra.Command{
	Use:   "cli",
	Short: "Use CircleCI from the command line.",
	Long:  `Use CircleCI from the command line.`,
}

func addCommands() {
	rootCmd.AddCommand(diagnosticCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(collapseCommand)
	rootCmd.AddCommand(configureCommand)
}

func userHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func init() {

	configDir := path.Join(userHomeDir(), ".circleci")
	cobra.OnInitialize(setup)

	viper.SetConfigName("cli")
	viper.AddConfigPath(configDir)
	viper.SetEnvPrefix("circleci_cli")
	viper.AutomaticEnv()

	err := viper.ReadInConfig()

	// If reading the config file failed, then we want to create it.
	// TODO - handle invalid YAML config files.
	if err != nil {
		if _, err = os.Stat(configDir); os.IsNotExist(err) {
			if err = os.MkdirAll(configDir, 0700); err != nil {
				panic(err)
			}
		}
		if _, err = os.Create(path.Join(configDir, "cli.yml")); err != nil {
			panic(err)
		}
	}

	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging.")
	rootCmd.PersistentFlags().StringP("endpoint", "e", "https://circleci.com/graphql", "the endpoint of your CircleCI GraphQL API")
	rootCmd.PersistentFlags().StringP("token", "t", "", "your token for using CircleCI")

	Logger.FatalOnError("Error binding endpoint flag", viper.BindPFlag("endpoint", rootCmd.PersistentFlags().Lookup("endpoint")))
	Logger.FatalOnError("Error binding token flag", viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token")))

	err = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	if err != nil {
		panic(err)
	}

	addCommands()
}

func setup() {
	Logger = logger.NewLogger(viper.GetBool("verbose"))
}
