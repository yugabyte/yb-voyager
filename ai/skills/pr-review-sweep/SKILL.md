---
name: pr-review-sweep
description: Sweep recently updated open GitHub PRs that need an AI review — never reviewed, or with new commits since the last AI review (incremental re-review, at most once per 24h) — running the branch-review skill and posting findings with the post-pr-review skill; fully automated, no per-PR confirmation. Use when the user asks to "sweep open PRs", "review all open PRs", "run the PR review sweep", or when invoked by the scheduled daily PR-review routine. Also accepts explicit PR numbers to force a review of specific PRs.
---

# PR Review Sweep

Orchestrates the `branch-review` and `post-pr-review` skills across open PRs that need a review — never reviewed, or carrying new commits since the last AI review. Designed to run unattended (e.g. from a daily scheduled routine), so it never pauses for confirmation.

This skill does not duplicate the review logic — it selects PRs, then delegates each one to a reviewer subagent and a poster subagent that apply the two existing skills with a small set of documented overrides. `ai/skills/branch-review/SKILL.md` and `ai/skills/post-pr-review/SKILL.md` are the source of truth for how a review is produced and posted.

## Defaults

| Parameter | Default | Override |
|---|---|---|
| Recency window | PRs updated in the last **7 days** | `--days N` |
| Max PRs per run | **5** (oldest-updated first) | `--limit N` |
| Size cutoff | Skip PRs with **> 5000** changed lines (additions + deletions) | none (bypassed in explicit-PR mode) |
| Drafts | Skipped | none |
| Bot authors | Skipped | none |
| Re-review after new commits | **Incremental re-review**, at most once per 24h per PR, when the head has new commits since the last AI-reviewed commit | explicit-PR mode forces a full review |

## Modes

- **Default mode** (no args): select PRs per the filters above.
- **Explicit-PR mode** (`/pr-review-sweep 3512 3520`): review exactly the given PR numbers. **All filters are bypassed** — recency, draft, bot, size, and the already-reviewed skip (the user asked for these PRs, so re-review even if an `[AI]` review exists).
- `--days N` / `--limit N` adjust the defaults in default mode.

## Workflow

Copy this checklist and track progress in your response:

```
- [ ] Step 0: Prerequisites (gh auth, repo resolution)
- [ ] Step 1: Build the candidate list
- [ ] Step 2: Decide per PR — full review, incremental re-review, or skip
- [ ] Step 3: Apply the size cutoff and the per-run cap
- [ ] Step 4: Process each PR sequentially via two subagents (reviewer, then poster)
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

### Step 2: Decide per PR — full review, incremental re-review, or skip

Gather the PR's AI-review history:

```bash
# Latest AI review: submitted_at + the commit it reviewed
gh api --paginate "repos/$OWNER_REPO/pulls/$N/reviews" \
  --jq '[ .[] | select(.body // "" | startswith("[AI]")) ] | max_by(.submitted_at) | {submitted_at, commit_id}'

# Latest AI marker comment (zero-findings runs post these; no commit_id)
gh api --paginate "repos/$OWNER_REPO/issues/$N/comments" \
  --jq '[ .[] | select(.body // "" | startswith("[AI]")) ] | max_by(.created_at) | .created_at'

# Current head
gh pr view $N --json headRefOid --jq .headRefOid
```

Decision (take the most recent of the review/marker timestamps as `LAST_AI`):

- **No AI review or marker** → full review.
- **Head commit == last AI-reviewed `commit_id`** → `skipped (already reviewed, no new commits)`.
- **New commits since the last AI review, and `LAST_AI` more than 24h ago** → **incremental re-review** of the range `<last AI commit_id>..head` (see Step 4). If the last AI activity was only a marker (no `commit_id`) or the old commit is gone (force-push), do a full re-review instead.
- **New commits but `LAST_AI` within the last 24h** → `skipped (re-reviewed recently)` — it will be picked up by a later run.

In explicit-PR mode this logic is bypassed entirely (always a full review).

### Step 3: Size cutoff and cap

- If `additions + deletions > 5000`, record the PR as `skipped (too large: <X> lines)` and post **nothing** — the PR stays unmarked so a human (or an explicit-PR run) can still review it.
- Take the first **5** remaining PRs (oldest-updated first). Record the rest as `deferred (over per-run cap)` — they'll be picked up by the next run.

### Step 4: Process each PR sequentially via two subagents

Process PRs strictly one at a time (never in parallel — sequential runs keep `gh` rate usage and repo state predictable). For each PR spawn **two subagents in sequence**: a **reviewer** whose only job is the review, then a **poster** whose only job is publishing the findings. Do not merge them into one agent — a single agent juggling setup, review, and posting produces shallow reviews.

**4a. Reviewer subagent** — prompt of this shape:

```
Review the code changes for GitHub PR #<N> ("<title>") in <OWNER_REPO>.
Your ONLY job is the review — do not post anything. Work non-interactively.

Repo root: <REPO_ROOT>. Base branch: <BASE>.

1. Set up an ephemeral worktree of the PR head (never touch the main checkout):
     cd <REPO_ROOT>
     git fetch origin "refs/pull/<N>/head"
     git worktree add --detach /tmp/pr-review-sweep-<N> FETCH_HEAD
     git fetch origin <BASE>
   Then work inside /tmp/pr-review-sweep-<N>.

2. Read <REPO_ROOT>/ai/skills/branch-review/SKILL.md and apply it fully and
   exhaustively to this worktree, with ONE override: the comparison base is
   the PR's base branch, not main — use
   MERGE_BASE=$(git merge-base "origin/<BASE>" HEAD).
   HEAD is detached here; use "PR #<N>" as the branch name in the summary.
   If the `sem` tool is unavailable, omit the entity-modification tables.
   Depth matters more than speed: read the full changed files, run the
   skill's mechanical checks, and apply every lens. Do not stop at the
   first few findings.

[Incremental mode only — include when Step 2 chose incremental re-review:]
3. This PR was already reviewed at commit <LAST_COMMIT>. Focus the review on
   the new range: git diff <LAST_COMMIT>..HEAD. Also re-check whether the
   previously posted [AI] findings (fetch them via
   gh api repos/<OWNER_REPO>/pulls/<N>/comments) were addressed by the new
   commits; do not re-report a finding that is already posted and still
   anchored to unchanged code.

4. Cleanup: git worktree remove --force /tmp/pr-review-sweep-<N>

5. Your final message is the complete findings report (every Critical,
   Warning, Suggestion, and Question with file:line), plus the change
   summary. This report is the input to a posting step — do not truncate it.
```

**4b. Poster subagent** — give it the reviewer's findings report verbatim and a prompt of this shape:

```
Post this review to GitHub PR #<N> in <OWNER_REPO>. Work non-interactively.

Read <REPO_ROOT>/ai/skills/post-pr-review/SKILL.md and apply it, with ONE
override: skip Step 6 (user confirmation) — post immediately after building
and validating the payload. Zero findings → post the "no findings" marker
comment exactly as that skill's Step 3 specifies.
All other rules in that skill (COMMENT event, [AI][<Severity>] prefixes,
JSON via --input file, anchor verification) apply unchanged. If a 422 says
a comment line is not part of the diff, drop or re-anchor that finding and
retry once. Clean up /tmp/pr-review-<N>*.json afterwards.

Findings report:
<REVIEWER REPORT>

Your final message must be exactly one of:
  POSTED <review html_url> (<c> critical, <w> warning, <s> suggestion, <q> question)
  MARKER (no findings)
  FAILED <one-line reason>
```

Failure policy: a `FAILED` PR never aborts the sweep — record it and continue with the next PR. Do not retry within the run; a failed PR stays unmarked, so the next scheduled run retries it naturally.

### Step 5: Run summary

End with a table covering **every** PR considered, including filtered ones:

```
## PR Review Sweep — <date>

| PR | Title | Outcome |
|----|-------|---------|
| #3512 | Fix sequence restore ordering | posted — 1 critical, 2 warnings, 3 suggestions (<review-url>) |
| #3515 | Retry snapshot batches        | re-reviewed (incremental) — 1 warning (<review-url>) |
| #3520 | Refactor import batching      | marker — no findings |
| #3518 | Add Oracle RAC support        | skipped (too large: 8 412 lines) |
| #3501 | Bump deps                     | skipped (already reviewed, no new commits) |
| #3499 | Live migration docs           | failed (422: line not in diff after retry) |
```

Plus one line of totals: `N reviewed, R re-reviewed, M markers, K skipped, J failed, D deferred`.

## Anti-patterns

- **Never pause for user confirmation** — this skill's contract is unattended operation. The safety properties come from `post-pr-review`'s rules (non-blocking `COMMENT`, `[AI][<Severity>]` prefixes), not from a human gate.
- **Never collapse the reviewer and poster into one subagent.** A single context doing setup + review + posting has produced shallow reviews before; the dedicated reviewer is what protects depth.
- **Never modify or check out branches in the main working copy.** All PR checkouts go through ephemeral `/tmp` worktrees.
- **Never compare against hardcoded `main`.** Always diff against the PR's `baseRefName`.
- **Never mark a PR as reviewed without posting something.** The marker comment is what makes the sweep idempotent — skipping it makes the PR get fully re-reviewed every day.
- **Never run PR subagents in parallel.**
- **Never let one PR's failure abort the sweep.**
