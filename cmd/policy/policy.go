package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CircleCI-Public/circle-policy-agent/cpa"
	"github.com/araddon/dateparse"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/api/policy"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

// NewCommand creates the root policy command with all policy subcommands attached.
func NewCommand(config *settings.Config, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "policy",
		PersistentPreRunE: preRunE,
		Short: "Policies ensures security of build configs via security policy management framework. " +
			"This group of commands allows the management of polices to be verified against build configs.",
	}

	policyBaseURL := cmd.PersistentFlags().String("policy-base-url", "https://internal.circleci.com", "base url for policy api")

	push := func() *cobra.Command {
		var ownerID, context string
		var noPrompt bool
		var request policy.CreatePolicyBundleRequest

		cmd := &cobra.Command{
			Short: "push policy bundle",
			Use:   "push <policy_dir_path>",
			RunE: func(cmd *cobra.Command, args []string) error {
				bundle, err := loadBundleFromFS(args[0])
				if err != nil {
					return fmt.Errorf("failed to walk policy directory path: %w", err)
				}

				request.Policies = bundle

				client := policy.NewClient(*policyBaseURL, config)

				if !noPrompt {
					request.DryRun = true
					diff, err := client.CreatePolicyBundle(ownerID, context, request)
					if err != nil {
						return fmt.Errorf("failed to get bundle diff: %v", err)
					}

					_, _ = io.WriteString(cmd.ErrOrStderr(), "The following changes are going to be made: ")
					_ = prettyJSONEncoder(cmd.ErrOrStderr()).Encode(diff)
					_, _ = io.WriteString(cmd.ErrOrStderr(), "\n")

					if !Confirm(cmd.ErrOrStderr(), cmd.InOrStdin(), "Do you wish to continue? (y/N)") {
						return nil
					}
					_, _ = io.WriteString(cmd.ErrOrStderr(), "\n")
				}

				request.DryRun = false

				diff, err := client.CreatePolicyBundle(ownerID, context, request)
				if err != nil {
					return fmt.Errorf("failed to push policy bundle: %w", err)
				}

				_, _ = io.WriteString(cmd.ErrOrStderr(), "Policy Bundle Pushed Successfully\n")
				_, _ = io.WriteString(cmd.ErrOrStderr(), "\ndiff: ")
				_ = prettyJSONEncoder(cmd.OutOrStdout()).Encode(diff)

				return nil
			},
			Args:    cobra.ExactArgs(1),
			Example: `policy push ./policies --owner-id 462d67f8-b232-4da4-a7de-0c86dd667d3f`,
		}

		cmd.Flags().StringVar(&context, "context", "config", "policy context")
		cmd.Flags().StringVar(&ownerID, "owner-id", "", "the id of the policy's owner")
		cmd.Flags().BoolVar(&noPrompt, "no-prompt", false, "removes the prompt")
		if err := cmd.MarkFlagRequired("owner-id"); err != nil {
			panic(err)
		}

		return cmd
	}()

	diff := func() *cobra.Command {
		var ownerID, context string
		cmd := &cobra.Command{
			Short: "Get diff between local and remote policy bundles",
			Use:   "diff <policy_dir_path>",
			RunE: func(cmd *cobra.Command, args []string) error {
				bundle, err := loadBundleFromFS(args[0])
				if err != nil {
					return fmt.Errorf("failed to walk policy directory path: %w", err)
				}

				diff, err := policy.NewClient(*policyBaseURL, config).CreatePolicyBundle(ownerID, context, policy.CreatePolicyBundleRequest{
					Policies: bundle,
					DryRun:   true,
				})
				if err != nil {
					return fmt.Errorf("failed to get diff: %w", err)
				}

				return prettyJSONEncoder(cmd.OutOrStdout()).Encode(diff)
			},
			Args:    cobra.ExactArgs(1),
			Example: `policy diff ./policies --owner-id 462d67f8-b232-4da4-a7de-0c86dd667d3f`,
		}
		cmd.Flags().StringVar(&context, "context", "config", "policy context")
		cmd.Flags().StringVar(&ownerID, "owner-id", "", "the id of the policy's owner")
		if err := cmd.MarkFlagRequired("owner-id"); err != nil {
			panic(err)
		}

		return cmd
	}()

	fetch := func() *cobra.Command {
		var ownerID, context, policyName string
		cmd := &cobra.Command{
			Short: "Fetch policy bundle (or a single policy)",
			Use:   "fetch [policy_name]",
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 1 {
					policyName = args[0]
				}
				policies, err := policy.NewClient(*policyBaseURL, config).FetchPolicyBundle(ownerID, context, policyName)
				if err != nil {
					return fmt.Errorf("failed to fetch policy bundle: %v", err)
				}

				if err := prettyJSONEncoder(cmd.OutOrStdout()).Encode(policies); err != nil {
					return fmt.Errorf("failed to output policy bundle in json format: %v", err)
				}

				return nil
			},
			Args:    cobra.MaximumNArgs(1),
			Example: `policy fetch --owner-id 516425b2-e369-421b-838d-920e1f51b0f5`,
		}

		cmd.Flags().StringVar(&context, "context", "config", "policy context")
		cmd.Flags().StringVar(&ownerID, "owner-id", "", "the id of the policy's owner")
		if err := cmd.MarkFlagRequired("owner-id"); err != nil {
			panic(err)
		}

		return cmd
	}()

	logs := func() *cobra.Command {
		var after, before, outputFile, ownerID, context string
		var request policy.DecisionQueryRequest

		cmd := &cobra.Command{
			Short: "Get policy decision logs",
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
					logsBatch, err := client.GetDecisionLogs(ownerID, context, request)
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
			Example: `policy logs --owner-id 462d67f8-b232-4da4-a7de-0c86dd667d3f --after 2022/03/14 --out output.json`,
		}

		cmd.Flags().StringVar(&request.Status, "status", "", "filter decision logs based on their status")
		cmd.Flags().StringVar(&after, "after", "", "filter decision logs triggered AFTER this datetime")
		cmd.Flags().StringVar(&before, "before", "", "filter decision logs triggered BEFORE this datetime")
		cmd.Flags().StringVar(&request.Branch, "branch", "", "filter decision logs based on branch name")
		cmd.Flags().StringVar(&request.ProjectID, "project-id", "", "filter decision logs based on project-id")
		cmd.Flags().StringVar(&outputFile, "out", "", "specify output file name ")
		cmd.Flags().StringVar(&context, "context", "config", "policy context")
		cmd.Flags().StringVar(&ownerID, "owner-id", "", "the id of the policy's owner")
		if err := cmd.MarkFlagRequired("owner-id"); err != nil {
			panic(err)
		}

		return cmd
	}()

	decide := func() *cobra.Command {
		var (
			inputPath  string
			policyPath string
			metaFile   string
			ownerID    string
			context    string
			request    policy.DecisionRequest
		)

		cmd := &cobra.Command{
			Short: "make a decision",
			Use:   "decide [policy_file_or_dir_path]",
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 1 {
					policyPath = args[0]
				}
				if (policyPath == "" && ownerID == "") || (policyPath != "" && ownerID != "") {
					return fmt.Errorf("either [policy_file_or_dir_path] or --owner-id is required")
				}

				input, err := os.ReadFile(inputPath)
				if err != nil {
					return fmt.Errorf("failed to read input file: %w", err)
				}

				var metadata map[string]interface{}
				if metaFile != "" {
					raw, err := os.ReadFile(metaFile)
					if err != nil {
						return fmt.Errorf("failed to read meta file: %w", err)
					}
					if err := yaml.Unmarshal(raw, &metadata); err != nil {
						return fmt.Errorf("failed to decode meta content: %w", err)
					}
				}

				decision, err := func() (interface{}, error) {
					if policyPath != "" {
						return getPolicyDecisionLocally(policyPath, input, metadata)
					}
					request.Input = string(input)
					request.Metadata = metadata
					return policy.NewClient(*policyBaseURL, config).MakeDecision(ownerID, context, request)
				}()
				if err != nil {
					return fmt.Errorf("failed to make decision: %w", err)
				}

				if err := prettyJSONEncoder(cmd.OutOrStdout()).Encode(decision); err != nil {
					return fmt.Errorf("failed to encode decision: %w", err)
				}

				return nil
			},
			Args:    cobra.MaximumNArgs(1),
			Example: `policy decide ./policies --input ./.circleci/config.yml`,
		}

		cmd.Flags().StringVar(&ownerID, "owner-id", "", "the id of the policy's owner")
		cmd.Flags().StringVar(&context, "context", "config", "policy context for decision")
		cmd.Flags().StringVar(&inputPath, "input", "", "path to input file")
		cmd.Flags().StringVar(&metaFile, "metafile", "", "decision metadata file")

		if err := cmd.MarkFlagRequired("input"); err != nil {
			panic(err)
		}

		return cmd
	}()

	eval := func() *cobra.Command {
		var inputPath, metaFile, query string
		cmd := &cobra.Command{
			Short: "perform raw opa evaluation locally",
			Use:   "eval <policy_file_or_dir_path>",
			RunE: func(cmd *cobra.Command, args []string) error {
				policyPath := args[0]
				input, err := os.ReadFile(inputPath)
				if err != nil {
					return fmt.Errorf("failed to read input file: %w", err)
				}

				var metadata map[string]interface{}
				if metaFile != "" {
					raw, err := os.ReadFile(metaFile)
					if err != nil {
						return fmt.Errorf("failed to read meta file: %w", err)
					}
					if err := yaml.Unmarshal(raw, &metadata); err != nil {
						return fmt.Errorf("failed to decode meta content: %w", err)
					}
				}

				decision, err := getPolicyEvaluationLocally(policyPath, input, metadata, query)
				if err != nil {
					return fmt.Errorf("failed to make decision: %w", err)
				}

				if err := prettyJSONEncoder(cmd.OutOrStdout()).Encode(decision); err != nil {
					return fmt.Errorf("failed to encode decision: %w", err)
				}

				return nil
			},
			Args:    cobra.ExactArgs(1),
			Example: `policy eval ./policies --input ./.circleci/config.yml`,
		}

		cmd.Flags().StringVar(&inputPath, "input", "", "path to input file")
		cmd.Flags().StringVar(&metaFile, "metafile", "", "decision metadata file")
		cmd.Flags().StringVar(&query, "query", "data", "policy decision query")

		if err := cmd.MarkFlagRequired("input"); err != nil {
			panic(err)
		}

		return cmd
	}()

	cmd.AddCommand(push)
	cmd.AddCommand(diff)
	cmd.AddCommand(fetch)
	cmd.AddCommand(logs)
	cmd.AddCommand(decide)
	cmd.AddCommand(eval)

	return cmd
}

// prettyJSONEncoder takes a writer and returns a new json encoder with indent set to two space characters
func prettyJSONEncoder(dst io.Writer) *json.Encoder {
	enc := json.NewEncoder(dst)
	enc.SetIndent("", "  ")
	return enc
}

// getPolicyDecisionLocally takes path of policy path/directory and input (eg build config) as string, and performs policy evaluation locally
func getPolicyDecisionLocally(policyPath string, rawInput []byte, meta map[string]interface{}) (*cpa.Decision, error) {
	var input interface{}
	if err := yaml.Unmarshal(rawInput, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	p, err := cpa.LoadPolicyFromFS(policyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load policy files: %w", err)
	}

	decision, err := p.Decide(context.Background(), input, cpa.Meta(meta))
	if err != nil {
		return nil, fmt.Errorf("failed to make decision: %w", err)
	}

	return decision, nil
}

// getPolicyEvaluationLocally takes path of policy path/directory and input (eg build config) as string, and performs policy evaluation locally and returns raw opa evaluation response
func getPolicyEvaluationLocally(policyPath string, rawInput []byte, meta map[string]interface{}, query string) (interface{}, error) {
	var input interface{}
	if err := yaml.Unmarshal(rawInput, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	p, err := cpa.LoadPolicyFromFS(policyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load policy files: %w", err)
	}

	decision, err := p.Eval(context.Background(), query, input, cpa.Meta(meta))
	if err != nil {
		return nil, fmt.Errorf("failed to make decision: %w", err)
	}

	return decision, nil
}

func loadBundleFromFS(root string) (map[string]string, error) {
	root = filepath.Clean(root)

	rootInfo, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("failed to get path info: %w", err)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("policy path is not a directory")
	}

	bundle := make(map[string]string)

	err = filepath.WalkDir(root, func(path string, f fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if f.IsDir() || filepath.Ext(path) != ".rego" {
			return nil
		}

		fileContent, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		bundle[path] = string(fileContent)

		return nil
	})

	return bundle, err
}

func Confirm(w io.Writer, r io.Reader, question string) bool {
	fmt.Fprint(w, question+" ")
	var answer string

	_, _ = fmt.Fscanln(r, &answer)

	answer = strings.ToLower(answer)
	return answer == "y" || answer == "yes"
}
