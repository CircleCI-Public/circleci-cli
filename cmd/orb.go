package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/references"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"

	"github.com/spf13/cobra"
)

type orbOptions struct {
	cfg  *settings.Config
	cl   *client.Client
	log  *logger.Logger
	args []string
}

var orbAnnotations = map[string]string{
	"<path>":      "The path to your orb (use \"-\" for STDIN)",
	"<namespace>": "The namespace used for the orb (i.e. circleci)",
	"<orb>":       "A fully-qualified reference to an orb. This takes the form namespace/orb@version",
}

var orbListUncertified bool
var orbListJSON bool
var orbListDetails bool

func newOrbCommand(config *settings.Config) *cobra.Command {
	opts := orbOptions{
		cfg: config,
	}

	listCommand := &cobra.Command{
		Use:   "list <namespace>",
		Short: "List orbs",
		Args:  cobra.MaximumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return listOrbs(opts)
		},
		Annotations: make(map[string]string),
	}
	listCommand.Annotations["<namespace>"] = orbAnnotations["<namespace>"] + " (Optional)"
	listCommand.PersistentFlags().BoolVarP(&orbListUncertified, "uncertified", "u", false, "include uncertified orbs")
	listCommand.PersistentFlags().BoolVar(&orbListJSON, "json", false, "print output as json instead of human-readable")
	listCommand.PersistentFlags().BoolVarP(&orbListDetails, "details", "d", false, "output all the commands, executors, and jobs, along with a tree of their parameters")
	if err := listCommand.PersistentFlags().MarkHidden("json"); err != nil {
		panic(err)
	}

	validateCommand := &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate an orb.yml",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token)
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
			opts.log = logger.NewLogger(config.Debug)
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token)
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
			opts.log = logger.NewLogger(config.Debug)
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token)
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
			opts.log = logger.NewLogger(config.Debug)
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token)
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
			opts.log = logger.NewLogger(config.Debug)
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token)
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
			opts.log = logger.NewLogger(config.Debug)
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token)
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

	orbInfoCmd := &cobra.Command{
		Use:   "info <orb>",
		Short: "Show the meta-data of an orb",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return orbInfo(opts)
		},
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	orbInfoCmd.Annotations["<orb>"] = orbAnnotations["<orb>"]
	orbInfoCmd.Example = `  circleci orb info circleci/python@0.1.4
  circleci orb info my-ns/foo-orb@dev:latest`

	orbCreate := &cobra.Command{
		Use:   "create <namespace>/<orb>",
		Short: "Create an orb in the specified namespace",
		Long: `Create an orb in the specified namespace
Please note that at this time all orbs created in the registry are world-readable.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token)
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
	orbCommand.AddCommand(orbInfoCmd)

	return orbCommand
}

func parameterDefaultToString(parameter api.OrbElementParameter) string {
	defaultValue := " (default: '"

	// If there isn't a default or the default value is for a steps parameter
	// then just ignore the value.
	// It's possible to have a very large list of steps that pollutes the output.
	if parameter.Default == nil || parameter.Type == "steps" {
		return ""
	}

	switch parameter.Type {
	case "enum":
		defaultValue += parameter.Default.(string)
	case "string":
		defaultValue += parameter.Default.(string)
	case "boolean":
		defaultValue += fmt.Sprintf("%t", parameter.Default.(bool))
	default:
		defaultValue += ""
	}

	return defaultValue + "')"
}

func addOrbElementParametersToBuffer(buf *bytes.Buffer, orbElement api.OrbElement) error {
	for parameterName, parameter := range orbElement.Parameters {
		var err error

		defaultValueString := parameterDefaultToString(parameter)
		_, err = buf.WriteString(fmt.Sprintf("       - %s: %s%s\n", parameterName, parameter.Type, defaultValueString))

		if err != nil {
			return err
		}
	}

	return nil
}

func addOrbElementsToBuffer(buf *bytes.Buffer, name string, namedOrbElements map[string]api.OrbElement) {
	var err error

	if len(namedOrbElements) > 0 {
		_, err = buf.WriteString(fmt.Sprintf("  %s:\n", name))
		for elementName, orbElement := range namedOrbElements {
			parameterCount := len(orbElement.Parameters)

			_, err = buf.WriteString(fmt.Sprintf("    - %s: %d parameter(s)\n", elementName, parameterCount))

			if parameterCount > 0 {
				err = addOrbElementParametersToBuffer(buf, orbElement)
			}
		}
	}

	// This will never occur. The docs for bytes.Buffer.WriteString says err
	// will always be nil. The linter still expects this error to be checked.
	if err != nil {
		panic(err)
	}
}

func orbToDetailedString(orb api.Orb) string {
	buffer := bytes.NewBufferString(orbToSimpleString(orb))

	addOrbElementsToBuffer(buffer, "Commands", orb.Commands)
	addOrbElementsToBuffer(buffer, "Jobs", orb.Jobs)
	addOrbElementsToBuffer(buffer, "Executors", orb.Executors)

	return buffer.String()
}

func orbToSimpleString(orb api.Orb) string {
	var buffer bytes.Buffer

	_, err := buffer.WriteString(fmt.Sprintln(orb.Name, "("+orb.HighestVersion+")"))
	if err != nil {
		// The WriteString docstring says that it will never return an error
		panic(err)
	}

	return buffer.String()
}

func orbCollectionToString(orbCollection *api.OrbCollection) (string, error) {
	var result string

	if orbListJSON {
		orbJSON, err := json.MarshalIndent(orbCollection, "", "  ")
		if err != nil {
			return "", errors.Wrapf(err, "Failed to convert to convert to JSON")
		}
		result = string(orbJSON)
	} else {
		result += fmt.Sprintf("Total orbs found: %d\n\n", len(orbCollection.Orbs))
		for _, orb := range orbCollection.Orbs {
			if orbListDetails {
				result += (orbToDetailedString(orb))
			} else {
				result += (orbToSimpleString(orb))
			}
		}
	}

	return result, nil
}

func logOrbs(logger *logger.Logger, orbCollection *api.OrbCollection) error {
	result, err := orbCollectionToString(orbCollection)
	if err != nil {
		return err
	}

	logger.Info(result)

	return nil
}

func listOrbs(opts orbOptions) error {
	if len(opts.args) != 0 {
		return listNamespaceOrbs(opts)
	}

	ctx := context.Background()
	orbs, err := api.ListOrbs(ctx, opts.log, opts.cl, orbListUncertified)
	if err != nil {
		return errors.Wrapf(err, "Failed to list orbs")
	}

	return logOrbs(opts.log, orbs)
}

func listNamespaceOrbs(opts orbOptions) error {
	namespace := opts.args[0]

	ctx := context.Background()
	orbs, err := api.ListNamespaceOrbs(ctx, opts.log, opts.cl, namespace)
	if err != nil {
		return errors.Wrapf(err, "Failed to list orbs in namespace `%s`", namespace)
	}

	return logOrbs(opts.log, orbs)
}

func validateOrb(opts orbOptions) error {
	ctx := context.Background()

	_, err := api.OrbQuery(ctx, opts.log, opts.cl, opts.args[0])

	if err != nil {
		return err
	}

	if opts.args[0] == "-" {
		opts.log.Infof("Orb input is valid.")
	} else {
		opts.log.Infof("Orb at `%s` is valid.", opts.args[0])
	}

	return nil
}

func processOrb(opts orbOptions) error {
	ctx := context.Background()
	response, err := api.OrbQuery(ctx, opts.log, opts.cl, opts.args[0])

	if err != nil {
		return err
	}

	opts.log.Info(response.OutputYaml)
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

	id, err := api.OrbID(ctx, opts.log, opts.cl, namespace, orb)
	if err != nil {
		return err
	}

	_, err = api.OrbPublishByID(ctx, opts.log, opts.cl, path, id.Orb.ID, version)
	if err != nil {
		return err
	}

	opts.log.Infof("Orb `%s` was published.", ref)
	opts.log.Info("Please note that this is an open orb and is world-readable.")

	if references.IsDevVersion(version) {
		opts.log.Infof("Note that your dev label `%s` can be overwritten by anyone in your organization.", version)
		opts.log.Infof("Your dev orb will expire in 90 days unless a new version is published on the label `%s`.", version)
	}
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

	response, err := api.OrbIncrementVersion(context.Background(), opts.log, opts.cl, opts.args[0], namespace, orb, segment)

	if err != nil {
		return err
	}

	opts.log.Infof("Orb `%s` has been incremented to `%s/%s@%s`.\n", ref, namespace, orb, response.HighestVersion)
	opts.log.Info("Please note that this is an open orb and is world-readable.")
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

	if !references.IsDevVersion(version) {
		return fmt.Errorf("The version '%s' must be a dev version (the string should begin `dev:`)", version)
	}

	response, err := api.OrbPromote(context.Background(), opts.log, opts.cl, namespace, orb, version, segment)
	if err != nil {
		return err
	}

	opts.log.Infof("Orb `%s` was promoted to `%s/%s@%s`.\n", ref, namespace, orb, response.HighestVersion)
	opts.log.Info("Please note that this is an open orb and is world-readable.")
	return nil
}

func createOrb(opts orbOptions) error {
	var err error
	ctx := context.Background()

	namespace, orb, err := references.SplitIntoOrbAndNamespace(opts.args[0])

	if err != nil {
		return err
	}

	_, err = api.CreateOrb(ctx, opts.log, opts.cl, namespace, orb)

	if err != nil {
		return err
	}

	opts.log.Infof("Orb `%s` created.\n", opts.args[0])
	opts.log.Info("Please note that any versions you publish of this orb are world-readable.\n")
	opts.log.Infof("You can now register versions of `%s` using `circleci orb publish`.\n", opts.args[0])
	return nil
}

func showSource(opts orbOptions) error {
	ref := opts.args[0]

	source, err := api.OrbSource(context.Background(), opts.log, opts.cl, ref)
	if err != nil {
		return errors.Wrapf(err, "Failed to get source for '%s'", ref)
	}
	opts.log.Info(source)
	return nil
}

func orbInfo(opts orbOptions) error {
	ref := opts.args[0]

	info, err := api.OrbInfo(context.Background(), opts.log, opts.cl, ref)
	if err != nil {
		return errors.Wrapf(err, "Failed to get info for '%s'", ref)
	}

	opts.log.Info("\n")

	revisions := info.OrbVersion.Orb.Versions

	if len(revisions) > 0 {
		opts.log.Infof("Latest: %s@%s", info.OrbVersion.Orb.Name, revisions[0].Version)
		opts.log.Infof("Last-updated: %s", revisions[0].CreatedAt)
		opts.log.Infof("Created: %s", info.OrbVersion.Orb.CreatedAt)
		firstRelease := revisions[len(revisions)-1]
		opts.log.Infof("First-release: %s @ %s", firstRelease.Version, firstRelease.CreatedAt)

		opts.log.Infof("Total-revisions: %d", len(revisions))
	} else {
		opts.log.Infof("This orb hasn't published any versions yet.")
	}

	opts.log.Info("\n")

	var (
		jobs      = 0
		commands  = 0
		executors = 0
	)

	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(info.OrbVersion.Source), &raw); err != nil {
		return errors.Wrap(err, "Unable to parse orb source")
	}

	var orbSource struct {
		Jobs      map[string]interface{}
		Commands  map[string]interface{}
		Executors map[string]interface{}
	}
	if err := mapstructure.WeakDecode(raw, &orbSource); err != nil {
		return errors.Wrap(err, "Unable to decode orb source")
	}

	for range orbSource.Commands {
		commands++
	}

	for range orbSource.Executors {
		executors++
	}

	for range orbSource.Jobs {
		jobs++
	}

	opts.log.Infof("Total-commands: %d", commands)
	opts.log.Infof("Total-executors: %d", executors)
	opts.log.Infof("Total-jobs: %d", jobs)

	return nil
}
