package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/project"
	"github.com/CircleCI-Public/circleci-cli/errs"
	"github.com/CircleCI-Public/circleci-cli/git"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
)

func newRunCommand(config *settings.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run <name> [args...]",
		Short: "Execute a circleci plugin",
		Long: `Execute a circleci plugin by looking for a binary called circleci-<name> in your PATH.
This command implements a plugin system similar to git, where you can extend
circleci functionality by creating executables with the 'circleci-' prefix.

For example, if you have a binary called 'circleci-foo' in your PATH,
you can run it with: circleci run foo [args...]`,
		Example: `  circleci run foo --help
  circleci run my-plugin arg1 arg2`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// When flag parsing is disabled, we need to manually validate args
			if len(args) < 1 {
				return fmt.Errorf("requires at least 1 arg(s), only received %d", len(args))
			}

			// Handle help flags manually since flag parsing is disabled
			if args[0] == "--help" || args[0] == "-h" {
				return cmd.Help()
			}

			pluginName := args[0]
			pluginArgs := args[1:]

			projectClient, err := project.NewProjectRestClient(*config)
			if err != nil {
				return err
			}

			// Construct the plugin binary name
			binaryName := fmt.Sprintf("circleci-%s", pluginName)

			// Look for the binary in PATH
			binaryPath, err := exec.LookPath(binaryName)
			if err != nil {
				return errs.NotFoundf("plugin '%s' not found: could not find '%s' in PATH: %w", pluginName, binaryName, err)
			}

			var telemetryEnabled bool
			telemetryClient, ok := telemetry.FromContext(cmd.Context())
			if ok {
				telemetryEnabled = telemetryClient.Enabled()
			}

			// Create the command to execute the plugin
			pluginCmd := exec.Command(binaryPath, pluginArgs...)
			// Connect stdin, stdout, and stderr to the current process
			pluginCmd.Stdin = os.Stdin
			pluginCmd.Stdout = config.Stdout
			pluginCmd.Stderr = config.Stderr
			pluginCmd.Env = os.Environ()

			info := autoDetectProject(projectClient)
			configEnv := map[string]string{
				"CIRCLE_URL":               config.Host,
				"CIRCLE_TOKEN":             config.Token,
				"CIRCLE_TELEMETRY_ENABLED": fmt.Sprintf("%t", telemetryEnabled),
			}
			if info != nil {
				configEnv["CIRCLE_PROJECT_ID"] = info.Id
			}
			for k, v := range configEnv {
				pluginCmd.Env = append(pluginCmd.Env, fmt.Sprintf("%s=%s", k, v))
			}

			// Run the plugin
			if err := pluginCmd.Run(); err != nil {
				return fmt.Errorf("failed to execute plugin '%s': %w", pluginName, err)
			}

			return nil
		},
	}

	return runCmd
}

func autoDetectProject(projectClient project.ProjectClient) *project.ProjectInfo {
	remote, err := git.InferProjectFromGitRemotes()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "unable to autodetect project from git remotes")
		return nil
	}

	info, err := projectClient.ProjectInfo(strings.ToLower(string(remote.VcsType)), remote.Organization, remote.Project)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to get project info: %s\n", err)
		return nil
	}

	return info
}
