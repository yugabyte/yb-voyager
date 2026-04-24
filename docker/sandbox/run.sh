#!/usr/bin/env bash
# Start (or restart) the voyager-sandbox container.
#
# Usage:
#   bash docker/sandbox/run.sh          # default name, 6G mem, 4 CPUs
#   MEM=8g CPUS=6 bash ...              # override resource limits
#   NAME=voyager-sandbox-foo bash ...   # override container name (e.g. per worktree)
#   EXPOSE=1 bash ...                   # publish PG/YB ports to host
#
# The bind mount is read-write by design: the agent rebuilds in-place, so host
# edits must flow into the container immediately. Use a dedicated git worktree
# (git worktree add ../voyager-sandbox) to contain any accidental damage.

set -euo pipefail

REPO="$(git rev-parse --show-toplevel)"
IMAGE="${IMAGE:-voyager-sandbox:latest}"
NAME="${NAME:-voyager-sandbox}"
# Default 6g fits an 8g Docker Desktop VM with ~2g headroom for the daemon.
# The Go voyager rebuild peaks around 5g, so anything below 6g will OOM.
# Bump MEM higher if your VM is larger.
MEM="${MEM:-6g}"
CPUS="${CPUS:-4}"
EXPOSE="${EXPOSE:-0}"

publish_args=()
if [ "$EXPOSE" = "1" ]; then
    publish_args=(
        -p 127.0.0.1:5432:5432
        -p 127.0.0.1:5433:5433
        -p 127.0.0.1:7000:7000
        -p 127.0.0.1:9000:9000
        -p 127.0.0.1:15433:15433
    )
fi

echo "Removing any previous ${NAME} container"
docker rm -f "${NAME}" >/dev/null 2>&1 || true

PLATFORM_ARG=()
if [ -n "${PLATFORM:-}" ]; then
    PLATFORM_ARG=(--platform="${PLATFORM}")
fi

echo "Starting ${NAME} (image=${IMAGE}, mem=${MEM}, cpus=${CPUS}, platform=${PLATFORM:-host-native})"
docker run -d \
    --name "${NAME}" \
    "${PLATFORM_ARG[@]}" \
    --memory="${MEM}" \
    --cpus="${CPUS}" \
    --security-opt=no-new-privileges \
    -v "${REPO}":/workspace/yb-voyager:rw \
    "${publish_args[@]}" \
    "${IMAGE}" >/dev/null

echo "Waiting for Postgres + YugabyteDB readiness (tail -f logs with: docker logs -f ${NAME})"
for i in $(seq 1 90); do
    if docker logs "${NAME}" 2>&1 | grep -q 'Sandbox ready'; then
        break
    fi
    sleep 2
done

if ! docker logs "${NAME}" 2>&1 | grep -q 'Sandbox ready'; then
    echo "ERROR: sandbox did not report ready within ~3 minutes. Recent logs:"
    docker logs --tail=80 "${NAME}" || true
    exit 1
fi

cat <<EOF

==================================================================
Sandbox ${NAME} is up.

Allowlist this command prefix in your Cursor permissions so the
agent can run anything inside the sandbox without prompting:

    docker exec -u voyager ${NAME} *

Sanity check:

    docker exec -u voyager ${NAME} bash -lc \\
      'yb-voyager version && pg_isready -h 127.0.0.1 -p 5432 && \\
       ysqlsh -h 127.0.0.1 -p 5433 -U yugabyte -c "select 1"'

Reset state (fresh PG + YB, keeps built binary):

    docker restart ${NAME}

Tear down:

    docker rm -f ${NAME}
==================================================================
EOF
