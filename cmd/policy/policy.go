package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/CircleCI-Public/circle-policy-agent/cpa"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/araddon/dateparse"

	"github.com/CircleCI-Public/circleci-cli/api/policy"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

// validator is a cobra command and args validator to be run as persistent PreRun for every policy command.
type validator func(cmd *cobra.Command, args []string) error

// NewCommand creates the root policy command with all policy subcommands attached.
func NewCommand(config *settings.Config, preRunE validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "policy",
		PersistentPreRunE: preRunE,
		Short: "Policies ensures security of build configs via security policy management framework. " +
			"This group of commands allows the management of polices to be verified against build configs.",
	}

	policyBaseURL := cmd.PersistentFlags().String("policy-base-url", "https://internal.circleci.com", "base url for policy api")
	ownerID := cmd.PersistentFlags().String("owner-id", "", "the id of the policy's owner")

	if err := cmd.MarkPersistentFlagRequired("owner-id"); err != nil {
		panic(err)
	}

	list := func() *cobra.Command {
		cmd := &cobra.Command{
			Short: "List all policies",
			Use:   "list",
			RunE: func(cmd *cobra.Command, _ []string) error {
				policies, err := policy.NewClient(*policyBaseURL, config).ListPolicies(*ownerID)
				if err != nil {
					return fmt.Errorf("failed to list policies: %v", err)
				}

				if err := prettyJSONEncoder(cmd.OutOrStdout()).Encode(policies); err != nil {
					return fmt.Errorf("failed to output policies in json format: %v", err)
				}

				return nil
			},
			Args:    cobra.ExactArgs(0),
			Example: `policy list --owner-id 516425b2-e369-421b-838d-920e1f51b0f5`,
		}
		return cmd
	}()

	create := func() *cobra.Command {
		var policyPath string
		var creationRequest policy.CreationRequest

		cmd := &cobra.Command{
			Short: "create policy",
			Use:   "create",
			RunE: func(cmd *cobra.Command, _ []string) error {
				policyData, err := os.ReadFile(policyPath)
				if err != nil {
					return fmt.Errorf("failed to read policy file: %w", err)
				}
				creationRequest.Content = string(policyData)

				result, err := policy.NewClient(*policyBaseURL, config).CreatePolicy(*ownerID, creationRequest)
				if err != nil {
					return fmt.Errorf("failed to create policy: %w", err)
				}

				if err := prettyJSONEncoder(cmd.OutOrStdout()).Encode(result); err != nil {
					return fmt.Errorf("failed to encode result to stdout: %w", err)
				}

				return nil
			},
			Args:    cobra.ExactArgs(0),
			Example: `policy create --owner-id 462d67f8-b232-4da4-a7de-0c86dd667d3f --name policy_name --policy ./policy.rego`,
		}

		cmd.Flags().StringVar(&creationRequest.Context, "context", "config", "policy context")
		cmd.Flags().StringVar(&policyPath, "policy", "", "path to rego policy file")

		if err := cmd.MarkFlagRequired("policy"); err != nil {
			panic(err)
		}

		return cmd
	}()

	get := func() *cobra.Command {
		cmd := &cobra.Command{
			Short: "Get a policy",
			Use:   "get <policyID>",
			RunE: func(cmd *cobra.Command, args []string) error {
				p, err := policy.NewClient(*policyBaseURL, config).GetPolicy(*ownerID, args[0])
				if err != nil {
					return fmt.Errorf("failed to get policy: %v", err)
				}

				if err := prettyJSONEncoder(cmd.OutOrStdout()).Encode(p); err != nil {
					return fmt.Errorf("failed to output policy in json format: %v", err)
				}

				return nil
			},
			Args:    cobra.ExactArgs(1),
			Example: `policy get 60b7e1a5-c1d7-4422-b813-7a12d353d7c6 --owner-id 516425b2-e369-421b-838d-920e1f51b0f5`,
		}
		return cmd
	}()

	delete := func() *cobra.Command {
		cmd := &cobra.Command{
			Short: "Delete a policy",
			Use:   "delete <policyID>",
			RunE: func(cmd *cobra.Command, args []string) error {
				err := policy.NewClient(*policyBaseURL, config).DeletePolicy(*ownerID, args[0])
				if err != nil {
					return fmt.Errorf("failed to delete policy: %v", err)
				}
				_, _ = io.WriteString(cmd.OutOrStdout(), "Deleted Successfully\n")
				return nil
			},
			Args:    cobra.ExactArgs(1),
			Example: `policy delete 60b7e1a5-c1d7-4422-b813-7a12d353d7c6 --owner-id 516425b2-e369-421b-838d-920e1f51b0f5`,
		}

		return cmd
	}()

	update := func() *cobra.Command {
		var policyPath string
		var context string
		var updateRequest policy.UpdateRequest

		cmd := &cobra.Command{
			Short: "Update a policy",
			Use:   "update <policyID>",
			RunE: func(cmd *cobra.Command, args []string) error {
				if !(cmd.Flag("policy").Changed || cmd.Flag("context").Changed) {
					return fmt.Errorf("one of policy or context must be set")
				}

				if cmd.Flag("policy").Changed {
					policyData, err := os.ReadFile(policyPath)
					if err != nil {
						return fmt.Errorf("failed to read policy file: %w", err)
					}
					content := string(policyData)
					updateRequest.Content = &content
				}

				if cmd.Flag("context").Changed {
					updateRequest.Context = &context
				}

				result, err := policy.NewClient(*policyBaseURL, config).UpdatePolicy(*ownerID, args[0], updateRequest)
				if err != nil {
					return fmt.Errorf("failed to update policy: %w", err)
				}

				if err := prettyJSONEncoder(cmd.OutOrStdout()).Encode(result); err != nil {
					return fmt.Errorf("failed to encode result to stdout: %w", err)
				}

				return nil
			},
			Args:    cobra.ExactArgs(1),
			Example: `policy update e9e300d1-5bab-4704-b610-addbd6e03b0b --owner-id 462d67f8-b232-4da4-a7de-0c86dd667d3f --name policy_name --policy ./policy.rego`,
		}

		cmd.Flags().StringVar(&context, "context", "", "policy context (if set, must be config)")
		cmd.Flags().StringVar(&policyPath, "policy", "", "path to rego file containing the updated policy")

		return cmd
	}()

	logs := func() *cobra.Command {
		var after, before, outputFile string
		var request policy.DecisionQueryRequest

		cmd := &cobra.Command{
			Short: "Get policy (decision) logs",
			Use:   "logs",
			RunE: func(cmd *cobra.Command, _ []string) (err error) {
				if cmd.Flag("after").Changed {
					request.After = new(time.Time)
					*request.After, err = dateparse.ParseStrict(after)
					if err != nil {
						return fmt.Errorf("error in parsing --after value: %v", err)
					}
				}

				if cmd.Flag("before").Changed {
					request.Before = new(time.Time)
					*request.Before, err = dateparse.ParseStrict(before)
					if err != nil {
						return fmt.Errorf("error in parsing --before value: %v", err)
					}
				}

				dst := cmd.OutOrStdout()
				if outputFile != "" {
					file, err := os.Create(outputFile)
					if err != nil {
						return fmt.Errorf("failed to create output file: %v", err)
					}
					dst = file
					defer func() {
						if closeErr := file.Close(); err == nil && closeErr != nil {
							err = closeErr
						}
					}()
				}

				allLogs := make([]interface{}, 0)

				spr := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(cmd.ErrOrStderr()))
				spr.Suffix = " Fetching Policy Decision Logs..."

				spr.PostUpdate = func(s *spinner.Spinner) {
					s.Suffix = fmt.Sprintf(" Fetching Policy Decision Logs... downloaded %d logs...", len(allLogs))
				}

				spr.Start()
				defer spr.Stop()

				client := policy.NewClient(*policyBaseURL, config)

				for {
					logsBatch, err := client.GetDecisionLogs(*ownerID, request)
					if err != nil {
						return fmt.Errorf("failed to get policy decision logs: %v", err)
					}

					if len(logsBatch) == 0 {
						break
					}

					allLogs = append(allLogs, logsBatch...)
					request.Offset = len(allLogs)
				}

				if err := prettyJSONEncoder(dst).Encode(allLogs); err != nil {
					return fmt.Errorf("failed to output policy decision logs in json format: %v", err)
				}

				return nil
			},
			Args:    cobra.ExactArgs(0),
			Example: `policy logs  --owner-id 462d67f8-b232-4da4-a7de-0c86dd667d3f --after 2022/03/14 --out output.json`,
		}

		cmd.Flags().StringVar(&request.Status, "status", "", "filter decision logs based on their status")
		cmd.Flags().StringVar(&after, "after", "", "filter decision logs triggered AFTER this datetime")
		cmd.Flags().StringVar(&before, "before", "", "filter decision logs triggered BEFORE this datetime")
		cmd.Flags().StringVar(&request.Branch, "branch", "", "filter decision logs based on branch name")
		cmd.Flags().StringVar(&request.ProjectID, "project-id", "", "filter decision logs based on project-id")
		cmd.Flags().StringVar(&outputFile, "out", "", "specify output file name ")

		return cmd
	}()

	decide := func() *cobra.Command {
		var inputPath, policyPath string
		var request policy.DecisionRequest

		cmd := &cobra.Command{
			Short: "make a decision",
			Use:   "decide",
			RunE: func(cmd *cobra.Command, _ []string) error {
				if policyPath == "" && *ownerID == "" {
					return fmt.Errorf("--owner-id or --policy is required")
				}

				var decision interface{}
				input, err := ioutil.ReadFile(inputPath)
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}
				request.Input = string(input)

				if policyPath != "" {
					// make decision from local policy
					decision, err = getPolicyDecisionLocally(policyPath, request.Input)
				} else {
					// make decision from policy obtained from policy-service
					decision, err = policy.NewClient(*policyBaseURL, config).MakeDecision(*ownerID, request)
				}
				if err != nil {
					return fmt.Errorf("failed to make decision: %w", err)
				}

				if err := prettyJSONEncoder(cmd.OutOrStdout()).Encode(decision); err != nil {
					return fmt.Errorf("failed to encode decision: %w", err)
				}
				return nil
			},
			Args: cobra.ExactArgs(0),
		}

		cmd.Flags().StringVar(&request.Context, "context", "config", "policy context for decision")
		cmd.Flags().StringVar(&inputPath, "input", "", "path to input file")
		cmd.Flags().StringVar(&policyPath, "policy", "", "path to rego policy file or directory containing policy files")
		cmd.Flags().StringVar(ownerID, "owner-id", "", "the id of the policy's owner") // Redeclared flag to make optional

		_ = cmd.MarkFlagRequired("input")

		return cmd
	}()

	cmd.AddCommand(list)
	cmd.AddCommand(create)
	cmd.AddCommand(get)
	cmd.AddCommand(delete)
	cmd.AddCommand(update)
	cmd.AddCommand(logs)
	cmd.AddCommand(decide)

	return cmd
}

// prettyJSONEncoder takes a writer and returns a new json encoder with indent set to two space characters
func prettyJSONEncoder(dst io.Writer) *json.Encoder {
	enc := json.NewEncoder(dst)
	enc.SetIndent("", "  ")
	return enc
}

// getPolicyDecisionLocally takes path of policy path/directory and input (eg build config) as string, and performs policy evaluation locally
func getPolicyDecisionLocally(policyPath, inputString string) (*cpa.Decision, error) {
	var input interface{}
	if err := yaml.Unmarshal([]byte(inputString), &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	var parsedPolicy *cpa.Policy
	pathInfo, err := os.Stat(policyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get path info: %w", err)
	}

	// if policyPath is directory get content of all files in this directory (non-recursively) and add to document bundle
	if pathInfo.IsDir() {
		parsedPolicy, err = cpa.LoadPolicyDirectory(policyPath)
	} else {
		parsedPolicy, err = cpa.LoadPolicyFile(policyPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load policy files: %w", err)
	}

	ctx := context.Background()
	decision, err := parsedPolicy.Decide(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to make decision: %w", err)
	}

	return decision, nil
}
