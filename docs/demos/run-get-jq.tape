# Records `circleci run get --json` and the built-in `--jq` filter —
# the scripting/composability story.
#
# Prep before recording:
#   - Be logged in, in a checkout whose project has recent runs — ideally one
#     with a failed job so the select() filter returns something.
#
# Record with: task demo:gif -- run-get-jq

Output docs/demos/run-get-jq.gif

Require circleci

Set Shell "bash"
Set Theme "JetBrains Darcula"
Set Width 2000
Set Height 1200
Set WindowBar Colorful
Set TypingSpeed 75ms

Env CIRCLE_NO_PAGER "1"

Sleep 500ms

# Full JSON document, colorized.
Type "circleci run get --json"
Sleep 500ms
Enter
Sleep 5s

# Built-in jq: pull the outcome of every job.
Type "circleci run get --json --jq '.workflows[].jobs[] | {name, outcome}'"
Sleep 500ms
Enter
Sleep 4s

# Just the names of the jobs that didn't succeed.
Type `circleci run get --json --jq '.workflows[].jobs[] | select(.outcome != "success").name'`
Sleep 500ms
Enter
Sleep 6s
