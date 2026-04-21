# Philosophy

CLI design is guided by eight core principles. These aren't rules to follow mechanically — they're lenses for making judgment calls when the right answer isn't obvious.

---

## 1. Human-First Design

Design for humans first. Traditional UNIX philosophy optimized for machine consumption; modern CLIs should optimize for the humans actually running them, while still maintaining composability.

> The user is not a script. When a human is at the terminal, they should feel helped, not just processed.

This doesn't mean abandoning composability — it means composability should be opt-in (via piping, `--json`, etc.) rather than the default at the expense of usability.

---

## 2. Simple Parts That Work Together

Design commands as modular units with clean interfaces. Use standard streams (stdin/stdout/stderr), exit codes, signals, and plain text to enable integration with other tools.

The goal: a program that works well on its own *and* composes naturally in pipelines.

---

## 3. Consistency

Follow established patterns so users can transfer knowledge from other tools. When a flag means one thing in `git`, it should mean the same thing in your tool — unless you have a deliberate reason to differ.

**When to break convention:** Only when following it would genuinely harm usability, and only with a clear intentional reason. Inconsistency without justification is just friction.

---

## 4. Saying Just Enough

Balance information density. Too little output leaves users confused about what happened. Too much buries the important details.

The right amount of output:
- Confirms what happened
- Surfaces anything the user needs to act on
- Stays out of the way for scripting contexts

---

## 5. Ease of Discovery

Make functionality findable through help text, examples, and error suggestions — not just through external documentation. A user should be able to orient themselves entirely within the terminal.

Combine CLI efficiency with GUI-like learnability: a new user should be able to figure out what to do next without leaving the command line.

---

## 6. Conversation as the Norm

CLIs are conversational. Users try things, get feedback, adjust, try again. Design for this loop:

- Suggest corrections when input is wrong
- Clarify intermediate states
- Confirm before taking dangerous actions
- Show what happened so the user can reason about next steps

---

## 7. Robustness

Robustness has two dimensions:
- **Objective:** Graceful error handling, idempotence, predictable behavior on edge cases
- **Subjective:** The tool *feels* solid and reliable — it won't surprise you

Keep implementations simple where possible. Complexity breeds fragility.

---

## 8. Empathy

Show users you're on their side. A well-designed CLI demonstrates that someone thought carefully about the experience. This means:

- Anticipating mistakes and helping recover from them
- Explaining what went wrong in human terms
- Suggesting what to do next
- Not silently doing the wrong thing

---

## 9. Transparency

Every action your command takes must be visible to the user. Implicit steps — things that happen without the user explicitly requesting them — are dangerous and erode trust.

The anti-pattern: an `init` command that silently overwrites existing configuration. Even if it's technically correct, the user can't tell what changed or why their previous setup disappeared.

The fix: surface implicit steps explicitly, or split the command so each step is a deliberate user action.

> "Every action taken by a command should be transparent to the user. Implicit steps are dangerous and damage the quality of the developer experience." — Thoughtworks

This extends beyond state changes to any cross-boundary action: file writes, network calls, credential lookups. If the user didn't explicitly ask for it, tell them it's happening.

---

## On Developer Experience (DX) as a Design Frame

For teams building internal developer platforms or shared engineering tooling, CLI quality is a DX problem — not just a correctness problem. The right question is: *does this make developers more effective and less frustrated?*

In this context, delight matters. A tool that engineers enjoy using gets adopted; one that frustrates gets worked around. Developer satisfaction with tooling is measurable and consequential. The same design question that applies to consumer UX applies here: would a developer smile or grimace when using this?

---

## On Breaking the Rules

Terminal conventions are inconsistent. Established tools break their own rules all the time. When you break a convention:

- Do it deliberately, not by accident
- Have a clear reason
- Be consistent within your own tool

The goal isn't rule-following for its own sake — it's making tools that feel good to use.
