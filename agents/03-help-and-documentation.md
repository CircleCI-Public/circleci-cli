# Help and Documentation

Good help text is often the difference between a tool that gets adopted and one that gets abandoned. Users should be able to orient themselves entirely from within the terminal.

---

## Two-Level Documentation: Summary and Description

Every command and flag should have two distinct levels of documentation:

- **Summary** — One tight sentence. Shown in command and topic listings where space is limited. Should be scannable: a user reading a list of 20 commands should be able to orient from summaries alone.
- **Description** — Full multi-paragraph explanation. Shown only in the command's own `--help` output. Can include context, caveats, related commands, and links.

```
# In the command listing (summary only):
  deploy    deploy your application to an environment
  rollback  roll back to a previous deployment
  status    show current deployment status

# In `myapp deploy --help` (full description):
  Deploy your application to the specified environment.

  The deploy command builds your current working directory,
  uploads it to the platform, and starts the new version.
  Previous deployments remain available for rollback.
  ...
```

Treat the summary as a hard constraint: it must fit on one line without wrapping, even on a narrow terminal.

---

## The Two Levels of Help

### Concise help (shown automatically)

When a user runs a command that requires arguments but provides none, show a brief help message — don't just error out silently or hang.

A concise help message should include:
1. One sentence describing what the command does
2. One or two example invocations
3. Key flag descriptions (or a note that `--help` shows more)
4. How to get full help

```
Usage: myapp deploy [options] <environment>

Deploy your application to a target environment.

Examples:
  myapp deploy production
  myapp deploy staging --dry-run

Run 'myapp deploy --help' for full options.
```

**Exception:** Interactive programs that prompt for missing input (like `npm init`) don't need to show help on empty invocation.

### Full help (`-h` / `--help`)

Triggered by `-h` or `--help`. Should include:
- What the command does
- All flags and their defaults
- Multiple usage examples progressing from simple to complex
- A link to web documentation for advanced topics
- A support path (GitHub issues URL, etc.)

```
myapp deploy [options] <environment>

Deploy your application to the specified environment.

Arguments:
  environment    Target environment (production, staging, preview)

Options:
  -n, --dry-run       Show what would be deployed without deploying  [default: false]
  -f, --force         Skip confirmation prompts                      [default: false]
  -t, --timeout <s>   Deployment timeout in seconds                  [default: 300]
  -h, --help          Show this help message

Examples:
  myapp deploy production
  myapp deploy staging --dry-run
  myapp deploy preview --force --timeout 60

Documentation: https://docs.myapp.com/deploy
Issues:        https://github.com/myapp/myapp/issues
```

---

## Help Text Best Practices

### Lead with examples
Users scan for examples first. Put your most common use cases at the top, then build toward more complex ones.

### Show the default values for flags
If a flag has a default, show it: `[default: false]`. Users shouldn't have to guess what happens when they omit a flag.

### Support all these help invocations (for git-style tools)
```sh
myapp help
myapp help <subcommand>
myapp <subcommand> --help
myapp <subcommand> -h
```

### Group flags into named sections for complex commands

When a command has more than ~8 flags, an undifferentiated list becomes hard to scan. Group flags into named sections:

```
Global Options:
  -a, --app <name>      app to use
  -r, --region <name>   region to deploy in

Output Options:
  --json                output in JSON format
  -q, --quiet           suppress non-essential output
  -v, --verbose         show more details

Advanced Options:
  --timeout <seconds>   request timeout  [default: 30]
  --no-cache            disable caching
```

Put the most commonly-used group first.

### Hidden commands and flags

Commands and flags can be hidden from help output while remaining fully functional. Use this for:
- Deprecated flags being phased out (still work, but not advertised)
- Internal debugging flags (`--debug`, `--trace`, `--verbose-debug`)
- Experimental commands not yet ready for broad use
- Backward-compatibility aliases

```sh
myapp --debug   # works even though --debug doesn't appear in --help
```

### Use formatting for scannability
Group flags under headings. Bold the flag names. Keep descriptions aligned. Terminal-independent formatting (not raw ANSI escape codes in the source) makes this maintainable.

### Don't overload `-h`
`-h` should always mean help. Never use it for another purpose (e.g., `-h` for `--host`).

---

## Suggesting Corrections

When a user makes a likely mistake (typo in subcommand, wrong flag name), suggest the correction:

```
Error: unknown command 'depoy'

Did you mean 'deploy'?

Run 'myapp --help' to see available commands.
```

**Important:** Suggest corrections — don't silently auto-correct. Auto-correcting without asking:
- Prevents the user from learning the correct syntax
- Commits you to supporting the incorrect syntax forever
- Can cause surprising behavior in scripts

---

## Documentation Beyond Help Text

### Web documentation
Publish documentation on the web so users can search for it, link to specific sections, and access it without installing the tool. Keep links in your help text pointing to specific anchors.

### Terminal documentation
Provide documentation accessible from the terminal for:
- Faster access (no browser switch)
- Offline use
- Version-specific docs (the docs match the installed version)

Example: `myapp docs deploy` opens or prints the deploy documentation.

### Man pages
Many users reflexively run `man myapp`. Consider generating man pages using tools like `ronn` (Ruby) or similar. Make them accessible both via `man` and via your own tool (e.g., `myapp man deploy`).

---

## Summary Checklist

- [ ] Concise help shown when command run with no args (if args are required)
- [ ] Full help available via `-h` and `--help`
- [ ] Help text leads with examples
- [ ] All flags documented with their defaults
- [ ] Support path (URL for issues/feedback) included in top-level help
- [ ] Link to web docs in help text
- [ ] Typo/correction suggestions implemented
- [ ] stdin-dependent commands show help instead of hanging on empty TTY
