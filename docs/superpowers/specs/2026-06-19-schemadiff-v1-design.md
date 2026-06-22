# schemadiff V1 — Design Spec

**Date:** 2026-06-19
**Status:** Approved (brainstorming) — pending implementation plan
**Branch:** `shivansh/schemadiff-engine` (cut from `shivansh/schemadiff-scaffolding`)

## 1. Purpose

`schemadiff` is the diff engine for the `schema drift-analysis` feature. Given two
`schemasnapshot.SchemaSnapshot` values (an "A" baseline and a "B" comparison), it
computes the set of schema changes between them and lets callers narrow that set to
a scope (specific tables and/or object types).

It is a **pure, database-agnostic** library: no I/O, no database access, no mutation
of its inputs. All database-specific capture already happened upstream in
`schemasnapshot`.

## 2. Scope of this PR (V1)

The diff can only compare what the snapshot captures. `schemasnapshot` V1 captures
**tables and columns only** (plus partition/inheritance links, and an empty `Attrs`
extension seam). Therefore this PR implements diffing for exactly that surface:

**In scope**
- Tables: matched by `Table.ID` (PostgreSQL OID). Detect added, dropped, renamed,
  schema-moved, kind-changed, and partition/inheritance link changes.
- Columns: matched by `Column.ID` (`{tableOID}:{attnum}`). Detect added, dropped,
  renamed, type-changed, nullability-changed, default-changed.
- Scope filtering over the produced diffs (tables + object-type selectors), including
  rename-aware table scoping.

**Out of scope (deferred until capture is extended)**
- Constraints, indexes, sequences, views, materialized views, functions, triggers,
  user-defined types — none are captured yet, so there is nothing to diff.
- Attr walking/promotion — `Attrs` is an empty seam in V1; no Attr comparison is
  performed and no `ATTR_CHANGED` DiffType is declared (added when Attr diffing lands).
- The `schema drift-analysis` command wiring (separate, later work).

## 3. Dependencies & package layout

One-way dependency: `schemadiff → schemasnapshot`. `schemadiff` never imports
`schemasnapshot/databases/*` or any database-specific code.

```
yb-voyager/src/schemadiff/
  diff.go        # Difference, DiffType constants, Diff(a, b), diffTables, diffColumns, sort
  filter.go      # ObjectType, Scope, FilterByScope (+ DiffType→ObjectType map)
  diff_test.go   # unit tests (hand-built snapshots)
  filter_test.go # unit tests (hand-built Difference slices)
  integration_test.go  # //go:build integration — real testcontainer captures via schemasnapshot
```

There is **no** `databases/` subpackage — the engine is pure and engine-agnostic.

## 4. Public API — two pure functions

Decided deliberately (see §10): expose two composable pure functions rather than a
`Differ` object with functional options. Scope is a post-diff filter; keeping it a
separate step keeps the data flow honest, preserves rename correctness, and avoids
functional-options machinery for a single knob. A `Differ` can be added later, non-
breaking, with these functions as its implementation, if configuration grows.

```go
// Diff returns every change between snapshots a and b, sorted deterministically.
// Pure: no I/O, does not mutate a or b.
func Diff(a, b *schemasnapshot.SchemaSnapshot) []Difference

// FilterByScope returns the subset of diffs that fall within scope.
// Pure: does not mutate diffs or scope. Rename-aware (see §7).
func FilterByScope(diffs []Difference, scope Scope) []Difference
```

Typical caller composition:

```go
all    := schemadiff.Diff(a, b)
scoped := schemadiff.FilterByScope(all, scope)
```

## 5. Data model

```go
type DiffType string

type Difference struct {
    Type        DiffType    // the change type (see §6)
    Object      ObjectRef   // anchor object: side-A for most changes, side-B for *_ADDED
    AnchorTable *ObjectRef  // the table a finding filters under; nil for non-table-anchored findings
    SubObject   string      // dependent's name (e.g. column name); "" for table-level findings
    Property    string      // changed attribute name; "" for object-level findings
    OldValue    any         // structured old value (not a rendered string); nil where N/A
    NewValue    any         // structured new value; nil where N/A
    Details     string      // optional human-readable summary
}
```

`ObjectRef` is `schemasnapshot.ObjectRef` (`{Schema, Name}` with `String() => "schema.name"`).
For a column finding, `Object` is the parent table's ref, `SubObject` is the column name,
and `AnchorTable` points at the parent table.

## 6. DiffType enumeration (V1)

Only the **15 V1-emitted** `DiffType` constants are declared in `diff.go` (the
broader vocabulary from `LIBRARY_DESIGN_V1.md` §7 is intentionally NOT declared yet
— it is added incrementally as each object type becomes captured, to keep this PR
minimal). The `filter.go` `DiffType → ObjectType` map covers exactly these 15 (all
bucket to `ObjectTypeTable`), guarded by an exhaustiveness test.

**Tables (emitted)**
- `TABLE_ADDED` — `TableAdded`
- `TABLE_DROPPED` — `TableDropped`
- `TABLE_NAME_CHANGED` — `TableNameChanged`
- `TABLE_SCHEMA_CHANGED` — `TableSchemaChanged`
- `TABLE_KIND_CHANGED` — `TableKindChanged`
- `PARTITION_PARENT_CHANGED` — `PartitionParentChanged` — child's `PartitionParent`
- `PARTITION_CHILDREN_CHANGED` — `PartitionChildrenChanged` — parent's `PartitionChildren`
- `TABLE_INHERITS_CHANGED` — `TableInheritsChanged` — child's `InheritsFrom`
- `TABLE_INHERITED_BY_CHANGED` — `TableInheritedByChanged` — parent's `InheritedBy`

**Columns (emitted)**
- `COLUMN_ADDED` — `ColumnAdded`
- `COLUMN_DROPPED` — `ColumnDropped`
- `COLUMN_NAME_CHANGED` — `ColumnNameChanged`
- `COLUMN_TYPE_CHANGED` — `ColumnTypeChanged`
- `COLUMN_NULLABILITY_CHANGED` — `ColumnNullabilityChanged` (`NotNull`)
- `COLUMN_DEFAULT_CHANGED` — `ColumnDefaultChanged`

**Not declared in V1 (added incrementally later)**
- `ATTR_CHANGED` and the entire constraint/index/sequence/view/function/trigger/type
  vocabulary — declared when their object types become captured and diffed.

### Partition/inheritance link diffing — report both sides (deliberate)

Links are stored bidirectionally: a parent's `PartitionChildren`/`InheritedBy` and a
child's `PartitionParent`/`InheritsFrom`. We diff **all four fields independently** and
emit a finding for each side that changed. This is **not** double-reporting: each
finding is anchored to a *different table* (the child for `PARTITION_PARENT_CHANGED` /
`TABLE_INHERITS_CHANGED`, the parent for `PARTITION_CHILDREN_CHANGED` /
`TABLE_INHERITED_BY_CHANGED`), and scope filtering is per-table — so a user scoping to
just the child, or just the parent, must independently see the structural change that
concerns that table. Each side is a distinct per-table fact.

## 7. Diff algorithm

`Diff(a, b)`:
1. **Tables pass** — build maps keyed by `Table.ID` for A and B.
   - ID only in A → `TABLE_DROPPED`.
   - ID only in B → `TABLE_ADDED`.
   - ID in both → compare properties: `Name` → `TABLE_NAME_CHANGED`, `Schema` →
     `TABLE_SCHEMA_CHANGED`, `Kind` → `TABLE_KIND_CHANGED`, partition/inheritance
     links per §6.
2. **Columns pass** — build maps keyed by `Column.ID`.
   - ID only in A → `COLUMN_DROPPED`; only in B → `COLUMN_ADDED`.
   - ID in both → compare `Name`, `DataType`, `NotNull`, `Default`.
3. **Suppress lifecycle-table columns** — a post-pass drops `COLUMN_ADDED` /
   `COLUMN_DROPPED` findings whose parent table is itself wholly added / dropped
   (its `TABLE_ADDED` / `TABLE_DROPPED` finding already conveys the change; the
   per-column findings would be redundant noise). Columns on *matched* tables —
   including renamed tables (which emit `TABLE_NAME_CHANGED`, not added/dropped) —
   are preserved, so real column changes on surviving tables are never lost.
4. **Sort** the result deterministically by
   `(Object.Schema, Object.Name, SubObject, Type, Property)`.

**Identity / matching**
- Matching is by stable ID (OID-based) when `SchemaSnapshot.StableIdentity` is true
  (always true for PostgreSQL). ID matching is what surfaces a rename as a single
  `*_NAME_CHANGED` event rather than an add+drop pair.
- Fallback: if an ID is empty on either side, fall back to name matching for that
  object (no rename recognition). Defensive; not expected for PostgreSQL.

**Purity**
- `Diff` and `FilterByScope` never mutate their inputs and perform no I/O. Enforced
  by explicit purity tests.

## 8. Filter / scope

Ported nearly verbatim from the parked `filter.go` (the one fully-implemented piece).

```go
type ObjectType string
const (
    ObjectTypeTable    ObjectType = "TABLE"
    ObjectTypeIndex    ObjectType = "INDEX"
    ObjectTypeSequence ObjectType = "SEQUENCE"
    ObjectTypeView     ObjectType = "VIEW"
    ObjectTypeFunction ObjectType = "FUNCTION"
    ObjectTypeType     ObjectType = "TYPE"
)

type Scope struct {
    Tables             []string     // empty = all; matched against Difference.AnchorTable
    ExcludeTables      []string
    ObjectTypes        []ObjectType // empty = all
    ExcludeObjectTypes []ObjectType
}
```

- `DiffType → ObjectType` map covers exactly the 15 declared DiffTypes (all →
  `ObjectTypeTable`). An exhaustiveness test guards that every declared `DiffType`
  has a mapping.
- The full six-value `ObjectType` enum **is** kept — it is the user-facing
  `--object-type-list` selector vocabulary the command needs, not output vocabulary.
  In V1 only `TABLE` matches an emitted diff; filtering by `INDEX`/`VIEW`/etc.
  correctly yields nothing (and is tested as such).
- **Rename-aware table scoping:** a `TABLE_NAME_CHANGED` (and any finding anchored to
  a renamed table) is kept when **either** the old or the new name is in `Tables`, via
  a bidirectional rename-alias map built from `TABLE_NAME_CHANGED` findings. This is
  the main reason scope stays a post-diff step (§10).
- Order: include ObjectTypes → include Tables → exclude ObjectTypes → exclude Tables.

Scoping is **post-diff** (filter the results), not pre-diff. Pre-diff scoping is a
possible future performance optimization but complicates rename-across-boundary
handling and is unnecessary for V1's tables+columns volume.

## 9. Testing strategy (TDD)

Every unit of work is built test-first (write failing test → implement → green),
each phase executed by a Sonnet subagent, with the parent verifying build/vet/tests
independently after each phase.

- **diff_test.go** — hand-built `SchemaSnapshot` structs (pure function; no DB):
  empty vs empty, identical (no findings), table add/drop/rename/schema-move/kind,
  partition link change, inheritance link change, column add/drop/rename/type/
  nullability/default, deterministic sort ordering, input-immutability (purity).
- **filter_test.go** — hand-built `[]Difference`: empty scope is a no-op, object-type
  include/exclude, table include/exclude, nil-anchor handling, rename either-side
  retention, multi-level rename alias chains, include-then-exclude order, map
  exhaustiveness, purity.
- **integration_test.go** (`//go:build integration`) — high-value end-to-end: spin a
  `postgres:17` testcontainer, capture snapshot A via `schemasnapshot`, apply a DDL
  change, capture snapshot B, run `Diff`, assert the findings. Proves the engine
  against real captures, not just hand-built structs.

## 10. Key decision — two pure functions over a Differ (rationale)

Rejected `NewDiffer(WithScope(...)).Diff()` for V1 because:
1. Functional options for a single knob (`WithScope`) is unjustified machinery —
   the same over-engineering avoided elsewhere in this codebase.
2. Scope is inherently a post-diff filter; two functions make that stage visible and
   let callers inspect the full diff (e.g. "N total, M in scope").
3. Rename correctness lives cleanly in `FilterByScope` (either-side retention).
4. Easiest to TDD — two pure functions, no constructor/option wiring.

A `Differ` (config-struct or functional-options) can be layered on later without
breaking these functions if configuration genuinely grows (rename-detection toggle,
attr-comparison mode, ignore-lists).

## 11. Branch / worktree / sync workflow

- Engine work lives on `shivansh/schemadiff-engine`, cut from
  `shivansh/schemadiff-scaffolding` (PR #3614), in worktree
  `.claude/worktrees/schemadiff-engine`.
- The engine couples only to `schemasnapshot`'s exported data model. The only review
  change to #3614 that can ripple here is a **shape change** to `SchemaSnapshot` /
  `Table` / `Column` / `ObjectRef`; compiler + tests catch it immediately.
- Keep current by **merging** `schemadiff-scaffolding` into `schemadiff-engine`
  (not rebasing — preserves history).
- Open the engine PR with **base = `shivansh/schemadiff-scaffolding`** so the diff
  shows only engine commits. When #3614 merges to `main`, **retarget the engine PR's
  base to `main`**.
