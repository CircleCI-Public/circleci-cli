package config

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// CircleCI Linux VM images that will be permanently removed on May 31st.
var deprecatedImages = []string{
	"circleci/classic:201710-01",
	"circleci/classic:201703-01",
	"circleci/classic:201707-01",
	"circleci/classic:201708-01",
	"circleci/classic:201709-01",
	"circleci/classic:201710-02",
	"circleci/classic:201711-01",
	"circleci/classic",
	"circleci/classic:latest",
	"circleci/classic:edge",
	"circleci/classic:201808-01",
	"ubuntu-1604:201903-01",
	"ubuntu-1604:202004-01",
	"ubuntu-1604:202007-01",
	"ubuntu-1604:202010-01",
	"ubuntu-1604:202101-01",
	"ubuntu-1604:202104-01",
}

// Simplified Config -> job Structure for an image

type job struct {
	Machine interface{} `yaml:"machine"`
}

// Simplified Config Structure for an image
type processedConfig struct {
	Jobs map[string]job `yaml:"jobs"`
}

// Processes the config down to v2.0, then checks image used against the block list
func deprecatedImageCheck(response *ConfigResponse) error {
	aConfig := processedConfig{}
	err := yaml.Unmarshal([]byte(response.OutputYaml), &aConfig)
	if err != nil {
		return err
	}

	// check each job
	for key := range aConfig.Jobs {

		switch aConfig.Jobs[key].Machine.(type) {
		case bool, nil:
			// using machine true
			continue
		}

		image := aConfig.Jobs[key].Machine.(map[string]interface{})["image"]

		// using the `docker`/`xcode` executors
		if image == nil {
			continue
		}

		for _, v := range deprecatedImages {
			if image.(string) == v {
				return errors.New("The config is using a deprecated Linux VM image (" + v + "). Please see https://circleci.com/blog/ubuntu-14-16-image-deprecation/. This error can be ignored by using the '--ignore-deprecated-images' flag.")
			}
		}
	}

	return nil
}
