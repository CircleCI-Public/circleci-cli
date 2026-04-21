# Arguments and Flags

Arguments and flags are the primary interface between your user and your command. Getting the conventions right makes your tool feel familiar; getting them wrong creates constant friction.

---

## Arguments vs. Flags

**Arguments** (positional parameters) are the primary inputs to a command. Their meaning is determined by position.

```sh
cp source.txt dest.txt    # source and dest are positional arguments
```

**Flags** (named parameters) modify behavior. They have names and are order-independent.

```sh
cp -r source/ dest/           # -r is a flag
cp --recursive source/ dest/  # --recursive is the long form
```

### When to use arguments vs. flags

**Use arguments for:** A single, unambiguous primary input where the intent is obvious — typically a filename or resource name. If you need more than one positional arg, or the meaning isn't immediately clear from position alone, use flags instead.

**Use flags for:** Everything else — options, modifiers, and any input where the name adds clarity.

**When in doubt, prefer flags.** They're more typing, but significantly more readable, and they're far easier to add or change later without breaking existing scripts. A useful rule of thumb for positional arguments:

> **One is fine. Two is questionable. Three is an absolute no.** — Thoughtworks

If you find yourself defining three positional arguments, that's a signal to redesign using flags:

```sh
# Hard to understand at a glance — what do these positional args mean?
myapp create foo bar baz

# Self-documenting
myapp create --name foo --type bar --region baz
```

Flags are also easier to evolve — you can add new flags without breaking existing invocations.

---

## Flag Conventions

### Single-letter vs. long flags

Short flags use a single dash: `-v`
Long flags use a double dash: `--verbose`

Provide both forms for commonly-used flags. Short forms reward power users; long forms make scripts readable.

### Grouping short flags

Allow `-abc` as shorthand for `-a -b -c`. This is standard UNIX behavior.

```sh
tar -xzf archive.tar.gz   # equivalent to tar -x -z -f archive.tar.gz
```

### Flag value syntax

Support both forms:
```sh
--output=file.txt
--output file.txt
```

### Required flag arguments

If a flag takes a value, always require it. Don't make the value optional and context-dependent — it creates confusing edge cases.

```sh
# Bad: is -o a boolean flag or does it take a value?
myapp export -o           # ambiguous

# Good: always requires a value
myapp export --output file.txt
```

---

## Standard Flag Names

Follow these conventions whenever your command needs the corresponding functionality. Deviating from these confuses users who've learned them from other tools.

| Short | Long | Meaning |
|-------|------|---------|
| `-h` | `--help` | Show help |
| `-V` | `--version` | Show version |
| `-v` | `--verbose` | Increase output verbosity |
| `-q` | `--quiet` | Suppress non-essential output |
| `-f` | `--force` | Force operation, skip confirmations |
| `-n` | `--dry-run` | Show what would happen without doing it |
| `-o` | `--output` | Output file or destination |
| `-r` | `--recursive` | Operate recursively |
| `-p` | `--port` | Port number |
| `-H` | `--host` | Hostname |
| `-u` | `--user` | Username |
| `-d` | `--debug` | Enable debug output |
| `-c` | `--config` | Config file path |

**Note:** `-h` should *always* mean help. Never use it for `--host` or anything else.

---

## Flag Defaults

Always document the default value for every flag. Don't make users guess what happens when they don't pass a flag.

```
Options:
  -t, --timeout <seconds>   Request timeout                [default: 30]
  -r, --retries <n>         Number of retries on failure   [default: 3]
  -f, --format <type>       Output format                  [default: table]
```

---

## Flag Typing

Where your language or framework supports it, declare flags as typed rather than treating everything as a raw string. The parser should validate the type before the command runs:

- **Integer flags** — validate that the value is a whole number
- **URL flags** — validate that the value is a well-formed URL
- **File flags** — validate that the file path exists and is readable
- **Directory flags** — validate that the directory path exists
- **Enum flags** — restrict to a discrete set of allowed values via `options`

Catching type errors at parse time — before any work begins — produces clearer errors and removes manual validation code from command logic.

---

## Flag Relationships

When flags interact with each other, declare those relationships explicitly at the flag level — don't check them manually in command logic:

| Relationship | Meaning | Example |
|-------------|---------|---------|
| `dependsOn` | This flag requires another flag to also be present | `--ssl` requires `--domain` |
| `exclusive` | This flag cannot be used together with another flag | `--json` conflicts with `--verbose` |
| `exactlyOne` | Exactly one of a set of flags must be provided | Must use `--input` or `--stdin`, but not both |

Enforcing these at parse time means the command never even starts if the constraints are violated, and the error message is automatic and consistent.

---

## Flag Description Text Style

When writing the description text for flags in help output, follow these conventions:

- **Lowercase** — start with a lowercase letter, not a capital
- **Concise** — short enough to fit on a narrow terminal without wrapping
- **No trailing period** — descriptions are labels, not sentences

```
# Good
--region    region to deploy in
--timeout   request timeout in seconds
--stack     stack name

# Bad
--region    Specifies the region to deploy in.    ← capitalized, ends in period
--timeout   The timeout value for the request, in seconds.   ← too long, ends in period
```

This keeps help text consistently readable and avoids a mix of styles across flags.

---

## Validation

Validate flag values early — before beginning any work — and reject clearly:

```
Error: --timeout must be a positive integer, got: "fast"
Error: --format must be one of: table, json, csv (got: "xml")
Error: --port must be between 1 and 65535 (got: 99999)
```

---

## Don't Require Flags for Basic Functionality

Flags modify behavior — they shouldn't be the gate to basic functionality. The most common use case should work without any flags.

```sh
# Good: works without flags
myapp deploy production

# Bad: requires flags for basic use
myapp deploy --environment production --strategy rolling --wait true
```

---

## Boolean Flags

Boolean flags should default to `false` and be enabled by presence:

```sh
myapp deploy --dry-run    # dry-run is now true
myapp deploy              # dry-run is false (default)
```

If you need to explicitly disable a default-true flag, support the `--no-` prefix:

```sh
myapp build               # color enabled by default
myapp build --no-color    # disable color
```

---

## The `--` Passthrough Convention

For commands that wrap child processes, support the `--` separator to pass remaining arguments through verbatim without flag parsing:

```sh
myapp run -- node server.js --port 3000
# Everything after -- is passed to `node` unchanged
```

This is a standard UNIX convention and essential for any "exec" or "run" style command.

---

## Sensitive Flag Values and Stdin

Flags that accept sensitive values (API tokens, passwords, secrets) should support reading from stdin to keep values out of shell history:

```sh
cat token.txt | myapp login --token -
# The `-` value signals: read this flag's value from stdin
```

This prevents the secret from appearing in `history`, process listings, or CI logs.

---

## Environment Variable Equivalents

Major flags should have corresponding environment variable alternatives. Where possible, declare the env var mapping **directly on the flag definition** rather than in separate configuration code — this makes the relationship discoverable and ensures it's auto-documented in help text:

```typescript
// Good: env var declared alongside the flag
region: Flags.string({ env: 'MYAPP_REGION', description: 'region to deploy in' })

// Less good: env var handled in separate config code, invisible to help output
```

See [10-configuration-and-env.md](./10-configuration-and-env.md) for naming conventions.

```sh
# These should be equivalent:
myapp --api-key=abc123 deploy
MYAPP_API_KEY=abc123 myapp deploy
```

---

## Summary Checklist

- [ ] Positional args used for primary inputs; flags used for modifiers
- [ ] Both short (`-v`) and long (`--verbose`) forms provided for common flags
- [ ] Short flags can be grouped (`-abc`)
- [ ] Both `--flag=value` and `--flag value` syntax supported
- [ ] Standard flag names followed (`-h` for help, `-v` for verbose, etc.)
- [ ] All flag defaults documented in help text
- [ ] Flag values validated early with clear error messages
- [ ] Basic functionality works without flags
- [ ] Boolean flags use `--no-flag` convention for disabling defaults
