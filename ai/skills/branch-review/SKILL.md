---
name: branch-review
description: Review all code changes on the current branch compared to main, applying the hierarchical AGENTS.md engineering standards from each changed file's directory up to the repo root. Use when the user asks to review a branch, review changes, compare against main, do a code review, or check branch diff.
---

# Branch Code Review

The review workflow lives in [`REVIEW.md`](../../../REVIEW.md) at the repository root, so every review agent applies the same instructions.

Read `$(git rev-parse --show-toplevel)/REVIEW.md` in full and apply it to the current branch, using `main` as the base unless the user names another.
