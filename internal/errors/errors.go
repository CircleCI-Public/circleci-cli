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

package errors

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CLIError is the structured error type used throughout the CLI.
// Every error surfaced to the user must carry all fields.
type CLIError struct {
	Code        string   // machine-readable identifier, e.g. "auth.token_missing"
	Title       string   // short label, e.g. "Authentication required"
	Message     string   // full explanation
	Suggestions []string // actionable next steps
	Ref         string   // documentation URL, may be empty
	ExitCode    int      // which exit code to use; defaults to ExitGeneralError
}

func (e *CLIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Title, e.Message)
}

// New constructs a CLIError with ExitGeneralError as the default exit code.
func New(code, title, message string) *CLIError {
	return &CLIError{
		Code:     code,
		Title:    title,
		Message:  message,
		ExitCode: ExitGeneralError,
	}
}

// WithSuggestions returns a copy of the error with suggestions appended.
func (e *CLIError) WithSuggestions(suggestions ...string) *CLIError {
	cp := *e
	cp.Suggestions = append(cp.Suggestions, suggestions...)
	return &cp
}

// WithRef returns a copy of the error with a documentation URL.
func (e *CLIError) WithRef(url string) *CLIError {
	cp := *e
	cp.Ref = url
	return &cp
}

// WithExitCode returns a copy of the error with the given exit code.
func (e *CLIError) WithExitCode(code int) *CLIError {
	cp := *e
	cp.ExitCode = code
	return &cp
}

// FormatJSON renders the error as a JSON object for stderr when --json is active.
// Consumers can detect errors by checking for the "error": true field.
func (e *CLIError) FormatJSON() string {
	v := struct {
		Error       bool     `json:"error"`
		Code        string   `json:"code"`
		Message     string   `json:"message"`
		Suggestions []string `json:"suggestions,omitempty"`
		Ref         string   `json:"ref,omitempty"`
		ExitCode    int      `json:"exit_code"`
	}{
		Error:       true,
		Code:        e.Code,
		Message:     e.Message,
		Suggestions: e.Suggestions,
		Ref:         e.Ref,
		ExitCode:    e.ExitCode,
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b) + "\n"
}

// Format renders the error for display on stderr.
// Output is intentionally multi-line so suggestions are readable.
func (e *CLIError) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "error: %s\n", e.Message)
	if len(e.Suggestions) > 0 {
		b.WriteString("\nSuggestions:\n")
		for _, s := range e.Suggestions {
			fmt.Fprintf(&b, "  • %s\n", s)
		}
	}
	if e.Ref != "" {
		fmt.Fprintf(&b, "\nReference: %s\n", e.Ref)
	}
	return b.String()
}
