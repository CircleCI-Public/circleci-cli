package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/CircleCI-Public/circleci-cli/references"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"
)

type orbOptions struct {
	cfg  *settings.Config
	cl   *client.Client
	args []string

	listUncertified bool
	listJSON        bool
	listDetails     bool
	sortBy          string
	// Allows user to skip y/n confirm when creating an orb
	noPrompt bool
	// This lets us pass in our own interface for testing
	tty createOrbUserInterface
	// Linked with --integration-testing flag for stubbing UI in gexec tests
	integrationTesting bool
}

var orbAnnotations = map[string]string{
	"<path>":      "The path to your orb (use \"-\" for STDIN)",
	"<namespace>": "The namespace used for the orb (i.e. circleci)",
	"<orb>":       "A fully-qualified reference to an orb. This takes the form namespace/orb@version",
}

type createOrbUserInterface interface {
	askUserToConfirm(message string) bool
}

type createOrbInteractiveUI struct{}

func (createOrbInteractiveUI) askUserToConfirm(message string) bool {
	return prompt.AskUserToConfirm(message)
}

type createOrbTestUI struct {
	confirm bool
}

func (ui createOrbTestUI) askUserToConfirm(message string) bool {
	fmt.Println(message)
	return ui.confirm
}

func newOrbCommand(config *settings.Config) *cobra.Command {
	opts := orbOptions{
		cfg: config,
		tty: createOrbInteractiveUI{},
	}

	listCommand := &cobra.Command{
		Use:   "list <namespace>",
		Short: "List orbs",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return listOrbs(opts)
		},
		Annotations: make(map[string]string),
	}
	listCommand.Annotations["<namespace>"] = orbAnnotations["<namespace>"] + " (Optional)"

	listCommand.PersistentFlags().StringVar(&opts.sortBy, "sort", "", `one of "builds"|"projects"|"orgs"`)
	listCommand.PersistentFlags().BoolVarP(&opts.listUncertified, "uncertified", "u", false, "include uncertified orbs")
	listCommand.PersistentFlags().BoolVar(&opts.listJSON, "json", false, "print output as json instead of human-readable")
	listCommand.PersistentFlags().BoolVarP(&opts.listDetails, "details", "d", false, "output all the commands, executors, and jobs, along with a tree of their parameters")
	if err := listCommand.PersistentFlags().MarkHidden("json"); err != nil {
		panic(err)
	}

	validateCommand := &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate an orb.yml",
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
		Long: strings.Join([]string{
			"Use `$ circleci orb process` to resolve an orb, and it's dependencies to see how it would be expanded when you publish it to the registry.",
			"", // purposeful new-line
			"This can be helpful for validating an orb and debugging the processed form before publishing.",
		}, "\n"),
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token, config.Debug)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return processOrb(opts)
		},
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	processCommand.Example = `  circleci orb process src/my-orb/@orb.yml`
	processCommand.Annotations["<path>"] = orbAnnotations["<path>"]

	publishCommand := &cobra.Command{
		Use:   "publish <path> <orb>",
		Short: "Publish an orb to the registry",
		Long: `Publish an orb to the registry.
Please note that at this time all orbs published to the registry are world-readable.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return publishOrb(opts)
		},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return validateToken(opts.cfg)
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
		RunE: func(_ *cobra.Command, _ []string) error {
			return promoteOrb(opts)
		},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return validateToken(opts.cfg)
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
		RunE: func(_ *cobra.Command, _ []string) error {
			return incrementOrb(opts)
		},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return validateToken(opts.cfg)
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
		RunE: func(_ *cobra.Command, _ []string) error {
			if opts.integrationTesting {
				opts.tty = createOrbTestUI{
					confirm: true,
				}
			}

			return createOrb(opts)
		},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return validateToken(opts.cfg)
		},
		Args: cobra.ExactArgs(1),
	}
	orbCreate.Flags().BoolVar(&opts.integrationTesting, "integration-testing", false, "Enable test mode to bypass interactive UI.")
	if err := orbCreate.Flags().MarkHidden("integration-testing"); err != nil {
		panic(err)
	}
	orbCreate.Flags().BoolVar(&opts.noPrompt, "no-prompt", false, "Disable prompt to bypass interactive UI.")

	orbCommand := &cobra.Command{
		Use:   "orb",
		Short: "Operate on orbs",
		Long:  orbHelpLong(config),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			opts.args = args
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token, config.Debug)

			// PersistentPreRunE overwrites the inherited persistent hook from rootCmd
			// So we explicitly call it here to retain that behavior.
			// As of writing this comment, that is only for daily update checks.
			return rootCmdPreRun(rootOptions)
		},
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

func orbHelpLong(config *settings.Config) string {
	// We should only print this for cloud users
	if config.Host != defaultHost {
		return ""
	}

	return fmt.Sprintf(`Operate on orbs

See a full explanation and documentation on orbs here: %s`, config.Data.Links.OrbDocs)
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

// nolint: errcheck, gosec
func addOrbElementParametersToBuffer(buf *bytes.Buffer, orbElement api.OrbElement) {
	keys := make([]string, 0, len(orbElement.Parameters))
	for k := range orbElement.Parameters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		parameterName := k
		parameter := orbElement.Parameters[k]

		defaultValueString := parameterDefaultToString(parameter)
		_, _ = buf.WriteString(fmt.Sprintf("       - %s: %s%s\n", parameterName, parameter.Type, defaultValueString))
	}
}

// nolint: errcheck, gosec
func addOrbElementsToBuffer(buf *bytes.Buffer, name string, namedOrbElements map[string]api.OrbElement) {
	if len(namedOrbElements) > 0 {
		keys := make([]string, 0, len(namedOrbElements))
		for k := range namedOrbElements {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		_, _ = buf.WriteString(fmt.Sprintf("  %s:\n", name))
		for _, k := range keys {
			elementName := k
			orbElement := namedOrbElements[k]

			parameterCount := len(orbElement.Parameters)

			_, _ = buf.WriteString(fmt.Sprintf("    - %s: %d parameter(s)\n", elementName, parameterCount))

			if parameterCount > 0 {
				addOrbElementParametersToBuffer(buf, orbElement)
			}
		}
	}
}

// nolint: unparam, errcheck, gosec
func addOrbStatisticsToBuffer(buf *bytes.Buffer, name string, stats api.OrbStatistics) {
	var (
		encoded []byte
		data    map[string]int
	)

	// Roundtrip the stats to JSON so we can iterate a map since we don't care about the fields
	encoded, err := json.Marshal(stats)
	if err != nil {
		panic(err)
	}

	if err := json.Unmarshal(encoded, &data); err != nil {
		panic(err)
	}

	_, _ = buf.WriteString(fmt.Sprintf("  %s:\n", name))

	// Sort the keys so we always get the same results even after the round-trip
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := data[key]
		_, _ = buf.WriteString(fmt.Sprintf("    - %s: %d\n", key, value))
	}
}

func orbToDetailedString(orb api.OrbWithData) string {
	buffer := bytes.NewBufferString(orbToSimpleString(orb))

	addOrbElementsToBuffer(buffer, "Commands", orb.Commands)
	addOrbElementsToBuffer(buffer, "Jobs", orb.Jobs)
	addOrbElementsToBuffer(buffer, "Executors", orb.Executors)

	addOrbStatisticsToBuffer(buffer, "Statistics", orb.Statistics)

	return buffer.String()
}

func orbToSimpleString(orb api.OrbWithData) string {
	var buffer bytes.Buffer

	_, err := buffer.WriteString(fmt.Sprintln(orb.Name, "("+orb.HighestVersion+")"))
	if err != nil {
		// The WriteString docstring says that it will never return an error
		panic(err)
	}

	return buffer.String()
}

func orbCollectionToString(orbCollection *api.OrbsForListing, opts orbOptions) (string, error) {
	var result string

	if opts.listJSON {
		orbJSON, err := json.MarshalIndent(orbCollection, "", "  ")
		if err != nil {
			return "", errors.Wrapf(err, "Failed to convert to convert to JSON")
		}
		result = string(orbJSON)
	} else {
		result += fmt.Sprintf("Orbs found: %d. ", len(orbCollection.Orbs))
		if opts.listUncertified {
			result += "Includes all certified and uncertified orbs.\n\n"
		} else {
			result += "Showing only certified orbs.\nAdd --uncertified for a list of all orbs.\n\n"
		}
		for _, orb := range orbCollection.Orbs {
			if opts.listDetails {
				result += (orbToDetailedString(orb))
			} else {
				result += (orbToSimpleString(orb))
			}
		}
		result += "\nIn order to see more details about each orb, type: `circleci orb info orb-namespace/orb-name`\n"
		result += "\nSearch, filter, and view sources for all Orbs online at https://circleci.com/orbs/registry/"
	}

	return result, nil
}

func logOrbs(orbCollection *api.OrbsForListing, opts orbOptions) error {
	result, err := orbCollectionToString(orbCollection, opts)
	if err != nil {
		return err
	}

	fmt.Println(result)

	return nil
}

var validSortFlag = map[string]bool{
	"builds":   true,
	"projects": true,
	"orgs":     true}

func validateSortFlag(sort string) error {
	if _, valid := validSortFlag[sort]; valid {
		return nil
	}
	// TODO(zzak): we could probably reuse the map above to print the valid values
	return fmt.Errorf("expected `%s` to be one of \"builds\", \"projects\", or \"orgs\"", sort)
}

func listOrbs(opts orbOptions) error {
	if opts.sortBy != "" {
		if err := validateSortFlag(opts.sortBy); err != nil {
			return err
		}
	}

	if len(opts.args) != 0 {
		return listNamespaceOrbs(opts)
	}

	orbs, err := api.ListOrbs(opts.cl, opts.listUncertified)
	if err != nil {
		return errors.Wrapf(err, "Failed to list orbs")
	}

	if opts.sortBy != "" {
		orbs.SortBy(opts.sortBy)
	}

	return logOrbs(orbs, opts)
}

func listNamespaceOrbs(opts orbOptions) error {
	namespace := opts.args[0]

	orbs, err := api.ListNamespaceOrbs(opts.cl, namespace)
	if err != nil {
		return errors.Wrapf(err, "Failed to list orbs in namespace `%s`", namespace)
	}

	if opts.sortBy != "" {
		orbs.SortBy(opts.sortBy)
	}

	return logOrbs(orbs, opts)
}

func validateOrb(opts orbOptions) error {
	_, err := api.OrbQuery(opts.cl, opts.args[0])

	if err != nil {
		return err
	}

	if opts.args[0] == "-" {
		fmt.Println("Orb input is valid.")
	} else {
		fmt.Printf("Orb at `%s` is valid.\n", opts.args[0])
	}

	return nil
}

func processOrb(opts orbOptions) error {
	response, err := api.OrbQuery(opts.cl, opts.args[0])

	if err != nil {
		return err
	}

	fmt.Println(response.OutputYaml)
	return nil
}

func publishOrb(opts orbOptions) error {
	path := opts.args[0]
	ref := opts.args[1]
	namespace, orb, version, err := references.SplitIntoOrbNamespaceAndVersion(ref)

	if err != nil {
		return err
	}

	id, err := api.OrbID(opts.cl, namespace, orb)
	if err != nil {
		return err
	}

	_, err = api.OrbPublishByID(opts.cl, path, id.Orb.ID, version)
	if err != nil {
		return err
	}

	fmt.Printf("Orb `%s` was published.\n", ref)
	fmt.Println("Please note that this is an open orb and is world-readable.")

	if references.IsDevVersion(version) {
		fmt.Printf("Note that your dev label `%s` can be overwritten by anyone in your organization.\n", version)
		fmt.Printf("Your dev orb will expire in 90 days unless a new version is published on the label `%s`.\n", version)
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

	response, err := api.OrbIncrementVersion(opts.cl, opts.args[0], namespace, orb, segment)

	if err != nil {
		return err
	}

	fmt.Printf("Orb `%s` has been incremented to `%s/%s@%s`.\n", ref, namespace, orb, response.HighestVersion)
	fmt.Println("Please note that this is an open orb and is world-readable.")
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

	response, err := api.OrbPromote(opts.cl, namespace, orb, version, segment)
	if err != nil {
		return err
	}

	fmt.Printf("Orb `%s` was promoted to `%s/%s@%s`.\n", ref, namespace, orb, response.HighestVersion)
	fmt.Println("Please note that this is an open orb and is world-readable.")
	return nil
}

func createOrb(opts orbOptions) error {
	var err error

	namespace, orb, err := references.SplitIntoOrbAndNamespace(opts.args[0])

	if err != nil {
		return err
	}

	if !opts.noPrompt {
		fmt.Printf(`You are creating an orb called "%s/%s".

You will not be able to change the name of this orb.

If you change your mind about the name, you will have to create a new orb with the new name.

`, namespace, orb)
	}

	confirm := fmt.Sprintf("Are you sure you wish to create the orb: `%s/%s`", namespace, orb)

	if opts.noPrompt || opts.tty.askUserToConfirm(confirm) {
		_, err = api.CreateOrb(opts.cl, namespace, orb)

		if err != nil {
			return err
		}

		fmt.Printf("Orb `%s` created.\n", opts.args[0])
		fmt.Println("Please note that any versions you publish of this orb are world-readable.")
		fmt.Printf("You can now register versions of `%s` using `circleci orb publish`.\n", opts.args[0])
	}

	return nil
}

func showSource(opts orbOptions) error {
	ref := opts.args[0]

	source, err := api.OrbSource(opts.cl, ref)
	if err != nil {
		return errors.Wrapf(err, "Failed to get source for '%s'", ref)
	}
	fmt.Println(source)
	return nil
}

func orbInfo(opts orbOptions) error {
	ref := opts.args[0]

	info, err := api.OrbInfo(opts.cl, ref)
	if err != nil {
		return errors.Wrapf(err, "Failed to get info for '%s'", ref)
	}

	fmt.Println("")

	if len(info.Orb.Versions) > 0 {
		fmt.Printf("Latest: %s@%s\n", info.Orb.Name, info.Orb.HighestVersion)
		fmt.Printf("Last-updated: %s\n", info.Orb.Versions[0].CreatedAt)
		fmt.Printf("Created: %s\n", info.Orb.CreatedAt)
		// firstRelease := info.Orb.Versions[len(info.Orb.Versions)-1]

		fmt.Printf("Total-revisions: %d\n", len(info.Orb.Versions))
	} else {
		fmt.Println("This orb hasn't published any versions yet.")
	}

	fmt.Println("")

	fmt.Printf("Total-commands: %d\n", len(info.Orb.Commands))
	fmt.Printf("Total-executors: %d\n", len(info.Orb.Executors))
	fmt.Printf("Total-jobs: %d\n", len(info.Orb.Jobs))

	fmt.Println("")
	fmt.Println("## Statistics (30 days):")
	fmt.Printf("Builds: %d\n", info.Orb.Statistics.Last30DaysBuildCount)
	fmt.Printf("Projects: %d\n", info.Orb.Statistics.Last30DaysProjectCount)
	fmt.Printf("Orgs: %d\n", info.Orb.Statistics.Last30DaysOrganizationCount)

	orbVersionSplit := strings.Split(ref, "@")
	orbRef := orbVersionSplit[0]
	fmt.Printf(`
Learn more about this orb online in the CircleCI Orb Registry:
https://circleci.com/orbs/registry/orb/%s
`, orbRef)

	return nil
}
