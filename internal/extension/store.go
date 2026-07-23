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
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// maxExtensionSize is the maximum size of a binary to prevent
// potential DoS vulnerability (G110). Set to 512MB.
const maxExtensionSize int64 = 512 * 1024 * 1024

type Store struct {
	extensionsDir string
}

func NewStore(extensionsDir string) *Store {
	return &Store{
		extensionsDir: extensionsDir,
	}
}

// FindAll scans path for extension manifest.yml files. It loads extension data
// from each manifest.yml and returns all loaded extension manifests.
func (m *Store) FindAll() ([]Manifest, error) {
	var exts []Manifest

	err := filepath.Walk(m.extensionsDir, func(path string, info fs.FileInfo, err error) error {
		// The extensions directory may not have been created if no extensions
		// are installed
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		if err != nil {
			return err
		}

		// continue if path is a directory
		if info.IsDir() {
			return nil
		}

		// if the basename is `manifest.yml` load the manifest and add to results
		// Then return filepath.SkipDir
		if filepath.Base(path) == "manifest.yml" {
			// path is passed from filepath.Walk
			// nolint:gosec
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()

			var ext Manifest

			decoder := yaml.NewDecoder(f)
			err = decoder.Decode(&ext)
			if err != nil {
				return &ErrCorruptExtensionManifest{
					ManifestPath: path,
					Err:          err,
				}
			}

			exts = append(exts, ext)

			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return exts, nil
}

func (m *Store) Get(binaryName string) (Manifest, error) {
	if err := validateName(binaryName); err != nil {
		return Manifest{}, err
	}

	target := filepath.Join(binaryName, "manifest.yml")

	fsRoot, err := os.OpenRoot(m.extensionsDir)
	if err != nil {
		return Manifest{}, err
	}

	defer func() { _ = fsRoot.Close() }()

	b, err := fsRoot.ReadFile(target)
	if err != nil {
		return Manifest{}, err
	}

	var manifest Manifest
	if err := yaml.Unmarshal(b, &manifest); err != nil {
		return Manifest{}, err
	}

	return manifest, nil
}

func (m *Store) Write(ext Extension, binary io.Reader) (_ Manifest, err error) {
	// Ensure the extensions directory exists.
	if err := os.MkdirAll(m.extensionsDir, 0750); err != nil {
		return Manifest{}, err
	}

	fsRoot, err := os.OpenRoot(m.extensionsDir)
	if err != nil {
		return Manifest{}, err
	}

	defer func() { _ = fsRoot.Close() }()

	binaryTarget := filepath.Join(ext.BinaryName, ext.BinaryName)
	if runtime.GOOS == "windows" {
		binaryTarget += ".exe"
	}

	if err := fsRoot.MkdirAll(filepath.Dir(binaryTarget), 0750); err != nil {
		return Manifest{}, err
	}

	out, err := fsRoot.OpenFile(binaryTarget, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		return Manifest{}, err
	}
	defer func() {
		// clean up created file on error.
		if err != nil {
			_ = os.Remove(binaryTarget)
		}

		_ = out.Close()
	}()

	n, err := io.CopyN(out, binary, maxExtensionSize+1)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return Manifest{}, err
		}
	}

	if n > maxExtensionSize {
		return Manifest{}, &ErrExtensionTooLarge{Name: ext.BinaryName, MaxSize: maxExtensionSize}
	}

	manifest := Manifest{
		Name:       strings.TrimPrefix(ext.BinaryName, "circleci-"),
		BinaryName: ext.BinaryName,
		Version:    ext.Version,
		Sha256:     ext.Sha256,
		URL:        ext.URL,
		Path:       filepath.Join(fsRoot.Name(), binaryTarget),
	}

	b, err := yaml.Marshal(manifest)
	if err != nil {
		return Manifest{}, err
	}

	manifestTarget := filepath.Join(manifest.BinaryName, "manifest.yml")
	if err := fsRoot.WriteFile(manifestTarget, b, 0600); err != nil {
		return Manifest{}, err
	}

	return manifest, nil
}

// Remove removes the given extension.
// The full contents of the extension directory are removed.
// If the extension manifest points to a binary outside the
// directory, the binary is not removed.
// Returns ErrExtensionNotInstalled if the extension is not installed.
func (m *Store) Remove(binaryName string) error {
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
