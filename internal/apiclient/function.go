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

// ErrFunctionNotFound is returned when a function is not published, or is
// published under a name the discovery API does not list.
var ErrFunctionNotFound = errors.New("function not found")

// Function is a published CircleCI function. Name is the VCS-style identifier
// it is published under, e.g. "github.com/circleci-functions/setup-go".
type Function struct {
	ID            uuid.UUID
	Name          string
	Description   string
	LatestVersion string
	Versions      []FunctionVersion
}

// FunctionVersion is one published version of a function.
type FunctionVersion struct {
	ID      uuid.UUID
	Version string
}

// Find returns the named version, or false when it is not published.
func (f *Function) Find(version string) (FunctionVersion, bool) {
	for _, v := range f.Versions {
		if v.Version == version {
			return v, true
		}
	}
	return FunctionVersion{}, false
}

type functionWire struct {
	ID         uuid.UUID `json:"id"`
	Attributes struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		LatestVersion string `json:"latest_version"`
	} `json:"attributes"`
	References struct {
		Versions []struct {
			ID         uuid.UUID `json:"id"`
			Attributes struct {
				Version string `json:"version"`
			} `json:"attributes"`
		} `json:"versions"`
	} `json:"references"`
}

func (w *functionWire) toFunction() *Function {
	fn := &Function{
		ID:            w.ID,
		Name:          w.Attributes.Name,
		Description:   w.Attributes.Description,
		LatestVersion: w.Attributes.LatestVersion,
	}
	for _, v := range w.References.Versions {
		fn.Versions = append(fn.Versions, FunctionVersion{ID: v.ID, Version: v.Attributes.Version})
	}
	return fn
}

// ListFunctions lists every discoverable published function, following the
// cursor until the whole collection has been read.
func (c *Client) ListFunctions(ctx context.Context) ([]*Function, error) {
	result := []*Function{}
	cursor := ""
	for {
		var page v3List[functionWire]
		_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/function/packages",
			pageCursor(cursor),
			httpcl.JSONDecoder(&page),
		))
		if err != nil {
			return nil, err
		}
		for i := range page.Data {
			result = append(result, page.Data[i].toFunction())
		}
		if page.Page.Next == nil || *page.Page.Next == "" {
			break
		}
		cursor = *page.Page.Next
	}
	return result, nil
}

// GetFunctionByName resolves a function by its VCS-style name. An unknown name,
// and any name the API does not consider discoverable, comes back as an empty
// collection rather than a 404.
func (c *Client) GetFunctionByName(ctx context.Context, name string) (*Function, error) {
	var env v3List[functionWire]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/function/packages",
		filterParam("name", name),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}
	// Matching the name here as well keeps a server that ignores or loosens the
	// filter from answering with a different function.
	for i := range env.Data {
		if env.Data[i].Attributes.Name == name {
			return env.Data[i].toFunction(), nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrFunctionNotFound, name)
}
