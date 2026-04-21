package settings

import (
	"encoding/json"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newListCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List current CLI settings",
		Long: heredoc.Doc(`
			Display the current CLI settings.

			The token value is masked for security. Settings are read from
			$XDG_CONFIG_HOME/circleci/config.yml (default: ~/.config/circleci/config.yml).

			JSON fields: token_set, host
		`),
		Example: heredoc.Doc(`
			# Show current settings
			$ circleci settings list

			# Output as JSON
			$ circleci settings list --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			streams := iostream.FromCmd(cmd)
			return runList(streams, jsonOut)
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

func runList(streams iostream.Streams, jsonOut bool) error {
	cfg, err := config.Load()
	if err != nil {
		return clierrors.New("settings.load_failed", "Failed to load settings", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	path, _ := config.Path()

	tokenSet := cfg.EffectiveToken() != ""

	if jsonOut {
		out := map[string]any{
			"token_set": tokenSet,
			"host":      cfg.EffectiveHost(),
		}
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	streams.Printf("Config file: %s\n\n", path)
	streams.Printf("%-10s  %s\n", "token", maskToken(cfg.EffectiveToken()))
	streams.Printf("%-10s  %s\n", "host", cfg.EffectiveHost())
	return nil
}

func maskToken(token string) string {
	if token == "" {
		return "(not set)"
	}
	if len(token) <= 8 {
		return "****"
	}
	return fmt.Sprintf("%s...%s", token[:4], token[len(token)-4:])
}
