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

package buildinfo

import (
	"runtime/debug"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestAgeDays(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		settings []debug.BuildSetting
		wantDays int
		wantOK   bool
	}{
		{
			name: "commit from three and a half days ago",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "0ee5af2e58d8a25799e48e17846ae3eb08957db1"},
				{Key: "vcs.time", Value: "2026-10-02T00:00:00Z"},
			},
			wantDays: 3,
			wantOK:   true,
		},
		{
			name:     "commit from earlier the same day",
			settings: []debug.BuildSetting{{Key: "vcs.time", Value: "2026-10-05T01:00:00Z"}},
			wantDays: 0,
			wantOK:   true,
		},
		{
			name:     "commit time ahead of the local clock",
			settings: []debug.BuildSetting{{Key: "vcs.time", Value: "2026-10-07T00:00:00Z"}},
			wantDays: 0,
			wantOK:   true,
		},
		{
			name:     "no commit time stamped",
			settings: []debug.BuildSetting{{Key: "GOOS", Value: "linux"}},
			wantOK:   false,
		},
		{
			name:     "unparseable commit time",
			settings: []debug.BuildSetting{{Key: "vcs.time", Value: "yesterday"}},
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days, ok := ageDays(tt.settings, now)
			assert.Check(t, cmp.Equal(ok, tt.wantOK))
			assert.Check(t, cmp.Equal(days, tt.wantDays))
		})
	}
}
