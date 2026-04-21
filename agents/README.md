# CLI Design Guidelines

A structured reference for writing well-designed command-line programs, based on [clig.dev](https://clig.dev/) — the Command Line Interface Guidelines by Aanand Prasad, Ben Firshman, Carl Tashian, and Eva Parish.

These files are intended for use by an agent writing a new CLI tool. Start with the checklist for a quick pass, then consult individual topic files for deeper guidance.

---

## File Index

| File | Topic |
|------|-------|
| [checklist.md](./checklist.md) | Quick-reference checklist — use this during implementation |
| [01-philosophy.md](./01-philosophy.md) | Core design philosophy and principles |
| [02-basics.md](./02-basics.md) | Essential requirements: exit codes, streams, argument parsing |
| [03-help-and-documentation.md](./03-help-and-documentation.md) | Help text, examples, and documentation |
| [04-output.md](./04-output.md) | Output formatting, color, JSON, verbosity |
| [05-errors.md](./05-errors.md) | Error messages, exit codes, stderr |
| [06-arguments-and-flags.md](./06-arguments-and-flags.md) | Arguments, flags, conventions, naming |
| [07-interactivity.md](./07-interactivity.md) | Prompts, confirmations, interactive mode |
| [08-subcommands.md](./08-subcommands.md) | Subcommand structure and design |
| [09-robustness.md](./09-robustness.md) | Edge cases, idempotency, signals |
| [10-configuration-and-env.md](./10-configuration-and-env.md) | Config files and environment variables |
| [11-naming-and-distribution.md](./11-naming-and-distribution.md) | Naming conventions and distribution |
| [12-analytics.md](./12-analytics.md) | Telemetry and analytics best practices |
| [13-extensibility.md](./13-extensibility.md) | Lifecycle hooks and plugin architecture |
| [heroku-gap-analysis.md](./heroku-gap-analysis.md) | Differences vs. Heroku CLI style guide |
| [oclif-gap-analysis.md](./oclif-gap-analysis.md) | Differences vs. oclif framework standards |
| [thoughtworks-gap-analysis.md](./thoughtworks-gap-analysis.md) | Differences vs. Thoughtworks CLI guidelines |

---

## The Core Tension

The central challenge of CLI design is **balancing human usability with machine composability**. A good CLI:

- Defaults to human-friendly output (formatted, colored, verbose enough to be clear)
- Degrades gracefully when piped or scripted (detects TTY, supports `--json`, `--plain`, `-q`)
- Follows conventions so users can transfer knowledge from other tools
- Communicates clearly: what happened, what went wrong, what to do next

---

## Quick Orientation for Agents

When building a new CLI, work through these concerns in order:

1. **What does the command do?** → Name it clearly, keep it lowercase with hyphens.
2. **What are its inputs?** → Prefer flags over positional args; follow UNIX conventions.
3. **What does it output?** → stdout for data, stderr for messages; support `--json`.
4. **How does it fail?** → Non-zero exit codes, clear error messages on stderr.
5. **How does it get configured?** → Flags override env vars override config files.
6. **How does it explain itself?** → `-h`/`--help` with examples, suggest next steps.

---

*Source: [https://clig.dev](https://clig.dev) — licensed under [Creative Commons Attribution-ShareAlike 4.0](https://creativecommons.org/licenses/by-sa/4.0/)*
