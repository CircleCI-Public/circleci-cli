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

package iostreamcobra

import (
	"context"

	"charm.land/glamour/v2"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// FromCmd builds iostream Options from cmd and returns a context carrying the
// resulting Streams.
//
// Streams come from the command's In/Out/Err, so anything cobra has redirected
// (as tests do) is honoured. The --quiet and --debug persistent flags are read
// when registered; a command that does not define them simply gets false, which
// is why the small internal tools under cmd/ can call this while registering
// only --debug.
//
// mdOpts are passed straight through to Options.MarkdownOptions.
func FromCmd(ctx context.Context, cmd *cobra.Command, configTheme string, mdOpts ...glamour.TermRendererOption) context.Context {
	quiet, _ := cmd.Flags().GetBool("quiet")
	debug, _ := cmd.Flags().GetBool("debug")

	return iostream.New(ctx, iostream.Options{
		In:              cmd.InOrStdin(),
		Out:             cmd.OutOrStdout(),
		Err:             cmd.ErrOrStderr(),
		Quiet:           quiet,
		Debug:           debug,
		Theme:           resolveTheme(cmd, configTheme),
		MarkdownOptions: mdOpts,
	})
}

// resolveTheme picks the color theme in precedence order:
//  1. an explicitly-passed --theme flag (always wins over config),
//  2. the stored config theme (configTheme; "" when none is configured),
//  3. the --theme flag's default value ("auto").
//
// A command with no --theme flag gets "", which iostream reads as "auto".
func resolveTheme(cmd *cobra.Command, configTheme string) string {
	flagTheme, _ := cmd.Flags().GetString("theme")
	if cmd.Flags().Changed("theme") {
		return flagTheme
	}
	if configTheme != "" {
		return configTheme
	}
	return flagTheme
}
