# Setting up an environment to run the hunt

The hunt needs to run voyager end to end against real PostgreSQL and YugabyteDB containers. This page gets any Linux/macOS host there, fastest path first. It assumes nothing about the agent harness; the last section adds notes for Claude Code cloud sessions, where most restrictions below were first hit.

Do the steps in order, record what you did in the report's **Environment** section, and stop with a clear message if a hard requirement can't be met — never fake results.

**Provision only on a throwaway host.** The steps below may start daemons, install packages, edit apt sources and write under `/opt`. Do that only in an ephemeral environment (a cloud session, a CI runner, a disposable VM). On a developer machine or a shared host, check the requirements, and if one is missing, stop and report it instead of changing the system.

## 0. Probe the host (2 minutes, always first)

```bash
uname -a; id -u                                  # root? (then drop "sudo")
nproc; free -g 2>/dev/null || sysctl hw.memsize  # CPU / RAM
docker info --format '{{.ServerVersion}} mem={{.MemTotal}} cpus={{.NCPU}}'   # HARD requirement
go version; java -version 2>&1 | head -1; mvn -v 2>/dev/null | head -1
pg_dump --version; psql --version
env | grep -i -E '^(https?|no)_proxy=' | sed 's/=.*/=<set>/'
env | grep -E '^(CONTROL_PLANE_TYPE|YUGABYTED_DB_CONN_STRING)=' | sed 's/=.*/=<set>/'
```

Then check which download hosts are reachable (restricted networks often allow some and block others):

```bash
for u in https://proxy.golang.org https://repo.maven.apache.org/maven2/ https://github.com \
         https://api.github.com https://objects.githubusercontent.com https://apt.postgresql.org/pub/repos/apt/ \
         https://registry-1.docker.io/v2/; do
  printf '%s %s\n' "$(curl -sS -o /dev/null -m 15 -w '%{http_code}' "$u" 2>/dev/null || echo ERR)" "$u"
done
```

| Requirement | Hard? | If missing |
|---|---|---|
| Docker daemon usable by this user | **yes** | if the client is installed but the daemon isn't running and this is a throwaway host where you are root, start it (`dockerd > $SCRATCH/dockerd.log 2>&1 &`, then poll `docker info`); otherwise stop: report "Docker unavailable"; the plan can still be produced. |
| Go matching `yb-voyager/go.mod` | yes | install from go.dev, or use `GOTOOLCHAIN=auto` if the proxy allows toolchain downloads |
| JDK 17 (Debezium runtime) | yes | distro package (`openjdk-17-jdk-headless`); JDK 18/19 also pass the installer check, 20+ do not |
| Debezium server under `/opt/yb-voyager/debezium-server` | yes | Step 2 |
| `pg_dump`, `pg_restore`, `psql` **≥ source PG major version** | yes (export data checks them even in live mode) | Step 3 |
| Maven | only for a local Debezium build | Step 2b |

## 1. Clean process environment

```bash
unset CONTROL_PLANE_TYPE YUGABYTED_DB_CONN_STRING   # otherwise every export fails at startup
export YB_VOYAGER_SEND_DIAGNOSTICS=0
export LANG=C.UTF-8 LC_ALL=C.UTF-8                  # see below
export JAVA_HOME=<jdk17 home>; export PATH=$JAVA_HOME/bin:$PATH
```

**Locale.** Minimal containers often run with a non-UTF-8 locale (`LANG=C`, or `setlocale` warnings in shell output). The Debezium JVM's default charset follows it, and under `C` it writes non-ASCII text to the CDC queue as `?` — a real voyager bug, found by the first cloud hunt. Run the default suite under `C.UTF-8` so that bug doesn't mask everything else; test the `C` locale only in cases that target it on purpose.

## 2. Debezium server (prefer a prebuilt release)

Building Debezium locally pulls artifacts from Maven repos outside Maven Central (e.g. `packages.confluent.io`, `jitpack.io`) and GitHub `/archive/` tarballs, which restricted networks commonly block. A prebuilt server from a voyager GitHub release is usually enough.

**2a. Prebuilt (default).** Pick the newest release tag whose Debezium plugin and connector versions match the target commit:

```bash
git fetch -q --tags origin
for tag in $(git tag -l 'yb-voyager/*' --sort=-creatordate | head -5); do
  n=$(git diff --stat "$tag" <target> -- debezium-server-voyager yb-voyager/versions/yb-cdc-connector-versions.json | tail -1)
  echo "$tag :: ${n:-identical}"
done
```

- Identical → download `https://github.com/yugabyte/yb-voyager/releases/download/<tag>/debezium-server.tar.gz` (release assets are usually reachable even when `/archive/` isn't), extract to `/opt/yb-voyager/debezium-server`, and record the tag.
- Differences only in files unrelated to the change set → still acceptable; record the diff in the report.
- Differences in code the change set touches (the change is *in* the Debezium plugin) → you must build locally (2b); if that's impossible, skip `value-encoding` cases that depend on the change and say so.

**2b. Local build (fallback).** `yes | bash installer_scripts/install-yb-voyager -l -p` (PG-only) from the repo root. Known blockers and workarounds — apply only the ones you hit, in a **scratch copy** of the installer, never in the repo:
- unreachable third-party apt repos (PPAs) → disable those source files and `apt-get update`
- `apt.postgresql.org` blocked → skip the installer's PostgreSQL-client step and use Step 3
- GitHub `/archive/<sha>.tar.gz` blocked → fetch the same commit with `git init; git fetch --depth 1 <repo-url> <sha>; git archive` (git over HTTPS usually works)
- Maven artifacts outside Maven Central blocked → no workaround; use 2a

## 3. PostgreSQL client tools ≥ source version

`export data` refuses to start ("Missing dependencies") if `pg_dump`/`pg_restore`/`psql` are older than the source PG major version (the tests use PG 17 unless `PG_VERSION` is set). Options, in order:
1. distro or PGDG packages of the right major version;
2. if the package repo is blocked: thin wrappers that run the tools from the same `postgres:<major>` image the tests pull, placed first on PATH:

```bash
for b in pg_dump pg_restore psql; do cat > $BIN/$b <<'EOF'
#!/bin/bash
exec docker run --rm -i --network host -v /tmp:/tmp -v "$PWD":"$PWD" -w "$PWD" \
  -e PGPASSWORD -e PGSSLMODE -e PGUSER -e PGHOST -e PGPORT -e PGDATABASE postgres:17 "$(basename "$0")" "$@"
EOF
chmod +x $BIN/$b; done
pg_dump --version   # must print the expected major version
```

Mount every directory the tools write to (`/tmp` covers testcontainers export dirs on Linux; add the scratch dir if the export dir lives elsewhere).

## 4. Build the binary under test

```bash
cd <worktree>/yb-voyager && go build -o $BIN/yb-voyager . && export PATH=$BIN:$PATH
which yb-voyager   # must resolve to $BIN/yb-voyager
```

`GIT_COMMIT_HASH` is only filled in for `git archive` builds, so `yb-voyager version` prints `VERSION=main` without a hash for a plain `go build`. Verify the binary by path and build time instead (`which yb-voyager`, `ls -l $BIN/yb-voyager` newer than the checkout), not by the hash.

## 5. Pre-pull images and size parallelism

```bash
docker pull postgres:${PG_VERSION:-17}
docker pull yugabytedb/yugabyte:<latest stable from yb-voyager/versions/yb-versions.json, or $YB_VERSION>
```

Parallelism `P = clamp(floor((DockerMemGB - 1) / 1.4), 1, 4)`, also capped by `nproc / 2`.

## 6. Smoke test before the real batches

Run one existing, fast live test (e.g. `TestBasicLiveMigrationWithCutover`) with `-run '^Name$'` and confirm it passes. This catches a broken environment in ~2 minutes instead of after a whole batch times out.

## Portable shell habits

- Put `grep` options before the pattern and paths (`grep -rn --include='*.go' -e PATTERN dir`); some environments set `POSIXLY_CORRECT` or ship a non-GNU grep, where options after paths are treated as file names.
- Don't rely on shell state persisting between commands; export env vars in each command or in a sourced env file (`$SCRATCH/env.sh`).
- Run long `go test` invocations in the background and wait for completion; poll logs with a filter (see harness → Running).

## Claude Code cloud sessions

Observed in Claude Code on the web / routines (Linux container, runs as root):

- **Docker is installed but the daemon may not be running** at session start (`docker info` fails on `/var/run/docker.sock`). The session runs as root, so start it with `dockerd > $SCRATCH/dockerd.log 2>&1 &` and poll `docker info`. Images pull from Docker Hub.
- **Network goes through an allowlisting proxy** (`HTTPS_PROXY` is set; `curl "$HTTPS_PROXY/__agentproxy/status"` lists `noProxy` and status). Reachable: GitHub over git, GitHub release assets, Maven Central, Go module proxy, Docker Hub, apt Ubuntu archives. Blocked (403 on CONNECT): third-party PPAs, `apt.postgresql.org`, GitHub `/archive/` tarballs, `packages.confluent.io`, `jitpack.io`. So: Step 2a for Debezium, Step 3 option 2 for PG tools.
- **Debezium behind the proxy.** In the first cloud run, Debezium exited at startup ("Failed to start application" in `<export-dir>/logs/debezium-source_db_exporter.log`) until the test runner's proxy settings were fixed. Connections from the exporter to the test containers must not go through the proxy: make sure `NO_PROXY`/`no_proxy` cover `localhost`, `127.0.0.1` and the Docker host address, and that any proxy system properties passed to Java (`JAVA_TOOL_OPTIONS`, `JAVA_OPTS`) include `-Dhttp.nonProxyHosts='localhost|127.*|[::1]'`. Check that log whenever exports fail without a voyager error.
- **Locale is not UTF-8 by default** (the installer prints `setlocale` warnings): set `LANG`/`LC_ALL=C.UTF-8` as in Step 1.
- Go and Maven are preinstalled; the default JDK may be 21, which the installer rejects — install `openjdk-17-jdk-headless` with apt and point `JAVA_HOME` at it.
- `JAVA_TOOL_OPTIONS` is preset with the proxy and truststore settings (visible as `Picked up JAVA_TOOL_OPTIONS` on every `java` call). Keep it, and check that its `-Dhttp.nonProxyHosts` covers the local addresses the exporter uses.
- The repo is checked out under `/home/user/<repo>`; use a scratch dir under `/tmp` and a separate worktree for the target commit.
- GitHub access for branches and PRs is available (via `git push` and the GitHub tools/`gh` if present). Slack posting uses the routine's attached Slack connector.
- Sessions are long but not unbounded: respect `--time-budget`, and write the report incrementally so a cut-off run still leaves results.
