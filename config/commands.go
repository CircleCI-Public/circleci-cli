package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func printValues(values Values) {
	for key, value := range values {
		fmt.Printf("\t%s:\t%s", key, value)
	}
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
	if optsOrgID == "" && optsOrgSlug == "" {
		fmt.Println("No org id or slug has been provided")
		return "", nil
	}

	var orgID string
	if strings.TrimSpace(optsOrgID) != "" {
		orgID = optsOrgID
	} else {
		orgs, err := c.GetOrgCollaborations()
		if err != nil {
			return "", err
		}
		orgID = GetOrgIdFromSlug(optsOrgSlug, orgs)
		if orgID == "" {
			fmt.Println("Could not fetch a valid org-id from collaborators endpoint.")
			fmt.Println("Check if you have access to this org by hitting https://circleci.com/api/v2/me/collaborations")
			fmt.Println("Continuing on - private orb resolution will not work as intended")
		}
	}

	return orgID, nil
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
		fmt.Println("Processing config with following values")
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
		fmt.Println("Validating config with following values")
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

	fmt.Printf("\nConfig file at %s is valid.\n", opts.ConfigPath)
	return nil
}
