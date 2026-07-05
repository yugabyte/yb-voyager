# Environment setup

How to get a live PostgreSQL source, a YugabyteDB target, and a branch-built `yb-voyager` binary. Three modes; **Docker is the default**.

All defaults below are wired into `scripts/provision-dbs.sh`, `scripts/build-voyager.sh`, `scripts/teardown.sh`. Override via the env vars named in each section.

## Connection conventions (match migtests)

The scripts and runbook use the same env-var names the `migtests/` harness uses, so the two are interchangeable:

| Role | Host | Port | User | Password | DB |
| --- | --- | --- | --- | --- | --- |
| Source (PG) | `SOURCE_DB_HOST` 127.0.0.1 | `SOURCE_DB_PORT` **5490** | `SOURCE_DB_USER` postgres | `SOURCE_DB_PASSWORD` postgres | `SOURCE_DB_NAME` mtv_src |
| Target (YB) | `TARGET_DB_HOST` 127.0.0.1 | `TARGET_DB_PORT` **5491** | `TARGET_DB_USER` yugabyte | `TARGET_DB_PASSWORD` yugabyte | `TARGET_DB_NAME` mtv_tgt |

Ports 5490/5491 are chosen to avoid the common local occupants (5432/5433) and typical dev containers. For offline testing the `postgres` superuser is used directly, which sidesteps the guardrail permission grants. Live testing needs the replication grants — see `migration-runbook.md`.

> Note: the migtests CI uses a dedicated `ybvoyager` user with password `Test@123#$%^&*()!` and a colocated target DB created `WITH COLOCATION=TRUE`. Using the admin superusers as above is simpler for ad-hoc runs; switch to the `ybvoyager` user + `create_yb_user` / `create_pg_user` scripts under `migtests/scripts/{postgresql,yugabytedb}/` when a scenario specifically tests permissions.

## Mode 1 — Docker (default)

Validated images and commands (used by `provision-dbs.sh`):

**PostgreSQL source** — `postgres:17`, logical replication pre-enabled so the same container also serves live tests:
```
docker run -d --name mtv-pg-src -p 5490:5432 -e POSTGRES_PASSWORD=postgres postgres:17 \
  -c wal_level=logical -c max_replication_slots=20 -c max_wal_senders=20
```
Readiness: `docker exec mtv-pg-src pg_isready -U postgres`.

**YugabyteDB target** — `yugabytedb/yugabyte:<tag>`. Port-map (not `--network=host`) so it coexists with local Postgres on 5433:
```
docker run -d --name mtv-yb-tgt -p 5491:5433 -p 15491:15433 yugabytedb/yugabyte:<tag> \
  bin/yugabyted start --background=false --callhome=false
```
- YSQL binds to `0.0.0.0:5433` inside the container by default, so the `-p 5491:5433` mapping works without extra flags. Default auth accepts user `yugabyte`.
- Readiness (takes ~30–60s): poll `PGPASSWORD=yugabyte psql -h 127.0.0.1 -p 5491 -U yugabyte -d yugabyte -c 'select 1'` until it succeeds. Do **not** trust the container being "up" — wait for YSQL.
- Version tag: default to a locally-present image (check `docker images | grep yugabyte`) to avoid a slow ~1 GB pull. The repo's supported versions live in `yb-voyager/src/version/... yb-versions.json` (latest stable + a few back). To fuzz across versions, loop the tag.

Create the databases after both are ready:
```
PGPASSWORD=postgres psql -h 127.0.0.1 -p 5490 -U postgres -d postgres -c "CREATE DATABASE mtv_src;"
PGPASSWORD=yugabyte psql -h 127.0.0.1 -p 5491 -U yugabyte -d yugabyte -c "CREATE DATABASE mtv_tgt;"
```

## Mode 2 — Local / already-running DBs

Point the env vars at any reachable PG and YB. For live tests the source PG must have `wal_level=logical`. This is also how you target a **real multi-node YB cluster** (the QA "verify across environments — local cluster and YB cluster" requirement): set `TARGET_DB_HOST/PORT/USER/PASSWORD` to the cluster. The testcontainers layer honors `YB_EXTERNAL_HOST` / `YB_EXTERNAL_PORT` for the same purpose in Go tests.

## Mode 3 — migtests harness

For standard flows, skip manual provisioning entirely and drive the existing harness:
```
export YB_VOYAGER_SEND_DIAGNOSTICS=false
bash migtests/scripts/run-test.sh <test-name> [env.sh] [--run-via-config-file]
```
This needs the source/target DBs + a globally-installed `yb-voyager`. Use it for the "automated" column of the test plan; use Modes 1/2 for novel manual scenarios.

## Building the binary under test

The globally-installed `yb-voyager` does **not** necessarily reflect the branch. Build from the working tree.

**Fast path (offline + most flags), no root, non-invasive** — `scripts/build-voyager.sh`:
```
cd yb-voyager
go build -o <scratch>/yb-voyager .
```
Version caveat: `yb-voyager version` reports `src/utils.GIT_COMMIT_HASH`, which is a Go **const** (`"$Format:%H$"`, substituted only by `git archive`). It **cannot** be overridden with `ldflags -X` (that works on vars only), and Go's automatic VCS stamp reads the **main** checkout's HEAD when you build inside a git *worktree* — so the reported commit can be misleading. This is cosmetic: the compiled code is the working tree's, which is what you're testing. Record the real worktree HEAD (`git rev-parse HEAD`) in the report separately. `scripts/build-voyager.sh` does this.
The bare `go build` works for `export/import schema`, `export/import data`, `analyze-schema`, and the assess flow does not need `/opt/yb-voyager` assets for the offline path. Reference the binary by full path; do not overwrite `/usr/local/bin/yb-voyager`.

**Full path (required for live / Debezium)** — the installer wires up the Debezium server (hardcoded lookup at `/opt/yb-voyager/debezium-server`), ora2pg, config templates, and guardrail scripts:
```
export JAVA_HOME=<jdk17>          # installer caps Java at 19; use 17
yes | bash installer_scripts/install-yb-voyager --install-from-local-source --only-pg-support
# fast rebuild of just the binaries on later iterations:
yes | bash installer_scripts/install-yb-voyager -v
```
`--install-from-local-source` compiles from the working tree. `--only-pg-support` skips Oracle/MySQL deps. This installs globally, so only use it when live testing needs Debezium, or run it in a throwaway/Cursor-Cloud environment.

## Environment matrix to consider per feature

Per the QA guidance, verify a feature across the environments where it's relevant:
- **Binary source**: branch `go build` vs full installer build (the latter for live).
- **Invocation**: individual CLI flags vs `--run-via-config-file` (config YAML) — behavior must match. See the config templates under `migtests/scripts/config-templates/`.
- **Target**: single-node Docker YB vs a real multi-node YB cluster (parallelism, colocation, and adaptive-parallelism behavior differ with node/core count).
- **YB version**: latest stable vs older supported tags for version-gated behavior.
