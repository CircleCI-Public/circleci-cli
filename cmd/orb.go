package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/pkg/errors"

	"github.com/machinebox/graphql"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

func newOrbCommand() *cobra.Command {

	listCommand := &cobra.Command{
		Use:   "list",
		Short: "List orbs",
		RunE:  listOrbs,
	}

	validateCommand := &cobra.Command{
		Use:   "validate PATH (use \"-\" for STDIN)",
		Short: "validate an orb.yml",
		RunE:  validateOrb,
		Args:  cobra.ExactArgs(1),
	}

	expandCommand := &cobra.Command{
		Use:   "expand PATH (use \"-\" for STDIN)",
		Short: "expand an orb.yml",
		RunE:  expandOrb,
		Args:  cobra.ExactArgs(1),
	}

	publishCommand := &cobra.Command{
		Use:   "publish",
		Short: "publish a version of an orb",
	}

	publishCommand.AddCommand(&cobra.Command{
		Use:   "release PATH NAMESPACE ORB SEMVER",
		Short: "release a semantic version of an orb",
		RunE:  releaseOrb,
		Args:  cobra.ExactArgs(4),
	})

	sourceCommand := &cobra.Command{
		Use:   "source <namespace>/<name>",
		Short: "Show the source of an orb",
		RunE:  showSource,
		Args:  cobra.ExactArgs(1),
	}

	orbCreate := &cobra.Command{
		Use:   "create <namespace>/<name>",
		Short: "create an orb",
		RunE:  createOrb,
		Args:  cobra.ExactArgs(1),
	}

	orbCommand := &cobra.Command{
		Use:   "orb",
		Short: "Operate on orbs",
	}

	orbCommand.AddCommand(listCommand)
	orbCommand.AddCommand(orbCreate)
	orbCommand.AddCommand(validateCommand)
	orbCommand.AddCommand(expandCommand)
	orbCommand.AddCommand(publishCommand)
	orbCommand.AddCommand(sourceCommand)

	return orbCommand
}

type orb struct {
	Commands  map[string]struct{}
	Jobs      map[string]struct{}
	Executors map[string]struct{}
}

func addOrbElementsToBuffer(buf *bytes.Buffer, name string, elems map[string]struct{}) error {
	var err error

	if len(elems) > 0 {
		_, err = buf.WriteString(fmt.Sprintf("  %s:\n", name))
		if err != nil {
			return err
		}
		for key := range elems {
			_, err = buf.WriteString(fmt.Sprintf("    - %s\n", key))
			if err != nil {
				return err
			}
		}
	}

	return err
}

func (orb orb) String() string {
	var buffer bytes.Buffer

	err := addOrbElementsToBuffer(&buffer, "Commands", orb.Commands)
	// FIXME: refactor this to handle the error
	if err != nil {
		panic(err)
	}
	err = addOrbElementsToBuffer(&buffer, "Jobs", orb.Jobs)
	if err != nil {
		panic(err)
	}
	err = addOrbElementsToBuffer(&buffer, "Executors", orb.Executors)
	if err != nil {
		panic(err)
	}
	return buffer.String()
}

func listOrbs(cmd *cobra.Command, args []string) error {

	ctx := context.Background()

	// Define a structure that matches the result of the GQL
	// query, so that we can use mapstructure to convert from
	// nested maps to a strongly typed struct.
	type orbList struct {
		Orbs struct {
			TotalCount int
			Edges      []struct {
				Cursor string
				Node   struct {
					Name     string
					Versions []struct {
						Version string
						Source  string
					}
				}
			}
			PageInfo struct {
				HasNextPage bool
			}
		}
	}

	request := graphql.NewRequest(`
query ListOrbs ($after: String!) {
  orbs(first: 20, after: $after) {
	totalCount,
    edges {
	  node {
	    name
		  versions(count: 1) {
			version,
			source
		  }
		}
	}
    pageInfo {
      hasNextPage
    }
  }
}
	`)

	client := client.NewClient(viper.GetString("endpoint"), Logger)

	var result orbList
	currentCursor := ""

	for {
		request.Var("after", currentCursor)
		err := client.Run(ctx, request, &result)

		if err != nil {
			return errors.Wrap(err, "GraphQL query failed")
		}

		// Debug logging of result fields.
		// Logger.Prettyify(result)
	Orbs:
		for i := range result.Orbs.Edges {
			edge := result.Orbs.Edges[i]
			currentCursor = edge.Cursor
			if len(edge.Node.Versions) > 0 {
				v := edge.Node.Versions[0]

				Logger.Infof("%s (%s)", edge.Node.Name, v.Version)

				var o orb

				err := yaml.Unmarshal([]byte(edge.Node.Versions[0].Source), &o)

				if err != nil {
					Logger.Error(fmt.Sprintf("Corrupt Orb %s %s", edge.Node.Name, v.Version), err)
					continue Orbs
				}

				Logger.Info(o.String())

			}
		}

		if !result.Orbs.PageInfo.HasNextPage {
			break
		}
	}
	return nil
}

func validateOrb(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	response, err := api.OrbQuery(ctx, Logger, args[0])

	if err != nil {
		return err
	}

	if !response.Valid {
		return response.ToError()
	}

	Logger.Infof("Orb at %s is valid", args[0])
	return nil
}

func expandOrb(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	response, err := api.OrbQuery(ctx, Logger, args[0])

	if err != nil {
		return err
	}

	if !response.Valid {
		return response.ToError()
	}

	Logger.Info(response.OutputYaml)
	return nil
}

func releaseOrb(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	response, err := api.OrbPublish(ctx, Logger, args[0], args[1], args[2], args[3])

	if err != nil {
		return err
	}

	if len(response.Errors) > 0 {
		return response.ToError()
	}

	Logger.Info("Orb published")
	return nil
}

func createOrb(cmd *cobra.Command, args []string) error {
	var err error
	ctx := context.Background()

	arr := strings.Split(args[0], "/")

	if len(arr) != 2 {
		return fmt.Errorf("Invalid orb name: %s", args[0])
	}

	response, err := api.CreateOrb(ctx, Logger, arr[1], arr[0])

	if err != nil {
		return err
	}

	if len(response.Errors) > 0 {
		return response.ToError()
	}

	Logger.Info("Orb created")
	return nil
}

func showSource(cmd *cobra.Command, args []string) error {
	source, err := api.OrbSource(context.Background(), Logger, args[0])
	if err != nil {
		return errors.Wrapf(err, "Failed to get source for '%s'", args[0])
	}
	Logger.Info(source)
	return nil
}
