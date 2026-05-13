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

	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
)

// Namespace represents a CircleCI orb registry namespace.
type Namespace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ErrNamespaceNotFound is returned by GetNamespace when the namespace does not exist.
var ErrNamespaceNotFound = errors.New("namespace not found")

// namespaceEnvelope is the response envelope for /api/v3/namespaces endpoints.
type namespaceEnvelope struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Name string `json:"name"`
		} `json:"attributes"`
	} `json:"data"`
}

func (e *namespaceEnvelope) toNamespace() *Namespace {
	return &Namespace{ID: e.Data.ID, Name: e.Data.Attributes.Name}
}

// GetNamespace looks up a namespace by name and returns its ID and name.
func (c *Client) GetNamespace(ctx context.Context, name string) (*Namespace, error) {
	var env namespaceEnvelope
	err := c.getV3(ctx, "/namespaces", &env, queryParam("filter[name]", name))
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return nil, fmt.Errorf("%w: %q", ErrNamespaceNotFound, name)
	}
	if err != nil {
		return nil, err
	}
	return env.toNamespace(), nil
}

// CreateNamespace creates a namespace for the given organization ID.
func (c *Client) CreateNamespace(ctx context.Context, name, orgID string) (*Namespace, error) {
	var env namespaceEnvelope
	err := c.postV3(ctx, "/namespaces", map[string]any{
		"name":            name,
		"organization_id": orgID,
	}, &env)
	if err != nil {
		return nil, err
	}
	return env.toNamespace(), nil
}

// RenameNamespace renames a namespace. The current name is resolved to an ID first.
func (c *Client) RenameNamespace(ctx context.Context, name, newName string) (*Namespace, error) {
	ns, err := c.GetNamespace(ctx, name)
	if err != nil {
		return nil, err
	}
	var env namespaceEnvelope
	err = c.postV3(ctx, "/namespaces/"+ns.ID+"/rename", map[string]any{"name": newName}, &env)
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return nil, fmt.Errorf("%w: %q", ErrNamespaceNotFound, name)
	}
	if err != nil {
		return nil, err
	}
	return env.toNamespace(), nil
}

// DeleteNamespace deletes a namespace and all its orbs.
// The name is resolved to an ID first.
func (c *Client) DeleteNamespace(ctx context.Context, name string) error {
	ns, err := c.GetNamespace(ctx, name)
	if err != nil {
		return err
	}
	err = c.deleteV3(ctx, "/namespaces/"+ns.ID)
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return fmt.Errorf("%w: %q", ErrNamespaceNotFound, name)
	}
	return err
}
