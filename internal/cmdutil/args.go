package cmdutil

import (
	"fmt"
	"strings"

	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
)

// RequireArgs returns a structured CLIError if args contains fewer elements
// than the number of names provided. Each name describes an expected positional
// argument (e.g. "workflow-id", "resource-class") and appears in the error
// message as <name>.
//
// Use alongside cobra.MaximumNArgs(N) so that too many args are still rejected
// by Cobra, while the missing-arg case produces a structured error from RunE.
func RequireArgs(args []string, names ...string) *clierrors.CLIError {
	if len(args) >= len(names) {
		return nil
	}
	var missing []string
	for i := len(args); i < len(names); i++ {
		missing = append(missing, "<"+names[i]+">")
	}
	noun := "argument"
	if len(missing) > 1 {
		noun = "arguments"
	}
	return clierrors.New("args.missing", "Missing required argument",
		fmt.Sprintf("Required %s missing: %s", noun, strings.Join(missing, " "))).
		WithExitCode(clierrors.ExitBadArguments)
}
