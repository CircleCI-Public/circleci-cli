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

// Package data is the read-only telemetry view modules receive. v1 has no live
// telemetry: the only source will be fixtures.
//
// View carries Has only. The payload accessors land one at a time as each
// payload contract is corrected, rather than freezing a method
// set known to be wrong.
package data

// Requirement names one piece of telemetry a module can ask for.
type Requirement string

// The requirements the v1 modules declare.
const (
	ReqJobResourceUsage Requirement = "job_resource_usage"
	ReqUsageExport      Requirement = "usage_export"
	ReqCacheStats       Requirement = "cache_stats"
	ReqWorkflowFailures Requirement = "workflow_failures"
	ReqStorageUsage     Requirement = "storage_usage"
	ReqComputeAdoption  Requirement = "compute_adoption"
)

// View is the read-only handle modules receive. A missing requirement is not
// an error: modules must degrade and say what was not evaluated.
type View interface {
	Has(Requirement) bool
}

// Empty returns a View with no data, the no-telemetry mode.
func Empty() View { return empty{} }

type empty struct{}

func (empty) Has(Requirement) bool { return false }
