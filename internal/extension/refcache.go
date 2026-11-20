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
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/config"
)

// CachedReference is the registry's reference for an extension that is not
// installed, so its commands can be documented before the binary is downloaded.
type CachedReference struct {
	Version   string    `yaml:"version"`
	FetchedAt time.Time `yaml:"fetched_at"`
	// FailedAt is when a fetch last failed, zero once one succeeds. It holds
	// off retrying, so an unreachable registry does not delay every start.
	FailedAt time.Time  `yaml:"failed_at,omitempty"`
	Ref      *Reference `yaml:"reference,omitempty"`
}

// ReferenceCache stores one CachedReference per extension, in a directory kept
// apart from the extensions directory so a cached reference is never mistaken
// for an installed extension.
type ReferenceCache struct {
	dir string
}

func NewReferenceCache(dir string) *ReferenceCache {
	return &ReferenceCache{dir: dir}
}

// Get returns the cached reference for binaryName, or the zero value when none
// has been cached.
func (c *ReferenceCache) Get(binaryName string) (CachedReference, error) {
	if err := validateName(binaryName); err != nil {
		return CachedReference{}, err
	}

	b, err := os.ReadFile(c.path(binaryName)) //#nosec:G304 // binaryName is validated above
	if errors.Is(err, fs.ErrNotExist) {
		return CachedReference{}, nil
	}
	if err != nil {
		return CachedReference{}, err
	}

	var cached CachedReference
	if err := yaml.Unmarshal(b, &cached); err != nil {
		return CachedReference{}, err
	}
	return cached, nil
}

// Put replaces the cached reference for binaryName. The file is written beside
// its destination and renamed into place, so a concurrent reader sees either
// the old reference or the new one, never a partial write.
func (c *ReferenceCache) Put(binaryName string, cached CachedReference) error {
	if err := validateName(binaryName); err != nil {
		return err
	}

	b, err := yaml.Marshal(cached)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(c.dir, binaryName+".*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), c.path(binaryName))
}

func (c *ReferenceCache) path(binaryName string) string {
	return filepath.Join(c.dir, binaryName+".yml")
}

// LookupReference returns the reference documenting binaryName.
func LookupReference(binaryName string) (*Reference, error) {
	extsDir, err := config.ExtensionsDir()
	if err != nil {
		return nil, err
	}

	manifest, err := NewStore(extsDir).Get(binaryName)
	switch {
	case err == nil:
		return manifest.Ref, nil
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}

	refsDir, err := config.ExtensionReferencesDir()
	if err != nil {
		return nil, err
	}

	cached, err := NewReferenceCache(refsDir).Get(binaryName)
	if err != nil {
		return nil, err
	}
	return cached.Ref, nil
}
