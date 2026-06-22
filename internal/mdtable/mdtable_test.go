// Copyright (c) 2026 Circle Internet Services, Inc.
//
// SPDX-License-Identifier: MIT

package mdtable

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestRender(t *testing.T) {
	tbl := New("Name", "Value")
	tbl.Row("FOO", "bar")

	want := "| Name | Value |\n| ---- | ----- |\n| FOO  | bar   |\n"
	assert.Check(t, cmp.Equal(tbl.Render(), want))
}

func TestRightAlignSeparator(t *testing.T) {
	tbl := New("Name", "Value")
	tbl.RightAlign(1)
	tbl.Row("FOO", "bar")

	// A right-aligned column should render a trailing colon in the separator
	// row so GFM renderers right-align it.
	want := "| Name | Value |\n| ---- | ----: |\n| FOO  |   bar |\n"
	assert.Check(t, cmp.Equal(tbl.Render(), want))
}
