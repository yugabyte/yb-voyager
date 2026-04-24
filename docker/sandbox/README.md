# voyager-sandbox

An isolated Docker container bundling **yb-voyager**, **Postgres 17**, and
**YugabyteDB** so AI agents (Cursor, etc.) can run migrations, tests, and
rebuilds without touching the host. The repo is bind-mounted read-write, so
edits made on the host are immediately visible inside the container.

## When to use it

- Manual / end-to-end voyager flows the agent should run autonomously.
- Experimenting with destructive commands (drop databases, corrupt export
  dirs, etc.) without risking host state.
- Letting an agent iterate: edit Go code on the host &rarr; agent rebuilds
  and re-tests inside the sandbox.

**Not** a replacement for:

- `go test -tags unit ./...` &mdash; run these on the host (fastest).
- Testcontainer-based integration tests (`issues_integration`, etc.) &mdash;
  they need Docker-in-Docker; run them on the host.
- Oracle / MySQL source databases &mdash; the image is built with `-p`
  (PG-only).

## Recommended layout: one worktree per sandbox

Because the bind mount is read-write, a rogue command inside the container
can delete host files. Keep blast radius small by running the agent against a
dedicated worktree:

```bash
cd /path/to/yb-voyager
git worktree add ../yb-voyager-sandbox
cd ../yb-voyager-sandbox
bash docker/sandbox/build.sh
bash docker/sandbox/run.sh
```

The default container name is `voyager-sandbox`. If you want multiple
worktrees running simultaneously, override `NAME` per worktree
(`NAME=voyager-sandbox-foo bash docker/sandbox/run.sh`) and update the
Cursor allowlist accordingly.

## Build

```bash
bash docker/sandbox/build.sh
```

First build takes ~10-15 min (pulls base image, PGDG repo, YugabyteDB tarball,
Go toolchain, downloads Go modules, compiles voyager + the Debezium plugin).
Subsequent builds reuse the layer cache.

Build-time knobs (env vars):

| Var | Default | Notes |
|-----|---------|-------|
| `IMAGE` | `voyager-sandbox:latest` | Output tag |
| `YB_VERSION` | `2025.2.2.2` | YugabyteDB release |
| `YB_BUILD` | `b11` | YugabyteDB build suffix |
| `GO_VERSION` | `1.24.2` | Matches `AGENTS.md` |

## Run

```bash
bash docker/sandbox/run.sh
```

Starts the container in detached mode, initializes ephemeral Postgres and
YugabyteDB, and prints the exact `docker exec` pattern to allowlist in
Cursor.

Runtime knobs:

| Var | Default | Notes |
|-----|---------|-------|
| `NAME` | `voyager-sandbox` | Container name |
| `MEM` | `6g` | Memory limit |
| `CPUS` | `4` | CPU limit |
| `EXPOSE` | `0` | Set to `1` to publish PG/YB ports to `127.0.0.1` for host-side debugging |

## Allowlist the agent

In Cursor, allowlist this command prefix (the exact pattern is also printed
by `run.sh`):

```
docker exec -u voyager voyager-sandbox *
```

`-u voyager` is critical: without it, `docker exec` runs as root, and any
files the agent creates inside the mounted repo end up root-owned on your
host.

## Agent workflow

1. Edit code on the host (regular IDE flow).
2. Rebuild the voyager binary inside the sandbox (fast, Go-only):
   ```bash
   docker exec -u voyager voyager-sandbox bash -lc \
     'cd /workspace/yb-voyager/installer_scripts && yes | ./install-yb-voyager -v'
   ```
3. Run voyager against the in-container DBs:
   ```bash
   docker exec -u voyager voyager-sandbox bash -lc '
     yb-voyager assess-migration \
       --source-db-type postgresql \
       --source-db-host 127.0.0.1 --source-db-port 5432 \
       --source-db-user postgres --source-db-password postgres \
       --source-db-name postgres \
       --export-dir /tmp/export
   '
   ```
4. Reset state between runs: `docker restart voyager-sandbox`. This wipes
   both PG and YB data dirs and re-initializes them; the built binary and Go
   module cache survive.

Connection info inside the container:

- **Postgres 17**: `127.0.0.1:5432`, user `postgres`, password `postgres`.
- **YugabyteDB**: `127.0.0.1:5433`, user `yugabyte`, password `yugabyte`.

These are also exported as `PGHOST`/`PGPORT`/`PGUSER`/`PGPASSWORD` and
`YB_HOST`/`YB_PORT`/`YB_USER`/`YB_PASSWORD` in login shells.

## Troubleshooting

- **`docker logs <name>` shows "YugabyteDB did not become ready within 120s"**:
  increase `MEM` (YB needs ~2 GB headroom) and try again.
- **Agent rebuild fails with "unknown module" errors**: `go.mod` changed and
  pulls a new module. Inside the container, run `go mod download` once (the
  container has internet; the module cache under `/home/voyager/go/pkg/mod`
  persists until the container is removed).
- **Permission denied writing to the mounted repo**: you omitted
  `-u voyager`. The UID inside the container (`voyager`, uid 1000) must
  match your host UID for bind-mount writes to look clean; if it doesn't,
  rebuild with `docker build --build-arg VOYAGER_UID=$(id -u)` after adding
  the arg to the Dockerfile.
- **YB admin UI (`:7000`, `:9000`, `:15433`)**: re-run with `EXPOSE=1` to
  publish these ports to `127.0.0.1` on the host.

## Architecture at a glance

```
Host git worktree  ──(bind mount rw)──▶  /workspace/yb-voyager in container
                                         │
                  docker exec -u voyager │
  AI agent ──────────────────────────────┤
                                         ▼
                     entrypoint.sh ──▶ Postgres 17 (:5432)
                                   └─▶ yugabyted (:5433 + :7000/:9000/:15433)
                                   └─▶ yb-voyager binary (/usr/local/bin)
```
