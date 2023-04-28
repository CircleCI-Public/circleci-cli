package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func printValues(values Values) {
	// Provide a stable sort order for printed values
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		fmt.Fprintf(os.Stderr, "%-18s %v\n", key+":", values[key])
	}

	// Add empty newline at end
	fmt.Fprintf(os.Stderr, "\n")
}

type ProcessConfigOpts struct {
	ConfigPath             string
	OrgID                  string
	OrgSlug                string
	PipelineParamsFilePath string

	VerboseOutput bool
}

func (c *ConfigCompiler) getOrgID(
	optsOrgID string,
	optsOrgSlug string,
) (string, error) {
	if strings.TrimSpace(optsOrgID) != "" {
		return optsOrgID, nil
	}

	if strings.TrimSpace(optsOrgSlug) == "" {
		return "", nil
	}

	coll, err := c.collaborators.GetCollaborationBySlug(optsOrgSlug)

	if err != nil {
		return "", err
	}

	if coll == nil {
		fmt.Println("Could not fetch a valid org-id from collaborators endpoint.")
		fmt.Println("Check if you have access to this org by hitting https://circleci.com/api/v2/me/collaborations")
		fmt.Println("Continuing on - private orb resolution will not work as intended")

		return "", nil
	}

	return coll.OrgId, nil
}

func (c *ConfigCompiler) ProcessConfig(opts ProcessConfigOpts) error {
	var response *ConfigResponse
	var params Parameters
	var err error

	if len(opts.PipelineParamsFilePath) > 0 {
		// The 'src' value can be a filepath, or a yaml string. If the file cannot be read successfully,
		// proceed with the assumption that the value is already valid yaml.
		raw, err := os.ReadFile(opts.PipelineParamsFilePath)
		if err != nil {
			raw = []byte(opts.PipelineParamsFilePath)
		}

		err = yaml.Unmarshal(raw, &params)
		if err != nil {
			return fmt.Errorf("invalid 'pipeline-parameters' provided: %s", err.Error())
		}
	}

	//if no orgId provided use org slug
	values := LocalPipelineValues()
	if opts.VerboseOutput {
		fmt.Fprintln(os.Stderr, "Processing config with following values:")
		printValues(values)
	}

	orgID, err := c.getOrgID(opts.OrgID, opts.OrgSlug)
	if err != nil {
		return fmt.Errorf("failed to get the appropriate org-id: %s", err.Error())
	}

	response, err = c.ConfigQuery(
		opts.ConfigPath,
		orgID,
		params,
		values,
	)
	if err != nil {
		return err
	}

	if !response.Valid {
		fmt.Println(response.Errors)
		return errors.New("config is invalid")
	}

	fmt.Print(response.OutputYaml)
	return nil
}

type ValidateConfigOpts struct {
	ConfigPath string
	OrgID      string
	OrgSlug    string

	IgnoreDeprecatedImages bool
	VerboseOutput          bool
}

// The <path> arg is actually optional, in order to support compatibility with the --path flag.
func (c *ConfigCompiler) ValidateConfig(opts ValidateConfigOpts) error {
	var err error
	var response *ConfigResponse

	//if no orgId provided use org slug
	values := LocalPipelineValues()
	if opts.VerboseOutput {
		fmt.Fprintln(os.Stderr, "Validating config with following values:")
		printValues(values)
	}

	orgID, err := c.getOrgID(opts.OrgID, opts.OrgSlug)
	if err != nil {
		return fmt.Errorf("failed to get the appropriate org-id: %s", err.Error())
	}

	response, err = c.ConfigQuery(
		opts.ConfigPath,
		orgID,
		nil,
		LocalPipelineValues(),
	)
	if err != nil {
		return err
	}

	if !response.Valid {
		fmt.Println(response.Errors)
		return errors.New("config is invalid")
	}

	// check if a deprecated Linux VM image is being used
	// link here to blog post when available
	// returns an error if a deprecated image is used
	if !opts.IgnoreDeprecatedImages {
		err := deprecatedImageCheck(response)
		if err != nil {
			return err
		}
	}

	fmt.Printf("Config file at %s is valid.\n", opts.ConfigPath)
	return nil
}
