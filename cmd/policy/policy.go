package policy

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/policy"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

type validator func(cmd *cobra.Command, args []string) error

func NewCommand(config *settings.Config, preRunE validator) *cobra.Command {
	cmd := &cobra.Command{
		Use: "policy",
		Short: "Policies ensures security of build configs via security policy management framework. " +
			"This group of commands allows the management of polices to be verified against build configs.",
	}

	policyBaseURL := cmd.PersistentFlags().String("policy-base-url", "https://internal.circleci.com", "base url for policy api")

	list := func() *cobra.Command {
		var ownerID string
		var active bool

		cmd := &cobra.Command{
			Short:   "List all policies",
			Use:     "list",
			PreRunE: preRunE,
			RunE: func(cmd *cobra.Command, args []string) error {
				var flags struct {
					OwnerID string
					Active  *bool
				}

				flags.OwnerID = ownerID
				if cmd.Flag("active").Changed {
					flags.Active = &active
				}

				policies, err := policy.NewClient(*policyBaseURL, config).ListPolicies(flags.OwnerID, flags.Active)
				if err != nil {
					return fmt.Errorf("failed to list policies: %v", err)
				}

				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")

				if err := enc.Encode(policies); err != nil {
					return fmt.Errorf("failed to output policies in json format: %v", err)
				}

				return nil
			},
			Args:    cobra.ExactArgs(0),
			Example: `policy list --owner-id 516425b2-e369-421b-838d-920e1f51b0f5 --active true`,
		}

		cmd.Flags().StringVar(&ownerID, "owner-id", "", "the id of the owner of a policy")
		cmd.Flags().BoolVar(&active, "active", false, "(OPTIONAL) filter policies based on active status (true or false)")
		cmd.MarkFlagRequired("owner-id")

		return cmd
	}()

	cmd.AddCommand(list)

	return cmd
}
