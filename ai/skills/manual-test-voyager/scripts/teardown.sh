#!/usr/bin/env bash
# Remove the throwaway manual-test containers. Only ever touches the mtv-* names
# (or the ones you pass via env) — never a developer's own containers.
#
#   PG_CONTAINER=mtv-pg-src  YB_CONTAINER=mtv-yb-tgt   KEEP_DATA=0
set -euo pipefail
PG_CONTAINER="${PG_CONTAINER:-mtv-pg-src}"
YB_CONTAINER="${YB_CONTAINER:-mtv-yb-tgt}"

for c in "$PG_CONTAINER" "$YB_CONTAINER"; do
  if docker inspect "$c" >/dev/null 2>&1; then
    echo "[teardown] removing $c" >&2
    docker rm -f "$c" >/dev/null
  else
    echo "[teardown] $c not present, skipping" >&2
  fi
done
echo "[teardown] done" >&2
