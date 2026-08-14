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

package apiclient

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// --- V3 wire types ---

type jobResourceUsageWire struct {
	ID         uuid.UUID `json:"id"`
	Attributes struct {
		ResourceClass      JobResourceClass            `json:"resource_class"`
		ParallelExecutions []JobResourceUsageExecution `json:"parallel_executions"`
	} `json:"attributes"`
}

// --- V3 domain types ---

// JobResourceUsage is a job's sampled CPU and memory usage, one series per
// parallel execution.
type JobResourceUsage struct {
	ID            uuid.UUID                   `json:"id"`
	ResourceClass JobResourceClass            `json:"resource_class"`
	Executions    []JobResourceUsageExecution `json:"executions"`
}

// JobResourceClass is the executor a job ran on, and the limits its usage should
// be read against.
type JobResourceClass struct {
	Name string `json:"name"`
	// CPUCount is how many cores the resource class provides, so a CPUCores
	// sample equal to it means the job was saturating the executor.
	CPUCount float64 `json:"cpu_count"`
	// MemoryLimitBytes is the executor's memory ceiling.
	MemoryLimitBytes int64 `json:"memory_limit_bytes"`
}

// JobResourceUsageExecution is one parallel execution's usage series. CPUCores
// and MemoryBytes are sampled every IntervalMS milliseconds from the start of
// the execution, so index i is the sample at i*IntervalMS elapsed; both are
// empty for an execution too short to have been sampled at all.
//
// The network counters are cumulative totals for the whole execution rather than
// series, which is why they are scalars here.
type JobResourceUsageExecution struct {
	Index          int       `json:"execution"`
	IntervalMS     int       `json:"interval_ms"`
	CPUCores       []float64 `json:"cpu_cores"`
	MemoryBytes    []int64   `json:"memory_bytes"`
	NetworkRxBytes int64     `json:"network_rx_bytes"`
	NetworkTxBytes int64     `json:"network_tx_bytes"`
}

// Interval is the sampling period as a duration. It is zero when the API
// reported no interval, which callers should read as "sample spacing unknown"
// and not as "no time passed".
func (e JobResourceUsageExecution) Interval() time.Duration {
	return time.Duration(e.IntervalMS) * time.Millisecond
}

// Samples is how many samples the execution recorded. The CPU and memory series
// are sampled together, so the shorter of the two bounds what can be plotted
// side by side.
func (e JobResourceUsageExecution) Samples() int {
	return min(len(e.CPUCores), len(e.MemoryBytes))
}

// Duration is the time span the samples cover: one interval per sample, so a
// single sample covers one interval rather than none.
func (e JobResourceUsageExecution) Duration() time.Duration {
	return time.Duration(e.Samples()) * e.Interval()
}

// GetJobResourceUsage fetches a job's sampled resource usage by UUID. The
// endpoint 404s for a job that never ran an executor — an approval job, or one
// that was cancelled before it started — so a not-found error here does not
// necessarily mean the job itself is missing.
func (c *Client) GetJobResourceUsage(ctx context.Context, id uuid.UUID) (*JobResourceUsage, error) {
	var env v3Entity[jobResourceUsageWire]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/jobs/%s/resource-usage",
		httpcl.RouteParams(id),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}
	return env.Data.toJobResourceUsage(), nil
}

func (w jobResourceUsageWire) toJobResourceUsage() *JobResourceUsage {
	return &JobResourceUsage{
		ID:            w.ID,
		ResourceClass: w.Attributes.ResourceClass,
		Executions:    w.Attributes.ParallelExecutions,
	}
}
