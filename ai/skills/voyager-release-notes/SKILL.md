---
name: voyager-release-notes
description: >-
  Generate yb-voyager release notes from a commit range, formatted like the
  YugabyteDB Voyager release-notes docs page. Use when the user asks to draft
  release notes, summarize a release, or write what's-new entries for a
  voyager version.
---

# Release Notes Generation

## Overview

This skill produces a release-notes block for a yb-voyager version in the
same shape as the canonical docs page
(`docs/content/stable/yugabyte-voyager/release-notes.md` in the yugabyte-db
repo). The output is intended to be pasted into that file by a docs writer.

Output is **printed to chat**. After printing, ask the user whether to also
write it to a markdown file and, if yes, where.

## Inputs

Accept inputs in free-form prose from the user. Parse them into this block
and **confirm with the user before generating**:

```
from:    <commit SHA or tag — exclusive>
to:      <commit SHA / branch / "HEAD" — inclusive>
version: <user-supplied, e.g., 2026.5.2>
```

Rules:
- `from` is **exclusive**, `to` is **inclusive** (matches `git log from..to`).
- **Always ask the user for the version number** — do not auto-detect.
- Do **not** include a date in the output (header is `## v<version>`, no " - <date>").
- The user occasionally lists extra "exception commits" to force-include
  (commits outside the range). When they do, add them to the block as
  `include_extra: [sha, sha]` and feed them through the same pipeline as
  in-range commits. If they don't mention any, omit the field.
- Show the parsed block, wait for explicit confirmation or edits, then proceed.

## Pipeline

### 1. Gather commits

Run `git log --oneline <from>..<to>` to list candidate commits. Add any
`include_extra` SHAs to the list.

### 2. Fetch context for each commit

For every commit, gather **both**:
- The full commit message (`git log -1 --format='%H%n%s%n%n%b' <sha>`).
- The PR body, if the subject ends with `(#NNNN)`:
  `gh pr view <N> --json title,body`.

Run these fetches in parallel where possible. If `gh` fails for a given PR,
log a warning and proceed with the commit message alone.

### 3. Classify each commit

For every commit, decide:

**(a) Is it relevant to users?** Use your judgment. "Relevant" is broader
than the PR template's "user-facing changes" question — it includes
behaviour changes, new guardrails, bug fixes a user could hit, and so on,
even when the PR description says "no user-facing changes." **Always
exclude:**
- Tests (de-flakes, new test coverage, test infra).
- Refactors with no behaviour change.
- Callhome / telemetry payload changes.
- Internal infra / CI / build / agent-config commits.
- Version-bump commits.

**(b) Which bucket does it belong in?**
- **New feature** — a capability the user did not have before (new command,
  new flag enabling new behaviour, support for a new datatype/source/workflow).
- **Enhancement** — improvement to an existing capability (better UX,
  better performance, new guardrail on an existing flow, better error
  messages, broader applicability of an existing feature).
- **Bug fix** — corrects incorrect behaviour the user could hit.

When a commit could plausibly be either New feature or Enhancement, prefer
**Enhancement** unless it clearly introduces something new.

### 4. Phrase each entry

Reduce each commit to **what the user observes** — the symptom that was
fixed, the capability that was added, the behaviour that improved. State
that and nothing more. Prefer a **single sentence**; never exceed two. The
goal is a point a user can scan in one read, not a paragraph that
re-explains the change.

- **Describe the WHAT, never the WHY or HOW.** Leave out the root cause and
  the mechanism of the fix — those live in the PR, not the release note.
  Write "Fixed an issue where `import data` could hang when
  `--adaptive-parallelism-max` was set below the default `parallel-jobs`
  value." Do **not** append why it hung (a blocked connection-pool init) or
  how it was fixed (capping parallelism to the user's max). The user only
  needs the symptom they could hit.
- This rule applies to **every** bucket, not just bug fixes. A new feature
  says what the user can now do; an enhancement says what is now better —
  neither describes internals, data structures, or the code path that
  changed.
- Use plain text. **No markdown links.** No Hugo template tags. Code
  identifiers (commands, flags, SQLSTATEs, datatypes) go in backticks.
- Write for the user, not the contributor. Skip "we", "this PR", "root
  cause", file paths, function names, struct/field names.
- Start with the user-visible effect ("Fixed an issue where…", "Added…",
  "Improved…").

### 5. Render

Emit the block in this exact shape (omit any bucket with no entries):

```
## v<version>

### New features

- <entry>
- <entry>

### Enhancements

- <entry>
- <entry>

### Bug fixes

- <entry>
- <entry>
```

Use singular `### New feature` / `### Enhancement` / `### Bug fix` when a
bucket has only one entry (matches existing docs convention).

### 6. Show the exclusion list

After the release-notes block, print a separate section listing every
commit that was excluded, with the one-line reason:

```
---

**Excluded commits** (not user-facing):

- `<short-sha>` <subject> — <reason: tests | refactor | callhome | infra | version bump>
```

This lets the user spot anything that was mis-categorized in one glance.

### 7. Offer to write a file

After printing both blocks, ask the user whether to write the output to a
markdown file. If yes, ask for the path. Do not write a file by default.

## Anti-patterns

- **Don't** auto-detect or guess the version number — always ask.
- **Don't** include a date in the header.
- **Don't** emit Hugo-style relative links (`[x](../reference/...)`) or
  absolute docs URLs — plain text only.
- **Don't** silently drop a commit you're unsure about; include it under
  the closest bucket and the user can move it.
- **Don't** describe the implementation ("changed `cleanImportState` to
  pass `tableNames` instead of `nonEmptyNts`"). Describe the effect on
  the user.
- **Don't** explain the cause or the fix mechanism. Write "Fixed an issue
  where `import data` could intermittently fail against multi-node targets
  with a prepared-statement error" — not "the statement cache was keyed by
  backend PID, which isn't unique across nodes, so it's now keyed by the
  connection object." State only the symptom the user hit, in one sentence.
- **Don't** pad a point to a paragraph. If the entry runs past two
  sentences, it's almost certainly carrying cause or mechanism that belongs
  in the PR — cut back to the user-visible symptom.
- **Don't** skip the confirmation block, even when the prose ask looks
  unambiguous — one parse error wastes more time than one round-trip.
- **Don't** write the release-notes file without being asked.
