# Extensibility and Lifecycle Hooks

For CLIs that support plugins or need shared cross-cutting behaviour across many commands, a lifecycle hook system is the standard pattern. Hooks let plugins extend the CLI at specific points without modifying core code.

---

## The Problem Hooks Solve

Without hooks, cross-cutting concerns end up duplicated in every command:

```typescript
// Without hooks: every command repeats this
async run() {
  await checkForUpdates()        // repeated in 40 commands
  await validateAuth()           // repeated in 40 commands
  // ... actual command logic
  await reportAnalytics()        // repeated in 40 commands
}
```

With hooks, these run once automatically around every command:

```typescript
// prerun hook: runs before every command
async function prerun() {
  await checkForUpdates()
  await validateAuth()
}

// postrun hook: runs after every successful command
async function postrun() {
  await reportAnalytics()
}
```

---

## Standard Lifecycle Hook Points

| Hook | When it fires | Common uses |
|------|-------------|-------------|
| `init` | CLI starts, before command is resolved | Load config, check environment, start telemetry |
| `preparse` | Before flags and args are validated | Inject default flags, manipulate raw input |
| `prerun` | After command found, just before execution | Auth checks, update notifications, validation |
| `postrun` | After **successful** command completion | Analytics, cleanup, success notifications |
| `command_not_found` | When the command string doesn't match any command | Suggest corrections, handle legacy aliases |

Note: `postrun` fires **only on success**. For cleanup that must run regardless of outcome, use `init` to register cleanup with a process signal handler.

---

## Hook Design Guidelines

### Keep hooks lightweight
Hooks run on every command invocation. Heavy work in `init` or `prerun` slows down every command — including fast commands like `--help` and `--version`. Anything slow should be async and ideally cached.

### Multiple hooks run in parallel
When multiple plugins register the same hook (e.g., two plugins both have a `prerun` hook), they run concurrently. Design hooks to be independent — don't assume ordering between hooks from different plugins.

### Handle hook errors carefully
A failing hook can abort the entire command. For optional hooks (analytics, update checks), catch errors internally and fail silently — don't let a failed update check prevent the user from running their command.

```typescript
// Good: analytics failure doesn't kill the command
async function postrun() {
  try {
    await reportAnalytics()
  } catch {
    // silently ignore — don't interrupt the user
  }
}
```

### Use `command_not_found` for helpful recovery
The `command_not_found` hook is called before the "command not found" error is displayed. Use it to:
- Suggest the most similar valid command (typo correction)
- Handle renamed commands from previous versions
- Provide context-specific guidance

```
$ myapp depoy production
Error: command 'depoy' not found

Did you mean 'deploy'?
Run 'myapp --help' to see all commands.
```

---

## Custom Hooks

Beyond the built-in lifecycle hooks, you can define your own hook events for inter-plugin communication:

```typescript
// Plugin A emits a custom hook
await this.config.runHook('my-custom-event', { data: payload })

// Plugin B listens for it
hooks: {
  'my-custom-event': async (options) => {
    // respond to the event
  }
}
```

This enables plugins to coordinate without direct dependencies on each other.

---

## What Belongs in a Hook vs. a Command

| Concern | Where it goes |
|---------|--------------|
| Auth validation before every command | `prerun` hook |
| CLI update checking | `init` hook (async, cached) |
| Usage analytics | `postrun` hook (async, fire-and-forget) |
| Config file loading | `init` hook |
| Cleanup after all commands | `init` hook + signal handler |
| Typo correction | `command_not_found` hook |
| Business logic for a specific command | The command's `run()` method |

---

## Summary Checklist

- [ ] Cross-cutting logic (auth, analytics, update checks) in hooks, not duplicated per-command
- [ ] `init` hook for setup and config loading
- [ ] `prerun` hook for validation that applies to all commands
- [ ] `postrun` hook for cleanup and analytics (only fires on success)
- [ ] `command_not_found` hook provides typo suggestions
- [ ] Hook errors handled silently when the hook is optional
- [ ] Hooks kept lightweight — avoid blocking slow operations
- [ ] Multiple hooks from different plugins designed to be order-independent
