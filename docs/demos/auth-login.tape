# Records `circleci auth login` — the OAuth login flow.
#
# The login TUI has three stages before the browser opens (see
# acceptance/login_test.go): a host picker ("Where do you use CircleCI?"),
# a method picker ("Login with a web browser"), and a "Press Enter to open
# in your browser..." prompt. Each needs its own Enter; the browser only
# opens on the last one.
#
# Prep before recording:
#   - Run `circleci auth logout` first, or the command short-circuits with
#     "already logged in" and there's nothing to show.
#   - The OAuth handoff happens in a real browser window. VHS records
#     wall-clock time, so approve the consent screen during the final wait.
#
# Record with: task demo:gif -- auth-login

Output docs/demos/auth-login.gif

Require circleci

Set Shell "bash"
Set Theme "JetBrains Darcula"
Set Width 1300
Set Height 650
Set WindowBar Colorful
Set TypingSpeed 75ms

Sleep 500ms
Type "circleci auth login"
Sleep 500ms
Enter

# Host picker: keep the default (circleci.com).
Wait+Screen@15s /Where do you use CircleCI/
Sleep 1.5s
Enter

# Method picker: first detour into the token flow — step down to "Paste an
# authentication token" and open it.
Wait+Screen@15s /Login with a web browser/
Sleep 1.5s
Down
Sleep 1.5s
Enter

# Show the personal-access-token entry screen…
Wait+Screen@15s /personal access token/
Sleep 2.5s

# …then back out (esc) and take the browser flow instead. The picker resets
# its cursor to the first option (the browser flow), so Enter picks it.
Escape
Sleep 1.5s
Enter

# The authorize URL is shown; this Enter opens it in the real browser.
Wait+Screen@15s /Press Enter to open/
Sleep 2s
Enter

# Approve the OAuth consent screen in the browser now. A few seconds of
# spinner are captured, then the Hide/Show around the wait trims however
# long the consent actually takes — the gif jumps straight to success.
Sleep 3s
Hide
Wait+Screen@120s /Logged in as/
Show

# Hold on the success message.
Sleep 4s
