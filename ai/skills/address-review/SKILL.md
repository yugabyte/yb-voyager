---
name: address-review
description: >-
  Work through reviewer feedback on your own open pull request — inline review
  threads, review-level bodies, and PR conversation comments — one item at a
  time, with user sign-off before any edit, reply, or push. Session-resumable
  via a tracking file, so an interrupted session picks up exactly where it left
  off. Use this whenever the user says "address review comments", "respond to
  the PR review", "work through reviewer feedback", "reply to the comments on my
  PR", "what's left on my PR", or "pick up where I left off on the review" — and
  also whenever they mention a PR number alongside "review", "comments",
  "threads", or "feedback", even without the word "address". This is the inbound
  direction (feedback *on* your PR); for reviewing someone else's changes use
  `branch-review` and `post-pr-review` instead.
---

# Address Review

A workflow for answering reviewer feedback on a PR you authored: fetch every
surface where feedback can hide, triage it, decide what is worth acting on,
make the changes, and reply — one thread at a time, with the user approving
each step.

## Where this sits among the PR skills

| Skill | Direction | Purpose |
|---|---|---|
| `branch-review` | outbound | Review changes on a branch against `main` |
| `post-pr-review` | outbound | Post those findings to a PR as `[AI]` comments |
| `pr-review-sweep` | outbound | Run the two above across many open PRs |
| **`address-review`** | **inbound** | **Respond to feedback left on your own PR** |

Because `post-pr-review` prefixes everything it writes with `[AI] `, this skill
can tell machine-generated findings from a human's and treat them differently —
see the triage step.

## Invocation

```
/address-review <branch> [<pr-number>]
```

If `pr-number` is omitted, derive it:

```bash
gh pr view <branch> --json number -q .number
```

This only works once the branch is pushed. If it fails, ask the user for the
number rather than guessing.

## Preconditions

Check these first and surface problems plainly instead of continuing quietly:

1. `gh auth status` — must pass
2. Inside a git repo with an `origin` remote
3. PR is open: `gh pr view <pr-number> --json state -q .state`
4. Working tree state: warn on uncommitted changes, but don't block

Capture the repo slug for later API calls:

```bash
gh repo view --json nameWithOwner -q .nameWithOwner
```

## Step 0 — Check for an existing session

Before anything else, look for a tracking file for this PR. If one exists this
is a **resume** — jump to the Resume section.

**Location:** `<repo-root>/.git/pr-reviews/pr-<number>.md`

Keeping the file inside `.git/` rather than in the working tree means it is
never staged by accident, needs no exclude entry, and — importantly — survives
the worktree being removed at cleanup, so a later session can still see what was
already answered.

```bash
GITDIR=$(git rev-parse --path-format=absolute --git-common-dir)
mkdir -p "$GITDIR/pr-reviews"
```

Use `--git-common-dir`, not `.git`. Inside a linked worktree `.git` is a *file*,
not a directory, so writing to `<worktree>/.git/info/exclude` fails with
"Not a directory". `--git-common-dir` resolves to the real shared git directory
from anywhere.

## Step 1 — Worktree, only if you'll be editing

If the session is likely to involve code changes, create an isolated worktree so
the user's checkout is untouched. If the user only wants to read and reply,
skip it — spinning up a worktree to post two replies is pure overhead.

Record the head SHA either way; it detects force-pushes between sessions:

```bash
git rev-parse HEAD
```

## Step 2 — Fetch every feedback surface

Reviewer feedback lives in three different places, and a PR can be blocked by
any of them. Fetching only inline threads is the most common way to miss the
thing that actually matters — an approval conditioned on one small change often
appears *only* in a review body.

| Surface | GraphQL field | What lives there |
|---|---|---|
| Inline threads | `reviewThreads` | Line-anchored comments, the bulk of feedback |
| Review bodies | `reviews` | `APPROVED` / `CHANGES_REQUESTED` states and summary text |
| PR comments | `comments` | Conversation-tab remarks, CI and bot notices |

Use GraphQL, not the REST `/pulls/{n}/comments` endpoint — REST gives no thread
structure, no `isResolved`, and no `isOutdated`.

```graphql
query($owner:String!, $repo:String!, $prNumber:Int!, $after:String) {
  repository(owner:$owner, name:$repo) {
    pullRequest(number:$prNumber) {
      author { login }
      reviewDecision
      reviewThreads(first:100, after:$after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id isResolved isOutdated isCollapsed
          path line startLine subjectType
          comments(first:50) {
            nodes {
              databaseId author { login } body createdAt
              originalLine originalStartLine diffHunk
              originalCommit { oid }
            }
          }
        }
      }
      reviews(first:50) {
        nodes { author { login } state body submittedAt }
      }
      comments(first:50) {
        nodes { databaseId author { login } body createdAt }
      }
    }
  }
}
```

```bash
gh api graphql -f query='<query>' -f owner=<owner> -f repo=<repo> -F prNumber=<number>
```

If `reviewThreads.pageInfo.hasNextPage` is true, re-run passing `-f after=<endCursor>`
and merge the nodes. Most PRs fit in one page, but a long-running branch will not,
and silently dropping page two means silently dropping feedback.

### Classify each thread — first matching rule wins

1. `isResolved == true` → `resolved` → skip
2. In tracking file as `status: posted`, with no comment newer than `posted_at` → `already-handled` → skip
3. Last comment body starts with `[AI] ` → `ai-finding` → show, but flag as machine-generated
4. Exactly one comment, no replies → `needs-attention`
5. Last comment's author is the PR author → `waiting-on-reviewer`
6. Otherwise → `needs-attention`

`isOutdated == true` is **not** a skip signal — the code moved, the question
usually didn't. Show those threads annotated "line has moved".

Treat `ai-finding` threads as real but lower-stakes: no human is waiting on a
reply, and a wrong machine suggestion can simply be declined with a one-line
reason. Say so when presenting one, so the user can spend their attention on
the human feedback.

### Triage table

Print this before the loop, covering all three surfaces:

```
 # | surface | file:line        | author   | classification      | tracking
---+---------+------------------+----------+---------------------+----------
 1 | thread  | src/foo.go:42    | alice    | needs-attention     | pending
 2 | thread  | cmd/root.go:108  | bob      | waiting-on-reviewer | pending
 3 | thread  | src/bar.go:17    | claude   | ai-finding          | pending
 4 | review  | —                | alice    | APPROVED            | "LGTM if you rename the flag"
 5 | comment | —                | ci-bot   | informational       | —
```

Then ask: "Which of these do you want to work through? (numbers, 'all', or
'needs-attention')"

Call out a `CHANGES_REQUESTED` review or a conditional approval explicitly —
that is the gate on merging, and it is easy to miss in a list of line comments.

## Step 3 — Per-thread loop

Work one thread at a time. Never advance without the user confirming the
current one is done or explicitly skipped.

### 3a. Show what the reviewer actually saw

Getting this right matters more than it looks. A thread's anchor can be in one
of three states, and the wrong handling shows the user unrelated code with full
confidence — worse than showing nothing.

**Live anchor** (`line` is non-null): read the current file. For a multi-line
comment, `startLine`..`line` is the range; show all of it, not just the last line.

**Outdated anchor** (`line` is null, only `originalLine` survives): do **not**
read the current file at `originalLine`. That number indexes an older commit and
now points somewhere unrelated. Instead read the file as it was:

```bash
git show <originalCommit.oid>:<path> | sed -n '<originalStartLine or originalLine>,<originalLine>p'
```

The comment's `diffHunk` carries the same context if the commit isn't fetched
locally (`git fetch origin <sha>` usually fixes that). Then find where the code
went — `git log -S '<distinctive line>'` or a grep for a stable fragment — and
show the user both: what was commented on, and where it lives now.

**File-level comment** (`subjectType == FILE`): there is no line at all. Show
the file's role in the diff and the comment on its own.

Format comments like this, summarizing earlier ones in a long thread and
printing the latest in full:

```
alice (2026-05-19):
> The return value here seems off — shouldn't we clamp this?
```

```go
// src/foo.go:40-42 (current)
func foo() {
    x := bar()
    return x + 1   // <- comment anchored here
}
```

### 3b. Form a view before asking

Don't jump straight to "what do you want to do?". Read the code and say whether
the reviewer is right, partly right, or has missed something — then ask. The
user invoked a skill to get judgment, not a comment-forwarding service.

Reviewers are often right, but not always, and two reviewers on the same PR can
ask for opposite things. When feedback looks mistaken, say so with the reason
and let the user decide; implementing a change you believe is wrong helps nobody.
If the project has the `superpowers:receiving-code-review` skill available, its
guidance on evaluating feedback applies directly here.

Watch for a thread that contains a GitHub ```suggestion block — that is a
literal proposed replacement and can be applied verbatim once the user agrees,
which is faster and less error-prone than retyping it.

### 3c. Decide

| User says | Action |
|---|---|
| "fix it" / describes a change | Go to 3d |
| "skip" | `status: skipped`, move on |
| "not sure" | `status: drafting`, park it, return at end of session |
| "resolve it" | Post reply (3f), then run the resolve mutation, `status: posted` |
| "wontfix" / "push back" | Draft a reply giving the reasoning, go to 3f |

### 3d. Code changes

Make the edits, then show the diff before staging:

```bash
git diff
```

Ask: "Does this look right? Stage and continue?" Don't stage without an explicit
yes.

### 3e. Commit decision

Ask whether to commit now or defer to the end. If committing now, record the SHA
against the thread — the reply text usually wants to cite it.

### 3f. Draft the reply

Show the draft clearly and iterate until the user approves it:

```
Draft reply:
---
Fixed in abc123 — the return value now clamps to [0, max] before returning.
---
Good to go, or want to change anything?
```

Keep replies short and specific. "Done" tells a reviewer nothing; naming the
commit and what changed lets them verify without re-reading the diff. If you
pushed back, give the reason in the reply rather than leaving the thread silent.

### 3g. Persist

Write `draft_reply`, `status: ready`, `related_commits`, `last_comment_at`, and
refresh `last_session_at`. Confirm and move on.

## Step 4 — Post

Summarize everything at `status: ready` and ask for one explicit confirmation
before posting anything.

```bash
gh api --method POST \
  /repos/{owner}/{repo}/pulls/{pull_number}/comments/{root_comment_id}/replies \
  -f body="<approved reply>"
```

Two things reliably go wrong here:

- **The path needs `{pull_number}`.** It is the one review-comment route that
  does — get, update and delete all take only the comment id — so it is easy to
  copy the wrong shape from adjacent docs. Omitting it returns 404 even when the
  comment id is valid.
- **`root_comment_id` is the integer `databaseId` of the thread's *first*
  comment**, not the thread's GraphQL node id, and not the id of the comment you
  are replying to.

Do not use `POST /pulls/{n}/reviews` for replies — that creates new standalone
line comments rather than threading.

After each post: set `status: posted`, record `posted_at`, write the file.

To resolve a thread the user asked to close:

```graphql
mutation($threadId:ID!) {
  resolveReviewThread(input:{threadId:$threadId}) { thread { id isResolved } }
}
```

```bash
gh api graphql -f query='<mutation>' -f threadId=<thread GraphQL node id>
```

That takes the opaque node id (`PRT_kwDO...`), not the numeric `databaseId`.

Optionally offer a wrap-up comment on the PR as a whole. Use `event: COMMENT` —
never `APPROVE` or `REQUEST_CHANGES` on your own PR:

```bash
gh api --method POST /repos/{owner}/{repo}/pulls/{pr_number}/reviews \
  -f body="Addressed all review comments in this batch." -f event="COMMENT"
```

## Step 5 — Push

Ask explicitly before pushing. Never force-push — a reviewer mid-read loses
their place, and outdated threads detach.

```bash
git push origin <branch>
```

## Step 6 — Cleanup

Remove the worktree if one was created:

```bash
git worktree remove <path>        # add --force only if untracked files remain
git worktree list                 # confirm
```

Leave the tracking file in place. It lives in `.git/pr-reviews/` outside the
worktree, costs nothing, and is what lets the next round of review start from
what was already answered. Delete it only when the PR is merged or closed.

## Resume

When a tracking file already exists:

1. Read it for last-session state
2. Re-fetch all three surfaces
3. Match threads by `thread_id`
4. Compute the delta and show it *before* the triage table:
   - Threads absent from tracking → `new`
   - `isResolved` now true but tracked `pending`/`drafting` → `resolved-upstream`
   - `status: posted` but a comment newer than `posted_at` → re-review
   - `head_sha_at_last_visit` no longer an ancestor of HEAD → force-push happened;
     warn that line anchors may have shifted
5. Merge the delta into the triage table and continue from Step 2

```
Changes since last session (2026-05-19):
- 2 new threads from alice
- Thread #3 resolved upstream
- Thread #1 (posted) has new replies from bob
```

## Tracking file format

```markdown
---
pr_number: 42
branch: "my-feature"
repo: "yugabyte/yb-voyager"
last_session_at: "2026-05-20T10:00:00Z"
head_sha_at_last_visit: "abc123def456"
---

## thread PRT_kwDOAbc123

thread_id: "PRT_kwDOAbc123"
root_comment_id: 12345678      # integer databaseId of the FIRST comment
file: "src/foo.go"
line: 42                       # null when outdated
original_line: 38
original_commit: "9a22d6cc"    # resolves the anchor when line is null
reviewer: "alice"
classification: "needs-attention"
status: "pending"
last_comment_at: "2026-05-19T08:00:00Z"
posted_at: null
draft_reply: |
  Fixed in abc456 — the return value now clamps to [0, max].
related_commits: ["abc456def789"]
notes: ""
```

**Status lifecycle**

```
pending → drafting → ready → posted
   ↓                            ↓
 skipped              resolved-upstream
```

| Status | Meaning |
|---|---|
| `pending` | Not yet looked at this session |
| `drafting` | Parked — user was undecided |
| `ready` | Reply approved, not yet posted |
| `posted` | Reply posted to GitHub |
| `skipped` | User explicitly skipped it |
| `resolved-upstream` | Resolved on GitHub since last visit |

**Classifications:** `needs-attention`, `waiting-on-reviewer`, `ai-finding`,
`resolved`, `already-handled`
