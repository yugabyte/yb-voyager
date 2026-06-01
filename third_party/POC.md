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

### a) Two-fork architecture

The PoC kept patches + regen tooling inside voyager's `third_party/` directory
because that was the smallest configuration sufficient to prove the pipeline.
For production at the projected scale (multiple YB feature clusters across
several hundred lines of grammar delta), the right configuration is to hoist
the parser delta out of voyager entirely and into two long-lived forks under
the `yugabyte/` org:

```
yugabyte/libpg_query    ← long-lived yb-main branch tracking pganalyze/libpg_query
├── patches/
│   ├── 00_wiring/               YB extension-hook patches (one-time, rare)
│   └── 10_features/             per-feature patches (orthogonal, reorderable)
├── src/postgres_deparse.c       committed edits, YB-EXT-fenced
├── srcdata/*.json               committed edits
└── protobuf/pg_query.proto      committed edits

yugabyte/pg_query_go    ← long-lived yb-main branch tracking pganalyze/pg_query_go
├── parser/                      regenerated snapshot, committed
├── pg_query.pb.go               regenerated, committed
├── yb-ext/
│   ├── regen.sh                 clones libpg_query-yb, runs regen, snapshots into parser/
│   └── Dockerfile.regen
└── Makefile                     LIBDIR points at sibling libpg_query-yb checkout

yugabyte/yb-voyager
└── yb-voyager/go.mod            require github.com/yugabyte/pg_query_go/v6 v6.1.0-yb.N
                                 (no third_party/, no patches, no regen tooling)
```

Voyager imports stay `pg_query "github.com/yugabyte/pg_query_go/v6"` — a
one-time `sed` across `yb-voyager/src/` to update the import path.

**Why two forks rather than one combined fork.** Matches upstream's project
boundary. libpg_query is the C library (grammar, deparse, srcdata extraction);
pg_query_go is the Go binding (cgo wrapper, protobuf glue). Upstream releases
them on independent cadences and our forks should track that.

**Why patches inside libpg_query-yb (rather than committing modified PG sources
directly).** `gram.y`, `kwlist.h`, and `parsenodes.h` are upstream PostgreSQL
files. libpg_query downloads them fresh from a PG tarball during
`extract_source`; they don't live in libpg_query's git tree. The cleanest way
to express our delta is libpg_query's existing `patches/` mechanism — which
upstream uses for the same purpose. Forking PG itself to commit grammar edits
directly is far heavier than our delta justifies.

**Why committed edits (rather than patches) for libpg_query's own files.**
`postgres_deparse.c`, `srcdata/*.json`, and `pg_query.proto` live directly in
libpg_query's git tree. They get edited the natural way — `git blame` works,
IDEs work, PRs show real code diffs.

### b) Patch layering and design disciplines

The full YB syntax surface modifies several PG productions (`CreateStmt`,
`AlterTableStmt`, `IndexStmt`) that exist in multiple grammar variants.
Naïve per-feature patches would interfere with each other (patch N's context
depends on patch N-1 having been applied). The mitigations:

**Tier 1 — `patches/00_wiring/`.** One-time hook patches that edit each
existing PG production *once* to introduce a neutral YB extension hook.
Conceptually:

```bison
CreateStmt: ... yb_table_extensions { ... }    /* wiring patch edits each variant once */
yb_table_extensions:
      yb_table_extensions yb_table_extension
    | /* EMPTY */
;
yb_table_extension: /* feature patches plug in here */ ;
```

Wiring patches change rarely. They concentrate the upstream-bump risk in one
place — when upstream restructures a production we hook into, only the wiring
patch needs careful rework. Feature patches stay stable.

**Tier 2 — `patches/10_features/`.** Per-feature patches that touch only their
own neutral hooks plus new productions / structs / enum values. They're
orthogonal to each other and to upstream changes elsewhere in the same file.

**Design rule: additive over invasive.** Every YB edit should be expressible
as one of:

- A new field on an existing struct (never a type change to an existing field)
- A new enum value (never a renamed or removed value)
- A new struct alongside upstream structs
- A new alternative in an existing grammar rule (via the wiring hooks)

Where YB upstream's own postgres tree changes types in-place (because YB owns
the full execution path), our fork takes the additive route instead. This
keeps every YB edit purely additive to upstream's data structures and immune
to upstream's internal refactors.

**Comment fences.** Every YB edit is wrapped:

```c
/* YB-EXT BEGIN: <feature> */
... edit ...
/* YB-EXT END */
```

`git grep YB-EXT` enumerates the full delta on demand.

### c) Where each kind of edit lives

| File type | Repo / location | How edited |
|---|---|---|
| PG source (`gram.y`, `kwlist.h`, `parsenodes.h`, `scan.l`) | `libpg_query-yb/patches/*.patch` | Patches — these files only exist transiently in `tmp/postgres/` during regen |
| libpg_query's own C (`postgres_deparse.c`, `pg_query_*.c`) | `libpg_query-yb/src/` | Committed edits, YB-EXT fenced |
| libpg_query's own data (`srcdata/*.json`) | `libpg_query-yb/srcdata/` | Committed edits |
| libpg_query's own schema (`pg_query.proto`) | `libpg_query-yb/protobuf/` | Committed edits |
| Regenerated outputs (`gram.c`, `pg_query.pb.go`, etc.) | `pg_query_go-yb/parser/`, `pg_query_go-yb/pg_query.pb.go` | Never hand-edited; produced by `yb-ext/regen.sh` |

### d) Workflows

| Event | Repos touched | Steps |
|---|---|---|
| New YB feature | libpg_query-yb → pg_query_go-yb → voyager | (1) Author patches in libpg_query-yb tier-2 (+ any deparse/srcdata edits). (2) Run `yb-ext/regen.sh` in pg_query_go-yb; commit snapshot; tag `v6.1.0-yb.N+1`. (3) Bump `go.mod` in voyager; add voyager-side queryparser extraction. |
| Upstream libpg_query bump | libpg_query-yb → pg_query_go-yb → voyager | (1) `git fetch upstream && git merge` in libpg_query-yb; address any wiring-patch drift. (2) Bump `LIB_TAG`, regen, tag in pg_query_go-yb. (3) Bump `go.mod` in voyager. |
| Upstream pg_query_go bump | pg_query_go-yb → voyager | (1) `git fetch upstream && git merge` in pg_query_go-yb. Regen. Tag. (2) Bump `go.mod` in voyager. |
| Per voyager release | voyager | Nothing extra — the fork tag in `go.mod` is the only voyager-side dependency. |

### e) Outstanding work to migrate the PoC into this layout

Each feature beyond the SORTBY_HASH PoC will be evaluated on its own when
implementing — design choices per feature are deferred until then. The
migration work itself is:

| Item | Effort |
|---|---|
| Create `yugabyte/libpg_query` and `yugabyte/pg_query_go` fork repos | half a day |
| Move PoC toolchain fixes and `99_yb_sortby_hash` patch into `libpg_query-yb` (initial tier-2 layout; tier-1 wiring patches added as features need them) | 1 day |
| Move `regen.sh` + `Dockerfile.regen` into `pg_query_go-yb/yb-ext/` | half a day |
| First tagged release `pg_query_go-yb v6.1.0-yb.1` covering SORTBY_HASH | half a day |
| Wire macOS `pg_config.h` restore into `regen.sh` (replace today's manual `cp`) | half a day |
| Drop `third_party/` from voyager; switch `go.mod`; one-time import-path `sed` across `yb-voyager/src/` | half a day |
| CI in both forks: regen-on-PR + cross-platform cgo smoke test | 1–2 days |
| Voyager-side: extract `SortByDir_SORTBY_HASH` into `Table.PrimaryKeyColumns` metadata, expose to assess-migration report | 1 day |

### f) Testing strategy

Coverage is unchanged from the PoC; what shifts is where each layer is
exercised:

1. **Static**: `regen.sh` exits clean. Run by CI in `pg_query_go-yb` on every
   PR. Anyone who edits patches, srcdata, or deparse code surfaces conflicts
   immediately.
2. **Parser unit tests** in `pg_query_go-yb` itself — extend upstream's
   `parse_test.go` with YB-syntax cases.
3. **Voyager queryparser tests** in voyager (`yb_<feature>_test.go` per
   feature). `yb_hash_test.go` already exists as the template.
4. **Voyager integration tests** — hand-crafted YB-syntax DDL run through
   `yb-voyager analyze-schema` and `yb-voyager import-schema` against a local
   YB cluster. Assert no parse errors; assert applied behavior on YB (e.g.
   tablet count via `yb_table_properties()`).
5. **Cross-platform compile**: `go build` + `go test -tags unit` on both
   Ubuntu and macOS runners in both fork repos and in voyager.
6. **Regression**: every existing `queryissue` / `queryparser` test in voyager
   must still pass with each YB-EXT addition.

### g) Maintenance burden — honest assessment

- **Per YB feature**: ~1–3 days of grammar design + patch authoring + tests +
  voyager integration. Each feature gets its own re-evaluation against the
  additive-only discipline.
- **Per upstream libpg_query release** (~6 mo cadence): ~1–3 days. Risk is
  concentrated in tier-1 wiring patches; tier-2 feature patches stay stable.
- **Per PG major bump** (~12 mo cadence): potentially more invasive — wiring
  patches may need substantial rework if PG restructures `CreateStmt` et al.
  Budget a week.
- **Per voyager release**: nothing extra.

In steady state with ~5 features tracked, expect ~2 person-weeks/year for
maintenance, not counting new feature additions.

The alternative — staying on stock pg_query_go and only ever emitting /
accepting plain PG SQL — saves all this maintenance but blocks the use cases
described in section 1.

### h) Risks

- **Tier-1 wiring patches are the rebase pivot.** Concentrated risk by design —
  feature patches stay safe at the cost of the wiring patches taking the
  upstream-bump hit. Acceptable tradeoff.
- **Three-repo coordination for a feature.** A new YB feature touches 3 repos
  in sequence. Mitigation: documented release checklist; consider a meta
  Makefile target in voyager for local development.
- **libpg_query maintainer divergence.** If pganalyze restructures their
  parser-tree-walking conventions, our patches could become harder to rebase.
  Mitigation: keep YB-EXT changes minimal, additive, and fenced.
- **Docker dependency for regen.** Linux contributors don't need it; macOS
  contributors do. If CI does regen on push, this becomes invisible to
  feature contributors.
- **`pg_config.h` cross-platform issue.** Currently papered over with a
  manual `cp`. Needs a real fix before relying on cross-platform CI regen at
  scale.
- **YB grammar evolution.** If YB upstream changes how a feature works in a
  new YB release, we'd need to port the new behavior. This is a one-way diff —
  we read YB's grammar but don't depend on it at build time, so it's purely
  a documentation-tracking concern.

---

## 8. Licensing

Four licenses are in play. All four are OSI-approved, permissive, and
non-copyleft — there are no conflicts.

| Component | License | Notes |
|---|---|---|
| **yb-voyager** | Apache License 2.0 | Top-level `LICENSE`; all `cmd/` and `src/` files I inspected carry Apache 2.0 headers. The repo `README.md` and `licenses/` directory also include Polyform Free Trial 1.0.0 for dual-licensing, but no current source files use it. |
| **pg_query_go** (3-Clause BSD) | BSD-3-Clause | Copyright Lukas Fittl, Duboce Labs / pganalyze. Vendored at `third_party/pg_query_go/LICENSE`. |
| **libpg_query** (3-Clause BSD) | BSD-3-Clause | Same authors as pg_query_go. In the proposed final layout, libpg_query is fetched on-demand at regen time rather than vendored, so its `LICENSE` doesn't ship in voyager — but the BSD-3 attribution still applies to the snapshot inside `pg_query_go/parser/` that derives from it. |
| **PostgreSQL sources** (inside libpg_query — `gram.y`, `parsenodes.h`, etc.) | The PostgreSQL License (≈ MIT/BSD-2) | Each extracted PG source file carries `Portions Copyright (c) 1996-2024, PostgreSQL Global Development Group` and `Portions Copyright (c) 1994, Regents of the University of California`. These headers must be preserved. |

### Compatibility

```
                    ┌─────────────────────────┐
                    │   Apache License 2.0    │  ← voyager (most restrictive)
                    └────────────▲────────────┘
                                 │
                       can absorb (one-way)
                                 │
              ┌──────────────────┼─────────────────┐
              │                                    │
       3-Clause BSD                        PostgreSQL License
   (pg_query_go, libpg_query)         (PG sources inside libpg_query)
```

Apache 2.0 absorbs BSD-3 and the PostgreSQL License cleanly. Apache 2.0 adds
an explicit patent grant and modification-notice requirement that BSD-3
doesn't have, but this is an asymmetry, not a conflict — voyager (the
absorbing project) is the one taking on the additional Apache obligations.

### Obligations and how they're satisfied

1. **Preserve license files where we redistribute code.**
   - `third_party/pg_query_go/LICENSE` and `third_party/pg_query_go/LICENSE.POSTGRESQL` ✓ already vendored
   - When we move to fetch-on-demand for libpg_query, the BSD-3 / PG-License coverage flows through to `pg_query_go/parser/` (the committed snapshot still contains the PG-derived sources with their headers)

2. **Preserve copyright headers in source files.** ✓ The extracted PG sources retain their `Portions Copyright` lines. Don't strip them when re-snapshotting.

3. **Don't use "pg_query" or "pganalyze" names to endorse YB Voyager** (BSD-3 clause 3). ✓ We don't.

4. **For our own modifications** (Apache 2.0 §4(b)): patch files should identify what they modify and carry our copyright. ✓ `/* YB-EXT BEGIN: <feature> */` fenced blocks document the changes; patch file headers carry the Yugabyte copyright.

### Recommended addition: `third_party/pg-yb-parser/NOTICE`

None of the upstream licenses *require* a NOTICE file, but Apache 2.0 §4(d)
makes it good hygiene. Proposed contents:

```
third_party/pg-yb-parser/NOTICE
─────────────────────────────────────
This directory contains YugabyteDB-specific modifications to:

  libpg_query (BSD-3-Clause)
    Upstream: https://github.com/pganalyze/libpg_query
    Pinned to release: see regen.sh LIB_TAG (currently 17-6.1.0)
    Copyright (c) 2015, Lukas Fittl
    Copyright (c) 2016-2023, Duboce Labs, Inc. (pganalyze)

  pg_query_go (BSD-3-Clause)
    Upstream: https://github.com/pganalyze/pg_query_go
    Pinned to release: see third_party/pg_query_go/Makefile (currently v6.1.0)
    Copyright (c) 2015, Lukas Fittl

The above projects incorporate sources from PostgreSQL
(The PostgreSQL License):
  Copyright (c) 1996-2024, PostgreSQL Global Development Group
  Copyright (c) 1994, Regents of the University of California

Modifications in this directory (the YB-EXT patches, the regen script,
the Dockerfile, and any YB-EXT-fenced edits in pg_query_go/parser/
postgres_deparse.c) are Copyright (c) YugabyteDB, Inc. and licensed
under the Apache License, Version 2.0 (see ../../yb-voyager/LICENSE).
```

This makes redistribution audits trivial and is a one-time addition.

### Net assessment

Safe to use, distribute, and ship in voyager. No restrictive licenses, no
copyleft, no patent encumbrances. The only ongoing obligation is to keep
license files and copyright headers intact through the regen pipeline —
which `make update_source` already does automatically by virtue of copying
files verbatim from upstream.

---

## 9. Quick reference

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

## 10. Bottom line

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
