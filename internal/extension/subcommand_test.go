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

package extension_test

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/extension"
)

func TestReferenceSubcommandPath(t *testing.T) {
	ref := &extension.Reference{
		Subcommands: []extension.ReferenceSub{
			{
				Name: "setup",
				Reference: extension.Reference{
					Subcommands: []extension.ReferenceSub{
						{Name: "guide", Reference: extension.Reference{
							Subcommands: []extension.ReferenceSub{
								{Name: "one", Reference: extension.Reference{
									Subcommands: []extension.ReferenceSub{{Name: "two"}},
								}},
							},
						}},
					},
				},
			},
			{Name: "run"},
			{Name: "Bad_Name"},
		},
	}

	tests := []struct {
		name string
		ref  *extension.Reference
		args []string
		want string
	}{
		{
			name: "nil reference",
			args: []string{"setup"},
		},
		{
			name: "no args",
			ref:  ref,
		},
		{
			name: "one level",
			ref:  ref,
			args: []string{"run"},
			want: "run",
		},
		{
			name: "nested",
			ref:  ref,
			args: []string{"setup", "guide"},
			want: "setup guide",
		},
		{
			name: "stops at a flag",
			ref:  ref,
			args: []string{"setup", "--json", "guide"},
			want: "setup",
		},
		{
			name: "stops at a value",
			ref:  ref,
			args: []string{"run", "ci tests"},
			want: "run",
		},
		{
			name: "a value first records nothing",
			ref:  ref,
			args: []string{"ci tests", "--doctor"},
		},
		{
			name: "a flag first records nothing",
			ref:  ref,
			args: []string{"--project", "setup"},
		},
		{
			name: "a name declared at another level is not matched",
			ref:  ref,
			args: []string{"guide"},
		},
		{
			name: "a declared name of the wrong shape is not reported",
			ref:  ref,
			args: []string{"Bad_Name"},
		},
		{
			name: "depth is capped",
			ref:  ref,
			args: []string{"setup", "guide", "one", "two"},
			want: "setup guide one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(tt.ref.SubcommandPath(tt.args), tt.want))
		})
	}
}
