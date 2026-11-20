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

// Package runnerinstall holds the steps for installing a CircleCI self-hosted
// runner on each supported platform, as documented at
// https://circleci.com/docs/runner/runner-overview/.
//
// It renders instructions only. Nothing here runs a command, so the CLI never
// needs root and never changes the machine it runs on: the user decides where
// the agent is installed, which is often not the machine running the CLI.
//
// Every command refers to the runner token through the RUNNER_AUTH_TOKEN
// environment variable rather than embedding it, so the instructions are safe
// to paste into a ticket or a log and the token stays out of shell history.
package runnerinstall

import (
	"fmt"
	"strings"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
)

// Platform is somewhere a runner agent can be installed.
type Platform string

const (
	// Debian is machine runner 3 on Debian or Ubuntu, from the apt package.
	Debian Platform = "debian"
	// RPM is machine runner 3 on RHEL, CentOS or Fedora, from the yum package.
	RPM Platform = "rpm"
	// MacOS is machine runner 3 on macOS, from Homebrew.
	MacOS Platform = "macos"
	// Windows is machine runner 3 on Windows Server, from the PowerShell installer.
	Windows Platform = "windows"
	// Docker is machine runner 3 in a container.
	Docker Platform = "docker"
	// Kubernetes is container runner, from the container-agent Helm chart.
	Kubernetes Platform = "kubernetes"
)

// TokenEnvVar is the environment variable every generated command reads the
// runner token from.
const TokenEnvVar = "RUNNER_AUTH_TOKEN" //nolint:gosec // the variable's name, not a credential

// platformInfo is a platform's flag value and the label shown in the prompt.
type platformInfo struct {
	Platform Platform
	Label    string
}

var platforms = []platformInfo{
	{Debian, "Linux: Debian or Ubuntu (apt)"},
	{RPM, "Linux: RHEL, CentOS or Fedora (yum)"},
	{MacOS, "macOS (Homebrew)"},
	{Windows, "Windows Server"},
	{Docker, "Docker"},
	{Kubernetes, "Kubernetes (container runner, Helm)"},
}

// Platforms returns the accepted --platform values, in prompt order.
func Platforms() []string {
	out := make([]string, len(platforms))
	for i, p := range platforms {
		out[i] = string(p.Platform)
	}
	return out
}

// Labels returns the prompt label for each entry of Platforms, in the same order.
func Labels() []string {
	out := make([]string, len(platforms))
	for i, p := range platforms {
		out[i] = p.Label
	}
	return out
}

// Parse validates a --platform value.
func Parse(v string) (Platform, *clierrors.CLIError) {
	for _, p := range platforms {
		if v == string(p.Platform) {
			return p.Platform, nil
		}
	}
	return "", clierrors.New("runner.invalid_platform", "Invalid --platform value",
		fmt.Sprintf("%q is not a platform a runner can be installed on.", v)).
		WithSuggestions("Use one of: " + strings.Join(Platforms(), ", ")).
		WithExitCode(clierrors.ExitBadArguments)
}

// Step is one stage of an install: a title, the commands to run, and an
// optional note for anything a command cannot express.
type Step struct {
	Title    string   `json:"title"`
	Commands []string `json:"commands,omitempty"`
	Note     string   `json:"note,omitempty"`
}

// Guide is the full set of instructions for one platform.
type Guide struct {
	Platform Platform `json:"platform"`
	Title    string   `json:"title"`
	Steps    []Step   `json:"steps"`
	// JobConfig is a .circleci/config.yml that runs a job on the new runner.
	JobConfig string `json:"job_config"`
	// DocsURL is the CircleCI documentation page this guide follows.
	DocsURL string `json:"docs_url"`
}

// Options is the input to Build.
type Options struct {
	// ResourceClass is the runner resource class, as "namespace/name".
	ResourceClass string
}

const docsRoot = "https://circleci.com/docs/runner/"

// Build returns the install guide for platform p.
func Build(p Platform, opts Options) (Guide, error) {
	g := Guide{Platform: p, JobConfig: jobConfig(p, opts.ResourceClass)}

	switch p {
	case Debian:
		g.Title = "Install machine runner 3 on Debian or Ubuntu"
		g.DocsURL = docsRoot + "install-machine-runner-3-on-linux/"
		g.Steps = linuxSteps(
			"curl -s https://packagecloud.io/install/repositories/circleci/runner/script.deb.sh?any=true | sudo bash",
			"sudo apt-get install -y circleci-runner",
		)
	case RPM:
		g.Title = "Install machine runner 3 on RHEL, CentOS or Fedora"
		g.DocsURL = docsRoot + "install-machine-runner-3-on-linux/"
		g.Steps = linuxSteps(
			"curl -s https://packagecloud.io/install/repositories/circleci/runner/script.rpm.sh?any=true | sudo bash",
			"sudo yum install circleci-runner -y",
		)
	case MacOS:
		g.Title = "Install machine runner 3 on macOS"
		g.DocsURL = docsRoot + "install-machine-runner-3-on-macos/"
		g.Steps = macosSteps()
	case Windows:
		g.Title = "Install machine runner 3 on Windows Server"
		g.DocsURL = docsRoot + "install-machine-runner-3-on-windows/"
		g.Steps = windowsSteps()
	case Docker:
		g.Title = "Run machine runner 3 in Docker"
		g.DocsURL = docsRoot + "install-machine-runner-3-on-docker/"
		g.Steps = dockerSteps()
	case Kubernetes:
		g.Title = "Install container runner on Kubernetes"
		g.DocsURL = docsRoot + "container-runner-installation/"
		g.Steps = kubernetesSteps(opts.ResourceClass)
	default:
		_, err := Parse(string(p))
		return Guide{}, err
	}
	return g, nil
}

func exportToken() string {
	return fmt.Sprintf(`export %s="<the token printed above>"`, TokenEnvVar)
}

const serverNote = "On CircleCI server, point the agent at your server: set the url field " +
	"under api: in the config, or CIRCLECI_RUNNER_API_URL in the environment."

func linuxSteps(addRepo, install string) []Step {
	const cfg = "/etc/circleci-runner/circleci-runner-config.yaml"
	return []Step{
		{Title: "Add the CircleCI package repository", Commands: []string{addRepo}},
		{Title: "Install the runner", Commands: []string{install}},
		{
			Title: "Put the token in the runner config",
			Commands: []string{
				exportToken(),
				fmt.Sprintf(`sudo sed -i "s/<< AUTH_TOKEN >>/$%s/g" %s`, TokenEnvVar, cfg),
			},
			Note: serverNote,
		},
		{
			Title: "Start the runner and check it is running",
			Commands: []string{
				"sudo systemctl enable circleci-runner && sudo systemctl start circleci-runner",
				"sudo systemctl status circleci-runner",
			},
			Note: "Logs: journalctl -u circleci-runner -f",
		},
	}
}

func macosSteps() []Step {
	const cfgDir = `$HOME/Library/Preferences/com.circleci.runner`
	return []Step{
		{
			Title: "Install the runner with Homebrew",
			Commands: []string{
				"brew tap circleci-public/circleci",
				"brew install circleci-runner",
			},
		},
		{
			Title: "Write the runner config",
			Commands: []string{
				exportToken(),
				fmt.Sprintf(`mkdir -p "%s"`, cfgDir),
				fmt.Sprintf("cat > \"%s/config.yaml\" <<EOF\n"+
					"runner:\n"+
					"  name: \"my-macos-runner\"\n"+
					"  working_directory: \"/Users/$USER/Library/com.circleci.runner/workdir\"\n"+
					"  cleanup_working_directory: true\n"+
					"api:\n"+
					"  auth_token: \"$%s\"\n"+
					"EOF", cfgDir, TokenEnvVar),
			},
			Note: serverNote,
		},
		{
			Title: "Allow the notarized binary to run",
			Commands: []string{
				`spctl -a -vvv -t install "$(brew --prefix)/bin/circleci-runner"`,
				`sudo xattr -r -d com.apple.quarantine "$(brew --prefix)/bin/circleci-runner"`,
			},
		},
		{
			Title: "Start the runner",
			Commands: []string{
				"launchctl bootstrap gui/$(id -u) $HOME/Library/LaunchAgents/com.circleci.runner.plist",
				"launchctl enable gui/$(id -u)/com.circleci.runner",
				"launchctl kickstart -k gui/$(id -u)/com.circleci.runner",
			},
			Note: "For a headless or remote session, load it in the user domain instead; see the docs page below.",
		},
		{
			Title:    "Check it is running",
			Commands: []string{"launchctl print gui/$(id -u)/com.circleci.runner"},
			Note:     "Logs: $HOME/Library/Logs/com.circleci.runner/runner.log",
		},
	}
}

func windowsSteps() []Step {
	return []Step{
		{
			Title: "Download the installer",
			Note: "Download Install-CircleCIRunner.ps1 from " +
				"https://github.com/CircleCI-Public/runner-installation-files/tree/main/windows-install/circleci-runner. " +
				"The server must be Windows Server 2019, 2022 or 2025, and not a domain controller.",
		},
		{
			Title: "Run the installer from an administrator PowerShell",
			Commands: []string{
				"Set-ExecutionPolicy Bypass -Scope Process -Force; " +
					"[System.Net.ServicePointManager]::SecurityProtocol = " +
					"[System.Net.ServicePointManager]::SecurityProtocol -bor 3072; " +
					"./Install-CircleCIRunner.ps1",
			},
		},
		{
			Title: "Fill in the config the installer opens",
			Note: "The installer opens runner-agent-config.yaml in Notepad. Set api.auth_token to the " +
				"token printed above (and runner.name if you want a custom name). " +
				"The runner starts automatically afterwards.",
		},
	}
}

func dockerSteps() []Step {
	return []Step{
		{
			Title: "Run the runner container",
			Commands: []string{
				exportToken(),
				fmt.Sprintf("docker run -d --name circleci-runner "+
					"--env CIRCLECI_RUNNER_API_AUTH_TOKEN=$%s "+
					"--env CIRCLECI_RUNNER_NAME=my-docker-runner "+
					"circleci/runner-agent:machine-3", TokenEnvVar),
			},
			Note: serverNote + " To add tools to the image, build your own FROM circleci/runner-agent:machine-3.",
		},
		{
			Title:    "Check it is running",
			Commands: []string{"docker logs -f circleci-runner"},
			Note:     "Stop it with: docker stop circleci-runner",
		},
	}
}

func kubernetesSteps(resourceClass string) []Step {
	return []Step{
		{
			Title: "Add the container-agent Helm repository",
			Commands: []string{
				"helm repo add container-agent https://packagecloud.io/circleci/container-agent/helm",
				"helm repo update",
				"kubectl create namespace circleci",
			},
		},
		{
			Title: "Generate the Helm values",
			Commands: []string{
				exportToken(),
				fmt.Sprintf(`circleci runner config %s --product container --token "$%s" --output values.yaml`,
					resourceClass, TokenEnvVar),
			},
			Note: "On CircleCI server, also set agent.runnerAPI in values.yaml to your server's URL.",
		},
		{
			Title:    "Install container runner",
			Commands: []string{"helm install container-agent container-agent/container-agent -n circleci -f values.yaml"},
		},
	}
}

// jobConfig returns a minimal .circleci/config.yml that runs one job on the
// new resource class. Container runner takes a docker executor; every machine
// runner takes machine: true.
func jobConfig(p Platform, resourceClass string) string {
	executor := "    machine: true\n"
	if p == Kubernetes {
		executor = "    docker:\n      - image: cimg/base:current\n"
	}
	return "version: 2.1\n\n" +
		"jobs:\n" +
		"  runner:\n" +
		executor +
		"    resource_class: " + resourceClass + "\n" +
		"    steps:\n" +
		"      - run: echo \"Hi I'm on Runners!\"\n\n" +
		"workflows:\n" +
		"  build-workflow:\n" +
		"    jobs:\n" +
		"      - runner\n"
}
