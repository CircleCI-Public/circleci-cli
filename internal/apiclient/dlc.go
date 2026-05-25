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

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// ErrDLCGone is returned by PurgeDLC when the endpoint responds 410 Gone,
// indicating the feature has been retired or the CLI needs upgrading.
var ErrDLCGone = errors.New("dlc: endpoint no longer available")

// PurgeDLC purges the Docker Layer Cache for the given project ID.
func (c *Client) PurgeDLC(ctx context.Context, projectID string) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v3/projects/%s/dlc",
		routeParams(projectID),
	))
	he, ok := errors.AsType[*httpcl.HTTPError](err)
	switch {
	case httpcl.HasStatusCode(err, http.StatusGone):
		return fmt.Errorf("%w", ErrDLCGone)
	case ok:
		fmt.Printf("API returned %d: %s\n", he.StatusCode, string(he.Body))
		return err
	case err != nil:
		return err
	default:
		return nil
	}
}
