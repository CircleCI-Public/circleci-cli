// Package completion implements the "circleci completion" command group.
package completion

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

const completionTag = "# circleci shell completion"

// NewCompletionCmd returns the "circleci completion" command group.
func NewCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion <command>",
		Short: "Manage shell completions",
		Long: heredoc.Doc(`
			Manage shell tab-completion for circleci.

			Run 'circleci completion install' to add completion to your shell
			profile automatically. Supported shells: bash, zsh.

			To install manually, add one of the following to your shell profile:

			  bash:  source <(circleci completion bash)
			  zsh:   source <(circleci completion zsh)
		`),
	}

	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newUninstallCmd())
	cmd.AddCommand(newBashCmd())
	cmd.AddCommand(newZshCmd())

	return cmd
}

type shellConfig struct {
	name   string
	rcFile string
	source string
}

func detectShell(home string) (shellConfig, error) {
	shell := os.Getenv("SHELL")
	switch {
	case strings.HasSuffix(shell, "zsh"):
		return shellConfig{
			name:   "zsh",
			rcFile: filepath.Join(home, ".zshrc"),
			source: "source <(circleci completion zsh)",
		}, nil
	case strings.HasSuffix(shell, "bash"):
		rcFile := filepath.Join(home, ".bash_profile")
		if _, err := os.Stat(filepath.Join(home, ".bashrc")); err == nil {
			rcFile = filepath.Join(home, ".bashrc")
		}
		return shellConfig{
			name:   "bash",
			rcFile: rcFile,
			source: "source <(circleci completion bash)",
		}, nil
	default:
		return shellConfig{}, fmt.Errorf("unsupported shell %q — set SHELL to bash or zsh", shell)
	}
}

// CompletionInstalled reports whether the completion tag is already present
// in the user's shell rc file.
func CompletionInstalled() (bool, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return false, fmt.Errorf("HOME not set")
	}
	sh, err := detectShell(home)
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(sh.rcFile)
	if err != nil {
		return false, nil // rc file doesn't exist — not installed
	}
	return strings.Contains(string(data), completionTag), nil
}

func installCompletion(streams iostream.Streams) error {
	home := os.Getenv("HOME")
	if home == "" {
		return fmt.Errorf("HOME not set")
	}
	sh, err := detectShell(home)
	if err != nil {
		return err
	}

	data, readErr := os.ReadFile(sh.rcFile)
	if readErr == nil && strings.Contains(string(data), completionTag) {
		streams.ErrPrintf("Completion already installed in %s\n", sh.rcFile)
		return nil
	}

	f, err := os.OpenFile(sh.rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", sh.rcFile, err)
	}
	defer func() { _ = f.Close() }()

	line := "\n" + completionTag + "\n" + sh.source + "\n"
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("write %s: %w", sh.rcFile, err)
	}

	streams.ErrPrintf("%s Installed %s completion in %s\n",
		streams.Symbol("✓", "OK:"), sh.name, sh.rcFile)
	return nil
}

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install shell completion into your shell profile",
		Long: heredoc.Doc(`
			Append a completion source line to your shell profile (~/.zshrc or ~/.bashrc).

			The line is tagged so it can be cleanly removed with:
			  circleci completion uninstall
		`),
		Example: heredoc.Doc(`
			# Install completion (detects shell from $SHELL)
			$ circleci completion install

			# Then reload your shell
			$ source ~/.zshrc

			# To install manually instead, source the output directly
			$ source <(circleci completion bash)
		`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return installCompletion(iostream.FromCmd(cmd))
		},
	}
}

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove shell completion from your shell profile",
		Long: heredoc.Doc(`
			Remove the completion block previously added by 'circleci completion install'.
			Other content in your shell profile is left untouched.
		`),
		Example: heredoc.Doc(`
			# Remove completion
			$ circleci completion uninstall

			# Then reload your shell
			$ source ~/.zshrc

			# Check whether completion is currently installed
			$ grep -l "circleci shell completion" ~/.zshrc ~/.bashrc 2>/dev/null
		`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			streams := iostream.FromCmd(cmd)
			home := os.Getenv("HOME")
			if home == "" {
				return fmt.Errorf("HOME not set")
			}
			sh, err := detectShell(home)
			if err != nil {
				return err
			}

			data, err := os.ReadFile(sh.rcFile)
			if err != nil {
				// Nothing to uninstall.
				streams.ErrPrintf("%s Completion not installed\n", streams.Symbol("✓", "OK:"))
				return nil
			}

			var lines []string
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			skip := false
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, completionTag) {
					skip = true
					continue
				}
				if skip && strings.Contains(line, "circleci completion") {
					skip = false
					continue
				}
				skip = false
				lines = append(lines, line)
			}

			if err := os.WriteFile(sh.rcFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", sh.rcFile, err)
			}

			streams.ErrPrintf("%s Removed completion from %s\n", streams.Symbol("✓", "OK:"), sh.rcFile)
			return nil
		},
	}
}

func newBashCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "bash",
		Short:  "Generate bash completion script",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenBashCompletion(iostream.FromCmd(cmd).Out)
		},
	}
}

func newZshCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "zsh",
		Short:  "Generate zsh completion script",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenZshCompletion(iostream.FromCmd(cmd).Out)
		},
	}
}
