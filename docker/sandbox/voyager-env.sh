# Sourced by interactive `docker exec -u voyager voyager-sandbox bash -l` shells.
# Mirrors the ENV set in the Dockerfile so non-login `bash -c` invocations can
# re-source this explicitly if needed.

export JAVA_HOME=/opt/java17
export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=/opt/yugabyte/bin:$GOROOT/bin:$GOPATH/bin:$JAVA_HOME/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

export YB_VOYAGER_SEND_DIAGNOSTICS=0

# Source DB (Postgres 17) - inside the container only.
export PGHOST=127.0.0.1
export PGPORT=5432
export PGUSER=postgres
export PGPASSWORD=postgres

# Target DB (YugabyteDB) - inside the container only.
export YB_HOST=127.0.0.1
export YB_PORT=5433
export YB_USER=yugabyte
export YB_PASSWORD=yugabyte

# Pick up the voyager rc file if the installer wrote one.
if [ -f "$HOME/.yb-voyager.rc" ]; then
    # shellcheck disable=SC1091
    . "$HOME/.yb-voyager.rc"
fi
