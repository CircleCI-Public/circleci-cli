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

package runnerinstall_test

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/runnerinstall"
)

func TestPlatformsAndLabelsAlign(t *testing.T) {
	assert.Check(t, cmp.Equal(len(runnerinstall.Platforms()), len(runnerinstall.Labels())))
	assert.Check(t, cmp.Equal(runnerinstall.Platforms()[0], "debian"), "the first platform is the prompt default")
}

func TestParse(t *testing.T) {
	t.Run("accepts every listed platform", func(t *testing.T) {
		for _, v := range runnerinstall.Platforms() {
			got, err := runnerinstall.Parse(v)
			assert.Check(t, cmp.Nil(err), "platform %q", v)
			assert.Check(t, cmp.Equal(string(got), v))
		}
	})

	t.Run("rejects an unknown platform with exit code 2", func(t *testing.T) {
		_, err := runnerinstall.Parse("amiga")
		assert.Assert(t, err != nil)
		assert.Check(t, cmp.Equal(err.ExitCode, clierrors.ExitBadArguments))
		assert.Check(t, cmp.Contains(err.Message, `"amiga"`))
		assert.Check(t, cmp.Contains(err.Suggestions[0], "debian"))
	})
}

func TestBuild(t *testing.T) {
	const rc = "my-org/my-runner"

	for _, name := range runnerinstall.Platforms() {
		p := runnerinstall.Platform(name)
		t.Run(name, func(t *testing.T) {
			g, err := runnerinstall.Build(p, runnerinstall.Options{ResourceClass: rc})
			assert.NilError(t, err)

			assert.Check(t, cmp.Equal(g.Platform, p))
			assert.Check(t, g.Title != "")
			assert.Check(t, cmp.Contains(g.DocsURL, "https://circleci.com/docs/runner/"))
			assert.Check(t, len(g.Steps) >= 2, "an install is more than one step")
			assert.Check(t, cmp.Contains(g.JobConfig, "resource_class: "+rc))

			for _, s := range g.Steps {
				assert.Check(t, s.Title != "", "every step is titled")
				assert.Check(t, len(s.Commands) > 0 || s.Note != "", "step %q has nothing to do", s.Title)
			}
		})
	}

	t.Run("no command embeds a token value", func(t *testing.T) {
		for _, name := range runnerinstall.Platforms() {
			g, err := runnerinstall.Build(runnerinstall.Platform(name), runnerinstall.Options{ResourceClass: rc})
			assert.NilError(t, err)
			for _, s := range g.Steps {
				for _, c := range s.Commands {
					if strings.Contains(c, "auth_token") || strings.Contains(c, "AUTH_TOKEN") {
						assert.Check(t, cmp.Contains(c, runnerinstall.TokenEnvVar), "%s: %q", name, c)
					}
				}
			}
		}
	})

	t.Run("container runner uses a docker executor and machine runners use machine", func(t *testing.T) {
		k8s, err := runnerinstall.Build(runnerinstall.Kubernetes, runnerinstall.Options{ResourceClass: rc})
		assert.NilError(t, err)
		assert.Check(t, cmp.Contains(k8s.JobConfig, "docker:"))
		assert.Check(t, !strings.Contains(k8s.JobConfig, "machine: true"))

		deb, err := runnerinstall.Build(runnerinstall.Debian, runnerinstall.Options{ResourceClass: rc})
		assert.NilError(t, err)
		assert.Check(t, cmp.Contains(deb.JobConfig, "machine: true"))
	})

	t.Run("kubernetes values come from runner config for the given resource class", func(t *testing.T) {
		g, err := runnerinstall.Build(runnerinstall.Kubernetes, runnerinstall.Options{ResourceClass: rc})
		assert.NilError(t, err)
		var found bool
		for _, s := range g.Steps {
			for _, c := range s.Commands {
				if strings.HasPrefix(c, "circleci runner config "+rc+" --product container") {
					found = true
				}
			}
		}
		assert.Check(t, found, "expected a `circleci runner config` command for %s", rc)
	})

	t.Run("an unknown platform is an error", func(t *testing.T) {
		_, err := runnerinstall.Build("amiga", runnerinstall.Options{ResourceClass: rc})
		assert.Check(t, cmp.ErrorContains(err, "amiga"))
	})
}
