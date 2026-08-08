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

package policy

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
)

// loadPolicyBundle reads all .rego files under root and returns them as a
// PolicyBundle. Returns a structured error if root is not a directory.
func loadPolicyBundle(root string) (apiclient.PolicyBundle, error) {
	root = filepath.Clean(root)

	info, err := os.Stat(root)
	if err != nil {
		return nil, clierrors.New("policy.bundle_read_failed", "Could not read policy directory",
			err.Error()).
			WithSuggestions("Check that the path exists and is readable").
			WithExitCode(clierrors.ExitBadArguments)
	}
	if !info.IsDir() {
		return nil, clierrors.New("policy.bundle_not_dir", "Policy path is not a directory",
			root+" is not a directory").
			WithSuggestions("Pass the path to a directory containing .rego files").
			WithExitCode(clierrors.ExitBadArguments)
	}

	bundle := make(apiclient.PolicyBundle)
	err = filepath.WalkDir(root, func(path string, f fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if f.IsDir() || filepath.Ext(path) != ".rego" {
			return nil
		}
		content, err := os.ReadFile(filepath.Clean(path)) //nolint:gosec // path comes from WalkDir
		if err != nil {
			return clierrors.New("policy.bundle_read_failed", "Could not read policy file",
				err.Error()).WithExitCode(clierrors.ExitBadArguments)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		bundle[rel] = string(content)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return bundle, nil
}
