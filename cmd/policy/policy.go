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
		Use:               "policy",
		PersistentPreRunE: preRunE,
		Short: "Policies ensures security of build configs via security policy management framework. " +
			"This group of commands allows the management of polices to be verified against build configs.",
	}

	policyBaseURL := cmd.PersistentFlags().String("policy-base-url", "https://internal.circleci.com", "base url for policy api")
	ownerID := cmd.PersistentFlags().String("owner-id", "", "the id of the owner of a policy")
	cmd.MarkPersistentFlagRequired("owner-id")

	list := func() *cobra.Command {
		var active bool

		cmd := &cobra.Command{
			Short: "List all policies",
			Use:   "list",
			RunE: func(cmd *cobra.Command, args []string) error {
				var flags struct {
					OwnerID string
					Active  *bool
				}

				flags.OwnerID = *ownerID
				if cmd.Flag("active").Changed {
					flags.Active = &active
				}

				policies, err := policy.NewClient(*policyBaseURL, config).ListPolicies(flags.OwnerID, flags.Active)
				if err != nil {
					return fmt.Errorf("failed to list policies: %v", err)
				}

				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")

				if err := enc.Encode(policies); err != nil {
					return fmt.Errorf("failed to output policies in json format: %v", err)
				}

				return nil
			},
			Args:    cobra.ExactArgs(0),
			Example: `policy list --owner-id 516425b2-e369-421b-838d-920e1f51b0f5 --active true`,
		}

		cmd.Flags().BoolVar(&active, "active", false, "(OPTIONAL) filter policies based on active status (true or false)")

		return cmd
	}()

	create := func() *cobra.Command {
		var policyPath string
		var creationRequest policy.CreationRequest

		cmd := &cobra.Command{
			Short: "create policy",
			Use:   "create",
			RunE: func(cmd *cobra.Command, args []string) error {
				policyData, err := os.ReadFile(policyPath)
				if err != nil {
					return fmt.Errorf("failed to read policy file: %w", err)
				}

				creationRequest.Content = string(policyData)

				client := policy.NewClient(*policyBaseURL, config)

				result, err := client.CreatePolicy(*ownerID, creationRequest)
				if err != nil {
					return fmt.Errorf("failed to create policy: %w", err)
				}

				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")

				if err := enc.Encode(result); err != nil {
					return fmt.Errorf("failed to encode result to stdout: %w", err)
				}

				return nil
			},
		}

		cmd.Flags().StringVar(&creationRequest.Name, "name", "", "name of policy to create")
		cmd.Flags().StringVar(&creationRequest.Context, "context", "config", "policy context")
		cmd.Flags().StringVar(&policyPath, "policy", "", "path to rego policy file")

		cmd.MarkFlagRequired("name")
		cmd.MarkFlagRequired("policy")

		return cmd
	}()

	get := func() *cobra.Command {
		var ownerID string

		cmd := &cobra.Command{
			Short:   "Get a policy",
			Use:     "get <policyID>",
			PreRunE: preRunE,
			RunE: func(cmd *cobra.Command, args []string) error {
				var flags struct {
					OwnerID string
				}

				flags.OwnerID = ownerID

				policy, err := policy.NewClient(*policyBaseURL, config).GetPolicy(flags.OwnerID, args[0])
				if err != nil {
					return fmt.Errorf("failed to get policy: %v", err)
				}

				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")

				if err := enc.Encode(policy); err != nil {
					return fmt.Errorf("failed to output policy in json format: %v", err)
				}

				return nil
			},
			Args:    cobra.ExactArgs(1),
			Example: `policy get 60b7e1a5-c1d7-4422-b813-7a12d353d7c6 --owner-id 516425b2-e369-421b-838d-920e1f51b0f5`,
		}

		cmd.Flags().StringVar(&ownerID, "owner-id", "", "the id of the owner of the policy")
		cmd.MarkFlagRequired("owner-id")

		return cmd
	}()

	cmd.AddCommand(list)
	cmd.AddCommand(create)
	cmd.AddCommand(get)

	return cmd
}
