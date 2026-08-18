# Plan template

**Rules**

1. **A plan written into `agents/` opens with "Guidelines consulted"** — the
   `agents/` files you actually read before writing it, each with the rule you
   took from it. An empty or hand-waved list is the signal that the plan is
   guesswork.
2. **Read the files the [CLAUDE.md](../CLAUDE.md) trigger table names for the work
   you are planning** — including [14-testing.md](./14-testing.md), because a plan
   that specifies the wrong test approach gets implemented that way.
3. **Say what you researched and rejected**, with the reason. "We can't reuse X
   because Y" is the most reusable part of a plan.
4. **Record deviations as you implement.** A plan that silently stops matching the
   code is worse than no plan; add a "What changed during implementation" section
   rather than editing history.
5. **Name the file `plan-<topic>.md`** and keep it beside the guidelines it cites.

---

## Why "Guidelines consulted" is first

A plan is reviewed before any code exists, which makes it the cheapest place to
catch a wrong convention — and the most expensive place to hide one, because a
plan gets implemented faithfully. Listing the files read turns "did you check the
rules?" into something a reviewer can see rather than infer.

This is not paperwork: it is the record that rule 8 in
[CLAUDE.md](../CLAUDE.md) was followed. If a row of the trigger table applies to
the work and its file is not in the list, the plan is not ready for review.

---

## Skeleton

```markdown
# Plan — <what this adds or changes>

<One paragraph: what is being built, where it lives, and for whom.>

Status: **<not started | implemented>.** <Where deviations are recorded.>

## Guidelines consulted

| File | What it required of this work |
|---|---|
| [14-testing.md](./14-testing.md) | Component tests via teatest; `t.Run` groupings |
| [07-interactivity.md](./07-interactivity.md) | Program in `internal/ui`, reusable parts in `clikit/ui/components` |
| … | … |

## Research: what already exists

<What you looked at, what you are reusing, and what you rejected with the reason.
Include the upstream libraries you checked, not just this repo.>

## Design

<The user-facing behaviour first — keys, output, errors — then the internals.>

## Files to change

<Per file or package: what changes, and any refactor that has to land first.>

## Tests

<Which kind of test covers what, per 14-testing.md's "Which kind of test" table.>

## Order of work

<Numbered steps, each independently verifiable. Put refactors first, on their own.>

## Deferred, and why

<What you are explicitly not doing, so a reader stops wondering.>

## What changed during implementation

<Filled in as you go: every departure from the design above, with the reason.>
```
