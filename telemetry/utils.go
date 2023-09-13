package telemetry

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Utility function used when creating telemetry events.
// It takes a cobra Command and creates a telemetry.CommandInfo of it
// If getParent is true, puts both the command's args in `LocalArgs` and the parent's args
// Else only put the command's args
// Note: child flags overwrite parent flags with same name
func GetCommandInformation(cmd *cobra.Command, getParent bool) CommandInfo {
	localArgs := map[string]string{}

	parent := cmd.Parent()
	if getParent && parent != nil {
		parent.LocalFlags().VisitAll(func(flag *pflag.Flag) {
			localArgs[flag.Name] = flag.Value.String()
		})
	}

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		localArgs[flag.Name] = flag.Value.String()
	})

	return CommandInfo{
		Name:      cmd.Name(),
		LocalArgs: localArgs,
	}
}
