package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/references"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"
)

var orbAnnotations = map[string]string{
	"PATH":      "The path to your orb (use \"-\" for STDIN)",
	"NAMESPACE": "The namespace used for the orb (i.e. circleci)",
	"ORB":       "The name of your orb (i.e. rails)",
	"<orb>":     "A fully-qualified reference to an orb. This takes the form namespace/orb@version",
}

var orbListUncertified bool
var orbListJSON bool

func newOrbCommand() *cobra.Command {

	listCommand := &cobra.Command{
		Use:         "list NAMESPACE",
		Short:       "List orbs",
		Args:        cobra.MaximumNArgs(1),
		RunE:        listOrbs,
		Annotations: make(map[string]string),
	}
	listCommand.Annotations["NAMESPACE"] = orbAnnotations["NAMESPACE"] + " (Optional)"
	listCommand.PersistentFlags().BoolVarP(&orbListUncertified, "uncertified", "u", false, "include uncertified orbs")
	listCommand.PersistentFlags().BoolVar(&orbListJSON, "json", false, "print output as json instead of human-readable")
	if err := listCommand.PersistentFlags().MarkHidden("json"); err != nil {
		panic(err)
	}

	validateCommand := &cobra.Command{
		Use:         "validate PATH",
		Short:       "validate an orb.yml",
		RunE:        validateOrb,
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	validateCommand.Annotations["PATH"] = orbAnnotations["PATH"]

	processCommand := &cobra.Command{
		Use:         "process PATH",
		Short:       "process an orb",
		RunE:        processOrb,
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	processCommand.Annotations["PATH"] = orbAnnotations["PATH"]

	publishCommand := &cobra.Command{
		Use:         "publish <path> <orb>",
		Short:       "publish an orb to the registry",
		RunE:        publishOrb,
		Args:        cobra.ExactArgs(2),
		Annotations: make(map[string]string),
	}
	publishCommand.Annotations["<orb>>"] = orbAnnotations["<orb>"]
	publishCommand.Annotations["<path>"] = orbAnnotations["PATH"]

	promoteCommand := &cobra.Command{
		Use:         "promote <orb> <segment>",
		Short:       "promote a development version of an orb to a semantic release",
		RunE:        promoteOrb,
		Args:        cobra.ExactArgs(2),
		Annotations: make(map[string]string),
	}
	promoteCommand.Annotations["<orb>"] = orbAnnotations["<orb>"]
	promoteCommand.Annotations["<segment>"] = `"major"|"minor"|"patch"`

	incrementCommand := &cobra.Command{
		Use:         "increment PATH NAMESPACE ORB SEGMENT",
		Short:       "increment a released version of an orb",
		RunE:        incrementOrb,
		Args:        cobra.ExactArgs(4),
		Annotations: make(map[string]string),
		Aliases:     []string{"inc"},
	}
	incrementCommand.Annotations["PATH"] = orbAnnotations["PATH"]
	incrementCommand.Annotations["NAMESPACE"] = orbAnnotations["NAMESPACE"]
	incrementCommand.Annotations["ORB"] = orbAnnotations["ORB"]
	incrementCommand.Annotations["SEGMENT"] = `"major"|"minor"|"patch"`

	publishCommand.AddCommand(promoteCommand)
	publishCommand.AddCommand(incrementCommand)

	sourceCommand := &cobra.Command{
		Use:         "source <orb>",
		Short:       "Show the source of an orb",
		RunE:        showSource,
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	sourceCommand.Annotations["<orb>"] = orbAnnotations["<orb>"]
	sourceCommand.Example = `  circleci orb source circleci/python@0.1.4 # grab the source at version 0.1.4
  circleci orb source my-ns/foo-orb@dev:latest # grab the source of dev release "latest"`

	orbCreate := &cobra.Command{
		Use:   "create <namespace>/<orb>",
		Short: "create an orb in the specified namespace",
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
	orbCommand.AddCommand(processCommand)
	orbCommand.AddCommand(publishCommand)
	orbCommand.AddCommand(sourceCommand)

	return orbCommand
}

func listOrbs(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return listNamespaceOrbs(args[0])
	}

	ctx := context.Background()
	orbs, err := api.ListOrbs(ctx, Logger, orbListUncertified)
	if err != nil {
		return errors.Wrapf(err, "Failed to list orbs")
	}
	if orbListJSON {
		orbJSON, err := json.MarshalIndent(orbs, "", "  ")
		if err != nil {
			return errors.Wrapf(err, "Failed to convert to convert to JSON")
		}
		Logger.Info(string(orbJSON))

	} else {
		Logger.Info(orbs.String())
	}
	return nil
}

func listNamespaceOrbs(namespace string) error {
	ctx := context.Background()
	orbs, err := api.ListNamespaceOrbs(ctx, Logger, namespace)
	if err != nil {
		return errors.Wrapf(err, "Failed to list orbs in namespace %s", namespace)
	}
	if orbListJSON {
		orbJSON, err := json.MarshalIndent(orbs, "", "  ")
		if err != nil {
			return errors.Wrapf(err, "Failed to convert to convert to JSON")
		}
		Logger.Info(string(orbJSON))
	} else {
		Logger.Info(orbs.String())
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

func processOrb(cmd *cobra.Command, args []string) error {
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

func publishOrb(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	path := args[0]
	ref := args[1]
	namespace, orb, version, err := references.SplitIntoOrbNamespaceAndVersion(ref)

	if err != nil {
		return err
	}

	id, err := api.OrbID(ctx, Logger, namespace, orb)
	if err != nil {
		return err
	}

	_, err = api.OrbPublishByID(ctx, Logger, path, id, version)
	if err != nil {
		return err
	}

	Logger.Infof("Orb `%s` was published.", ref)
	Logger.Info("Please note that this is an open orb and is world-readable.")
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
	return fmt.Errorf(`expected %s to be one of "major", "minor", or "patch"`, label)
}

func incrementOrb(cmd *cobra.Command, args []string) error {
	if err := validateSegmentArg(args[3]); err != nil {
		return err
	}

	response, err := api.OrbIncrementVersion(context.Background(), Logger, args[0], args[1], args[2], args[3])
	if err != nil {
		return err
	}

	Logger.Infof("Orb %s/%s bumped to %s\n", args[1], args[2], response.HighestVersion)
	return nil
}

func promoteOrb(cmd *cobra.Command, args []string) error {

	ref := args[0]
	segment := args[1]

	if err := validateSegmentArg(segment); err != nil {
		return err
	}

	namespace, orb, version, err := references.SplitIntoOrbNamespaceAndVersion(ref)

	if err != nil {
		return err
	}

	if !references.IsDevVersion(version) {
		return fmt.Errorf("The version '%s' must be a dev version (the string should begin `dev:`)", version)
	}

	response, err := api.OrbPromote(context.Background(), Logger, namespace, orb, version, segment)
	if err != nil {
		return err
	}

	Logger.Infof("Orb %s promoted to %s", ref, response.HighestVersion)
	return nil
}

func createOrb(cmd *cobra.Command, args []string) error {
	var err error
	ctx := context.Background()

	namespace, orb, err := references.SplitIntoOrbAndNamespace(args[0])

	if err != nil {
		return err
	}

	response, err := api.CreateOrb(ctx, Logger, namespace, orb)

	// Only fall back to native graphql errors if there are no in-band errors.
	if err != nil && (response == nil || len(response.Errors) == 0) {
		return err
	}

	if len(response.Errors) > 0 {
		return response.ToError()
	}

	Logger.Infof("Orb `%s` created.", args[0])
	Logger.Info("Please note that any versions you publish of this orb are world-readable.")
	Logger.Infof("You can now register versions of `%s` using `circleci orb publish`", args[0])
	return nil
}

func showSource(cmd *cobra.Command, args []string) error {

	ref := args[0]

	source, err := api.OrbSource(context.Background(), Logger, ref)
	if err != nil {
		return errors.Wrapf(err, "Failed to get source for '%s'", ref)
	}
	Logger.Info(source)
	return nil
}
