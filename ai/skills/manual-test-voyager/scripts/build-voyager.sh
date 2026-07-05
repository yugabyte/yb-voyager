#!/usr/bin/env bash
# Build the yb-voyager binary from the current working tree (the code under test).
# Fast path: plain `go build`. Non-invasive — writes to $OUT, never overwrites the
# globally-installed binary.
#
# NOTE ON VERSION: `yb-voyager version` reports `src/utils.GIT_COMMIT_HASH`, which is a
# Go *const* ("$Format:%H$", substituted only by `git archive`). It CANNOT be overridden
# by `ldflags -X` (that works on vars only), and Go's auto VCS stamp reads the *main*
# checkout's HEAD when building inside a git worktree. So the reported commit may be
# stale/misleading. What matters is that the compiled code IS the current working tree's
# — this script builds exactly that. We print the real worktree HEAD below for the record.
#
# For LIVE / Debezium testing use the installer instead (see references/environment-setup.md):
#   yes | bash installer_scripts/install-yb-voyager --install-from-local-source --only-pg-support
#
#   OUT=<path>   output binary path (default: <repo>/manual-test-runs/bin/yb-voyager)
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
OUT="${OUT:-${REPO_ROOT}/manual-test-runs/bin/yb-voyager}"
mkdir -p "$(dirname "$OUT")"

COMMIT="$(git -C "$REPO_ROOT" rev-parse HEAD)"

echo "[build] building from $REPO_ROOT/yb-voyager (worktree HEAD ${COMMIT:0:12}) -> $OUT" >&2
( cd "${REPO_ROOT}/yb-voyager" && go build -o "$OUT" . )

echo "[build] done. worktree HEAD = $COMMIT" >&2
echo "[build] (note: 'version' below may report a different commit — see header)" >&2
"$OUT" version 2>&1 | head -3
echo "export MTV_BIN=$OUT"
