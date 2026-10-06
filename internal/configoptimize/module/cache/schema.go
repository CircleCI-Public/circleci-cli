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

package cache

import "github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"

// Name is the module identifier.
const Name = "cache"

// Verdicts of the cache module. Both are report-only: no edit, no credit
// figure, impact unknown.
const (
	// VerdictCandidate: a dependency install with no cache visible in the
	// job (RuleF4). One per job, with the install steps as evidence. Ranked
	// low: whether a cache would pay needs step times and history.
	VerdictCandidate finding.Verdict = "CACHE_CANDIDATE"
	// VerdictVolatileRestore: every key a restore policy tries changes
	// between pipelines, with no stable fallback (RuleX1). One per restore
	// policy. A {{ .Revision }} key can still match on a rerun of the same
	// commit, so the claim is "cannot match across commits".
	VerdictVolatileRestore finding.Verdict = "CACHE_VOLATILE_RESTORE"
)

// Rule IDs, named in a finding's evidence. They follow the numbering of the
// Cache Fit Criteria, narrowed to what a config can prove.
const (
	// RuleF4: the job installs dependencies and has no visible cache.
	RuleF4 = "F4-static-v1"
	// RuleX1: every key the restore tries is scoped
	// (TIME, JOB, WORKFLOW, PIPELINE or REVISION); within a key the
	// narrowest token wins, across the keys the broadest; reuse is limited
	// to that scope. WORKFLOW, PIPELINE and REVISION need dependency paths.
	RuleX1 = "X1-static-v2"
)

// Reason codes. Metadata for ranking, never a disposition.
const (
	// CodeNoSaveInConfig: no save_cache in this config can satisfy the
	// restore policy.
	CodeNoSaveInConfig = "NO_SAVE_IN_CONFIG"
	// CodeKeyNotTraced: a key's per-run status could not be read from the
	// authored source, so the policy is not judged stable.
	CodeKeyNotTraced = "KEY_NOT_TRACED"
	// CodeUnknownStep: the job has a step of an unknown type that might
	// cache, which blocks CACHE_CANDIDATE.
	CodeUnknownStep = "UNKNOWN_STEP"
)
