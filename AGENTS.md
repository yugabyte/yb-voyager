# AGENTS.md

## Cursor Cloud specific instructions

### Project overview

YugabyteDB Voyager (`yb-voyager`) is a database migration CLI tool (Go) with a Debezium CDC plugin (Java/Maven). See `README.md` for details.

### Key environment requirements

- **Go 1.24.2** — required by `go.mod`. Installed at `/usr/local/go/bin/go`.
- **JDK 17** (not 21+) — the installer script enforces Java 17–19 for the Debezium build. Installed at `/usr/lib/jvm/java-17-openjdk-amd64`. Set `JAVA_HOME` accordingly.
- **Maven 3.8.4** — auto-installed by the installer to `/opt/yb-voyager/yb-debezium-maven-3.8.4`.
- **PostgreSQL client 17** — provides `pg_dump`/`pg_restore`.
- **YugabyteDB** — installed at `/opt/yugabyte-2025.2.1.0`. The target database for migrations.
- **PostgreSQL 17 server** — local source database for development/testing.

### Building from source

Use the installer script: `yes | bash installer_scripts/install-yb-voyager -l -p` for PG-only, or without `-p` for full Oracle/MySQL support. The `-l` flag builds from local source. The binary is installed to `/usr/local/bin/yb-voyager`.

Ensure these env vars are set before running:
```
export JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64
export PATH=/usr/local/go/bin:$JAVA_HOME/bin:$PATH
```

To rebuild only the Go binary after code changes (faster):
```
yes | bash installer_scripts/install-yb-voyager -v
```

### Running tests

- Unit tests use the `unit` build tag: `go test -tags unit ./...` (from `yb-voyager/` directory).
- Integration tests use tags like `issues_integration` and require Docker + testcontainers.
- Lint: `go vet ./...` and `staticcheck -tags unit ./...` (from `yb-voyager/`).

### Running local databases

**PostgreSQL** (source):
- Already running via systemd on port 5432. User `postgres`, password `postgres`.
- Start if stopped: `sudo pg_ctlcluster 17 main start`

**YugabyteDB** (target):
- Start: `/opt/yugabyte-2025.2.1.0/bin/yugabyted start --advertise_address 127.0.0.1`
- Status: `/opt/yugabyte-2025.2.1.0/bin/yugabyted status`
- Stop: `/opt/yugabyte-2025.2.1.0/bin/yugabyted stop`
- YSQL on port 5433, user `yugabyte`, password `yugabyte`.
- ysqlsh: `/opt/yugabyte-2025.2.1.0/bin/ysqlsh -U yugabyte -d <dbname> -h 127.0.0.1`
- The YugabyteDB data directory must be owned by `ubuntu` (not root). If you get FIPS/permission errors on start, run `sudo chown -R ubuntu:ubuntu /opt/yugabyte-2025.2.1.0`.

### Gotchas

- The installer script has interactive prompts (ora2pg license, bashrc update). Pipe `yes` to accept them non-interactively.
- The installer's Java check enforces max version 19. JDK 21+ will fail the check. Always use JDK 17.
- The `-l` (local build) flag requires `rsync` to be installed on the system.
- Lock files (`.lck`) in the export directory can block re-runs; remove them if a previous `yb-voyager` process was killed.
- Set `YB_VOYAGER_SEND_DIAGNOSTICS=0` to disable telemetry during development/testing.
