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

package run

import (
	"errors"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

func TestValidateRunStatus(t *testing.T) {
	t.Run("accepts every pipeline status", func(t *testing.T) {
		for _, s := range apiclient.RunStatuses {
			assert.Check(t, validateRunStatus(s), "status %q", s)
		}
	})

	t.Run("accepts empty as no filter", func(t *testing.T) {
		assert.Check(t, validateRunStatus(""))
	})

	t.Run("rejects an unknown status as a bad argument", func(t *testing.T) {
		err := validateRunStatus("succeeded")
		cliErr, ok := errors.AsType[*clierrors.CLIError](err)
		assert.Assert(t, ok, "want a CLIError, got %v", err)
		assert.Check(t, is.Equal(cliErr.Code, "args.invalid_status"))
		assert.Check(t, is.Equal(cliErr.Message, `"succeeded" is not a valid run status.`))
		assert.Check(t, is.Equal(cliErr.ExitCode, clierrors.ExitBadArguments))
		assert.Check(t, is.DeepEqual(cliErr.Suggestions, []string{
			"Valid values: canceled, error, failed, failing, not_run, on_hold, queued, running, success, unauthorized",
		}))
	})
}
