# Records `circleci run get` with no arguments in a TTY — the interactive
# TUI: filter the run list, pick a run, drill into workflows, jobs, and
# step output.
#
# Prep before recording:
#   - Be logged in, in a checkout whose project has recent runs — ideally one
#     with a failed job so the drill-down has something interesting to show.
#   - The key presses below are a starting script; tune the Sleeps and the
#     number of Down presses to match the data on screen when you record.
#
# The filter dialog ("/" on the run picker) is a three-tab overlay —
# Trigger, Status, Created. ←/→ switch tabs, ↑/↓ drive the active tab's
# list, space flips older/newer on Created, enter applies, esc cancels.
#
# Record with: task demo:gif -- run-get-tui

Output docs/demos/run-get-tui.gif

Require circleci

Set Shell "bash"
Set Theme "JetBrains Darcula"
Set Width 2000
Set Height 1200
Set WindowBar Colorful
Set TypingSpeed 75ms

Sleep 500ms
Type "circleci run get"
Sleep 500ms
Enter

# Wait for the run picker to load.
Sleep 4s

# Open the filter dialog and tour its three tabs.
Type "/"
Sleep 2.5s

# Trigger tab: the scope list (current branch, all branches, my runs, …).
# Step down to "all branches" so applying broadens rather than empties the
# run list for the drill-down that follows.
Down
Sleep 2s

# Status tab: filter by pipeline status. Highlight one, then return to
# "all statuses" so the applied filter keeps the list full.
Right
Sleep 2s
Down
Sleep 1.5s
Up
Sleep 1s

# Created tab: filter by age; space flips between older/newer than the
# chosen age. Return to "all dates" before applying.
Right
Sleep 2s
Down
Sleep 1.5s
Space
Sleep 1.5s
Space
Sleep 1s
Up
Sleep 1s

# Apply — the run list refreshes showing all branches.
Enter
Sleep 3s

# Browse the run list, then open a run.
Down
Sleep 1s
Down
Sleep 1s
Enter
Sleep 3s

# Drill into a workflow / job.
Down
Sleep 1s
Enter
Sleep 3s

# Page through step output.
Down
Sleep 1s
Enter
Sleep 2s
PageDown
Sleep 2s
PageDown
Sleep 2s

# Back out and quit.
Escape
Sleep 1s
Escape
Sleep 1s
Type "q"
Sleep 2s
