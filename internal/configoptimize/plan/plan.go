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

// Package plan is the reconcile-to-apply contract.
// Reconcile produces a Plan; apply consumes only the Plan and never reads
// findings, so reconcile's conflict and ordering rules can change
// without changing apply.
package plan

// Plan is the ordered set of changes to attempt.
type Plan struct {
	Groups []Group
}

// Group is one actionable finding, applied all or nothing.
type Group struct {
	// FindingID and Fingerprint identify the finding. Apply re-analyses the
	// config after each accepted group and looks the rest up by ID, then
	// checks the fingerprint matches.
	FindingID   string
	Fingerprint Fingerprint

	Edits []Edit
	// Expected is the change the group must make to the compiled config.
	Expected Delta
}

// Fingerprint is everything a finding ID is derived from.
type Fingerprint struct {
	Module   string
	Job      string
	Workflow string
	Path     []string
}

// Op is an edit operation.
type Op int

const (
	// Delete removes a mapping entry.
	Delete Op = iota
	// Replace sets a one-line scalar.
	Replace
	// Insert adds one key to a mapping that lacks it.
	Insert
)

// String returns the lowercase name used in reports.
func (o Op) String() string {
	switch o {
	case Delete:
		return "delete"
	case Replace:
		return "replace"
	case Insert:
		return "insert"
	}
	return "delete"
}

// Edit is one change to the authored config.
type Edit struct {
	Op Op
	// Path is the authored path, e.g. ["jobs", "build", "machine", "docker_layer_caching"].
	Path []string
	// Expect is the value the node must still hold before the edit; a
	// different value refuses the edit.
	Expect any
	// Value is the new value for Replace and Insert.
	Value any
}

// Delta is the change a group must make to the compiled config.
type Delta struct {
	// Removed are effective paths that must be gone afterwards.
	Removed [][]string
	// FalseMeansAbsent lets a removed boolean key survive as an explicit
	// false. It covers every path in Removed, so a module sets it only when
	// false is the default of all of them (dlc's paths are all
	// docker_layer_caching).
	FalseMeansAbsent bool
	// Set are effective paths that must hold a new scalar afterwards.
	Set []SetValue
}

// SetValue is one compiled path and the value it must hold after the edit.
type SetValue struct {
	Path  []string
	Value any
}
