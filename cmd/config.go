package cmd

import (
	"bytes"
	"context"
	"io/ioutil"

	"github.com/CircleCI-Public/circleci-cli/client"
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

var expandCommand = &cobra.Command{
	Use:   "expand",
	Short: "Expand the config.",
	RunE:  expandConfig,
}

func init() {
	configCmd.PersistentFlags().StringVarP(&configPath, "path", "p", ".circleci/config.yml", "path to build config")
	configCmd.AddCommand(validateCommand)
	configCmd.AddCommand(expandCommand)
}

// Define a structure that matches the result of the GQL
// query, so that we can use mapstructure to convert from
// nested maps to a strongly typed struct.
type buildConfigResponse struct {
	BuildConfig struct {
		Valid      bool
		SourceYaml string
		OutputYaml string

		Errors []struct {
			Message string
		}
	}
}

func queryAPI(ctx context.Context, query string, variables map[string]string, response interface{}) error {

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)

	for varName, varValue := range variables {
		request.Var(varName, varValue)
	}

	client := graphql.NewClient(viper.GetString("endpoint"))

	return client.Run(ctx, request, response)
}

func loadYaml(path string) (string, error) {

	config, err := ioutil.ReadFile(path)

	if err != nil {
		return "", errors.Wrapf(err, "Could not load config file at %s", path)
	}

	return string(config), nil
}

func (response buildConfigResponse) processErrors() error {
	var buffer bytes.Buffer

	buffer.WriteString("\n")
	for i := range response.BuildConfig.Errors {
		buffer.WriteString("-- ")
		buffer.WriteString(response.BuildConfig.Errors[i].Message)
		buffer.WriteString(",\n")
	}

	return errors.New(buffer.String())
}

func configQuery(ctx context.Context) (*buildConfigResponse, error) {

	query := `
		query ValidateConfig ($config: String!) {
			buildConfig(configYaml: $config) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`

	config, err := loadYaml(configPath)
	if err != nil {
		return nil, err
	}

	variables := map[string]string{
		"config": config,
	}

	var response buildConfigResponse
	err = queryAPI(ctx, query, variables, &response)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to validate config")
	}

	return &response, nil
}

func validateConfig(cmd *cobra.Command, args []string) error {

	ctx := context.Background()
	response, err := configQuery(ctx)

	if err != nil {
		return err
	}

	if !response.BuildConfig.Valid {
		return response.processErrors()
	}

	Logger.Infof("Config file at %s is valid", configPath)
	return nil
}

func expandConfig(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	response, err := configQuery(ctx)

	if err != nil {
		return err
	}

	if !response.BuildConfig.Valid {
		return response.processErrors()
	}

	Logger.Info(response.BuildConfig.OutputYaml)
	return nil
}
