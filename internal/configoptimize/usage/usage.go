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

// Package usage reads per-job resource usage for the resource-class
// downsize. The file is a normalized form of an export from the
// CircleCI v3 resource-usage API, which gives 15-second CPU and memory
// traces per job run. The Usage API export is not accepted: its
// MAX_CPU_UTILIZATION_PCT is capped at 100, and a rule built on it would
// have upsized jobs that were not CPU-bound.
package usage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
)

// SchemaPerContainer is the usage format whose runs list every parallel
// container. Without schema_version a file is the first format: one
// container per run, the busiest.
const SchemaPerContainer = 2

// Data is the usage of every job in the file, by compiled job name.
type Data struct {
	// SchemaVersion is 0 (absent) for the first format, or
	// SchemaPerContainer.
	SchemaVersion int `json:"schema_version,omitempty"`
	// Source says where the numbers came from, e.g. "CircleCI v3
	// resource-usage API, gh/org/repo, exported 2026-09-25".
	Source string         `json:"source"`
	Jobs   map[string]Job `json:"jobs"`
}

// Job is the recent runs of one job.
type Job struct {
	// ResourceClass the runs used, to confirm the samples describe the class
	// the config names today. Empty when no run had a resource-usage record.
	ResourceClass string `json:"resource_class"`
	Runs          []Run  `json:"runs"`
}

// Run is one job run. In the first format its samples are the busiest
// container's, in CPUPct and MemoryPct. In the per-container format they are
// in Executions, one per parallel container; a run with no executions is
// duration only (the API had no resource-usage record for it).
type Run struct {
	DurationSeconds float64     `json:"duration_seconds"`
	CPUPct          []float64   `json:"cpu_pct,omitempty"`
	MemoryPct       []float64   `json:"memory_pct,omitempty"`
	Executions      []Execution `json:"executions,omitempty"`

	// Per-container format only, all optional.

	// ResourceClass the run used, when it differs from the job's or the job
	// has none; empty for a duration-only run, whose class is unknown.
	ResourceClass string `json:"resource_class,omitempty"`
	// Outcome is the job's outcome, e.g. "succeeded". Only a successful run
	// is evidence for a prediction.
	Outcome string `json:"outcome,omitempty"`
	// Context of the run: which inputs it had.
	RunID     string `json:"run_id,omitempty"`
	Workflow  string `json:"workflow,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Revision  string `json:"revision,omitempty"`
	Trigger   string `json:"trigger,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
}

// Class is the class the run used: its own, or the job's.
func (r Run) Class(job Job) string {
	if r.ResourceClass != "" {
		return r.ResourceClass
	}
	if len(r.Containers()) == 0 {
		return "" // duration only: unknown
	}
	return job.ResourceClass
}

// Succeeded says whether the run can be evidence: its outcome is success,
// or unrecorded (the first format keeps only runs it could read).
func (r Run) Succeeded() bool {
	return r.Outcome == "" || r.Outcome == "succeeded"
}

// Execution is one parallel container's 15-second samples, as percentages of
// the class's CPU ceiling and RAM.
type Execution struct {
	Index     int       `json:"index"`
	CPUPct    []float64 `json:"cpu_pct"`
	MemoryPct []float64 `json:"memory_pct"`
}

// Containers are the run's containers that have samples: every one in the
// per-container format, the busiest in the first.
func (r Run) Containers() []Execution {
	if len(r.Executions) > 0 {
		return r.Executions
	}
	if len(r.CPUPct) > 0 {
		return []Execution{{CPUPct: r.CPUPct, MemoryPct: r.MemoryPct}}
	}
	return nil
}

// Load reads a usage file. Unknown fields are an error, so a file in another
// shape (such as a Usage API export) is refused rather than half-read.
func Load(path string) (*Data, error) {
	b, err := os.ReadFile(path) //#nosec:G304 // the user's usage file
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse reads usage JSON. A run in the per-container format must not also
// carry the first format's fields, so there is one source of truth.
func Parse(b []byte) (*Data, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var d Data
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("parse usage file: %w", err)
	}
	if d.SchemaVersion != 0 && d.SchemaVersion != SchemaPerContainer {
		return nil, fmt.Errorf("parse usage file: schema_version %d is not supported", d.SchemaVersion)
	}
	for name, j := range d.Jobs {
		for i, r := range j.Runs {
			if r.DurationSeconds <= 0 {
				return nil, fmt.Errorf("parse usage file: job %s run %d has no duration", name, i)
			}
			perContainer := d.SchemaVersion == SchemaPerContainer
			switch {
			case perContainer && (r.CPUPct != nil || r.MemoryPct != nil):
				return nil, fmt.Errorf("parse usage file: job %s run %d mixes cpu_pct with executions", name, i)
			case !perContainer && r.Executions != nil:
				return nil, fmt.Errorf("parse usage file: job %s run %d has executions, which need schema_version %d", name, i, SchemaPerContainer)
			}
			samples := slices.Concat(r.CPUPct, r.MemoryPct)
			seen := map[int]bool{}
			for _, e := range r.Executions {
				switch {
				case e.Index < 0 || seen[e.Index]:
					return nil, fmt.Errorf("parse usage file: job %s run %d has a duplicate or negative execution index %d", name, i, e.Index)
				case len(e.CPUPct) == 0 || len(e.MemoryPct) == 0:
					return nil, fmt.Errorf("parse usage file: job %s run %d execution %d has no CPU or memory samples", name, i, e.Index)
				}
				seen[e.Index] = true
				samples = slices.Concat(samples, e.CPUPct, e.MemoryPct)
			}
			if !perContainer && (r.ResourceClass != "" || r.Outcome != "" || r.RunID != "" || r.Branch != "") {
				return nil, fmt.Errorf("parse usage file: job %s run %d has per-run fields, which need schema_version %d", name, i, SchemaPerContainer)
			}
			for _, v := range samples {
				if v < 0 || v > 100 {
					return nil, fmt.Errorf("parse usage file: job %s run %d has a sample outside 0-100", name, i)
				}
			}
		}
	}
	return &d, nil
}

// PerContainer says whether runs list every parallel container.
func (d *Data) PerContainer() bool {
	return d != nil && d.SchemaVersion == SchemaPerContainer
}

// Job returns a job's usage, and whether the file has any.
func (d *Data) Job(name string) (Job, bool) {
	if d == nil {
		return Job{}, false
	}
	j, ok := d.Jobs[name]
	return j, ok && len(j.Runs) > 0
}
