# Records `circleci setting set theme` with no value in a TTY — the
# interactive theme picker with live preview.
#
# Prep before recording:
#   - No login needed; the picker is purely local.
#   - Arrowing through the list previews each theme; Enter applies it.
#   - The current theme must be "auto" (the default) — the picker opens on
#     it, and the key script below is written to land back on it, so the
#     recording doesn't change your setting. Reset with:
#     circleci setting set theme auto
#
# Record with: task demo:gif -- setting-theme

Output docs/demos/setting-theme.gif

Require circleci

Set Shell "bash"
Set Theme "JetBrains Darcula"
Set Width 2000
Set Height 1200
Set WindowBar Colorful
Set TypingSpeed 75ms

Sleep 500ms
Type "circleci setting set theme"
Sleep 500ms
Enter

# Wait until the theme list is actually on screen — keypresses sent while
# the picker is still starting up are swallowed, which shifts the whole walk
# below. Waiting for the last list entry beats guessing with a fixed Sleep.
Wait+Screen@15s /Preview of the selected theme/
Sleep 1s

# Arrow through the themes to show the live preview: starting on
# "auto (default)", go all the way down to tokyo-night, skipping straight
# over light (3 down from the start) — it makes for a jarring frame.
Down
Sleep 1.5s  # dark
Down
Sleep 1.5s  # dracula
Down
Sleep 200ms # light — skip past it
Down
Sleep 1.5s  # notty
Down
Sleep 1.5s  # pink
Down
Sleep 2s    # tokyo-night

# …then walk all the way up to the top, skipping light again on the way…
Up
Sleep 1.5s  # pink
Up
Sleep 1.5s  # notty
Up
Sleep 200ms # light — skip past it
Up
Sleep 1.5s  # dracula
Up
Sleep 1.5s  # dark
Up
Sleep 1.5s  # auto (default)
Up
Sleep 1.5s  # ascii
Up
Sleep 2s    # ansi (top)

# …and back down to auto for the pick.
Down
Sleep 1.5s  # ascii
Down
Sleep 1.5s  # auto (default)

# Apply "auto (default)" — leaves the setting as it started.
Enter
Sleep 3s
