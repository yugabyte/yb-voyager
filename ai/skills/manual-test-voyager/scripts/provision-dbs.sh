#!/usr/bin/env bash
# Provision a throwaway PostgreSQL source + YugabyteDB target for manual testing.
# Idempotent: reuses healthy containers, waits for readiness, creates the DBs.
# Never touches non-mtv containers. Override any default via env vars.
#
#   PG_CONTAINER=mtv-pg-src  PG_PORT=5490  PG_IMAGE=postgres:17
#   YB_CONTAINER=mtv-yb-tgt  YB_PORT=5491  YB_UI_PORT=15491  YB_IMAGE=<auto-detected local yugabyte image>
#   SOURCE_DB_NAME=mtv_src   TARGET_DB_NAME=mtv_tgt
#
# On success prints `export ...` lines to eval into your shell / the runbook env.
set -euo pipefail

PG_CONTAINER="${PG_CONTAINER:-mtv-pg-src}"
YB_CONTAINER="${YB_CONTAINER:-mtv-yb-tgt}"
PG_PORT="${PG_PORT:-5490}"
YB_PORT="${YB_PORT:-5491}"
YB_UI_PORT="${YB_UI_PORT:-15491}"
PG_IMAGE="${PG_IMAGE:-postgres:17}"
SOURCE_DB_NAME="${SOURCE_DB_NAME:-mtv_src}"
TARGET_DB_NAME="${TARGET_DB_NAME:-mtv_tgt}"

log() { printf '\033[36m[provision]\033[0m %s\n' "$*" >&2; }
err() { printf '\033[31m[provision] ERROR:\033[0m %s\n' "$*" >&2; }

# Pick a locally-present YugabyteDB image unless one is given, to avoid a slow ~1GB pull.
if [[ -z "${YB_IMAGE:-}" ]]; then
  YB_IMAGE="$(docker images --format '{{.Repository}}:{{.Tag}}' | grep '^yugabytedb/yugabyte:' | grep -v ':<none>' | head -1 || true)"
  if [[ -z "$YB_IMAGE" ]]; then
    err "No local yugabytedb/yugabyte image found. Set YB_IMAGE=yugabytedb/yugabyte:<tag> (will pull) or 'docker pull' one first."
    exit 1
  fi
fi
log "YB image: $YB_IMAGE"

container_running() { [[ "$(docker inspect -f '{{.State.Running}}' "$1" 2>/dev/null || echo false)" == "true" ]]; }

# --- PostgreSQL source ---
if container_running "$PG_CONTAINER"; then
  log "reusing running $PG_CONTAINER"
else
  docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true
  log "starting $PG_CONTAINER on :$PG_PORT ($PG_IMAGE, wal_level=logical)"
  docker run -d --name "$PG_CONTAINER" -p "${PG_PORT}:5432" -e POSTGRES_PASSWORD=postgres "$PG_IMAGE" \
    -c wal_level=logical -c max_replication_slots=20 -c max_wal_senders=20 >/dev/null
fi

# --- YugabyteDB target ---
if container_running "$YB_CONTAINER"; then
  log "reusing running $YB_CONTAINER"
else
  docker rm -f "$YB_CONTAINER" >/dev/null 2>&1 || true
  log "starting $YB_CONTAINER on :$YB_PORT ($YB_IMAGE)"
  docker run -d --name "$YB_CONTAINER" -p "${YB_PORT}:5433" -p "${YB_UI_PORT}:15433" "$YB_IMAGE" \
    bin/yugabyted start --background=false --callhome=false >/dev/null
fi

# --- wait for PG ---
log "waiting for PostgreSQL ..."
for i in $(seq 1 30); do
  if docker exec "$PG_CONTAINER" pg_isready -U postgres >/dev/null 2>&1; then log "PostgreSQL ready"; break; fi
  [[ $i -eq 30 ]] && { err "PostgreSQL not ready after 30 tries"; exit 1; }
  sleep 2
done

# --- wait for YB YSQL (30-60s cold start) ---
log "waiting for YugabyteDB YSQL (can take ~60s) ..."
for i in $(seq 1 60); do
  if PGPASSWORD=yugabyte psql -h 127.0.0.1 -p "$YB_PORT" -U yugabyte -d yugabyte -tAc 'select 1' >/dev/null 2>&1; then
    log "YugabyteDB ready"; break
  fi
  [[ $i -eq 60 ]] && { err "YugabyteDB YSQL not ready after 60 tries"; docker logs --tail 20 "$YB_CONTAINER" >&2; exit 1; }
  sleep 2
done

# --- create databases (idempotent) ---
log "creating databases $SOURCE_DB_NAME / $TARGET_DB_NAME"
PGPASSWORD=postgres psql -h 127.0.0.1 -p "$PG_PORT" -U postgres -d postgres -tAc \
  "SELECT 1 FROM pg_database WHERE datname='${SOURCE_DB_NAME}'" | grep -q 1 || \
  PGPASSWORD=postgres psql -h 127.0.0.1 -p "$PG_PORT" -U postgres -d postgres -c "CREATE DATABASE ${SOURCE_DB_NAME};" >/dev/null
PGPASSWORD=yugabyte psql -h 127.0.0.1 -p "$YB_PORT" -U yugabyte -d yugabyte -tAc \
  "SELECT 1 FROM pg_database WHERE datname='${TARGET_DB_NAME}'" | grep -q 1 || \
  PGPASSWORD=yugabyte psql -h 127.0.0.1 -p "$YB_PORT" -U yugabyte -d yugabyte -c "CREATE DATABASE ${TARGET_DB_NAME};" >/dev/null

log "done. Connection env:"
cat <<EOF
export SOURCE_DB_HOST=127.0.0.1 SOURCE_DB_PORT=${PG_PORT} SOURCE_DB_USER=postgres SOURCE_DB_PASSWORD=postgres SOURCE_DB_NAME=${SOURCE_DB_NAME}
export TARGET_DB_HOST=127.0.0.1 TARGET_DB_PORT=${YB_PORT} TARGET_DB_USER=yugabyte TARGET_DB_PASSWORD=yugabyte TARGET_DB_NAME=${TARGET_DB_NAME}
EOF
