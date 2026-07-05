---
name: manual-test-sweep
description: Sweep recently updated open GitHub PRs that don't yet have an AI manual-test comment, and for each one run the manual-test-voyager skill (PR-scoped test plan + applicability triage + feasible execution) in a subagent, then post the summary as a single PR conversation comment. Fully automated, no per-PR confirmation. Use when the user asks to "sweep PRs for manual testing", "run the manual-test sweep", "manually test all open PRs", or when invoked by the scheduled manual-test routine. Also accepts explicit PR numbers.
---

# Manual Test Sweep

Orchestrates the `manual-test-voyager` skill across open PRs that haven't received an AI manual-test summary yet. Designed to run unattended (e.g. from a scheduled routine), so it never pauses for confirmation. Mirrors `pr-review-sweep`, but delegates to `manual-test-voyager` and posts a single **PR conversation comment** (not inline line-anchored review comments — manual-test findings are about runtime behavior, not specific lines).

This skill does not duplicate the testing logic — it selects PRs, then delegates each to a subagent that applies `ai/skills/manual-test-voyager/SKILL.md` (the source of truth for how a PR is manually tested).

## Idempotency marker (read this first)

Every comment this sweep posts begins with the literal marker **`[AI-manual-test]`**. A PR is "already swept" iff an issue comment starting with that exact marker exists.

> The marker is deliberately **`[AI-manual-test]`, not `[AI]`**. `pr-review-sweep` treats any issue comment `startswith("[AI]")` as "already reviewed"; `[AI-manual-test]` does **not** start with `[AI]` (4th char is `-`, not `]`), so the two sweeps stay independent — neither mistakes the other's comment for its own. Do not change this to `[AI]…` or the two sweeps will cross-suppress each other.

## Defaults

| Parameter | Default | Override |
|---|---|---|
| Recency window | PRs updated in the last **7 days** | `--days N` |
| Max PRs per run | **5** (oldest-updated first) | `--limit N` |
| Size cutoff | Skip PRs with **> 5000** changed lines | none (bypassed in explicit-PR mode) |
| Depth | **plan-only** (planning + applicability + static findings; no DB/build) | `--execute` (add feasible offline execution), `--execute --live` (add live) |
| Drafts | Skipped | none |
| Bot authors | Skipped | none |
| Re-sweep after new commits | Never — once a PR has an `[AI-manual-test]` comment it is skipped forever | explicit-PR mode forces a re-run |

**Why plan-only by default:** full execution provisions Docker DBs, builds the branch binary, and runs migrations; live additionally needs a global installer/Debezium build that cannot be parallelized. That is too heavy and too stateful for an unattended default. In practice most PRs have no offline runtime surface anyway (test tooling, unwired libraries, live-only internals), so planning + applicability triage is the high-value default. Use `--execute` for a heavier, interactive or dedicated run.

## Modes

- **Default mode** (no args): select PRs per the filters above; run each plan-only.
- **Explicit-PR mode** (`/manual-test-sweep 3643 3642`): sweep exactly those PRs. **All filters bypassed** (recency, draft, bot, size, already-swept) — the user asked for them.
- `--days N` / `--limit N` adjust default-mode selection. `--execute` / `--execute --live` raise depth (applies in either mode).

## Workflow

Copy this checklist and track progress in your response:

```
- [ ] Step 0: Prerequisites (gh auth, repo resolution)
- [ ] Step 1: Build the candidate list
- [ ] Step 2: Filter out PRs that already have an [AI-manual-test] comment
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

Skip in explicit-PR mode (the supplied numbers *are* the list — confirm each is open with `gh pr view <N> --json state`; warn and skip any that aren't).

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

`sort_by(.updatedAt)` ascending → oldest-updated first, so nothing starves under the cap. The `fromdateiso8601` filter is portable across macOS/Linux (no `date -d`/`date -v` divergence).

### Step 2: Filter out already-swept PRs

A PR is already swept iff:

```bash
gh api --paginate "repos/$OWNER_REPO/issues/$N/comments" \
  --jq '[ .[] | select(.body // "" | startswith("[AI-manual-test]")) ] | length'
```

returns non-zero. Record those as `skipped (already swept)`. New commits after a sweep do **not** trigger a re-run (explicit-PR mode does). Bypassed entirely in explicit-PR mode.

### Step 3: Size cutoff and cap

- If `additions + deletions > 5000`, record `skipped (too large: <X> lines)` and post **nothing** (stays unmarked for a human / explicit run).
- Take the first **5** remaining (oldest-updated first). Record the rest as `deferred (over per-run cap)`.

### Step 4: Process each PR sequentially via a subagent

Spawn **one subagent per PR, strictly sequential** (never parallel — full-execution runs share Docker containers, ports, and the global `/opt/yb-voyager` install; parallel runs corrupt each other. Even plan-only stays sequential for predictable `gh` usage). Use the Agent tool with this prompt shape:

```
Manually test GitHub PR #<N> ("<title>") in <OWNER_REPO> and post a summary
comment. Work non-interactively — never pause for confirmation.

Repo root: <REPO_ROOT>. Base branch: <BASE>. Depth: <plan-only | execute | execute+live>.

1. Read <REPO_ROOT>/ai/skills/manual-test-voyager/SKILL.md and its references/ —
   that skill defines the method.

2. Get the change: `gh pr diff <N>` and `gh pr view <N> --json files,title,body`.

3. Run manual-test-voyager Phase 1 (test planning + applicability). Classify
   applicability: OFFLINE-EXECUTABLE | LIVE-ONLY | NO-RUNTIME-SURFACE | INFRA/TEST-ONLY.
   Produce the test plan (scenarios + expected-outcome oracles) and any static
   risk findings.

   IF depth is plan-only: stop after planning. Do NOT provision DBs or build.

   IF depth is execute (and applicability is OFFLINE-EXECUTABLE): set up an
   ephemeral worktree of the PR head, build the binary (scripts/build-voyager.sh),
   provision an ISOLATED env (scripts/provision-dbs.sh with UNIQUE container
   names/ports: PG_CONTAINER=mtv-<N>-pg YB_CONTAINER=mtv-<N>-yb PG_PORT=<pick>
   YB_PORT=<pick>), execute the offline scenarios under `timeout`, apply the
   validation oracles, then teardown (scripts/teardown.sh with the same names).
     - ephemeral worktree: `git fetch origin refs/pull/<N>/head && git worktree
       add --detach /tmp/manual-test-sweep-<N> FETCH_HEAD`; work inside it, but
       read the skill from <REPO_ROOT> (worktrees don't carry uncommitted skill files).
   IF depth is execute+live AND applicability is LIVE-ONLY: additionally do the
   installer build + live flow per the runbook (heavy; only when explicitly enabled).

4. Post exactly ONE PR conversation comment via:
     gh api --method POST "repos/<OWNER_REPO>/issues/<N>/comments" --input <file>
   Build the body in a file (never inline JSON). The body MUST:
     - start with the literal marker `[AI-manual-test]`
     - use the comment template in this skill (the three buckets)
     - end with the signature block:
           ---
           _automated · Claude Code (Opus 4.8)_
   Always post a comment even when nothing was executed / out of scope — the
   comment is what makes the sweep idempotent.

5. Cleanup (always, even on failure): remove any worktree
   (`git worktree remove --force /tmp/manual-test-sweep-<N>`) and any mtv-<N>-*
   containers (`scripts/teardown.sh`). Remove temp files.

6. Final message must be exactly one of:
     POSTED <comment html_url> (<applicability>; <verified> verified, <needs> needs-looking)
     FAILED <one-line reason>
```

Failure policy: a `FAILED` PR never aborts the sweep — record it and continue. Do not retry within the run; a failed PR stays unmarked so the next run retries it.

### Step 5: Run summary

End with a table covering **every** PR considered, including filtered ones:

```
## Manual Test Sweep — <date> (<depth>)

| PR | Title | Applicability | Outcome |
|----|-------|---------------|---------|
| #3643 | NUM_EVENT_CHANNELS warnings | LIVE-ONLY | posted — plan + 2 static findings (<url>) |
| #3614 | schemasnapshot library      | NO-RUNTIME-SURFACE | posted — out of scope, unit-tier (<url>) |
| #3644 | Fix dockerfile delve        | INFRA/TEST-ONLY | posted — out of scope (<url>) |
| #3512 | Big refactor                | — | skipped (too large: 8 412 lines) |
| #3501 | Bump deps                   | — | skipped (already swept) |
```

Plus one totals line: `N posted, K skipped, J failed, D deferred`.

## Comment template

The single conversation comment the subagent posts:

```
[AI-manual-test] Manual test summary — <plan-only | plan + offline execution | plan + live execution>

**Applicability:** <OFFLINE-EXECUTABLE | LIVE-ONLY | NO-RUNTIME-SURFACE | INFRA/TEST-ONLY>
**Affected surface:** <commands / flags / config keys / flows / commonly-impacted areas>

**✅ Tested & verified:** <scenarios executed and passed, with the oracle each satisfied> — or "none executed (plan-only; see plan below)".
**⚠️ Needs looking into:** <static or dynamic findings, each one line> — or "none".
**🚫 Out of scope:** <what this skill can't cover for this PR and why (live-only / no runtime surface / FF-FB / Oracle-MySQL)>.

<Top test-plan scenarios (type · scenario · expected oracle), or "full plan omitted for brevity">

<if executed> _Env: <docker images, offline/live, binary build method>._ </if>

---
_automated · Claude Code (Opus 4.8)_
```

Rules for the comment:
- Starts with `[AI-manual-test]` exactly once. Self-contained (the GitHub reader has no chat context): state what was tested, what wasn't, and why.
- No `REQUEST_CHANGES`/approval semantics — this is a plain conversation comment, informational only.
- Always append the `---\n_automated · <agent>_` signature block (substitute the actual model/harness if not Opus 4.8) — required for autonomously-authored PR comments.
- No emojis beyond the three bucket markers.

## Anti-patterns

- **Never pause for confirmation** — the contract is unattended operation.
- **Never run PR subagents in parallel** — full-execution runs share Docker/ports/the global install.
- **Never use the `[AI]` marker** — it collides with `pr-review-sweep`. Always `[AI-manual-test]`.
- **Never mark a PR as swept without posting a comment** — the comment is the idempotency record.
- **Never do a global installer build in default (plan-only) mode** — that's `--execute --live` territory only, and even then sequential.
- **Never leave containers or worktrees behind** — teardown in the subagent's cleanup step, always.
- **Never let one PR's failure abort the sweep.**
