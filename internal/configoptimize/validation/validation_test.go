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

package validation_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/validation"
)

func TestParse(t *testing.T) {
	rs, err := validation.Parse([]byte(`{"note": "kept", "experiments": [
		{"id": "a", "status": "measured", "rule_version": "RC-R1-v2", "decision": "rejected", "expires": "2027-01-31",
		 "candidate": {"test": {"from": "2xlarge", "to": "xlarge", "fingerprint": "ab"}},
		 "attributable": {"test": "oom"}, "measured": {"anything": 1}},
		{"id": "b", "status": "attempted_not_triggered", "rule_version": "RC-R1-v2", "candidate": {}}]}`))
	assert.NilError(t, err)
	assert.Assert(t, cmp.Len(rs, 2))
	assert.Check(t, rs[0].Lost())
	assert.Check(t, !rs[1].Lost())
	edit, ok := rs[0].Candidate["test"]
	assert.Assert(t, ok, "no candidate edit for job %s", "test")
	assert.Check(t, cmp.Equal(edit.Fingerprint, "ab"))

	tests := []struct {
		name    string
		doc     string
		wantErr string
	}{
		{name: "no id", doc: `{"experiments": [{"rule_version": "x", "candidate": {}}]}`,
			wantErr: "experiment 0 has no id"},
		{name: "duplicate id", doc: `{"experiments": [{"id": "a", "candidate": {}}, {"id": "a", "candidate": {}}]}`,
			wantErr: "experiment id a appears twice"},
		{name: "bad date", doc: `{"experiments": [{"id": "a", "expires": "31/01/2027", "candidate": {}}]}`,
			wantErr: `expires "31/01/2027" is not a YYYY-MM-DD date`},
		{name: "bad measured_at", doc: `{"experiments": [{"id": "a", "measured_at": "yesterday", "candidate": {}}]}`,
			wantErr: `measured_at "yesterday" is not an RFC 3339 time or a YYYY-MM-DD date`},
		{name: "bad algorithm", doc: `{"experiments": [{"id": "a", "algorithm_version": "2.x", "candidate": {}}]}`,
			wantErr: `algorithm_version "2.x" is not MAJOR.MINOR.PATCH`},
		{name: "stray job", doc: `{"experiments": [{"id": "a", "candidate": {}, "attributable": {"lint": "oom"}}]}`,
			wantErr: "attributable job lint (oom) is not in the candidate"},
		{name: "string edits", doc: `{"experiments": [{"id": "a", "candidate": {"test": "2xlarge → xlarge"}}]}`,
			wantErr: "cannot unmarshal string"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validation.Parse([]byte(tc.doc))
			assert.Check(t, cmp.ErrorContains(err, tc.wantErr))
		})
	}
}

func TestLive(t *testing.T) {
	day := func(t *testing.T, s string) time.Time {
		t.Helper()
		d, err := time.Parse(time.DateOnly, s)
		assert.NilError(t, err)
		return d
	}
	r := validation.Record{Status: validation.StatusMeasured, MeasuredAt: "2026-09-30T18:00:00Z", Expires: "2027-01-31"}
	assert.Check(t, r.Live(day(t, "2027-01-31").Add(23*time.Hour)), "the whole expiry day is live")
	assert.Check(t, !r.Live(day(t, "2027-02-01")))
	assert.Check(t, r.Live(day(t, "2026-09-30")), "the measurement day itself is live, whatever the hour")
	assert.Check(t, !r.Live(day(t, "2026-09-29")), "a replay as of an earlier day does not see the record")
	noDate := r
	noDate.MeasuredAt = ""
	assert.Check(t, !noDate.Live(day(t, "2026-10-01")), "a record without measured_at never suppresses")
	assert.Check(t, !validation.Record{Status: validation.StatusMeasured, MeasuredAt: "2026-01-01"}.Live(day(t, "2026-01-01")), "a record without an expiry never suppresses")
	assert.Check(t, !validation.Record{Status: "observational", MeasuredAt: "2026-01-01", Expires: "2027-01-31"}.Live(day(t, "2026-01-01")), "only measured records suppress")
}

func TestAlgorithm(t *testing.T) {
	for _, tc := range []struct {
		r            validation.Record
		major, minor int
		ok           bool
	}{
		{validation.Record{AlgorithmVersion: "2.6.0", Rule: "RC-R1-v2"}, 2, 6, true},
		{validation.Record{AlgorithmMajor: 2}, 2, -1, true},
		{validation.Record{Rule: "RC-R0-v1"}, 1, -1, true},
		{validation.Record{Rule: "RC-R1-v1"}, 2, -1, true},
		{validation.Record{Rule: "other"}, 0, -1, false},
	} {
		major, minor, ok := tc.r.Algorithm()
		assert.Check(t, cmp.DeepEqual([]any{major, minor, ok}, []any{tc.major, tc.minor, tc.ok}), "%+v", tc.r)
	}
}

func TestSelect(t *testing.T) {
	day := func(t *testing.T, s string) time.Time {
		t.Helper()
		d, err := time.Parse(time.DateOnly, s)
		assert.NilError(t, err)
		return d
	}
	live := validation.Record{ID: "live", Status: validation.StatusMeasured, MeasuredAt: "2026-09-30", Expires: "2026-12-29", PricingSHA256: "a"}
	other := live
	other.ID, other.PricingSHA256 = "other", "b"
	expired := live
	expired.ID, expired.Expires = "expired", "2026-09-30"
	got := validation.Select([]validation.Record{live, other, expired}, day(t, "2026-10-01"), "a")
	assert.Assert(t, cmp.Len(got, 2))
	assert.Check(t, !got[0].PricedElsewhere)
	assert.Check(t, got[1].PricedElsewhere, "priced with another table")
}

func TestLoadAll(t *testing.T) {
	dir := t.TempDir()
	write := func(t *testing.T, name, id string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		assert.NilError(t, os.WriteFile(p, []byte(fmt.Sprintf(`{"experiments": [{"id": %q, "candidate": {}}]}`, id)), 0o600))
		return p
	}
	a, b, dup := write(t, "a.json", "x"), write(t, "b.json", "y"), write(t, "c.json", "x")
	rs, err := validation.LoadAll([]string{a, b})
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(rs, 2))
	_, err = validation.LoadAll([]string{a, dup})
	assert.Check(t, cmp.ErrorContains(err, "experiment id x is in both"))
}

func TestLoadAllLedger(t *testing.T) {
	dir := t.TempDir()
	write := func(t *testing.T, rel, body string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		assert.NilError(t, os.MkdirAll(filepath.Dir(p), 0o700))
		assert.NilError(t, os.WriteFile(p, []byte(body), 0o600))
	}
	measurement := func(id, extra string) string {
		return `{"schema": "config-optimize.measurement/1", "id": "` + id + `", "type": "measurement", "candidate_id": "c1",
			"status": "measured", "decision": "more_expensive", "measured_at": "2026-10-01", "expires": "2026-12-29",
			"applicability": "applicable", "edits": {"test": {"from": "2xlarge", "to": "xlarge", "fingerprint": "f", "fingerprint_version": "fp2"}}` + extra + `}`
	}
	write(t, "records/measurement/m0.json", measurement("m0", ""))
	write(t, "records/measurement/m1.json", measurement("m1", `, "supersedes": "m0"`))
	write(t, "records/prediction/p1.json", `{"schema": "config-optimize.prediction/1", "id": "p1", "type": "prediction", "candidate_id": "c1",
		"algorithm_version": "2.11.0", "predicted": {"jobs": {"test": {"job_saving": 0.4}}}}`)
	write(t, "records/prediction/p2.json", `{"schema": "config-optimize.prediction/1", "id": "p2", "type": "prediction", "candidate_id": "c1",
		"contestant": {"kind": "model"}}`)
	write(t, "records/run/r1.json", `{"schema": "cpa.run/1", "id": "r1", "type": "run"}`)
	write(t, "old.json", `{"experiments": [{"id": "x", "candidate": {}}]}`)

	rs, err := validation.LoadAll([]string{dir})
	assert.NilError(t, err)
	assert.Assert(t, cmp.Len(rs, 2), "m0 is superseded, the run record is skipped")
	i := slices.IndexFunc(rs, func(r validation.Record) bool { return r.ID == "m1" })
	assert.Assert(t, i >= 0, "no record %s", "m1")
	m := rs[i]

	t.Run("the measurement carries its algorithm's prediction", func(t *testing.T) {
		assert.Check(t, m.Contestantless)
		assert.Check(t, cmp.Len(m.Predictions, 1), "a model's prediction without an algorithm version is not attached")
		saving := m.SavingPredictedBy(2, 11)
		assert.Assert(t, saving != nil)
		v, ok := saving("test")
		assert.Check(t, ok, "no predicted saving for job %s", "test")
		assert.Check(t, cmp.Equal(v, 0.4))
		assert.Check(t, cmp.Nil(m.SavingPredictedBy(2, 10)))
	})

	t.Run("a glob loads only the files it matches", func(t *testing.T) {
		glob, err := validation.LoadAll([]string{filepath.Join(dir, "records", "measurement", "*.json")})
		assert.NilError(t, err)
		assert.Check(t, cmp.Len(glob, 1))
		_, err = validation.LoadAll([]string{filepath.Join(dir, "nothing", "*.json")})
		assert.Check(t, cmp.ErrorContains(err, "matches no file"))
	})

	t.Run("an invalid or unknown record is refused", func(t *testing.T) {
		write(t, "bad/records/measurement/m.json", measurement("mb", `, "applicability": "maybe"`))
		_, err := validation.LoadAll([]string{filepath.Join(dir, "bad")})
		assert.Check(t, cmp.ErrorContains(err, "unknown applicability"))
		write(t, "stray/x.json", `{"hello": 1}`)
		_, err = validation.LoadAll([]string{filepath.Join(dir, "stray")})
		assert.Check(t, cmp.ErrorContains(err, "neither a calibration file nor a ledger record"))
	})

	t.Run("only an applicable measurement suppresses", func(t *testing.T) {
		drifted := m
		drifted.Applicability = "environment_drifted"
		day, err := time.Parse(time.DateOnly, "2026-10-02")
		assert.NilError(t, err)
		assert.Check(t, m.Live(day))
		assert.Check(t, !drifted.Live(day), "only an applicable measurement suppresses")
	})
}
