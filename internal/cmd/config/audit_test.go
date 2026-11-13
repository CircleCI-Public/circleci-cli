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

package cmdconfig

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configaudit"
)

// jsonTagNames collects the json tag name of every field reachable from t,
// skipping fields tagged json:"-" and descending through pointers, slices and
// embedded structs.
func jsonTagNames(t reflect.Type, seen map[reflect.Type]bool, names map[string]bool) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return
	}
	seen[t] = true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag, ok := f.Tag.Lookup("json")
		name, _, _ := strings.Cut(tag, ",")
		switch {
		case name == "-":
			continue
		case !ok && f.Anonymous:
			// An embedded struct without a tag is flattened into its parent.
		case name != "":
			names[name] = true
		default:
			continue
		}
		jsonTagNames(f.Type, seen, names)
	}
}

func TestAuditLongListsEveryJSONField(t *testing.T) {
	long := newAuditCmd().Long
	names := map[string]bool{}
	jsonTagNames(reflect.TypeOf(configaudit.Report{}), map[reflect.Type]bool{}, names)
	assert.Assert(t, cmp.Contains(names, "findings"), "the walk found the report fields")

	for name := range names {
		word := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
		assert.Check(t, word.MatchString(long), "json field %q is missing from the --help JSON field list", name)
	}
}
