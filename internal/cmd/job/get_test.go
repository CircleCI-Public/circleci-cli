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

package job

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func Test_formatCommand(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty",
			in:   "",
			want: "-",
		},
		{
			name: "single line",
			in:   "task ci:test",
			want: "`task ci:test`",
		},
		{
			name: "shebang and command",
			in:   "#!/bin/bash -eo pipefail\ntask ci:test",
			want: "`#!/bin/bash -eo pipefail` `task ci:test`",
		},
		{
			name: "truncates to first two lines with ellipsis",
			in:   "#!/bin/bash -eo pipefail\ntask ci:test\ntask ci:lint\ntask ci:build",
			want: "`#!/bin/bash -eo pipefail` `task ci:test` …",
		},
		{
			name: "escapes pipes",
			in:   "cat foo | grep bar",
			want: "`cat foo \\| grep bar`",
		},
		{
			name: "escapes pipes across both lines",
			in:   "#!/bin/bash -eo pipefail\ncat foo | grep bar | wc -l",
			want: "`#!/bin/bash -eo pipefail` `cat foo \\| grep bar \\| wc -l`",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(formatCommand(tt.in), tt.want))
		})
	}
}
