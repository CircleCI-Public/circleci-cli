package policy

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/policy"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

func NewCommand(config *settings.Config, preRunE validator) *cobra.Command {
	var policyClient ClientInterface
	var ownerID, activeFilter string

	initClient := func(cmd *cobra.Command, args []string) (e error) {
		if policyClient, e = policy.NewClient(*config); e != nil {
			return e
		}
		return preRunE(cmd, args)
	}

	command := &cobra.Command{
		Use: "policy",
		Short: "Policies ensures security of build configs via security policy management framework. " +
			"This group of commands allows the management of polices to be verified against build configs.",
	}

	listPoliciesCommand := &cobra.Command{
		Short:   "List all policies",
		Use:     "list",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listPolicies(policyClient, ownerID, activeFilter)
		},
		Args:    cobra.ExactArgs(0),
		Example: `policy list --owner-id 516425b2-e369-421b-838d-920e1f51b0f5 --active true`,
	}
	listPoliciesCommand.Flags().StringVar(&ownerID, "owner-id", "", "the id of the owner of a policy")
	listPoliciesCommand.Flags().StringVar(&activeFilter, "active", "", "(OPTIONAL) filter policies based on active status (true or false)")
	listPoliciesCommand.MarkFlagRequired("owner-id")
	command.AddCommand(listPoliciesCommand)

	return command
}

func listPolicies(policyClient ClientInterface, ownerID, activeFilter string) error {
	if activeFilter != "" && !(activeFilter == "true" || activeFilter == "false") {
		return errors.New("activeFilter value can only be true or false")
	}
	policies, err := policyClient.ListPolicies(ownerID, activeFilter)
	if err != nil {
		return err
	}
	fmt.Println(policies)
	return nil
}

type validator func(cmd *cobra.Command, args []string) error

type ClientInterface interface {
	ListPolicies(ownerID, activeFilter string) (string, error)
}
