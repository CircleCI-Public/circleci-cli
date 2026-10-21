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
	"encoding/json"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
)

const feedbackMessage = "run get shows no jobs for a run that has three"

func feedbackEnv(t *testing.T, fake *fakes.CircleCI) *testenv.TestEnv {
	t.Helper()
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return env
}

func TestFeedback(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := feedbackEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"feedback", feedbackMessage, "--tool", "run get"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/tool/send-feedback"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"message":"` + feedbackMessage + `","tool_name":"run get","source":"cli"}`),
		}, ignoreCommonHeaders))
	})
}

func TestFeedback_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := feedbackEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"feedback", feedbackMessage, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, map[string]any{"id": fakes.FeedbackReportID}))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

// TestFeedback_UnderMCP covers a tool call from the CLI's own MCP server: the
// source says so, and the agent is reported without the "mcp/" prefix.
func TestFeedback_UnderMCP(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := feedbackEnv(t, fake)
	env.Extra = map[string]string{
		"CIRCLE_MCP": "1",
		"CLAUDECODE": "1",
	}

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"feedback", feedbackMessage},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	var body map[string]any
	assert.NilError(t, fake.LastRequest().Decode(&body))
	assert.Check(t, cmp.DeepEqual(body, map[string]any{
		"message":    feedbackMessage,
		"source":     "cli_mcp",
		"agent_type": "claude-code",
	}))
}

func TestFeedback_Validation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "blank message",
			args: []string{"feedback", "   "},
		},
		{
			// Multi-byte characters show the limit counts characters, not bytes.
			name: "message too long",
			args: []string{"feedback", strings.Repeat("é", 2001)},
		},
		{
			name: "tool too long",
			args: []string{"feedback", feedbackMessage, "--tool", strings.Repeat("x", 201)},
		},
		{
			name: "unquoted message",
			args: []string{"feedback", "run", "get", "is", "broken"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := fakes.NewCircleCI(t)
			env := feedbackEnv(t, fake)

			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    tc.args,
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})

			assert.Check(t, cmp.Equal(result.ExitCode, 2))
			assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
			assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
			assert.Check(t, cmp.Len(fake.AllRequests(), 0))
		})
	}
}

func TestFeedback_NoToken(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"feedback", feedbackMessage},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 3))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, cmp.Len(fake.AllRequests(), 0))
}

func TestFeedback_Rejected(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetFeedbackReportError(http.StatusBadRequest, "Validation Error",
		"Field 'message' failed 'max=2000' validation.")
	env := feedbackEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"feedback", feedbackMessage},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 4))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestFeedback_Unavailable covers a host without the endpoint, such as a
// server install.
func TestFeedback_Unavailable(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetFeedbackReportError(http.StatusNotFound, "Not Found", "")
	env := feedbackEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"feedback", feedbackMessage},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestFeedback_ServerError shows a failed report does not suggest sending
// another one.
func TestFeedback_ServerError(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetFeedbackReportError(http.StatusInternalServerError, "Internal Server Error.", "")
	env := feedbackEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"feedback", feedbackMessage},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 4))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, !strings.Contains(result.Stderr, "circleci feedback"), "stderr: %s", result.Stderr)

	t.Run("retries before giving up", func(t *testing.T) {
		assert.Check(t, cmp.Len(fake.AllRequests(), 4))
	})
}
