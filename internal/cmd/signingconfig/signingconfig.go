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

// Package signingconfig implements the "circleci signing-config" command group
// for managing iOS signing configs (a certificate paired with provisioning
// profiles).
package signingconfig

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iossigning"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

// NewSigningConfigCmd returns the "circleci signing-config" command group.
func NewSigningConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "signing-config <command>",
		Short: "Manage iOS signing configs",
		Long: heredoc.Doc(`
			Create, list, and delete iOS signing configs.

			A signing config pairs an uploaded .p12 certificate with one or more
			provisioning profiles under a stable name. Reference the signing config
			by name from your pipeline config under the 'code_signing' block to
			install it onto the macOS runner during a job.
		`),
	}

	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newDeleteCmd())

	return cmd
}

func apiErr(err error, subject string) *clierrors.CLIError {
	return cmdutil.APIErr(err, subject,
		"signing_config.not_found", "No iOS signing config found for %q.",
		"Check the signing config ID and try again",
		"Run: circleci signing-config list --org-id <org-uuid>")
}

// createAPIErr translates errors from POST /signing-configs into structured
// CLI errors. The create endpoint can fail in ways that aren't about the
// signing config itself (the referenced cert may not exist; a config with the
// same name may already exist), so the generic "not found for <name>" mapping
// is wrong here.
func createAPIErr(err error, name, certID string) *clierrors.CLIError {
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return clierrors.New("signing_config.cert_not_found", "Certificate not found",
			fmt.Sprintf("No certificate with id %q exists in this organization.", certID)).
			WithSuggestions(
				"Check the value passed to --cert-id and try again",
				"Run: circleci certificate list",
			).
			WithExitCode(clierrors.ExitNotFound)
	}
	if httpcl.HasStatusCode(err, http.StatusConflict) {
		return clierrors.New("signing_config.duplicate_name", "Signing config name already in use",
			fmt.Sprintf("A signing config named %q already exists in this organization.", name)).
			WithSuggestions(
				"Pick a different --name",
				"Or delete the existing config first: circleci signing-config delete <id> --force",
			).
			WithExitCode(clierrors.ExitAPIError)
	}
	return apiErr(err, name)
}

// --- signing-config create ---

func newCreateCmd() *cobra.Command {
	var (
		orgID       string
		name        string
		certID      string
		profilePath []string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an iOS signing config",
		Long: heredoc.Doc(`
			Create a signing config that pairs a previously-uploaded certificate
			with one or more provisioning profiles. The signing config name is what
			you reference in your pipeline config under 'code_signing'.

			The organization is inferred from the current git repository's remote
			unless overridden with --org-id.

			Each --profile flag points to a single provisioning profile file on
			disk. The file is read and base64-encoded locally. Repeat the flag to
			add additional profiles.

			JSON fields: id, name, cert_id
		`),
		Example: heredoc.Doc(`
			# Create a signing config (org inferred from git remote)
			$ circleci signing-config create \
			    --name production-signing \
			    --cert-id <cert-id> \
			    --profile ./MyApp.mobileprovision

			# Multiple profiles
			$ circleci signing-config create \
			    --name multi-target-signing \
			    --cert-id <cert-id> \
			    --profile ./MyApp.mobileprovision \
			    --profile ./MyAppExtension.mobileprovision

			# Explicit org and capture the id for scripting
			$ circleci signing-config create --org-id <org-uuid> --name prod --cert-id <cert-id> --profile ./p.mobileprovision --json --jq -r '.id'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return cmdutil.RequireFlag("name")
			}
			if certID == "" {
				return cmdutil.RequireFlag("cert-id")
			}
			if len(profilePath) == 0 {
				return cmdutil.RequireFlag("profile")
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)

			profiles := make([]apiclient.IOSProvisioningProfile, len(profilePath))
			for i, p := range profilePath {
				fileName, blob, err := iossigning.EncodeFile(p)
				if err != nil {
					return clierrors.New("signing_config.profile_file_unreadable", "Cannot read provisioning profile", err.Error()).
						WithExitCode(clierrors.ExitBadArguments)
				}
				profiles[i] = apiclient.IOSProvisioningProfile{FileName: fileName, Blob: blob}
			}

			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			resolvedOrgID, err := cmdutil.ResolveOrgID(ctx, client, orgID, "circleci signing-config create")
			if err != nil {
				return err
			}
			id, err := client.CreateIOSSigningConfig(ctx, resolvedOrgID, name, certID, profiles)
			if err != nil {
				return createAPIErr(err, name, certID)
			}

			if jsonOut {
				return iostream.PrintJSON(ctx, map[string]string{
					"id":      id,
					"name":    name,
					"cert_id": certID,
				})
			}
			iostream.Printf(ctx, "%s Created signing config %q (id: %s)\n", iostream.SymbolOK(ctx), name, id)
			return nil
		},
	}

	cmd.Flags().StringVar(&orgID, "org-id", "", "CircleCI organization UUID; defaults to the org of the current git project")
	cmd.Flags().StringVar(&name, "name", "", "Name for the signing config (referenced in pipeline config)")
	cmd.Flags().StringVar(&certID, "cert-id", "", "ID of an uploaded certificate (see: circleci certificate list)")
	cmd.Flags().StringArrayVar(&profilePath, "profile", nil, "Path to a provisioning profile file (repeatable)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// --- signing-config list ---

type listEntry struct {
	ID                   string                             `json:"id"`
	Name                 string                             `json:"name,omitempty"`
	Certificate          *apiclient.IOSCertificateRef       `json:"certificate,omitempty"`
	ProvisioningProfiles []apiclient.IOSProvisioningProfile `json:"provisioning_profiles,omitempty"`
}

func newListCmd() *cobra.Command {
	var (
		orgID   string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List iOS signing configs",
		Long: heredoc.Doc(`
			List the iOS signing configs defined for your organization.

			The organization is inferred from the current git repository's remote
			unless overridden with --org-id.

			JSON fields: id, name, certificate, provisioning_profiles
		`),
		Example: heredoc.Doc(`
			# List signing configs (org inferred from git remote)
			$ circleci signing-config list

			# List for a specific org
			$ circleci signing-config list --org-id <org-uuid>

			# Output as JSON
			$ circleci signing-config list --json

			# Get signing config names only
			$ circleci signing-config list --json --jq '.[].name'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			resolvedOrgID, err := cmdutil.ResolveOrgID(ctx, client, orgID, "circleci signing-config list")
			if err != nil {
				return err
			}
			return runList(ctx, client, resolvedOrgID, jsonOut)
		},
	}

	cmd.Flags().StringVar(&orgID, "org-id", "", "CircleCI organization UUID; defaults to the org of the current git project")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runList(ctx context.Context, client *apiclient.Client, orgID string, jsonOut bool) error {
	configs, err := client.ListIOSSigningConfigs(ctx, orgID)
	if err != nil {
		return apiErr(err, orgID)
	}

	entries := make([]listEntry, len(configs))
	for i, c := range configs {
		entries[i] = listEntry{
			ID:                   c.ID,
			Name:                 c.Name,
			Certificate:          c.Certificate,
			ProvisioningProfiles: c.ProvisioningProfiles,
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(entries) == 0 {
		iostream.ErrPrintln(ctx, "No signing configs found.")
		return nil
	}

	tbl := mdtable.New("ID", "Name", "Certificate Name", "Profiles")
	for _, e := range entries {
		certFile := ""
		if e.Certificate != nil {
			certFile = e.Certificate.FileName
		}
		tbl.Row(e.ID, e.Name, certFile, strconv.Itoa(len(e.ProvisioningProfiles)))
	}
	iostream.PrintMarkdown(ctx, "# iOS Signing Configs\n"+tbl.Render())
	return nil
}

// --- signing-config delete ---

func newDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <signing-config-id>",
		Short: "Delete an iOS signing config",
		Long: heredoc.Doc(`
			Permanently remove an iOS signing config from your organization.

			This action is irreversible. Pipelines that reference the signing config
			by name will fail until they are updated.

			In a terminal, you will be prompted to confirm before deleting.
			Use --force (-f) to skip the prompt for scripting.
		`),
		Example: heredoc.Doc(`
			# Delete a signing config (with confirmation)
			$ circleci signing-config delete <signing-config-id>

			# Delete without confirmation
			$ circleci signing-config delete <signing-config-id> --force
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "signing-config-id"); err != nil {
				return err
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			id := args[0]

			if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
				fmt.Sprintf("Delete signing config %q? Pipelines using it will fail until updated.", id),
				clierrors.New("signing_config.delete_aborted", "Deletion aborted",
					"Signing config deletion was not confirmed.").
					WithExitCode(clierrors.ExitCancelled),
				clierrors.New("signing_config.delete_requires_force", "Deletion requires --force",
					fmt.Sprintf("Deleting signing config %q is irreversible.", id)).
					WithExitCode(clierrors.ExitCancelled),
			); err != nil {
				return err
			}

			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			if err := client.DeleteIOSSigningConfig(ctx, id); err != nil {
				return apiErr(err, id)
			}
			iostream.Printf(ctx, "%s Deleted signing config %s\n", iostream.SymbolOK(ctx), id)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")

	return cmd
}
