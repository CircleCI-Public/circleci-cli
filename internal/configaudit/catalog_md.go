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

package configaudit

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
)

// CatalogMarkdown renders the rule catalog and the audit's reference material
// as the markdown behind `circleci help config-audit`. The rules come from
// Catalog, so the topic cannot drift from what the audit checks.
func CatalogMarkdown() string {
	var b strings.Builder

	b.WriteString(heredoc.Docf(`
		%[1]scircleci config audit%[1]s checks a pipeline config for supply-chain and secret-handling
		risks: untrusted input reaching a shell, unpinned dependencies, secrets in the file,
		and contexts handed to every branch. It runs offline and reads only the file you give
		it. With %[1]s--online%[1]s it also reads project, org and context settings, using GET requests
		only; it never changes anything in your organization.

		Orbs are not expanded, so findings inside an orb's own commands and jobs are not
		reported. Only the config in the file is audited.

		Each finding has a severity (how much it matters) and a confidence (how likely it is to
		be a real problem).

		## Rules

		The Needs column says what a rule audits: the config file, or project, org and context
		settings, which are read only with %[1]s--online%[1]s.

		| Rule | Severity | Confidence | Catches | Needs |
		|---|---|---|---|---|
	`, "`"))
	for _, r := range catalog {
		needs := "file"
		if r.Online {
			needs = "`--online`"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n",
			r.ID, r.Severity, r.Confidence, strings.ReplaceAll(r.Short, "|", `\|`), needs)
	}
	b.WriteString("\n")
	for _, r := range catalog {
		fmt.Fprintf(&b, "### %s\n\n%s\n\nFix: %s\n\n", r.ID, r.Why, r.Fix)
	}

	b.WriteString(heredoc.Docf(`
		## Suppressing a finding

		Put a comment on the flagged line, or alone on the line directly above it:

		%[1]s%[1]s%[1]syaml
		- run: curl -fsSL https://example.com/install.sh | sh # circleci-audit: ignore[curl-pipe-shell]
		# circleci-audit: ignore[unpinned-image, unpinned-orb]
		- image: cimg/node:lts
		%[1]s%[1]s%[1]s

		Separate several rule ids with commas. An id that names no rule is reported as a
		warning, and a known id that does not match the finding suppresses nothing.

		The comment also works inside a %[1]srun: |%[1]s block, where a shell comment on the line above a
		command annotates that exact command; the shell ignores it. Suppressed findings do not
		change the exit code, and are counted as suppressed in the summary.

		Settings findings from %[1]s--online%[1]s have no line to carry a comment, so a comment cannot
		suppress them. Raise %[1]s--min-severity%[1]s to leave them out.

		## Exit codes

		- 0: nothing at or above %[1]s--min-severity%[1]s (default low), or only suppressed findings.
		- 7: one or more findings at or above %[1]s--min-severity%[1]s. The report is still written in full
		  to stdout; the error goes to stderr.
		- 2: the file could not be read or parsed, or a flag was invalid.

		With %[1]s--online%[1]s, a missing token, an unresolvable project or a failed request is a warning:
		the offline rules still run, and the exit code is not affected.

		## Code scanning

		%[1]s--format sarif%[1]s writes a SARIF 2.1.0 log to stdout, which GitHub code scanning and other
		SARIF consumers can ingest. Suppressed findings are included, marked as suppressed. A
		SARIF upload needs a file path, not stdin, so write the log to a file and audit a file
		path rather than %[1]s-%[1]s so locations point at the file in your repository. The audit exits 7
		when it finds something, so keep the job going with %[1]s|| true%[1]s and let the upload step
		report the findings.

		A CircleCI job that writes the file (it assumes the circleci CLI is installed):

		%[1]s%[1]s%[1]syaml
		jobs:
		  audit-config:
		    docker:
		      - image: cimg/base:current
		    steps:
		      - checkout
		      - run:
		          name: Audit pipeline config
		          command: circleci config audit --format sarif > audit.sarif || true
		      - store_artifacts:
		          path: audit.sarif
		%[1]s%[1]s%[1]s

		Then upload %[1]saudit.sarif%[1]s with the code scanning upload step of your choice, for example the
		GitHub CLI or the code scanning REST API.
	`, "`"))
	return b.String()
}
