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

// Package pricing is the only source of credit rates in the program.
// It holds the interface only; the table lives in internal/pricing/static,
// which modules, the planner and the report may not import.
package pricing

// Provider supplies every credit rate.
type Provider interface {
	// CreditsPerMinute for a resource class on a platform and generation.
	CreditsPerMinute(platform, class, generation string) (float64, bool)
	// FlatCharge for per-run charges such as "dlc".
	FlatCharge(kind string) (float64, bool)
	// LadderBelow returns the next class down within a platform and
	// generation, since the ladder must be generation-aware.
	LadderBelow(platform, class, generation string) (string, bool)
	// Size of a resource class: vCPUs and RAM in GB. No API returns these,
	// so they are entered by hand in the table with the rates.
	Size(platform, class, generation string) (vcpus, ramGB float64, ok bool)
}

// None is a Provider with no rates, used when no table was given. Every
// lookup reports false, so modules state the rate is unknown.
type None struct{}

// CreditsPerMinute implements Provider.
func (None) CreditsPerMinute(string, string, string) (float64, bool) { return 0, false }

// FlatCharge implements Provider.
func (None) FlatCharge(string) (float64, bool) { return 0, false }

// LadderBelow implements Provider.
func (None) LadderBelow(string, string, string) (string, bool) { return "", false }

// Size implements Provider.
func (None) Size(string, string, string) (float64, float64, bool) { return 0, 0, false }
