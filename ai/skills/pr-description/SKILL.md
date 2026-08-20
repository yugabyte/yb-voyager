---
name: pr-description
description: >-
  Create and update GitHub pull request descriptions using the project PR
  template. Use when the user asks to create a PR, write a PR description,
  update a PR description, or manage pull requests.
---

# PR Description Management

## Overview

This skill creates and updates GitHub PR descriptions for the yb-voyager project.
Descriptions summarize the **entire PR diff as a whole** — not individual commits.

## Core rule: the shortest description that still explains the change

A PR description is skimmed in under a minute. Optimize for that, always.

- **Hard cap: ~150 words** for the whole description (the template's reference
  table doesn't count). Over budget means cut content, not reformat it.
- **Every section is 1-3 lines.** Prefer a couple of short bullets to a paragraph.
- **Simple language.** Plain words, active voice, present tense. Write "fixes a
  crash when a table has no primary key", not "addresses a suboptimal behavioral
  characteristic in the primary-key-less code path".
- **No filler.** Drop "this PR", "in order to", "it is worth noting that",
  "comprehensive", "robust", "leverage", "various", "as mentioned above".
- **No diff narration.** The reviewer can see the file list. Don't walk through
  files, functions, or commits.
- **Examples only when they earn their place.** One short snippet — a command, a
  config line, a before/after — when it is genuinely clearer than a sentence.
  At most one per PR, at most ~5 lines. Otherwise skip it.

## PR Description Template

Use the project's PR template file at `.github/PULL_REQUEST_TEMPLATE` as the base
structure for every PR description. Read that file each time to get the latest
section headings and reference tables — if the template is updated in the future,
your descriptions will automatically stay in sync.

### Section-by-section filling guidance

When populating the template, follow these instructions for each section:

- **Describe the changes in this pull request** — **2-4 short bullets, or 2-3
  sentences.** What was broken or missing, and what the PR does about it. Add the
  *why* only when it isn't obvious from the *what*. Mention a design decision only
  if a reviewer would otherwise question the approach. No commit-by-commit
  breakdown.

- **Describe if there are any user-facing changes** — One line per question that
  actually applies (command line, configuration, installation, reports). Show the
  new flag or config key literally instead of describing it. If nothing applies,
  write exactly "No user-facing changes." and nothing more.

- **How was this pull request tested?** — **1-3 lines.** Name the real tests or
  commands (`TestFooBar`, `make unit-tests`), and say plainly if something is only
  manually tested or still uncovered. No prose like "testing was performed to
  ensure correctness".

- **Does your PR have changes in callhome/yugabyted payloads?** — One line: "No."
  or "Yes — payload version bumped to N."

- **Does your PR have changes to on-disk structures that can cause upgrade issues?**
  — One line: "No." or "Yes — <structures>", naming them from the reference table
  at the bottom of the template.

## Example

Too long — narrates the diff, pads with filler, explains nothing new:

> ### Describe the changes in this pull request
> This PR introduces a comprehensive set of changes in order to improve the
> robustness of the CDC event handling code path. In `handleEvent`, we now
> leverage the error returned by `WaitUntilNoConflict` rather than discarding it.
> Various call sites in `event_processor.go` were updated accordingly, and the
> function signature was changed to return an error. It is worth noting that
> this makes the behavior consistent with the rest of the package.

Right size — one problem, one fix, one consequence:

> ### Describe the changes in this pull request
> `handleEvent` silently dropped the error from `WaitUntilNoConflict`, so an
> import kept applying events after a conflict wait failed. It now propagates the
> error, and the import fails fast instead of writing bad data.

An example earns its place when a sentence can't show the shape of the change:

> ### Describe if there are any user-facing changes
> New optional flag on `import data`:
> ```
> --on-conflict-wait-timeout 30s
> ```

## Workflow: Create a PR

When the user asks to create a PR:

1. **Gather context** — run these commands in parallel:
   - `git status` to check for uncommitted changes
   - `git log main..HEAD --oneline` to see all commits on the branch
   - `git diff main...HEAD --stat` to get a summary of changed files
   - `git diff main...HEAD` to get the full diff

2. **Analyze the full diff holistically** — understand the overall purpose of the
   changes as a unified body of work, not as individual commits.

3. **Draft the PR description** using the template above. Fill in each section
   based on the diff analysis, then run the "Trim pass" checklist below before
   showing it to anyone.

4. **Draft a concise PR title** — a short imperative sentence summarizing the change
   (e.g., "Add retry logic for failed CDC events").

5. **Present the title and description** to the user for review before creating.

6. **Create the PR** using `gh pr create`:
   ```bash
   git push -u origin HEAD
   gh pr create --title "the title" --body "$(cat <<'EOF'
   <filled template>
   EOF
   )"
   ```

7. Return the PR URL to the user.

## Workflow: Update a PR Description

When the user asks to update an existing PR description:

1. **Get the current PR** — determine which PR to update:
   - If the user provides a PR number/URL, use that.
   - Otherwise, use `gh pr view --json number,title,body,url` on the current branch.

2. **Get the current description** and show it to the user.

3. **Gather the full PR diff** (branch vs base, i.e. the entire PR — not just
   changes since the last description update):
   - `gh pr diff` to get the complete diff of the PR
   - `gh pr view --json commits` to see all commits on the branch

4. **Analyze the full PR diff holistically** and draft an updated description
   using the template. The description must reflect the totality of changes
   in the PR, as if writing it from scratch — length budget included. An update
   is a rewrite, not an append: never grow a description by tacking the newest
   changes onto the old text. Run the "Trim pass" checklist before showing it.

5. **Show the user exactly what will change** — present the proposed new description
   clearly, highlighting what's different from the current one. Use a format like:

   > Here's the updated PR description I'd like to apply:
   >
   > *(show full new description)*
   >
   > **Key changes from the current description:**
   > - *(bullet list of what changed and why)*
   >
   > Would you like me to apply this update?

6. **Wait for explicit user approval** before making any changes. Do NOT update
   the PR description without the user confirming.

7. **Apply the update** only after approval:
   ```bash
   gh pr edit <number> --body "$(cat <<'EOF'
   <filled template>
   EOF
   )"
   ```

8. Confirm the update was applied and show the PR URL.

## Writing Guidelines

- **Short beats complete**: A short description that gets read fully is worth more
  than a thorough one that gets skipped. When in doubt, cut.
- **Holistic, not granular**: Describe the PR as one cohesive change. Don't list
  commits or say "in commit X, we did Y".
- **Simple words**: Explain it the way you would to a teammate in chat. No
  marketing adjectives, no abstract noun where a verb works.
- **Why over what**: One clause of motivation beats a paragraph of mechanics.
- **Be specific in testing**: Name actual tests, commands, or scenarios — not
  just "tested manually".
- **Be honest about gaps**: If testing is incomplete, say so in a few words.
  Don't fabricate test coverage.

## Trim pass

Before showing any description to the user, delete:

- any sentence a reviewer could get from the file list or the diff
- any word from the filler list in "Core rule" above
- any example that only repeats what the prose already said
- any background the team already knows
- restated headings ("This section describes the changes...")

Then check: is the whole thing under ~150 words, and is every section 1-3 lines?
If not, cut again. Never pad a section to look thorough — "No user-facing
changes." is a complete answer.
