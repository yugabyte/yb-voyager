#!/usr/bin/env bash
#
# Regenerate the third_party/pg_query_go snapshot from a fresh upstream
# libpg_query checkout with YB-EXT patches applied.
#
# Run this after editing any of:
#   - third_party/pg-yb-parser/features/*.patch       (per-feature grammar/AST)
#   - third_party/pg-yb-parser/00_yb_toolchain.patch  (toolchain — rarely)
#   - third_party/pg_query_go/parser/postgres_deparse.c  (deparse emit cases)
#
# Or just to refresh the snapshot against a new upstream libpg_query release
# (bump LIB_TAG below).
#
# Prereqs:
#   - docker
#   - Ruby + protoc available via the libpg_query-regen Docker image
#     (this script builds it if not present)

set -euo pipefail

LIB_TAG="${LIB_TAG:-17-6.1.0}"
PG_QUERY_GO_TAG="${PG_QUERY_GO_TAG:-v6.1.0}"

# Resolve repo paths relative to this script
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
PARSER_DIR="$SCRIPT_DIR"

WORK=$(mktemp -d -t yb-parser-regen.XXXXXX)
trap 'rm -rf "$WORK"' EXIT

echo "==> regen.sh: working dir = $WORK"
echo "==> regen.sh: libpg_query tag = $LIB_TAG"

# 1. Fetch upstream libpg_query
echo "==> Fetching upstream libpg_query @ $LIB_TAG"
git clone --quiet --depth 1 --branch "$LIB_TAG" \
    https://github.com/pganalyze/libpg_query.git "$WORK/libpg_query"

cd "$WORK/libpg_query"

# 2. Apply toolchain patch (Makefile portability, scripts/extract_source.rb)
echo "==> Applying 00_yb_toolchain.patch"
patch -p1 --silent < "$PARSER_DIR/00_yb_toolchain.patch"

# 3a. Apply srcdata-edits at libpg_query root level (these touch srcdata/*.json,
#     which lives in the libpg_query repo, not in the tmp/postgres PG-source tree)
echo "==> Applying srcdata-edits"
for p in "$PARSER_DIR"/srcdata-edits/*.patch; do
  [ -e "$p" ] || continue
  patch -p1 --silent < "$p"
done

# 3b. Stage per-feature PG-source patches into libpg_query/patches/ so the
#     PGDIR rule applies them to tmp/postgres/ after PG is unzipped
echo "==> Staging feature patches"
for p in "$PARSER_DIR"/features/*.patch; do
  [ -e "$p" ] || continue
  cp "$p" "patches/$(basename "$p")"
done

# 4. Build regen Docker image if missing
if ! docker image inspect libpg_query-regen >/dev/null 2>&1; then
  echo "==> Building Docker regen image (first time, ~2 min)"
  docker build -q -f "$PARSER_DIR/Dockerfile.regen" -t libpg_query-regen "$PARSER_DIR" >/dev/null
fi

# 5. Run extract_source + regen_proto in the container
echo "==> Running extract_source (downloads PG, applies patches, bison runs)"
docker run --rm -v "$WORK/libpg_query":/work -w /work libpg_query-regen \
    make extract_source

echo "==> Running regen_proto (regenerates proto + enum_defs.c from srcdata)"
docker run --rm -v "$WORK/libpg_query":/work -w /work libpg_query-regen \
    make regen_proto

# 6. Snapshot into committed pg_query_go/
echo "==> Snapshotting into third_party/pg_query_go/"
PG_QUERY_GO_DIR="$REPO_ROOT/third_party/pg_query_go"
if [ ! -d "$PG_QUERY_GO_DIR" ]; then
  echo "ERROR: $PG_QUERY_GO_DIR not found — pg_query_go must be vendored at this path"
  exit 1
fi

(cd "$PG_QUERY_GO_DIR" && LIBDIR="$WORK/libpg_query" make update_source)

# 7. Restore committed YB-EXT deparse edits (update_source clobbered them)
echo "==> Restoring committed postgres_deparse.c YB-EXT cases"
cd "$REPO_ROOT"
git checkout HEAD -- third_party/pg_query_go/parser/postgres_deparse.c

# 8. Restore upstream's macOS-extracted pg_config.h (the Linux-extracted one
#    has OS-specific defines that conflict with macOS host compilation)
#    Source: cached at $GOMODCACHE/.../pg_query_go/v6@v6.1.0/parser/include/postgres/pg_config.h
GOMODCACHE=$(go env GOMODCACHE 2>/dev/null || echo "$HOME/go/pkg/mod")
UPSTREAM_PG_CONFIG="$GOMODCACHE/github.com/pganalyze/pg_query_go/v6@$PG_QUERY_GO_TAG/parser/include/postgres/pg_config.h"
if [ -f "$UPSTREAM_PG_CONFIG" ]; then
  echo "==> Restoring upstream's portable pg_config.h (avoids macOS strlcat conflict)"
  cp "$UPSTREAM_PG_CONFIG" "$PG_QUERY_GO_DIR/parser/include/postgres/pg_config.h"
  chmod u+w "$PG_QUERY_GO_DIR/parser/include/postgres/pg_config.h"
else
  echo "WARN: upstream pg_config.h not in Go module cache. macOS host builds may fail."
  echo "      Fetch upstream pg_query_go first:"
  echo "        (cd /tmp && GOPATH=/tmp/go go mod download github.com/pganalyze/pg_query_go/v6@$PG_QUERY_GO_TAG)"
fi

echo ""
echo "==> Done. Review and commit changes under third_party/pg_query_go/."
echo "==> Verify with: make pg-yb-parser-check"
