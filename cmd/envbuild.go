package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"

	"github.com/CircleCI-Public/chunk-cli/envbuilder"
	"github.com/spf13/cobra"
)

var validEnvbuildDockerTag = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/\-]*(:[a-zA-Z0-9._\-]+)?$`)

func newEnvbuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "envbuild",
		Short: "Detect tech stack and build test environments",
	}

	cmd.AddCommand(newEnvbuildEnvCmd())
	cmd.AddCommand(newEnvbuildBuildCmd())

	return cmd
}

func newEnvbuildEnvCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "env",
		Short: "Detect tech stack and print environment spec as JSON",
		Long: `Analyse the repository at --dir, detect its tech stack, and print
a JSON environment spec to stdout. Pipe this into 'circleci envbuild build' to
generate a Dockerfile and build a test image.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := os.Stat(dir); err != nil {
				return fmt.Errorf("directory %q not found: %w", dir, err)
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Detecting environment in %s...\n", dir)

			env, err := envbuilder.DetectEnvironment(cmd.Context(), dir)
			if err != nil {
				return fmt.Errorf("detect environment: %w", err)
			}

			out, err := json.MarshalIndent(env, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal environment: %w", err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", out)
			return err
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to detect environment in")

	return cmd
}

func newEnvbuildBuildCmd() *cobra.Command {
	var dir, tag string

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Generate a Dockerfile from an environment spec and build a test image",
		Long: `Read a JSON environment spec from stdin (produced by 'circleci envbuild env'),
write Dockerfile.test to --dir, and build a Docker test image from it.

Example:
  circleci envbuild env --dir . | circleci envbuild build --dir .`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if tag != "" && !validEnvbuildDockerTag.MatchString(tag) {
				return fmt.Errorf("invalid docker tag %q", tag)
			}

			// Guard against interactive use: if stdin is a terminal (not a pipe),
			// fail fast rather than blocking silently.
			if f, ok := cmd.InOrStdin().(*os.File); ok {
				if fi, err := f.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
					return fmt.Errorf("no input on stdin — pipe a JSON env spec from 'circleci envbuild env'")
				}
			}

			raw, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("read environment spec: %w", err)
			}
			var env envbuilder.Environment
			if err := json.Unmarshal(raw, &env); err != nil {
				return fmt.Errorf("parse environment spec: %w", err)
			}

			dockerfilePath, err := envbuilder.WriteDockerfile(dir, &env)
			if err != nil {
				return fmt.Errorf("write dockerfile: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %s\n", dockerfilePath)
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Building Docker image in %s...\n", dir)

			args := []string{"build", "-f", "Dockerfile.test"}
			if tag != "" {
				args = append(args, "-t", tag)
			}
			args = append(args, ".")

			dockerCmd := exec.CommandContext(cmd.Context(), "docker", args...)
			dockerCmd.Dir = dir
			dockerCmd.Stdout = cmd.OutOrStdout()
			dockerCmd.Stderr = cmd.ErrOrStderr()
			if err := dockerCmd.Run(); err != nil {
				return fmt.Errorf("docker build: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Docker image built successfully")
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to write Dockerfile.test and build from")
	cmd.Flags().StringVar(&tag, "tag", "", "Image tag (e.g. myapp:latest)")

	return cmd
}
