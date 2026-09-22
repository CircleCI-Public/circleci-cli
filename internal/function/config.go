// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

package function

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
)

// blockKey is the top-level config key holding function declarations.
const blockKey = "functions"

// Pin is one entry in the functions: block. Alias is the key, and the name a
// step uses to invoke the function.
type Pin struct {
	Alias    string `json:"alias"`
	Function string `json:"function"`
	Version  string `json:"version"`
}

// ListPins returns every entry in the functions: block, in config order.
//
// A functions: key that is not a map of declarations yields nothing rather than
// an error, so a config using the name to hold YAML anchors keeps working.
func ListPins(configPath string) ([]Pin, error) {
	doc, err := readConfig(configPath)
	if err != nil {
		return nil, err
	}

	block := resolve(findMappingValue(doc, blockKey))
	if block == nil || block.Kind != yaml.MappingNode {
		return nil, nil
	}

	pins := make([]Pin, 0, len(block.Content)/2)
	for i := 0; i+1 < len(block.Content); i += 2 {
		value := resolve(block.Content[i+1])
		if value == nil || value.Kind != yaml.ScalarNode {
			continue
		}
		pin := Pin{Alias: block.Content[i].Value, Function: value.Value}
		if path, version, ok := splitReference(value.Value); ok {
			pin.Function, pin.Version = path, version
		}
		pins = append(pins, pin)
	}
	return pins, nil
}

// splitReference reads a "<path>@<version>" value. A malformed value is
// reported as-is rather than hidden: config validation will reject it too.
func splitReference(value string) (path, version string, ok bool) {
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] != '@' {
			continue
		}
		path, version = value[:i], value[i+1:]
		if ValidatePath(path) == nil && ValidateVersion(version) == nil {
			return path, version, true
		}
		return "", "", false
	}
	return "", "", false
}

func readConfig(configPath string) (*yaml.Node, error) {
	data, err := os.ReadFile(configPath) //nolint:gosec // configPath is user-supplied via --config
	if err != nil {
		if os.IsNotExist(err) {
			return nil, clierrors.New("function.config_not_found", "Config file not found",
				fmt.Sprintf("No config file at %q.", configPath)).
				WithSuggestions(
					"Run this from a repository with a .circleci/config.yml",
					"Point at the file explicitly: --config path/to/config.yml",
				).
				WithExitCode(clierrors.ExitNotFound)
		}
		return nil, err
	}

	var parsed yaml.Node
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, clierrors.New("function.config_invalid", "Config file could not be parsed",
			fmt.Sprintf("%s is not valid YAML: %s", configPath, err)).
			WithSuggestions("Run 'circleci config validate' to see what is wrong").
			WithExitCode(clierrors.ExitValidationFail)
	}
	if len(parsed.Content) == 0 {
		return nil, clierrors.New("function.config_empty", "Config file is empty",
			fmt.Sprintf("%s has no content.", configPath)).
			WithExitCode(clierrors.ExitValidationFail)
	}
	return parsed.Content[0], nil
}

func findMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// resolve follows an anchor reference to the node it points at.
func resolve(n *yaml.Node) *yaml.Node {
	if n != nil && n.Kind == yaml.AliasNode {
		return n.Alias
	}
	return n
}
