# YB-EXT SQL Parser Fork — Proof of Concept

This document captures the state of a proof-of-concept that teaches voyager's SQL
parser to accept YugabyteDB-specific syntax extensions (e.g. `SPLIT INTO N TABLETS`,
`PRIMARY KEY (col HASH)`) while still tracking PostgreSQL 17 grammar.

The PoC delivered an end-to-end working pipeline for **one** YB extension
(`HASH` as a column-ordering modifier) and laid down the reproducible regen
workflow, tooling, and patch format that any future YB extension would reuse.

---

## 1. Why this exists

Voyager's schema migration flow funnels every SQL statement through
`github.com/pganalyze/pg_query_go` (libpg_query under the hood, PG 17 base):

```
export schema   →  analyze schema  →  import schema
       │                  │                  │
       └──────────────────┴──────────────────┘
                          │
                pg_query_go.Parse(sql)
```

Because pg_query_go only knows PG 17 grammar, any YB-dialect clause fails at
the parser with `syntax error at or near "SPLIT"`, short-circuiting the entire
analyze/import pipeline. User-edited schemas containing YB clauses cannot be
re-processed by voyager.

We cannot switch to a YB-only parser either — voyager still needs the PG 17
parser to *detect* features new in PG 16/17 that won't run on YB (which is
PG 15-compatible).

**Goal**: a parser that accepts the *union* of PG 17 + YB grammar extensions.

---

## 2. Architecture

There are four distinct layers that move SQL into Go data structures:

```
┌─────────────────────────────────────────────────────────────────────┐
│  yb-voyager   (Go)                                                  │
│  imports github.com/pganalyze/pg_query_go/v6                        │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ go build → cgo
┌──────────────────────────────▼──────────────────────────────────────┐
│  pg_query_go   (Go module, ~1% Go + ~99% vendored C)                │
│  pg_query.go        — public Go API (Parse, Deparse, Normalize…)    │
│  pg_query.pb.go     — generated from pg_query.proto (protoc-gen-go) │
│  parser/*.c, *.h    — SNAPSHOT of libpg_query's output (vendored)   │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ make update_source (re-snapshot)
┌──────────────────────────────▼──────────────────────────────────────┐
│  libpg_query   (C library — the actual parser owner)                │
│  ┌────────────────────────────────────────────────────────────┐     │
│  │ HAND-WRITTEN (source-of-truth)                             │     │
│  │   patches/*.patch          ← modifications to PG sources   │     │
│  │   srcdata/*.json           ← AST metadata for code-gen     │     │
│  │   src/postgres_deparse.c   ← AST → SQL emitter             │     │
│  │   src/pg_query_*.c         ← entry points / utilities      │     │
│  └────────────────────────────────────────────────────────────┘     │
│  ┌────────────────────────────────────────────────────────────┐     │
│  │ AUTO-GENERATED (committed)                                 │     │
│  │   protobuf/pg_query.proto, pg_query.pb-c.{c,h}             │     │
│  │   src/include/pg_query_enum_defs.c                         │     │
│  │   src/postgres/src_backend_parser_gram.c    ← bison output │     │
│  │   src/postgres/include/.../parsenodes.h     ← from PG      │     │
│  └────────────────────────────────────────────────────────────┘     │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ make extract_source (rebuild from PG)
┌──────────────────────────────▼──────────────────────────────────────┐
│  PostgreSQL 17.4 source tarball  (transient — lives in tmp/)        │
│  Real `gram.y`, `scan.l`, `parsenodes.h`, `kwlist.h`, etc.          │
│  Patches are applied here; bison runs here; outputs harvested.      │
└─────────────────────────────────────────────────────────────────────┘
```

### Where the YB content actually lives

Three files in our `libpg_query` fork carry every YB extension:

```
third_party/libpg_query/
├── patches/99_yb_<feature>.patch   ← gram.y + parsenodes.h + kwlist.h hunks
├── srcdata/{enum,struct,…}_defs.json  ← AST shape descriptors for protoc-gen
└── src/postgres_deparse.c          ← hand-written deparse case
```

Everything else regenerates from these three inputs.

---

## 3. What landed in this PoC

### Working end-to-end demonstration

Voyager now accepts:

```sql
CREATE INDEX idx ON t (col HASH)              -- ✓ parses, ordering = SORTBY_HASH
CREATE INDEX idx ON t (c1 HASH, c2 ASC)       -- ✓ parses, deparse round-trip preserves HASH
```

Test file: `yb-voyager/src/query/queryparser/yb_hash_test.go`. All three test
cases (parse, deparse round-trip, AST field extraction) pass. The existing
unit suites for `queryparser`, `queryissue`, `sqltransformer` show zero
regression.

### Repository layout

```
yb-voyager/                              ← worktree root (this PoC branch)
├── Makefile                             ← top-level: pg-yb-parser-rebuild target
├── third_party/
│   ├── README.md                        ← contributor workflow
│   ├── POC.md                           ← (this document)
│   ├── libpg_query/                     ← fork of libpg_query @ 17-6.1.0
│   │   ├── Dockerfile.regen             ← Linux regen toolchain
│   │   ├── Makefile                     ← patched for portability + new targets
│   │   ├── patches/
│   │   │   ├── 01..10_*.patch           ← upstream libpg_query patches (untouched)
│   │   │   └── 99_yb_sortby_hash.patch  ← our YB-EXT patch (100 lines)
│   │   ├── srcdata/enum_defs.json       ← +5 lines for SORTBY_HASH
│   │   ├── scripts/extract_source.rb    ← patched for modern ffi-clang API
│   │   └── src/postgres_deparse.c       ← +10 lines (2 SORTBY_HASH cases)
│   └── pg_query_go/                     ← fork of pg_query_go @ v6.1.0
│       └── Makefile                     ← LIBDIR points at sibling libpg_query/
└── yb-voyager/
    ├── go.mod                           ← `replace … => ../third_party/pg_query_go`
    └── src/query/queryparser/
        └── yb_hash_test.go              ← new test file
```

### The full YB-EXT patch in one place

For reference, our `patches/99_yb_sortby_hash.patch` (~100 lines) modifies
three files in PG 17 sources:

1. `src/include/nodes/parsenodes.h` — adds `SORTBY_HASH` to the `SortByDir` enum
2. `src/include/parser/kwlist.h` — adds `HASH` as an unreserved keyword
3. `src/backend/parser/gram.y` — adds `HASH` to:
   - token list (line 738 in PG 17.4 gram.y)
   - `opt_asc_desc` production (line 8284)
   - `unreserved_keyword` and `bare_label_keyword` lists
   - declares `%nonassoc HASH; %nonassoc NO_OPCLASS` precedences
   - tags `opt_qualified_name`'s empty case with `%prec NO_OPCLASS`
     (resolves shift/reduce conflict)

Plus the matching `srcdata/enum_defs.json` entry and `postgres_deparse.c`
emit case. Total YB-specific surface: ~120 lines of source code across
three files.

---

## 4. The regen pipeline (how a feature gets in)

Once the fork is set up, the per-feature workflow is:

```sh
# 1. Author the patch (modify gram.y, parsenodes.h, kwlist.h)
$EDITOR third_party/libpg_query/patches/99_yb_<feature>.patch

# 2. Register new enum values / struct fields in srcdata
$EDITOR third_party/libpg_query/srcdata/enum_defs.json
$EDITOR third_party/libpg_query/srcdata/struct_defs.json   # for new node types

# 3. Add deparse case (always hand-written)
$EDITOR third_party/libpg_query/src/postgres_deparse.c

# 4. Wire patch into libpg_query Makefile PGDIR rule
$EDITOR third_party/libpg_query/Makefile

# 5. Regenerate (uses Docker — see toolchain notes below)
make pg-yb-parser-rebuild

# 6. Test
cd yb-voyager && go test -tags unit ./src/query/...
```

The `pg-yb-parser-rebuild` target chains:

```
docker run libpg_query-regen make extract_source
    → downloads PG 17.4 source zip
    → applies libpg_query's 01..10 patches + our 99_yb_*.patch
    → runs PG's configure + makefiles to produce gram.c, scan.c, kwlist_d.h
    → runs extract_source.rb to copy everything into src/postgres/

docker run libpg_query-regen make regen_proto
    → runs generate_protobuf_and_funcs.rb against srcdata/
    → regenerates pg_query.proto, pg_query_enum_defs.c,
      pg_query_outfuncs_defs.c, pg_query_readfuncs_defs.c
    → runs protoc-c to refresh pg_query.pb-c.{c,h}

cd ../pg_query_go && make update_source
    → copies regenerated C files into pg_query_go/parser/
    → runs protoc-gen-go to refresh pg_query.pb.go

cd ../../yb-voyager && go build ./...
    → cgo recompiles all .c files against the host's compiler
```

Total time: ~15–20 min cold (PG download + bison + libclang AST walk +
protoc + cgo build). On a warm repeat, ~2–3 min.

---

## 5. Toolchain bits we had to add

This is incidental complexity from libpg_query's regen pipeline never having
been engineered for non-maintainer use. We hit and fixed:

| Issue | Fix |
|---|---|
| `extract_source.rb` only emits Win32 stubs on macOS (libclang fails on a Win32 include despite `-DWIN32` not being set for that file — root cause unclear, but libpg_query CI only tests this on Linux anyway) | Run regen in an Ubuntu 24.04 Docker container |
| Hardcoded macOS `LIBCLANG=/Library/Developer/CommandLineTools/...` path in Makefile | Made recipe respect `${LIBCLANG:-default}` env var |
| BSD `sed -i ""` syntax fails on GNU sed (Linux) | Replaced with `sed -i` (Linux path only) |
| Script calls `xcrun --sdk macosx` (macOS-only) | Stubbed `xcrun` in Docker image to return empty string |
| Script references `FFI::Clang::Types::Array/Pointer` — constants that don't exist in any released ffi-clang gem | Replaced with method-based predicates `cursor.type.array?` / `cursor.type.pointer?` |
| ffi-clang ≥ 0.9 (the version with FFI::Clang::Type fully fleshed out) requires Ruby ≥ 3.1 | Switched Docker base from Ubuntu 22.04 to 24.04 (ships Ruby 3.2) |
| Stale `tmp/analysis/` cache from earlier runs gets consumed via `FileAnalysis.restore` and contaminates new runs | `make clean` wipes it (existing target — just need to call it) |
| protoc-c emits `#include "pg_query.pb-c.h"` but libpg_query's source layout expects `"protobuf/pg_query.pb-c.h"` | Added `sed` step to `regen_proto` target |
| New `regen_proto` Makefile target wasn't in libpg_query (it's maintainer-only) | Added it explicitly |

None of these are "YB-specific" fixes — they would all be needed for *any*
fork that wants to run libpg_query's regen pipeline outside the maintainer's
machine.

---

## 6. Known limitations

### a) `pg_config.h` is platform-specific (the one remaining manual step)

When `make extract_source` runs inside the Linux Docker container, PG's
`./configure` detects Linux capabilities and writes Linux-specific defines
into `src/postgres/include/pg_config.h` (e.g. `HAVE_DECL_FDATASYNC=1`,
`DLSUFFIX=".so"`). When voyager later does `go build` on **macOS**, cgo tries
to compile the C against the macOS SDK using those Linux defines and fails
with conflicting `strlcat` / `strlcpy` declarations.

Current workaround: after `make update_source`, manually `cp` upstream
pg_query_go's macOS-extracted `pg_config.h` back over the regenerated one.
That single file is the only remaining "bypass" — every other YB-EXT output
in `pg_query_go/parser/` is now a faithful product of the regen pipeline.

A proper fix is one of:

- Build/extract per-target-OS (we'd need a way to run a macOS environment in CI)
- Make `pg_config_overrides.h` more aggressive so OS-detected differences don't leak through
- Generate the platform-portable header during `make update_source` rather than copying it from upstream

### b) PoC coverage

The PoC implements **one** YB extension (`HASH` ordering). Other YB clauses —
SPLIT INTO, TABLEGROUP, COLOCATION, NONCONCURRENTLY — are not implemented but
should follow the same pattern. Of these, SPLIT INTO is the most complex
because it introduces a brand-new AST node type (`YbOptSplit`) — exercising
the `srcdata/struct_defs.json` + `srcdata/nodetypes.json` editing path that
HASH didn't need.

Additionally, `PRIMARY KEY (col HASH)` as inline column constraint doesn't
work yet — PG's PK column list uses a different grammar production
(`columnList`, not `index_elem_options`). Adding HASH there is a separate
grammar change.

### c) `extract_source.rb` on native macOS

We did not invest in making the regen pipeline work on a macOS host directly.
The official libpg_query maintainer presumably runs it on macOS, but we hit
issues we didn't fully diagnose and chose Docker-on-Linux as the pragmatic
path. macOS users today run `docker run libpg_query-regen make extract_source`
instead of running it natively.

### d) Voyager-side integration is minimal

Voyager's `queryparser` package can now parse YB syntax without erroring, but
it does **not** yet:

- Extract `SortByDir_SORTBY_HASH` into a structured field on the `Table` / `Index`
  DDL object (it just sits on `IndexElem.Ordering` in the protobuf AST)
- Report or rewrite anything in the analyze-schema flow
- Emit YB syntax during export-schema optimization

This was deliberately deferred — the PoC scope was **accept-only**. Voyager's
own behavior on YB syntax is a follow-up.

---

## 7. Path to production

### a) Repository structure: should the fork stay in `third_party/`?

**Recommendation**: For the immediate next phase, keep both forks vendored under
`third_party/` in the voyager repo. Reasons:

1. **Single source of truth for voyager developers.** Adding a YB grammar
   feature requires touching the patch + srcdata + deparse + maybe voyager-side
   extraction. Keeping all of it in one repo means one PR, one CI run, one
   review.
2. **Avoid premature repo-spawning.** A separate `yugabyte/libpg_query` repo
   would need its own CI, release process, owner, and tracking. Until we have
   multiple YB consumers (cdc, planner, etc.) reusing this parser, the
   maintenance overhead doesn't pay off.
3. **The third_party/ pattern is well-understood.** Most Go projects vendor
   forked C libraries this way (k8s vendors etcd, Docker vendors runc).

**When to split into separate repos** (graduation criteria):

- Any consumer beyond yb-voyager wants the YB-extended parser
- The YB-EXT patch surface grows beyond ~5 features (then the repo becomes
  meaningfully a fork with its own identity)
- We start needing to publish standalone libpg_query release tarballs (e.g.,
  for non-Go consumers like Python / Java tooling)

When that day comes, the migration is:

```
third_party/libpg_query → github.com/yugabyte/libpg_query
third_party/pg_query_go → github.com/yugabyte/pg_query_go
yb-voyager/go.mod:        replace … => github.com/yugabyte/pg_query_go (no longer ../path)
```

The patch files, srcdata, and deparse code move 1:1.

### b) Upgrading when upstream releases a new PG version

libpg_query bumps PG every ~6 months (e.g. 17-6.1.0 → 17-7.0.0 → 18-1.0.0).
Our update workflow:

```sh
# 1. Pull upstream's new release tarballs
LIB_PG_QUERY_TAG=18-1.0.0 GOLANG_TAG=v6.5.0      # whatever the new versions are

# 2. Replace the vendored libpg_query
cd third_party/libpg_query
git rm -r ./*    # caution: preserves patches/ if we move it aside first
curl -sSL -o /tmp/lpq.tar.gz \
    https://codeload.github.com/pganalyze/libpg_query/tar.gz/$LIB_PG_QUERY_TAG
tar -xzf /tmp/lpq.tar.gz --strip-components=1

# 3. Re-apply our YB-EXT patches against the new PG grammar
mv /tmp/yb_patches/* patches/        # restore the 99_yb_*.patch files we saved
docker run libpg_query-regen make extract_source
# → patch -p1 may fail with offsets / conflicts if upstream renamed productions
# → fix patches if needed, commit

# 4. Re-apply our srcdata edits
$EDITOR srcdata/enum_defs.json   # re-add SORTBY_HASH if upstream regenerated

# 5. Re-apply our postgres_deparse.c edit (the YB-EXT BEGIN/END fenced block)
$EDITOR src/postgres_deparse.c

# 6. Same for pg_query_go
cd ../pg_query_go
# similar tarball-replace, then re-apply our Makefile changes

# 7. Run full regen + voyager tests
cd ../..
make pg-yb-parser-rebuild
cd yb-voyager && go test -tags unit ./src/query/...
```

The patch-based approach pays off here: if upstream's gram.y around line 8284
didn't shift, our `99_yb_sortby_hash.patch` applies cleanly. If they renamed
`opt_asc_desc` to something else, the patch fails with a hunk reject and we
re-author the rule against the new context. In the worst case, "re-author the
patch from scratch" is still small work because each YB feature is ~30 lines
of grammar.

**Realistic estimate**: 1–3 days per PG bump per active YB feature, mostly
spent on patch conflicts in `gram.y` and verification testing. If we're
tracking *many* YB features, this scales linearly; we should batch upstream
bumps and re-do the patches together.

**Suggestion**: tag releases of our fork with `17-6.1.0-yb1`,
`17-6.1.0-yb2`, etc. After each upstream bump, increment the prefix.

### c) Outstanding work to ship

Before this lands in voyager's `main` branch and real users see it:

| Item | Effort | Owner |
|---|---|---|
| Wire upstream pg_query_go's macOS `pg_config.h` restore into the Makefile (auto, not manual `cp`) | half a day | TBD |
| Add a CI job that runs `make pg-yb-parser-rebuild` on every PR touching `third_party/libpg_query/` (catches patch drift) | 1 day | TBD |
| Add CI smoke test that voyager builds + parses sample YB-syntax SQL on both Linux and macOS runners | half a day | TBD |
| SPLIT INTO patch — exercises the new-node-type pattern (`srcdata/struct_defs.json` edits) | 2–3 days | TBD |
| Voyager-side: extract `SortByDir_SORTBY_HASH` into `Table.PrimaryKeyColumns` metadata, expose to assess-migration report | 1 day | TBD |
| Documentation: add a `CONTRIBUTING.md` section pointing at `third_party/README.md` for grammar contributors | half a day | TBD |
| Investigate native-macOS `extract_source.rb` (avoid Docker dependency for Mac devs)  | 1–2 days, risk: may not be feasible | TBD |
| Trim repo size — drop libpg_query's C++ protobuf bindings (11 MB unused) and test fixtures we don't need | half a day | TBD |

### d) Testing strategy

What CI should cover:

1. **Static**: `make pg-yb-parser-rebuild` exits clean. Anyone who edits
   `patches/`, `srcdata/`, or `src/postgres_deparse.c` will surface conflicts
   immediately.
2. **Parser unit tests**: in `pg_query_go` itself, add YB-syntax cases to
   `parse_test.go` / `parse_protobuf_test.go`. These run as part of libpg_query's
   own test suite via `make test`.
3. **Voyager queryparser tests**: `yb_hash_test.go` exists; extend with
   per-feature test files as we add SPLIT INTO etc.
4. **Voyager integration tests**: hand-crafted `schema/tables.sql` containing
   YB clauses, run through `yb-voyager analyze-schema` and `yb-voyager
   import-schema` against a local YB cluster. Assert no parse errors, assert
   tablet count via `yb_table_properties()`.
5. **Cross-platform compile**: voyager `go build` + `go test -tags unit` on
   both Ubuntu and macOS runners.
6. **Regression**: every existing `queryissue` / `queryparser` test must still
   pass with each YB-EXT addition.

### e) Maintenance burden — honest assessment

Forking a parser is a recurring tax. Concretely:

- **Per YB feature**: ~1–3 days of grammar design + patch authoring + tests
- **Per upstream libpg_query release** (~6 mo cadence): ~1–3 days of patch
  rebasing per active YB feature, plus toolchain re-verification
- **Per PG major bump** (~12 mo cadence): potentially more invasive — gram.y
  structure can shift between PG majors. Budget a week.
- **Per voyager release**: nothing extra, the fork comes along for the ride

In steady state with 5 YB features tracked, expect ~2 person-weeks/year for
maintenance, not counting new feature additions.

The alternative — staying on stock pg_query_go and only ever emitting/accepting
plain PG SQL — saves all this maintenance but blocks the use cases described
in section 1. The PoC validates the path; whether the maintenance cost is
worth it depends on how much voyager wants to do with YB-native syntax in
the next 2-3 years.

### f) Risks

- **libpg_query maintainer divergence.** If pganalyze restructures their
  parser-tree-walking conventions, our patches could become harder to rebase.
  Mitigation: keep YB-EXT changes minimal and fenced (`/* YB-EXT BEGIN */`).
- **PG 18 grammar churn.** PG occasionally restructures grammar productions
  meaningfully (e.g., MERGE was a big addition in PG 15). Mitigation: review
  PG release notes ahead of each libpg_query bump.
- **Docker dependency for the regen step.** Linux developers don't need it.
  macOS developers do. If voyager moves to CI-driven regen-on-push, this
  becomes invisible.
- **The unfixed `pg_config.h` issue.** Currently papered over with a manual
  `cp`. If the upstream macOS-extracted pg_config.h ever drifts incompatibly
  from our Linux-extracted one (e.g., a new `HAVE_*` define we'd need), we'd
  notice via cgo build failures. Mitigation: get a real fix in before relying
  on cross-platform regen at scale.
- **YB grammar evolution.** If YB upstream changes how SPLIT INTO works in a
  new YB release, we'd need to port the new behavior. This is a one-way diff —
  we read YB's grammar but don't depend on it at build time, so it's purely a
  documentation-tracking concern. Mitigation: subscribe to YB release notes.

---

## 8. Quick reference

### To verify what's working today

```sh
make pg-yb-parser-check    # build voyager + run parser/queryissue/sqltransformer tests
```

### To add a new YB feature (canonical workflow)

```sh
# 1. Author/edit
$EDITOR third_party/libpg_query/patches/99_yb_<feature>.patch
$EDITOR third_party/libpg_query/srcdata/enum_defs.json
$EDITOR third_party/libpg_query/src/postgres_deparse.c

# 2. Rebuild whole stack from sources
make pg-yb-parser-rebuild

# 3. Add a Go test
$EDITOR yb-voyager/src/query/queryparser/yb_<feature>_test.go

# 4. Verify
cd yb-voyager && go test -tags unit ./src/query/...
```

### Key files

```
third_party/README.md                                  ← detailed contributor guide
third_party/libpg_query/Dockerfile.regen               ← regen toolchain
third_party/libpg_query/patches/99_yb_sortby_hash.patch  ← worked example
third_party/libpg_query/Makefile                       ← extract_source, regen_proto
third_party/pg_query_go/Makefile                       ← update_source
yb-voyager/src/query/queryparser/yb_hash_test.go       ← worked-example test
```

---

## 9. Bottom line

The PoC proves the path works:

- One YB grammar feature flows end-to-end from a small patch file (~120 lines
  of YB-specific source) into voyager parsing real SQL
- The pipeline is reproducible from scratch — no hand-edits to vendored
  generated files (with the single pg_config.h exception called out above)
- Toolchain quirks have been ironed out; future contributors edit three files
  per feature and run one Makefile target

What's not yet done:

- Only one (the simplest) feature is implemented; SPLIT INTO and the rest are
  on the roadmap
- Voyager-side integration is minimal; analyze/import behavior on YB clauses
  is a follow-up
- CI is not wired up — currently all regen is manual via `make`
- `pg_config.h` cross-platform issue has a known workaround but not a real
  fix

The remaining work is concrete and time-bounded (~3–4 weeks for full
SPLIT INTO + HASH + voyager integration + CI). The bulk of the unknown-unknowns
were burnt down in this PoC.
