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

package extschema_test

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/extschema"
)

func sortedCopy(s []string) []string {
	c := slices.Clone(s)
	slices.Sort(c)
	return c
}

// TestExportMatchesCommittedFile keeps schema.json equal to what the code
// holds. Regenerate it with -update.
func TestExportMatchesCommittedFile(t *testing.T) {
	got, err := extschema.Export()
	assert.NilError(t, err)
	assert.Check(t, golden.String(string(got), "../schema.json"))
}

func TestExportIsStable(t *testing.T) {
	a, err := extschema.Export()
	assert.NilError(t, err)
	b, err := extschema.Export()
	assert.NilError(t, err)
	assert.Check(t, cmp.DeepEqual(a, b))
}

// TestDeclarationMatchesSchema fails when the schema and the extension's
// declaration name different short names.
func TestDeclarationMatchesSchema(t *testing.T) {
	raw, err := os.ReadFile("testdata/configoptimizer.declaration.json")
	assert.NilError(t, err)
	var decl struct {
		ShortNames []string `json:"short_names"`
	}
	err = json.Unmarshal(raw, &decl)
	assert.NilError(t, err)
	assert.Check(t, cmp.DeepEqual(sortedCopy(decl.ShortNames), extschema.ShortNames("configoptimizer")))
}
