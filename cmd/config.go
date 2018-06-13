package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"

	"github.com/pkg/errors"

	"github.com/machinebox/graphql"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Path to the config.yml file to operate on.
var configPath string

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Operate on build config files",
}

var validateCommand = &cobra.Command{
	Use:     "validate",
	Aliases: []string{"check"},
	Short:   "Check that the config file is well formed.",
	RunE:    validateConfig,
}

func init() {
	validateCommand.Flags().StringVarP(&configPath, "path", "p", ".circleci/config.yml", "path to build config")
	configCmd.AddCommand(validateCommand)
}

func validateConfig(cmd *cobra.Command, args []string) error {

	ctx := context.Background()

	// Define a structure that matches the result of the GQL
	// query, so that we can use mapstructure to convert from
	// nested maps to a strongly typed struct.
	type validateResult struct {
		BuildConfig struct {
			Valid      bool
			SourceYaml string
			Errors     []struct {
				Message string
			}
		}
	}

	request := graphql.NewRequest(`
		query ValidateConfig ($config: String!) {
			buildConfig(configYaml: $config) {
				valid,
				errors { message },
				sourceYaml
			}
		}`)

	config, err := ioutil.ReadFile(configPath)

	if err != nil {
		return errors.Wrapf(err, "Could not load config file at %s", configPath)
	}

	request.Var("config", string(config))

	client := graphql.NewClient(viper.GetString("endpoint"))

	var result validateResult

	err = client.Run(ctx, request, &result)

	if err != nil {
		return errors.Wrap(err, "GraphQL query failed")
	}

	if !result.BuildConfig.Valid {

		var buffer bytes.Buffer

		for i := range result.BuildConfig.Errors {
			buffer.WriteString(result.BuildConfig.Errors[i].Message)
			buffer.WriteString("\n")
		}

		return fmt.Errorf("config file is invalid:\n%s", buffer.String())
	}

	fmt.Println("Config is valid")
	return nil

}
