package policy

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circle-policy-agent/cpa"
)

//getPolicyDecisionLocally takes path of policy path/directory and input (eg build config) as string, and performs policy evaluation locally
func getPolicyDecisionLocally(policyPath, input string) (*cpa.Decision, error) {
	var config interface{}
	if err := yaml.Unmarshal([]byte(input), &config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	documentBundle, err := getDocumentBundleFromPath(policyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get document bundle for path: %w", err)
	}

	parsedPolicy, err := cpa.ParseBundle(documentBundle)
	if err != nil {
		return nil, fmt.Errorf("failed to parse policy bundle: %w", err)
	}

	ctx := context.Background()
	decision, err := parsedPolicy.Decide(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to make decision: %w", err)
	}

	return decision, nil
}

//getDocumentBundleFromPath takes policy path as an input which could be path to a file or directory, and returns a policy bundle
//if policyPath is a file, its content will be the only data in the decision bundle
//if policyPath is a directory, every file in the directory (non-recursive) will be considered a policy in the output policy bundle
func getDocumentBundleFromPath(policyPath string) (map[string]string, error) {
	documentBundle := map[string]string{}
	pathInfo, err := os.Stat(policyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get path info: %w", err)
	}

	//if policyPath is directory get content of all files in this directory (non-recursively) and add to document bundle
	if pathInfo.IsDir() {
		policyFiles, err := ioutil.ReadDir(policyPath) //get list of all files in given directory path
		if err != nil {
			return nil, fmt.Errorf("failed to get list of policy files: %w", err)
		}
		if len(policyFiles) == 0 {
			return nil, fmt.Errorf("no files found in: %s", policyPath)
		}
		for _, f := range policyFiles {
			if f.IsDir() {
				continue
			}
			filePath := filepath.Join(policyPath, f.Name()) //get absolute file path
			if err = setFileContentToMap(filePath, normalise(f.Name()), documentBundle); err != nil {
				return nil, err
			}
		}
		return documentBundle, nil
	}

	//if policyPath is a file, add this to the document bundle as the only file
	if err = setFileContentToMap(policyPath, normalise(pathInfo.Name()), documentBundle); err != nil {
		return nil, err
	}
	return documentBundle, nil
}

//setFileContentToMap Sets key(usually file-name) and value(content of given file(path)) to a given map
func setFileContentToMap(filePath string, key string, contentMap map[string]string) error {
	if key == "" {
		return fmt.Errorf("invalid key")
	}
	if contentMap == nil {
		return fmt.Errorf("uninitialized contentMap")
	}
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	contentMap[key] = string(fileContent)
	return nil
}

//normalise makes a string suitable to be used as a map key
func normalise(name string) string {
	var spaces = regexp.MustCompile(`\s+`)
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "-", "_")
	return spaces.ReplaceAllString(name, "_")
}
