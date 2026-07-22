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
	"net/http/httptest"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// TestDeprecationWarning_SunsetInOutput checks that Deprecation + Sunset response
// headers produce a warning on stderr while the command still succeeds.
func TestDeprecationWarning_SunsetInOutput(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddResourceClass(map[string]any{
		"id":             "rc-1",
		"resource_class": "myns/myclass",
		"description":    "test class",
	})
	fake.ExtraHeaders = http.Header{
		"Deprecation": []string{"true"},
		"Sunset":      []string{"Sat, 01 Jan 2028 00:00:00 GMT"},
	}

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--namespace", "myns"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	// Command succeeds — endpoint still works during the sunset window.
	assert.Check(t, result.ExitCode == 0, "expected exit 0, stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "myns/myclass"), "expected resource class in stdout: %s", result.Stdout)

	// Warning appears on stderr.
	assert.Check(t, strings.Contains(result.Stderr, "deprecated"), "expected deprecation warning on stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "days"), "expected days-remaining in warning: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "upgrade"), "expected upgrade hint in warning: %s", result.Stderr)
}

// TestGone_410ProducesUpgradeError checks that a 410 response causes the CLI to
// exit non-zero with a clear "out of date" message.
func TestGone_410ProducesUpgradeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusGone)
	}))
	t.Cleanup(srv.Close)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = srv.URL

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--namespace", "myns"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, result.ExitCode != 0, "expected non-zero exit on 410")
	assert.Check(t, strings.Contains(result.Stderr, "out of date"), "expected 'out of date' in stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "pgrade"), "expected upgrade suggestion in stderr: %s", result.Stderr)
}

// TestGone_410WithServerMessageInOutput checks that the server-provided message
// in the 410 body is surfaced in the CLI error output.
func TestGone_410WithServerMessageInOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGone)
		body, _ := json.Marshal(map[string]string{"error": "upgrade circleci CLI to v2.x or later"})
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = srv.URL

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"runner", "resource-class", "list", "--namespace", "myns"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, result.ExitCode != 0, "expected non-zero exit on 410")
	assert.Check(t, strings.Contains(result.Stderr, "out of date"), "expected 'out of date' in stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "upgrade circleci CLI to v2.x"), "expected server message in stderr: %s", result.Stderr)
}
