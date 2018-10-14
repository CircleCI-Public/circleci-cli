package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/references"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"
)

type orbOptions struct {
	*settings.Config
	args []string
}

var orbAnnotations = map[string]string{
	"<path>":      "The path to your orb (use \"-\" for STDIN)",
	"<namespace>": "The namespace used for the orb (i.e. circleci)",
	"<orb>":       "A fully-qualified reference to an orb. This takes the form namespace/orb@version",
}

var orbListUncertified bool
var orbListJSON bool

func newOrbCommand(config *settings.Config) *cobra.Command {
	opts := orbOptions{
		Config: config,
	}

	listCommand := &cobra.Command{
		Use:   "list <namespace>",
		Short: "List orbs",
		Args:  cobra.MaximumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return listOrbs(opts)
		},
		Annotations: make(map[string]string),
	}
	listCommand.Annotations["<namespace>"] = orbAnnotations["<namespace>"] + " (Optional)"
	listCommand.PersistentFlags().BoolVarP(&orbListUncertified, "uncertified", "u", false, "include uncertified orbs")
	listCommand.PersistentFlags().BoolVar(&orbListJSON, "json", false, "print output as json instead of human-readable")
	if err := listCommand.PersistentFlags().MarkHidden("json"); err != nil {
		panic(err)
	}

	validateCommand := &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate an orb.yml",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return validateOrb(opts)
		},
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	validateCommand.Annotations["<path>"] = orbAnnotations["<path>"]

	processCommand := &cobra.Command{
		Use:   "process <path>",
		Short: "Validate an orb and print its form after all pre-registration processing",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return processOrb(opts)
		},
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	processCommand.Annotations["<path>"] = orbAnnotations["<path>"]

	publishCommand := &cobra.Command{
		Use:   "publish <path> <orb>",
		Short: "Publish an orb to the registry",
		Long: `Publish an orb to the registry.
Please note that at this time all orbs published to the registry are world-readable.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return publishOrb(opts)
		},
		Args:        cobra.ExactArgs(2),
		Annotations: make(map[string]string),
	}
	publishCommand.Annotations["<orb>"] = orbAnnotations["<orb>"]
	publishCommand.Annotations["<path>"] = orbAnnotations["<path>"]

	promoteCommand := &cobra.Command{
		Use:   "promote <orb> <segment>",
		Short: "Promote a development version of an orb to a semantic release",
		Long: `Promote a development version of an orb to a semantic release.
Please note that at this time all orbs promoted within the registry are world-readable.

Example: 'circleci orb publish promote foo/bar@dev:master major' => foo/bar@1.0.0`,
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return promoteOrb(opts)
		},
		Args:        cobra.ExactArgs(2),
		Annotations: make(map[string]string),
	}
	promoteCommand.Annotations["<orb>"] = orbAnnotations["<orb>"]
	promoteCommand.Annotations["<segment>"] = `"major"|"minor"|"patch"`

	incrementCommand := &cobra.Command{
		Use:   "increment <path> <namespace>/<orb> <segment>",
		Short: "Increment a released version of an orb",
		Long: `Increment a released version of an orb.
Please note that at this time all orbs incremented within the registry are world-readable.

Example: 'circleci orb publish increment foo/orb.yml foo/bar minor' => foo/bar@1.1.0`,
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return incrementOrb(opts)
		},
		Args:        cobra.ExactArgs(3),
		Annotations: make(map[string]string),
		Aliases:     []string{"inc"},
	}
	incrementCommand.Annotations["<path>"] = orbAnnotations["<path>"]
	incrementCommand.Annotations["<segment>"] = `"major"|"minor"|"patch"`

	publishCommand.AddCommand(promoteCommand)
	publishCommand.AddCommand(incrementCommand)

	sourceCommand := &cobra.Command{
		Use:   "source <orb>",
		Short: "Show the source of an orb",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return showSource(opts)
		},
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	sourceCommand.Annotations["<orb>"] = orbAnnotations["<orb>"]
	sourceCommand.Example = `  circleci orb source circleci/python@0.1.4 # grab the source at version 0.1.4
  circleci orb source my-ns/foo-orb@dev:latest # grab the source of dev release "latest"`

	orbCreate := &cobra.Command{
		Use:   "create <namespace>/<orb>",
		Short: "Create an orb in the specified namespace",
		Long: `Create an orb in the specified namespace
Please note that at this time all orbs created in the registry are world-readable.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return createOrb(opts)
		},
		Args: cobra.ExactArgs(1),
	}

	orbCommand := &cobra.Command{
		Use:   "orb",
		Short: "Operate on orbs",
	}

	orbCommand.AddCommand(listCommand)
	orbCommand.AddCommand(orbCreate)
	orbCommand.AddCommand(validateCommand)
	orbCommand.AddCommand(processCommand)
	orbCommand.AddCommand(publishCommand)
	orbCommand.AddCommand(sourceCommand)

	return orbCommand
}

func listOrbs(opts orbOptions) error {
	if len(opts.args) != 0 {
		return listNamespaceOrbs(opts)
	}

	ctx := context.Background()
	orbs, err := api.ListOrbs(ctx, opts.Config, orbListUncertified)
	if err != nil {
		return errors.Wrapf(err, "Failed to list orbs")
	}
	if orbListJSON {
		orbJSON, err := json.MarshalIndent(orbs, "", "  ")
		if err != nil {
			return errors.Wrapf(err, "Failed to convert to convert to JSON")
		}
		opts.Logger.Info(string(orbJSON))

	} else {
		opts.Logger.Info(orbs.String())
	}
	return nil
}

func listNamespaceOrbs(opts orbOptions) error {
	namespace := opts.args[0]

	ctx := context.Background()
	orbs, err := api.ListNamespaceOrbs(ctx, opts.Config, namespace)
	if err != nil {
		return errors.Wrapf(err, "Failed to list orbs in namespace `%s`", namespace)
	}
	if orbListJSON {
		orbJSON, err := json.MarshalIndent(orbs, "", "  ")
		if err != nil {
			return errors.Wrapf(err, "Failed to convert to convert to JSON")
		}
		opts.Logger.Info(string(orbJSON))
	} else {
		opts.Logger.Info(orbs.String())
	}
	return nil
}

func validateOrb(opts orbOptions) error {
	ctx := context.Background()

	_, err := api.OrbQuery(ctx, opts.Config, opts.args[0])

	if err != nil {
		return err
	}

	opts.Logger.Infof("Orb at `%s` is valid.", opts.args[0])
	return nil
}

func processOrb(opts orbOptions) error {
	ctx := context.Background()
	response, err := api.OrbQuery(ctx, opts.Config, opts.args[0])

	if err != nil {
		return err
	}

	opts.Logger.Info(response.OutputYaml)
	return nil
}

func publishOrb(opts orbOptions) error {
	ctx := context.Background()

	path := opts.args[0]
	ref := opts.args[1]
	namespace, orb, version, err := references.SplitIntoOrbNamespaceAndVersion(ref)

	if err != nil {
		return err
	}

	id, err := api.OrbID(ctx, opts.Config, namespace, orb)
	if err != nil {
		return err
	}

	_, err = api.OrbPublishByID(ctx, opts.Config, path, id.Data.Orb.ID, version)
	if err != nil {
		return err
	}

	opts.Logger.Infof("Orb `%s` was published.", ref)
	opts.Logger.Info("Please note that this is an open orb and is world-readable.")
	return nil
}

var validSegments = map[string]bool{
	"major": true,
	"minor": true,
	"patch": true}

func validateSegmentArg(label string) error {
	if _, valid := validSegments[label]; valid {
		return nil
	}
	return fmt.Errorf("expected `%s` to be one of \"major\", \"minor\", or \"patch\"", label)
}

func incrementOrb(opts orbOptions) error {
	ref := opts.args[1]
	segment := opts.args[2]

	if err := validateSegmentArg(segment); err != nil {
		return err
	}

	namespace, orb, err := references.SplitIntoOrbAndNamespace(ref)
	if err != nil {
		return err
	}

	response, err := api.OrbIncrementVersion(context.Background(), opts.Config, opts.args[0], namespace, orb, segment)

	if err != nil {
		return err
	}

	opts.Logger.Infof("Orb `%s` has been incremented to `%s/%s@%s`.\n", ref, namespace, orb, response.HighestVersion)
	opts.Logger.Info("Please note that this is an open orb and is world-readable.")
	return nil
}

func promoteOrb(opts orbOptions) error {
	ref := opts.args[0]
	segment := opts.args[1]

	if err := validateSegmentArg(segment); err != nil {
		return err
	}

	namespace, orb, version, err := references.SplitIntoOrbNamespaceAndVersion(ref)
	if err != nil {
		return err
	}

	if err = references.IsDevVersion(version); err != nil {
		return err
	}

	response, err := api.OrbPromote(context.Background(), opts.Config, namespace, orb, version, segment)
	if err != nil {
		return err
	}

	opts.Logger.Infof("Orb `%s` was promoted to `%s/%s@%s`.\n", ref, namespace, orb, response.HighestVersion)
	opts.Logger.Info("Please note that this is an open orb and is world-readable.")
	return nil
}

func createOrb(opts orbOptions) error {
	var err error
	ctx := context.Background()

	namespace, orb, err := references.SplitIntoOrbAndNamespace(opts.args[0])

	if err != nil {
		return err
	}

	_, err = api.CreateOrb(ctx, opts.Config, namespace, orb)

	if err != nil {
		return err
	}

	opts.Logger.Infof("Orb `%s` created.\n", opts.args[0])
	opts.Logger.Info("Please note that any versions you publish of this orb are world-readable.\n")
	opts.Logger.Infof("You can now register versions of `%s` using `circleci orb publish`.\n", opts.args[0])
	return nil
}

func showSource(opts orbOptions) error {
	ref := opts.args[0]

	source, err := api.OrbSource(context.Background(), opts.Config, ref)
	if err != nil {
		return errors.Wrapf(err, "Failed to get source for '%s'", ref)
	}
	opts.Logger.Info(source)
	return nil
}
