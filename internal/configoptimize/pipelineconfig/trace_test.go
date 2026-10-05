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

package pipelineconfig

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// Every class of tracer message maps to a cause, so an uninspectable result
// always says which kind of unknown it is.
func TestTraceCauses(t *testing.T) {
	tests := []struct {
		name string
		want TraceCause
	}{
		{name: "pipeline parameter k can be overridden when the pipeline is triggered", want: CauseExternal},
		{name: "pipeline parameter k has no enum list", want: CauseExternal},
		{name: "job build is shared through an anchor, alias or merge key", want: CauseUnsupported},
		{name: "job build is invoked with pre-steps or post-steps, which shift step indexes", want: CauseUnsupported},
		{name: "job docker/publish comes from an orb", want: CauseUnsupported},
		{name: "<< parameters.a and \"b\" >> is a parameter expression, which the tracer does not evaluate", want: CauseUnsupported},
		{name: "<< matrix.x >> is not a value the tracer knows", want: CauseUnsupported},
		{name: "a value of << parameters.k >> is itself a reference", want: CauseUnsupported},
		{name: "no authored job produces effective job x that the tracer can see: a workflow entry uses a YAML merge key, which it does not follow", want: CauseUnsupported},
		{name: "more than one workflow invocation could produce job build", want: CauseProvenance},
		{name: "the step cannot be matched to one authored step", want: CauseProvenance},
		{name: "no authored job produces effective job x", want: CauseProvenance},
		{name: "jobs.a.steps.1: substituting the traced values does not give the compiled text", want: CauseProvenance},
		{name: "parameter k of job a has no call-site value and no default", want: CauseProvenance},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(causeOf(tc.name), tc.want))
		})
	}
	assert.Check(t, cmp.Equal(Trace{}.Cause(), CauseNone))
}
