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

// Package update discovers whether a newer circleci release exists and, when so,
// hands the caller a notice to print after a successful command. It never blocks
// the command, corrupts piped output, or surfaces its own errors.
package update

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	goversion "github.com/hashicorp/go-version"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/agent"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
)

const (
	// ToolName is the value sent as filter[tool] and echoed back as the "tool"
	// attribute. It is the CLI's GitHub repository name and part of the server's
	// public contract, so it must never be derived from anything dynamic.
	ToolName = "circleci-cli"

	// releaseTagBaseURL is the GitHub release-page prefix the notice links to. The
	// tag carries a leading "v" that the endpoint's version string omits, so the
	// URL is built as releaseTagBaseURL + version. It is derived client-side from
	// the CLI's own repository rather than returned by the API, keeping the V3
	// payload free of link fields.
	releaseTagBaseURL = "https://github.com/CircleCI-Public/circleci-cli/releases/tag/v"

	// ForceEnv bypasses the TTY and dev-build gates and supplies the version to
	// treat as current. Internal test hook (double underscore); never user-set.
	ForceEnv = "__CIRCLE_UPDATE_FORCE"

	// notifyDelay is how long after publication a release stays quiet, giving the
	// package managers time to catch up. One value, every channel.
	//
	// Kept well under our (roughly daily) release cadence: a delay at or above
	// the cadence would leave the newest release perpetually inside the window,
	// so a user who is behind would never be nagged. Six hours comfortably clears
	// the observed bot-moderated propagation of the slow channels — homebrew-core
	// autobump and winget-pkgs both typically merge within ~1-6h of a release —
	// while still leaving most of each day as a nag window. A release that
	// propagates unusually slowly may be advertised a few hours early; that is
	// the accepted cost of not detecting channels (install.sh and direct-archive
	// users can fetch the latest immediately regardless).
	notifyDelay = 6 * time.Hour

	// cacheWindow is how often we are willing to hit the network. Matched to our
	// ~daily release cadence so the version we advertise never lags reality by
	// more than about a day: a longer window would keep pointing at a stale
	// "latest" (still a correct nag, just an out-of-date target) while a shorter
	// one would probe more often than a new release can appear.
	cacheWindow = 24 * time.Hour
)

// ReleaseInfo is the newest stable release the source reported.
type ReleaseInfo struct {
	Version     string    // semver, no leading "v" — matches main.version
	PublishedAt time.Time // zero when unknown; treated as "notify"
}

// Source returns the newest stable release, or nil if it cannot tell. proxySource
// (GET /api/v3/tool/releases) is the only implementation; see the package plan
// for the contingency it exists for.
type Source interface {
	Latest(ctx context.Context) (*ReleaseInfo, error)
}

// EffectiveVersion returns the version the check should treat as current: the
// ForceEnv override when set (test hook), otherwise version.
func EffectiveVersion(version string) string {
	if f := os.Getenv(ForceEnv); f != "" {
		return f
	}
	return version
}

// ShouldCheck reports whether this invocation may perform an update check.
//
// Agents and MCP cannot act on a prompt, and the notice must never corrupt a
// pipe — so a detected agent or a non-TTY stream silences it. No token is
// required: GET /api/v3/tool/releases serves anonymously, so unauthenticated
// users (fresh installs, config-validate-only) still get the notice. ForceEnv
// bypasses the TTY and dev-build gates for tests but not the agent/preference
// gates.
func ShouldCheck(ctx context.Context, cfg *config.Config, version string) bool {
	// CI always suppresses the notice — a CI job can't act on it. Checked before
	// the ForceEnv hook so it holds even in tests.
	if os.Getenv("CI") != "" {
		return false
	}
	if os.Getenv(ForceEnv) == "" {
		if version == "" || version == "dev" {
			return false
		}
		s := iostream.Get(ctx)
		if !s.IsTerminal() || !s.ErrIsTerminal() || !s.IsInteractive() {
			return false
		}
	}
	if !cfg.IsUpdateCheck() {
		return false
	}
	if agent.Detect() != "" {
		return false
	}
	return true
}

// Check returns a release worth telling the user about, or nil. It refreshes
// from src at most once per cacheWindow, but evaluates notifyDelay against cached
// state on every call — so a notice suppressed by the delay appears the moment
// the delay elapses, without waiting for the next network fetch.
//
// Errors are best-effort: a contended lock or unreadable state returns nil, nil
// rather than surfacing, because an update check is the last thing that should
// ever print an error.
func Check(ctx context.Context, src Source, statePath, currentVersion string) (*ReleaseInfo, error) {
	// Locking, directory creation and (de)serialisation all live in config's
	// LoadState / SaveState now. A contended lock or unreadable state surfaces as
	// an error there; here it is best-effort — an update check is the last thing
	// that should ever block a command or print an error, so any failure is
	// debug-logged and swallowed (this is the deliberate divergence from config,
	// which returns the lock error to its caller).
	st, err := config.LoadState(ctx, statePath)
	if err != nil {
		iostream.DebugContext(ctx, "update check: could not load state", "err", err)
		return nil, nil
	}

	stored := st.LatestRelease()
	release := ReleaseInfo{Version: stored.Version, PublishedAt: stored.PublishedAt}
	checkedAt := st.CheckedForUpdateAt()

	if checkedAt.IsZero() || time.Since(checkedAt) >= cacheWindow {
		info, ferr := src.Latest(ctx)
		if ferr != nil {
			// Transient/unexpected failure: don't write state, so the cache
			// window isn't burned and the next invocation retries.
			iostream.DebugContext(ctx, "update check: fetch failed", "err", ferr)
			return nil, nil
		}

		werr := config.SaveState(ctx, statePath, func(s *config.State) error {
			s.SetCheckedForUpdateAt(time.Now())
			if info != nil {
				s.SetLatestRelease(config.Release{Version: info.Version, PublishedAt: info.PublishedAt})
			} else {
				// Recognised-but-unactionable response: clear any stale release so
				// the fresh timestamp doesn't leave us advertising an old target.
				s.SetLatestRelease(config.Release{})
			}
			return nil
		})
		if werr != nil {
			iostream.DebugContext(ctx, "update check: could not write state", "err", werr)
			return nil, nil
		}
		if info == nil {
			// Recognised-but-unactionable (e.g. 400/401/403): state is written so
			// we back off for cacheWindow, but there is nothing to show.
			return nil, nil
		}
		release = *info
	}

	result := evaluate(release, currentVersion)
	iostream.DebugContext(ctx, "update check evaluated",
		"latest", release.Version,
		"current", currentVersion,
		"published_at", release.PublishedAt,
		"newer", isNewer(release.Version, currentVersion),
		"within_delay", !release.PublishedAt.IsZero() && time.Since(release.PublishedAt) < notifyDelay,
		"notify", result != nil,
	)
	return result, nil
}

// evaluate decides whether release is worth a notice given the current version.
func evaluate(release ReleaseInfo, currentVersion string) *ReleaseInfo {
	if release.Version == "" {
		return nil
	}
	if !isNewer(release.Version, currentVersion) {
		return nil
	}
	// Blanket delay: stay quiet until the package managers have had time to catch
	// up. published_at is always present on a 200, so the IsZero guard is only a
	// fail-open against a malformed response.
	if !release.PublishedAt.IsZero() && time.Since(release.PublishedAt) < notifyDelay {
		return nil
	}
	r := release
	return &r
}

// gitDescribeRE matches a git-describe version like "1.2.3-14-gabcdef0": a base
// semver, the number of commits since that tag, and the abbreviated commit hash.
var gitDescribeRE = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)-(\d+)-g[0-9a-f]+$`)

// normalizeBuildVersion rewrites a git-describe version so a source build made
// after a tag sorts above that tag but below the next patch: "1.2.3-14-gabc"
// becomes "1.2.4-pre.0". This keeps builds that are already ahead of the last
// tagged release from being nagged. Other version strings are returned as-is.
func normalizeBuildVersion(v string) string {
	m := gitDescribeRE.FindStringSubmatch(v)
	if m == nil {
		return v
	}
	patch, err := strconv.Atoi(m[3])
	if err != nil {
		return v
	}
	return fmt.Sprintf("%s.%s.%d-pre.0", m[1], m[2], patch+1)
}

// isNewer reports whether latest is a strictly greater semver than current,
// after normalising a git-describe current version.
func isNewer(latest, current string) bool {
	current = normalizeBuildVersion(current)
	lv, lerr := goversion.NewVersion(latest)
	cv, cerr := goversion.NewVersion(current)
	if lerr != nil || cerr != nil {
		return false
	}
	return lv.GreaterThan(cv)
}

// Notifier carries a background update check. Create one with Start, then call
// Finish once the command has produced all its output.
type Notifier struct {
	cancel context.CancelFunc
	done   chan *ReleaseInfo
	forced bool
}

// Start launches an update check in the background against src. The check runs
// with a child of ctx so Finish can cancel any in-flight request.
func Start(ctx context.Context, src Source, statePath, currentVersion string) *Notifier {
	ctx, cancel := context.WithCancel(ctx)
	n := &Notifier{
		cancel: cancel,
		done:   make(chan *ReleaseInfo, 1),
		// Under the test hook the check runs to completion rather than being
		// cancelled when the (typically instant) command finishes, so the notice
		// is deterministic instead of racing the request.
		forced: os.Getenv(ForceEnv) != "",
	}
	go func() {
		info, _ := Check(ctx, src, statePath, currentVersion)
		n.done <- info
	}()
	return n
}

// Finish returns the release worth telling the user about, or nil. It cancels
// any in-flight request first (so a slow request never adds command latency) and
// then blocks until the background check settles — bounded, because the cancel
// aborts the request.
func (n *Notifier) Finish() *ReleaseInfo {
	if n == nil {
		return nil
	}
	if !n.forced {
		n.cancel()
	}
	rel := <-n.done
	n.cancel()
	return rel
}

// PrintNotice writes the two-line update notice to stderr, blank-line padded and
// after all command output. The second line links the new release's GitHub
// release page. It is a no-op when rel is nil.
//
// The notice only prints when both Out and Err are TTYs (see ShouldCheck), so
// color is always safe here — there is no pipe to corrupt. The color helpers
// still fall back to plain text under NO_COLOR / TERM=dumb, so the message text
// is unchanged when color is disabled.
func PrintNotice(ctx context.Context, currentVersion string, rel *ReleaseInfo) {
	if rel == nil {
		return
	}
	iostream.ErrPrintf(ctx,
		"\n%s %s → %s\n%s\n\n",
		iostream.Warning(ctx, "A new version of circleci is available:"),
		iostream.Muted(ctx, currentVersion),
		iostream.Success(ctx, rel.Version),
		iostream.Muted(ctx, releaseURL(rel.Version)))
}

// releaseURL returns the GitHub release page for version, tolerating a leading
// "v" that the endpoint normally strips.
func releaseURL(version string) string {
	return releaseTagBaseURL + strings.TrimPrefix(version, "v")
}
