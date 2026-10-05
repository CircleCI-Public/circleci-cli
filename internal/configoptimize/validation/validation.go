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

// Package validation reads the results of paired real runs, the calibration
// records kept for each experiment, so a candidate that real runs rejected is
// not proposed again.
package validation

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

// Decisions a record can carry, from scripts/compare-runs.py. Only the two
// that say the candidate lost suppress anything.
const (
	DecisionCheaper       = "cheaper"
	DecisionMoreExpensive = "more_expensive"
	DecisionRejected      = "rejected"
)

// Statuses and decisions a record may carry; anything else is refused.
var (
	statuses  = []string{"pending", "measured", "attempted_not_triggered", "observational"}
	decisions = []string{"", DecisionCheaper, DecisionMoreExpensive, DecisionRejected, "needs_more_pairs", "inconclusive"}
	// failures a job's own run can show; only these make it attributable.
	failures = []string{"oom", "timeout"}
)

// StatusMeasured is the only status whose result can suppress: the paired
// runs were made and compared.
const StatusMeasured = "measured"

// Applicable is the only applicability under which a ledger measurement can
// suppress; the others say the project or its environment has moved
// on, or the result needs revalidating.
const Applicable = "applicable"

var applicabilities = []string{Applicable, "historical", "environment_drifted", "source_drifted", "revalidation_required"}

// Records is a calibration file: its experiments, whatever else it notes.
type Records struct {
	Experiments []Record `json:"experiments"`
}

// Record is one experiment. Fields the matching does not use (predictions,
// measurements, notes) are left to the file and ignored here.
type Record struct {
	ID     string `json:"id"`
	Status string `json:"status,omitempty"`
	Rule   string `json:"rule_version"`
	// AlgorithmVersion is the resourceclass Version that made the
	// prediction, when known exactly; AlgorithmMajor its MAJOR when only that
	// is known. Without either, the MAJOR comes from Rule.
	AlgorithmVersion string `json:"algorithm_version,omitempty"`
	AlgorithmMajor   int    `json:"algorithm_major,omitempty"`
	Decision         string `json:"decision,omitempty"`
	// Expires is a date (YYYY-MM-DD). A record without one never suppresses.
	Expires string `json:"expires,omitempty"`
	// MeasuredAt is when the result was recorded, in UTC (RFC 3339 or a
	// YYYY-MM-DD date). A record without one never suppresses: a replay as
	// of a past day must not see evidence measured after it.
	MeasuredAt string `json:"measured_at,omitempty"`
	// Candidate is the whole set of edits the paired runs judged, by job.
	Candidate map[string]Edit `json:"candidate"`
	// Attributable names jobs whose own failure rejected the candidate
	// (out of memory, timeout), with the kind.
	Attributable map[string]string `json:"attributable,omitempty"`
	// PricingSHA256 is the pricing table the comparison priced with.
	PricingSHA256 string `json:"pricing_sha256,omitempty"`
	// Predicted is kept for the drift check: a job whose predicted saving has
	// moved since no longer matches.
	Predicted struct {
		Jobs map[string]struct {
			JobSaving *float64 `json:"job_saving"`
		} `json:"jobs"`
	} `json:"predicted"`
	// PricedElsewhere is set by the caller when the record was priced with
	// another table than this run's: its cost decision then says nothing
	// about this run, though a job's own failure still does.
	PricedElsewhere bool `json:"-"`

	// Ledger measurements only. Contestantless marks a measurement
	// record, which belongs to no contestant: any algorithm version may use
	// it, and its drift check uses Predictions, the contestants' predictions
	// of the same candidate. Applicability other than "applicable" stops it
	// suppressing anything.
	Contestantless bool         `json:"-"`
	CandidateID    string       `json:"-"`
	Applicability  string       `json:"-"`
	Predictions    []Prediction `json:"-"`
}

// Prediction is one contestant's predicted job savings for a candidate,
// from a ledger prediction record.
type Prediction struct {
	AlgorithmVersion string
	JobSaving        map[string]float64
}

// SavingPredictedBy returns the predicted job saving of the first prediction
// made by algorithm version major.minor, or nil when there is none.
func (r Record) SavingPredictedBy(major, minor int) func(job string) (float64, bool) {
	for _, p := range r.Predictions {
		var ma, mi, pa int
		if _, err := fmt.Sscanf(p.AlgorithmVersion, "%d.%d.%d", &ma, &mi, &pa); err == nil && ma == major && mi == minor {
			return func(job string) (float64, bool) {
				v, ok := p.JobSaving[job]
				return v, ok
			}
		}
	}
	return nil
}

// Edit is one job's class change, and the job it was made to.
type Edit struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Fingerprint identifies the job's compiled definition apart from its
	// class. Without one the edit matches nothing.
	Fingerprint string `json:"fingerprint,omitempty"`
	// FingerprintVersion is how Fingerprint was computed (fp1, fp2). An edit
	// matches only a fingerprint of the same version.
	FingerprintVersion string `json:"fingerprint_version,omitempty"`
}

// Lost reports whether the paired runs showed the candidate not to be
// cheaper.
func (r Record) Lost() bool {
	return r.Decision == DecisionRejected || r.Decision == DecisionMoreExpensive
}

// Algorithm is the version of the algorithm that made the record's
// prediction: its MAJOR, and its MINOR when known exactly (minor -1
// otherwise). ok is false when not even the MAJOR is known. An old rule
// label maps to a MAJOR only (RC-R0 → 1, RC-R1 → 2); a full version is never
// invented for it.
func (r Record) Algorithm() (major, minor int, ok bool) {
	if r.AlgorithmVersion != "" {
		var patch int
		if _, err := fmt.Sscanf(r.AlgorithmVersion, "%d.%d.%d", &major, &minor, &patch); err == nil {
			return major, minor, true
		}
		return 0, -1, false
	}
	if r.AlgorithmMajor > 0 {
		return r.AlgorithmMajor, -1, true
	}
	for prefix, m := range map[string]int{"RC-R0-": 1, "RC-R1-": 2} {
		if strings.HasPrefix(r.Rule, prefix) {
			return m, -1, true
		}
	}
	return 0, -1, false
}

// PredictedSaving is the job saving the record predicted for job, if known.
func (r Record) PredictedSaving(job string) (float64, bool) {
	j, ok := r.Predicted.Jobs[job]
	if !ok || j.JobSaving == nil {
		return 0, false
	}
	return *j.JobSaving, true
}

// Live reports whether the record can suppress as of a UTC calendar day: it
// was measured on or before that day, and has not expired by it. Every
// comparison is between calendar days, so the result depends on the day
// alone, never on the time of day.
func (r Record) Live(asOf time.Time) bool {
	exp, err := time.Parse(time.DateOnly, r.Expires)
	if err != nil || r.Status != StatusMeasured || (r.Applicability != "" && r.Applicability != Applicable) {
		return false
	}
	measured, ok := day(r.MeasuredAt)
	if !ok {
		return false
	}
	today := Day(asOf)
	return !measured.After(today) && !today.After(exp)
}

// Day is t's calendar day in UTC, at midnight.
func Day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// day reads an RFC 3339 time or a YYYY-MM-DD date as a UTC calendar day.
func day(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return Day(t), true
	}
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// Select keeps the records live as of the UTC day asOf, and marks
// those priced with another table than pricingSHA (this run's pricing
// table's hash, "" when none was given): such a record keeps only its jobs'
// own failures.
func Select(records []Record, asOf time.Time, pricingSHA string) []Record {
	var live []Record
	for _, r := range records {
		if r.Live(asOf) {
			r.PricedElsewhere = r.PricingSHA256 != "" && r.PricingSHA256 != pricingSHA
			live = append(live, r)
		}
	}
	return live
}

// Load reads one calibration file.
func Load(path string) ([]Record, error) {
	b, err := os.ReadFile(path) //#nosec:G304 // the user's results file
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse reads a calibration file. A record's expiry must be a date when it
// is set, so a typo cannot silently turn suppression off or on.
func Parse(b []byte) ([]Record, error) {
	var rs Records
	if err := json.Unmarshal(b, &rs); err != nil {
		return nil, fmt.Errorf("parse results file: %w", err)
	}
	seen := map[string]bool{}
	for i, r := range rs.Experiments {
		switch {
		case r.ID == "":
			return nil, fmt.Errorf("parse results file: experiment %d has no id", i)
		case seen[r.ID]:
			return nil, fmt.Errorf("parse results file: experiment id %s appears twice", r.ID)
		}
		seen[r.ID] = true
		if err := check(r); err != nil {
			return nil, fmt.Errorf("parse results file: experiment %s: %w", r.ID, err)
		}
	}
	return rs.Experiments, nil
}

// check validates the fields matching reads, so a typo cannot silently turn
// suppression off or on.
func check(r Record) error {
	if _, _, ok := r.Algorithm(); r.AlgorithmVersion != "" && !ok {
		return fmt.Errorf("algorithm_version %q is not MAJOR.MINOR.PATCH", r.AlgorithmVersion)
	}
	switch {
	case r.Status != "" && !slices.Contains(statuses, r.Status):
		return fmt.Errorf("unknown status %q", r.Status)
	case !slices.Contains(decisions, r.Decision):
		return fmt.Errorf("unknown decision %q", r.Decision)
	case r.Applicability != "" && !slices.Contains(applicabilities, r.Applicability):
		return fmt.Errorf("unknown applicability %q", r.Applicability)
	}
	if _, ok := day(r.MeasuredAt); r.MeasuredAt != "" && !ok {
		return fmt.Errorf("measured_at %q is not an RFC 3339 time or a YYYY-MM-DD date", r.MeasuredAt)
	}
	if r.Expires != "" {
		if _, err := time.Parse(time.DateOnly, r.Expires); err != nil {
			return fmt.Errorf("expires %q is not a YYYY-MM-DD date", r.Expires)
		}
	}
	for job, kind := range r.Attributable {
		if _, ok := r.Candidate[job]; !ok {
			return fmt.Errorf("attributable job %s (%s) is not in the candidate", job, kind)
		}
		if !slices.Contains(failures, kind) {
			return fmt.Errorf("job %s's failure %q is not oom or timeout", job, kind)
		}
	}
	return nil
}
