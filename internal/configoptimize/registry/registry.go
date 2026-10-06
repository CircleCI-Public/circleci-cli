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

// Package registry builds the ordered module catalog. There is no init()
// registration: Catalog constructs a fresh slice on every call, so tests can
// build isolated subsets without shared state.
package registry

import (
	"fmt"
	"slices"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module/resourceclass"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/reconcile"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/usage"
)

// Deps are the dependencies modules receive through their constructors.
type Deps struct {
	Pricing pricing.Provider
	// Usage is per-job resource usage for the resource-class downsize, nil
	// when none was given.
	Usage *usage.Data
}

// Catalog returns every module, freshly constructed, in catalog order: the
// order findings are reported in.
func Catalog(deps Deps) []module.Analyzer {
	return []module.Analyzer{
		resourceclass.New(deps.Pricing, deps.Usage),
	}
}

// Policy returns module policy for reconcile's feasibility stage. It is data
// kept beside the catalog so every caller applies the same policy.
func Policy() reconcile.Policy {
	return reconcile.Policy{}
}

// Select filters catalog down to names, module names from ParseChecks,
// without reordering it. An empty names list selects everything.
func Select(catalog []module.Analyzer, names []string) []module.Analyzer {
	if len(names) == 0 {
		return slices.Clone(catalog)
	}
	out := make([]module.Analyzer, 0, len(names))
	for _, m := range catalog {
		if slices.Contains(names, m.Name()) {
			out = append(out, m)
		}
	}
	return out
}

// checks maps the names config optimize's --only takes to module names.
var checks = map[string]string{"resource-class": resourceclass.Name}

// CheckNames are the names --only takes, sorted.
func CheckNames() []string {
	names := make([]string, 0, len(checks))
	for n := range checks {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// UnknownCheckError reports a --only name that is not a check.
type UnknownCheckError struct {
	Name  string
	Known []string
}

func (e *UnknownCheckError) Error() string {
	return fmt.Sprintf("%q is not a check; the checks are %s", e.Name, strings.Join(e.Known, ", "))
}

// ParseChecks turns --only names into module names for Select; none means
// every module.
func ParseChecks(only []string) ([]string, error) {
	var out []string
	for _, name := range only {
		m, ok := checks[strings.TrimSpace(name)]
		if !ok {
			return nil, &UnknownCheckError{Name: name, Known: CheckNames()}
		}
		out = append(out, m)
	}
	return out, nil
}
