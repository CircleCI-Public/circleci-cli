package deploy

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/config"
	"github.com/CircleCI-Public/circleci-cli/git"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

// deploysDashboardURL is the base URL printed at the end of a
// successful init. We intentionally do not try to build an org-scoped
// URL here: doing so reliably across OAuth orgs (gh/bb slug) and
// standalone CircleCI orgs (opaque CIAM id) requires an authenticated
// API call, which would break the "works offline" design goal. The
// dashboard landing page redirects users to their most recent org.
const deploysDashboardURL = "https://app.circleci.com/deploys"

// deployMarkersDocsURL points to the canonical guide for deploy markers.
// We print it alongside the dashboard URL so users who outgrow the
// `log`-only setup this command produces (for example, users who want
// the full plan/update lifecycle with statuses) can find the next step.
const deployMarkersDocsURL = "https://circleci.com/docs/guides/deploy/configure-deploy-markers/"

type initOptions struct {
	// configPath points to the .circleci/config.yml the command reads
	// and writes back. Defaults to config.DefaultConfigPath; exposed as
	// a hidden flag so tests can redirect it at a temp file.
	configPath string
}

func newInitCommand(_ *settings.Config, deployOpts *deployOpts) *cobra.Command {
	iopts := initOptions{
		configPath: config.DefaultConfigPath,
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Wire deploy markers into your .circleci/config.yml",
		Long: `Scan the project's .circleci/config.yml, detect jobs that look like
deployments, and append a step that records a deploy marker on every run.

The command is idempotent: re-running it against an already instrumented
config leaves it untouched. It never commits or pushes any changes —
after it finishes you are prompted to review the diff and commit
yourself.`,
		Example: `  # Run from the root of your repository:
  circleci deploy init`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd.OutOrStdout(), deployOpts, iopts)
		},
	}

	cmd.Flags().StringVar(&iopts.configPath, "config", iopts.configPath, "Path to the config file to patch")
	_ = cmd.Flags().MarkHidden("config")

	return cmd
}

// runInit drives the whole interactive flow: read config → detect
// deploy jobs → gather component/environment answers → patch config →
// print follow-up instructions. It always writes through the supplied
// writer so tests can capture the output.
func runInit(out io.Writer, deployOpts *deployOpts, iopts initOptions) error {
	fmt.Fprintf(out, "Scanning %s...\n\n", iopts.configPath)

	_, root, err := ReadConfig(iopts.configPath)
	if err != nil {
		return err
	}

	detected := DetectDeployJobs(root)
	if len(detected) == 0 {
		fmt.Fprintln(out, "No deploy jobs detected.")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "We look for jobs whose names contain \"deploy\", \"release\", \"publish\" or \"ship\".")
		fmt.Fprintln(out, "If one of your jobs performs a deployment under a different name, you can wire it up")
		fmt.Fprintln(out, "manually by adding this step:")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "  - run:")
		fmt.Fprintln(out, "      name: Log deploy marker")
		fmt.Fprintln(out, "      command: |")
		fmt.Fprintln(out, "        circleci run release log \\")
		fmt.Fprintln(out, "          --component-name=<your-service> \\")
		fmt.Fprintln(out, "          --environment-name=<your-environment> \\")
		fmt.Fprintln(out, "          --target-version=$CIRCLE_SHA1")
		fmt.Fprintln(out, "")
		fmt.Fprintf(out, "More options (status tracking, rollbacks, etc.): %s\n", deployMarkersDocsURL)
		return nil
	}

	fmt.Fprintf(out, "Found %d deploy job%s:\n", len(detected), pluralS(len(detected)))
	for _, j := range detected {
		marker := ""
		if j.AlreadyInstrumented {
			marker = "  (already instrumented — will be skipped)"
		}
		fmt.Fprintf(out, "  - %s%s\n", j.Name, marker)
	}
	fmt.Fprintln(out, "")

	// If every detected job is already instrumented there is no need to
	// ask any questions or touch the file.
	if allInstrumented(detected) {
		fmt.Fprintln(out, "Every detected deploy job already logs a marker. Nothing to do.")
		fmt.Fprintf(out, "\nDashboard: %s\n", deploysDashboardURL)
		fmt.Fprintf(out, "Docs:      %s\n", deployMarkersDocsURL)
		return nil
	}

	componentDefault := defaultComponentName(iopts.configPath)
	componentName := deployOpts.reader.ReadStringFromUser(
		"What is this service called?",
		componentDefault,
	)
	if componentName == "" {
		componentName = componentDefault
	}

	steps := make([]MarkerStep, 0, len(detected))
	for _, j := range detected {
		if j.AlreadyInstrumented {
			continue
		}
		envDefault, inferred := InferEnvironmentName(j.Name)
		if !inferred {
			envDefault = "production"
		}
		env := deployOpts.reader.ReadStringFromUser(
			fmt.Sprintf("What environment does %q target?", j.Name),
			envDefault,
		)
		if env == "" {
			env = envDefault
		}
		steps = append(steps, MarkerStep{
			JobName:         j.Name,
			ComponentName:   componentName,
			EnvironmentName: env,
		})
	}

	result, err := PatchConfig(root, steps)
	if err != nil {
		return err
	}

	if len(result.Modified) == 0 {
		fmt.Fprintln(out, "\nNo changes were needed — all detected jobs were already instrumented.")
		fmt.Fprintf(out, "\nDashboard: %s\n", deploysDashboardURL)
		fmt.Fprintf(out, "Docs:      %s\n", deployMarkersDocsURL)
		return nil
	}

	if err := WriteConfig(iopts.configPath, root); err != nil {
		return err
	}

	fmt.Fprintf(out, "\nUpdated %s.\n", iopts.configPath)
	fmt.Fprintf(out, "Added deploy marker step to: %s\n", joinJobs(result.Modified))
	if len(result.Skipped) > 0 {
		fmt.Fprintf(out, "Left untouched (already instrumented): %s\n", joinJobs(result.Skipped))
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Next steps:")
	fmt.Fprintln(out, "  1. Review the diff:")
	fmt.Fprintf(out, "       git diff %s\n", iopts.configPath)
	fmt.Fprintln(out, "  2. Commit and push when you're happy with it:")
	fmt.Fprintf(out, "       git add %s\n", iopts.configPath)
	fmt.Fprintln(out, "       git commit -m \"Wire up CircleCI deploy markers\"")
	fmt.Fprintln(out, "       git push")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "After your next pipeline run you'll see deploy markers on the dashboard:")
	fmt.Fprintf(out, "  %s\n", deploysDashboardURL)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Want status tracking, rollbacks, or more control? See the deploy markers guide:")
	fmt.Fprintf(out, "  %s\n", deployMarkersDocsURL)

	return nil
}

// defaultComponentName proposes a sensible default for the component
// name prompt. We prefer the project name inferred from git remotes
// (works for GitHub / Bitbucket) and fall back to the repo directory
// name so the command still has a reasonable default when run in a
// checkout without a recognisable remote.
func defaultComponentName(configPath string) string {
	if remote, err := git.InferProjectFromGitRemotes(); err == nil && remote.Project != "" {
		return remote.Project
	}
	abs, err := filepath.Abs(configPath)
	if err == nil {
		// configPath is typically ".circleci/config.yml"; walk two
		// levels up to reach the repo root.
		return filepath.Base(filepath.Dir(filepath.Dir(abs)))
	}
	return ""
}

func allInstrumented(jobs []DetectedJob) bool {
	for _, j := range jobs {
		if !j.AlreadyInstrumented {
			return false
		}
	}
	return len(jobs) > 0
}

func joinJobs(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	out := names[0]
	for _, n := range names[1:] {
		out += ", " + n
	}
	return out
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
