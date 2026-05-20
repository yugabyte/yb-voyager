---
name: update-yb-latest-stable
description: >-
  Bump YugabyteDB latest_stable in yb-versions.json, coordinate Jenkins migtest
  cluster upgrades, fix version-gated issue/integration tests, and open a PR only
  after all GitHub Actions pass. Use when updating latest stable YB version,
  yb-versions.json, Jenkins YB cluster, or Voyager supported YB versions.
---

# Update Latest Stable YugabyteDB Version

Single source of truth: `yb-voyager/versions/yb-versions.json`. GitHub Actions read it via `.github/workflows/prepare-versions.yml`. Jenkins migtests use a **separate** long-lived YB cluster (env vars), not this file directly.

## Before you start

1. Confirm the new stable release tag from [YugabyteDB releases](https://docs.yugabyte.com/preview/releases/ybdb-releases/) (full string with build, e.g. `2025.2.4.0-b150`).
2. Confirm the Docker image exists: `yugabytedb/yugabyte:<tag>`.
3. Create a branch: `update-yb-latest-stable-<version>` (e.g. `update-yb-latest-stable-2025.2.4.0`).

## Step 1: Update `yb-versions.json`

Edit `yb-voyager/versions/yb-versions.json`. **How** you change `version[]` depends on the bump type.

Always set `latest_stable` to the new full tag (e.g. `2025.2.4.0-b150`).

### A. Same stable series — patch/minor (e.g. `2025.2.3` → `2025.2.4`)

Replace the existing entry for that series **in place**; do **not** prepend a second `2025.2.*` line.

```json
// Before
"version": ["2025.2.3.0-b149", "2025.1.3.2-b1", ...],
"latest_stable": "2025.2.3.0-b149"

// After
"version": ["2025.2.4.0-b150", "2025.1.3.2-b1", ...],
"latest_stable": "2025.2.4.0-b150"
```

### B. New stable series released (e.g. `2025.2` becomes latest stable)

**Prepend** the new series tag to `version[]`. Keep older series (e.g. `2025.1.*`) in the list for CI matrix coverage unless maintainers retire them.

```json
// Before
"version": ["2025.1.3.2-b1", "2024.2.8.0-b85", ...],
"latest_stable": "2025.1.3.2-b1"

// After — prepend 2025.2.*, keep 2025.1.*
"version": ["2025.2.3.0-b149", "2025.1.3.2-b1", "2024.2.8.0-b85", ...],
"latest_stable": "2025.2.3.0-b149"
```

Only remove an older series entry when the team explicitly drops Voyager support for that release line.

### C. Adding support without changing latest stable

Rare — only when bumping `version[]` for matrix coverage while `latest_stable` stays on another tag. Coordinate with maintainers.

### Rules (all cases)

- **`latest_stable` must exactly match one entry in `version`** (enforced in `prepare-versions.yml`).
- **At most one tag per stable series** in `version[]` (e.g. one `2025.2.*`, not both `2025.2.3` and `2025.2.4` — use in-place replace for that case).
- Version format: `A.B.C.D-b<N>` (four numeric segments + build).
- Order: newest / current stable series first is the convention.

Validate locally:

```bash
bash .cursor/skills/update-yb-latest-stable/validate-yb-versions.sh
```

## Step 2: Code that auto-tracks `latest_stable` (usually no edit)

These read `yb-versions.json` at build/runtime — **no change** for a typical bump:

- `yb-voyager/versions/versions.go` (`GetLatestStableYBVersion`, embedded JSON)
- `yb-voyager/src/ybversion/constants.go` — `LatestStable` is set from `versions.GetLatestStableYBVersionWithoutBuildNumber()` in `init()`
- All workflows using `prepare-versions` outputs (`yb_latest_stable`, `yb_versions` matrix)

## Step 3: Issue tests — required after every bump

YB version bumps often change error messages or fix issues. **Always run and fix** the issue integration tests before opening the PR.

### DDL / DML issue tests (mandatory)

From `yb-voyager/`:

```bash
export YB_VERSION=$(jq -r '.latest_stable' versions/yb-versions.json)

# All supported versions (matches CI issue-tests matrix)
for v in $(jq -r '.version[]' versions/yb-versions.json); do
  echo "=== DDL issues @ $v ==="
  YB_VERSION="$v" go test -tags issues_integration -run '^TestDDLIssuesInYBVersion$' -count=1 -timeout=30m ./src/query/queryissue/

  echo "=== DML issues @ $v ==="
  YB_VERSION="$v" go test -tags issues_integration -run '^TestDMLIssuesInYBVersion$' -count=1 -timeout=30m ./src/query/queryissue/
done

# Latest-stable-only tag (CI also runs this)
go test -tags 'issues_integration,yb_version_latest_stable' -count=1 -timeout=30m ./src/query/queryissue/...
```

Files to inspect when tests fail:

- `yb-voyager/src/query/queryissue/issues_ddl_test.go` — `TestDDLIssuesInYBVersion`
- `yb-voyager/src/query/queryissue/issues_dml_test.go` — `TestDMLIssuesInYBVersion`
- `yb-voyager/src/query/queryissue/issues_ddl.go` / `issues_dml.go` — `MinimumVersionsFixedIn`, issue definitions
- `yb-voyager/src/query/queryissue/issues_ddl_test.go` — `assertErrorCorrectlyThrownForIssueForYBVersion`

### Debugging and fixing failing issue tests

For each failure:

1. **Identify the subtest** — use `-run 'TestDMLIssuesInYBVersion/copy_from_where'` (or the failing `t.Run` name).
2. **Compare exec error vs expectation** — log `err` from `conn.Exec`; YB may return a different message or no error on newer builds.
3. **Check `issue.IsFixedIn(testYbVersion)`** — if the issue is marked fixed for this series, tests expect **success** (`assert.NoError`), not the old error string.
4. **Add version gates** when behavior changes at a specific 3-dot release (e.g. `ybversion.V2025_2_3_0`, `packaging.version.Version` in migtest `validate` scripts).
5. **Add `ybversion` constants** in `src/ybversion/constants.go` only when tests compare against a new `V*` constant.
6. Re-run the single failing subtest, then the full `TestDDLIssuesInYBVersion` / `TestDMLIssuesInYBVersion` for that `YB_VERSION`.

External YB (matches local psql instead of testcontainers):

```bash
export YB_CONN_STR="postgresql://yugabyte:yugabyte@127.0.0.1:5433/yugabyte?sslmode=disable"
YB_VERSION=$(jq -r '.latest_stable' versions/yb-versions.json)
go test -tags issues_integration -run '^TestDMLIssuesInYBVersion$' -count=1 -v ./src/query/queryissue/
```

### Other tests that commonly fail on version bumps

Run and fix as needed (do not ignore red CI):

| Test area | Command / workflow |
|-----------|-------------------|
| Issue tests (all) | `go test -tags issues_integration ./src/query/queryissue/...` |
| Latest-stable tag | `go test -tags yb_version_latest_stable ./...` |
| Integration / live migration | `.github/workflows/integration-tests.yml` |
| Failpoints | `.github/workflows/failpoint-tests.yml` |
| Migtests | `pg-13-migtests`, `pg-17-migtests`, `mysql-migtests`, `misc-migtests` |
| Unit | `go test -tags unit ./...` |

**If any test fails:** reproduce locally, root-cause against the new YB behavior, fix test expectations or product code in this PR, re-run until green. Do not merge with failing or skipped issue tests.

```bash
rg "2025\.2\.3|V2025_2_3|latest_stable" yb-voyager migtests
```

## Step 4: Optional follow-up edits (product behavior)

| Area | When to touch |
|------|----------------|
| `yb-voyager/src/ybversion/constants.go` | New **3-dot** gate needed |
| `migtests/tests/**/validate` | Version-specific extension or behavior checks |
| `yb-voyager/config-templates/*.yaml` | Docs mention minimum YB version |
| `yb-voyager/cmd/*.go` help text | User-facing minimum version strings |

## Step 5: Jenkins pipeline (external cluster)

Jenkins config is **not** in this repo. Migtests run via `migtests/scripts/jenkins-wrapper` against a cluster set by Jenkins job env vars (`TARGET_DB_HOST`, `TARGET_DB_PORT`, etc. — see `migtests/scripts/yugabytedb/env.sh` defaults).

**Required (human / infra):**

1. Upgrade the Jenkins shared YugabyteDB cluster to `<NEW_VERSION>` (same build as `latest_stable`).
2. Re-run the Jenkins migtest job(s) for this PR branch after upgrade.
3. Verify:

```bash
psql "postgresql://yugabyte:<password>@<TARGET_DB_HOST>:<TARGET_DB_PORT>/yugabyte" \
  -c "SELECT version();"
```

**PR description must note:** Jenkins cluster upgraded (or infra ticket) and Jenkins job name + result.

Do **not** change `migtests/scripts/yugabytedb/env.sh` default host unless intentionally moving the shared dev cluster for everyone.

## Step 6: Open PR and monitor CI

Push branch and create PR to `main`. Use the `pr-description` skill.

```bash
git push -u origin HEAD
gh pr checks --watch
gh pr checks --json name,state,link --jq '.[] | select(.state != "SUCCESS")'
```

Use the `babysit` skill to fix CI failures in a loop (do not weaken workflows to force green).

## DO NOT MERGE until

- [ ] `validate-yb-versions.sh` passes
- [ ] **`TestDDLIssuesInYBVersion` passes** for every entry in `yb-versions.json` `version[]` (run locally before push; confirmed green in **Issue Tests** workflow)
- [ ] **`TestDMLIssuesInYBVersion` passes** for every entry in `version[]` (same)
- [ ] **`yb_version_latest_stable` issue tests pass** (`go test -tags yb_version_latest_stable ./src/query/queryissue/...`)
- [ ] **Any other failing test is debugged and fixed in this PR** — reproduce locally, update expectations or code, re-run the failing package/workflow; no “known failure” left behind without maintainer sign-off
- [ ] All required **GitHub Actions** checks are green (see table below)
- [ ] **Jenkins** migtests run against the upgraded cluster (if required for this repo/team) with result linked in the PR
- [ ] No unresolved review comments on the version bump

### GitHub Actions checklist

| Workflow | What it exercises |
|----------|-------------------|
| `Go` | Unit tests, staticcheck |
| `Build Voyager` | Installer / build |
| `Integration Tests` | Live migration @ `yb_latest_stable` |
| `Failpoint Tests` | Failpoint flows @ `yb_latest_stable` |
| `Issue Tests` | **DDL/DML issue tests** matrix over all `version[]` + `yb_version_latest_stable` |
| `PG 13: Migration Tests` | Migtests matrix per `yb_versions` |
| `PG 17: Migration Tests` | Migtests matrix per `yb_versions` |
| `MySQL: Migration Tests` | Migtests matrix per `yb_versions` |
| `Misc Migration Tests` | Import-file tests per `yb_versions` |

## Common failures after a bump

| Failure | Likely fix |
|---------|------------|
| `prepare-versions` — `latest_stable` not in `version` | Fix JSON alignment |
| `TestDDLIssuesInYBVersion` / `TestDMLIssuesInYBVersion` | Update `assertErrorCorrectlyThrownForIssueForYBVersion` expected strings, `MinimumVersionsFixedIn`, or version branches |
| Issue tests — testcontainers vs local YB differ | Use `YB_CONN_STR` for local cluster; document docker image gap |
| Migtests — extension list in `validate` | Version-gate `expectedExtensions` (e.g. `pg_parquet` only before 2025.2.3) |
| Integration tests — image pull | Ensure `yugabytedb/yugabyte:<tag>` exists |

## PR title / commit message

```
chore: bump latest stable YugabyteDB to <NEW_VERSION>
```
