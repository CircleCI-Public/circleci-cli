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

// Package module defines the contract every analysis module implements. It
// imports no module implementation; internal/registry sits above both, so
// modules are listed in one place instead of registering themselves.
package module

import (
	"context"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// Analyzer is implemented by every module. Implementations are pure:
// no I/O, no shared mutable state, constructors do nothing on startup, safe to
// run concurrently. Dependencies such as pricing arrive through the
// constructor called by registry.Catalog.
type Analyzer interface {
	// Name is the module identifier: lowercase, no spaces, e.g. "dlc".
	Name() string

	// Evaluate inspects the effective config and returns findings. It never
	// returns edits, never consults the mutability map, and never drops a
	// finding because it looks unactionable; reconcile makes it report-only.
	Evaluate(ctx context.Context, cfg *pipelineconfig.Effective) ([]finding.Finding, error)
}

// InputReporter is implemented by a module that needs an input the run may
// not have supplied, such as a usage file. MissingInputs names each absent
// one, so a run that finds nothing can say why.
type InputReporter interface {
	MissingInputs() []string
}
