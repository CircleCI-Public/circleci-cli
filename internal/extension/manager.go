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

package extension

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"runtime"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

type Manager struct {
	client *httpcl.Client
}

type Config struct {
	Version string
	Agent   string
	BaseURL string
}

func NewManager(cfg Config) *Manager {
	clientCfg := httpcl.Config{
		UserAgent: httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, cfg.Version, cfg.Agent),
		BaseURL:   cfg.BaseURL,
	}

	return &Manager{
		client: httpcl.New(clientCfg),
	}
}

type Extension struct {
	BinaryName string
	Version    string
	Arch       string
	Sys        string
	Sha256     string
	URL        string
}

type Binary struct {
	Path   string `json:"name"`
	Arch   string `json:"arch"`
	Sys    string `json:"sys"`
	Sha256 string `json:"sha256"`
}

type LatestData struct {
	Version  string   `json:"version"`
	Binaries []Binary `json:"binaries"`
}

// validExtensionName matches names that are safe to use as a URL path segment
// and as a filesystem directory name.
var validExtensionName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

func (m *Manager) Get(ctx context.Context, binaryName string) (Extension, error) {
	if err := validateName(binaryName); err != nil {
		return Extension{}, err
	}

	var resp LatestData
	route := fmt.Sprintf("/circleci-cli-plugins/%s/latest/release.json", binaryName)
	req := httpcl.NewRequest(
		http.MethodGet,
		route,
		httpcl.JSONDecoder(&resp),
	)
	_, err := m.client.Call(ctx, req)
	if err != nil {
		httpErr, ok := errors.AsType[*httpcl.HTTPError](err)
		if ok && httpErr.StatusCode == http.StatusNotFound {
			return Extension{}, &ErrExtensionNotFound{Name: binaryName}
		}
		return Extension{}, err
	}

	var b *Binary
	for _, binary := range resp.Binaries {
		if binary.Arch == runtime.GOARCH && binary.Sys == runtime.GOOS {
			b = &binary
			break
		}
	}

	if b == nil {
		return Extension{}, &ErrNoBinaryForPlatform{Name: binaryName, OS: runtime.GOOS, Arch: runtime.GOARCH}
	}

	return Extension{
		BinaryName: binaryName,
		Version:    resp.Version,
		Arch:       b.Arch,
		Sys:        b.Sys,
		Sha256:     b.Sha256,
		URL:        path.Join("/circleci-cli-plugins", binaryName, resp.Version, b.Path),
	}, nil
}

func (m *Manager) Download(ctx context.Context, ext Extension) (io.ReadCloser, error) {
	buf := bytes.NewBuffer(nil)

	hasher := sha256.New()
	req := httpcl.NewRequest(
		http.MethodGet,
		ext.URL,
		// Explicit Accept-Encoding suppresses Go's transparent gzip decompression,
		// ensuring the hasher and file receive the raw bytes matching the release SHA256.
		httpcl.Header("Accept-Encoding", "identity"),
		httpcl.Header("Accept", "application/gzip"),
		httpcl.CopyDecoder(io.MultiWriter(buf, hasher)),
	)
	_, err := m.client.Call(ctx, req)
	if err != nil {
		if httpErr, ok := errors.AsType[*httpcl.HTTPError](err); ok {
			return nil, &ErrDownloadFailed{StatusCode: httpErr.StatusCode}
		}
		return nil, err
	}

	got := hex.EncodeToString(hasher.Sum(nil))
	if got != ext.Sha256 {
		return nil, &ErrChecksumMismatch{Expected: ext.Sha256, Got: got}
	}

	return gzip.NewReader(buf)
}

func validateName(name string) error {
	if !validExtensionName.MatchString(name) {
		return &ErrInvalidName{Name: name}
	}
	return nil
}
