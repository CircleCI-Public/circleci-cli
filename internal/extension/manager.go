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
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// maxExtensionSize is the maximum size of a binary to prevent
// potential DoS vulnerability (G110). Set to 512MB.
const maxExtensionSize int64 = 512 * 1024 * 1024

type Manager struct {
	client        *httpcl.Client
	extensionsDir string
}

type Config struct {
	Version       string
	Agent         string
	BaseURL       string
	ExtensionsDir string
}

func NewManager(cfg Config) *Manager {
	clientCfg := httpcl.Config{
		UserAgent: httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, cfg.Version, cfg.Agent),
		BaseURL:   cfg.BaseURL,
	}

	return &Manager{
		client:        httpcl.New(clientCfg),
		extensionsDir: cfg.ExtensionsDir,
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

func (m *Manager) Install(ctx context.Context, ext Extension) error {
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
			return &ErrDownloadFailed{StatusCode: httpErr.StatusCode}
		}
		return err
	}

	got := hex.EncodeToString(hasher.Sum(nil))
	if got != ext.Sha256 {
		return &ErrChecksumMismatch{Expected: ext.Sha256, Got: got}
	}

	// Ensure the extensions directory exists.
	if err := os.MkdirAll(m.extensionsDir, 0750); err != nil {
		return err
	}

	fsRoot, err := os.OpenRoot(m.extensionsDir)
	if err != nil {
		return err
	}

	defer func() { _ = fsRoot.Close() }()

	target := filepath.Join(ext.BinaryName, ext.BinaryName)
	if runtime.GOOS == "windows" {
		target += ".exe"
	}

	if err := fsRoot.MkdirAll(filepath.Dir(target), 0750); err != nil {
		return err
	}

	out, err := fsRoot.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	zr, err := gzip.NewReader(buf)
	if err != nil {
		_ = os.Remove(target)
		return err
	}
	defer func() { _ = zr.Close() }()

	n, err := io.CopyN(out, zr, maxExtensionSize+1)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
	}

	if n > maxExtensionSize {
		return &ErrExtensionTooLarge{Name: ext.BinaryName, MaxSize: maxExtensionSize}
	}

	return m.writeManifest(fsRoot, Manifest{
		Name:       strings.TrimPrefix(ext.BinaryName, "circleci-"),
		BinaryName: ext.BinaryName,
		Version:    ext.Version,
		Sha256:     ext.Sha256,
		URL:        ext.URL,
		Path:       filepath.Join(fsRoot.Name(), target),
	})
}

// Remove removes the given extension.
// The full contents of the extension directory are removed.
// If the extension manifest points to a binary outside the
// directory, the binary is not removed.
// Returns ErrExtensionNotInstalled if the extension is not installed.
func (m *Manager) Remove(_ context.Context, binaryName string) error {
	if err := validateName(binaryName); err != nil {
		return err
	}

	fsRoot, err := os.OpenRoot(m.extensionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &ErrExtensionNotInstalled{Name: binaryName}
		}

		return err
	}
	defer func() { _ = fsRoot.Close() }()

	if _, err := fsRoot.Stat(binaryName); err != nil {
		if os.IsNotExist(err) {
			return &ErrExtensionNotInstalled{Name: binaryName}
		}

		return err
	}

	return fsRoot.RemoveAll(binaryName)
}

func validateName(name string) error {
	if !validExtensionName.MatchString(name) {
		return &ErrInvalidName{Name: name}
	}
	return nil
}

func (m *Manager) writeManifest(fs *os.Root, manifest Manifest) error {
	target := filepath.Join(manifest.BinaryName, "manifest.yml")

	err := fs.MkdirAll(filepath.Dir(target), 0750)
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}

	return fs.WriteFile(target, b, 0600)
}
