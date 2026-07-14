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
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
)

const testToken = "test-token"

// osc8IDPattern matches the id parameter of an OSC-8 hyperlink. glamour derives
// it from an fnv hash of the full URL — which includes the random test port —
// so it must be neutralized alongside the host for stable golden files.
var osc8IDPattern = regexp.MustCompile(`8;id=\d+;`)

// normalizeAppHost replaces the fake server's app host (e.g.
// "app.127.0.0.1:54321", derived from the random test port) with a stable
// placeholder so that golden files containing browser URLs stay deterministic.
// It also neutralizes the port-dependent OSC-8 hyperlink id emitted in color
// output.
func normalizeAppHost(s, fakeURL string) string {
	u, err := url.Parse(fakeURL)
	if err == nil && u.Host != "" {
		s = strings.ReplaceAll(s, "app."+u.Host, "app.circleci.test")
	}
	return osc8IDPattern.ReplaceAllString(s, "8;id=link;")
}

// ignoreCommonHeaders excludes headers that are set uniformly by the API client
// (and therefore uninteresting to a per-command mutation assertion) so that
// request comparisons can focus on Method, URL, Authorization, User-Agent, and
// the request body.
var ignoreCommonHeaders = httprecorder.IgnoreHeaders(
	"Accept",
	"Accept-Encoding",
	"Content-Length",
	"Content-Type",
)

var (
	binaryPath     string
	testBinaryPath string
	testBinaryDir  string
)

func TestMain(m *testing.M) {
	var err error
	var cleanup, cleanup2 func()
	binaryPath, cleanup, err = binary.Build(
		"circleci",
		"..",
		"./cmd/circleci",
	)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "skipping acceptance tests: %v\n", err)
		os.Exit(0)
	}
	_, _ = fmt.Fprintf(os.Stderr, "built circleci binary: %s\n", binaryPath)

	testBinaryPath, cleanup2, err = binary.Build(
		"circleci-testextension",
		".",
		"./testdata/circleci-testextension",
	)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "skipping acceptance tests: %v\n", err)
		os.Exit(0)
	}
	_, _ = fmt.Fprintf(os.Stderr, "built testapp binary: %s\n", testBinaryPath)
	testBinaryDir = filepath.Dir(testBinaryPath)

	code := m.Run()
	cleanup()
	cleanup2()

	os.Exit(code)
}

// goldenTemplate renders templateFile with data and asserts that actual
// matches the result.
func goldenTemplate(t *testing.T, actual, templateFile string, data any) {
	t.Helper()

	f := filepath.Join("testdata", templateFile)
	tmpl, err := template.New(t.Name()).ParseFiles(f)
	assert.NilError(t, err)

	var rendered bytes.Buffer
	err = tmpl.ExecuteTemplate(&rendered, filepath.Base(f), data)
	assert.NilError(t, err)

	tmpDir := t.TempDir()
	goldenFile := filepath.Join(tmpDir, f+".txt")

	assert.NilError(t, os.MkdirAll(filepath.Dir(goldenFile), 0750))

	// windows.
	d := bytes.ReplaceAll(rendered.Bytes(), []byte("\r\n"), []byte("\n"))
	err = os.WriteFile(goldenFile, d, 0640)
	assert.NilError(t, err)

	assert.Check(t, golden.String(actual, goldenFile))
}
