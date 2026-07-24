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

package fakes

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/internal/extension"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder/chirecorder"
)

type ExtensionRegistry struct {
	RequestRecorder *httprecorder.RequestRecorder

	mu         sync.RWMutex
	server     *httptest.Server
	extensions map[string]extensionEntry
}

func NewExtensionRegistry(t *testing.T) *ExtensionRegistry {
	t.Helper()

	registry := &ExtensionRegistry{
		RequestRecorder: httprecorder.New(),
		extensions:      make(map[string]extensionEntry),
	}

	r := newRouter()
	r.Use(chirecorder.Middleware(registry.RequestRecorder))
	r.Get("/circleci-cli-plugins/{extName}/latest/release.json", registry.handleGetLatest)
	r.Get("/circleci-cli-plugins/{extName}/{version}/{goos}/{goarch}/{binaryName}", registry.handleInstall)

	registry.server = httptest.NewServer(r)
	t.Cleanup(registry.server.Close)

	return registry
}

func (e *ExtensionRegistry) URL() string {
	return e.server.URL
}

type ExtensionMeta struct {
	Version string
	Sha256  string
	Arch    string
	Sys     string
}

type extensionEntry struct {
	gzipBinary []byte
	manifest   extension.Manifest
	meta       ExtensionMeta
}

func (e *ExtensionRegistry) WithExtension(t *testing.T, ext extension.Manifest, meta ExtensionMeta) *ExtensionRegistry {
	t.Helper()
	hasher := sha256.New()

	var buf bytes.Buffer
	func() {
		zw := gzip.NewWriter(&buf)
		defer func() { _ = zw.Close() }()

		b, err := os.ReadFile(ext.Path)
		assert.NilError(t, err)

		_, err = zw.Write(b)
		assert.NilError(t, err)
	}()

	all := buf.Bytes()

	_, err := hasher.Write(all)
	assert.NilError(t, err)

	ext.Sha256 = fmt.Sprintf("%x", hasher.Sum(nil))

	e.mu.Lock()
	defer e.mu.Unlock()

	version := ext.Version
	if meta.Version != "" {
		version = meta.Version
	}

	sha := ext.Sha256
	if meta.Sha256 != "" {
		sha = meta.Sha256
	}

	e.extensions[ext.BinaryName] = extensionEntry{
		gzipBinary: all,
		manifest:   ext,
		meta: ExtensionMeta{
			Version: version,
			Sha256:  sha,
			Arch:    meta.Arch,
			Sys:     meta.Sys,
		},
	}

	return e
}

func (e *ExtensionRegistry) Manifest(binaryName string) extension.Manifest {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.extensions[binaryName].manifest
}

type extensionBinary struct {
	Path   string `json:"name"`
	Arch   string `json:"arch"`
	Sys    string `json:"sys"`
	Sha256 string `json:"sha256"`
}

type extensionLatestData struct {
	Version  string               `json:"version"`
	Binaries []extensionBinary    `json:"binaries"`
	Ref      *extension.Reference `json:"reference,omitempty"`
}

func (e *ExtensionRegistry) handleGetLatest(w http.ResponseWriter, r *http.Request) {
	extName := chi.URLParam(r, "extName")

	e.mu.RLock()
	defer e.mu.RUnlock()
	ext, ok := e.extensions[extName]

	if !ok {
		http.Error(w, "extension not found", http.StatusNotFound)
		return
	}

	path := fmt.Sprintf("%s/%s/%s", ext.meta.Sys, ext.meta.Arch, extName)
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	path += ".gz"

	d := extensionLatestData{
		Version: ext.meta.Version,
		Binaries: []extensionBinary{
			{
				Path:   path,
				Arch:   ext.meta.Arch,
				Sys:    ext.meta.Sys,
				Sha256: ext.meta.Sha256,
			},
		},
		Ref: ext.manifest.Ref,
	}

	render.JSON(w, r, d)
}

func (e *ExtensionRegistry) handleInstall(w http.ResponseWriter, r *http.Request) {
	version := chi.URLParam(r, "version")
	goos := chi.URLParam(r, "goos")
	goarch := chi.URLParam(r, "goarch")
	extName := chi.URLParam(r, "extName")

	e.mu.RLock()
	defer e.mu.RUnlock()
	ext, ok := e.extensions[extName]

	if !ok {
		http.Error(w, "extension not found", http.StatusNotFound)
		return
	}

	if version != ext.manifest.Version {
		http.Error(w, "unknown version", http.StatusBadRequest)
		return
	}

	if goos != runtime.GOOS || goarch != runtime.GOARCH {
		http.Error(w, "unknown binary", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/gzip")
	_, _ = w.Write(ext.gzipBinary)
}
