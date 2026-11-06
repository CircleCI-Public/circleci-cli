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

package acceptance_test

import (
	"context"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/segmentio/analytics-go/v3"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/extension"
	"github.com/CircleCI-Public/circleci-cli/internal/telemetry"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakesegment"
)

func TestExtensionTelemetry(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	registry := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = registry.URL()

	const extName = "testextension"
	registry.WithExtension(t,
		extension.Manifest{
			Name:       extName,
			BinaryName: "circleci-" + extName,
			Version:    "1.0.0",
			Path:       testBinaryPath,
			Ref: &extension.Reference{
				Subcommands: []extension.ReferenceSub{
					{Name: "setup", Reference: extension.Reference{
						Subcommands: []extension.ReferenceSub{{Name: "guide"}},
					}},
					{Name: "exit"},
				},
			},
		},
		fakes.ExtensionMeta{Arch: runtime.GOARCH, Sys: runtime.GOOS},
	)
	assert.Assert(t, t.Run("install", func(t *testing.T) {
		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    []string{"extension", "install", extName},
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})
		assert.Assert(t, cmp.Equal(result.ExitCode, 0))
	}))

	// run executes the extension with telemetry on and returns the properties of
	// its command_invocation event.
	run := func(t *testing.T, args ...string) (binary.CLIResult, analytics.Properties) {
		t.Helper()
		env.Telemetry = true
		fs := fakesegment.New(ctx, telemetry.SegmentKey)
		fsSrv := httptest.NewServer(fs)
		t.Cleanup(fsSrv.Close)
		env.Extra["CIRCLE_TELEMETRY_ENDPOINT"] = fsSrv.URL

		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    append([]string{extName}, args...),
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})

		var props analytics.Properties
		poll.WaitOn(t, func(t poll.LogT) poll.Result {
			for _, batch := range fs.Batches() {
				for _, msg := range batch.Messages {
					if msg.Event == "command_invocation" {
						props = msg.Properties
						return poll.Success()
					}
				}
			}
			return poll.Continue("no command_invocation event yet")
		})
		return result, props
	}

	t.Run("records a declared subcommand path", func(t *testing.T) {
		result, props := run(t, "setup", "guide", "--json")
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, cmp.Equal(props["command"], "circleci "+extName))
		assert.Check(t, cmp.Equal(props["extension_subcommand"], "setup guide"))
		assert.Check(t, cmp.Equal(props["exit_code"], float64(0)))
	})

	t.Run("records no subcommand when the first arg is a value", func(t *testing.T) {
		_, props := run(t, "ci tests", "--doctor")
		_, ok := props["extension_subcommand"]
		assert.Check(t, !ok, "an undeclared arg must not be recorded: %v", props)
	})

	t.Run("records the extension's own exit code", func(t *testing.T) {
		result, props := run(t, "exit", "3")
		assert.Check(t, cmp.Equal(result.ExitCode, 3))
		assert.Check(t, cmp.Equal(props["extension_subcommand"], "exit"))
		assert.Check(t, cmp.Equal(props["exit_code"], float64(result.ExitCode)))
		assert.Check(t, cmp.Equal(props["error_code"], "extension.exited"))
	})
}
