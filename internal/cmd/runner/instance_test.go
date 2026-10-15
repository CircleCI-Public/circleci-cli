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

package runner

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestInstanceStatus(t *testing.T) {
	ago := func(d time.Duration) time.Time { return time.Now().Add(-d) }

	tests := []struct {
		name          string
		lastConnected time.Time
		want          string
	}{
		{"online when connected under 2 minutes ago", ago(1 * time.Minute), "online"},
		{"online at zero age", time.Now(), "online"},
		{"idle when connected 2 to 30 minutes ago", ago(10 * time.Minute), "idle"},
		{"offline when connected over 30 minutes ago", ago(45 * time.Minute), "offline"},
		{"just past 2 minutes is idle, not online", ago(2*time.Minute + time.Second), "idle"},
		{"just past 30 minutes is offline, not idle", ago(30*time.Minute + time.Second), "offline"},
		// An absent timestamp must not read as decades offline.
		{"unknown when the timestamp is absent", time.Time{}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(instanceStatus(tt.lastConnected), tt.want))
		})
	}
}

func TestInstancePositionalTarget(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{
			name: "bare word is a namespace",
			arg:  "my-org",
			want: "namespace",
		},
		{
			name: "namespace/name is a resource class",
			arg:  "my-org/my-runner",
			want: "resource-class",
		},
		{
			name: "uuid is a resource class",
			arg:  "11111111-1111-4111-8111-111111111111",
			want: "resource-class",
		},
		{
			name: "trailing slash is left to resource class validation",
			arg:  "my-org/",
			want: "resource-class",
		},
		{
			name: "leading slash is left to resource class validation",
			arg:  "/my-runner",
			want: "resource-class",
		},
		{
			name: "truncated uuid is a namespace",
			arg:  "11111111-1111-4111-8111",
			want: "namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(instancePositionalTarget(tt.arg), tt.want))
		})
	}
}

func TestOtherScopeFlags(t *testing.T) {
	tests := []struct {
		target string
		want   []string
	}{
		{
			target: "namespace",
			want:   []string{"org", "resource-class"},
		},
		{
			target: "resource-class",
			want:   []string{"org", "namespace"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			assert.Check(t, cmp.DeepEqual(otherScopeFlags(tt.target), tt.want))
		})
	}
}
