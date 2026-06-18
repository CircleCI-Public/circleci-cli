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

// Command packagecloud talks to the packagecloud.io REST API: it publishes Linux
// nfpm packages (.rpm/.deb) and lists, shows, and creates repositories.
//
// goreleaser only pushes to packagecloud under a Pro licence, so this small
// release-time tool does it against the public REST API instead. Each package is
// published to packagecloud's generic distribution for its format ("rpm_any" for
// rpm, "any" for deb) so a single upload serves every distro version.
//
// Usage:
//
//	PACKAGECLOUD_TOKEN=... go run ./cmd/packagecloud pkg push --repo circleci/circleci dist/*.rpm dist/*.deb
//	PACKAGECLOUD_TOKEN=... go run ./cmd/packagecloud repo list
//	PACKAGECLOUD_TOKEN=... go run ./cmd/packagecloud repo get circleci/runner
//	PACKAGECLOUD_TOKEN=... go run ./cmd/packagecloud repo create circleci/circleci
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
	"text/tabwriter"
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
	cmd := &cobra.Command{
		Use:          "packagecloud <command>",
		Short:        "Publish packages to, and inspect, packagecloud.io",
		SilenceUsage: true,
	}
	// --debug is read by iostream.FromCmd to enable httpcl's request logging; as a
	// persistent flag it's available to every subcommand.
	cmd.PersistentFlags().Bool("debug", false, "log HTTP requests to stderr")
	cmd.AddCommand(pkgCmd(), repoCmd())
	return cmd
}

func pkgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pkg <command>",
		Short: "Manage packagecloud packages",
	}
	cmd.AddCommand(pushCmd())
	return cmd
}

func pushCmd() *cobra.Command {
	var repo string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "push [flags] <package>...",
		Short: "Upload Linux nfpm packages (.rpm/.deb) to packagecloud.io",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd, "")
			return runPush(ctx, repo, args, dryRun)
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "circleci/circleci", "packagecloud repository as <user>/<repo>")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate the packages and print what would be uploaded, without contacting packagecloud")
	return cmd
}

func repoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo <command>",
		Short: "Manage packagecloud repositories",
	}
	cmd.AddCommand(repoListCmd(), repoGetCmd(), repoCreateCmd())
	return cmd
}

func repoListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the repositories the API token can access",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd, "")
			return runRepoList(ctx)
		},
	}
}

func repoGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <user>/<repo>",
		Short: "Show a single packagecloud repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd, "")
			return runRepoGet(ctx, args[0])
		},
	}
}

func repoCreateCmd() *cobra.Command {
	var private bool
	cmd := &cobra.Command{
		Use:   "create <user>/<repo>",
		Short: "Create a packagecloud repository",
		// The API creates the repository under the account the token belongs to,
		// so the <user> in the argument is informational; the created repository's
		// real owner is shown in the output.
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd, "")
			return runRepoCreate(ctx, args[0], private)
		},
	}
	cmd.Flags().BoolVar(&private, "private", false, "create a private repository")
	return cmd
}

// requireToken reads the packagecloud API token from the environment.
func requireToken() (string, error) {
	token := os.Getenv("PACKAGECLOUD_TOKEN")
	if token == "" {
		return "", errors.New("PACKAGECLOUD_TOKEN must be set")
	}
	return token, nil
}

func runPush(ctx context.Context, repo string, files []string, dryRun bool) error {
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

	token, err := requireToken()
	if err != nil {
		return err
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

func runRepoList(ctx context.Context) error {
	token, err := requireToken()
	if err != nil {
		return err
	}

	repos, err := newClient(token).listRepos(ctx)
	if err != nil {
		return err
	}

	// tabwriter buffers writes and reports any error on Flush, so the intermediate
	// writes can be discarded.
	w := tabwriter.NewWriter(iostream.Out(ctx), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "REPOSITORY\tVISIBILITY\tCREATED\tPRIVATE")
	for _, r := range repos {
		visibility := "public"
		if r.Private {
			visibility = "private"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%t\n", r.FQName, visibility, r.CreatedAt, r.Private)
	}
	return w.Flush()
}

func runRepoGet(ctx context.Context, fqname string) error {
	user, name, err := splitRepo(fqname)
	if err != nil {
		return err
	}
	token, err := requireToken()
	if err != nil {
		return err
	}
	repo, err := newClient(token).getRepo(ctx, user, name)
	if err != nil {
		return err
	}
	// The show response doesn't always echo fqname; fall back to what was asked for.
	if repo.FQName == "" {
		repo.FQName = user + "/" + name
	}
	return printRepoDetail(ctx, repo)
}

func runRepoCreate(ctx context.Context, fqname string, private bool) error {
	user, name, err := splitRepo(fqname)
	if err != nil {
		return err
	}
	token, err := requireToken()
	if err != nil {
		return err
	}
	pc := newClient(token)
	if err := pc.createRepo(ctx, name, private); err != nil {
		return err
	}
	iostream.InfoContext(ctx, "created repository", "repo", fqname)

	// The create response is minimal, so re-fetch to show the same details as
	// `repo get`. The repository is owned by the token's account, which should
	// match the requested <user>.
	repo, err := pc.getRepo(ctx, user, name)
	if err != nil {
		return err
	}
	if repo.FQName == "" {
		repo.FQName = user + "/" + name
	}
	return printRepoDetail(ctx, repo)
}

// splitRepo splits a "<user>/<repo>" argument into its parts.
func splitRepo(fqname string) (user, name string, err error) {
	user, name, ok := strings.Cut(fqname, "/")
	if !ok || user == "" || name == "" {
		return "", "", fmt.Errorf("repository must be <user>/<repo>, got %q", fqname)
	}
	return user, name, nil
}

// printRepoDetail writes a repository's details as an aligned key/value table.
func printRepoDetail(ctx context.Context, r *repoDetail) error {
	url := ""
	if r.Path != "" {
		url = baseURL + r.Path
	}
	rows := [][2]string{{"REPOSITORY", r.FQName}}
	if r.RepoType != "" {
		rows = append(rows, [2]string{"TYPE", r.RepoType})
	}
	rows = append(rows,
		[2]string{"INSTALLS", strconv.Itoa(r.TotalInstalls)},
		[2]string{"CREATED", r.CreatedAt},
		[2]string{"UPDATED", r.UpdatedAt},
		[2]string{"URL", url},
	)

	w := tabwriter.NewWriter(iostream.Out(ctx), 0, 0, 2, ' ', 0)
	for _, row := range rows {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", row[0], row[1])
	}
	return w.Flush()
}

// pkgExt returns the package type ("rpm"/"deb") from a file's extension.
func pkgExt(path string) string {
	return strings.TrimPrefix(filepath.Ext(path), ".")
}

// baseURL is the packagecloud API and website host.
const baseURL = "https://packagecloud.io"

// client is a thin packagecloud REST API client over httpcl.
type client struct {
	http *httpcl.Client
}

func newClient(token string) *client {
	return &client{
		http: httpcl.New(httpcl.Config{
			BaseURL: baseURL,
			// packagecloud authenticates with HTTP Basic: token as username, blank password.
			AuthHeader: "Authorization",
			AuthToken:  "Basic " + base64.StdEncoding.EncodeToString([]byte(token+":")),
			UserAgent:  "circleci-cli-release",
			Timeout:    2 * time.Minute,
		}),
	}
}

// repository holds the fields the list endpoint returns and the table renders.
type repository struct {
	FQName    string `json:"fqname"` // "<user>/<repo>"
	Private   bool   `json:"private"`
	CreatedAt string `json:"created_at"`
}

// listRepos returns every repository the API token can access.
func (c *client) listRepos(ctx context.Context) ([]repository, error) {
	var repos []repository
	if _, err := c.http.Call(ctx, httpcl.NewRequest("GET", "/api/v1/repos.json",
		httpcl.JSONDecoder(&repos),
	)); err != nil {
		return nil, fmt.Errorf("listing repositories: %w", err)
	}
	return repos, nil
}

// repoDetail is the GET /api/v1/repos/:user/:repo.json response. It does not
// include the "private" flag or a browser URL (the browser URL is baseURL+Path).
type repoDetail struct {
	Name          string `json:"name"`
	FQName        string `json:"fqname"` // "<user>/<repo>"
	Path          string `json:"path"`   // API/browser path, e.g. /circleci/runner
	RepoType      string `json:"repo_type"`
	TotalInstalls int    `json:"total_installs_count"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// getRepo returns a single repository's details.
func (c *client) getRepo(ctx context.Context, user, name string) (*repoDetail, error) {
	var repo repoDetail
	if _, err := c.http.Call(ctx, httpcl.NewRequest("GET", "/api/v1/repos/%s/%s.json",
		httpcl.RouteParams(user, name),
		httpcl.JSONDecoder(&repo),
	)); err != nil {
		return nil, fmt.Errorf("getting repository %s/%s: %w", user, name, err)
	}
	return &repo, nil
}

// createRepo creates a repository named name under the token's account. The API
// has no owner parameter, so the repository belongs to whoever the token
// authenticates as. private toggles repository visibility.
func (c *client) createRepo(ctx context.Context, name string, private bool) error {
	body := struct {
		Repository struct {
			Name    string `json:"name"`
			Private bool   `json:"private"`
		} `json:"repository"`
	}{}
	body.Repository.Name = name
	body.Repository.Private = private

	if _, err := c.http.Call(ctx, httpcl.NewRequest("POST", "/api/v1/repos.json",
		httpcl.Body(body),
	)); err != nil {
		return fmt.Errorf("creating repository %q: %w", name, err)
	}
	return nil
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
