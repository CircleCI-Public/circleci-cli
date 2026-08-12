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

// Command virustotal submits release artifacts to VirusTotal via the public v3
// API so every published binary has an antivirus scan on record.
//
// Like cmd/packagecloud and cmd/cloudsmith, this is a small release-time tool
// driving a REST API directly. It is deliberately best-effort: submission is a
// nice-to-have record, not a release gate. VirusTotal frequently false-flags Go
// binaries, so this tool never inspects the verdict and never fails the release —
// it uploads, logs where each report will appear, and moves on. A missing API
// key or a per-file upload error is logged and skipped, not fatal.
//
// Uploading a file is one of two flows depending on size:
//
//   - ≤32 MiB: POST the file as multipart/form-data to /api/v3/files.
//   - >32 MiB: GET a one-time upload URL from /api/v3/files/upload_url, then POST
//     the file to that URL.
//
// Both authenticate with the x-apikey header and return an analysis object. The
// permanent report lives at https://www.virustotal.com/gui/file/<sha256>.
//
// Usage:
//
//	VT_APIKEY=... go run ./cmd/virustotal submit dist/*.tar.gz dist/*.zip dist/*.deb dist/*.rpm
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostreamcobra"
)

const (
	apiBase = "https://www.virustotal.com"
	// directUploadLimit is the largest file the /api/v3/files endpoint accepts.
	// Anything larger must be uploaded via a one-time URL from
	// /api/v3/files/upload_url.
	directUploadLimit = 32 << 20 // 32 MiB
	// guiFileBase is the permanent, human-readable report URL, keyed by the file's
	// SHA-256. It resolves as soon as the analysis completes.
	guiFileBase = "https://www.virustotal.com/gui/file/"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "virustotal <command>",
		Short:        "Submit release artifacts to VirusTotal",
		SilenceUsage: true,
	}
	// --debug is read by iostreamcobra.FromCmd to enable httpcl's request logging.
	cmd.PersistentFlags().Bool("debug", false, "log HTTP requests to stderr")
	cmd.AddCommand(submitCmd())
	return cmd
}

func submitCmd() *cobra.Command {
	var opts submitOpts
	cmd := &cobra.Command{
		Use:   "submit [flags] <file>...",
		Short: "Upload files to VirusTotal for scanning",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostreamcobra.FromCmd(cmd.Context(), cmd, "")
			return runSubmit(ctx, opts, args)
		},
	}
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "list the files that would be submitted, without contacting VirusTotal")
	return cmd
}

type submitOpts struct {
	dryRun bool
}

func runSubmit(ctx context.Context, opts submitOpts, args []string) error {
	// Shell globs that match nothing (e.g. no .zip in dist/) expand to the literal
	// pattern, and dist/ holds directories and metadata alongside the artifacts.
	// Keep only regular files that exist, logging what we skip.
	var files []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		switch {
		case err != nil:
			iostream.InfoContext(ctx, "skipping, not found", "path", arg)
		case info.IsDir():
			iostream.InfoContext(ctx, "skipping, not a file", "path", arg)
		default:
			files = append(files, arg)
		}
	}
	if len(files) == 0 {
		iostream.InfoContext(ctx, "no files to submit")
		return nil
	}

	if opts.dryRun {
		for _, f := range files {
			iostream.InfoContext(ctx, "would submit", "file", filepath.Base(f))
		}
		return nil
	}

	// Submission is best-effort: without a key we skip rather than fail the release.
	key := os.Getenv("VT_APIKEY")
	if key == "" {
		iostream.InfoContext(ctx, "skipping VirusTotal submission, VT_APIKEY not set")
		return nil
	}
	vt := newClient(key)

	iostream.InfoContext(ctx, "submitting to VirusTotal", "count", len(files))
	var failed int
	for _, f := range files {
		if err := vt.submit(ctx, f); err != nil {
			// Never gate the release on a VirusTotal error — log and continue.
			iostream.InfoContext(ctx, "submission failed", "file", filepath.Base(f), "error", err.Error())
			failed++
		}
	}
	if failed > 0 {
		iostream.InfoContext(ctx, "finished with errors", "submitted", len(files)-failed, "failed", failed)
	} else {
		iostream.InfoContext(ctx, "all files submitted", "count", len(files))
	}
	return nil
}

// client is a thin VirusTotal v3 API client. api targets www.virustotal.com;
// upload has no base URL so it can POST to the absolute one-time upload URL the
// API hands back for large files. Both authenticate with the x-apikey header.
type client struct {
	api    *httpcl.Client
	upload *httpcl.Client
}

func newClient(apiKey string) *client {
	cfg := httpcl.Config{
		AuthHeader: "x-apikey",
		AuthToken:  apiKey,
		UserAgent:  "circleci-cli-release",
		Timeout:    10 * time.Minute,
	}
	apiCfg := cfg
	apiCfg.BaseURL = apiBase
	return &client{
		api:    httpcl.New(apiCfg),
		upload: httpcl.New(cfg), // no BaseURL: routes are absolute upload URLs
	}
}

// submit uploads one file and logs where its report will appear. Small files go
// straight to /api/v3/files; large ones first fetch a one-time upload URL.
func (c *client) submit(ctx context.Context, path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // path is an operator-supplied release artifact
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])

	body, contentType, err := buildMultipart(filepath.Base(path), data)
	if err != nil {
		return err
	}

	target := "/api/v3/files"
	c2 := c.api
	if needsUploadURL(len(data)) {
		url, err := c.fetchUploadURL(ctx)
		if err != nil {
			return fmt.Errorf("fetching upload URL: %w", err)
		}
		target = url
		c2 = c.upload
	}

	var resp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if _, err := c2.Call(ctx, httpcl.NewRequest("POST", target,
		httpcl.RawBody(body, contentType),
		httpcl.JSONDecoder(&resp),
	)); err != nil {
		return err
	}

	iostream.InfoContext(ctx, "submitted",
		"file", filepath.Base(path),
		"sha256", sha,
		"analysis", resp.Data.ID,
		"report", guiFileBase+sha,
	)
	return nil
}

// fetchUploadURL requests a one-time upload URL for a file larger than the direct
// upload limit.
func (c *client) fetchUploadURL(ctx context.Context) (string, error) {
	var resp struct {
		Data string `json:"data"`
	}
	if _, err := c.api.Call(ctx, httpcl.NewRequest("GET", "/api/v3/files/upload_url",
		httpcl.JSONDecoder(&resp),
	)); err != nil {
		return "", err
	}
	if resp.Data == "" {
		return "", errors.New("empty upload URL")
	}
	return resp.Data, nil
}

// needsUploadURL reports whether a file of the given size is too large for the
// direct /api/v3/files endpoint.
func needsUploadURL(size int) bool {
	return size > directUploadLimit
}

// buildMultipart encodes data as a multipart/form-data body with a single "file"
// part, returning the body bytes and the matching Content-Type header value.
func buildMultipart(filename string, data []byte) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(data); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}
