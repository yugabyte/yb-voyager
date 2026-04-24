#!/usr/bin/env bash
# voyager-sandbox entrypoint: start Postgres 17 and YugabyteDB, wait for both
# to accept connections, then exec CMD (defaults to `sleep infinity`).
#
# DB state is ephemeral: we re-initialize on every container start so the
# agent always gets a clean slate. If you want persistence, mount named
# volumes at $PGDATA and $YB_BASE_DIR and remove the rm -rf below.

set -euo pipefail

PGBIN=/usr/lib/postgresql/17/bin
PGDATA=/home/voyager/pgdata
YB_BASE_DIR=/home/voyager/yb-data

log() { printf '[entrypoint] %s\n' "$*"; }

start_postgres() {
    log "Initializing Postgres 17 at ${PGDATA}"
    rm -rf "${PGDATA}"
    "${PGBIN}/initdb" -D "${PGDATA}" -U postgres --auth=trust --encoding=UTF8 >/tmp/initdb.log 2>&1

    cat >> "${PGDATA}/postgresql.conf" <<EOF
listen_addresses = '127.0.0.1'
port = 5432
logging_collector = off
EOF
    # Allow local md5 + trust from loopback so the agent can use either.
    cat > "${PGDATA}/pg_hba.conf" <<EOF
local   all             all                                     trust
host    all             all             127.0.0.1/32            trust
host    all             all             ::1/128                 trust
EOF

    log "Starting Postgres"
    "${PGBIN}/pg_ctl" -D "${PGDATA}" -l /tmp/pg.log -w -t 60 start

    "${PGBIN}/psql" -U postgres -h 127.0.0.1 -c \
        "ALTER USER postgres WITH PASSWORD 'postgres';" >/dev/null
    log "Postgres ready on 127.0.0.1:5432 (postgres/postgres)"
}

start_yugabyted() {
    log "Starting YugabyteDB (base_dir=${YB_BASE_DIR})"
    rm -rf "${YB_BASE_DIR}"
    /opt/yugabyte/bin/yugabyted start \
        --advertise_address=127.0.0.1 \
        --base_dir="${YB_BASE_DIR}" \
        >/tmp/yugabyted.start.log 2>&1

    log "Waiting for YSQL on 127.0.0.1:5433 (may take ~30-60s on cold start)"
    for i in $(seq 1 60); do
        if /opt/yugabyte/bin/ysqlsh -h 127.0.0.1 -p 5433 -U yugabyte \
                -c 'select 1' >/dev/null 2>&1; then
            log "YugabyteDB ready on 127.0.0.1:5433 (yugabyte/yugabyte)"
            return 0
        fi
        sleep 2
    done
    log "ERROR: YugabyteDB did not become ready within 120s"
    tail -n 50 /tmp/yugabyted.start.log || true
    return 1
}

start_postgres
start_yugabyted

log "Sandbox ready. exec: $*"
exec "$@"
