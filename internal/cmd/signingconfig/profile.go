// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

package signingconfig

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iossigning"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile <command>",
		Short: "Add or remove provisioning profiles on a signing config",
		Long: heredoc.Doc(`
			Add, replace, or remove a provisioning profile on an existing signing
			config, without recreating the whole config.

			A profile is matched to an existing one by the bundle identifier and
			profile type embedded in the provisioning profile file itself, not by
			file name: the same bundle ID and type replaces it in place, and
			anything else is added alongside it.

			Recreating a signing config (delete then create) leaves a gap where
			pipelines referencing it by name fail; these commands update it in
			place instead.
		`),
	}

	cmd.AddCommand(newProfileAddCmd())
	cmd.AddCommand(newProfileRemoveCmd())

	return cmd
}

func parseSigningConfigID(idArg string) (uuid.UUID, error) {
	id, err := uuid.Parse(idArg)
	if err != nil {
		return uuid.Nil, clierrors.New("signing_config.invalid_id", "Invalid signing config ID",
			fmt.Sprintf("%q is not a valid signing config ID.", idArg)).
			WithSuggestions("Run: circleci signing-config list --org <org>").
			WithExitCode(clierrors.ExitBadArguments)
	}
	return id, nil
}

// --- signing-config profile add ---

func newProfileAddCmd() *cobra.Command {
	var profilePath string

	cmd := &cobra.Command{
		Use:   "add <signing-config-id>",
		Short: "Add or replace a provisioning profile on a signing config",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<signing-config-id>%[1]s is the ID of the signing config to update.
				Use %[1]scircleci signing-config list%[1]s to find the ID.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Add a provisioning profile to a signing config, or replace one that
			already has the same bundle identifier and profile type.

			The file at --profile is read and base64-encoded locally, then sent
			to CircleCI. The config's other profiles and certificate are left
			untouched.
		`),
		Example: heredoc.Doc(`
			# Add a new profile
			$ circleci signing-config profile add <signing-config-id> --profile ./MyAppExtension.mobileprovision

			# Replace an existing profile (same bundle ID and profile type, regardless of file name)
			$ circleci signing-config profile add <signing-config-id> --profile ./MyApp.mobileprovision

			# Find the signing config ID first
			$ circleci signing-config list --json --jq '.[].id'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "signing-config-id"); err != nil {
				return err
			}
			if profilePath == "" {
				return cmdutil.RequireFlag("profile")
			}
			ctx := cmd.Context()
			idArg := args[0]
			id, err := parseSigningConfigID(idArg)
			if err != nil {
				return err
			}

			fileName, blob, err := iossigning.EncodeFile(profilePath)
			if err != nil {
				return clierrors.New("signing_config.profile_file_unreadable", "Cannot read provisioning profile", err.Error()).
					WithExitCode(clierrors.ExitBadArguments)
			}

			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			if err := client.UpdateIOSSigningConfigProfile(ctx, id, fileName, blob); err != nil {
				return apiErr(err, idArg)
			}
			iostream.Printf(ctx, "%s Added profile %q to signing config %s\n", iostream.SymbolOK(ctx), fileName, idArg)
			return nil
		},
	}

	cmd.Flags().StringVar(&profilePath, "profile", "", "Path to a provisioning profile file")

	return cmd
}

// --- signing-config profile remove ---

func newProfileRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <signing-config-id> <profile-id>",
		Aliases: []string{"rm"},
		Short:   "Remove a provisioning profile from a signing config",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<signing-config-id>%[1]s is the ID of the signing config to update.
				%[1]s<profile-id>%[1]s is the ID of the provisioning profile to remove.
				Use %[1]scircleci signing-config list --json%[1]s to find both IDs.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Remove a single provisioning profile from a signing config, without
			recreating the whole config.

			Removing a profile that no longer exists is not an error.
		`),
		Example: heredoc.Doc(`
			# Remove a profile by ID
			$ circleci signing-config profile remove <signing-config-id> <profile-id>

			# Find the profile ID first
			$ circleci signing-config list --json --jq '.[] | select(.id=="<signing-config-id>") | .provisioning_profiles'
		`),
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "signing-config-id", "profile-id"); err != nil {
				return err
			}
			ctx := cmd.Context()
			idArg, profileIDArg := args[0], args[1]
			id, err := parseSigningConfigID(idArg)
			if err != nil {
				return err
			}
			profileID, err := uuid.Parse(profileIDArg)
			if err != nil {
				return clierrors.New("signing_config.invalid_profile_id", "Invalid profile ID",
					fmt.Sprintf("%q is not a valid provisioning profile ID.", profileIDArg)).
					WithSuggestions("Run: circleci signing-config list --json").
					WithExitCode(clierrors.ExitBadArguments)
			}

			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			if err := client.RemoveIOSSigningConfigProfile(ctx, id, profileID); err != nil {
				return apiErr(err, idArg)
			}
			iostream.Printf(ctx, "%s Removed profile %s from signing config %s\n", iostream.SymbolOK(ctx), profileIDArg, idArg)
			return nil
		},
	}

	return cmd
}
