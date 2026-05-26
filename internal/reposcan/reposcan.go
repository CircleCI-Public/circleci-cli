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

// Package reposcan detects the language stack, container image, and setup
// commands for a local repository. It backs the `circleci repo scan` command
// and is also reusable by other commands that need detection.
package reposcan

// StackUnknown is the sentinel value used by scan backends when no supported
// stack is detected.
const StackUnknown = "unknown"

// SetupStep is a single named provisioning command (e.g. "install", "test",
// "system") that the scan determined should run before the user's tests.
type SetupStep struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

// Result is the outcome of a repo scan. It is intentionally decoupled from
// any specific scan backend so the CLI can render and reason about it without
// importing the backend's types.
type Result struct {
	Stack        string      `json:"stack"`
	Image        string      `json:"image"`
	ImageVersion string      `json:"image_version"`
	Setup        []SetupStep `json:"setup"`
}

// IsEmpty reports whether the scan failed to identify a supported stack.
// A nil receiver counts as empty so callers can render the fallback path
// without an extra nil-guard.
func (r *Result) IsEmpty() bool {
	if r == nil {
		return true
	}
	return r.Stack == "" || r.Stack == StackUnknown
}

// SetupCommand returns the command for the named setup step, or an empty
// string if the scan did not produce that step.
func (r *Result) SetupCommand(name string) string {
	if r == nil {
		return ""
	}
	for _, step := range r.Setup {
		if step.Name == name {
			return step.Command
		}
	}
	return ""
}
