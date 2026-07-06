---
name: branch-review
description: Review all code changes on the current branch compared to main, applying hierarchical BUGBOT.md and .cursor/BUGBOT.md guidelines from each changed file's directory up to the repo root. Use when the user asks to review a branch, review changes, compare against main, do a code review, or check branch diff.
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

#### BUGBOT.md review guidelines (hierarchical)

Many repositories keep extra review rules in `BUGBOT.md` files. They may live directly in a directory as `BUGBOT.md` or under `.cursor/BUGBOT.md` (both conventions are valid—check **both** at each level).

**For each changed file** (added, modified, or renamed—use its path on the branch under review; for deleted files, use the path from the diff so ancestor rules still apply), treat **every** applicable BUGBOT file as mandatory review guidance:

1. Resolve the repository root: `REPO_ROOT=$(git rev-parse --show-toplevel)`.
2. Let `DIR` be the directory of the changed file relative to `REPO_ROOT` (e.g. for `yb-voyager/cmd/foo.go`, start at `yb-voyager/cmd`). If the file is at the repo root, `DIR` is `.`.
3. Walk **from `DIR` up to the repository root** (including `DIR` and the root). At **each** ancestor directory `D`, if either file exists under `$REPO_ROOT/$D`, read it and fold it into the review:
   - `$REPO_ROOT/$D/BUGBOT.md`
   - `$REPO_ROOT/$D/.cursor/BUGBOT.md`
4. **Deduplicate** paths (the same file must not be applied twice if multiple changed files share an ancestor).
5. **Reading order**: Process guidelines from **repository root downward** toward the file’s directory (outer → inner). Outer files set broad product or repo-wide rules; inner files add package- or subtree-specific rules.
6. **Conflicts**: If two levels disagree, treat the **inner** (closer to the changed file) guideline as **more specific** and take precedence for that file’s review. You must still honor **all non-conflicting** rules from outer levels.

When presenting findings, **cite which BUGBOT scope informed a finding** when it is not obvious (e.g. “Per `yb-voyager/cmd/.cursor/BUGBOT.md` …”).

You may collect all unique BUGBOT paths that apply to the branch’s changed files with a small script or by walking paths logically; a one-liner is not required as long as the walk matches the rules above.

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

Evaluate every change against the BUGBOT hierarchy (4a) and the review lenses (4b). Two failure modes have caused this skill to miss important findings in the past — avoid both:

- **Only flagging the obvious bugs.** Swallowed errors, bad SQL, and nil derefs are easy to spot and this skill already finds them. The higher-value findings a human reviewer catches are usually about **scope, design, hot-path cost, simplification, and test coverage** — apply *every* lens below, not just Correctness/Security.
- **Reading BUGBOT rules but not enforcing them.** Most previously-missed findings mapped to a rule that was *already loaded* from an applicable BUGBOT.md. Loading a rule is not reviewing against it. Treat the loaded rules as an active checklist and scan the diff for a concrete violation of each one.

#### 4a. Apply the BUGBOT hierarchy as an active checklist

For each changed file, take the union of all `BUGBOT.md` / `.cursor/BUGBOT.md` files from that file's directory through the repo root (Step 1). Walk each rule and actively look for a violation in the diff — do not just confirm you read it. Voyager rules that are easy to miss and *have been missed before*:

- `utils.ErrExit` (or a silent swallow) where a function should return an error to its caller — cmd / yb-voyager error-handling rules.
- Object names assembled by string concatenation instead of `sqlname` / `NameTuple` / `AsKey` — the "Object Names" rule.
- Missing test cases for **partitioned / multi-level tables** and **case-sensitive identifiers** — root + srcdb rules.
- Dead / unused code, or code (fields, functions, whole files) added for a *future* PR with no caller yet.
- Backward-compat surfaces (MSR JSON tags, assessment SQLite schema, callhome/YugabyteD payload version) changed non-additively.
- Layering: business logic landing in `cmd/`, or a lower layer (e.g. persist/metadb) doing work that belongs to a higher one.

**Turn each lexically-detectable rule into a search over the *added* (`+`) lines** — do not rely on reading comprehension alone. A rule you only *read* is a rule you will miss; a rule you *grep for* is one you enforce. After loading the rules, scan the diff's added lines for these tell-tale signatures and inspect each hit in context:

- `utils.ErrExit(` inside a **newly-added** leaf/helper function (a `func save…/get…/build…` that returns nothing) — flag it **even when the enclosing file already uses `ErrExit` pervasively** (e.g. a `cmd/` command file). Existing usage is not license to add more; the new helper should return a wrapped error.
- `.AsQualifiedCatalogName()` or `.String()` used to build a **map key** or a metaDB key from a `NameTuple`/`ObjectName` — should be the canonical key method (`Key()` / `ForKey()`), which round-trips through the name registry for case-sensitive identifiers.
- A new `func Test…` body that calls an **unexported** helper (lowercase first letter) instead of the exported entry point it is meant to cover.
- New exported symbols, fields, or files that nothing else references yet — `git -C "$WT" grep <symbol>` to confirm; zero hits outside the definition ⇒ speculative/dead code for a later PR.
- `UNION ALL`, reformatting, or renamed locals in a file whose stated purpose is unrelated to the change — incidental churn to question.

#### 4b. Review lenses

| Lens | What to check |
|------|---------------|
| **Correctness** | Logic errors, off-by-one, **inverted / negated conditions** (`!found`, a flipped `<`/`>`, a wrong `!`), nil/null handling, race conditions, edge cases. |
| **Scope & necessity** | Is every hunk needed *for this PR*? Flag speculative code, unused fields/params/abstractions added "for a later PR" with no caller, and **incidental changes unrelated to the stated goal** — e.g. a `UNION`→`UNION ALL` swap with no functional reason, a needless new file, drive-by reformatting. AI/Cursor-generated diffs routinely carry this noise; make the author justify each such change or drop it. |
| **Design & maintainability** | Especially for new packages/interfaces/abstractions: unnecessary indirection (a registry/factory/extra interface where a `switch` or direct call would do), **YAGNI** (seams/fields not exercised yet — recommend pulling them from the PR), **layering** (each layer does only its job), names that collide with domain terms (e.g. a field named `Role` next to DB roles), redundant concepts (two fields meaning the same thing), and two-sources-of-truth for one fact. |
| **Hot-path performance** | *First decide whether the change is on a performance-critical path* — per-event / CDC / conflict-detection, the per-row import-data loop, per-tuple value conversion (see root BUGBOT "Performance-Critical (Hot) Paths"). On those paths scrutinize per-iteration allocations, string building (`+` in a loop, or building a temp string before `WriteString`), and repeated map/JSON/regex/lookup work; expect a benchmark for non-trivial changes. Off hot paths, apply ordinary performance judgment. |
| **Simplification** | Is there a materially simpler implementation? Push aggregation/sorting/dedup into SQL instead of post-processing in Go; dedupe once at the end instead of on every append; reuse an existing helper instead of re-implementing. |
| **Security** | Injection, hardcoded secrets, auth gaps, input validation. |
| **Tests** | New logic has tests that actually assert behavior; edge cases covered. Prefer exercising the **exported/public API** over calling unexported helpers directly. **Cross-variant coverage**: sources (PG/Oracle/MySQL/YB), migration flows, partitioned/multi-level tables, case-sensitive names. If a touched code path can't be easily tested (e.g. Oracle), confirm the author *manually verified* it. Tests must clean up and restore any shared state (schemas, global vars). |
| **Documentation** | Public APIs documented; non-obvious gating/branching logic commented with *why* and *when it applies*; genuinely unclear concepts (new fields, enums, labels) explained — if you can't tell what a field is for, ask. |

Severity is not tied to lens: a hot-path regression or an inverted condition is Critical/Warning, not a nitpick. Design, scope, and clarity concerns that need author input but aren't defects go in the **Question** class (Step 5) — surface them, but don't invent a "bug" to justify them.

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
Style improvements, minor refactors, optional enhancements.

Format:
```
**[SUGGESTION]** `file:line` — Brief description
- Problem/Suggestion: ...
- Code line: ...
- Failure Code Case: ...
- Suggested Code Change: ...
```

#### Question — Needs author input
Not a defect, but something where intent or necessity is unclear and the author should respond: a design/scope concern (unnecessary abstraction, YAGNI field, layering, naming), an incidental change with no obvious reason, a "why is this nil / why this dedupe / why a separate file" doubt, or a request to confirm untested source paths were manually verified. Many of a human reviewer's most valuable comments are questions — do not suppress them just because they aren't bugs. (Note: these are surfaced in the review but are *not* auto-posted by the `post-pr-review` skill, which only posts Critical/Warning.)

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

- Load and apply **BUGBOT.md** / **.cursor/BUGBOT.md** per changed file using the directory walk to repo root (Step 1), as an *active checklist* (Step 4a) — do not skip because the repo has many such files, and do not treat "I read it" as "I applied it."
- Apply **all** of the Step 4b lenses, not just Correctness/Security. The findings this skill has historically missed were scope, design, hot-path, simplification, and test-coverage issues — many of which were already covered by a loaded BUGBOT rule.
- Be specific — always reference file and line number.
- Suggest fixes, not just problems.
- Acknowledge good patterns and clean code briefly.
- **Raise questions liberally.** If intent, necessity, or a design choice is unclear — or a change looks incidental / tool-generated — file it as a **Question** (Step 5) rather than staying silent. A good question is often more valuable than a weak assertion.
- Always lead with the high-level change summary (Step 2) before any detailed findings.
