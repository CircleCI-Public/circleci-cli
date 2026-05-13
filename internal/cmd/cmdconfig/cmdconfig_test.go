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

package cmdconfig_test

import (
	"errors"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/cmd/cmdconfig"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
)

func TestNewConfigCmd_Shape(t *testing.T) {
	cmd := cmdconfig.NewConfigCmd()
	assert.Check(t, cmp.Equal(cmd.Use, "config <command>"))
	assert.Check(t, cmd.Short != "", "Short must be non-empty")
	assert.Check(t, cmd.Long != "", "Long must be non-empty")
	assert.Check(t, cmd.FParseErrWhitelist.UnknownFlags, "group must whitelist unknown flags so subcommand resolution wins over flag parsing")
}

func TestNewConfigCmd_NoArgs_ShowsHelp(t *testing.T) {
	cmd := cmdconfig.NewConfigCmd()
	// RunE with no args should return nil (help is rendered) — matches GroupRunE.
	err := cmd.RunE(cmd, nil)
	assert.NilError(t, err)
}

func TestNewConfigCmd_UnknownSubcommand_ReturnsStructuredError(t *testing.T) {
	cmd := cmdconfig.NewConfigCmd()
	err := cmd.RunE(cmd, []string{"bogus"})
	assert.Assert(t, err != nil, "expected error for unknown subcommand")

	var cliErr *clierrors.CLIError
	assert.Assert(t, errors.As(err, &cliErr), "expected CLIError, got %T", err)
	assert.Check(t, cmp.Equal(cliErr.Code, "command.unknown"))
	assert.Check(t, cmp.Equal(cliErr.ExitCode, clierrors.ExitBadArguments))
}
