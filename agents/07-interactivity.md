# Interactivity

Interactive prompts help humans but break automation. The key is knowing when to be interactive and when to get out of the way.

---

## The Right Mental Model: Prompts Are a First-Time-User Affordance

Prompts help users who are new to the command, or returning after a long break. They slow down power users and break automation entirely. Design with this in mind:

> *"Accepting input as a prompt is desirable to guide first-time users. However, you should never require a prompt. Users need to be able to use your CLI for automation and should always be able to override prompts."* — Thoughtworks

**Design order:** spec the flag-based interface first. Then add prompts as a discoverability layer on top. Never design prompts as the primary interface and flags as an afterthought.

---

## The Core Tension

Interactive prompts are valuable for humans making one-off decisions. They're a disaster in scripts, CI, or any automated context. Design so that:

- Interactive mode is the default when a TTY is present
- Non-interactive/scriptable mode is always achievable (via flags or detecting no TTY)

---

## Confirming Dangerous Operations

Before any destructive or irreversible operation, ask for confirmation when in an interactive TTY:

```
$ myapp delete project my-app
⚠ This will permanently delete 'my-app' and all associated data.
  This action cannot be undone.

Continue? [y/N] 
```

### Design principles for confirmation prompts

- **Make the safe answer the default.** Use uppercase to indicate the default: `[y/N]` means "N" is the default; pressing Enter without typing = No.
- **Be specific** about what will be deleted/changed. Users need to know what they're confirming.
- **Support `--force` / `-f`** to bypass confirmation in scripts:
  ```sh
  myapp delete project my-app --force   # no prompt, just does it
  ```
- **Support `--dry-run` / `-n`** to preview without executing:
  ```sh
  myapp delete project my-app --dry-run  # shows what would be deleted
  ```

---

## Prompting for Missing Information

In interactive mode, prompting for missing required information is better than erroring:

```
# In a TTY — prompt for what's missing
$ myapp init
Project name: my-app
Environment (production/staging/preview): staging
API key: ****

✓ Initialized project 'my-app' for staging environment.
```

```
# Not in a TTY — error with clear message
$ myapp init
Error: --name is required
Error: --environment is required

Run 'myapp init --help' to see all options.
```

### When to prompt vs. error

| Context | Behavior |
|---------|----------|
| Interactive TTY | Prompt for missing required inputs |
| Non-TTY (piped/CI) | Error with clear message listing required flags |
| Optional inputs | Use sensible defaults, don't prompt |

---

## Prompt Best Practices

### Be clear and direct
State exactly what you're asking for. Don't use vague prompts like "Value:".

```
# Bad
Value: 

# Good
Database host [localhost]: 
```

### Show the default
Include the default in brackets: `[localhost]`. Pressing Enter accepts the default.

### Use `[y/N]` / `[Y/n]` convention consistently
- `[y/N]` → default is No (uppercase)
- `[Y/n]` → default is Yes (uppercase)

### Mask sensitive input
When prompting for passwords or tokens, don't echo input back to the terminal:

```
API Token: ••••••••••••••••
```

### Validate immediately
If a prompt accepts constrained values, validate on entry and re-prompt rather than accepting then erroring:

```
Port [8080]: 99999
Invalid port (must be 1-65535). Try again.
Port [8080]: 
```

---

## Don't Over-Prompt

Interactive prompts have costs:
- They break scripting
- They slow down power users who know what they want
- They feel paternalistic when overused

Use prompts sparingly:
- **Do prompt:** Before destructive/irreversible operations
- **Do prompt:** For genuinely required information with no reasonable default
- **Don't prompt:** For every optional setting (use defaults instead)
- **Don't prompt:** In situations where the user has clearly expressed intent via flags

---

## Disable Interactivity for CI/Scripts

Always provide a way to disable all interactivity:

```sh
myapp deploy --force        # skip confirmations
myapp init --non-interactive --name foo --env staging  # no prompts at all
CI=true myapp deploy        # many tools check this env var
```

Many CI environments set `CI=true`. Consider checking it and automatically disabling prompts when it's set.

---

## Summary Checklist

- [ ] Destructive operations ask for confirmation in a TTY
- [ ] Confirmation prompts default to "No" (the safe answer) via `[y/N]`
- [ ] `--force` / `-f` bypasses confirmations for scripting
- [ ] `--dry-run` available for previewing destructive operations
- [ ] Missing required inputs prompt when in TTY, error with helpful message when not
- [ ] Sensitive inputs (passwords, tokens) masked during entry
- [ ] Prompts show defaults in brackets: `[default value]`
- [ ] Non-interactive mode achievable via flags or `CI=true`
- [ ] Prompts used sparingly — not for every optional setting
