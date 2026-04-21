# The Basics

These are the non-negotiable foundations. Every CLI should get these right before worrying about anything else.

---

## Use an Argument Parsing Library

Don't hand-roll argument parsing. Use a well-maintained library for your language. A good library gives you:

- Consistent flag parsing (`-v`, `--verbose`, `--output=file`, `--output file`)
- Automatic `--help` generation
- Useful error messages for invalid flags
- Spelling suggestions for mistyped commands

**Examples by language:**
- Python: `argparse`, `click`, `typer`
- Go: `cobra`, `flag` (stdlib)
- Node.js: `commander`, `yargs`, `meow`
- Rust: `clap`
- Ruby: `optparse` (stdlib), `thor`

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error |
| `2` | Misuse of shell or command (bad arguments) |
| Other non-zero | Specific, documented failure modes |

**Rules:**
- Always return `0` on success
- Always return non-zero on failure
- Map meaningful non-zero codes to your most important failure modes and document them
- Scripts depend on exit codes to determine whether to continue — getting this wrong breaks automation silently

**Bad pattern:**
```sh
# Returns 0 even on failure — scripts can't detect this
myapp do-thing || echo "failed"  # "failed" never prints even when it should
```

**Good pattern:**
```sh
myapp do-thing
echo $?  # 0 = success, 1 = error — script can branch on this
```

---

## Standard Streams

### stdout — Primary output
- All primary command output goes to stdout
- Machine-readable output (JSON, CSV, etc.) goes to stdout
- This is what piping captures by default

```sh
myapp list | grep "active"   # works because list output is on stdout
myapp export --json | jq '.items[]'  # works because JSON is on stdout
```

### stderr — Messages and errors
- Log messages, status updates, warnings, and errors go to stderr
- stderr displays to the user but is NOT captured by piping
- This means informational messages don't corrupt piped output

> **Note on warnings:** Thoughtworks recommends routing warnings to stdout (alongside informational output), reserving stderr for errors only. The consensus from clig.dev, oclif, and Heroku is that warnings belong on stderr. The reasoning: if a user is piping output to another command, warning text on stdout will corrupt the data stream. Warnings should be on stderr so they're visible to the human but invisible to the pipe. Keep warnings on stderr unless you have a specific reason to diverge.

```sh
myapp export > data.json
# "Exporting 42 records..." appears on terminal (stderr)
# but data.json only contains the JSON (stdout)
```

### stdin — Input
- If your command reads from stdin, handle the case where stdin is a TTY (interactive terminal)
- Don't hang waiting for piped input that isn't coming — display help and exit instead

```sh
# Bad: hangs waiting for input
myapp process

# Good: detects no stdin, shows help
myapp process
# Usage: myapp process [file]
# Error: no input provided. Pass a file path or pipe data via stdin.
```

---

## Summary Checklist

- [ ] Using an argument parsing library (not hand-rolled)
- [ ] Exit code `0` on success, non-zero on all failures
- [ ] Primary output on stdout
- [ ] Errors and messages on stderr
- [ ] stdin handled gracefully (no hanging on empty TTY)
