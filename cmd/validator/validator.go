package validator

import "github.com/spf13/cobra"

type Validator func(cmd *cobra.Command, args []string) error
