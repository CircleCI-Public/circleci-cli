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

package apicompile_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/apicompile"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const configYAML = "version: 2.1\njobs: {}\n"

func compiler(url string) apicompile.Compiler {
	return apicompile.Compiler{Client: apiclient.New(apiclient.Config{BaseURL: url, Token: "tok", Version: "1.0.0"})}
}

// TestCompile covers how the compile API's failures map to errors; a
// successful compile is covered by acceptance/config_optimize_test.go.
func TestCompile(t *testing.T) {
	t.Run("a rejected config is an InvalidConfigError with the API's messages", func(t *testing.T) {
		fake := fakes.NewCircleCI(t)
		fake.SetCompileResponse(false, "", "unknown orb 'myorg/unknown@1.0.0'")
		_, err := compiler(fake.URL()).Compile(iostream.Testing(context.Background()), pipelineconfig.CompileInput{ConfigYAML: []byte(configYAML)})
		invalid, ok := errors.AsType[*pipelineconfig.InvalidConfigError](err)
		assert.Assert(t, ok, "want an InvalidConfigError, got %v", err)
		assert.Check(t, cmp.DeepEqual(invalid.Diagnostics, []string{"unknown orb 'myorg/unknown@1.0.0'"}))
	})
	t.Run("a failed call is a TransportError that keeps the HTTP status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
		}))
		t.Cleanup(srv.Close)
		_, err := compiler(srv.URL).Compile(iostream.Testing(context.Background()), pipelineconfig.CompileInput{ConfigYAML: []byte(configYAML)})
		_, ok := errors.AsType[*pipelineconfig.TransportError](err)
		assert.Check(t, ok, "want a TransportError, got %v", err)
		assert.Check(t, httpcl.HasStatusCode(err, http.StatusUnauthorized), "the cause is kept: %v", err)
	})
}
