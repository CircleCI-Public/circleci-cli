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

// Command cloudsmith publishes Linux packages (.deb and .rpm) to a Cloudsmith
// repository via the public REST API.
//
// Like cmd/packagecloud, this is a small release-time tool: goreleaser can't push
// to Cloudsmith without a Pro licence, so we drive the API directly instead.
//
// Uploading a package is a two-step flow:
//
//  1. PUT the file to upload.cloudsmith.io with a Content-Sha256 header. The
//     response returns an identifier for the stored blob.
//  2. POST that identifier to api.cloudsmith.io/v1/packages/.../upload/<format>/
//     along with the target distribution.
//
// Every package is published to the generic "any-distro/any-version"
// distribution, so a single upload serves every distro and version.
//
// Usage:
//
//	CLOUDSMITH_API_KEY=... go run ./cmd/cloudsmith push deb --repo circleci-deps/public dist/*.deb
//	CLOUDSMITH_API_KEY=... go run ./cmd/cloudsmith push rpm --repo circleci-deps/public dist/*.rpm
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// defaultDistribution is Cloudsmith's generic "fat" distribution: a single upload
// to it serves every distro and version, for both deb and rpm packages.
const defaultDistribution = "any-distro/any-version"

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "cloudsmith <command>",
		Short:        "Publish packages to Cloudsmith",
		SilenceUsage: true,
	}
	// --debug is read by iostream.FromCmd to enable httpcl's request logging; as a
	// persistent flag it's available to every subcommand.
	cmd.PersistentFlags().Bool("debug", false, "log HTTP requests to stderr")
	cmd.AddCommand(pushCmd())
	return cmd
}

func pushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push <format> <package>...",
		Short: "Upload Linux packages to Cloudsmith",
	}
	cmd.AddCommand(
		pushFormatCmd("deb", "Debian (.deb)", true),
		pushFormatCmd("rpm", "RPM (.rpm)", false),
	)
	return cmd
}

// pushFormatCmd builds the `push deb` / `push rpm` subcommands. They share all
// logic; only the package format, file extension, and whether a --component flag
// applies (deb only) differ. withComponent toggles the component flag, which the
// rpm format does not support.
func pushFormatCmd(format, label string, withComponent bool) *cobra.Command {
	opts := pushOpts{format: format}
	cmd := &cobra.Command{
		Use:   format + " [flags] <package>...",
		Short: "Upload " + label + " packages to Cloudsmith",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd, "")
			return runPush(ctx, opts, args)
		},
	}
	cmd.Flags().StringVar(&opts.repo, "repo", "circleci-deps/public", "Cloudsmith repository as <owner>/<repo>")
	cmd.Flags().StringVar(&opts.distribution, "distribution", defaultDistribution, "distribution to publish each package to")
	cmd.Flags().BoolVar(&opts.republish, "republish", false, "overwrite an existing package with the same attributes instead of failing as a duplicate")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "validate the packages and print what would be uploaded, without contacting Cloudsmith")
	if withComponent {
		cmd.Flags().StringVar(&opts.component, "component", "main", "repository component (channel) to publish into")
	}
	return cmd
}

type pushOpts struct {
	format       string // "deb" or "rpm"
	repo         string
	component    string // deb only; empty for rpm
	distribution string
	republish    bool
	dryRun       bool
}

// requireKey reads the Cloudsmith API key from the environment.
func requireKey() (string, error) {
	key := os.Getenv("CLOUDSMITH_API_KEY")
	if key == "" {
		return "", errors.New("CLOUDSMITH_API_KEY must be set")
	}
	return key, nil
}

func runPush(ctx context.Context, opts pushOpts, files []string) error {
	owner, repo, ok := strings.Cut(opts.repo, "/")
	if !ok || owner == "" || repo == "" {
		return fmt.Errorf("--repo must be <owner>/<repo>, got %q", opts.repo)
	}
	if opts.distribution == "" {
		return errors.New("--distribution must not be empty")
	}

	// Validate package type and existence up front so --dry-run surfaces bad
	// inputs without contacting Cloudsmith.
	for _, f := range files {
		if ext := pkgExt(f); ext != opts.format {
			return fmt.Errorf("%s: expected a .%s package, got %q", f, opts.format, ext)
		}
		if _, err := os.Stat(f); err != nil {
			return err
		}
	}

	if opts.dryRun {
		for _, f := range files {
			iostream.InfoContext(ctx, "would publish",
				"package", filepath.Base(f),
				"repo", opts.repo,
				"distribution", opts.distribution,
			)
		}
		return nil
	}

	key, err := requireKey()
	if err != nil {
		return err
	}
	cs := newClient(key)

	iostream.InfoContext(ctx, "publishing packages",
		"count", len(files),
		"repo", opts.repo,
		"format", opts.format,
	)
	for _, f := range files {
		if err := cs.publish(ctx, owner, repo, f, opts); err != nil {
			return err
		}
	}
	return nil
}

// pkgExt returns the package type ("deb"/"rpm") from a file's extension.
func pkgExt(path string) string {
	return strings.TrimPrefix(filepath.Ext(path), ".")
}

const (
	uploadURL = "https://upload.cloudsmith.io"
	apiURL    = "https://api.cloudsmith.io"
)

// client is a thin Cloudsmith REST API client. Cloudsmith splits raw uploads
// (upload.cloudsmith.io) from the package API (api.cloudsmith.io), so it holds a
// client for each host. Both authenticate with the X-Api-Key header.
type client struct {
	upload *httpcl.Client
	api    *httpcl.Client
}

func newClient(apiKey string) *client {
	return &client{
		upload: httpcl.New(httpcl.Config{
			BaseURL:    uploadURL,
			AuthHeader: "X-Api-Key",
			AuthToken:  apiKey,
			UserAgent:  "circleci-cli-release",
			Timeout:    5 * time.Minute,
		}),
		api: httpcl.New(httpcl.Config{
			BaseURL:    apiURL,
			AuthHeader: "X-Api-Key",
			AuthToken:  apiKey,
			UserAgent:  "circleci-cli-release",
			Timeout:    2 * time.Minute,
		}),
	}
}

// publish uploads a single package and registers it. A duplicate (a package with
// the same attributes already exists and --republish was not set) is treated as
// success so re-running a release is idempotent.
func (c *client) publish(ctx context.Context, owner, repo, path string, opts pushOpts) error {
	data, err := os.ReadFile(path) //nolint:gosec // path is an operator-supplied package file
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])

	identifier, err := c.uploadFile(ctx, owner, repo, filepath.Base(path), data, sha)
	if err != nil {
		return fmt.Errorf("uploading %s: %w", filepath.Base(path), err)
	}

	pkg, status, err := c.createPackage(ctx, owner, repo, identifier, opts)
	switch {
	case err == nil:
		iostream.InfoContext(ctx, "published",
			"package", filepath.Base(path),
			"distribution", opts.distribution,
			"slug", pkg.SlugPerm,
			"url", pkg.SelfHTMLURL,
		)
		return nil
	case isDuplicate(status, err):
		iostream.InfoContext(ctx, "skipped, already published",
			"package", filepath.Base(path),
			"distribution", opts.distribution,
		)
		return nil
	default:
		return fmt.Errorf("publishing %s to %s: %w", filepath.Base(path), opts.distribution, err)
	}
}

// uploadFile PUTs the package bytes to the upload host and returns the identifier
// for the stored blob.
func (c *client) uploadFile(ctx context.Context, owner, repo, filename string, data []byte, sha string) (string, error) {
	var resp struct {
		Identifier string `json:"identifier"`
	}
	if _, err := c.upload.Call(ctx, httpcl.NewRequest("PUT", "/%s/%s/%s",
		httpcl.RouteParams(owner, repo, filename),
		httpcl.RawBody(data, "application/octet-stream"),
		httpcl.Header("Content-Sha256", sha),
		httpcl.JSONDecoder(&resp),
	)); err != nil {
		return "", err
	}
	if resp.Identifier == "" {
		return "", errors.New("upload returned an empty identifier")
	}
	return resp.Identifier, nil
}

// pkgResult holds the fields of the create-package response worth logging.
type pkgResult struct {
	SlugPerm    string `json:"slug_perm"`
	SelfHTMLURL string `json:"self_html_url"`
	Filename    string `json:"filename"`
	Version     string `json:"version"`
}

// createPackage registers a previously uploaded blob as a package of opts.format
// in the given distribution. distribution and package_file are the only required
// fields; component applies to deb only and is omitted when empty.
func (c *client) createPackage(ctx context.Context, owner, repo, identifier string, opts pushOpts) (*pkgResult, int, error) {
	body := struct {
		PackageFile  string `json:"package_file"`
		Distribution string `json:"distribution"`
		Component    string `json:"component,omitempty"`
		Republish    bool   `json:"republish,omitempty"`
	}{
		PackageFile:  identifier,
		Distribution: opts.distribution,
		Component:    opts.component,
		Republish:    opts.republish,
	}

	var pkg pkgResult
	status, err := c.api.Call(ctx, httpcl.NewRequest("POST", "/v1/packages/%s/%s/upload/%s/",
		httpcl.RouteParams(owner, repo, opts.format),
		httpcl.Body(body),
		httpcl.JSONDecoder(&pkg),
	))
	return &pkg, status, err
}

// isDuplicate reports whether a create failed only because an identical package
// already exists and republishing was not requested.
func isDuplicate(status int, err error) bool {
	if status != 400 && status != 422 {
		return false
	}
	var httpErr *httpcl.HTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	body := bytes.ToLower(httpErr.Body)
	return bytes.Contains(body, []byte("already exist")) ||
		bytes.Contains(body, []byte("duplicate")) ||
		bytes.Contains(body, []byte("republish"))
}
