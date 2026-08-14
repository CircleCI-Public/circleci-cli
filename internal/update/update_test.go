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

package update

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
)

// fakeSource is a test double for Source that records how often it is called.
type fakeSource struct {
	info  *ReleaseInfo
	err   error
	calls int
}

func (s *fakeSource) Latest(context.Context) (*ReleaseInfo, error) {
	s.calls++
	return s.info, s.err
}

func testCtx() context.Context {
	return iostream.Testing(context.Background())
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		latest  string
		current string
		want    bool
	}{
		{"latest newer", "1.3.0", "1.2.0", true},
		{"current prerelease of latest", "1.3.0", "1.3.0-rc.1", true},
		{"built from source ahead of tag", "1.2.3", "1.2.3-14-gabcdef0", false},
		{"built from source ahead of prerelease tag", "1.2.3", "1.2.4-14-gabcdef0", false},
		{"equal", "1.2.0", "1.2.0", false},
		{"older", "1.2.0", "1.3.0", false},
		{"unparseable current", "1.2.0", "not-a-version", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(IsNewer(tt.latest, tt.current), tt.want))
		})
	}
}

// TestSingleSourceAcrossChannels locks in Decision 5: one version source for
// every channel, including @next prerelease users.
func TestSingleSourceAcrossChannels(t *testing.T) {
	// A @next user on an rc is told about the stable release above it...
	assert.Check(t, cmp.Equal(IsNewer("1.3.0", "1.3.0-rc.1"), true))
	// ...and told nothing when stable is below their rc.
	assert.Check(t, cmp.Equal(IsNewer("1.2.0", "1.3.0-rc.1"), false))
}

func TestNormalizeBuildVersion(t *testing.T) {
	assert.Check(t, cmp.Equal(normalizeBuildVersion("1.2.3-14-gabcdef0"), "1.2.4-pre.0"))
	assert.Check(t, cmp.Equal(normalizeBuildVersion("v1.2.3-14-gabcdef0"), "1.2.4-pre.0"))
	assert.Check(t, cmp.Equal(normalizeBuildVersion("1.2.3"), "1.2.3"))
	assert.Check(t, cmp.Equal(normalizeBuildVersion("1.2.3-rc.1"), "1.2.3-rc.1"))
}

func statePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "state.yml")
}

func seedState(t *testing.T, path string, checkedAt time.Time, rel config.Release) {
	t.Helper()
	assert.NilError(t, config.SaveState(testCtx(), path, func(s *config.State) error {
		s.SetCheckedForUpdateAt(checkedAt)
		s.SetLatestRelease(rel)
		return nil
	}))
}

func loadState(t *testing.T, path string) *config.State {
	t.Helper()
	st, err := config.LoadState(testCtx(), path)
	assert.NilError(t, err)
	return st
}

func TestCheck_CacheFresh_NoFetch(t *testing.T) {
	path := statePath(t)
	seedState(t, path, time.Now().Add(-time.Hour),
		config.Release{Version: "1.3.0", PublishedAt: time.Now().Add(-48 * time.Hour)})
	src := &fakeSource{}

	rel, err := Check(testCtx(), src, path, "1.2.0")
	assert.NilError(t, err)
	assert.Assert(t, rel != nil)
	assert.Check(t, cmp.Equal(rel.Version, "1.3.0"))
	assert.Check(t, cmp.Equal(src.calls, 0)) // fresh cache: no network
}

func TestCheck_CacheStale_Fetches(t *testing.T) {
	path := statePath(t)
	seedState(t, path, time.Now().Add(-100*time.Hour), config.Release{})
	src := &fakeSource{info: &ReleaseInfo{Version: "1.3.0", PublishedAt: time.Now().Add(-48 * time.Hour)}}

	rel, err := Check(testCtx(), src, path, "1.2.0")
	assert.NilError(t, err)
	assert.Assert(t, rel != nil)
	assert.Check(t, cmp.Equal(src.calls, 1))

	// State was refreshed with the fetched release and a recent timestamp.
	got := loadState(t, path)
	assert.Check(t, cmp.Equal(got.LatestRelease().Version, "1.3.0"))
	assert.Check(t, time.Since(got.CheckedForUpdateAt()) < time.Minute)
}

func TestCheck_FailedFetch_DoesNotWriteState(t *testing.T) {
	path := statePath(t)
	src := &fakeSource{err: errors.New("boom")}

	rel, err := Check(testCtx(), src, path, "1.2.0")
	assert.NilError(t, err) // errors are swallowed
	assert.Check(t, cmp.Nil(rel))
	assert.Check(t, cmp.Equal(src.calls, 1))

	// No state written, so the next run retries rather than caching a failure.
	got := loadState(t, path)
	assert.Check(t, got.CheckedForUpdateAt().IsZero())
}

func TestCheck_DelaySuppressed(t *testing.T) {
	path := statePath(t)
	// Fresh cache, release published only an hour ago → within notifyDelay.
	seedState(t, path, time.Now().Add(-time.Hour),
		config.Release{Version: "1.3.0", PublishedAt: time.Now().Add(-time.Hour)})
	src := &fakeSource{}

	rel, err := Check(testCtx(), src, path, "1.2.0")
	assert.NilError(t, err)
	assert.Check(t, cmp.Nil(rel))
	assert.Check(t, cmp.Equal(src.calls, 0))
}

func TestCheck_DelayElapsed_FromCacheNoFetch(t *testing.T) {
	path := statePath(t)
	// Fresh cache (no fetch), release published 12h ago. 12h is past the delay,
	// so the notice appears from cached state without a second network call —
	// and, being under a day, this also guards against notifyDelay regressing
	// back toward (or above) our ~daily release cadence, which would perpetually
	// suppress the notice.
	seedState(t, path, time.Now().Add(-time.Hour),
		config.Release{Version: "1.3.0", PublishedAt: time.Now().Add(-12 * time.Hour)})
	src := &fakeSource{}

	rel, err := Check(testCtx(), src, path, "1.2.0")
	assert.NilError(t, err)
	assert.Assert(t, rel != nil)
	assert.Check(t, cmp.Equal(rel.Version, "1.3.0"))
	assert.Check(t, cmp.Equal(src.calls, 0))
}

func TestCheck_ZeroPublishedAt_Notifies(t *testing.T) {
	path := statePath(t)
	seedState(t, path, time.Now().Add(-time.Hour),
		config.Release{Version: "1.3.0"}) // PublishedAt zero → fail open
	src := &fakeSource{}

	rel, err := Check(testCtx(), src, path, "1.2.0")
	assert.NilError(t, err)
	assert.Assert(t, rel != nil)
	assert.Check(t, cmp.Equal(rel.Version, "1.3.0"))
}

func TestCheck_HeldLock_ReturnsNil(t *testing.T) {
	t.Parallel()
	path := statePath(t)

	// Pre-acquire the advisory lock so Check cannot take it.
	fl := flock.New(config.LockPath(path))
	locked, err := fl.TryLock()
	assert.NilError(t, err)
	assert.Assert(t, locked)
	t.Cleanup(func() { _ = fl.Unlock() })

	ctx, cancel := context.WithTimeout(testCtx(), 200*time.Millisecond)
	defer cancel()

	src := &fakeSource{info: &ReleaseInfo{Version: "1.3.0"}}
	rel, err := Check(ctx, src, path, "1.2.0")
	assert.NilError(t, err)
	assert.Check(t, cmp.Nil(rel))
	assert.Check(t, cmp.Equal(src.calls, 0)) // never got far enough to fetch
}

func TestCheck_ConcurrentCallsKeepStateParseable(t *testing.T) {
	t.Parallel()
	path := statePath(t)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			src := &fakeSource{info: &ReleaseInfo{Version: "1.3.0", PublishedAt: time.Now().Add(-48 * time.Hour)}}
			_, _ = Check(testCtx(), src, path, "1.2.0")
		}()
	}
	wg.Wait()

	// After concurrent writes the state file is still valid YAML.
	got := loadState(t, path)
	assert.Check(t, cmp.Equal(got.LatestRelease().Version, "1.3.0"))
}
