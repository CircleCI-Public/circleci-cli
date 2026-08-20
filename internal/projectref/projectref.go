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

// Package projectref reads and writes .circleci/info.yml — the per-checkout
// file that records which CircleCI project this directory is bound to.
//
// It is consulted by slug-resolution code so that repository renames and
// non-VCS-derived slugs (e.g. standalone projects) work without forcing
// every caller to pass --project explicitly.
package projectref

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FilePath is the path to the info file relative to the checkout root.
const FilePath = ".circleci/info.yml"

// Info is the on-disk record written by `circleci project link`.
//
// Schema:
//
//	organization:
//	  id: <uuid>          # required
//	  name: <string>      # optional, populated from API when available
//	project:
//	  id: <uuid>          # required
//	  slug: <string>      # required — VCS-style or canonical "circleci/<orgID>/<projectID>"
//	  name: <string>      # optional, populated from API when available
//
// Project.Slug is always populated. Organization.ID and Project.ID are also
// populated when the slug was verified against the CircleCI API at link time;
// they let callers reference the project by its stable UUID even after a repo
// rename.
type Info struct {
	Organization Organization `yaml:"organization"`
	Project      Project      `yaml:"project"`
}

// Organization is the CircleCI organization that owns the project.
type Organization struct {
	ID   string `yaml:"id,omitempty"`
	Name string `yaml:"name,omitempty"`
}

// Project identifies a CircleCI project.
type Project struct {
	ID   string `yaml:"id,omitempty"`
	Slug string `yaml:"slug"`
	Name string `yaml:"name,omitempty"`
}

// EffectiveSlug returns the slug to use when calling the CircleCI API.
//
// For a CircleCI-native project — one whose stored slug is already a "circleci/"
// slug — the canonical "circleci/<orgID>/<projectID>" form is returned when both
// IDs are known, so the lookup is stable across renames. Anything else returns
// the stored Project.Slug as-is.
//
// The ID form addresses CircleCI-native projects only. A classic VCS project must
// be addressed by its own "<vcs>/<org>/<repo>" slug: the API answers 404 for the
// ID form even though both IDs are valid and `circleci project link` records them
// for every project type.
func (i *Info) EffectiveSlug() string {
	if i == nil {
		return ""
	}
	if strings.HasPrefix(i.Project.Slug, "circleci/") && i.Project.ID != "" && i.Organization.ID != "" {
		return SlugFor(i.Organization.ID, i.Project.ID)
	}
	return i.Project.Slug
}

// SlugFor builds the slug of a CircleCI-native project from its organization and
// project IDs. The API accepts UUIDs in place of the compact IDs a slug usually
// carries, which is what lets a caller holding only IDs address the project.
func SlugFor(orgID, projectID string) string {
	return "circleci/" + orgID + "/" + projectID
}

// ErrNotFound is returned by Read when no info file exists.
var ErrNotFound = errors.New("projectref: .circleci/info.yml not found")

// Path returns the absolute path to the info file inside workDir.
func Path(workDir string) string {
	return filepath.Join(workDir, FilePath)
}

// Read parses .circleci/info.yml from workDir. Returns ErrNotFound if the
// file does not exist; other errors signal a malformed file or I/O failure.
func Read(workDir string) (*Info, error) {
	target := Path(workDir)
	data, err := os.ReadFile(target) //#nosec:G304 // workDir is the caller's chosen working directory; FilePath is constant
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", FilePath, err)
	}
	var info Info
	if err := yaml.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", FilePath, err)
	}
	if info.Project.Slug == "" {
		return nil, fmt.Errorf("%s is missing required 'project.slug' field", FilePath)
	}
	return &info, nil
}

// Write serialises info to .circleci/info.yml inside workDir, creating the
// .circleci directory if needed.
func Write(workDir string, info *Info) error {
	target := Path(workDir)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { //#nosec:G301 // .circleci/ is a repo-shared directory, world-readable like the surrounding workspace
		return fmt.Errorf("creating .circleci directory: %w", err)
	}
	data, err := yaml.Marshal(info)
	if err != nil {
		return fmt.Errorf("serialising info: %w", err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil { //#nosec:G306 // info.yml is intended to be committed alongside .circleci/config.yml
		return fmt.Errorf("writing %s: %w", FilePath, err)
	}
	return nil
}
