---
name: pr-review-sweep
description: Sweep recently updated open GitHub PRs that don't yet have an AI review, and for each one run the branch-review skill and post the findings with the post-pr-review skill — fully automated, no per-PR confirmation. Use when the user asks to "sweep open PRs", "review all open PRs", "run the PR review sweep", or when invoked by the scheduled daily PR-review routine. Also accepts explicit PR numbers to force a review of specific PRs.
---

# PR Review Sweep

Orchestrates the `branch-review` and `post-pr-review` skills across open PRs that haven't received an AI review yet. Designed to run unattended (e.g. from a daily scheduled routine), so it never pauses for confirmation.

This skill does not duplicate the review logic — it selects PRs, then delegates each one to a subagent that applies the two existing skills with a small set of documented overrides. `ai/skills/branch-review/SKILL.md` and `ai/skills/post-pr-review/SKILL.md` are the source of truth for how a review is produced and posted.

## Defaults

| Parameter | Default | Override |
|---|---|---|
| Recency window | PRs updated in the last **7 days** | `--days N` |
| Max PRs per run | **5** (oldest-updated first) | `--limit N` |
| Size cutoff | Skip PRs with **> 5000** changed lines (additions + deletions) | none (bypassed in explicit-PR mode) |
| Drafts | Skipped | none |
| Bot authors | Skipped | none |
| Re-review after new commits | Never — once a PR has an AI review, it is skipped forever | explicit-PR mode forces a review |

## Modes

- **Default mode** (no args): select PRs per the filters above.
- **Explicit-PR mode** (`/pr-review-sweep 3512 3520`): review exactly the given PR numbers. **All filters are bypassed** — recency, draft, bot, size, and the already-reviewed skip (the user asked for these PRs, so re-review even if an `[AI]` review exists).
- `--days N` / `--limit N` adjust the defaults in default mode.

## Workflow

Copy this checklist and track progress in your response:

```
- [ ] Step 0: Prerequisites (gh auth, repo resolution)
- [ ] Step 1: Build the candidate list
- [ ] Step 2: Filter out PRs that already have an AI review
- [ ] Step 3: Apply the size cutoff and the per-run cap
- [ ] Step 4: Process each PR sequentially via a subagent
- [ ] Step 5: Print the run summary table
```

### Step 0: Prerequisites

```bash
gh auth status          # must be authenticated with repo scope
OWNER_REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)
REPO_ROOT=$(git rev-parse --show-toplevel)
```

If `gh` is missing or unauthenticated, stop and report — do not attempt anonymous API calls. The repository is always derived from `origin`; never hardcode it.

### Step 1: Build the candidate list

Skip this step in explicit-PR mode (the user-supplied numbers *are* the list — just confirm each exists and is open with `gh pr view <N> --json state`; warn and skip any that aren't open).

```bash
DAYS=7   # or --days override
gh pr list --state open --limit 100 \
  --json number,title,author,updatedAt,isDraft,baseRefName,additions,deletions \
  --jq "[ .[]
        | select(.isDraft | not)
        | select((.author.is_bot // false) | not)
        | select((.updatedAt | fromdateiso8601) >= (now - ${DAYS}*86400)) ]
        | sort_by(.updatedAt)"
```

Notes:
- `sort_by(.updatedAt)` ascending → **oldest-updated PRs are processed first**, so nothing starves under the per-run cap.
- The `fromdateiso8601`-based filter is portable (no `date -v` / `date -d` divergence between macOS and Linux).

### Step 2: Filter out PRs that already have an AI review

A PR counts as already reviewed if **either** of these returns a non-zero count:

```bash
# (a) A pull-request review whose body starts with "[AI]"
gh api --paginate "repos/$OWNER_REPO/pulls/$N/reviews" \
  --jq '[ .[] | select(.body // "" | startswith("[AI]")) ] | length'

# (b) An issue comment starting with "[AI]" (covers the zero-findings marker, see Step 4)
gh api --paginate "repos/$OWNER_REPO/issues/$N/comments" \
  --jq '[ .[] | select(.body // "" | startswith("[AI]")) ] | length'
```

Already-reviewed PRs are recorded as `skipped (already reviewed)` for the summary. New commits after an AI review do **not** trigger a re-review.

In explicit-PR mode this filter is bypassed entirely.

### Step 3: Size cutoff and cap

- If `additions + deletions > 5000`, record the PR as `skipped (too large: <X> lines)` and post **nothing** — the PR stays unmarked so a human (or an explicit-PR run) can still review it.
- Take the first **5** remaining PRs (oldest-updated first). Record the rest as `deferred (over per-run cap)` — they'll be picked up by the next run.

### Step 4: Process each PR sequentially via a subagent

Spawn **one subagent per PR**, strictly one at a time (never in parallel — sequential runs keep `gh` rate usage and repo state predictable). Use the Agent tool with a prompt of this shape, filling in the placeholders:

```
Review GitHub PR #<N> ("<title>") in <OWNER_REPO> and post the review. Work
non-interactively — never pause to ask for confirmation.

Repo root: <REPO_ROOT>. Base branch: <BASE>.

1. Set up an ephemeral worktree of the PR head (never touch the main checkout):
     cd <REPO_ROOT>
     git fetch origin "refs/pull/<N>/head"
     git worktree add --detach /tmp/pr-review-sweep-<N> FETCH_HEAD
     git fetch origin <BASE>
   Then work inside /tmp/pr-review-sweep-<N>.

2. Read <REPO_ROOT>/ai/skills/branch-review/SKILL.md and apply it to this
   worktree, with ONE override: the comparison base is the PR's base branch,
   not main — use MERGE_BASE=$(git merge-base "origin/<BASE>" HEAD).
   HEAD is detached here; use "PR #<N>" as the branch name in the summary.
   If the `sem` tool is unavailable, omit the entity-modification tables.

   Keep every finding pragmatic — no fluff. Each finding must use these
   bullet points, in this order:
     - Problem/Suggestion: <the issue or recommendation, if any>
     - Code line: <file:line of the failing/affected code>
     - Failure Code Case: <the case that breaks, if any>
     - Suggested Code Change: <the concrete fix>
   Omit a bullet's value only when it genuinely does not apply (e.g. "if any"
   parts); never pad with restatements, praise, or generic advice.

3. Read <REPO_ROOT>/ai/skills/post-pr-review/SKILL.md and apply it to post
   the findings to PR #<N>, with TWO overrides:
     a. Skip Step 6 (user confirmation) — post immediately after building
        and validating the payload.
     b. If zero Critical/Warning findings remain after filtering, do NOT
        abort. Instead mark the PR as reviewed by posting a single
        conversation comment:
          gh api --method POST "repos/<OWNER_REPO>/issues/<N>/comments" \
            -f body='[AI] Automated review — no Critical/Warning findings.'
   All other rules in that skill (COMMENT event, [AI] prefix, Critical +
   Warning only, JSON via --input file, anchor verification) apply unchanged.
   If a 422 says a comment line is not part of the diff, drop or re-anchor
   that finding and retry once.

4. Cleanup (always, even on failure):
     cd <REPO_ROOT>
     git worktree remove --force /tmp/pr-review-sweep-<N>
     rm -f /tmp/pr-review-<N>.json /tmp/pr-review-<N>-response.json

5. Your final message must be exactly one of:
     POSTED <review html_url> (<c> critical, <w> warning)
     MARKER (no critical/warning findings)
     FAILED <one-line reason>
```

Failure policy: a `FAILED` PR never aborts the sweep — record it and continue with the next PR. Do not retry within the run; a failed PR stays unmarked, so the next scheduled run retries it naturally.

### Step 5: Run summary

End with a table covering **every** PR considered, including filtered ones:

```
## PR Review Sweep — <date>

| PR | Title | Outcome |
|----|-------|---------|
| #3512 | Fix sequence restore ordering | posted — 1 critical, 2 warnings (<review-url>) |
| #3520 | Refactor import batching      | marker — no critical/warning findings |
| #3518 | Add Oracle RAC support        | skipped (too large: 8 412 lines) |
| #3501 | Bump deps                     | skipped (already reviewed) |
| #3499 | Live migration docs           | failed (422: line not in diff after retry) |
```

Plus one line of totals: `N reviewed, M markers, K skipped, J failed, D deferred`.

## Anti-patterns

- **Never pause for user confirmation** — this skill's contract is unattended operation. The safety properties come from `post-pr-review`'s rules (non-blocking `COMMENT`, `[AI]` prefix, Critical/Warning only), not from a human gate.
- **Never modify or check out branches in the main working copy.** All PR checkouts go through ephemeral `/tmp` worktrees.
- **Never compare against hardcoded `main`.** Always diff against the PR's `baseRefName`.
- **Never mark a PR as reviewed without posting something.** The marker comment is what makes the sweep idempotent — skipping it makes the PR get re-reviewed every day.
- **Never run PR subagents in parallel.**
- **Never let one PR's failure abort the sweep.**
