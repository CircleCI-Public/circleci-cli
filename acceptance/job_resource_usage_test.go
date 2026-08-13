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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// gib is a mebibyte-friendly shorthand for the byte counts in this file's
// fixtures.
const mib = 1024 * 1024

// singleExecutionUsage is a job that ran on one executor for four samples,
// ramping up and easing off — enough shape to be visible in a chart, and a CPU
// peak that touches the resource class limit so peak-of-limit reads 100%.
func singleExecutionUsage() fakes.ResourceUsage {
	return fakes.ResourceUsage{
		ClassName:        "large",
		CPUCount:         4,
		MemoryLimitBytes: 8 * 1024 * mib,
		Executions: []fakes.ResourceUsageExecution{{
			Index:          0,
			IntervalMS:     15000,
			CPUCores:       []float64{0.5, 2.25, 4, 1.75},
			MemoryBytes:    []int64{256 * mib, 1024 * mib, 2048 * mib, 1536 * mib},
			NetworkRxBytes: 350 * mib,
			NetworkTxBytes: 2 * mib,
		}},
	}
}

// parallelUsage is the same job at parallelism 3, with the executions running for
// different lengths of time so the shared x domain is exercised: execution 1 stops
// two samples before the others.
func parallelUsage() fakes.ResourceUsage {
	u := singleExecutionUsage()
	u.Executions = []fakes.ResourceUsageExecution{
		{
			Index: 0, IntervalMS: 15000,
			CPUCores:       []float64{0.5, 2.25, 4, 1.75},
			MemoryBytes:    []int64{256 * mib, 1024 * mib, 2048 * mib, 1536 * mib},
			NetworkRxBytes: 350 * mib, NetworkTxBytes: 2 * mib,
		},
		{
			Index: 1, IntervalMS: 15000,
			CPUCores:       []float64{0.25, 1.5},
			MemoryBytes:    []int64{200 * mib, 900 * mib},
			NetworkRxBytes: 340 * mib, NetworkTxBytes: 1 * mib,
		},
		{
			Index: 2, IntervalMS: 15000,
			CPUCores:       []float64{1, 3, 2.5, 2},
			MemoryBytes:    []int64{300 * mib, 1500 * mib, 1800 * mib, 1700 * mib},
			NetworkRxBytes: 360 * mib, NetworkTxBytes: 3 * mib,
		},
	}
	return u
}

func TestJobResourceUsageGet(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddJobResourceUsage(testJobID, singleExecutionUsage())

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "resource-usage", "get", testJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobResourceUsageGet_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddJobResourceUsage(testJobID, singleExecutionUsage())

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "resource-usage", "get", testJobID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestJobResourceUsageGet_ParallelSeparate covers the default layout for a
// parallel job in a pipe: with no color to tell overlaid lines apart, "auto"
// resolves to a chart per execution.
func TestJobResourceUsageGet_ParallelSeparate(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddJobResourceUsage(testJobID, parallelUsage())

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "resource-usage", "get", testJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestJobResourceUsageGet_ParallelCombined covers --chart combined, which
// overlays the executions on one chart per metric even with no color available to
// distinguish them — the layout "auto" picks in a terminal.
func TestJobResourceUsageGet_ParallelCombined(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddJobResourceUsage(testJobID, parallelUsage())

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "resource-usage", "get", testJobID, "--chart", "combined"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobResourceUsageGet_Execution(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddJobResourceUsage(testJobID, parallelUsage())

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "resource-usage", "get", testJobID, "--execution", "2"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobResourceUsageGet_ExecutionNotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddJobResourceUsage(testJobID, singleExecutionUsage())

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "resource-usage", "get", testJobID, "--execution", "7"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestJobResourceUsageGet_NoSamples covers an execution shorter than the sampling
// interval: it recorded nothing, which the report has to say rather than drawing
// an empty chart or reporting zeroes.
func TestJobResourceUsageGet_NoSamples(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	usage := singleExecutionUsage()
	usage.Executions[0].CPUCores = nil
	usage.Executions[0].MemoryBytes = nil
	fake.AddJobResourceUsage(testJobID, usage)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "resource-usage", "get", testJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestJobResourceUsageGet_NotFound covers a job that never ran an executor — an
// approval job, or one cancelled before it started. The endpoint 404s, which must
// not read as "no such job".
func TestJobResourceUsageGet_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "resource-usage", "get", testJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobResourceUsageGet_BadChartMode(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "resource-usage", "get", testJobID, "--chart", "bananas"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}
