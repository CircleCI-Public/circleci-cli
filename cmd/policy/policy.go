package policy

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/policy"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

func NewCommand(config *settings.Config) *cobra.Command {
	var policyClient policy.PolicyInterface

	initClient := func(cmd *cobra.Command, args []string) (e error) {
		if policyClient, e = policy.NewPolicyRestClient(*config); e != nil {
			return e
		}
		return nil
	}

	command := &cobra.Command{
		Use: "policy",
		Short: "Policies ensures security of build configs via security policy management framework. " +
			"This group of commands allows the management of polices to be verified against build configs.",
	}

	listPoliciesCommand := &cobra.Command{
		Short:   "List all policies",
		Use:     "list <ownerID> [<active>]",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listPolicies(policyClient, args)
		},
		Args:        cobra.RangeArgs(1, 2),
		Annotations: make(map[string]string),
		Example:     `circleci policy list 516425b2-e369-421b-838d-920e1f51b0f5 true`,
	}
	listPoliciesCommand.Annotations["<ownerID>"] = `the id of the owner of a policy. These are in uuid format`
	listPoliciesCommand.Annotations["[<active>]"] = `(OPTIONAL) filter policies based on active status (true or false)`
	command.AddCommand(listPoliciesCommand)

	return command
}

func listPolicies(policyClient policy.PolicyInterface, args []string) error {
	ownerID, activeFilter := args[0], ""
	if len(args) > 1 {
		activeFilter = args[1]
		if !(activeFilter == "true" || activeFilter == "false") {
			return errors.New("activeFilter value can only be true or false")
		}
	}
	policies, err := policyClient.ListPolicies(ownerID, activeFilter)
	if err != nil {
		return err
	}
	fmt.Println(policies)
	return nil
}
