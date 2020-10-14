package cmd

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/filetree"
	"github.com/CircleCI-Public/circleci-cli/process"
	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/CircleCI-Public/circleci-cli/references"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/manifoldco/promptui"
)

type orbOptions struct {
	cfg  *settings.Config
	cl   *graphql.Client
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

type orbProtectTemplateRelease struct {
	ZipUrl string `json:"zipball_url"`
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
			opts.cl = graphql.NewClient(config.Host, config.Endpoint, config.Token, config.Debug)
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

	unlistCmd := &cobra.Command{
		Use:   "unlist <namespace>/<orb> <true|false>",
		Short: "Disable or enable an orb's listing in the registry",
		Long: `Disable or enable an orb's listing in the registry.
This only affects whether the orb is displayed in registry search results;
the orb remains world-readable as long as referenced with a valid name.

Example: Run 'circleci orb unlist foo/bar true' to disable the listing of the
orb in the registry and 'circleci orb unlist foo/bar false' to re-enable the
listing of the orb in the registry.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return setOrbListStatus(opts)
		},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return validateToken(opts.cfg)
		},
		Args: cobra.ExactArgs(2),
	}

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

	orbPack := &cobra.Command{
		Use:   "pack <path>",
		Short: "Pack an Orb with local scripts.",
		Long:  ``,
		RunE: func(_ *cobra.Command, _ []string) error {
			return packOrbCommand(opts)
		},
		Args: cobra.ExactArgs(1),
	}

	listCategoriesCommand := &cobra.Command{
		Use:   "list-categories",
		Short: "List orb categories",
		Args:  cobra.ExactArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			return listOrbCategories(opts)
		},
	}

	listCategoriesCommand.PersistentFlags().BoolVar(&opts.listJSON, "json", false, "print output as json instead of human-readable")
	if err := listCategoriesCommand.PersistentFlags().MarkHidden("json"); err != nil {
		panic(err)
	}

	addCategorizationToOrbCommand := &cobra.Command{
		Use:   "add-to-category <namespace>/<orb> \"<category-name>\"",
		Short: "Add an orb to a category",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, _ []string) error {
			return addOrRemoveOrbCategorization(opts, api.Add)
		},
	}

	removeCategorizationFromOrbCommand := &cobra.Command{
		Use:   "remove-from-category <namespace>/<orb> \"<category-name>\"",
		Short: "Remove an orb from a category",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, _ []string) error {
			return addOrRemoveOrbCategorization(opts, api.Remove)
		},
	}

	orbInit := &cobra.Command{
		Use:   "init <path>",
		Short: "Initialize a new orb.",
		Long:  ``,
		RunE: func(_ *cobra.Command, _ []string) error {
			return initOrb(opts)
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
			opts.cl = graphql.NewClient(config.Host, config.Endpoint, config.Token, config.Debug)

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
	orbCommand.AddCommand(unlistCmd)
	orbCommand.AddCommand(sourceCommand)
	orbCommand.AddCommand(orbInfoCmd)
	orbCommand.AddCommand(orbPack)
	orbCommand.AddCommand(addCategorizationToOrbCommand)
	orbCommand.AddCommand(removeCategorizationFromOrbCommand)
	orbCommand.AddCommand(listCategoriesCommand)
	orbCommand.AddCommand(orbInit)

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
		result += "\nSearch, filter, and view sources for all Orbs online at https://circleci.com/developer/orbs/"
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

func orbCategoryCollectionToString(orbCategoryCollection *api.OrbCategoriesForListing, opts orbOptions) (string, error) {
	var result string

	if opts.listJSON {
		orbCategoriesJSON, err := json.MarshalIndent(orbCategoryCollection, "", "  ")
		if err != nil {
			return "", errors.Wrapf(err, "Failed to convert to JSON")
		}
		result = string(orbCategoriesJSON)
	} else {
		var categories []string = make([]string, 0, len(orbCategoryCollection.OrbCategories))
		for _, orbCategory := range orbCategoryCollection.OrbCategories {
			categories = append(categories, orbCategory.Name)
		}
		result = strings.Join(categories, "\n")
	}

	return result, nil
}

func logOrbCategories(orbCategoryCollection *api.OrbCategoriesForListing, opts orbOptions) error {
	result, err := orbCategoryCollectionToString(orbCategoryCollection, opts)
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

func setOrbListStatus(opts orbOptions) error {
	ref := opts.args[0]
	unlistArg := opts.args[1]
	var err error

	namespace, orb, err := references.SplitIntoOrbAndNamespace(ref)

	if err != nil {
		return err
	}

	unlist, err := strconv.ParseBool(unlistArg)
	if err != nil {
		return fmt.Errorf("expected \"true\" or \"false\", got \"%s\"", unlistArg)
	}

	listed, err := api.OrbSetOrbListStatus(opts.cl, namespace, orb, !unlist)
	if err != nil {
		return err
	}

	if listed != nil {
		displayedStatus := "enabled"
		if !*listed {
			displayedStatus = "disabled"
		}
		fmt.Printf("The listing of orb `%s` is now %s.\n"+
			"Note: changes may not be immediately reflected in the registry.\n", ref, displayedStatus)
	} else {
		return fmt.Errorf("unexpected error in setting the list status of orb `%s`", ref)
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

	if len(info.Orb.Categories) > 0 {
		fmt.Println("")
		fmt.Println("## Categories:")
		for _, category := range info.Orb.Categories {
			fmt.Printf("%s\n", category.Name)
		}
	}

	orbVersionSplit := strings.Split(ref, "@")
	orbRef := orbVersionSplit[0]
	fmt.Printf(`
Learn more about this orb online in the CircleCI Orb Registry:
https://circleci.com/developer/orbs/orb/%s
`, orbRef)

	return nil
}

func listOrbCategories(opts orbOptions) error {
	orbCategories, err := api.ListOrbCategories(opts.cl)
	if err != nil {
		return errors.Wrapf(err, "Failed to list orb categories")
	}

	return logOrbCategories(orbCategories, opts)
}

func addOrRemoveOrbCategorization(opts orbOptions, updateType api.UpdateOrbCategorizationRequestType) error {
	var err error

	namespace, orb, err := references.SplitIntoOrbAndNamespace(opts.args[0])

	if err != nil {
		return err
	}

	err = api.AddOrRemoveOrbCategorization(opts.cl, namespace, orb, opts.args[1], updateType)

	if err != nil {
		var errorString = "Failed to add orb %s to category %s"
		if updateType == api.Remove {
			errorString = "Failed to remove orb %s from category %s"
		}
		return errors.Wrapf(err, errorString, opts.args[0], opts.args[1])
	}

	var output = `%s is successfully added to the "%s" category.` + "\n"
	if updateType == api.Remove {
		output = `%s is successfully removed from the "%s" category.` + "\n"
	}

	fmt.Printf(output, opts.args[0], opts.args[1])

	return nil
}

type OrbSchema struct {
	Version     float32                  `yaml:"version,omitempty"`
	Description string                   `yaml:"description,omitempty"`
	Display     yaml.Node                `yaml:"display,omitempty"`
	Orbs        yaml.Node                `yaml:"orbs,omitempty"`
	Commands    yaml.Node                `yaml:"commands,omitempty"`
	Executors   yaml.Node                `yaml:"executors,omitempty"`
	Jobs        yaml.Node                `yaml:"jobs,omitempty"`
	Examples    map[string]ExampleSchema `yaml:"examples,omitempty"`
}

type ExampleUsageSchema struct {
	Version   string      `yaml:"version,omitempty"`
	Orbs      interface{} `yaml:"orbs,omitempty"`
	Jobs      interface{} `yaml:"jobs,omitempty"`
	Workflows interface{} `yaml:"workflows"`
}

type ExampleSchema struct {
	Description string             `yaml:"description,omitempty"`
	Usage       ExampleUsageSchema `yaml:"usage,omitempty"`
	Result      ExampleUsageSchema `yaml:"result,omitempty"`
}

func packOrbCommand(opts orbOptions) error {
	result, err := packOrb(opts.args[0])
	if err != nil {
		return err
	}

	fmt.Println(result)

	return nil
}

func packOrb(path string) (string, error) {
	// Travel our Orb and build a tree from the YAML files.
	// Non-YAML files will be ignored here.
	_, err := os.Stat(filepath.Join(path, "@orb.yml"))
	if err != nil {
		return "", errors.New("@orb.yml file not found, are you sure this is the Orb root?")
	}

	tree, err := filetree.NewTree(path, "executors", "jobs", "commands", "examples")
	if err != nil {
		return "", errors.Wrap(err, "An unexpected error occurred")
	}

	y, err := yaml.Marshal(&tree)
	if err != nil {
		return "", errors.Wrap(err, "An unexpected error occurred")
	}

	var orbSchema OrbSchema
	err = yaml.Unmarshal(y, &orbSchema)
	if err != nil {
		return "", errors.Wrap(err, "An unexpected error occurred")
	}

	err = func(nodes ...*yaml.Node) error {
		for _, node := range nodes {
			err = inlineIncludes(node, path)
			if err != nil {
				return errors.Wrap(err, "An unexpected error occurred")
			}
		}
		return nil
	}(&orbSchema.Jobs, &orbSchema.Commands, &orbSchema.Executors)
	if err != nil {
		return "", err
	}

	final, err := yaml.Marshal(&orbSchema)
	if err != nil {
		return "", errors.Wrap(err, "Failed trying to marshal Orb YAML")
	}

	return string(final), nil
}

// Travel down a YAML node, replacing values as we go.
func inlineIncludes(node *yaml.Node, orbRoot string) error {
	// If we're dealing with a ScalarNode, we can replace the contents.
	// Otherwise, we recurse into the children of the Node in search of
	// a matching regex.
	if node.Kind == yaml.ScalarNode && node.Value != "" {
		v, err := process.MaybeIncludeFile(node.Value, orbRoot)
		if err != nil {
			return err
		}
		node.Value = v
	} else {
		// I am *slightly* worried about performance related to this approach, but don't have any
		// larger Orbs to test against.
		for _, child := range node.Content {
			err := inlineIncludes(child, orbRoot)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func initOrb(opts orbOptions) error {
	orbPath := opts.args[0]
	var err error
	fmt.Println("Note: This command is in preview. Please report any bugs! https://github.com/CircleCI-Public/circleci-cli/issues/new/choose")

	fullyAutomated := promptui.Select{
		Label: "Would you like to perform an automated setup of this orb?",
		Items: []string{"Yes, walk me through the process.", "No, just download the template."},
	}

	index, _, err := fullyAutomated.Run()
	if err != nil {
		return errors.Wrap(err, "Unexpected error")
	}

	fmt.Printf("Downloading Orb Project Template into %s\n", orbPath)
	httpClient := http.Client{}
	req, err := httpClient.Get("https://api.github.com/repos/CircleCI-Public/Orb-Project-Template/tags")
	if err != nil {
		return errors.Wrap(err, "Unexpected error")
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return errors.Wrap(err, "Unexpected error")
	}
	tags := []orbProtectTemplateRelease{}
	err = json.Unmarshal(body, &tags)
	if err != nil {
		return errors.Wrap(err, "Unexpected error")
	}

	latestTag := tags[0].ZipUrl
	resp, err := http.Get(latestTag)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath.Join(os.TempDir(), "orb-project-template.zip"))
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	err = unzipToOrbPath(filepath.Join(os.TempDir(), "orb-project-template.zip"), orbPath)
	if err != nil {
		return err
	}

	if index == 1 {
		fmt.Println("Opted for manual setup, exiting")
		fmt.Printf("The Orb Project Template has been extracted to %s\n", orbPath)
		return nil
	}

	fmt.Println("A few questions to get you up and running.")

	vcsProvider := ""
	useDefaultVcs := false
	if opts.cfg.OrbPublishing.DefaultVcsProvider != "" {
		useDefaultVcsPrompt := promptui.Select{
			Label: fmt.Sprintf("Use %s?", opts.cfg.OrbPublishing.DefaultVcsProvider),
			Items: []string{fmt.Sprintf("Yes, use %s", opts.cfg.OrbPublishing.DefaultVcsProvider), "No, use a different provider."},
		}
		useDefault, _, err := useDefaultVcsPrompt.Run()
		if err != nil {
			return err
		}
		useDefaultVcs = useDefault == 0
		if useDefaultVcs {
			vcsProvider = opts.cfg.OrbPublishing.DefaultVcsProvider
		}
	}

	if !useDefaultVcs {
		vcsSelect := promptui.Select{
			Label: "Are you using GitHub or Bitbucket?",
			Items: []string{"GitHub", "Bitbucket"},
		}
		vcsOption, _, err := vcsSelect.Run()
		if err != nil {
			return err
		}
		if vcsOption == 0 {
			vcsProvider = "github"
		} else {
			vcsProvider = "bitbucket"
		}
	}

	ownerNameInput := promptui.Prompt{
		Label:   fmt.Sprintf("Enter your %s username or organization", vcsProvider),
		Default: opts.cfg.OrbPublishing.DefaultOwner,
	}
	ownerName, err := ownerNameInput.Run()
	if err != nil {
		return errors.Wrap(err, "Unexpected error")
	}

	namespace := ownerName
	if opts.cfg.OrbPublishing.DefaultNamespace != "" {
		namespace = opts.cfg.OrbPublishing.DefaultNamespace
	}
	namespaceInput := promptui.Prompt{
		Label:   "Enter the namespace to use for this orb",
		Default: namespace,
	}
	namespace, err = namespaceInput.Run()
	if err != nil {
		return errors.Wrap(err, "Unexpected error")
	}

	fmt.Printf("Saving namespace %s as default\n", namespace)
	opts.cfg.OrbPublishing.DefaultNamespace = namespace
	_, err = api.GetNamespace(opts.cl, namespace)
	if err != nil {
		fmt.Println("Namespace does not exist, attempting to create it...")
		_, err = api.CreateNamespace(opts.cl, namespace, ownerName, vcsProvider)
		if err != nil {
			return err
		}
	}

	orbPathSplit := strings.Split(orbPath, "/")
	orbName := orbPathSplit[len(orbPathSplit)-1]
	orbNamePrompt := promptui.Prompt{
		Label:     "Orb name",
		Default:   orbName,
		AllowEdit: true,
	}
	orbName, err = orbNamePrompt.Run()
	if err != nil {
		return errors.Wrap(err, "Unexpected error")
	}

	contextPrompt := promptui.Select{
		Label: "Would you like to automatically set up a publishing context for your orb?",
		Items: []string{"Yes, set up a publishing context with my API key.", "No, I'll do this later."},
	}
	shouldCreateContext, _, err := contextPrompt.Run()
	if err != nil {
		return err
	}

	gitActionPrompt := promptui.Select{
		Label: "Would you like to set up your git project?",
		Items: []string{"Yes, set up the git project.", "No, I'll do this later."},
	}
	gitAction, _, err := gitActionPrompt.Run()
	if err != nil {
		return err
	}

	vcsShort := func() string {
		vcs := "gh"
		if vcsProvider == "bitbucket" {
			vcs = "bb"
		}
		return vcs
	}()

	if gitAction == 1 {
		err = finalizeOrbInit(ownerName, vcsProvider, vcsShort, namespace, orbName, "", &opts)
		if err != nil {
			return err
		}
		return nil
	}

	gitLocationPrompt := promptui.Prompt{
		Label: "Enter the remote git repository",
	}
	gitLocation, err := gitLocationPrompt.Run()
	if err != nil {
		return err
	}
	fmt.Println("Thank you! Setting up your orb...")

	if shouldCreateContext == 0 {
		contextGql := api.NewContextGraphqlClient(opts.cfg.Host, opts.cfg.Endpoint, opts.cfg.Token, opts.cfg.Debug)
		err = contextGql.CreateContext(vcsProvider, ownerName, "orb-publishing")
		if err != nil {
			return err
		}
		ctx, err := contextGql.ContextByName(vcsProvider, ownerName, "orb-publishing")
		if err != nil {
			return err
		}
		err = contextGql.CreateEnvironmentVariable(ctx.ID, "CIRCLE_TOKEN", opts.cfg.Token)
		if err != nil {
			return err
		}
	}

	projectName := func() string {
		x := strings.Split(gitLocation, "/")
		y := strings.Split(x[len(x)-1], ".")
		return y[0]
	}()

	circleConfig, err := ioutil.ReadFile(path.Join(orbPath, ".circleci", "config.yml"))
	if err != nil {
		return err
	}

	circle := string(circleConfig)
	err = ioutil.WriteFile(path.Join(orbPath, ".circleci", "config.yml"), []byte(orbTemplate(circle, projectName, ownerName, orbName, namespace)), 0644)
	if err != nil {
		return err
	}

	readme, err := ioutil.ReadFile(path.Join(orbPath, "README.md"))
	if err != nil {
		return err
	}
	readmeString := string(readme)
	err = ioutil.WriteFile(path.Join(orbPath, "README.md"), []byte(orbTemplate(readmeString, projectName, ownerName, orbName, namespace)), 0644)
	if err != nil {
		return err
	}

	r, err := git.PlainInit(orbPath, false)
	if err != nil {
		return err
	}

	_, err = r.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{gitLocation},
	})
	if err != nil {
		return err
	}
	err = r.CreateBranch(&config.Branch{
		Name:   "master",
		Remote: "origin",
	})
	if err != nil {
		return errors.Wrap(err, "Git error")
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	_, err = w.Add(".")
	if err != nil {
		return err
	}
	_, err = w.Commit("[semver:skip] Initial commit.", &git.CommitOptions{})
	if err != nil {
		return err
	}

	// Push a dev version of the orb.
	newOrb, err := api.CreateOrb(opts.cl, namespace, orbName)
	if err != nil {
		return errors.Wrap(err, "Unable to create orb")
	}
	packedOrb, err := packOrb(filepath.Join(orbPath, "src"))
	if err != nil {
		return err
	}

	tempOrbDir := filepath.Join(os.TempDir(), "_packed_orb_"+orbName)
	err = os.Mkdir(tempOrbDir, 0755)
	if err != nil {
		return errors.Wrap(err, "Unable to write packed orb")
	}

	tempOrbFile := filepath.Join(tempOrbDir, "orb.yml")
	err = ioutil.WriteFile(tempOrbFile, []byte(packedOrb), 0644)
	if err != nil {
		return errors.Wrap(err, "Unable to write packed orb")
	}

	_, err = api.OrbPublishByID(opts.cl, tempOrbFile, newOrb.CreateOrb.Orb.ID, "dev:alpha")
	if err != nil {
		return err
	}

	fmt.Println("An initial commit has been created - please run \033[1;34m'git push origin master'\033[0m to publish your first commit!")
	confirmGitPush := promptui.Select{
		Label: "I have pushed to my git repository using the above command",
		Items: []string{"Done"},
	}
	_, _, err = confirmGitPush.Run()
	if err != nil {
		return err
	}

	fr, err := api.FollowProject(opts.cfg.Host, vcsShort, ownerName, projectName, opts.cfg.Token)
	if err != nil {
		return err
	}
	if fr.Followed {
		fmt.Println("Project has been followed on CircleCI.")
	} else if fr.Message == "Project not found" {
		fmt.Println("Unable to determine project slug for CircleCI (slug is case sensitive).")
	}
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("alpha"),
		Create: true,
	})
	if err != nil {
		return errors.Wrap(err, "Unable to create alpha branch")
	}
	err = finalizeOrbInit(ownerName, vcsProvider, vcsShort, namespace, orbName, projectName, &opts)
	if err != nil {
		return err
	}
	return nil
}

func finalizeOrbInit(ownerName string, vcsProvider string, vcsShort string, namespace string, orbName string, projectName string, opts *orbOptions) error {
	opts.cfg.OrbPublishing.DefaultOwner = ownerName
	opts.cfg.OrbPublishing.DefaultVcsProvider = vcsProvider
	err := opts.cfg.WriteToDisk()
	if err != nil {
		return err
	}
	if projectName != "" {
		fmt.Printf("Your orb project is building here: https://circleci.com/%s/%s/%s\n", vcsShort, ownerName, projectName)
		fmt.Println("You are now working in the alpha branch.")
	}
	fmt.Printf("Once the first public version is published, you'll be able to see it here: https://circleci.com/developer/orbs/orb/%s/%s\n", namespace, orbName)
	fmt.Println("View orb publishing doc: https://circleci.com/docs/2.0/orb-author")
	return nil
}

// From https://stackoverflow.com/questions/20357223/easy-way-to-unzip-file-with-golang
func unzipToOrbPath(src, dest string) (error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	err = os.MkdirAll(dest, 0755)
	if err != nil {
		return err
	}
	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		// This is neccesary because the zip downloaded from GitHub will have a
		// directory with the actual template, rather than the template being
		// top-level.
		pathParts := strings.Split(f.Name, string(os.PathSeparator))
		pathParts = append([]string{dest}, pathParts[1:]...)
		path := filepath.Join(pathParts...)

		if f.FileInfo().IsDir() {
			err = os.MkdirAll(path, f.Mode())
			if err != nil {
				return err
			}
		} else {
			err = os.MkdirAll(filepath.Dir(path), f.Mode())
			if err != nil {
				return err
			}
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func orbTemplate(fileContents string, projectName string, orgName string, orbName string, namespace string) string {
	x := strings.Replace(fileContents, "<orb-name>", orbName, -1)
	x = strings.Replace(x, "<namespace>", namespace, -1)
	x = strings.Replace(x, "<publishing-context>", "orb-publishing", -1)
	x = strings.Replace(x, "<project-name>", projectName, -1)
	x = strings.Replace(x, "<organization>", orgName, -1)
	x = strings.Replace(x, "<!---", "", -1)
	x = strings.Replace(x, "--->", "", -1)
	var re = regexp.MustCompile(`\*\*Meta\*\*.*`)
	x = re.ReplaceAllString(x, "")

	return x
}
