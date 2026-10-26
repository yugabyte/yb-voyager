---
name: upgrade-go-version
description: >-
  Upgrade the Go toolchain version used to build and test yb-voyager: pick a
  stable target version, bump every pin (go.mod, ci-config.json, installer,
  Dockerfile, AGENTS.md), keep go.mod/go.sum clean, and validate with vet,
  golangci-lint across all build-tag groups, unit tests, and integration tests
  before opening a PR. Use when asked to upgrade/bump Go, golang, the Go
  toolchain, or the `go` directive in go.mod.
---

# Upgrade the Go version

Previous upgrades for reference: #769 (→1.20), #1717 + #1723 + #1762 (→1.23.1), #3149 (→1.24.2), and the 1.24.2 → 1.26.10 upgrade that added this skill.

## Step 0: Pick the target version and confirm with the user

```bash
curl -s 'https://go.dev/dl/?mode=json&include=all' | python3 -c "import json,sys; [print(r['version']) for r in json.load(sys.stdin)[:30] if r['stable']]"
curl -s https://go.dev/doc/devel/release | grep -oE 'go1\.[0-9]+\.0 \(released [0-9-]+\)' | head -4
```

- Go supports the two newest minor lines. Default to the **latest patch of the line that has been out for ~6+ months** (e.g. in Oct 2026: 1.26.x, released Feb 2026, rather than 1.27.x, released Aug 2026). Pick the newest line only if the user asks for it.
- Use a full patch version (`1.26.10`), never `1.26` or `1.26.0`.
- **Ask the user to confirm the version before editing anything.**

## Step 1: Check tool compatibility for the target version

| Tool | Pin location | Constraint |
| --- | --- | --- |
| golangci-lint | `yb-voyager/versions/ci-config.json` → `golangci_lint` | Must be built with Go ≥ target. Check `go.mod` of the pinned tag: `curl -s https://raw.githubusercontent.com/golangci/golangci-lint/<tag>/go.mod \| grep ^go`. If it is older than the target, bump the pin (the user must agree) and re-run lint. |
| delve | `docker/Dockerfile` (`go install .../dlv@vX`) | `curl -s https://raw.githubusercontent.com/go-delve/delve/<tag>/pkg/goversion/compat.go \| grep MaxSupportedVersionOfGoMinor` must be ≥ target minor. |
| golang Docker image | `docker/Dockerfile` (`FROM golang:X`) | `docker manifest inspect golang:<version>` must succeed. |
| Homebrew `go@X.Y` | release pipelines (Homebrew formula) | `curl -s https://formulae.brew.sh/api/formula/go@X.Y.json` must exist and not be deprecated. |
| Installer tarballs | `installer_scripts/install-yb-voyager` | `curl -sIL -o /dev/null -w '%{http_code}' https://golang.org/dl/go<version>.<os>-<arch>.tar.gz` → 200 for linux/darwin × amd64/arm64. |

## Step 2: Bump every pin

All of these must change together (`grep -rn "<old-version>"` over the repo, excluding `go.sum`/test data, to confirm none are missed):

| File | What |
| --- | --- |
| `yb-voyager/go.mod` | `go X.Y.Z` directive |
| `yb-voyager/versions/ci-config.json` | `versions.go`. The workflows that call `setup-go` (go, lint, build-voyager, integration-tests, failpoint-tests, issue-tests) read it through `prepare-versions.yml`, which **fails the build if it differs from go.mod**. |
| `installer_scripts/install-yb-voyager` | `GO_VERSION="X.Y.Z"` in `install_golang()`. The pg-13/17/18, mysql and misc migtest workflows have no `setup-go`, so they get Go only from this pin (or the runner's Go if it is already newer). No CI check compares it with go.mod. |
| `docker/Dockerfile` | `FROM golang:X.Y.Z` |
| `AGENTS.md` (root) | two `Go X.Y.Z` mentions. Edit `AGENTS.md`, not the `CLAUDE.md` symlink. |

Do **not** add a `toolchain` line to go.mod.

## Step 3: Get the toolchain and clean the module files

No global install is needed. With the default `GOTOOLCHAIN=auto`, any local `go` downloads the version named in `go.mod` into the module cache:

```bash
cd yb-voyager
go version            # should print the target version
go mod tidy
go mod tidy -diff && echo TIDY_CLEAN
go mod verify
go build ./...
```

`go.sum` should normally stay unchanged by a toolchain-only bump. If tidy changes `go.sum` or dependency versions, look at why before keeping it. Upgrading dependencies is a separate concern (#3149 bundled pgx; prefer separate PRs).

## Step 4: Fix new vet / compiler findings

Newer Go versions tighten `go vet`. Example: 1.24's printf check flagged non-constant format strings, and #3149 renamed `utils.PrintAndLog(fmt, args...)` calls to `utils.PrintAndLogf`. Run vet for every tag group (CI's `go test` runs vet too):

```bash
for t in "" unit integration integration_voyager_command integration_live_migration issues_integration failpoint_export failpoint_import failpoint_cutover cdc_benchmark manual yb_version_latest_stable connector_latest_stable; do
  go vet ${t:+-tags $t} ./... || echo "VET FAIL: $t"
done
```

Read the Go release notes (`https://go.dev/doc/go1.N`) for every minor version crossed. Watch for vet, `GODEBUG` default, crypto/TLS, and `net/http` changes, which can change runtime behavior without breaking the build.

## Step 5: Lint

golangci-lint must be the CI pin and built with Go ≥ the target. If it is not on `PATH`, build it into a scratch dir instead of piping the install script into `sh`:

```bash
GOBIN=<scratch>/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(jq -r .versions.golangci_lint yb-voyager/versions/ci-config.json)
env PATH="<scratch>/bin:$(dirname "$(which go)"):/usr/bin:/bin" bash yb-voyager/scripts/lint.sh
```

All 13 tag groups must report `0 issues.`

## Step 6: Tests

1. **Unit** (required locally): `go test -tags unit ./...`
2. **Integration (testcontainers, needs Docker)**, the same env as CI:
   ```bash
   YB_VOYAGER_SEND_DIAGNOSTICS=0 YB_VERSION=$(jq -r .latest_stable versions/yb-versions.json) \
   PG_VERSION=17 ORACLE_VERSION=21 MYSQL_VERSION=8.4 \
   go test -tags integration -timeout 60m -parallel 4 ./...
   ```
   Known local limitation: on an **arm64 Mac**, `gvenzl/oracle-xe:21` is amd64-only and exits during `TestMain`, so `src/srcdb` and `src/tgtdb` fail with `Failed to start oracle container`. That is environmental. Those two packages are validated by CI on ubuntu amd64. Every other package must pass.
3. **CI-only tiers.** These need a built/installed voyager, Debezium, failpoint rewriting, or a yugabyted tarball on Linux, and run in GitHub Actions on the PR: `integration_voyager_command`, `integration_live_migration`, `integration_live_migration_with_failpoint`, `issues_integration` (all YB versions), `yb_version_latest_stable`, `connector_latest_stable`, `failpoint_*`, plus `build-voyager.yml` (installer end to end) and the PG/Oracle/MySQL migtests.

## Step 7: PR

- Branch `upgrade-go-<version>`. Commit only the pin files plus any vet/lint fixes.
- PR body: old → new version, why this version, tool compatibility checked (golangci-lint, delve, Docker image, tarballs), and local results (vet, lint 13/13, unit, integration with the Oracle caveat).
- Merge only after **all** GitHub Actions checks are green. The CI-only tiers are the real validation for the installer and live-migration paths.
- The release build pipelines (apt, rhel8, rhel9, airgapped variants, and the Homebrew formula's `go@X.Y` build dependency) pin Go separately and need a matching PR. Land it alongside this one.
- After merge, update any Jenkins/migtest build agents that pin Go separately. The installer skips its own download when the agent's existing Go is already ≥ `GO_VERSION`.
