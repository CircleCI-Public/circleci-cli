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

package apiclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// --- Sentinel errors ---

// ErrOrbNotFound is returned when an orb package does not exist.
var ErrOrbNotFound = errors.New("orb not found")

// ErrOrbVersionNotFound is returned when an orb version does not exist.
var ErrOrbVersionNotFound = errors.New("orb version not found")

// ErrOrbCategoryNotFound is returned when an orb category does not exist.
var ErrOrbCategoryNotFound = errors.New("orb category not found")

// --- Wire types ---

type orbPackageAttributes struct {
	Name                   string `json:"name"`
	IsPrivate              bool   `json:"is_private"`
	IsListed               bool   `json:"is_listed"`
	CreatedAt              string `json:"created_at"`
	Last30DaysBuildCount   int64  `json:"last_30_days_build_count"`
	Last30DaysProjectCount int64  `json:"last_30_days_project_count"`
	Last30DaysOrgCount     int64  `json:"last_30_days_org_count"`
}

type namespaceRef struct {
	ID         string `json:"id"`
	Attributes struct {
		Name string `json:"name"`
	} `json:"attributes"`
}

type orbVersionRef struct {
	ID         string `json:"id"`
	Attributes struct {
		Version   string `json:"version"`
		CreatedAt string `json:"created_at"`
	} `json:"attributes"`
}

type orbCategoryRef struct {
	ID         string `json:"id"`
	Attributes struct {
		Name string `json:"name"`
	} `json:"attributes"`
}

type orbPackageReferences struct {
	Namespace  namespaceRef     `json:"namespace"`
	Versions   []orbVersionRef  `json:"orb_versions"`
	Categories []orbCategoryRef `json:"orb_categories"`
}

// orbPackageWire is the detail response shape (GET /orb/packages/{id}).
type orbPackageWire struct {
	ID         string               `json:"id"`
	Attributes orbPackageAttributes `json:"attributes"`
	References orbPackageReferences `json:"references"`
}

// orbPackageListWire is the shape returned by the list endpoint
// (GET /orb/packages). The namespace reference contains only an id (no name);
// usage stats are not included.
type orbPackageListWire struct {
	ID         string `json:"id"`
	Attributes struct {
		Name      string `json:"name"`
		IsPrivate bool   `json:"is_private"`
		IsListed  bool   `json:"is_listed"`
	} `json:"attributes"`
	References struct {
		Namespace struct {
			ID string `json:"id"`
		} `json:"namespace"`
		Versions []struct {
			ID         string `json:"id"`
			Attributes struct {
				Version   string `json:"version"`
				CreatedAt string `json:"created_at"`
			} `json:"attributes"`
		} `json:"orb_versions"`
		Categories []orbCategoryRef `json:"orb_categories"`
	} `json:"references"`
}

func (w *orbPackageListWire) toOrbPackage() *OrbPackage {
	pkg := &OrbPackage{
		ID:          w.ID,
		Name:        w.Attributes.Name,
		NamespaceID: w.References.Namespace.ID,
		IsPrivate:   w.Attributes.IsPrivate,
		IsListed:    w.Attributes.IsListed,
	}
	if len(w.References.Versions) > 0 {
		pkg.LatestVersion = w.References.Versions[0].Attributes.Version
		pkg.LatestVersionAt = w.References.Versions[0].Attributes.CreatedAt
	}
	for _, c := range w.References.Categories {
		pkg.Categories = append(pkg.Categories, OrbCategory{ID: c.ID, Name: c.Attributes.Name})
	}
	return pkg
}

type orbVersionAttributes struct {
	Version   string `json:"version"`
	CreatedAt string `json:"created_at"`
}

type orbVersionOrbRef struct {
	ID         string `json:"id"`
	Attributes struct {
		Name string `json:"name"`
	} `json:"attributes"`
}

type orbVersionReferences struct {
	Package orbVersionOrbRef `json:"orb_package"`
}

type orbVersionWire struct {
	ID         string               `json:"id"`
	Attributes orbVersionAttributes `json:"attributes"`
	References orbVersionReferences `json:"references"`
}

type orbCategoryWire struct {
	ID         string `json:"id"`
	Attributes struct {
		Name string `json:"name"`
	} `json:"attributes"`
}

type createOrbPackageWire struct {
	Attributes struct {
		Name      string `json:"name"`
		IsPrivate bool   `json:"is_private"`
	} `json:"attributes"`
	References struct {
		Namespace struct {
			ID string `json:"id"`
		} `json:"namespace"`
	} `json:"references"`
}

type publishOrbVersionWire struct {
	Attributes PublishOrbVersionRequest `json:"attributes"`
}

type orbValidateWire struct {
	ID         string `json:"id"`
	Attributes struct {
		Valid      bool     `json:"is_valid"`
		OutputYAML string   `json:"output_yaml"`
		Errors     []string `json:"errors"`
	} `json:"attributes"`
}

// --- Domain types ---

// OrbPackage is a domain-level orb package.
type OrbPackage struct {
	ID                     string
	Name                   string
	Namespace              string
	NamespaceID            string
	IsPrivate              bool
	IsListed               bool
	CreatedAt              string
	LatestVersion          string
	LatestVersionAt        string
	Last30DaysBuildCount   int64
	Last30DaysProjectCount int64
	Last30DaysOrgCount     int64
	Categories             []OrbCategory
}

// OrbVersion is a domain-level orb version.
type OrbVersion struct {
	ID        string
	OrbID     string
	OrbName   string
	Version   string
	CreatedAt string
}

// OrbCategory is a domain-level orb category.
type OrbCategory struct {
	ID   string
	Name string
}

// OrbValidation holds the result of a validate or process API call.
type OrbValidation struct {
	Valid      bool
	OutputYAML string
	Errors     []string
}

// --- Converters ---

func (w *orbPackageWire) toOrbPackage() *OrbPackage {
	pkg := &OrbPackage{
		ID:                     w.ID,
		Name:                   w.Attributes.Name,
		Namespace:              w.References.Namespace.Attributes.Name,
		NamespaceID:            w.References.Namespace.ID,
		IsPrivate:              w.Attributes.IsPrivate,
		IsListed:               w.Attributes.IsListed,
		CreatedAt:              w.Attributes.CreatedAt,
		Last30DaysBuildCount:   w.Attributes.Last30DaysBuildCount,
		Last30DaysProjectCount: w.Attributes.Last30DaysProjectCount,
		Last30DaysOrgCount:     w.Attributes.Last30DaysOrgCount,
	}
	if len(w.References.Versions) > 0 {
		pkg.LatestVersion = w.References.Versions[0].Attributes.Version
		pkg.LatestVersionAt = w.References.Versions[0].Attributes.CreatedAt
	}
	for _, c := range w.References.Categories {
		pkg.Categories = append(pkg.Categories, OrbCategory{ID: c.ID, Name: c.Attributes.Name})
	}
	return pkg
}

func (w *orbVersionWire) toOrbVersion() *OrbVersion {
	return &OrbVersion{
		ID:        w.ID,
		OrbID:     w.References.Package.ID,
		OrbName:   w.References.Package.Attributes.Name,
		Version:   w.Attributes.Version,
		CreatedAt: w.Attributes.CreatedAt,
	}
}

// --- API methods ---

// ListOrbPackages lists all orb packages, depaginating automatically.
// namespaceID filters by namespace (empty = global). When uncertified is
// false, only certified orbs are returned.
func (c *Client) ListOrbPackages(ctx context.Context, namespaceID string, uncertified, private bool) ([]*OrbPackage, error) {
	certified := ""
	if !uncertified {
		certified = "true"
	}
	visibility := ""
	if private {
		visibility = "private"
	}

	var result []*OrbPackage
	cursor := ""
	for {
		var page v3List[orbPackageListWire]
		if err := c.getV3(ctx, "/orb/packages", &page,
			filterParam("namespace_id", namespaceID),
			filterParam("certified", certified),
			filterParam("visibility", visibility),
			pageCursor(cursor),
		); err != nil {
			return nil, err
		}
		for i := range page.Data {
			result = append(result, page.Data[i].toOrbPackage())
		}
		if page.Page.Next == nil || *page.Page.Next == "" {
			break
		}
		cursor = *page.Page.Next
	}
	return result, nil
}

// GetOrbPackageByID gets a single orb package by UUID.
func (c *Client) GetOrbPackageByID(ctx context.Context, id uuid.UUID) (*OrbPackage, error) {
	var env v3Entity[orbPackageWire]
	err := c.getV3(ctx, "/orb/packages/%s", &env,
		routeParams(id),
	)
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return nil, fmt.Errorf("%w: %q", ErrOrbNotFound, id)
	}
	if err != nil {
		return nil, err
	}
	return env.Data.toOrbPackage(), nil
}

// GetOrbPackageByName resolves an orb by its full name (e.g. "ns/name").
// It first resolves the namespace, then filters orbs by name.
func (c *Client) GetOrbPackageByName(ctx context.Context, fullName string) (*OrbPackage, error) {
	var env v3List[orbPackageWire]
	err := c.getV3(ctx, "/orb/packages", &env,
		filterParam("name", fullName),
	)
	if err != nil {
		return nil, err
	}
	if len(env.Data) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrOrbNotFound, fullName)
	}
	return env.Data[0].toOrbPackage(), nil
}

// CreateOrbPackageRequest is the body for creating an orb.
type CreateOrbPackageRequest struct {
	Name        string `json:"name"`
	NamespaceID string `json:"namespace_id"`
	IsPrivate   bool   `json:"is_private"`
}

// CreateOrbPackage creates a new orb package.
func (c *Client) CreateOrbPackage(ctx context.Context, req CreateOrbPackageRequest) (*OrbPackage, error) {
	var wire v3Entity[createOrbPackageWire]
	wire.Data.Attributes.Name = req.Name
	wire.Data.Attributes.IsPrivate = req.IsPrivate
	wire.Data.References.Namespace.ID = req.NamespaceID

	var env v3Entity[orbPackageWire]
	err := c.postV3(ctx, "/orb/packages", wire, &env)
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return nil, fmt.Errorf("%w: namespace %q", ErrOrbNotFound, req.NamespaceID)
	}
	if err != nil {
		return nil, err
	}
	return env.Data.toOrbPackage(), nil
}

// ValidateOrbYAML validates orb YAML. orgID is optional.
func (c *Client) ValidateOrbYAML(ctx context.Context, yaml, orgID string) (*OrbValidation, error) {
	var env v3Entity[orbValidateWire]
	err := c.postV3(ctx, "/orb/packages/validate", orbYAMLBody{YAML: yaml, OrgID: orgID}, &env)
	if err != nil {
		return nil, err
	}
	return &OrbValidation{
		Valid:      env.Data.Attributes.Valid,
		OutputYAML: env.Data.Attributes.OutputYAML,
		Errors:     env.Data.Attributes.Errors,
	}, nil
}

// SetOrbListed sets the listed status of an orb package.
func (c *Client) SetOrbListed(ctx context.Context, orbID string, listed bool) error {
	var env v3Entity[orbPackageWire]
	err := c.postV3(ctx, "/orb/packages/%s/set-listed", orbSetListedBody{Listed: listed}, &env,
		routeParams(orbID),
	)
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return fmt.Errorf("%w: %q", ErrOrbNotFound, orbID)
	}
	return err
}

// AddOrbToCategory adds an orb to a category.
func (c *Client) AddOrbToCategory(ctx context.Context, orbID, categoryID string) error {
	var env v3Entity[orbPackageWire]
	err := c.postV3(ctx, "/orb/packages/%s/add-category", orbCategoryBody{CategoryID: categoryID}, &env,
		routeParams(orbID),
	)
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return fmt.Errorf("%w: %q", ErrOrbNotFound, orbID)
	}
	return err
}

// RemoveOrbFromCategory removes an orb from a category.
func (c *Client) RemoveOrbFromCategory(ctx context.Context, orbID, categoryID string) error {
	var env v3Entity[orbPackageWire]
	err := c.postV3(ctx, "/orb/packages/%s/remove-category", orbCategoryBody{CategoryID: categoryID}, &env,
		routeParams(orbID),
	)
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return fmt.Errorf("%w: %q", ErrOrbNotFound, orbID)
	}
	return err
}

// ListOrbVersions lists all versions for an orb, depaginating automatically.
// channel can be "stable", "dev", or "" for all.
func (c *Client) ListOrbVersions(ctx context.Context, orbID, channel string) ([]*OrbVersion, error) {
	var result []*OrbVersion
	cursor := ""
	for {
		var page v3List[orbVersionWire]
		if err := c.getV3(ctx, "/orb/versions", &page,
			filterParam("orb_id", orbID),
			filterParam("channel", channel),
			pageCursor(cursor),
		); err != nil {
			return nil, err
		}
		for i := range page.Data {
			result = append(result, page.Data[i].toOrbVersion())
		}
		if page.Page.Next == nil || *page.Page.Next == "" {
			break
		}
		cursor = *page.Page.Next
	}
	return result, nil
}

// GetOrbVersionByRef gets an orb version by its full ref (e.g. "ns/name@1.2.3" or "ns/name@volatile").
func (c *Client) GetOrbVersionByRef(ctx context.Context, ref string) (*OrbVersion, error) {
	var env v3List[orbVersionWire]
	err := c.getV3(ctx, "/orb/versions", &env,
		filterParam("ref", ref),
	)
	if err != nil {
		return nil, err
	}
	if len(env.Data) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrOrbVersionNotFound, ref)
	}
	return env.Data[0].toOrbVersion(), nil
}

// GetOrbVersionByID gets a single orb version by UUID (includes source YAML).
func (c *Client) GetOrbVersionByID(ctx context.Context, id string) (*OrbVersion, error) {
	var env v3Entity[orbVersionWire]
	err := c.getV3(ctx, "/orb/versions/%s", &env,
		routeParams(id),
	)
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return nil, fmt.Errorf("%w: %q", ErrOrbVersionNotFound, id)
	}
	if err != nil {
		return nil, err
	}
	return env.Data.toOrbVersion(), nil
}

func (c *Client) GetOrbSource(ctx context.Context, id string) (string, error) {
	body := ""
	err := c.getV3String(ctx, "/orb/versions/%s/source", &body,
		routeParams(id),
	)
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return "", fmt.Errorf("%w: %q", ErrOrbVersionNotFound, id)
	}
	if err != nil {
		return "", err
	}
	return body, nil
}

type orbYAMLBody struct {
	YAML  string `json:"yaml"`
	OrgID string `json:"org_id,omitempty"`
}

type orbSetListedBody struct {
	Listed bool `json:"is_listed"`
}

type orbCategoryBody struct {
	CategoryID string `json:"category_id"`
}

type orbPromoteBody struct {
	Segment string `json:"segment"`
}

// PublishOrbVersionRequest is the body for publishing an orb version.
type PublishOrbVersionRequest struct {
	OrbID   string `json:"orb_id"`
	YAML    string `json:"yaml"`
	Version string `json:"version"`
}

// PublishOrbVersion publishes a new orb version.
func (c *Client) PublishOrbVersion(ctx context.Context, req PublishOrbVersionRequest) (*OrbVersion, error) {
	var wire v3Entity[publishOrbVersionWire]
	wire.Data.Attributes = req

	var env v3Entity[orbVersionWire]
	err := c.postV3(ctx, "/orb/versions", wire, &env)
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return nil, fmt.Errorf("%w: orb %q", ErrOrbNotFound, req.OrbID)
	}
	if err != nil {
		return nil, err
	}
	return env.Data.toOrbVersion(), nil
}

// PromoteOrbVersion promotes a dev orb version to a stable semver.
// segment must be "major", "minor", or "patch".
func (c *Client) PromoteOrbVersion(ctx context.Context, versionID, segment string) (*OrbVersion, error) {
	var env v3Entity[orbVersionWire]
	err := c.postV3(ctx, "/orb/versions/%s/promote", orbPromoteBody{Segment: segment}, &env,
		routeParams(versionID),
	)
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return nil, fmt.Errorf("%w: version %q", ErrOrbVersionNotFound, versionID)
	}
	if err != nil {
		return nil, err
	}
	return env.Data.toOrbVersion(), nil
}

// ListOrbCategories lists all orb categories, depaginating automatically.
func (c *Client) ListOrbCategories(ctx context.Context) ([]*OrbCategory, error) {
	var result []*OrbCategory
	cursor := ""
	for {
		var page v3List[orbCategoryWire]
		if err := c.getV3(ctx, "/orb/categories", &page, pageCursor(cursor)); err != nil {
			return nil, err
		}
		for _, cat := range page.Data {
			result = append(result, &OrbCategory{ID: cat.ID, Name: cat.Attributes.Name})
		}
		if page.Page.Next == nil || *page.Page.Next == "" {
			break
		}
		cursor = *page.Page.Next
	}
	return result, nil
}

// GetOrbCategoryByName finds a category by exact name.
func (c *Client) GetOrbCategoryByName(ctx context.Context, name string) (*OrbCategory, error) {
	var env v3List[orbCategoryWire]
	err := c.getV3(ctx, "/orb/categories", &env,
		filterParam("name", name),
	)
	if err != nil {
		return nil, err
	}
	if len(env.Data) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrOrbCategoryNotFound, name)
	}
	return &OrbCategory{ID: env.Data[0].ID, Name: env.Data[0].Attributes.Name}, nil
}
