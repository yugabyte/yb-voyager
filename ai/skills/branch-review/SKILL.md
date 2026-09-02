---
name: branch-review
description: Review all code changes on the current branch compared to main, applying the hierarchical AGENTS.md engineering standards from each changed file's directory up to the repo root. Use when the user asks to review a branch, review changes, compare against main, do a code review, or check branch diff.
---

# Branch Code Review

Review all changes on the current checked-out branch compared to `main`.

## Workflow

### Step 1: Gather context

Run these commands to understand the scope of changes:

```bash
# Current branch name
git rev-parse --abbrev-ref HEAD

# Merge base (where this branch diverged from main)
MERGE_BASE=$(git merge-base main HEAD)

# List of changed files with status (Added/Modified/Deleted)
git diff --name-status $MERGE_BASE..HEAD

# Full diff for review
git diff $MERGE_BASE..HEAD

# Commit history on this branch
git log --oneline $MERGE_BASE..HEAD
```

If the diff is very large, review file-by-file using:

```bash
git diff $MERGE_BASE..HEAD -- <filepath>
```

#### AGENTS.md engineering standards (hierarchical)

Standards live in an `AGENTS.md` file in the directory they govern. They are authoring standards as well as review criteria, so the author's agent and this review read the same text.

**For each changed file** (added, modified, or renamed—use its path on the branch under review; for deleted files, use the path from the diff so ancestor rules still apply), treat **every** applicable standards file as mandatory review guidance:

1. Resolve the repository root: `REPO_ROOT=$(git rev-parse --show-toplevel)`.
2. Let `DIR` be the directory of the changed file relative to `REPO_ROOT` (e.g. for `yb-voyager/cmd/foo.go`, start at `yb-voyager/cmd`). If the file is at the repo root, `DIR` is `.`.
3. Walk **from `DIR` up to the repository root** (including `DIR` and the root). At **each** ancestor directory `D`, read `$REPO_ROOT/$D/AGENTS.md` if it exists and fold it into the review.
4. **Do not also read `$D/CLAUDE.md` or `$D/.cursor/BUGBOT.md`** — both are symlinks to the same `AGENTS.md`, and reading them duplicates the rules. Only if `$D/AGENTS.md` is absent (an older branch predating the migration) fall back to `$D/BUGBOT.md` or `$D/.cursor/BUGBOT.md`.
5. **Deduplicate** paths (the same file must not be applied twice if multiple changed files share an ancestor).
6. **Reading order**: Process standards from **repository root downward** toward the file’s directory (outer → inner). Outer files set broad product or repo-wide rules; inner files add package- or subtree-specific rules.
7. **Conflicts**: If two levels disagree, treat the **inner** (closer to the changed file) standard as **more specific** and take precedence for that file’s review. You must still honor **all non-conflicting** rules from outer levels.

When presenting findings, **cite which standards scope informed a finding** when it is not obvious (e.g. “Per `yb-voyager/cmd/AGENTS.md` …”).

You may collect all unique `AGENTS.md` paths that apply to the branch’s changed files with a small script or by walking paths logically; a one-liner is not required as long as the walk matches the rules above.

### Step 2: High-level change summary

**Always present this first**, before any detailed review. Analyze the commit messages and diffs to identify the distinct logical changes on the branch. Then produce a summary in this format:

```
## Change Summary

### 1. <Short name of logical change>
<1-2 sentence description of what this change accomplishes and why.>

**Files:**
- `path/to/file.go` — <what changed in this file for this logical change>

  Entity modifications (one row per entity):

  | Added | Modified | Deleted | Renamed |
  |-------|----------|---------|---------|
  | xyz   |          |         |         |
  |       | abc      |         |         |
  |       | def      |         |         |

- `path/to/other.go` — <what changed>

### 2. <Next logical change>
...
```

Rules for this summary:
- Group files by the **logical change** they belong to, not alphabetically.
- A single file may appear under multiple logical changes if it was touched for different reasons.
- Keep descriptions concrete: "adds a new `retryExport()` function" not "modifies the file".
- If there is only one logical change, still use this format (just one section).
- Use the sem tool to get changes at the entity level (to populate Entity Modification Table). `MERGE_BASE=$(git merge-base main HEAD) && sem diff --from $MERGE_BASE --to HEAD`.
- In the Entity Modification Table, list **one entity per row**. Do NOT comma-separate or concatenate multiple entities into a single cell. Each added/modified/deleted/renamed entity gets its own row, with empty cells in the other columns. 

### Step 3: Read and understand changed files

For each modified file, read the full current version to understand surrounding context — not just the diff lines. This helps catch issues like:
- Broken assumptions from nearby code
- Missing updates to related functions
- Inconsistent patterns within the file

### Step 4: Review each change

Evaluate every change against the standards hierarchy (4a) and the review lenses (4b). Two failure modes have caused this skill to miss important findings in the past — avoid both:

- **Only flagging the obvious bugs.** Swallowed errors, bad SQL, and nil derefs are easy to spot and this skill already finds them. The higher-value findings a human reviewer catches are usually about **scope, design, hot-path cost, simplification, and test coverage** — apply *every* lens below, not just Correctness/Security.
- **Reading the standards but not enforcing them.** Most previously-missed findings mapped to a rule that was *already loaded* from an applicable `AGENTS.md`. Loading a rule is not reviewing against it. Treat the loaded rules as an active checklist and scan the diff for a concrete violation of each one.

#### 4a. Apply the standards hierarchy as an active checklist

For each changed file, take the union of all `AGENTS.md` files from that file's directory through the repo root (Step 1). Walk each rule and actively look for a violation in the diff — do not just confirm you read it.

Because these are authoring standards, the author's agent may already have had them in context. That does **not** make the check redundant: standards are guidance, not enforcement, and a rule the author saw is still a rule the diff can violate.

**Turn each lexically-detectable rule into a search over the *added* (`+`) lines** — do not rely on reading comprehension alone. A rule you only *read* is a rule you will miss; a rule you *grep for* is one you enforce.

**Mandatory mechanical checks** — run these on the changed files, always (they are cheap and have caught real regressions):

- `go vet` on the changed packages (with the appropriate build tags) when it runs in a few seconds.
- Grep the added lines for: `utils.ErrExit` inside functions that return `error` (or inside new helper functions); new `TODO`/`FIXME`; commented-out code; `time.Sleep` in tests; single-value map reads (`m[k]` without `, ok`) where a missing key is meaningful; new or changed `json:` tags on persisted structs (→ raise the backward-compatibility question).


#### 4b. Review lenses

| Lens | What to check |
|------|---------------|
| **Correctness** | Logic errors, off-by-one, **inverted / negated conditions** (`!found`, a flipped `<`/`>`, a wrong `!`), nil/null handling, race conditions, edge cases. |
| **Scope & necessity** | Is every hunk needed *for this PR*? Flag speculative code, unused fields/params/abstractions added "for a later PR" with no caller, and **incidental changes unrelated to the stated goal** —  a needless new file, drive-by reformatting. AI-generated diffs routinely carry this noise; make the author justify each such change or drop it. |
| **Design & maintainability** | Especially for new packages/interfaces/abstractions: unnecessary indirection, **YAGNI** , **layering** (each layer does only its job), redundant concepts (two fields meaning the same thing), and two-sources-of-truth for one fact. |
| **Placement & naming** | Does each new function live in the right file/package? New feature-area helpers belong in their own file, not appended to an already-large command file; logic reachable from multiple commands belongs in a shared location. Long new blocks inside an existing function should be extracted. Names must be self-describing; several parallel maps/params keyed by the same thing usually want a struct. |
| **Error-handling doctrine** | Unexpected state must fail loudly: no warn-and-continue, no silent fallback to a weaker mechanism, no in-band sentinel values (empty string / zero / nil carrying two meanings). For every `log.Warn` + continue or defensive skip on a "shouldn't happen" branch, ask: should this be an error? Errors must be wrapped with enough context (operation, object) to act on. |
| **Hot-path performance** | *First decide whether the change is on a performance-critical path* — per-event / CDC / conflict-detection, the per-row import-data loop, per-tuple value conversion (see root `AGENTS.md` "Performance-critical (hot) paths").  |
| **Simplification** | Is there a materially simpler implementation?  |
| **Security** | Injection, hardcoded secrets, auth gaps, input validation. |
| **Tests** | New logic has tests that actually assert behavior; edge cases covered. **Test quality**: no fixed sleeps or timing-dependent assertions; count assertions on asynchronous work need justified bounds on *both* sides (a vacuous lower bound like `>= 0` asserts nothing; a missing upper bound misses over-triggering); no reads of state a concurrent process is still writing. **Coverage variants** the repo's `AGENTS.md` standards require (e.g. case-sensitive identifiers, partitioned tables, expression indexes) must be checked explicitly, not assumed. |
| **Documentation** | Public APIs documented; non-obvious gating/branching logic commented with *why* and *when it applies*; genuinely unclear concepts (new fields, enums, labels) explained — if you can't tell what a field is for, ask. |

Severity is not tied to lens: a hot-path regression or an inverted condition is Critical/Warning, not a nitpick. A concrete violation of a written BUGBOT rule defaults to **Warning**, not Suggestion. Design, scope, and clarity concerns that need author input but aren't defects go in the **Question** class (Step 5) — surface them, but don't invent a "bug" to justify them.

### Step 5: Present findings

Group findings by severity. Keep every finding pragmatic — no fluff. Each finding must use these bullet points, in this order:

- Problem/Suggestion: <the issue or recommendation, if any>
- Code line: <file:line of the failing/affected code>
- Failure Code Case: <the case that breaks, if any>
- Suggested Code Change: <the concrete fix; if any code is being suggested, give it in a fenced code block>

Omit a bullet's value only when it genuinely does not apply (e.g. "if any" parts); never pad with restatements, praise, or generic advice.

#### Critical — Must fix before merge
Issues that cause bugs, security holes, or data loss.

Format:
```
**[CRITICAL]** `file:line` — Brief description
- Problem/Suggestion: ...
- Code line: ...
- Failure Code Case: ...
- Suggested Code Change: ...
```

#### Warning — Should fix
Issues that may cause problems or hurt maintainability.

Format:
```
**[WARNING]** `file:line` — Brief description
- Problem/Suggestion: ...
- Code line: ...
- Failure Code Case: ...
- Suggested Code Change: ...
```

#### Suggestion — Nice to have
Style improvements, minor refactors, optional enhancements. **Suggestions must stay within the scope of the diff**: anchor them to code this branch adds or modifies, and suggest only changes the author could reasonably make in this PR. Do not suggest refactors or improvements to untouched code (pre-existing issues worth tracking belong in a Question, or are simply out of scope) — the goal is not to overwhelm the author.

Format:
```
**[SUGGESTION]** `file:line` — Brief description
- Problem/Suggestion: ...
- Code line: ...
- Failure Code Case: ...
- Suggested Code Change: ...
```

#### Question — Needs author input
Not a defect, but something where intent or necessity is unclear and the author should respond: a design/scope concern (unnecessary abstraction, YAGNI field, layering, naming), an incidental change with no obvious reason, a "why is this nil / why this dedupe / why a separate file" doubt, or a request to confirm untested source paths were manually verified. Many of a human reviewer's most valuable comments are questions — do not suppress them just because they aren't bugs. (Note: Questions are surfaced in this review for the user but are *not* posted to GitHub by the `post-pr-review` skill, which posts Critical/Warning/Suggestion only.)

Format:
```
**[QUESTION]** `file:line` — Brief description
- Problem/Suggestion: <the question, and why it matters>
- Code line: ...
- Suggested Code Change: <if you have a concrete alternative in mind>
```

### Step 6: Summary

End with a brief summary:

```
## Review Summary

- **Branch**: <branch-name>
- **Commits**: <count>
- **Files changed**: <count>
- **Findings**: <critical-count> critical, <warning-count> warnings, <suggestion-count> suggestions, <question-count> questions

### Overall assessment
<1-3 sentences: is this ready to merge, what are the key concerns?>
```

## Guidelines

- Load and apply **AGENTS.md** per changed file using the directory walk to repo root (Step 1), as an *active checklist* (Step 4a) — do not skip because the repo has many such files, and do not treat "I read it" as "I applied it."
- Apply **all** of the Step 4b lenses, not just Correctness/Security. The findings this skill has historically missed were scope, design, hot-path, simplification, and test-coverage issues — many of which were already covered by a loaded standard.
- Be specific — always reference file and line number.
- Suggest fixes, not just problems.
- Acknowledge good patterns and clean code briefly.
- **Raise questions liberally.** If intent, necessity, or a design choice is unclear — or a change looks incidental / tool-generated — file it as a **Question** (Step 5) rather than staying silent. A good question is often more valuable than a weak assertion.
- Always lead with the high-level change summary (Step 2) before any detailed findings.
