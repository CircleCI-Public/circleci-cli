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

// Command packagecloud uploads Linux nfpm packages (.rpm/.deb) to packagecloud.io.
//
// goreleaser only pushes to packagecloud under a Pro licence, so this small
// release-time tool does it against the public REST API instead. Each package is
// published to packagecloud's generic distribution for its format ("rpm_any" for
// rpm, "any" for deb) so a single upload serves every distro version.
//
// Usage:
//
//	PACKAGECLOUD_TOKEN=... go run ./cmd/packagecloud --repo circleci/circleci dist/*.rpm dist/*.deb
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var repo string
	var dryRun bool
	cmd := &cobra.Command{
		Use:          "packagecloud [flags] <package>...",
		Short:        "Upload Linux nfpm packages (.rpm/.deb) to packagecloud.io",
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// FromCmd installs Streams into the context so httpcl's
			// iostream.DebugContext logging works; --debug enables it.
			ctx := iostream.FromCmd(cmd.Context(), cmd, "")
			return run(ctx, repo, args, dryRun)
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "circleci/circleci", "packagecloud repository as <user>/<repo>")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate the packages and print what would be uploaded, without contacting packagecloud")
	cmd.Flags().Bool("debug", false, "log HTTP requests to stderr")
	return cmd
}

func run(ctx context.Context, repo string, files []string, dryRun bool) error {
	user, name, ok := strings.Cut(repo, "/")
	if !ok || user == "" || name == "" {
		return fmt.Errorf("--repo must be <user>/<repo>, got %q", repo)
	}

	// Validate package type and existence up front, so --dry-run surfaces bad
	// inputs without contacting packagecloud.
	for _, f := range files {
		if ext := pkgExt(f); ext != "rpm" && ext != "deb" {
			return fmt.Errorf("%s: unsupported package type %q", f, ext)
		}
		if _, err := os.Stat(f); err != nil {
			return err
		}
	}

	if dryRun {
		for _, f := range files {
			iostream.InfoContext(ctx, "would publish",
				"package", filepath.Base(f),
				"repo", repo,
				"distro", anyDistro[pkgExt(f)],
			)
		}
		return nil
	}

	token := os.Getenv("PACKAGECLOUD_TOKEN")
	if token == "" {
		return errors.New("PACKAGECLOUD_TOKEN must be set")
	}

	pc := newClient(token)

	// The upload API keys packages by a numeric distro_version_id, so resolve the
	// ids for the generic "any" distributions once up front.
	ids, err := pc.resolveAnyDistroIDs(ctx)
	if err != nil {
		return err
	}

	iostream.InfoContext(ctx, "publishing packages",
		"count", len(files),
		"repo", repo,
	)
	for _, f := range files {
		if err := pc.push(ctx, user, name, f, ids[pkgExt(f)]); err != nil {
			return err
		}
	}
	return nil
}

// pkgExt returns the package type ("rpm"/"deb") from a file's extension.
func pkgExt(path string) string {
	return strings.TrimPrefix(filepath.Ext(path), ".")
}

// client is a thin packagecloud REST API client over httpcl.
type client struct {
	http *httpcl.Client
}

func newClient(token string) *client {
	return &client{
		http: httpcl.New(httpcl.Config{
			BaseURL: "https://packagecloud.io",
			// packagecloud authenticates with HTTP Basic: token as username, blank password.
			AuthHeader: "Authorization",
			AuthToken:  "Basic " + base64.StdEncoding.EncodeToString([]byte(token+":")),
			UserAgent:  "circleci-cli-release",
			Timeout:    2 * time.Minute,
		}),
	}
}

// push uploads a single package as multipart/form-data. An "already published"
// 422 is treated as success so re-running a release is idempotent.
func (c *client) push(ctx context.Context, user, repo, path string, distroVersionID int) error {
	body, contentType, err := multipartBody(path, distroVersionID)
	if err != nil {
		return err
	}

	status, err := c.http.Call(ctx, httpcl.NewRequest("POST", "/api/v1/repos/%s/%s/packages.json",
		httpcl.RouteParams(user, repo),
		httpcl.RawBody(body, contentType),
	))
	switch {
	case err == nil:
		iostream.InfoContext(ctx, "pushed",
			"package", filepath.Base(path),
			"status", status,
		)
		return nil
	case status == 422 && alreadyPublished(err):
		iostream.InfoContext(ctx, "skipped, already published",
			"package", filepath.Base(path),
			"status", status,
		)
		return nil
	default:
		return fmt.Errorf("uploading %s: %w", filepath.Base(path), err)
	}
}

func multipartBody(path string, distroVersionID int) (body []byte, contentType string, err error) {
	file, err := os.Open(path) //nolint:gosec // path is an operator-supplied package file
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = file.Close() }()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("package[distro_version_id]", strconv.Itoa(distroVersionID)); err != nil {
		return nil, "", err
	}
	fw, err := w.CreateFormFile("package[package_file]", filepath.Base(path))
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(fw, file); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

func alreadyPublished(err error) bool {
	var httpErr *httpcl.HTTPError
	return errors.As(err, &httpErr) && bytes.Contains(httpErr.Body, []byte("already been taken"))
}

// anyDistro maps a package extension to packagecloud's generic, distro-agnostic
// distribution. A single upload to one of these serves every version of that
// package format. The names are NOT symmetric: rpm uses "rpm_any", but
// deb/dsc use "any". See https://packagecloud.io/docs#announcing-fat-manifests.
var anyDistro = map[string]string{
	"rpm": "rpm_any",
	"deb": "any",
}

// resolveAnyDistroIDs returns the distro_version_id of the generic distributions
// in anyDistro, keyed by package extension ("rpm"/"deb").
func (c *client) resolveAnyDistroIDs(ctx context.Context) (map[string]int, error) {
	// Distributions are grouped by package type (deb, rpm, dsc, ...). Decode the
	// whole document and search every group so we don't depend on which group a
	// generic distribution happens to live under.
	var dists map[string][]distro
	if _, err := c.http.Call(ctx, httpcl.NewRequest("GET", "/api/v1/distributions.json",
		httpcl.JSONDecoder(&dists),
	)); err != nil {
		return nil, fmt.Errorf("listing distributions: %w", err)
	}

	ids := map[string]int{}
	for ext, name := range anyDistro {
		id, ok := findDistroVersion(dists, name)
		if !ok {
			return nil, fmt.Errorf("packagecloud distribution %q not found among %v", name, versionNames(dists))
		}
		ids[ext] = id
	}
	return ids, nil
}

// findDistroVersion returns the id of the distro version whose index_name equals
// name (e.g. "rpm_any", "any"), searching every distribution across all groups.
func findDistroVersion(dists map[string][]distro, name string) (int, bool) {
	for _, list := range dists {
		for _, d := range list {
			for _, v := range d.Versions {
				if v.IndexName == name {
					return v.ID, true
				}
			}
		}
	}
	return 0, false
}

// versionNames returns every distro version index_name across all groups, for error messages.
func versionNames(dists map[string][]distro) []string {
	var names []string
	for _, list := range dists {
		for _, d := range list {
			for _, v := range d.Versions {
				names = append(names, v.IndexName)
			}
		}
	}
	return names
}

type distro struct {
	Versions []struct {
		ID        int    `json:"id"`
		IndexName string `json:"index_name"`
	} `json:"versions"`
}
