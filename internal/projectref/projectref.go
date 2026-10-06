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
	"regexp"
	"strings"

	"github.com/google/uuid"
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
		orgID, oErr := canonicaliseUUIDString(i.Organization.ID)
		projectID, pErr := canonicaliseUUIDString(i.Project.ID)
		if oErr == nil && pErr == nil {
			return "circleci/" + orgID + "/" + projectID
		}
	}
	return i.Project.Slug
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
	info, err = normalise(info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// Write serialises info to .circleci/info.yml inside workDir, creating the
// .circleci directory if needed.
func Write(workDir string, info *Info) error {
	if info == nil {
		return errors.New("nil is not a valid repository info struct")
	}
	i, err := normalise(*info)
	if err != nil {
		return err
	}
	target := Path(workDir)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { //#nosec:G301 // .circleci/ is a repo-shared directory, world-readable like the surrounding workspace
		return fmt.Errorf("creating .circleci directory: %w", err)
	}
	data, err := yaml.Marshal(i)
	if err != nil {
		return fmt.Errorf("serialising info: %w", err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil { //#nosec:G306 // info.yml is intended to be committed alongside .circleci/config.yml
		return fmt.Errorf("writing %s: %w", FilePath, err)
	}
	return nil
}

func normalise(info Info) (Info, error) {
	var orgID string
	var projectID string
	var err error

	if info.Organization.ID != "" {
		orgID, err = canonicaliseUUIDString(info.Organization.ID)
		if err != nil {
			return Info{}, fmt.Errorf("%s is not a valid org ID: %w", info.Organization.ID, err)
		}
	}
	if info.Organization.Name != "" && info.Organization.Name != strings.TrimSpace(info.Organization.Name) {
		return Info{}, fmt.Errorf("%q is not a valid org name", info.Organization.Name)
	}
	if info.Project.ID != "" {
		projectID, err = canonicaliseUUIDString(info.Project.ID)
		if err != nil {
			return Info{}, fmt.Errorf("%s is not a valid project ID: %w", info.Project.ID, err)
		}
	}
	if info.Project.Name != "" && info.Project.Name != strings.TrimSpace(info.Project.Name) {
		return Info{}, fmt.Errorf("%q is not a valid project name", info.Project.Name)
	}
	slug, err := normaliseSlug(info.Project.Slug)
	if err != nil {
		return Info{}, err
	}

	return Info{
		Organization: Organization{
			ID:   orgID,
			Name: info.Organization.Name,
		},
		Project: Project{
			ID:   projectID,
			Name: info.Project.Name,
			Slug: slug,
		},
	}, nil
}

func normaliseSlug(slug string) (string, error) {
	if slug == "" {
		return "", fmt.Errorf("%s is missing required 'project.slug' field", FilePath)
	}
	slugParts := strings.SplitN(slug, "/", 3)
	if len(slugParts) != 3 {
		return "", fmt.Errorf("%q is not a valid slug", slug)
	}
	switch slugParts[0] {
	case "circleci":
		if slugParts[1] == "" || !validCircleCISlugComponent(slugParts[1]) {
			return "", fmt.Errorf("%q is not a valid slug", slug)
		}
		if slugParts[2] == "" || !validCircleCISlugComponent(slugParts[2]) {
			return "", fmt.Errorf("%q is not a valid slug", slug)
		}
		canonicalOrgID, oErr := canonicaliseUUIDString(slugParts[1])
		canonicalProjectID, pErr := canonicaliseUUIDString(slugParts[2])
		if (oErr == nil) != (pErr == nil) {
			return "", errors.New("mixing ID encodings in a slug is not permitted")
		}
		return fmt.Sprintf("circleci/%s/%s", canonicalOrgID, canonicalProjectID), nil
	case "bb", "bitbucket":
		if slugParts[1] == "" || !validVCSName(slugParts[1]) {
			return "", fmt.Errorf("%q is not a valid slug", slug)
		}
		if slugParts[2] == "" || !validVCSName(slugParts[2]) {
			return "", fmt.Errorf("%q is not a valid slug", slug)
		}
		return fmt.Sprintf("bb/%s/%s", slugParts[1], slugParts[2]), nil
	case "gh", "github":
		if slugParts[1] == "" || !validVCSName(slugParts[1]) {
			return "", fmt.Errorf("%q is not a valid slug", slug)
		}
		if slugParts[2] == "" || !validVCSName(slugParts[2]) {
			return "", fmt.Errorf("%q is not a valid slug", slug)
		}
		return fmt.Sprintf("gh/%s/%s", slugParts[1], slugParts[2]), nil
	default:
		return "", fmt.Errorf("slugs may not start with %s/", slugParts[0])
	}
}

func canonicaliseUUIDString(s string) (string, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return s, err
	}
	return id.String(), nil
}

// This is intentionally looser than the actual enforcement for CircleCI org
// and project names. This is just rough validation to prevent obviously-wrong
// data from being persisted.
var validCircleCISlugComponentRegex = regexp.MustCompile(`^[A-Za-z0-9-]{14,36}$`)

func validCircleCISlugComponent(s string) bool {
	return validCircleCISlugComponentRegex.MatchString(s) &&
		s != "." &&
		s != ".." &&
		!strings.ContainsRune(s, '/')
}

// This is intentionally looser than the actual enforcement by Bitbucket and
// GitHub. We're aiming here for rough validation to allow for fast, local and
// potentially offline error handling and to avoid persisting obviously-wrong
// data. This also has to match both UUIDs and base58-encoded UUIDs.
var validVCSNameRegex = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func validVCSName(s string) bool {
	return validVCSNameRegex.MatchString(s) &&
		s != "." &&
		s != ".." &&
		!strings.ContainsRune(s, '/')
}
