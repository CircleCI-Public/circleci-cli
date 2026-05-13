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
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

// rawConfig is a minimal config with two deploy-like jobs and branch filters.
const rawConfig = `version: 2.1

jobs:
  build:
    docker:
      - image: cimg/base:stable
    steps:
      - checkout
      - run: make build

  deploy-staging:
    docker:
      - image: cimg/base:stable
    steps:
      - checkout
      - run: make deploy-staging

  deploy-prod:
    docker:
      - image: cimg/base:stable
    steps:
      - checkout
      - run: make deploy

workflows:
  main:
    jobs:
      - build
      - deploy-staging:
          requires:
            - build
          filters:
            branches:
              only: develop
      - deploy-prod:
          requires:
            - build
          filters:
            branches:
              only: main
`

func TestDeployInit(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, rawConfig)

	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "init", "--component", "api", "--environment", "production"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	patched, err := os.ReadFile(filepath.Join(dir, ".circleci", "config.yml"))
	assert.NilError(t, err)
	assert.Check(t, golden.Bytes(patched, t.Name()+".config.yml"))
}

func TestDeployInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, rawConfig)

	env := testenv.New(t)
	env.Token = testToken

	args := []string{"deploy", "init", "--component", "api", "--environment", "production"}
	opts := binary.RunOpts{Binary: binaryPath, Args: args, Env: env.Environ(), WorkDir: dir}

	r1 := binary.RunCLI(t, opts)
	assert.Equal(t, r1.ExitCode, 0, "first run stderr: %s", r1.Stderr)

	r2 := binary.RunCLI(t, opts)
	assert.Equal(t, r2.ExitCode, 0, "second run stderr: %s", r2.Stderr)
	assert.Check(t, golden.String(r2.Stdout, t.Name()+".txt"))
}

func TestDeployInit_NoJobs(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `version: 2.1
jobs:
  build:
    docker:
      - image: cimg/base:stable
    steps:
      - checkout
`)

	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "init", "--component", "api"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, result.ExitCode != 0)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestDeployInit_InferEnvironments(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `version: 2.1
jobs:
  deploy-staging:
    docker:
      - image: cimg/base:stable
    steps:
      - checkout
      - run: make deploy-staging
  deploy-production:
    docker:
      - image: cimg/base:stable
    steps:
      - checkout
      - run: make deploy-prod
workflows:
  main:
    jobs:
      - deploy-staging
      - deploy-production
`)

	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "init", "--component", "myapp"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	patched, err := os.ReadFile(filepath.Join(dir, ".circleci", "config.yml"))
	assert.NilError(t, err)
	assert.Check(t, golden.Bytes(patched, t.Name()+".config.yml"))
}
