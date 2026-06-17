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

// Package certificate implements the "circleci certificate" command group for
// managing iOS code signing certificates (.p12 files).
package certificate

import (
	"context"
	"fmt"
	"net/http"

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

// NewCertificateCmd returns the "circleci certificate" command group.
func NewCertificateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "certificate <command>",
		GroupID: "management",
		Short:   "Manage iOS code signing certificates",
		Long: heredoc.Doc(`
			Upload, list, and delete Apple .p12 code signing certificates stored in
			your CircleCI organization's secure storage.

			Certificates are referenced by ID from a signing config. Deleting a
			certificate that is referenced by a signing config will invalidate that
			signing config.
		`),
	}

	cmdutil.AddGroup(cmd, "General commands",
		newListCmd(),
	)
	cmdutil.AddGroup(cmd, "Targeted commands",
		newDeleteCmd(),
		newUploadCmd(),
	)

	return cmd
}

func apiErr(err error, subject string) *clierrors.CLIError {
	return cmdutil.APIErr(err, subject,
		"certificate.not_found", "No iOS certificate found for %q.",
		"Check the certificate ID and try again",
		"Run: circleci certificate list --org-id <org-uuid>")
}

// --- certificate upload ---

func newUploadCmd() *cobra.Command {
	var (
		orgID    string
		certPath string
		password string
		jsonOut  bool
	)

	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload a .p12 certificate",
		Long: heredoc.Doc(`
			Upload an Apple .p12 code signing certificate to your CircleCI
			organization's secure storage.

			The organization is inferred from the current git repository's remote
			unless overridden with --org-id.

			The certificate file is read from disk and base64-encoded locally
			before being sent. In a terminal, --password may be omitted and you
			will be prompted with input masking.

			Pass --password - to read the password from stdin, keeping it out of
			shell history and process listings.

			JSON fields: id, file_name
		`),
		Example: heredoc.Doc(`
			# Upload a certificate (org inferred from git remote, password prompted)
			$ circleci certificate upload --cert-file ./Certificates.p12

			# Read the password from stdin (no shell history exposure)
			$ echo "$P12_PASSWORD" | circleci certificate upload --cert-file ./Certificates.p12 --password -

			# Explicit org and capture the new cert id for scripting
			$ echo "$P12_PASSWORD" | circleci certificate upload --org-id <org-uuid> --cert-file ./Certificates.p12 --password - --json --jq -r '.id'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if certPath == "" {
				return cmdutil.RequireFlag("cert-file")
			}
			ctx := cmd.Context()
			// Validate the file before any password handling so a mistyped
			// --cert-file path doesn't waste an interactive prompt entry.
			fileName, blob, err := iossigning.EncodeFile(certPath)
			if err != nil {
				return clierrors.New("certificate.file_unreadable", "Cannot read certificate file", err.Error()).
					WithExitCode(clierrors.ExitBadArguments)
			}
			if password == "" {
				if !iostream.IsInteractive(ctx) {
					return clierrors.New("certificate.password_required", "Password required",
						"--password is required in non-interactive mode.").
						WithSuggestions(
							"Pass --password - to read it from stdin",
							"Or run in a terminal to be prompted with input masking",
						).
						WithExitCode(clierrors.ExitBadArguments)
				}
				pwd, err := iostream.PromptSecret(ctx, "Enter .p12 password")
				if err != nil {
					return clierrors.New("certificate.prompt_failed", "Password prompt failed", err.Error()).
						WithExitCode(clierrors.ExitGeneralError)
				}
				if pwd == "" {
					return clierrors.New("certificate.password_aborted", "Aborted",
						"No password was entered.").
						WithExitCode(clierrors.ExitCancelled)
				}
				password = pwd
			} else {
				pwd, err := iostream.ReadSecret(ctx, password)
				if err != nil {
					return clierrors.New("certificate.password_stdin_read_failed", "Failed to read password from stdin", err.Error()).
						WithExitCode(clierrors.ExitBadArguments)
				}
				password = pwd
			}
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			resolvedOrgID, err := cmdutil.ResolveOrgID(ctx, client, orgID, "circleci certificate upload")
			if err != nil {
				return err
			}
			certID, err := client.UploadIOSCertificate(ctx, resolvedOrgID, fileName, blob, password)
			if err != nil {
				return apiErr(err, fileName)
			}

			if jsonOut {
				return iostream.PrintJSON(ctx, map[string]string{
					"id":        certID,
					"file_name": fileName,
				})
			}
			iostream.Printf(ctx, "%s Uploaded %s (id: %s)\n", iostream.SymbolOK(ctx), fileName, certID)
			return nil
		},
	}

	cmd.Flags().StringVar(&orgID, "org-id", "", "CircleCI organization UUID; defaults to the org of the current git project")
	cmd.Flags().StringVar(&certPath, "cert-file", "", "Path to the .p12 certificate file")
	cmd.Flags().StringVar(&password, "password", "", "Password for the .p12 file. Pass - to read from stdin. Prompted if omitted in a terminal.")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// --- certificate list ---

type listEntry struct {
	ID       string `json:"id"`
	FileName string `json:"file_name,omitempty"`
	CertType string `json:"cert_type,omitempty"`
}

func newListCmd() *cobra.Command {
	var (
		orgID   string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List uploaded iOS certificates",
		Long: heredoc.Doc(`
			List Apple .p12 certificates currently stored in your organization's
			secure storage.

			The organization is inferred from the current git repository's remote
			unless overridden with --org-id.

			JSON fields: id, file_name, org_id, created_at
		`),
		Example: heredoc.Doc(`
			# List certificates (org inferred from git remote)
			$ circleci certificate list

			# List for a specific org
			$ circleci certificate list --org-id <org-uuid>

			# Output as JSON
			$ circleci certificate list --json

			# Get cert IDs only
			$ circleci certificate list --json --jq '.[].id'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			resolvedOrgID, err := cmdutil.ResolveOrgID(ctx, client, orgID, "circleci certificate list")
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
	certs, err := client.ListIOSCertificates(ctx, orgID)
	if err != nil {
		return apiErr(err, orgID)
	}

	entries := make([]listEntry, len(certs))
	for i, c := range certs {
		entries[i] = listEntry{
			ID:       c.ID,
			FileName: c.FileName,
			CertType: c.CertType,
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(entries) == 0 {
		iostream.ErrPrintln(ctx, "No certificates found.")
		return nil
	}

	tbl := mdtable.New("ID", "Certificate Name", "Type")
	for _, e := range entries {
		tbl.Row(e.ID, e.FileName, e.CertType)
	}
	iostream.PrintMarkdown(ctx, "# iOS Certificates\n"+tbl.Render())
	return nil
}

// --- certificate delete ---

func newDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <cert-id>",
		Short: "Delete an iOS certificate",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				<cert-id> is the ID of the certificate to delete
				(see: circleci certificate list).
			`),
		},
		Long: heredoc.Doc(`
			Remove an Apple certificate from your organization's secure storage.

			This action is irreversible. The server rejects the delete with an
			error if the certificate is referenced by any signing config; delete
			those signing configs first.

			In a terminal, you will be prompted to confirm before deleting.
			Use --force (-f) to skip the prompt for scripting.
		`),
		Example: heredoc.Doc(`
			# Delete a certificate (with confirmation)
			$ circleci certificate delete <cert-id>

			# Delete without confirmation
			$ circleci certificate delete <cert-id> --force
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "cert-id"); err != nil {
				return err
			}
			ctx := cmd.Context()
			certID := args[0]

			if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
				fmt.Sprintf("Delete certificate %q? This action cannot be undone.", certID),
				clierrors.New("certificate.delete_aborted", "Deletion aborted",
					"Certificate deletion was not confirmed.").
					WithExitCode(clierrors.ExitCancelled),
				clierrors.New("certificate.delete_requires_force", "Deletion requires --force",
					fmt.Sprintf("Deleting certificate %q is irreversible.", certID)).
					WithExitCode(clierrors.ExitCancelled),
			); err != nil {
				return err
			}

			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			if err := client.DeleteIOSCertificate(ctx, certID); err != nil {
				if httpcl.HasStatusCode(err, http.StatusConflict) {
					return clierrors.New("certificate.in_use", "Cannot delete certificate",
						fmt.Sprintf("Certificate %q is referenced by one or more signing configs.", certID)).
						WithSuggestions(
							"Run: circleci signing-config list --org-id <org-uuid>",
							"Delete the signing configs that reference this certificate, then retry",
						).
						WithExitCode(clierrors.ExitAPIError)
				}
				return apiErr(err, certID)
			}
			iostream.Printf(ctx, "%s Deleted certificate %s\n", iostream.SymbolOK(ctx), certID)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")

	return cmd
}
