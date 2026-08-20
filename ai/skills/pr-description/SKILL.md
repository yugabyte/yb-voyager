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

- **Match the length to the PR** — see "Length budget" below. Spend the budget on
  distinct facts, never on longer sentences.
- **Short answers stay one line.** The callhome and on-disk questions, and the
  user-facing section when nothing applies, are one line each no matter how large
  the PR is. Testing is 1-3 lines.
- **Simple language.** Plain words, active voice, present tense. Write "fixes a
  crash when a table has no primary key", not "addresses a suboptimal behavioral
  characteristic in the primary-key-less code path".
- **One idea per sentence.** Aim for about 20 words. Don't chain facts together
  with commas, semicolons, or "and". Split them instead — a bullet that needs a
  semicolon is two bullets. Two plain sentences beat one clause-stacked one.
- **No filler.** Drop "this PR", "in order to", "it is worth noting that",
  "comprehensive", "robust", "leverage", "various", "as mentioned above".
- **No diff narration.** The reviewer can see the file list. Don't walk through
  files, functions, or commits.
- **Examples only when they earn their place.** One short snippet — a command, a
  config line, a before/after — when it is genuinely clearer than a sentence.
  At most one per PR, at most ~5 lines. Otherwise skip it.

### Length budget

| PR size | "Describe the changes" | Whole description |
| :-- | :-- | :-- |
| Small — one fix, 1-3 files | 2-3 bullets, or 2-3 sentences | ~100 words |
| Typical — one feature or fix, a handful of files | 3-5 bullets | ~200 words |
| Large — new feature, refactor, or several subsystems | one-sentence lede + one bullet per reviewable piece (usually 6-10) | ~350 words |

**Big PRs get more lines, not longer lines.** For a large PR:

1. Open with one sentence saying what the PR delivers as a whole.
2. Then one bullet per independently reviewable piece, one line each. A 15-file
   PR earns 8 bullets; it never earns 8 paragraphs.
3. Group the bullets under bold labels once there are more than about six.
4. Name anything deliberately left for a follow-up, in one line.

If a PR genuinely can't be explained in ~350 words, say so and suggest splitting
it — don't quietly write 800.

## PR Description Template

Use the project's PR template file at `.github/PULL_REQUEST_TEMPLATE` as the base
structure for every PR description. Read that file each time to get the latest
section headings and reference tables — if the template is updated in the future,
your descriptions will automatically stay in sync.

### Section-by-section filling guidance

When populating the template, follow these instructions for each section:

- **Describe the changes in this pull request** — Size it from the length budget
  above: 2-3 bullets for a small PR, up to ~10 for a large one. Say what was
  broken or missing and what the PR does about it. Add the *why* only when it
  isn't obvious from the *what*. Mention a design decision only if a reviewer
  would otherwise question the approach. No commit-by-commit breakdown.

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
> `handleEvent` silently dropped the error from `WaitUntilNoConflict`. An import
> then kept applying events after a conflict wait had failed. It now propagates
> the error, so the import fails fast instead of writing bad data.

An example earns its place when a sentence can't show the shape of the change:

> ### Describe if there are any user-facing changes
> New optional flag on `import data`:
> ```
> --on-conflict-wait-timeout 30s
> ```

A large PR — a lede, then one line per reviewable piece, still scannable:

> ### Describe the changes in this pull request
> Adds resumable data export to the fall-back flow.
>
> - **Export** — writes a per-tablet checkpoint after each batch.
> - **Restart** — resumes from the last checkpoint instead of re-exporting the
>   whole snapshot.
> - **State** — new `export_checkpoint` table in MetaDB. It is created lazily, so
>   existing export dirs keep working.
> - **CLI** — `export data --resume` replaces `--restart-if-interrupted`. The old
>   flag stays as a hidden alias.
> - **Not in this PR** — resumption for `export data from target` (DB-1234).

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

   `gh pr edit` can fail on this repo with a GraphQL error about Projects
   (classic) being deprecated. The edit does not go through when that happens.
   Fall back to the REST API, and check the body afterwards either way:

   ```bash
   gh api -X PATCH repos/yugabyte/yb-voyager/pulls/<number> -F body=@body.md
   gh api repos/yugabyte/yb-voyager/pulls/<number> --jq '.body'
   ```

8. Confirm the update was applied and show the PR URL.

## Writing Guidelines

- **Short beats complete**: A short description that gets read fully is worth more
  than a thorough one that gets skipped. When in doubt, cut.
- **Holistic, not granular**: Describe the PR as one cohesive change. Don't list
  commits or say "in commit X, we did Y".
- **Simple words**: Explain it the way you would to a teammate in chat. No
  marketing adjectives, no abstract noun where a verb works.
- **Low density**: One fact per sentence. A reader should never have to unpack a
  sentence twice. Short sentences are what make a short description readable —
  cramming the same content into fewer, denser lines defeats the point.
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

Then split: any sentence carrying two facts becomes two sentences, and any bullet
with a semicolon becomes two bullets.

Then check it against the length budget for this PR's size. If it's over, cut
again — a large PR is a licence for more bullets, not for padding. Never pad a
section to look thorough: "No user-facing changes." is a complete answer.
