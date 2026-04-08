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

Evaluate every change against:

1. **BUGBOT.md hierarchy** — For each changed file, apply the union of all `BUGBOT.md` / `.cursor/BUGBOT.md` files from that file’s directory through the repository root, as described in Step 1. Call out violations or gaps against those rules explicitly.
2. **General criteria** (below).

| Area | What to check |
|------|---------------|
| **Correctness** | Logic errors, off-by-one, nil/null handling, race conditions, edge cases |
| **Security** | Injection, hardcoded secrets, auth gaps, input validation |
| **Performance** | Unnecessary allocations, N+1 queries, missing indexes, unbounded loops |
| **Style** | Naming, consistency with codebase conventions, dead code, magic numbers |
| **Tests** | New logic has tests, edge cases covered, tests actually assert behavior |
| **Documentation** | Public APIs documented, non-obvious logic commented, changelog updated if needed |

### Step 5: Present findings

Group findings by severity:

#### Critical — Must fix before merge
Issues that cause bugs, security holes, or data loss.

Format:
```
**[CRITICAL]** `file:line` — Brief description
> Explanation of the issue and suggested fix
```

#### Warning — Should fix
Issues that may cause problems or hurt maintainability.

Format:
```
**[WARNING]** `file:line` — Brief description
> Explanation and recommendation
```

#### Suggestion — Nice to have
Style improvements, minor refactors, optional enhancements.

Format:
```
**[SUGGESTION]** `file:line` — Brief description
> Rationale
```

### Step 6: Summary

End with a brief summary:

```
## Review Summary

- **Branch**: <branch-name>
- **Commits**: <count>
- **Files changed**: <count>
- **Findings**: <critical-count> critical, <warning-count> warnings, <suggestion-count> suggestions

### Overall assessment
<1-3 sentences: is this ready to merge, what are the key concerns?>
```

## Guidelines

- Load and apply **BUGBOT.md** / **.cursor/BUGBOT.md** per changed file using the directory walk to repo root (Step 1); do not skip because the repo has many such files.
- Be specific — always reference file and line number.
- Suggest fixes, not just problems.
- Acknowledge good patterns and clean code briefly.
- If unsure about intent, flag it as a question rather than an issue.
- Always lead with the high-level change summary (Step 2) before any detailed findings.
