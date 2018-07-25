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

var orbVersion string
var orbID string
var organizationName string
var organizationVcs string

func newOrbCommand() *cobra.Command {

	orbListCommand := &cobra.Command{
		Use:   "list",
		Short: "List orbs",
		RunE:  listOrbs,
	}

	orbValidateCommand := &cobra.Command{
		Use:   "validate [orb.yml]",
		Short: "validate an orb.yml",
		RunE:  validateOrb,
		Args:  cobra.MaximumNArgs(1),
	}

	orbExpandCommand := &cobra.Command{
		Use:   "expand [orb.yml]",
		Short: "expand an orb.yml",
		RunE:  expandOrb,
		Args:  cobra.MaximumNArgs(1),
	}

	orbPublishCommand := &cobra.Command{
		Use:   "publish [orb.yml]",
		Short: "publish a version of an orb",
		RunE:  publishOrb,
		Args:  cobra.MaximumNArgs(1),
	}
	orbPublishCommand.PersistentFlags().StringVarP(&orbVersion, "orb-version", "o", "", "version of orb to publish")
	orbPublishCommand.PersistentFlags().StringVarP(&orbID, "orb-id", "i", "", "id of orb to publish")

	orbCreateNamespace := &cobra.Command{
		Use:   "create",
		Short: "create an orb namespace",
		RunE:  createOrbNamespace,
		Args:  cobra.ExactArgs(1),
	}

	namespaceCommand := &cobra.Command{
		Use: "ns",
	}

	orbCommand := &cobra.Command{
		Use:   "orb",
		Short: "Operate on orbs",
	}

	orbCommand.AddCommand(orbListCommand)

	orbCommand.AddCommand(orbValidateCommand)

	orbCommand.AddCommand(orbExpandCommand)

	orbCommand.AddCommand(orbPublishCommand)

	orbCreateNamespace.PersistentFlags().StringVar(&organizationName, "org-name", "", "organization name")
	orbCreateNamespace.PersistentFlags().StringVar(&organizationVcs, "vcs", "github", "organization vcs, e.g. 'github', 'bitbucket'")
	namespaceCommand.AddCommand(orbCreateNamespace)
	orbCommand.AddCommand(namespaceCommand)

	return orbCommand
}

type orb struct {
	Commands  map[string]struct{}
	Jobs      map[string]struct{}
	Executors map[string]struct{}
}

func addOrbElementsToBuffer(buf *bytes.Buffer, name string, elems map[string]struct{}) {
	if len(elems) > 0 {
		buf.WriteString(fmt.Sprintf("  %s:\n", name))
		for key := range elems {
			buf.WriteString(fmt.Sprintf("    - %s\n", key))
		}
	}
}

func (orb orb) String() string {
	var buffer bytes.Buffer
	addOrbElementsToBuffer(&buffer, "Commands", orb.Commands)
	addOrbElementsToBuffer(&buffer, "Jobs", orb.Jobs)
	addOrbElementsToBuffer(&buffer, "Executors", orb.Executors)
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

const defaultOrbPath = "orb.yml"

func validateOrb(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	orbPath := defaultOrbPath
	if len(args) == 1 {
		orbPath = args[0]
	}
	response, err := api.OrbQuery(ctx, Logger, orbPath)

	if err != nil {
		return err
	}

	if !response.Valid {
		return response.ToError()
	}

	Logger.Infof("Orb at %s is valid", orbPath)
	return nil
}

func expandOrb(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	orbPath := defaultOrbPath
	if len(args) == 1 {
		orbPath = args[0]
	}
	response, err := api.OrbQuery(ctx, Logger, orbPath)

	if err != nil {
		return err
	}

	if !response.Valid {
		return response.ToError()
	}

	Logger.Info(response.OutputYaml)
	return nil
}

func publishOrb(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	orbPath := defaultOrbPath
	if len(args) == 1 {
		orbPath = args[0]
	}

	response, err := api.OrbPublish(ctx, Logger, orbPath, orbVersion, orbID)

	if err != nil {
		return err
	}

	if len(response.Errors) > 0 {
		return response.ToError()
	}

	Logger.Info("Orb published")
	return nil
}

func createOrbNamespace(cmd *cobra.Command, args []string) error {
	var err error
	ctx := context.Background()

	response, err := api.CreateNamespace(ctx, Logger, args[0], organizationName, strings.ToUpper(organizationVcs))

	if err != nil {
		return err
	}

	if len(response.Errors) > 0 {
		return response.ToError()
	}

	Logger.Info("Namespace created")
	return nil
}
