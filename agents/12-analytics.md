# Analytics and Telemetry

Telemetry can provide valuable insight into how your tool is used. It can also erode user trust if done poorly. The rules here are straightforward: be transparent, collect minimally, and always allow opt-out.

---

## Core Principles

### Transparency first
Document exactly what data you collect, how it's used, and where it's stored. Users shouldn't need to read source code to understand what information leaves their machine.

Place this documentation in:
- Your README
- Your help text (`myapp --help` or `myapp telemetry --help`)
- A dedicated web page linked from your docs

### Collect minimally
Collect only what genuinely helps you improve the tool. Resist the urge to collect "just in case" data. Every piece of data you collect is a privacy liability.

**Useful to collect:**
- Which subcommands are used (aggregate counts)
- Error types that occur (aggregate counts)
- Rough version distribution
- OS/platform breakdown

**Avoid collecting:**
- File paths or filenames
- Command arguments and flag values
- Personal information (names, emails, IPs where possible)
- Environment variable names or values
- Hostnames or network details

---

## Opt-Out (or Opt-In)

### Always provide opt-out
Users must be able to disable telemetry easily:

```sh
# Via flag
myapp --no-telemetry deploy

# Via environment variable (most important for CI)
MYAPP_NO_TELEMETRY=1 myapp deploy
NO_ANALYTICS=1 myapp deploy  # many users expect this convention

# Via config
myapp config set telemetry false
```

### Consider opt-in instead
For privacy-sensitive tools or user bases, consider making telemetry opt-in:

```sh
# First run prompt (only in interactive TTY)
To help improve myapp, we'd like to collect anonymous usage data.
No personal information is collected. Data is aggregated and anonymized.

Enable telemetry? [y/N] 
```

If opting in, save the preference so you don't ask every time.

### Don't collect before asking
For opt-in telemetry: if you haven't received user consent yet, collect nothing.

---

## Implementation Guidelines

### Be asynchronous
Never let telemetry calls block the main command execution. Fire and forget — if the telemetry request fails, swallow the error silently.

### Use aggregated data
Design your telemetry to send aggregate counts rather than individual events where possible. Aggregate on the client side before sending, or use a backend that aggregates.

### Respect `NO_COLOR` conventions
Some users run tools like `DO_NOT_TRACK=1` in their shell globally. Respect these signals even if they're not your specific variable.

### Standard environment variables to respect

| Variable | Behavior |
|----------|---------|
| `MYAPP_NO_TELEMETRY=1` | Disable all telemetry |
| `NO_ANALYTICS=1` | Disable all telemetry |
| `DO_NOT_TRACK=1` | Disable all telemetry (broad convention) |
| `CI=true` | Consider disabling by default in CI |

---

## Example Disclosure

This is what transparent telemetry documentation looks like:

```
Telemetry
---------
myapp collects anonymous usage statistics to help improve the tool.

What we collect:
  - Which commands and subcommands are used
  - Error types (not error messages or file paths)
  - myapp version, OS, and architecture

What we DO NOT collect:
  - Command arguments or flag values
  - File or directory names
  - IP addresses or hostnames
  - Any personally identifiable information

To disable telemetry:
  Set MYAPP_NO_TELEMETRY=1 in your environment, or:
  Run: myapp config set telemetry false

Full privacy policy: https://myapp.com/privacy
```

---

## Summary Checklist

- [ ] Telemetry clearly documented (what's collected, how it's used)
- [ ] Documentation linked from README and help text
- [ ] `MYAPP_NO_TELEMETRY=1` env var disables all collection
- [ ] `NO_ANALYTICS=1` and `DO_NOT_TRACK=1` also respected
- [ ] No PII collected (no file paths, argument values, IPs, hostnames)
- [ ] Telemetry calls are asynchronous and non-blocking
- [ ] Errors in telemetry silently swallowed — never surface to user
- [ ] Consider opt-in rather than opt-out for privacy-sensitive contexts
- [ ] CI environments handled (disable or treat specially)
