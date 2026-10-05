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

package validation

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Schemas of the ledger records config-optimize reads. Other ledger
// records (run plans, runs, reports) are skipped.
const (
	MeasurementSchema = "config-optimize.measurement/1"
	PredictionSchema  = "config-optimize.prediction/1"
)

// envelope is the part every ledger record shares.
type envelope struct {
	Schema      string           `json:"schema"`
	ID          string           `json:"id"`
	Type        string           `json:"type"`
	Supersedes  string           `json:"supersedes"`
	Experiments *json.RawMessage `json:"experiments"`
}

type measurementFile struct {
	envelope
	CandidateID   string            `json:"candidate_id"`
	Status        string            `json:"status"`
	Decision      string            `json:"decision"`
	Expires       string            `json:"expires"`
	MeasuredAt    string            `json:"measured_at"`
	PricingSHA256 string            `json:"pricing_sha256"`
	Applicability string            `json:"applicability"`
	Edits         map[string]Edit   `json:"edits"`
	Attributable  map[string]string `json:"attributable"`
}

type predictionFile struct {
	envelope
	CandidateID      string `json:"candidate_id"`
	AlgorithmVersion string `json:"algorithm_version"`
	Predicted        struct {
		Jobs map[string]struct {
			JobSaving *float64 `json:"job_saving"`
		} `json:"jobs"`
	} `json:"predicted"`
}

// LoadAll reads results: calibration files ({"experiments": […]}) and ledger
// records (one per file). A path may be a file, a directory (every .json
// file under it) or a glob. An id may appear only once across them all; a
// record another one supersedes is dropped; predictions are attached to the
// measurements of the same candidate.
func LoadAll(paths []string) ([]Record, error) {
	files, err := expand(paths)
	if err != nil {
		return nil, err
	}
	var (
		all         []Record
		predictions []predictionFile
		seen        = map[string]string{}
		superseded  = map[string]bool{}
	)
	add := func(id, file string) error {
		if other, ok := seen[id]; ok {
			return fmt.Errorf("experiment id %s is in both %s and %s", id, other, file)
		}
		seen[id] = file
		return nil
	}
	for _, f := range files {
		b, err := os.ReadFile(f) //#nosec:G304 // the user's results files
		if err != nil {
			return nil, err
		}
		var env envelope
		if err := json.Unmarshal(b, &env); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		if env.Supersedes != "" {
			superseded[env.Supersedes] = true
		}
		switch {
		case env.Experiments != nil:
			rs, err := Parse(b)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f, err)
			}
			for _, r := range rs {
				if err := add(r.ID, f); err != nil {
					return nil, err
				}
			}
			all = append(all, rs...)
		case env.Schema == MeasurementSchema:
			r, err := measurement(b)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f, err)
			}
			if err := add(r.ID, f); err != nil {
				return nil, err
			}
			all = append(all, r)
		case env.Schema == PredictionSchema:
			var p predictionFile
			if err := json.Unmarshal(b, &p); err != nil {
				return nil, fmt.Errorf("%s: %w", f, err)
			}
			if err := add(p.ID, f); err != nil {
				return nil, err
			}
			predictions = append(predictions, p)
		case env.Type != "":
			// Another kind of ledger record: not results.
		default:
			return nil, fmt.Errorf("%s is neither a calibration file nor a ledger record", f)
		}
	}
	all = slices.DeleteFunc(all, func(r Record) bool { return superseded[r.ID] })
	for i := range all {
		for _, p := range predictions {
			if superseded[p.ID] || !all[i].Contestantless || p.CandidateID != all[i].CandidateID || p.AlgorithmVersion == "" {
				continue
			}
			saving := map[string]float64{}
			for job, j := range p.Predicted.Jobs {
				if j.JobSaving != nil {
					saving[job] = *j.JobSaving
				}
			}
			all[i].Predictions = append(all[i].Predictions, Prediction{AlgorithmVersion: p.AlgorithmVersion, JobSaving: saving})
		}
	}
	return all, nil
}

// measurement reads a ledger measurement record as a Record.
func measurement(b []byte) (Record, error) {
	var m measurementFile
	if err := json.Unmarshal(b, &m); err != nil {
		return Record{}, err
	}
	if m.ID == "" || m.CandidateID == "" {
		return Record{}, fmt.Errorf("a measurement needs an id and a candidate_id")
	}
	r := Record{ID: m.ID, Status: m.Status, Decision: m.Decision, Expires: m.Expires, MeasuredAt: m.MeasuredAt,
		Candidate: m.Edits, Attributable: m.Attributable, PricingSHA256: m.PricingSHA256,
		Contestantless: true, CandidateID: m.CandidateID, Applicability: m.Applicability}
	if r.Applicability == "" {
		return Record{}, fmt.Errorf("measurement %s has no applicability", m.ID)
	}
	if err := check(r); err != nil {
		return Record{}, fmt.Errorf("measurement %s: %w", m.ID, err)
	}
	return r, nil
}

// expand turns files, directories and globs into a sorted list of files.
func expand(paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		if strings.ContainsAny(p, "*?[") {
			m, err := filepath.Glob(p)
			if err != nil {
				return nil, err
			}
			if len(m) == 0 {
				return nil, fmt.Errorf("%s matches no file", p)
			}
			sub, err := expand(m)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".json") {
				out = append(out, path)
			}
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}
