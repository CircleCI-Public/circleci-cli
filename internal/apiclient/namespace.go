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
)

// Namespace represents a CircleCI orb registry namespace.
type Namespace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ErrNamespaceNotFound is returned by GetNamespace when the namespace does not exist.
var ErrNamespaceNotFound = errors.New("namespace not found")

// GetNamespace looks up a namespace by name and returns its ID and name.
func (c *Client) GetNamespace(ctx context.Context, name string) (*Namespace, error) {
	var data struct {
		RegistryNamespace *struct {
			ID string `json:"id"`
		} `json:"registryNamespace"`
	}
	if err := c.graphQL(ctx,
		`query GetNamespace($name: String!) { registryNamespace(name: $name) { id } }`,
		map[string]any{"name": name}, &data); err != nil {
		return nil, err
	}
	if data.RegistryNamespace == nil || data.RegistryNamespace.ID == "" {
		return nil, fmt.Errorf("%w: %q", ErrNamespaceNotFound, name)
	}
	return &Namespace{ID: data.RegistryNamespace.ID, Name: name}, nil
}

// CreateNamespace creates a namespace for the given organization ID.
func (c *Client) CreateNamespace(ctx context.Context, name, orgID string) (*Namespace, error) {
	var data struct {
		CreateNamespace struct {
			Namespace *struct {
				ID string `json:"id"`
			} `json:"namespace"`
			Errors []gqlAppError `json:"errors"`
		} `json:"createNamespace"`
	}
	if err := c.graphQL(ctx, `
		mutation CreateNamespace($name: String!, $organizationId: UUID!) {
			createNamespace(name: $name, organizationId: $organizationId) {
				namespace { id }
				errors { message type }
			}
		}`,
		map[string]any{"name": name, "organizationId": orgID}, &data); err != nil {
		return nil, err
	}
	if len(data.CreateNamespace.Errors) > 0 {
		return nil, gqlAppErrorsToErr(data.CreateNamespace.Errors)
	}
	if data.CreateNamespace.Namespace == nil {
		return nil, fmt.Errorf("namespace creation failed for unknown reasons")
	}
	return &Namespace{ID: data.CreateNamespace.Namespace.ID, Name: name}, nil
}

// RenameNamespace renames a namespace. The current name is resolved to an ID first.
func (c *Client) RenameNamespace(ctx context.Context, name, newName string) (*Namespace, error) {
	ns, err := c.GetNamespace(ctx, name)
	if err != nil {
		return nil, err
	}
	var data struct {
		RenameNamespace struct {
			Namespace *struct {
				ID string `json:"id"`
			} `json:"namespace"`
			Errors []gqlAppError `json:"errors"`
		} `json:"renameNamespace"`
	}
	if err := c.graphQL(ctx, `
		mutation RenameNamespace($namespaceId: UUID!, $newName: String!) {
			renameNamespace(namespaceId: $namespaceId, newName: $newName) {
				namespace { id }
				errors { message type }
			}
		}`,
		map[string]any{"namespaceId": ns.ID, "newName": newName}, &data); err != nil {
		return nil, err
	}
	if len(data.RenameNamespace.Errors) > 0 {
		if gqlHasType(data.RenameNamespace.Errors, "NOT_FOUND") {
			return nil, fmt.Errorf("%w: %q", ErrNamespaceNotFound, name)
		}
		return nil, gqlAppErrorsToErr(data.RenameNamespace.Errors)
	}
	if data.RenameNamespace.Namespace == nil {
		return nil, fmt.Errorf("namespace rename failed for unknown reasons")
	}
	return &Namespace{ID: data.RenameNamespace.Namespace.ID, Name: newName}, nil
}

// DeleteNamespace deletes a namespace and all its orbs.
// The name is resolved to an ID first.
func (c *Client) DeleteNamespace(ctx context.Context, name string) error {
	ns, err := c.GetNamespace(ctx, name)
	if err != nil {
		return err
	}
	var data struct {
		DeleteNamespaceAndRelatedOrbs struct {
			Deleted bool          `json:"deleted"`
			Errors  []gqlAppError `json:"errors"`
		} `json:"deleteNamespaceAndRelatedOrbs"`
	}
	if err := c.graphQL(ctx, `
		mutation DeleteNamespace($id: UUID!) {
			deleteNamespaceAndRelatedOrbs(namespaceId: $id) {
				deleted
				errors { message type }
			}
		}`,
		map[string]any{"id": ns.ID}, &data); err != nil {
		return err
	}
	if len(data.DeleteNamespaceAndRelatedOrbs.Errors) > 0 {
		if gqlHasType(data.DeleteNamespaceAndRelatedOrbs.Errors, "NOT_FOUND") {
			return fmt.Errorf("%w: %q", ErrNamespaceNotFound, name)
		}
		return gqlAppErrorsToErr(data.DeleteNamespaceAndRelatedOrbs.Errors)
	}
	if !data.DeleteNamespaceAndRelatedOrbs.Deleted {
		return fmt.Errorf("namespace deletion failed for unknown reasons")
	}
	return nil
}

type gqlAppError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func gqlAppErrorsToErr(errs []gqlAppError) *GQLError {
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Message
	}
	return &GQLError{messages: msgs}
}

func gqlHasType(errs []gqlAppError, typ string) bool {
	for _, e := range errs {
		if e.Type == typ {
			return true
		}
	}
	return false
}
