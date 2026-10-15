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
package cmdutil

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
)

func TestBindPositional(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		flags     []string // flags to mark as set, as "--name=value" pairs
		conflicts []string
		wantValue string
		wantCode  string
		wantMsg   string
		wantFirst string // expected first suggestion, when set
	}{
		{name: "no positional leaves flag alone", wantValue: ""},
		{name: "no positional keeps flag value", flags: []string{"--resource-class=from-flag"}, wantValue: "from-flag"},
		{name: "positional binds to target", args: []string{"ns/rc"}, wantValue: "ns/rc"},
		{
			name:      "positional with target flag conflicts",
			args:      []string{"ns/rc"},
			flags:     []string{"--resource-class=other"},
			wantMsg:   `Got both a positional argument "ns/rc" and --resource-class.`,
			wantFirst: "Pass the value either as the positional argument or with --resource-class, not both",
		},
		{name: "empty positional is rejected", args: []string{""}, wantCode: "args.invalid_value", wantMsg: "The positional argument is empty."},
		{name: "whitespace positional is rejected", args: []string{"  "}, wantCode: "args.invalid_value", wantMsg: "The positional argument is empty."},
		{
			name:      "positional with named conflicting flag",
			args:      []string{"ns/rc"},
			flags:     []string{"--org=gh/o"},
			conflicts: []string{"org"},
			wantMsg:   `Got both a positional argument "ns/rc" and --org.`,
			wantFirst: "Drop --org: the positional argument already selects the resource-class filter",
		},
		{
			name:      "unset conflicting flag is fine",
			args:      []string{"ns/rc"},
			conflicts: []string{"org"},
			wantValue: "ns/rc",
		},
		{
			name:      "conflict flag without positional is fine",
			flags:     []string{"--org=gh/o"},
			conflicts: []string{"org"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rc, org string
			cmd := &cobra.Command{Use: "list"}
			cmd.Flags().StringVar(&rc, "resource-class", "", "")
			cmd.Flags().StringVar(&org, "org", "", "")
			assert.NilError(t, cmd.ParseFlags(tt.flags))

			err := BindPositional(cmd, tt.args, "resource-class", tt.conflicts...)

			if tt.wantMsg != "" {
				var cliErr *clierrors.CLIError
				assert.Assert(t, errors.As(err, &cliErr), "err: %v", err)
				wantCode := tt.wantCode
				if wantCode == "" {
					wantCode = "args.positional_conflict"
				}
				assert.Check(t, cmp.Equal(cliErr.Code, wantCode))
				assert.Check(t, cmp.Equal(cliErr.Message, tt.wantMsg))
				assert.Check(t, cmp.Equal(cliErr.ExitCode, clierrors.ExitBadArguments))
				assert.Check(t, cmp.Len(cliErr.Suggestions, 2))
				if tt.wantFirst != "" {
					assert.Assert(t, cmp.Len(cliErr.Suggestions, 2))
					assert.Check(t, cmp.Equal(cliErr.Suggestions[0], tt.wantFirst))
				}
				return
			}
			assert.NilError(t, err)
			assert.Check(t, cmp.Equal(rc, tt.wantValue))
			if len(tt.args) > 0 {
				assert.Check(t, cmp.Equal(cmd.Flags().Changed("resource-class"), true), "positional must mark the flag changed")
			}
		})
	}
}
