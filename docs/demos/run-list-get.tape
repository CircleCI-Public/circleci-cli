# Records `circleci run list`, `circleci run get <run-id>`, then drills into
# one of the workflow's jobs with `circleci job get <job-id>` — the
# markdown-report (non-TUI) path.
#
# Prep before recording:
#   - Be logged in, in a checkout whose project has recent runs (the project
#     and branch are inferred from the git remote).
#
# The id of the latest succeeded run (and of one of its jobs) is captured
# off-camera into $RUN_ID/$JOB_ID by the first Hide/Show block below. The
# variables are also typed and expanded off-camera — Escape Ctrl+E is bash's
# shell-expand-line (bound to ESC Ctrl-E) — so on camera the real UUIDs
# simply appear after the typed command prefixes, as if pasted; the variable
# names are never visible.
#
# Record with: task demo:gif -- run-list-get

Output docs/demos/run-list-get.gif

Require circleci

Set Shell "bash"
Set Theme "JetBrains Darcula"
Set Width 2000
Set Height 1200
Set WindowBar Colorful
Set TypingSpeed 75ms

Env CIRCLE_NO_PAGER "1"

# Capture the latest succeeded run's id, and the id of one of its jobs,
# off-camera ("not run" pipelines make for an empty-looking report).
Hide
Type `RUN_ID=$(circleci run list --json --jq '[.[] | select(.current_outcome == "succeeded")][0].id')`
Enter
# Let each API call finish and the screen settle before capture resumes —
# without these sleeps the tail of this hidden session leaks into the gif.
Sleep 5s
Type `JOB_ID=$(circleci run get $RUN_ID --json --jq '.workflows[0].jobs[0].id')`
Enter
Sleep 5s
Type "clear"
Enter
Sleep 1s
Show

Sleep 500ms
Type "circleci run list"
Sleep 500ms
Enter
Sleep 5s

Type "circleci run get "
Sleep 500ms
Hide
Type "$RUN_ID"
Escape
Ctrl+E
Sleep 500ms
Show
Sleep 1s
Enter

# Hold on the rendered run report.
Sleep 8s

# Drill into one of the workflow's jobs.
Type "circleci job get "
Sleep 500ms
Hide
Type "$JOB_ID"
Escape
Ctrl+E
Sleep 500ms
Show
Sleep 1s
Enter

# Hold on the job report and its steps table.
Sleep 8s
