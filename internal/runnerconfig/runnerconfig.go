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

// Package runnerconfig renders the YAML a CircleCI self-hosted runner needs to
// start, for each runner product.
//
// The three products do not share a schema, which is why this package renders
// three different documents rather than one parameterised template:
//
//   - Machine runner 3 reads a circleci-runner-config.yaml, passed to the agent
//     as "circleci-runner machine -c <file>". Keys are snake_case.
//   - Container runner has no agent config file at all. Its agent is configured
//     by flags and environment variables supplied by the container-agent Helm
//     chart, so the file a user authors is a Helm values.yaml.
//   - Runner provisioner is likewise Helm-configured, under a different
//     top-level key, and renders its own ConfigMap and Secret from those values.
//
// Output is deterministic: struct field order controls layout and yaml.v3 sorts
// map keys alphabetically.
package runnerconfig

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
)

// Product is a runner product that `circleci runner config` can target.
type Product string

const (
	// Machine is machine runner 3: an agent config file.
	Machine Product = "machine"
	// Container is container runner: container-agent Helm values.
	Container Product = "container"
	// Provisioner is runner provisioner: runner-provisioner Helm values.
	Provisioner Product = "provisioner"
)

// Products are the accepted --product values. Machine is first so it is both
// the flag default and the preselected entry in the interactive prompt.
var Products = []string{string(Machine), string(Container), string(Provisioner)}

// Options is the input to Render. Name and WorkingDirectory apply to Machine
// only; the Helm products take nothing but the resource class and its token.
type Options struct {
	// ResourceClass is the runner resource class, as "namespace/name".
	ResourceClass string
	// Token is the resource class token the agent authenticates with.
	Token string
	// Name becomes runner.name. Required for Machine; callers that have no
	// explicit name should use DefaultName.
	Name string
	// WorkingDirectory becomes runner.working_directory. Defaults to
	// DefaultWorkingDirectory when empty.
	WorkingDirectory string
}

// ParseProduct validates a --product value.
func ParseProduct(v string) (Product, *clierrors.CLIError) {
	for _, valid := range Products {
		if v == valid {
			return Product(v), nil
		}
	}
	return "", clierrors.New("runner.invalid_product", "Invalid --product value",
		fmt.Sprintf("%q is not a runner product.", v)).
		WithSuggestions("Use one of: " + strings.Join(Products, ", ")).
		WithExitCode(clierrors.ExitBadArguments)
}

// Render returns the configuration document for product p.
func Render(p Product, opts Options) ([]byte, error) {
	if err := validateResourceClass(opts.ResourceClass); err != nil {
		return nil, err
	}
	if err := validateToken(opts.Token); err != nil {
		return nil, err
	}

	switch p {
	case Machine:
		return renderMachine(opts)
	case Container:
		return renderContainer(opts)
	case Provisioner:
		return renderProvisioner(opts)
	default:
		_, err := ParseProduct(string(p))
		return nil, err
	}
}

func validateResourceClass(rc string) *clierrors.CLIError {
	ns, name, found := strings.Cut(rc, "/")
	if found && ns != "" && name != "" && !strings.Contains(name, "/") {
		return nil
	}
	return clierrors.New("runner.invalid_resource_class", "Invalid resource class",
		fmt.Sprintf("%q is not a resource class.", rc)).
		WithSuggestions(
			"Use the form namespace/name, for example my-org/my-runner",
			"List your resource classes with: circleci runner resource-class list",
		).
		WithExitCode(clierrors.ExitBadArguments)
}

// validateToken rejects the placeholder the packaged config template ships with,
// which the agent refuses to start on.
func validateToken(token string) *clierrors.CLIError {
	if strings.TrimSpace(token) != "" && strings.TrimSpace(token) != "<< AUTH_TOKEN >>" {
		return nil
	}
	return clierrors.New("runner.invalid_token", "Invalid runner token",
		"The runner token is empty or a placeholder, which the agent rejects at startup.").
		WithSuggestions("Pass a resource class token with --token, or omit it to create one").
		WithExitCode(clierrors.ExitBadArguments)
}

// encode renders v as YAML behind a comment header. The agent treats an empty
// file as a parse error, so header must never be emitted on its own.
func encode(header string, v any) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(header)

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
