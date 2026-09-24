---
name: git-rebase
description: >-
  Interactive git rebase workflow with conflict-by-conflict review. Use when the
  user asks to rebase a branch, rebase onto main, resolve rebase conflicts, or
  update a branch with upstream changes.
---

# Git Rebase

## Overview

This skill performs a git rebase interactively, pausing at every conflict to
show the user exactly what happened and waiting for explicit approval before
resolving. The user stays in control at every step.

## Pre-Rebase: Gather Context

First, update the local main branch so the rebase uses the latest upstream:

```bash
git fetch origin main
git branch -f main origin/main
```

Then run these commands in parallel to gather context:

1. `git branch --show-current` — current branch name
2. `git log --oneline main..HEAD` — commits on this branch (ahead of main)
3. `git log --oneline HEAD..origin/main` — commits on main not yet in branch
4. `git diff --stat origin/main...HEAD` — files changed on this branch

Present a summary to the user:

> **Branch:** `feature-branch`
> **Commits ahead of main:** N
> **Commits behind main:** M
> **Files changed on branch:** (list)

Ask the user to confirm before starting the rebase.

## Rebase Execution

Start the rebase:

```bash
git rebase origin/main 2>&1
```

### If rebase completes cleanly

Report success and show final `git log --oneline -5`.

### If a conflict occurs

**STOP immediately.** Do NOT resolve anything yet. Follow the conflict review
workflow below.

## Conflict Review Workflow

For EVERY conflict, follow this exact sequence:

### Step 1: Identify

Run:
- `git diff --name-only --diff-filter=U` — list conflicting files

Report which commit caused the conflict (shown in rebase output) and how far
along the rebase is (e.g., "Conflict at commit 8/22").

### Step 2: Show Each Conflict

For each conflicting file, search for conflict markers and show them with
surrounding context (10+ lines on each side):

```bash
grep -n "^<<<<<<<\|^=======\|^>>>>>>>" <file>
```

For every conflict hunk, explain clearly:
- **What HEAD (main) has** — and why (which PR or commit introduced it)
- **What the branch has** — and why
- **What the semantic difference is** — not just the textual diff

For binary files, show `git show <commit> --stat` and explain what each side
changed.

### Step 3: Propose a Resolution

For each conflict hunk, propose a specific resolution and explain the
reasoning. Use precise language:
- "Keep both imports — they are independent additions"
- "Take main's version — the branch change is incompatible with the new return type"
- "Merge both — keep main's API but add the branch's nil checks"

### Step 4: Wait for Approval

**CRITICAL: Ask the user for explicit approval before resolving.**

Use the AskQuestion tool or ask conversationally. Present options like:
- Resolve as proposed
- Resolve differently (ask for guidance)
- Skip this commit
- Abort the rebase

**Do NOT proceed until the user responds.**

### Step 5: Resolve and Continue

Only after approval:

1. Edit the file to resolve conflict markers
2. `git add <file>`
3. `GIT_EDITOR=true git rebase --continue 2>&1`

If the continue triggers another conflict, go back to Step 1.

## After Rebase

1. Run `git status --short` to check for any unstaged changes (e.g., from hooks)
2. Run `git log --oneline -5` to show the result
3. Report the final state: commits ahead/behind main

If there are unstaged changes from hooks, inform the user and ask whether to
commit or discard them.

## Rules

- **Never resolve a conflict without showing it to the user first.**
- **Never continue past a conflict without user approval.**
- **Never force-push** unless the user explicitly asks.
- If there are more than 5 conflicting files in a single commit, summarize
  first, then walk through each file one at a time.
- For binary file conflicts, default to keeping main's version unless the user
  says otherwise.
- If a commit becomes empty after conflict resolution, inform the user and ask
  whether to skip it or keep it.
