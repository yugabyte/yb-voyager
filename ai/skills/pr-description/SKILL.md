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

## PR Description Template

Use the project's PR template file at `.github/PULL_REQUEST_TEMPLATE` as the base
structure for every PR description. Read that file each time to get the latest
section headings and reference tables — if the template is updated in the future,
your descriptions will automatically stay in sync.

### Section-by-section filling guidance

When populating the template, follow these instructions for each section:

- **Describe the changes in this pull request** — Write a concise summary of what
  the PR does and why (3-8 sentences). Focus on the overall intent, the problem
  being solved, the approach taken, and key design decisions. Do NOT give a
  commit-by-commit breakdown.

- **Describe if there are any user-facing changes** — Answer the questions listed
  in the template's HTML comments (command line, configuration, installation,
  reports). If none apply, write "No user-facing changes."

- **How was this pull request tested?** — Be specific: mention whether existing
  tests cover the changes, any new unit tests added, manual testing done, and
  whether integration tests are needed. Cite actual test names or commands where
  possible.

- **Does your PR have changes in callhome/yugabyted payloads?** — Answer Yes/No.
  If yes, confirm the payload version was incremented.

- **Does your PR have changes to on-disk structures that can cause upgrade issues?**
  — Answer Yes/No. If yes, list which on-disk structures are affected, using the
  reference table at the bottom of the template.

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
   based on the diff analysis.

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
   in the PR, as if writing it from scratch.

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

- **Holistic, not granular**: Describe the PR as one cohesive change. Don't list
  commits or say "in commit X, we did Y".
- **Why over what**: Explain the motivation and design decisions, not just what
  files changed.
- **Be specific in testing**: Mention actual test names, commands run, or scenarios
  verified — not just "tested manually".
- **Be honest about gaps**: If testing is incomplete, say so. Don't fabricate
  test coverage.
- **Keep it concise**: Aim for clarity, not length. Each section should be
  meaningful, not padded.
