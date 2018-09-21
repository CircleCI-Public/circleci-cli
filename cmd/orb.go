package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"
)

var orbAnnotations = map[string]string{
	"PATH":      "The path to your orb (use \"-\" for STDIN)",
	"NAMESPACE": "The namespace used for the orb (i.e. circleci)",
	"ORB":       "The name of your orb (i.e. rails)",
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
		Use:   "publish",
		Short: "publish a version of an orb",
	}

	releaseCommand := &cobra.Command{
		Use:         "release PATH NAMESPACE ORB SEMVER",
		Short:       "release a semantic version of an orb",
		RunE:        releaseOrb,
		Args:        cobra.ExactArgs(4),
		Annotations: make(map[string]string),
	}
	releaseCommand.Annotations["PATH"] = orbAnnotations["PATH"]
	releaseCommand.Annotations["NAMESPACE"] = orbAnnotations["NAMESPACE"]
	releaseCommand.Annotations["ORB"] = orbAnnotations["ORB"]
	releaseCommand.Annotations["SEMVER"] = "The semantic version used for this release (i.e. 0.3.6)"

	devCommand := &cobra.Command{
		Use:         "dev PATH NAMESPACE ORB LABEL",
		Short:       "release a development version of an orb",
		RunE:        devOrb,
		Args:        cobra.ExactArgs(4),
		Annotations: make(map[string]string),
	}
	devCommand.Annotations["PATH"] = orbAnnotations["PATH"]
	devCommand.Annotations["NAMESPACE"] = orbAnnotations["NAMESPACE"]
	devCommand.Annotations["ORB"] = orbAnnotations["ORB"]
	devCommand.Annotations["LABEL"] = `Tag to use for this development version (i.e. "volatile")`

	promoteCommand := &cobra.Command{
		Use:         "promote NAMESPACE ORB LABEL SEGMENT",
		Short:       "promote a development version of an orb to a semantic release",
		RunE:        promoteOrb,
		Args:        cobra.ExactArgs(4),
		Annotations: make(map[string]string),
	}
	promoteCommand.Annotations["NAMESPACE"] = orbAnnotations["NAMESPACE"]
	promoteCommand.Annotations["ORB"] = orbAnnotations["ORB"]
	promoteCommand.Annotations["LABEL"] = `Tag to use for this development version (i.e. "volatile")`
	promoteCommand.Annotations["SEGMENT"] = `"major"|"minor"|"patch"`

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

	publishCommand.AddCommand(releaseCommand)
	publishCommand.AddCommand(promoteCommand)
	publishCommand.AddCommand(devCommand)
	publishCommand.AddCommand(incrementCommand)

	sourceCommand := &cobra.Command{
		Use:         "source NAMESPACE ORB",
		Short:       "Show the source of an orb",
		RunE:        showSource,
		Args:        cobra.ExactArgs(2),
		Annotations: make(map[string]string),
	}
	sourceCommand.Annotations["NAMESPACE"] = orbAnnotations["NAMESPACE"]
	sourceCommand.Annotations["ORB"] = orbAnnotations["ORB"]

	orbCreate := &cobra.Command{
		Use:         "create NAMESPACE ORB",
		Short:       "create an orb",
		RunE:        createOrb,
		Args:        cobra.ExactArgs(2),
		Annotations: make(map[string]string),
	}
	orbCreate.Annotations["NAMESPACE"] = orbAnnotations["NAMESPACE"]
	orbCreate.Annotations["ORB"] = orbAnnotations["ORB"]

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

func releaseOrb(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	id, err := api.OrbID(ctx, Logger, args[1], args[2])
	if err != nil {
		return err
	}

	response, err := api.OrbPublishByID(ctx, Logger, args[0], id, args[3])
	if err != nil {
		return err
	}

	Logger.Infof("Orb published %s", response.Orb.Version)
	return nil
}

func devLabel(label string) string {
	// Ensure the `dev:` tag is prefixed to the label, no matter what
	return fmt.Sprintf("dev:%s", strings.TrimPrefix(label, "dev:"))
}

func devOrb(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	id, err := api.OrbID(ctx, Logger, args[1], args[2])
	if err != nil {
		return err
	}

	response, err := api.OrbPublishByID(ctx, Logger, args[0], id, devLabel(args[3]))
	if err != nil {
		return err
	}

	Logger.Infof("Orb published %s", response.Orb.Version)
	return nil
}

var validSegments = []string{"major", "minor", "patch"}

func validateSegmentArg(label string) error {
	for _, segment := range validSegments {
		if label == segment {
			return nil
		}
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

	Logger.Infof("Orb %s/%s bumped to %s\n", args[1], args[2], response.Orb.Version)
	return nil
}

func promoteOrb(cmd *cobra.Command, args []string) error {
	if err := validateSegmentArg(args[3]); err != nil {
		return err
	}

	response, err := api.OrbPromote(context.Background(), Logger, args[0], args[1], devLabel(args[2]), args[3])
	if err != nil {
		return err
	}

	Logger.Infof("Orb promoted to %s", response.Orb.Version)
	return nil
}

func createOrb(cmd *cobra.Command, args []string) error {
	var err error
	ctx := context.Background()

	response, err := api.CreateOrb(ctx, Logger, args[0], args[1])

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
	source, err := api.OrbSource(context.Background(), Logger, args[0], args[1])
	if err != nil {
		return errors.Wrapf(err, "Failed to get source for '%s' in %s", args[1], args[0])
	}
	Logger.Info(source)
	return nil
}
