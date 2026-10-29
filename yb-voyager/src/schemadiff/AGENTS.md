# schemadiff Package — Engineering Standards

> **Scope:** files under `yb-voyager/src/schemadiff/`. These are standards for **writing** code here, not only for reviewing it.
> Repo-wide standards live in `AGENTS.md` at each parent directory up to the repo root — read those as well. Design: `yb-voyager/design-specs/schema-detect-drift.md` (§3.1 for Scope, §5.9 for keys and indexes). Input: `src/schemasnapshot/AGENTS.md`. Consumer: `src/schema/schemadrift/AGENTS.md`.

## Layering

- Pure: two `SnapshotContent` values (A older, B newer) in, sorted `[]Difference` out. No I/O, no input mutation, no severity, no migration context — meaning belongs to `schemadrift`.
- The API is `Diff`, `FilterByScope`, and `NewDiffer(Config{Scope})`. Keep `Config` as a struct (future post-diff knobs land beside `Scope`); no functional options.
- Keep the library small: declare only the `DiffType`s and `ObjectType`s the engine emits, and no fields or selectors nothing consumes yet.

## Findings

- `DiffType` is the primary value; `Operation`, `ObjectType`, `Attribute` are its facets, set explicitly at each call to `newDifference`. Every finding goes through `newDifference`. Do not add a registry or table deriving facets from the type.
- `Operation` is a closed set: ADDED, DROPPED, CHANGED. It decides which sides are set: ADDED sets only `ObjectB`/`SideBValue`, DROPPED only `ObjectA`/`SideAValue`. CHANGED always sets both identities, but a value side can be nil: `TABLE_PARTITION_DETACHED`/`_ATTACHED` and the `TABLE_INHERIT*` types set only one. Consumers must not assume both values are non-nil. `Attribute` is `AttrNone` exactly for ADDED/DROPPED.
- One `DiffType` per direction (`COLUMN_NOT_NULL_SET`/`_DROPPED`, `TABLE_PARTITION_ATTACHED`/`_DETACHED`, `TABLE_INHERIT_ADDED`/`_REMOVED`): the directions need different guidance downstream.
- `Difference` holds identities (`ObjectA`/`ObjectB` as `ObjectIdent`), not a stored anchor table or sub-object. Derive the anchor from the identity (`anchorTableOf`).
- Never merge a drop and an add of the same name into one finding; the drop may be what broke the migration.
- `TABLE_ADDED`/`TABLE_DROPPED` carry the whole `Table`, deep-copied by `cloneTable`.
- Link changes, all `OpChanged`:
  - Inheritance: at most one `*_ADDED` and one `*_REMOVED` finding per table for each of `InheritsFrom` and `InheritedBy` (`diffLinkSet`), each carrying a sorted `[]ObjectRef` of only the changed links.
  - Partition children: one `TABLE_PARTITION_CHILDREN_CHANGED` carrying the full before and after child lists, in capture (`pg_inherits` OID) order, unsorted.
  - Partition parent: separate `TABLE_PARTITION_DETACHED` and `TABLE_PARTITION_ATTACHED` findings.
- Type strings reach the report JSON and callhome `drifts_by_type`; never rename one.

## Matching

- Tables and columns: `matchByKey` with keys from `chooseMatchKeys`. IDs when both sides are the same engine and it is PG or YB; names otherwise. The choice is a property of the engine, decided once — do not scan objects for missing IDs, fall back to names per object, or add a helper like `providesStableIDs`.
- Known gap (`TODO` on `chooseMatchKeys`, issue #3672): two instances of one engine have separate OID spaces. Do not widen ID matching before instance identity reaches `Diff`.
- Constraints and indexes: OID only, in `diffConstraintsOrIndexes`, with the primary key passed through `primaryKeyAsSlice`. Same OID and new name → `*_NAME_CHANGED`; new OID → `*_DROPPED` + `*_ADDED`, even under the same name; `FromParent` on either side → nothing. Name-pairing, `*_RECREATED`, definition-change and deferrable types were designed and cut (spec §5.9); do not add them back.
- Never compare `Definition`; PostgreSQL re-prints it with current names.
- Compare data an older snapshot lacks only when both sides' `Version` has it (`compareKeys` in `diffTables`). An old snapshot without keys must not read as one whose keys were dropped.
- Partition and inheritance links compare through `linkKeyFunc`, keyed like table matching, so a renamed parent is not a detach and attach on every child.

## Determinism

- `Diff` sorts with `sort.Slice` on a full key; do not claim stable order in comments. Slices built from maps inside findings are sorted (`diffLinkSet`); slices copied from a snapshot keep the snapshot's order. Map order must never reach output.
- Findings never share memory with the input. Extend `cloneTable` when `Table` gains a slice or pointer field.
- The Go type in `SideAValue`/`SideBValue` per type is a contract with `schemadrift`'s `stringifyValue`, which errors on unknown types. A new value type needs a case there.

## Scope

- `Scope` holds three exact keep-sets: `Schemas`, `Tables`, `ObjectTypes`. Empty means keep nothing, not all; an unfiltered caller passes the whole universe. Do not add exclude lists; the command turns `--exclude-*` into keep-sets because only it knows the universe.
- Filter after diffing. Never project snapshots to the requested schemas first: a `public` → `sales` move would become `TABLE_DROPPED`. The schema filter matches either side of a finding.
- A finding with no anchor table passes the table filter.
- `FilterByScope` never errors; validating names is the command's job.
- Rename-alias matching in `filter.go` is disabled by commenting out, pending a cross-window decision (spec §10). Keep the code and the KNOWN GAP note; do not delete it as dead code, and do not re-enable it for one window only.

## When you add a DiffType

1. Constant in `difftypes.go` and an entry in `AllDiffTypes()`.
2. Emit it through `newDifference` with the right facets.
3. Guidance for it in `schemadrift`'s `infoByDiffType`, and its severity row in `classification_test.go`. Without these, `getDriftInfo` errors at runtime.
4. A `stringifyValue` case if its value type is new.
5. Tests below. New enum values do not bump the callhome payload version.

## Code style here

- Pass method expressions directly (`schemasnapshot.Table.ForKey`); do not wrap a function in a closure that only calls it. Readable names (`byKeyA`, not `byA`). Shared helpers are named for what they do (`diffConstraintsOrIndexes`, not `diffByOID`).
- No unreachable guards. Use `toSet` and `lo` helpers rather than hand-rolled loops.

## Testing

- Unit tests (`unit` tag) build `SnapshotContent` literals and assert whole `Difference` values: every facet, both identities, both values. Never assert `Differ.Diff(...) == FilterByScope(Diff(...))`.
- `integration_test.go` (`integration` tag) applies real DDL on PostgreSQL. When you change or disable behaviour, update it too; a missed integration test has broken CI before.
- Cover ID and name matching, case-sensitive names, partition and inheritance copies, and a v1-vs-current pair for version-gated comparisons. Do not add low-value duplicates (per-kind copies of one rule, JSON round trips).
