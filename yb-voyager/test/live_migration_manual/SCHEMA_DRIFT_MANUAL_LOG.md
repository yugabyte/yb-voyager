# Schema drift manual log

Database **`schema_drift`**, tables **`public.control_t`** / **`public.subject_t`**. Reset: run **`control_subject_baseline_schema.sql`** on **source and target** (empty tables); run **`control_subject_baseline_source_seeds.sql`** on **source only**. Or run **`control_subject_baseline.sql`** on source (schema + seeds in one go). Target must not run the seeds file. **Scenario C** adds **`public.side_t`** (with **PK**); **scenario D** adds **`public.nopk_side_t`** (**no PK**). Between runs drop on **both** sides: `DROP TABLE IF EXISTS public.side_t CASCADE;` and `DROP TABLE IF EXISTS public.nopk_side_t CASCADE;`. **Scenario E** renames **`public.subject_t`** → **`public.subject_renamed_t`** — afterward restore baseline (re-run schema + seeds) or reverse the renames on both sides so later scenarios stay comparable. **Scenario F** renames column **`note` → `note_renamed`** on **`public.subject_t`** on the **source** first — afterward restore baseline or **`RENAME COLUMN`** back on **both** sides so **`note`** exists again for other scenarios. **Scenario G** changes **`public.subject_t.score`** from **`INT`** to **`NUMERIC`** on the **source** first (see **prerequisite** in that section — **`score`** must exist on **both** sides). Afterward restore baseline or **`ALTER TABLE public.subject_t DROP COLUMN IF EXISTS score`** on **both** sides. **Scenario H** changes **`score`** from **`NUMERIC(10,2)`** to **`TEXT`** on the **source** first while the target stays **`NUMERIC`** (“**compatible**” in the sense that **string payloads often still cast** into **`NUMERIC`** on apply — record whether import stayed green). Same **`DROP COLUMN score`** / baseline cleanup afterward. **Scenario I** runs **`DROP TABLE public.subject_t`** on the **source** only (table is in the live migration set). **Destructive** for the lab DB — use a **full baseline** when you want **`subject_t`** back for **other** scenarios. **Do not** rely on **re-creating the old `subject_t` on the source** to “fix” export; use **mid-migration surgery** under **`<export-dir>/metainfo/`** so **stored lists + Debezium** match the **new** source catalog (**`control_t`** only, or whatever you keep). **Scenario J** drops the **nullable** column **`note`** (**`TEXT`**, no **`NOT NULL`**) on **`public.subject_t`** on the **source** first while the target keeps **`note`** — use a **fresh baseline** if **`subject_t`** is missing after **I**; afterward restore **`note`** with **`ALTER TABLE public.subject_t ADD COLUMN note TEXT`** on **both** sides or re-run **`control_subject_baseline_schema.sql`**. **Scenario K** drops a **`NOT NULL`** column on the **source** first (see its **prerequisite** — add **`tag`** on **both** sides); afterward **`DROP COLUMN IF EXISTS tag`** on **both** or re-baseline. **Scenario L** is the same **`DROP COLUMN`** skew as **K**, but the lagging target column is **`NOT NULL`** **without** any **`DEFAULT`** (see **prerequisite** — add **`tag2`** on **both** sides, backfill, **`SET NOT NULL`**, then **`DROP DEFAULT`**); afterward **`DROP COLUMN IF EXISTS tag2`** on **both** or re-baseline. **Scenario M** drops the **primary key** on **`public.subject_t`** on the **source** only (**`ALTER TABLE … DROP CONSTRAINT …`** on the **`PRIMARY KEY`**); afterward **`ADD PRIMARY KEY (id)`** on the **source** (and re-align the **target** if you changed it) or **full baseline** — live **`export data`** calls **`reportUnsupportedTablesForLiveMigration`** (`exportData.go`) and **`ErrExit`** if a captured table has **no PK** on the **source** when the table list is finalized. **Scenario N** adds a **new label** to a **PostgreSQL `ENUM`** on **`public.subject_t.phase`** on the **source** first while **YugabyteDB** keeps the **older** enum type definition — **`INSERT`**s using the **new** label fail on the target until **`ALTER TYPE … ADD VALUE`** (or equivalent) on **Yugabyte**; afterward **`DROP COLUMN IF EXISTS phase`** on **both** sides, then **`DROP TYPE IF EXISTS public.subject_phase_t CASCADE`** on **both** (after the column is gone), or re-baseline. **Scenario O** changes **`public.subject_t`**’s **primary key on the source only** (e.g. **`PRIMARY KEY (name)`** while **YugabyteDB** keeps **`PRIMARY KEY (id)`**) so live **`INSERT`** SQL built from **CDC `event.Key`** (**`ON CONFLICT (name)`**) no longer matches any **unique / PK** on the **target** — record the exact **SQLSTATE** / message; afterward restore **`PRIMARY KEY (id)`** on **PostgreSQL** (and drop the **`name`** PK) or re-baseline. Run **O** from a catalog where **`subject_t`** is still **`PK (id)`** on the **source** and **`name`** values are **unique** (baseline seeds satisfy this). **Scenario P** is **partition add** on **`public.part_t`** (mid-run capture + publication caveats). **Scenario Q** is **default change** on **`public.subject_t.flag`**. **Scenario R** is **target-only `NOT NULL`** on **`public.subject_t.strict_col`** (vs nullable source) — overlaps **L**’s “implicit null vs **`NOT NULL`**” theme for the **forward** path. **Scenario S** is **post-cutover fallback** (stream **Yugabyte → PostgreSQL**): **target** relaxes **`NOT NULL`** while **PostgreSQL** stays strict — **`NULL`** / omitted values from the **target** can fail on **import to source** (**`23502`**). **Scenario T** is **`DETACH PARTITION`** on the **source** only (reuses `public.part_t` from **P**): a captured leaf partition is detached from its root on the source and becomes a **standalone** table. Debezium's `table.include.list` still contains the leaf name and voyager's `SourceRenameTablesMap` still maps the leaf back to the root — after detach this rename is semantically wrong (events from a now-standalone table get re-routed to a root the source no longer treats as its parent). Afterward **re-attach** on the **source** (`ALTER TABLE public.part_t ATTACH PARTITION public.part_t_p2 FOR VALUES FROM ('2026-04-01') TO ('2026-07-01');`) or re-baseline `part_t` on both sides.

For each scenario: run the **DDL** on the side shown, wait and watch **export** / **import**. Then use the steps in order: **DML on source after resume**, then an **alignment** step (**DDL on the side that is behind** — not always sufficient; see **Findings** / **Final notes**), then **exit + resume**, then **more DML on source** to probe behavior. **Target-only DML** is not part of the CDC path for replicated tables; use **DML on source** to generate change events. All **source** SQL below is for **PostgreSQL** unless noted.

### How to read each scenario

| Section | Purpose |
|--------|---------|
| Numbered **steps** | What to run, in order. |
| **Findings** | What happened, split so you can skim or dig in: **At a glance** (table), **Observed** (facts from the run), **Why (from code)** (why the tool behaved that way), **Notes** (**Failure** vs **Workaround (observed)** when you have them). Fill **Why** / **Notes** only **after** **Observed**. |
| **Final notes** | One-line **Failure** · **Workaround** summaries for scenarios whose **Findings → Observed** is filled (see **### Final notes** header). |

---

## Scenario A — New column on **source** only

**1. DDL (source):**

```sql
ALTER TABLE public.subject_t ADD COLUMN from_source TEXT;
```

**Do not** run this `ALTER` on the target until you finish steps **1–4** (before **target alignment**), or reset baseline after.

**2. Optional DML (source)** — right after DDL, if you want traffic before any resume:

```sql
INSERT INTO public.control_t (name) VALUES ('after_src_ddl_ctl');
INSERT INTO public.subject_t (name, from_source) VALUES ('after_src_ddl_sub', 'v1');
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume** — use new literal names so you can grep logs / rows:

```sql
INSERT INTO public.control_t (name) VALUES ('a_post_resume_ctl');
INSERT INTO public.subject_t (name, from_source) VALUES ('a_post_resume_sub', 'v_after_resume');
```

**5. Target alignment (DDL on Yugabyte)** — add the missing column so the target catalog can match CDC (may clear import errors once events reference this column):

```sql
ALTER TABLE public.subject_t ADD COLUMN from_source TEXT;
```

**6. Exit + resume** export and import again.

**7. DML (source) after alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('a_post_workaround_ctl');
INSERT INTO public.subject_t (name, from_source) VALUES ('a_post_workaround_sub', 'v_after_workaround');
```

### Findings — A

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After first `subject_t` insert using `from_source` (target **without** column) | OK | **Panic** — cannot map `from_source` to target columns |
| After exit + resume import (still no target column) | OK | **Same panic** — backlog / new events still carry `from_source` |
| After target `ADD COLUMN from_source` + resume import | OK | OK — events line up with target catalog |

#### Observed

- First **`INSERT INTO subject_t` … `(name, from_source)`** after source-only DDL caused **import** to fail immediately with a panic (message like: `from_source` not found amongst target columns `[id note name]`). **Export** kept running.
- **Exit + resume import** did not clear the failure; **post-resume** `subject_t` inserts still could not be applied until the target schema was aligned.
- **Alignment that worked:** `ALTER TABLE public.subject_t ADD COLUMN from_source TEXT` on **Yugabyte**, then **resume import** — import progressed; subsequent source inserts with `from_source` succeeded.

#### Why (from code)

1. **Change events carry source column names.** After `from_source` exists only on the source, Debezium emits inserts/updates whose payload includes a field for `from_source`. The importer builds SQL from that payload (`event.Fields` keys = column names coming from CDC).

2. **Inserts are validated against the target catalog, not the source.** For prepared inserts, `getPreparedInsertStmt` walks every key in `event.Fields` and calls `TargetDB.QuoteAttributeName` for each. That goes through `AttributeNameRegistry.QuoteAttributeName` in `yb-voyager/src/tgtdb/attr_name_registry.go`, which loads the target table’s attribute list (`GetListOfTableAttributes`) and runs `findBestMatchingColumnName` (exact match, then case-insensitive). If the name is absent, you get an error like `column "from_source" not found amongst table columns [...]`.

3. **The failure surfaces as a panic on this path.** In `yb-voyager/src/tgtdb/event.go`, `getPreparedInsertStmt` uses `panic(fmt.Errorf(...))` when `QuoteAttributeName` returns an error for a column in the insert field list. So a schema mismatch is a hard stop for the import process rather than a returned error.

4. **Resume does not rewrite events.** Restarting import reprocesses the same stream / segments; events still list `from_source`. Until the target has that column (or the pipeline gains some other handling), `QuoteAttributeName` will keep failing for those events.

#### Notes

- **Failure:** Events referenced **`from_source`** while Yugabyte **`subject_t`** did not → import **panic** on column quoting. **Workaround (observed):** **`ALTER TABLE public.subject_t ADD COLUMN from_source TEXT`** on the **target**, then **resume import**.
- Panic stack (for grep / cross-ref): `(*Event).getPreparedInsertStmt` → `QuoteAttributeName` / `find best matching column name for "from_source"`.
- **Mitigation for this scenario:** keep target column set **superset-equal** to what CDC emits for that table (here: add `from_source` on target before or as soon as source traffic uses it), or avoid emitting the new column until both sides match.

---

## Scenario B — New column on **target** only

**1. DDL (YugabyteDB / target):**

```sql
ALTER TABLE public.subject_t ADD COLUMN from_target TEXT;
```

**Do not** run this `ALTER` on the source until you finish steps **1–4** (before **source alignment**).

**2. Optional DML (source)** — baseline columns only (source has no `from_target` yet):

```sql
INSERT INTO public.control_t (name) VALUES ('tgt_ddl_only_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('tgt_ddl_only_sub', NULL);
```

**3. Exit + resume** export and import.

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('b_post_resume_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('b_post_resume_sub', NULL);
```

**5. Source alignment (DDL on PostgreSQL)** — add the same column on the source so CDC payloads can include `from_target`:

```sql
ALTER TABLE public.subject_t ADD COLUMN from_target TEXT;
```

**6. Exit + resume** export and import again.

**7. DML (source) after alignment + resume** — can reference `from_target` on source (replicates to target once both sides have the column):

```sql
INSERT INTO public.control_t (name) VALUES ('b_post_workaround_ctl');
INSERT INTO public.subject_t (name, note, from_target) VALUES ('b_post_workaround_sub', NULL, 'v_after_workaround');
```

### Findings — B

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After **target-only** `ADD COLUMN from_target` + **step 2** optional DML (both inserts) | OK | **OK** — no failures |
| **Exit + resume** `export data` and `import data` (with source still **without** `from_target`) | OK | OK |
| After **step 5** source alignment (`ADD COLUMN from_target`) + **resume import** | OK | **Failed once** — see error below |
| **Quit + resume import** again (no DDL change) | OK | **OK**; **step 7**-style new events also OK |

#### Observed

- **Target-only column:** With `from_target` only on Yugabyte, the **step 2** pair of inserts imported cleanly; **exit + resume** for **both** export and import was fine. No **alignment** step was required for that phase.
- **After source alignment (step 5):** First **resume import** failed while streaming a segment, with an error like:
  - `Failed to stream changes to yugabytedb` → `error transforming event key fields` → `convert event fields` → `fetch column schema` → **`Column from_target not found in table schema`** with column list **`id`, `name`, `note` only** (no `from_target` in that schema object).
- **Recovery:** Stopping and **resuming import again** succeeded; additional inserts (**step 7**) replicated without that error.

#### Why (from code)

1. **Early Scenario B (no `from_target` on source)**  
   Same as before: CDC **payload keys** match the **source** catalog. Events do not contain `from_target`, so neither **`QuoteAttributeName`** (live target catalog) nor **value conversion** needs a type entry for `from_target`. Target-only columns are invisible to the event.

2. **After source gains `from_target` (step 5+)**  
   Events can include **`from_target`** in **`event.Fields`**. Before SQL is built, **`StreamingPhaseDebeziumValueConverter.ConvertEvent`** (`yb-voyager/src/dbzm/valueConverter.go`) runs **`convertMap`** on keys and fields; for each non-null field it calls **`SchemaRegistry.GetColumnType`** (`yb-voyager/src/utils/schemareg/schemaRegistry.go`). The registry’s **`TableSchema`** comes from **JSON files on disk** under **`{exportDir}/data/schemas/{exporterRole}/`** (`SchemaRegistry.Init`), not from a live `information_schema` poll on every event.

3. **Why the failure showed only `{id, name, note}`**  
   The embedded table schema in the error is whatever was **decoded from that snapshot file** for `subject_t` at the time import ran. If export has **not yet written** (or import has **not yet re-read**) a schema file that includes **`from_target`**, **`getColumnType`** cannot find the column and returns **`Column %s not found in table schema %v`**. That is **orthogonal** to Scenario A’s failure: there the **Yugabyte table** was missing the column; here the **live target DB can already have `from_target`**, but the **importer’s Debezium schema snapshot used for typing/formatting** can still be stale.

4. **Why the second resume worked**  
   The schema registry keeps **`TableNameToSchema`** in memory after the first successful load; **`GetColumnType`** only calls **`Init()`** again if the table was **missing** from the map, not if the on-disk JSON changed while the map still holds an older **`TableSchema`**. A **new import process** (quit + resume starts fresh) reloads **`data/schemas/.../*.json`** from disk. Once export has written **`from_target`** into **`subject_t`**’s schema file, that reload succeeds for **`from_target`** lookups.

5. **Log message quirk**  
   `handleEvent` in `yb-voyager/cmd/live_migration.go` wraps **any** `ConvertEvent` failure as **`error transforming event key fields`**, even when the wrapped chain is **`convert event fields`** (fields path, not key-only).

#### Notes

- **Failure:** After the **source** gained **`from_target`**, one import resume failed because **on-disk** Debezium schema JSON still omitted that column. **Workaround (observed):** **quit + resume import** again once export has refreshed the schema files.
- **Contrast A vs B:** A breaks on **SQL quoting** against the **target DB catalog** when the event names a column the **table** lacks. B (post–source DDL) can break earlier on **typing** when the event names a column the **on-disk Debezium schema snapshot** still lacks—even if the physical target table already has the column.
- If this race shows up again: ensure **export** has produced updated schema artifacts before relying on the first import resume after a source **`ADD COLUMN`**, or retry **import resume** once schemas on disk match CDC.

---

## Scenario C — Adding new **PK** table (source DDL only)

Use **`public.side_t`** with **`id BIGSERIAL PRIMARY KEY`** (not in the baseline scripts). Ensure your **source** publication / connector setup actually captures **new** tables if your environment requires it (some setups only list initial tables).

**1. DDL (source) only:**

```sql
CREATE TABLE public.side_t (
	id    BIGSERIAL PRIMARY KEY,
	label TEXT NOT NULL
);
ALTER TABLE public.side_t REPLICA IDENTITY FULL;
```

**Do not** create `side_t` on the target until you finish steps **1–4** (before **target alignment**), or reset baseline and drop `side_t` on both sides.

**2. Optional DML (source)** — traffic on the new table (and a control ping on `control_t`):

```sql
INSERT INTO public.control_t (name) VALUES ('c_after_src_side_ddl_ctl');
INSERT INTO public.side_t (label) VALUES ('c_src_only_side');
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('c_post_resume_ctl');
INSERT INTO public.side_t (label) VALUES ('c_post_resume_side');
```

**5. Target alignment (DDL on Yugabyte)** — create a table matching the source definition so import *could* apply `side_t` rows **if** export ever emits them (this step alone does **not** register the table with export or the publication):

```sql
CREATE TABLE public.side_t (
	id    BIGSERIAL PRIMARY KEY,
	label TEXT NOT NULL
);
ALTER TABLE public.side_t REPLICA IDENTITY FULL;
```

**6. Exit + resume** export and import again.

**7. DML (source) after target alignment + resume** — generate more change events on source (still the way to exercise CDC):

```sql
INSERT INTO public.control_t (name) VALUES ('c_post_workaround_ctl');
INSERT INTO public.side_t (label) VALUES ('c_post_workaround_side');
```

### Findings — C (PK table)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After **`side_t` DDL + DML** on source (target initially without `side_t`) | **`control_t` events only** — **no `side_t`** rows in the stream | **`control_t`** rows applied; **`side_t`** had nothing to apply |
| **Exit + resume** export/import | Same — **`side_t`** still not captured | Same |
| **Step 5** (target `CREATE TABLE side_t`) + resume import | **`side_t` still not exported** | Still only **`control_t`** traffic |

#### Observed

- **`public.side_t`** existed on the **source** with traffic, but **only `public.control_t`** events showed up in **export → import**; **`side_t`** inserts never appeared in the replicated path.
- **Exit + resume** for **both** `export data` and `import data` succeeded but did **not** start moving **`side_t`** data.
- **Target alignment** (**`CREATE TABLE side_t`** on Yugabyte) and resuming **import** did **not** cause **`side_t`** to be exported (export never produced those events).
- The **effective table set stayed** **`subject_t` + `control_t`** in command behavior / validation messages.
- Adding **`--table-list public.subject_t,public.control_t,public.side_t`** on a **subsequent** `export data` **failed** with *Unknown table names in the include list: [public.side_t]* and *Valid table names are: [public.subject_t public.control_t]* (schema-qualified form in the error may match how names are registered).

#### Why (from code)

1. **Live import table set** comes from migration metadata (**`TableListExportedFromSource`** in the MSR, wired through **`getInitialImportTableListForLive`** in `importData.go`); it is **not** re-derived from a live source catalog on every resume. If **`side_t`** was **never** part of that stored export/migration table list, import will not expect or apply **`side_t`** segments—regardless of target DDL.

2. **`checkTablesPresentInTarget`** (`importData.go`) only runs for tables **already** in that import set vs target presence; it explains **Scenario C** when a listed table is missing on Yugabyte, but **not** “table missing from the migration entirely.” Here the dominant issue was **`side_t` absent from capture**, not target-only DDL mismatch alone.

3. **Subsequent `export data` table-list flags** are validated against the **name-registry / first-run list**, not “all tables currently on the source.” **`getInitialTableList`** (`exportData.go`) on a **continuing** run uses the **stored** first-run list and **`getRegisteredNameRegList()`**; **`applyTableListFlagsOnCurrentAndRemoveRootsFromBothLists`** calls **`applyTableListFlagsOnFullListAndAddLeafPartitions(registeredList, …)`**, and the comment there states that **tables not in that registered list error as unknown**—matching the *Unknown table names in the include list* failure for **`public.side_t`**.

4. **CDC / publication:** even with matching DDL on both sides, if the **logical replication publication** (or Debezium connector table include list) was **never** updated to **`side_t`**, PostgreSQL will **not** emit change events for that table—export cannot stream what the source never publishes.

#### Notes

- **Failure (this run):** **`side_t`** never entered the export stream; **only `control_t`** replicated. **Workaround (observed):** **none** from this playbook — **target `CREATE TABLE` + import resume** did not help; **`--table-list … side_t`** on resume was **rejected**. **What would work (conceptually):** a **new migration / export** that registers and publishes **`side_t` from the start** (or a supported **add-table** flow), not DDL-only alignment on an existing run.
- **Workaround attempt (mid-migration surgery; got streaming but required backfill):**
  - **Edited Debezium config** at `<export-dir>/metainfo/conf/application.properties`:
    - `debezium.source.table.include.list=public.control_t,public.subject_t,public.side_t`
    - `debezium.source.column.include.list=public.control_t.*,public.subject_t.*,public.side_t.*`
    - Added `side_t` entries to `debezium.sink.ybexporter.column_sequence.map` and `debezium.sink.ybexporter.sequence.max.map` (including `"public"."side_t_id_seq"`).
  - **Edited name registry** at `<export-dir>/metainfo/name_registry.json` to add:
    - `side_t` under both `SourceDBTableNames.public` and `YBTableNames.public`
    - `side_t_id_seq` under both `SourceDBSequenceNames.public` and `YBSequenceNames.public`
  - **Patched metadb** (`<export-dir>/metainfo/meta.db`, SQLite `json_objects`) to add `public.side_t` into `migration_status`:
    - appended to `TableListExportedFromSource` and `SourceExportedTableListWithLeafPartitions`
    - added `"public.side_t.id" -> "\"public\".\"side_t_id_seq\""` in `SourceColumnToSequenceMapping`
    - patched `import_data_status` to add `"\"public\".\"side_t\"" -> "pk"` in `tableToCDCPartitioningStrategyMap`
  - **Outcome:** after resuming, **export ran and `side_t` started streaming**, but **previous `side_t` events/rows were not present** and needed **backfill** into the new table.

---

## Scenario D — Adding new **non-PK** table (source DDL only)

Use **`public.nopk_side_t`**: **no `PRIMARY KEY`**, **`REPLICA IDENTITY FULL`** so PostgreSQL logical decoding can still ship full rows for updates/deletes if you add those later. Same publication caveats as **Scenario C**.

**1. DDL (source) only:**

```sql
CREATE TABLE public.nopk_side_t (
	label TEXT NOT NULL,
	n     INT NOT NULL DEFAULT 0
);
ALTER TABLE public.nopk_side_t REPLICA IDENTITY FULL;
```

**Do not** create `nopk_side_t` on the target until you finish steps **1–4** (before **target alignment**), or drop it on both sides when resetting.

**2. Optional DML (source)** — traffic on the new table plus `control_t`:

```sql
INSERT INTO public.control_t (name) VALUES ('d_after_src_nopk_ddl_ctl');
INSERT INTO public.nopk_side_t (label, n) VALUES ('d_src_only_nopk', 1);
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('d_post_resume_ctl');
INSERT INTO public.nopk_side_t (label, n) VALUES ('d_post_resume_nopk', 2);
```

**5. Target alignment (DDL on Yugabyte)** — create a heap-shaped table matching the source (same caveats as **Scenario C** step 5 — may not add the table to export capture):

```sql
CREATE TABLE public.nopk_side_t (
	label TEXT NOT NULL,
	n     INT NOT NULL DEFAULT 0
);
ALTER TABLE public.nopk_side_t REPLICA IDENTITY FULL;
```

**6. Exit + resume** export and import again.

**7. DML (source) after target alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('d_post_workaround_ctl');
INSERT INTO public.nopk_side_t (label, n) VALUES ('d_post_workaround_nopk', 3);
```

### Findings — D (non-PK table)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After **`nopk_side_t` DDL + DML** on source (target initially without `nopk_side_t`) | **`control_t` events only** — **no `nopk_side_t`** rows in the stream | **`control_t`** rows applied; **`nopk_side_t`** had nothing to apply |
| **Exit + resume** export/import | Same — **`nopk_side_t`** still not captured | Same |
| **Step 5** (target `CREATE TABLE nopk_side_t`) + resume import | **`nopk_side_t` still not exported** | Still only **`control_t`** traffic |
| **Mid-migration surgery** (same as **C**: `application.properties`, `name_registry.json`, `meta.db`; **no** sequence map / sequence name edits — heap has no `SERIAL`) | **`export data` exits** — see error below | — |

#### Observed

**Early run (before surgery)** — same pattern as **Scenario C** (table not in capture / `--table-list` rejected if tried):

- **`public.nopk_side_t`** on source with traffic, but **only `public.control_t`** in **export → import**; **`nopk_side_t`** not replicated.
- **Target alignment** + resume did not start **`nopk_side_t`** export; effective registered list stayed **`control_t` / `subject_t`** until registry/metadb were edited.

**After surgery (aligned with Scenario C, minus sequences)**

- Updated **`<export-dir>/metainfo/conf/application.properties`**: `debezium.source.table.include.list` / `debezium.source.column.include.list` included **`public.nopk_side_t`**; **did not** add `debezium.sink.ybexporter.column_sequence.map` / `sequence.max.map` entries (no **`id`** / **`_id_seq`** on this heap).
- Updated **`<export-dir>/metainfo/name_registry.json`**: added **`nopk_side_t`** under **`SourceDBTableNames`** / **`YBTableNames`** only (**no** sequence arrays — none exist for this table).
- Patched **`<export-dir>/metainfo/meta.db`** (`json_objects`): appended **`public.nopk_side_t`** to **`TableListExportedFromSource`** and **`SourceExportedTableListWithLeafPartitions`**; adjusted sequence mapping / partitioning only where applicable (user skipped sequence fields).

**Hard stop on `export data`**

- `yb-voyager export data` printed: **`Table names without a Primary key: [public.nopk_side_t]`** and exited with: *Currently voyager does not support live-migration for tables without a primary key.* (suggests **`--exclude-table-list`**.) This matches the live-migration limitation banner (**tables without a PK are not supported**).

#### Why (from code)

1. **Hard guardrail (non-PK):** `finalizeTableAndColumnList` calls **`reportUnsupportedTablesForLiveMigration`** (`cmd/exportData.go`). For live export it loads **all non-Pk tables** from the source (`GetNonPKTables`) and **any table in `finalTableList` without a PK** triggers **`ErrExit`** with the message above — **before** Debezium can stream that table. Editing **`application.properties`** / metadb **does not bypass** this check; **`nopk_side_t`** in **`finalTableList`** is enough to fail.

2. **Same “registered list / MSR table list / publication” story as C** still applies *if* export got past the non-PK check — but here export **never does** for a heap in the included list.

#### Notes

- **Failure:** **`nopk_side_t`** cannot be part of **live** `export data`’s finalized table list **without a primary key** — voyager **rejects** it explicitly. **Workaround (observed):** **none** via mid-migration surgery alone; add a **PRIMARY KEY** (or use a **PK-backed** table design) if live migration is required, **or** **`--exclude-table-list public.nopk_side_t`** for live export and handle that table outside live CDC.

---

## Scenario E — Renaming a table mid-migration (source DDL first)

Rename **`public.subject_t`** to **`public.subject_renamed_t`** on the **source** first; keep **`public.control_t`** unchanged as a control ping. **Findings** below record one mid-migration run (metadata edits + descriptor + truncate); adjust if your run differs.

**1. DDL (source) only:**

```sql
ALTER TABLE public.subject_t RENAME TO subject_renamed_t;
```

**Do not** rename on the target until you finish steps **1–4** (before **target alignment**), or reset baseline after.

**2. Optional DML (source)** — use the **new** table name:

```sql
INSERT INTO public.control_t (name) VALUES ('e_after_src_rename_ctl');
INSERT INTO public.subject_renamed_t (name, note) VALUES ('e_after_src_rename_sub', NULL);
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('e_post_resume_ctl');
INSERT INTO public.subject_renamed_t (name, note) VALUES ('e_post_resume_sub', NULL);
```

**5. Target alignment (DDL on Yugabyte)** — same rename on the target so the physical table name matches:

```sql
ALTER TABLE public.subject_t RENAME TO subject_renamed_t;
```

**6. Exit + resume** export and import again.

**7. DML (source) after target alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('e_post_align_ctl');
INSERT INTO public.subject_renamed_t (name, note) VALUES ('e_post_align_sub', NULL);
```

### Findings — E (table rename)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After **source** `RENAME` only (steps **1–4**) | No crash; **`control_t`** traffic OK | **`subject_*`** path unhealthy — **`subject_renamed_t`** not replicating as expected |
| **Restart `export data`** (MSR / lists still **`public.subject_t`**) | **Fails** — `reltuples` query for **`public.subject_t`** → relation does not exist | — |
| After **metadb + Debezium + name registry + target `RENAME`** | **Resumes** | **Fails** — registry lookup still for **`"public"."subject_t"`** (stale **`dataFileDescriptor.json`**) |
| After **`dataFileDescriptor.json`** points **`subject_renamed_t`** | OK | **`COPY`** snapshot → **duplicate PK** (snapshot treated **not started**) |
| After **`TRUNCATE`** `subject_renamed_t` on target + resume import | — | Snapshot **reloads**; **one historical CDC event** still **missing**; **new** events OK |

#### Observed

- After **source** rename to **`subject_renamed_t`**, **export/import did not crash** immediately; **`control_t`** inserts worked; **`subject_renamed_t`** did not behave like a healthy replicated table early on.
- **Restart `export data`:** failed with **`Failed to query for approx row count of table ... subject_t`** — `SELECT reltuples ... 'public.subject_t'::regclass` → **`relation "public.subject_t" does not exist`**. Log still showed **`table list for data export: [public.control_t public.subject_t]`** (stored list / NameTuple lagged the rename).
- **Mid-migration alignment (worked toward consistency):** patched **`metainfo/meta.db`** `migration_status` (`TableListExportedFromSource` / `SourceExportedTableListWithLeafPartitions` entries → **`public.subject_renamed_t`**, sequence mapping key → **`public.subject_renamed_t.id`**, etc.); updated **`metainfo/conf/application.properties`** (`table.include.list`, `column.include.list`, sink maps) everywhere the old name appeared; updated **`metainfo/name_registry.json`** so **`SourceDBTableNames`** / **`YBTableNames`** list **`subject_renamed_t`**; ran **`ALTER TABLE public.subject_t RENAME TO subject_renamed_t`** on **Yugabyte**. **Export** then resumed fine.
- **`import data`:** failed with **`lookup table name from name registry ... ["public"."subject_t"]` / `table name not found: subject_t`** — **`discoverFilesToImport`** still read the **old** table name from **`metainfo/dataFileDescriptor.json`** while the registry no longer listed **`subject_t`**.
- **Descriptor fix:** set **`FileList[].TableName`** and **`TableNameToExportedColumns`** keys to **`"public"."subject_renamed_t"`** (kept **`FilePath`** `.../subject_t_data.sql`). **Import** then ran **`COPY`** into **`public.subject_renamed_t`** and hit **`23505` duplicate key** on **`subject_t_pkey`** (constraint name unchanged after table rename is normal).
- **After `TRUNCATE public.subject_renamed_t` on target** and resuming **import:** snapshot **reloaded** successfully; **streaming did not replay** one already-consumed historical CDC change — **one row/event remained missing**; **new** source events after resume replicated normally.

#### Why (from code)

1. **Export approx row count:** `prepareDebeziumConfig` / export path builds **`getTableApproxRowCount`** from the **final table list** (`exportData.go` → `getTableNameToApproxRowCountMap` → `PostgreSQL.GetTableApproxRowCount` in `src/srcdb/postgres.go`). If the list still says **`public.subject_t`** but the relation was renamed, **`regclass`** lookup fails.

2. **Import snapshot task discovery:** `discoverFilesToImport` uses **`fileEntry.TableName`** from **`metainfo/dataFileDescriptor.json`** and **`namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole`** (`importData.go`). Stale **`"public"."subject_t"`** in the descriptor with an updated registry causes the **lookup** failure you saw.

3. **Snapshot resume vs rename:** `classifyTasksForImport` calls **`GetFileImportState(task.FilePath, task.TableNameTup)`** (`importData.go`). **`GetFileImportState`** uses **`getFileStateDir`** which embeds **`tableNameTup.ForKey()`** in the path (`importDataState.go` — `.../table::<ForKey>/file::<basename>::<hash>/`). Changing the logical table name **without moving** the old `table::"public"."subject_t"/...` progress tree leaves **zero batches** under the new key → **`FILE_IMPORT_NOT_STARTED`** → snapshot **`COPY`** runs again.

4. **CDC not rewound on truncate:** Truncating the **target** does not reset **replication offsets** or importer **applied event** positions; anything already past in the stream is not automatically re-emitted, so a **single lost change** can remain missing while **tail** events still flow.

#### Notes

- **Failure:** Mid-**rename**, voyager’s **stored names** (MSR, descriptor, registry, Debezium, **`import_data_state` paths**) fell **out of sync** with the live catalog — **export** then **import** failures, then **duplicate snapshot** risk after descriptor/registry fixes. **Workaround (observed):** align **`meta.db`** + **`application.properties`** + **`name_registry.json`** + **target `RENAME`** + **`dataFileDescriptor.json`**; for **duplicate PK**, **`TRUNCATE`** target table (or delete conflicting PKs) before resuming import; accept **manual repair** (re-emit change / hand-fix row) for **gaps** the stream will not rewind.

---

## Scenario F — Rename column on **source** only

Rename **`public.subject_t.note`** to **`note_renamed`** on **PostgreSQL** first. **`public.control_t`** stays as a control ping. Use literal names prefixed **`f_`** so you can grep logs and rows.

**1. DDL (source) only:**

```sql
ALTER TABLE public.subject_t RENAME COLUMN note TO note_renamed;
```

**Do not** run the matching **`RENAME COLUMN`** on Yugabyte until you finish steps **1–4** (before **target alignment**), or reset baseline after.

**2. Optional DML (source)** — use the **new** column name:

```sql
INSERT INTO public.control_t (name) VALUES ('f_after_src_col_rename_ctl');
INSERT INTO public.subject_t (name, note_renamed) VALUES ('f_after_src_col_rename_sub', NULL);
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('f_post_resume_ctl');
INSERT INTO public.subject_t (name, note_renamed) VALUES ('f_post_resume_sub', NULL);
```

**5. Target alignment (DDL on Yugabyte)** — same column rename on the target:

```sql
ALTER TABLE public.subject_t RENAME COLUMN note TO note_renamed;
```

**6. Exit + resume** export and import again.

**7. DML (source) after target alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('f_post_align_ctl');
INSERT INTO public.subject_t (name, note_renamed) VALUES ('f_post_align_sub', 'v_after_align');
```

### Findings — F (rename column, source first)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After source **`RENAME COLUMN`** (`note` → `note_renamed`), before target DDL | Still running; **CDC events for `subject_t` exported** | **`control_t`** events applied; first **`subject_t`** event → **panic** |
| **Restart** import only (target still `note`) | _(unchanged — export still OK)_ | **Same panic** on replay / next **`subject_t`** batch |
| After **`ALTER TABLE … RENAME COLUMN note TO note_renamed`** on **Yugabyte** + resume | OK | OK; **new events** through |

#### Observed

- **`control_t`** DML continued to replicate while **`subject_t`** was broken — consistent with table-local schema: importer maps **per-event column names** against the **target** catalog for each table.
- **`import data` / live stream** panicked when applying a **`subject_t`** change that referenced **`note_renamed`**:  
  `panic: quote column name : find best matching column name for "note_renamed" in table [CurrentName=(subject_t) SourceName=(subject_t) TargetName=(subject_t)]: column "note_renamed" not found amongst table columns [id note name]`  
  Stack: **`tgtdb.(*Event).getPreparedInsertStmt`** (`event.go` ~356) → **`GetPreparedSQLStmt`** → **`TargetYugabyteDB.ExecuteBatch`** → **`processEvents`** (`live_migration.go`).
- **Restarting import** did **not** help — same failure until the **target** DDL aligned the physical column name with the **source/CDC** payload.
- After **`RENAME COLUMN`** on the **target** to **`note_renamed`**, import succeeded and **tail events** replicated normally.

#### Why (from code)

- Live import builds SQL from **`Event.Fields`** / **`Event.Key`** using **`TargetDB.QuoteAttributeName`** (`tgtdb/event.go` — e.g. **`getPreparedInsertStmt`**). For YugabyteDB that goes through **`AttributeNameRegistry.QuoteAttributeName`** (`tgtdb/attr_name_registry.go`): it loads **target** column names via **`GetListOfTableAttributes`**, then **`findBestMatchingColumnName`**. If the CDC column name (**`note_renamed`**) is not in that list (target still had **`id`, `note`, `name`**), lookup fails and **`getPreparedInsertStmt`** **panics** on the error (unlike some other stmt builders that return `error`).
- **`control_t`** never referenced **`note_renamed`**, so it never hit that code path.

#### Notes

- **Failure:** **Source-ahead column rename** — Debezium / export emitted **`note_renamed`** while Yugabyte still had **`note`** → **hard panic** on **`subject_t`** event application; **resume alone** cannot fix without **target DDL** (or some other way to make target columns match event names).
- **Workaround (observed):** **`ALTER TABLE public.subject_t RENAME COLUMN note TO note_renamed`** on the **target** (same as source), then **resume** — import and **new** events OK.

---

## Scenario G — Alter column type on **source** only (incompatible vs target)

Widen **`public.subject_t.score`** from **`INTEGER`** to **`NUMERIC(10,2)`** on **PostgreSQL** while **YugabyteDB** keeps **`INT`** / **`INTEGER`**, so CDC payloads can carry **fractional** or **wider** values the target type rejects or mishandles. **`public.control_t`** is the control ping. Use literals prefixed **`g_`** for grepping.

### Prerequisite (both sides — baseline has no **`score`**)

Run **the same** DDL on **source** and **target** after your usual baseline (empty tables + seeds on source only):

```sql
ALTER TABLE public.subject_t
	ADD COLUMN score INTEGER NOT NULL DEFAULT 0;
```

Confirm **`subject_t`** lists **`score`** as **`integer`** on the target before you run **step 1** on the source only.

**1. DDL (source) only** — type change (PostgreSQL):

```sql
ALTER TABLE public.subject_t
	ALTER COLUMN score TYPE NUMERIC(10, 2)
	USING score::numeric;
```

**Do not** run the matching **`ALTER … TYPE`** on Yugabyte until you finish steps **1–4** (before **target alignment**), or reset afterward per the intro.

**2. Optional DML (source)** — values that are valid **`NUMERIC`** but stress **`INTEGER`** on the lagging target:

```sql
INSERT INTO public.control_t (name) VALUES ('g_after_src_type_ctl');
INSERT INTO public.subject_t (name, note, score) VALUES ('g_after_src_type_sub', NULL, 1.25);
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('g_post_resume_ctl');
INSERT INTO public.subject_t (name, note, score) VALUES ('g_post_resume_sub', NULL, 42.5);
```

**5. Target alignment (DDL on Yugabyte)** — match source type (YSQL):

```sql
ALTER TABLE public.subject_t
	ALTER COLUMN score TYPE NUMERIC(10, 2)
	USING score::numeric;
```

If your Yugabyte build rejects that exact **`USING`** form, use the closest supported equivalent (e.g. two-step cast) and record what worked in **Observed**.

**6. Exit + resume** export and import again.

**7. DML (source) after target alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('g_post_align_ctl');
INSERT INTO public.subject_t (name, note, score) VALUES ('g_post_align_sub', NULL, 99.99);
```

### Findings — G (column type drift, source ahead)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After source **`ALTER COLUMN score TYPE NUMERIC…`**, target still **`INTEGER`** | No immediate issue; **exporter kept running** and **exported** the **`subject_t`** events | **`control_t`** event(s) applied; first **`subject_t`** batch → **fatal error** (not a panic) |
| **Resume** import (target still **`INTEGER`**) | Still **exporting** events on resume | **Same `22P02` failure** replaying the stuck batch |
| After **`ALTER COLUMN score TYPE NUMERIC…`** on **Yugabyte** + resume | OK | OK — **all events** eventually **imported** |

#### Observed

- No failure right after **DDL** on the source alone; **`control_t`** DML replicated (**metrics** showed **1** imported event — control path).
- **`subject_t`** insert with fractional **`score`** (**`1.25`**) caused import to **exit**:  
  `error executing batch on channel 17: error executing batch: error preparing statements for events in batch (2:2) or when executing event with vsn(2): ERROR: invalid input syntax for integer: "1.25" (SQLSTATE 22P02)`  
  **Exporter** remained healthy and had **exported** the event(s).
- **Resume** hit the **same** **`22P02`** (still binding **`"1.25"`** into target **`INTEGER`**).
- After **target** **`ALTER … TYPE NUMERIC(10,2)`** (alignment DDL), **resume** succeeded; **all** events **exported and imported**.

#### Why (from code)

- Live import applies CDC rows via **`TargetYugabyteDB.ExecuteBatch`** (`tgtdb/yugabytedb.go`): events are turned into **prepared** `INSERT`/`DELETE` (and raw SQL for **`u`**) and run in a **`pgx` batch** inside a transaction. **`QuoteAttributeName`** still resolves **`score`** on the target, but the **parameter values** come from the **CDC event** (here a **decimal** string like **`"1.25"`** after the source widened to **`NUMERIC`**). Yugabyte/YSQL still types the column as **`integer`**, so the server rejects the literal → **`22P02 invalid input syntax for integer`**.
- The wrapper message **`error preparing statements … or when executing event with vsn(…)`** on the **first** `br.Exec()` in the batch is **ambiguous by design** in **`ExecuteBatch`** (comment in **`yugabytedb.go`** ~1141–1154: pgx can surface **prepare** vs **execute** failures on the first `Exec`); here the underlying cause is the **execute** path / **invalid cast** into **`INTEGER`**.

#### Notes

- **Failure:** **Source-ahead type widen** — fractional **`NUMERIC`** values in the stream while the target column stayed **`INTEGER`** → **`SQLSTATE 22P02`** on batch apply; **`resume` does not skip** the bad batch, so it **fails again** until the **target type** matches what the events carry.
- **Workaround (observed):** **`ALTER TABLE public.subject_t ALTER COLUMN score TYPE NUMERIC(10,2) USING score::numeric`** (or equivalent) on **Yugabyte**, then **resume** — full **catch-up**.

---

## Scenario H — Alter column type on **source** only (**NUMERIC** → **TEXT**, vs target **NUMERIC**)

Change **`public.subject_t.score`** to **`TEXT`** on **PostgreSQL** while **YugabyteDB** keeps **`NUMERIC(10,2)`**, to see whether live import keeps accepting **CDC string** values into the narrower target type (often **implicit cast** succeeds — unlike **G**’s fraction-into-**`INTEGER`** case). **`control_t`** is the control ping. Literals prefixed **`h_`**.

### Prerequisite (both sides — **`score`** as **`NUMERIC`**)

After baseline (same as **G**): add **`score`** as **`NUMERIC`** on **source and target**:

```sql
ALTER TABLE public.subject_t
	ADD COLUMN score NUMERIC(10, 2) NOT NULL DEFAULT 0;
```

**1. DDL (source) only:**

```sql
ALTER TABLE public.subject_t
	ALTER COLUMN score TYPE TEXT
	USING score::text;
```

**Do not** run the matching **`ALTER … TYPE TEXT`** on Yugabyte until after steps **1–4**, or reset afterward per the intro.

**2. Optional DML (source)** — **`score`** is now **text** (use quotes / casts as you like):

```sql
INSERT INTO public.control_t (name) VALUES ('h_after_src_type_ctl');
INSERT INTO public.subject_t (name, note, score) VALUES ('h_after_src_type_sub', NULL, '7.5');
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('h_post_resume_ctl');
INSERT INTO public.subject_t (name, note, score) VALUES ('h_post_resume_sub', NULL, 'not-a-number');
```

(Second row probes **non-numeric** text — may fail on target while still **`NUMERIC`**; note the error in **Observed**.)

**5. Target alignment (DDL on Yugabyte)** — match source:

```sql
ALTER TABLE public.subject_t
	ALTER COLUMN score TYPE TEXT
	USING score::text;
```

**6. Exit + resume** export and import again.

**7. DML (source) after target alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('h_post_align_ctl');
INSERT INTO public.subject_t (name, note, score) VALUES ('h_post_align_sub', NULL, 'plain-text-ok');
```

### Findings — H (NUMERIC → TEXT on source, target lagging)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After source **`score`** **`NUMERIC` → `TEXT`**, DML with **`'7.5'`** (steps **2–3**) | OK | **Both events applied** — **no failure** |
| **Exit + resume** (still target **`NUMERIC`**, source **`TEXT`**) | OK | **OK** |
| DML with **`score = 'not-a-number'`** (step **4**) while target still **`NUMERIC`** | OK | **`22P02`** — import **exits**; **resume** did **not** recover (same class of stuck batch as **G**) |
| After target **`ALTER … TYPE TEXT`** + **resume** | OK | **All events** applied |

#### Observed

- **Source-only** type change (**`NUMERIC` → `TEXT`**) did **not** by itself break live import: events with **numeric-looking** text (**`'7.5'`**) replicated while the target column was still **`NUMERIC`**.
- **Exit + resume** with that **source/target type skew** remained **fine** until a value **stopped being a valid `NUMERIC` literal**.
- Insert **`h_post_resume_sub`** with **`'not-a-number'`** caused:  
  `error executing batch on channel 6: error executing batch: error preparing statements for events in batch (4:4) or when executing event with vsn(4): ERROR: invalid input syntax for type numeric: "not-a-number" (SQLSTATE 22P02)`  
  **Resume** did **not** clear it (same **bad batch** / **non-retryable** outcome pattern as **G**).
- **Target** **`ALTER COLUMN score TYPE TEXT`** (alignment), then **resume import** — **succeeded**; **all events** **exported and imported**.

#### Why (from code)

- Same ingestion path as **G**: **`TargetYugabyteDB.ExecuteBatch`** (`tgtdb/yugabytedb.go`) applies **prepared** `INSERT` parameters against the **target** column type. With **`score`** still **`NUMERIC`** on Yugabyte, the server **coerces** string parameters that are **valid numeric text** (**`'7.5'`**) but rejects **`'not-a-number'`** → **`22P02 invalid input syntax for type numeric`**. **`processEvents`** treats the batch as failed and **`ErrExit`** after retries — **resume** reprocesses the same event batch until the **target type** widens to **`TEXT`**.

#### Notes

- **“Compatible” only up to a point:** **Source `TEXT` / target `NUMERIC`** is **not** a safe long-term skew — it works only for values that still **parse as `NUMERIC`** on the target.
- **Failure:** **Non-numeric** text in the CDC stream while the target stayed **`NUMERIC`** → **`22P02`**; **resume alone** insufficient.
- **Workaround (observed):** **`ALTER COLUMN score TYPE TEXT`** on **Yugabyte**, **resume** — full **catch-up**.

---

## Scenario I — **`DROP TABLE`** on **source** only

**`DROP TABLE public.subject_t`** on **PostgreSQL** while **`public.subject_t`** still exists on **YugabyteDB** and is still listed in voyager’s **export / CDC** configuration. **`public.control_t`** remains for control traffic. Literals **`i_*`**.

**1. DDL (source) only:**

```sql
DROP TABLE public.subject_t CASCADE;
```

**Do not** drop **`subject_t`** on the target until you have finished the **observe** part of steps **1–4**, unless you are deliberately mirroring “gone on both sides” — record outcomes in **Observed**.

**2. Optional DML (source)** — **`control_t`** only (**`subject_t`** no longer exists):

```sql
INSERT INTO public.control_t (name) VALUES ('i_after_drop_ctl');
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('i_post_resume_ctl');
```

**5. Mid-migration surgery (same `<export-dir>` — source no longer has `public.subject_t`)**

Dropping **`subject_t`** on **Yugabyte** alone does **not** fix **export**: resume still reads the **stored** table list from **`<export-dir>/metainfo/meta.db`** and runs **`GetTableApproxRowCount`** / Debezium prep against the **source**. Until voyager’s lists match the **live** source, **`public.subject_t`** in MSR but missing on PostgreSQL → **`relation "public.subject_t" does not exist` (`42P01`)** (same class as **Scenario E**).

Work on a **stopped** migration: **quit** **`export data`** and **`import data`** so nothing holds **`meta.db`** open.

**5.1 — Back up**

Copy at least:

- **`<export-dir>/metainfo/meta.db`**
- **`<export-dir>/metainfo/conf/application.properties`**
- **`<export-dir>/metainfo/name_registry.json`**
- **`<export-dir>/metainfo/dataFileDescriptor.json`**
- **`<export-dir>/data/postdata.sql`** (if it exists — see **5.2b**)
- **`<export-dir>/data/schemas/`** (at least any files you will delete — see **5.4a**)
- **`<export-dir>/data/queue/`** (if you will edit **`segment.*.ndjson`** — see **5.4b**)

**5.2 — Patch migration status in `meta.db`**

Voyager stores the MSR under SQLite table **`json_objects`**, row **`key` = `migration_status`**, column **`json_text`** (JSON object).

1. Open **`<export-dir>/metainfo/meta.db`** with **`sqlite3`** (or any SQLite tool).
2. **`SELECT json_text FROM json_objects WHERE key = 'migration_status';`** — save the payload to a file, edit as JSON, write back with **`UPDATE json_objects SET json_text = '…' WHERE key = 'migration_status';`** (escape quotes as required), or use a small script to load → modify → save.
3. In that JSON object, edit **both**:
   - **`TableListExportedFromSource`**: remove every string that refers to **`public.subject_t`** (match the **exact** spelling/format already stored, e.g. **`"\"public\".\"subject_t\""`** vs **`public.subject_t`** — copy from the file, do not guess).
   - **`SourceExportedTableListWithLeafPartitions`**: remove **`public.subject_t`** **and** any **partition-only** names that belong to that table (PostgreSQL **resume** reads this list in **`retrieveFirstRunListAndPartitionsRootMap`** — `exportData.go`). Leave only entries for tables that **still exist** on the source (e.g. **`public.control_t`**).
4. If **`SourceRenameTablesMap`** (or other MSR maps) still mention **`subject_t`**, remove or adjust those keys/values so they do not reference a dropped relation.

**5.2a — `SourceColumnToSequenceMapping` (same `migration_status` JSON)**

**`fetchOrRetrieveColToSeqMap`** (`exportDataDebezium.go`) returns the **stored** map whenever it is **non-`null`**, without re-querying the source for the new table list. Remove **every** key whose column belongs to **`public.subject_t`** (not only **`…id`** — keys follow **`GetColumnToSequenceMap`** / qualified column form, e.g. **`public.subject_t.id`** depending on how it was serialized).

If you are unsure you got them all, **delete the `SourceColumnToSequenceMapping` property from the JSON entirely** (or set it to JSON **`null`**) so it unmarshals as **nil** in Go and the next **`export data`** recomputes **`GetColumnToSequenceMap(tableList)`** for the **shrunken** list (e.g. **`control_t`** only) and writes a fresh map. **Do not** replace it with **`{}`** only: an **empty object** is still a **non-`nil`** map and can skip the refresh path.

**5.2b — `data/postdata.sql` (sequence / identity snapshot tail)**

For **live / snapshot + streaming** PostgreSQL export, **`getSequenceInitialValues`** (`exportData.go`) reads **`<export-dir>/data/postdata.sql`**, finds **`SELECT … setval('…seq' , …)`** lines, and runs **`namereg.NameReg.LookupTableName`** on each sequence name. If that file still contains **`public.subject_t_id_seq`** (or any sequence for the dropped table) but you removed the sequence from **`name_registry.json`**, resume fails with **`get sequence initial values: lookup for sequence name public.subject_t_id_seq: …`**.

**Edit `postdata.sql`:** delete every **`setval`** line for sequences tied to **`subject_t`** (typically **`public.subject_t_id_seq`** for a **`BIGSERIAL`** PK). Leave **`setval`** lines only for sequences that still exist on the source **and** remain registered.

**5.3 — Debezium: `application.properties`**

Edit **`<export-dir>/metainfo/conf/application.properties`**:

- Set **`debezium.source.table.include.list=`** to a comma-separated list of **qualified tables that still exist on the source** (same catalog names Debezium expects — typically **`schema_drift.public.control_t`** only if that is the sole survivor). **Do not** leave **`public.subject_t`** in the list.  
  _(Property name comes from **`src/dbzm/config.go`**.)_

**5.4 — Name registry**

Edit **`<export-dir>/metainfo/name_registry.json`**:

- Remove **`subject_t`** from **`SourceDBTableNames`** / **`YBTableNames`** (and any nested structure) so the registry no longer maps a dropped source table. Keep **`control_t`** (and your other live tables) consistent between source and target sides of the file.

**5.4a — Debezium schema JSON (`data/schemas/`)**

**`SchemaRegistry.Init`** (`src/utils/schemareg/schemaRegistry.go`) walks **every** non-**`.tmp`** file under **`<export-dir>/data/schemas/<exporter_role>/`** (e.g. **`source_db_exporter`**, and similarly **`target_db_exporter_fb`** / **`target_db_exporter_ff`** if present). For each **`*_schema.json`**, it derives the table name from the filename and calls **`namereg.NameReg.LookupTableNameAndIgnoreIfTargetNotFoundBasedOnRole`**. If you **delete `subject_t` from `name_registry.json`** but leave **`…subject_t…_schema.json`** on disk, **`import data`** can fail at **`NewStreamingPhaseDebeziumValueConverter`** / **`initializing schema registry`** with **`lookup subject_t from name registry: … table name not found: subject_t`** — **dropping the target table does not remove these files**.

**Delete** (or move aside) all **`*_schema.json`** files that correspond to **`public.subject_t`** under **`data/schemas/`** for the roles your migration uses, **after** backup.

**5.4b — Event queue (`data/queue/*.ndjson`)**

**`Event.UnmarshalJSON`** (`tgtdb/event.go`) resolves **`schema_name` + `table_name`** through **`namereg.NameReg.LookupTableName`** for **every** line read from **`segment.<n>.ndjson`** (**`eventQueue.go`** → **`json.Unmarshal`**). If **`subject_t`** was removed from **`name_registry.json`** but a **queued** line still has **`"table_name":"subject_t"`**, **`import data`** fails while **streaming** with **`failed to unmarshal json event …: lookup table public.subject_t in name registry`** — **not** a JSON syntax error; **unmarshal** includes that lookup.

**After backup:** remove **all NDJSON lines** for the dropped table from **`data/queue/segment.*.ndjson`** (one JSON object per line; keep the final **`\.`** EOF line if your segment uses it). **Do not leave blank lines** — **`NextEvent`** (`cmd/eventQueue.go`) calls **`json.Unmarshal`** on every line read; an **empty line** yields **`failed to unmarshal json event : … unexpected end of JSON input`** (notice the empty payload before the colon). Likewise, **`size_committed` smaller than the true file length** can **truncate** the tail of a line and produce a similar JSON error. Then set **`queue_segment_meta.size_committed`** in **`meta.db`** for each edited **`segment_no`** to the **new file byte length** (see **`GetLastValidOffsetInSegmentFile`** in **`metadb/metadataDB.go`** — the importer **tails** from that offset). If you are unsure, **archive** the whole **`data/queue/`** tree and **`DELETE FROM queue_segment_meta;`** only as a **last resort** with a plan for **offsets / duplicates** (risky).

**Practical recommendation:** Editing **segment files** and keeping **`queue_segment_meta.size_committed`** aligned with **`meta.db`** is easy to get wrong (blank lines, truncated JSON, offset / duplicate risk). Unless you **must** preserve this export directory, **starting a new migration** (fresh export path, clean catalog alignment on source/target) is usually **simpler and safer** than deep **queue** surgery.

**5.5 — Snapshot descriptor**

Edit **`<export-dir>/metainfo/dataFileDescriptor.json`**:

- Remove **`DataFileList`** (or equivalent) entries whose **`TableName`** is **`public.subject_t`** (again: match **stored** spelling). Otherwise **import** can still try to resolve snapshot work for a table that should leave the pipeline (same theme as **Scenario E**).

**5.6 — Import progress tree (optional but often needed)**

Under **`<export-dir>/metainfo/import_data_state/`**, locate directories keyed like **`table::"public"."subject_t"`** (exact layout depends on **`NameTuple.ForKey()`** — see **`importDataState.go`**). **Remove** that subtree (or rename aside) so import state for the dropped table does not fight a shrunken table list. **Expect** to need **target cleanup** (e.g. **`TRUNCATE`** / **`DROP`**) if you ever reintroduce snapshot **`COPY`** for a table that partially imported — same duplicate risk as **Scenario E**; record what you did in **Observed**.

**5.7 — Optional: stats rows in `meta.db`**

If **`exported_events_stats_per_table`** (or related tables) still has rows for **`subject_t`**, you may **`DELETE`** those rows to avoid confusing status — only after backup; not always required.

**5.8 — Optional: target catalog parity**

On **YugabyteDB**, **`DROP TABLE IF EXISTS public.subject_t CASCADE;`** if you want the target to match “table gone on source.” This **does not** fix **export** by itself; it aligns the **target** with the **new** schema.

**5.9 — Source publication (sanity check)**

On **PostgreSQL**, confirm the replication **publication** voyager/Debezium uses no longer expects the dropped table (after **`DROP TABLE`**, the catalog usually drops it from the publication; if you use a custom publication, **`ALTER PUBLICATION … DROP TABLE …`** if a stale entry remains).

**6. Exit + resume** `export data` and `import data` again (after **5.1–5.9** as needed).

**7. DML (source)** — **`control_t`** only ( **`subject_t`** stays dropped ):

```sql
INSERT INTO public.control_t (name) VALUES ('i_post_align_ctl');
```

### Findings — I (source drop, target / metainfo lagging)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| Right after **`DROP TABLE public.subject_t`** on source | No immediate crash | **`control_t`** events still applied |
| **Resume** while MSR / Debezium / descriptors still listed **`subject_t`** | **`42P01`**: **`Failed to query for approx row count`** — **`relation "public.subject_t" does not exist`** (stored list vs live catalog) | **Importer** could **resume safely** (did not mirror export failure) |
| **Target** **`DROP subject_t`** only | **No fix** — same **source**-side **`reltuples` / `regclass`** path | — |
| After **mid-migration surgery** (**`meta.db`** lists, **`import_data_status`**, **`dataFileDescriptor.json`**, **`name_registry.json`**, **`application.properties`**) but **`postdata.sql`** still had **`setval`** for **`subject_t_id_seq`** | **`get sequence initial values: lookup for sequence name public.subject_t_id_seq: … table name not found: subject_t_id_seq`** | _(not reported broken at this step)_ |
| After **removing** the **`subject_t`** **`setval`** line(s) from **`<export-dir>/data/postdata.sql`** | **Exporter resumed and ran fine** | _(user run: OK where exercised)_ |

#### Observed

- **`DROP TABLE`** on the source did **not** panic live paths immediately; **`control_t`** DML continued to replicate.
- **`export data` resume** failed while **`migration_status`** still named **`public.subject_t`**: approx row count query uses **`'public.subject_t'::regclass`** on the **source** → **`42P01`**. Log also showed the generic **“Tables without a Primary Key…”** line alongside the real failure.
- **`DROP TABLE public.subject_t`** on **Yugabyte** did **not** repair **export** (target is irrelevant to that **PostgreSQL** catalog query).
- **Surgery** removing **`subject_t`** from **`TableListExportedFromSource`**, **`SourceExportedTableListWithLeafPartitions`**, **`SourceColumnToSequenceMapping."public.subject_t.id"`**, tightening **`import_data_status`** **`tableToCDCPartitioningStrategyMap`**, cleaning **`dataFileDescriptor.json`** and **`name_registry.json`**, was **necessary but not sufficient**: export still failed on **`get sequence initial values`** for **`public.subject_t_id_seq`** until **`postdata.sql`** was edited.
- **Deleting** the **`setval('public.subject_t_id_seq', …)`** (or equivalent) line from **`<export-dir>/data/postdata.sql`** fixed the remaining error; **exporter** then **resumed and worked**.

#### Why (from code)

- **Approx row count:** **`prepareDebeziumConfig`** → **`getTableNameToApproxRowCountMap`** → **`PostgreSQL.GetTableApproxRowCount`** (`exportDataDebezium.go` / `exportData.go` / `postgres.go`) walks the **stored** table list from **`meta.db`** (**`retrieveFirstRunListAndPartitionsRootMap`** in **`exportData.go`** uses **`SourceExportedTableListWithLeafPartitions`** on PG). Missing relation on the source → **`42P01`**.
- **Sequences:** For **snapshot + streaming**, **`getSequenceInitialValues`** (`exportData.go`) reads **`<export-dir>/data/postdata.sql`**, parses **`setval`** sequence names, and resolves them with **`namereg.NameReg.LookupTableName`**. Stale **`subject_t_id_seq`** after **`name_registry`** / table surgery → lookup failure. **`fetchOrRetrieveColToSeqMap`** (`exportDataDebezium.go`) can also keep a **stale** **`SourceColumnToSequenceMapping`** if the MSR map stays **non-`null`** (including a wrongly **empty `{}`** — see step **5.2a** in the playbook).

#### Notes

- **Failure:** **Source `DROP TABLE`** while voyager + files still described **`subject_t`** → **export resume** **`42P01`**; further stale **`postdata.sql`** / sequence mapping → **`get sequence initial values`** error. **Target-only DDL** does **not** fix **export**.
- **Workaround (observed):** **Mid-migration surgery** per **step 5** (MSR table lists, Debezium **`table.include.list`**, registry, descriptor, optional **`import_data_state`**, **`SourceColumnToSequenceMapping` / `null` not `{}`**) **plus** removing **`subject_t`** **`setval`** lines from **`data/postdata.sql`** — then **exporter OK**.

---

## Scenario J — **`DROP COLUMN`** (**nullable** `note`) on **source** only

Baseline **`note`** is **`TEXT`** **without** **`NOT NULL`** (nullable). Drop **`public.subject_t.note`** on **PostgreSQL** first while **YugabyteDB** still has **`note`**. **`control_t`** is the control ping. Literals **`j_*`**.

**1. DDL (source) only:**

```sql
ALTER TABLE public.subject_t DROP COLUMN note;
```

**Do not** run **`DROP COLUMN note`** on Yugabyte until after steps **1–4** (before **target alignment**), or restore **`note`** afterward per the intro.

**2. Optional DML (source)** — inserts **without** **`note`**:

```sql
INSERT INTO public.control_t (name) VALUES ('j_after_drop_ctl');
INSERT INTO public.subject_t (name) VALUES ('j_after_drop_sub');
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('j_post_resume_ctl');
INSERT INTO public.subject_t (name) VALUES ('j_post_resume_sub');
```

**5. Target alignment (DDL on Yugabyte)** — match source:

```sql
ALTER TABLE public.subject_t DROP COLUMN note;
```

**6. Exit + resume** export and import again.

**7. DML (source) after target alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('j_post_align_ctl');
INSERT INTO public.subject_t (name) VALUES ('j_post_align_sub');
```

### Findings — J (nullable column drop, source ahead)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After source **`DROP COLUMN note`** (target still has **`note`**) | OK | **`control_t`** and **`subject_t`** events **OK** |
| **Exit + resume** (skew unchanged) | OK | OK |
| After **target** **`DROP COLUMN note`** + resume | OK | OK |

#### Observed

- **`control_t`** and **`subject_t`** replication stayed **healthy** after the source dropped the **nullable** **`note`** column while the target still had **`note`**.
- **Exit + resume** of export and import **succeeded** with that skew.
- **Target alignment** (**`DROP COLUMN note`** on Yugabyte) + resume — **still fine**; no errors reported in this run.

#### Why (from code)

- New **`subject_t`** rows use **`INSERT`** events whose **field set** matches the **source** table (no **`note`** after the drop). The importer builds **`INSERT`** / prepared statements from **`Event.Fields`** (`tgtdb/event.go`); columns **not** in the payload are simply **absent** from the statement. On the target, **`note`** can remain as a real column and receive the **column default** (**`NULL`** for nullable **`TEXT`**) for those rows, so there is **no** “unknown column name” panic (contrast **Scenario F** rename) and no type coercion failure for removed fields.

#### Notes

- **Failure:** **None observed** for **nullable** **`note`** in this run.  
- **Workaround:** **None required** for replication; **target `DROP COLUMN note`** was still done for **catalog parity** with the source.

---

## Scenario K — **`DROP COLUMN`** (**`NOT NULL`**) on **source** only

Drop a **`NOT NULL`** column on **PostgreSQL** first while **YugabyteDB** keeps it, to see whether **CDC** / importer behave differently than **J** (e.g. historical events still carrying the field, defaults, or errors). **`control_t`** control ping. Literals **`k_*`**.

### Prerequisite (both sides — add **`tag`**)

After baseline, add the same column on **source** and **target**:

```sql
ALTER TABLE public.subject_t
	ADD COLUMN tag TEXT NOT NULL DEFAULT 'k_seed';
```

The **`DEFAULT`** matters for the **skew phase** (see **Findings — K → Why**): CDC **`INSERT`**s built from the source omit **`tag`** once it is dropped there; on the target, **`INSERT`** statements generated by voyager typically **omit** that column too, so Yugabyte fills **`tag`** from the **table default**. To probe **`NOT NULL`** **without** a default, you must end up with a target definition that still allows those inserts (e.g. **empty** table + **`ADD COLUMN tag TEXT NOT NULL`** with **no** default on both sides **before** any rows exist, or a **two-step** add: add nullable → backfill → set **`NOT NULL`** → **`ALTER … DROP DEFAULT`** on the target only while still testing — expect **`23502`** / not-null violations if the engine cannot infer a value).

**1. DDL (source) only:**

```sql
ALTER TABLE public.subject_t DROP COLUMN tag;
```

**Do not** drop **`tag`** on Yugabyte until after steps **1–4**, or remove **`tag`** on both sides afterward (**`ALTER TABLE public.subject_t DROP COLUMN IF EXISTS tag`**) or re-baseline.

**2. Optional DML (source)** — omit **`tag`** (column gone):

```sql
INSERT INTO public.control_t (name) VALUES ('k_after_drop_ctl');
INSERT INTO public.subject_t (name) VALUES ('k_after_drop_sub');
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('k_post_resume_ctl');
INSERT INTO public.subject_t (name) VALUES ('k_post_resume_sub');
```

**5. Target alignment (DDL on Yugabyte)** — match source:

```sql
ALTER TABLE public.subject_t DROP COLUMN tag;
```

**6. Exit + resume** export and import again.

**7. DML (source) after target alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('k_post_align_ctl');
INSERT INTO public.subject_t (name) VALUES ('k_post_align_sub');
```

### Findings — K (`NOT NULL` column drop, source ahead)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After source **`DROP COLUMN tag`** while target still has **`NOT NULL tag`** (with **`DEFAULT 'k_seed'`**) | OK | **`control_t`** and **`subject_t`** events **OK** |
| **Exit + resume** | OK | OK — **all events** applied |
| After **target** **`DROP COLUMN tag`** + resume | OK | OK |

#### Observed

- Same qualitative outcome as **J**: after dropping **`tag`** on the **source** only, **`control_t`** and **`subject_t`** traffic **succeeded**; **resume** was fine; **target `DROP COLUMN tag`** + resume was fine; **all events** went through.

#### Why (from code)

- Debezium **`INSERT`** payloads after the source **`DROP COLUMN`** no longer carry **`tag`** (field **omitted** or **null** in the event object — either way, voyager’s **`Event.Fields`** used in **`getPreparedInsertStmt`** / **`tgtdb/event.go`** only lists columns present). The generated **`INSERT`** on Yugabyte therefore **does not** include **`"tag"`** in the column list.
- On the **lagging** target, **`tag`** was **`NOT NULL`** **with** **`DEFAULT 'k_seed'`**. In PostgreSQL-compatible **`INSERT`**, columns **not** listed receive their **declared default** (or **`NULL`** if the column is nullable and has no default). So the row still **satisfies** **`NOT NULL`** without the stream ever sending **`'k_seed'`** explicitly — the **default expression** supplies it at insert time. This is **not** really “null events carrying the row”; it is **“missing column → server applies default.”**

#### Notes

- **Failure:** **None observed** in this run — behavior matched **J**, with the extra guarantee that **`NOT NULL`** on the target was satisfied by **`DEFAULT 'k_seed'`**.  
- **Contrast:** **`NOT NULL`** **without** **`DEFAULT`** on the lagging target is **Scenario L** (**`tag2`**).  
- **Workaround (observed):** **none** required; **target `DROP COLUMN`** for parity.

---

## Scenario L — **`DROP COLUMN`** (**`NOT NULL`**, **no `DEFAULT`**) on **source** only

Same shape as **K**, but the column left on the **target** during the skew phase is **`NOT NULL`** **and** has **no** **default** expression — so **`INSERT`**s that **omit** that column should hit a **not-null violation** (often **`SQLSTATE 23502`**) until the target is aligned. Uses **`tag2`** (not **`tag`**) so it can coexist with **K**’s cleanup state. Literals **`l_*`**.

### Prerequisite (both sides — build **`tag2`** as **`NOT NULL`** **without** **`DEFAULT`**)

Run the **same** block on **PostgreSQL** and **YugabyteDB** (order matters on non-empty tables):

```sql
ALTER TABLE public.subject_t ADD COLUMN tag2 TEXT;
UPDATE public.subject_t SET tag2 = 'l_init' WHERE tag2 IS NULL;
ALTER TABLE public.subject_t ALTER COLUMN tag2 SET NOT NULL;
ALTER TABLE public.subject_t ALTER COLUMN tag2 DROP DEFAULT;
```

The last line removes any implicit default from earlier steps; the column must stay **`NOT NULL`** with **no** **`DEFAULT`** in **`information_schema.columns`** before you run **step 1**. On an **empty** **`subject_t`**, a single-step **`ADD COLUMN tag2 TEXT NOT NULL`** (no default) is enough if your PostgreSQL / Yugabyte version allows it; otherwise use the multi-step block above. After you **`DROP COLUMN tag2`** on the source, **new** **`INSERT`**s from CDC **omit** **`tag2`**, while the target still requires a value for that column until **step 5**.

**1. DDL (source) only:**

```sql
ALTER TABLE public.subject_t DROP COLUMN tag2;
```

**Do not** drop **`tag2`** on Yugabyte until after steps **1–4**, or run **`ALTER TABLE public.subject_t DROP COLUMN IF EXISTS tag2`** on **both** sides afterward.

**2. Optional DML (source)** — no **`tag2`** column:

```sql
INSERT INTO public.control_t (name) VALUES ('l_after_drop_ctl');
INSERT INTO public.subject_t (name) VALUES ('l_after_drop_sub');
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('l_post_resume_ctl');
INSERT INTO public.subject_t (name) VALUES ('l_post_resume_sub');
```

**5. Target alignment (DDL on Yugabyte)** — match source:

```sql
ALTER TABLE public.subject_t DROP COLUMN tag2;
```

**6. Exit + resume** export and import again.

**7. DML (source) after target alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('l_post_align_ctl');
INSERT INTO public.subject_t (name) VALUES ('l_post_align_sub');
```

### Findings — L (`NOT NULL` **no default**, source ahead)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After source **`DROP COLUMN tag2`**, target still **`NOT NULL tag2`** **without** **`DEFAULT`** | No immediate failure | **`control_t`** OK (**metrics**: **1** imported); first **`subject_t`** batch → **`23502`** |
| **Resume** import (target unchanged) | _(unchanged)_ | **Same `23502`** on replay |
| After **target** **`DROP COLUMN tag2`** + resume | OK | **All events** through |

#### Observed

- No failure right after **DDL** on the source alone; **`control_t`** DML replicated (**`Total Imported events` / `Events Imported in this Run` = 1** — control path).
- First **`subject_t`** **`INSERT`** (after **`DROP COLUMN tag2`** on source) failed:  
  `error executing batch on channel 17: error executing batch: error preparing statements for events in batch (2:2) or when executing event with vsn(2): ERROR: null value in column "tag2" violates not-null constraint (SQLSTATE 23502)`  
  Same **`ExecuteBatch`** / first-`br.Exec` ambiguity wrapper as **G** / **H** (`tgtdb/yugabytedb.go`).
- **Resume** did **not** clear the error — same **`23502`** until the **target** schema matched the **source** (no **`tag2`**).
- **`ALTER TABLE public.subject_t DROP COLUMN tag2`** on **Yugabyte** + **resume** — **all events** imported.

#### Why (from code)

- Same **`INSERT`** construction as **K** (**`getPreparedInsertStmt`** / **`Event.Fields`** in **`tgtdb/event.go`**): after the source drop, CDC **`INSERT`**s **omit** **`tag2`**, so the prepared statement lists only columns present (e.g. **`id`**, **`name`**) and does **not** bind **`tag2`**.
- On **K**, the lagging target column had **`DEFAULT 'k_seed'`**, so the engine filled **`tag`** when it was missing from the **`INSERT`**. Here **`tag2`** was **`NOT NULL`** **with no default** — PostgreSQL/Yugabyte treats the missing column as an attempt to store **`NULL`** → **`23502 null value in column "tag2" violates not-null constraint`**.

#### Notes

- **Failure:** **Source-ahead `DROP COLUMN`** for **`NOT NULL`** **without** **`DEFAULT`** on the target — **`INSERT`** omits the column → **implicit null** → **`23502`**; **`resume`** alone **does not** fix (same stuck batch pattern as **G** / **H**).
- **Workaround (observed):** **`DROP COLUMN tag2`** on **Yugabyte** (align with source), **resume** — **catch-up OK**.

---

## Scenario M — **`DROP`** primary key on **source** only

Remove the **`PRIMARY KEY`** on **`public.subject_t`** on **PostgreSQL** while the table remains in the migration set. Default constraint name from **`control_subject_baseline_schema.sql`** is typically **`subject_t_pkey`** — confirm on your cluster:

```sql
SELECT conname
FROM pg_constraint
WHERE contype = 'p' AND conrelid = 'public.subject_t'::regclass;
```

**`control_t`** is the control ping. Literals **`m_*`**.

**1. DDL (source) only** (replace **`subject_t_pkey`** if your catalog uses a different name):

```sql
ALTER TABLE public.subject_t DROP CONSTRAINT subject_t_pkey;
```

**Do not** drop the **target** primary key until you have recorded behavior for steps **1–4**, unless you are explicitly testing “no PK on both sides” (expect a **worse** live-migration posture).

**2. Optional DML (source)** — rows still have **`id`** from **`BIGSERIAL`**:

```sql
INSERT INTO public.control_t (name) VALUES ('m_after_drop_pk_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('m_after_drop_pk_sub', NULL);
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('m_post_resume_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('m_post_resume_sub', NULL);
```

**5. Alignment — restore a primary key on the source** (so **`export data`** can pass **`reportUnsupportedTablesForLiveMigration`** again):

```sql
ALTER TABLE public.subject_t ADD PRIMARY KEY (id);
```

If **`subject_t`** had **duplicate `id` values** while the PK was absent, **`ADD PRIMARY KEY`** will **fail** — clean data first.

**6. Exit + resume** export and import again.

**7. DML (source) after alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('m_post_align_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('m_post_align_sub', NULL);
```

_(If **`note`** was removed by **J**, use **`INSERT INTO public.subject_t (name) VALUES ('…');`** instead.)_

### If you must drop **`public.subject_t`** from capture (flags blocked mid-flight)

Live migration **rejects** changing **`--table-list` / `--exclude-table-list`** after the initial run (guardrail: *“Changing the table list during live-migration is not allowed”* / missing tables vs initial list). To **stop capturing** **`subject_t`** when the **source** no longer has a **PK** (or you refuse to restore it), use the **same mid-migration surgery** as **Scenario I — step 5** (stopped processes, then in order):

1. **Backup:** **`meta.db`**, **`application.properties`**, **`name_registry.json`**, **`dataFileDescriptor.json`**, **`data/postdata.sql`**, **`data/schemas/`**, and **`data/queue/`** (as needed for **§5.4a** / **§5.4b**).
2. **`meta.db` → `json_objects` → `migration_status`:** remove **`public.subject_t`** from **`TableListExportedFromSource`** and **`SourceExportedTableListWithLeafPartitions`**; strip **`subject_t`** keys from **`SourceColumnToSequenceMapping`** (or **`null`** the whole map — see **I §5.2a**); clean **`SourceRenameTablesMap`** if present.
3. **`meta.db` → `json_objects` → `import_data_status`:** remove **`subject_t`** from **`tableToCDCPartitioningStrategyMap`** (and any other keys that still reference it).
4. **`metainfo/conf/application.properties`:** **`debezium.source.table.include.list=`** — **only** tables that still exist **and** are valid for live migration (e.g. **`schema_drift.public.control_t`**).
5. **`name_registry.json`:** remove **`subject_t`** entries (see **I §5.4**).
6. **`dataFileDescriptor.json`:** remove **`subject_t`** **`DataFileList`** rows (see **I §5.5**).
7. **`data/postdata.sql`:** remove **`setval`** lines for **`subject_t_id_seq`** (see **I §5.2b**).
8. **`data/schemas/`:** remove **`subject_t`** **`*_schema.json`** files under **`source_db_exporter`** (and **`target_db_exporter_fb` / `target_db_exporter_ff`** if present) — **required** if **`subject_t`** was removed from **`name_registry.json`** (see **I §5.4a**); otherwise **`import data`** fails at **streaming phase value converter** / **schema registry** with **`lookup subject_t from name registry`**.
9. **`data/queue/`:** strip **`subject_t`** lines from **`segment.*.ndjson`** and fix **`queue_segment_meta.size_committed`** (see **I §5.4b**) — **required** if **`subject_t`** events remain on disk but **`name_registry`** no longer lists **`subject_t`**; otherwise **`failed to unmarshal json event … lookup table public.subject_t`**. **Or** skip this path: **I §5.4b** recommends a **new migration** when queue surgery is not worth the operational risk.
10. **`metainfo/import_data_state/`:** remove the **`table::…subject_t…`** subtree.
11. **Optional:** **`DROP TABLE public.subject_t`** on **Yugabyte** for catalog parity (does **not** delete **`data/schemas`** or **queue** files); **optional** **`exported_events_stats_per_table`** cleanup.

**Simpler recovery (no surgery):** restore **`PRIMARY KEY`** on the **source** (**scenario step 5** above) so **`reportUnsupportedTablesForLiveMigration`** passes again.

### Findings — M (source PK removed)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After source **`DROP CONSTRAINT …_pkey`**, target still has **PK** | No immediate failure while processes keep running | **`control_t`** OK; first **`subject_t`** batch → **`42601`** |
| **`export data` resume** | **`ErrExit`**: **`Table names without a Primary key: [public.subject_t]`** — **`Currently voyager does not support live-migration for tables without a primary key`** (`reportUnsupportedTablesForLiveMigration`, **`exportData.go`**) | Same **`42601`** on **`subject_t`** |
| **`--exclude-table-list`** mid-run | **Rejected:** *“Changing the table list during live-migration is not allowed”* / *“Missing tables … compared to the initial list”* | — |
| **Recovery** | _(restore PK on **source** **or** surgery per subsection above)_ | _(expect **`42601`** until **`subject_t`** events are cleared or PK restored)_ |

#### Observed

- **No immediate failure** right after **`DROP`** of the **PK** on the source; **`control_t`** DML replicated.
- **`subject_t`** **`INSERT`** failed on import:  
  `error executing batch on channel 13: error executing batch: error preparing stmt: failed to prepare statement "\"public\".\"subject_t\"_src_c": ERROR: syntax error at or near ")" (SQLSTATE 42601)`
- **`export data` resume** failed with **`Table names without a Primary key: [public.subject_t]`** and **`Currently voyager does not support live-migration for tables without a primary key`** (same class of message as **Scenario D** / **`reportUnsupportedTablesForLiveMigration`**).
- **`import data` resume** still hit the **same `42601`** prepare failure for **`subject_t`**.
- Trying **`--exclude-table-list`** (or otherwise changing the table list) mid-migration was **rejected** — tool enforces the **initial** table list for live runs.
- **Further surgery** ( **`json_objects`**, **`postdata.sql`**, **`name_registry`**, **`dataFileDescriptor`**, **`application.properties`**) let **export** resume, but **`import data`** still failed:  
  `Failed to stream changes to yugabytedb: Failed to create streaming phase value converter: initializing schema registry: lookup subject_t from name registry: error lookup source and target names for table [subject_t]: lookup source table name [.subject_t]: table name not found: subject_t`  
  **Dropping `subject_t` on the target** did **not** fix it — the failure is from **on-disk Debezium schema JSON** under **`data/schemas/…`** still naming **`subject_t`** while **`name_registry.json`** no longer contains that table (**`SchemaRegistry.Init`**, **`schemareg/schemaRegistry.go`**).
- After **`rm …/data/schemas/source_db_exporter/subject_t_schema.json`**, **import** then failed on **queue** replay:  
  `failed to unmarshal json event {"…","table_name":"subject_t",…}: lookup table public.subject_t in name registry: … table name not found: subject_t`  
  i.e. **`segment.*.ndjson`** still contains **`subject_t`** events (**`Event.UnmarshalJSON`**, **`event.go`**) even though **`name_registry`** no longer has **`subject_t`**.

#### Why (from code)

- **`INSERT`** live apply uses **`getPreparedInsertStmt`** (`tgtdb/event.go`): for Yugabyte/PostgreSQL targets it appends **`ON CONFLICT (<key columns>) DO NOTHING`**. After the source **PK** is gone, **`event.Key`** for **`c`** events can be **empty** (no replica-identity key columns) → **`strings.Join(keyColumns, ",")`** is empty → SQL becomes **`ON CONFLICT () DO NOTHING`** → **`42601` syntax error at or near ")"`** when preparing **`"\"public\".\"subject_t\"_src_c"`**.
- **`reportUnsupportedTablesForLiveMigration`** (`exportData.go`) intersects the export table list with **`GetNonPKTables()`** on the **source**; if **`public.subject_t`** is listed and has **no PK**, voyager **`ErrExit`** with the message you saw.
- **`NewStreamingPhaseDebeziumValueConverter`** (`live_migration.go` → **`dbzm/valueConverter.go`**) builds **`SchemaRegistry`** instances that **`Init()`** by scanning **`<export-dir>/data/schemas/<exporter_role>/*_schema.json`** and **lookup** each table in **`name_registry.json`**. Stale **`subject_t`** schema files + removed registry entry → **lookup** error before any batch runs.
- **`json.Unmarshal`** into **`tgtdb.Event`** runs **`UnmarshalJSON`**, which **always** **`LookupTableName`** for non-cutover events — **queued** **`subject_t`** lines fail the same way if **`name_registry`** no longer lists **`subject_t`** (**`eventQueue.go`** + **`event.go`**).

#### Notes

- **Failure:** **Source PK removed** → **`subject_t`** live **`INSERT`** **invalid SQL** (**`42601`**); **export resume** blocked by **no-PK** guardrail; **CLI** cannot **shrink** the table list mid-migration. **Partial surgery** (MSR + registry + … **without** **`data/schemas/`** **or** **`data/queue/`** cleanup) → **export** may run while **import** fails (**schema registry** and/or **`failed to unmarshal json event … lookup subject_t`**). **Target `DROP TABLE`** removes neither **schemas** nor **queue** files.
- **Workaround (observed / expected):** **Preferred:** **`ALTER TABLE public.subject_t ADD PRIMARY KEY (id)`** on the **source** (scenario **step 5**), then **resume** — restores supported export. **Alternative:** full surgery including **I §5.4a** (**`data/schemas/`**) **and** **I §5.4b** (**`data/queue/`** + **`queue_segment_meta.size_committed`**), then **resume import** — or, per **I §5.4b**, **start a new migration** instead of hand-editing segments when that is acceptable.

---

## Scenario N — **`ENUM`**: new label on **source** only

Add a **new enum value** to **`public.subject_phase_t`** on **PostgreSQL** while **YugabyteDB** still has only the **original** labels, then **`INSERT`** rows that use the **new** label. **`public.control_t`** is the control ping. Literals prefixed **`n_`**.

This exercises **“enum values changed”** in the sense of **source catalog ahead**: the **CDC payload** carries a **string** label the **target type** does not yet accept. **Renaming** or **removing** enum labels mid-flight is a different (harsher) problem — not this playbook.

### Prerequisite (both sides — baseline has no **`phase`** / **`subject_phase_t`**)

Use a **`public.subject_t`** definition **without** leftover drift columns from **G**/**H**/**K**/**L** (re-run **`control_subject_baseline_schema.sql`** + **`control_subject_baseline_source_seeds.sql`** on the **source** and **schema only** on the **target** if unsure). Then run **the same** DDL on **PostgreSQL** and **YugabyteDB**:

```sql
CREATE TYPE public.subject_phase_t AS ENUM ('n_alpha', 'n_beta');

ALTER TABLE public.subject_t
	ADD COLUMN phase public.subject_phase_t NOT NULL DEFAULT 'n_alpha';
```

**1. DDL (source) only** — append a value to the enum (PostgreSQL):

```sql
ALTER TYPE public.subject_phase_t ADD VALUE 'n_gamma';
```

Use **`ADD VALUE IF NOT EXISTS`** on PostgreSQL **15+** if you need a re-runnable script. **Do not** run **`ALTER TYPE … ADD VALUE`** on **Yugabyte** until after steps **1–4** (before **target alignment**), or reset afterward (see intro).

**2. Optional DML (source)** — use the **new** label:

```sql
INSERT INTO public.control_t (name) VALUES ('n_after_src_enum_ctl');
INSERT INTO public.subject_t (name, note, phase) VALUES ('n_after_src_enum_sub', NULL, 'n_gamma');
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('n_post_resume_ctl');
INSERT INTO public.subject_t (name, note, phase) VALUES ('n_post_resume_sub', NULL, 'n_gamma');
```

**5. Target alignment (DDL on Yugabyte)** — add the **same** enum label (YSQL):

```sql
ALTER TYPE public.subject_phase_t ADD VALUE 'n_gamma';
```

If your **YugabyteDB** build rejects **`ADD VALUE`**, note the exact error in **Observed** and try the closest supported path (e.g. new type + **`ALTER COLUMN … TYPE`** migration — out of scope here).

**6. Exit + resume** export and import again.

**7. DML (source) after alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('n_post_align_ctl');
INSERT INTO public.subject_t (name, note, phase) VALUES ('n_post_align_sub', NULL, 'n_gamma');
```

### Cleanup (after **N**)

On **both** sides (column before type):

```sql
ALTER TABLE public.subject_t DROP COLUMN IF EXISTS phase;

DROP TYPE IF EXISTS public.subject_phase_t CASCADE;
```

### Findings — N (enum label drift, source ahead)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After source **`ALTER TYPE … ADD VALUE 'n_gamma'`**, target enum still **`n_alpha` / `n_beta` only** | **OK** — both **`control_t`** and **`subject_t`** events **exported** | **`control_t`** OK; first **`subject_t`** batch with **`phase` = `n_gamma`** → **`22P02`** **`invalid input value for enum subject_phase_t: "n_gamma"`** |
| **Restart / resume** import (target still missing **`n_gamma`**) | Still **OK** | **Same** error (stuck batch / queue replay) |
| After **`ALTER TYPE … ADD VALUE 'n_gamma'`** on **Yugabyte** + exit + resume | **OK** | **OK** — backlog + **new** **`n_*`** events applied |

#### Observed

- After **source-only** enum extension and **`INSERT`**s using **`n_gamma`**, **`control_t`** replicated; **`subject_t`** failed on live apply:  
  `error executing batch on channel 40: error executing batch: error preparing statements for events in batch (2:2) or when executing event with vsn(2): ERROR: invalid input value for enum subject_phase_t: "n_gamma" (SQLSTATE 22P02)`  
  (same **prepare vs execute** wrapper ambiguity as **G**/**H** on first batch **`Exec`**.)
- **Exporter** kept running and **exported** both **`control_t`** and **`subject_t`** events.
- **Restart / resume import** did **not** clear the failure — **same `22P02`** until the **target** enum listed **`n_gamma`**.
- **Target alignment:** **`ALTER TYPE public.subject_phase_t ADD VALUE 'n_gamma'`** on **Yugabyte**, then **exit + resume** **`export data`** and **`import data`** — both completed **OK**; **post-alignment** source **`INSERT`**s (**`n_post_align_*`**) also **went through**.

#### Why (from code)

- **Same theme as G / H:** live import binds **CDC field values** into the **target** column type. **`phase`** is an **`ENUM`** on both sides; the target’s **`pg_enum`** / YSQL catalog **does not list** **`n_gamma`** until **`ALTER TYPE … ADD VALUE`**. The server rejects the **string** **`n_gamma`** as **not a valid enum label** → **`SQLSTATE 22P02`** (compare **`ExecuteBatch`** / **`yugabytedb.go`** path in **Findings — G**).
- **`resume`** does **not** rewrite queued events; alignment must change the **target** type definition (or start over).

#### Notes

- **Failure (observed):** **Source-ahead enum extension** — **`INSERT`** events carry **`n_gamma`** while the **target** type lacks that label → **`22P02`**; **`resume`** alone **does not** clear it.
- **Workaround (observed):** **`ALTER TYPE public.subject_phase_t ADD VALUE 'n_gamma'`** on **Yugabyte**, then **exit + resume** both processes — **catch-up** and **new** events **OK** (same pattern as **G**’s target type alignment).

---

## Scenario O — **PK mismatch** (**`subject_t`**): **source** PK ≠ **target** / CDC **key**

**PostgreSQL** starts with **`public.subject_t`** using **`PRIMARY KEY (id)`** (default from **`control_subject_baseline_schema.sql`**). Then we change the **source PK** to **`PRIMARY KEY (name)`** only while **YugabyteDB** keeps the old **`PRIMARY KEY (id)`**. After the source PK changes, **Debezium** **`INSERT`** events carry **`event.Key`** keyed by **`name`**, so live import builds **`ON CONFLICT (name) DO NOTHING`** — but the target has **no** matching **`UNIQUE`** / **PK** on **`name`**, so the statement should fail. **`public.control_t`** is the control ping. Literals prefixed **`o_`**.

This is **not** the same as **Scenario M** (**no PK on the source** → empty **`event.Key`** → **`ON CONFLICT ()`**). Here the **source** still has a PK, but its key columns diverge from the target’s.

### Prerequisite

- **`public.subject_t`** on **both** sides: **`PRIMARY KEY (id)`**, **`name`** **unique** among rows (baseline **`sub_seed_*`** rows are fine). Remove **Scenario N** artifacts (**`phase`** / **`subject_phase_t`**) and other drift columns (**G**/**H**/**K**/**L**) if still present — re-baseline if unsure.
- **Run skew after streaming is up:** start **`export data` / `import data`** until **snapshot** for **`subject_t`** has **completed** and **CDC** is **healthy** with **matching `PK(id)`** on both sides — **then** apply **step 1** on **PostgreSQL** only. (If you instead change the **source** PK **before** the first **`subject_t`** snapshot, note a different failure mode in **Observed** — snapshot may interact with the new key definition.)

**1. DDL (source) only** — replace **`PK(id)`** with **`PK(name)`**:

```sql
ALTER TABLE public.subject_t DROP CONSTRAINT subject_t_pkey;

ALTER TABLE public.subject_t ADD PRIMARY KEY (name);
```

**Do not** run these **`ALTER`**s on **Yugabyte** until **alignment** (step **5**), or reset afterward via **Cleanup**.

**2. Optional DML (source)** — new row(s); **`id`** is assigned by the source sequence:

```sql
INSERT INTO public.control_t (name) VALUES ('o_after_tgt_pk_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('o_after_tgt_pk_sub', NULL);
```

**3. Exit + resume** `export data` and `import data` (both), if needed so the importer picks up new events.

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('o_post_resume_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('o_post_resume_sub', NULL);
```

**5. Target alignment (DDL on Yugabyte)** — switch the **target PK** to match the CDC key so **`ON CONFLICT (name)`** is valid:

```sql
ALTER TABLE public.subject_t DROP CONSTRAINT subject_t_pkey;

ALTER TABLE public.subject_t ADD PRIMARY KEY (name);
```

If this fails due to duplicate `name` (or `NULL name`), clean data and retry.

**6. Exit + resume** export and import again.

**7. DML (source) after alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('o_post_align_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('o_post_align_sub', NULL);
```

### Cleanup (if you skip alignment and only reset the lab)

- On **PostgreSQL**, restore **`PRIMARY KEY (id)`** (drop the **`name`** PK first).
- On **Yugabyte**, drop the **`UNIQUE (name)`** constraint if you added it.

### Findings — O (target uniqueness does not back `ON CONFLICT` columns from CDC)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After **source-only** **`PK(name)`** while **target** stays **`PK(id)`**, **`event.Key`** uses **`name`** | **Expected:** **OK** — **`subject_t`** **`INSERT`** events **exported** | **Expected:** **`control_t`** OK; **`subject_t`** live apply **fails** (prepare/execute) because **`ON CONFLICT (name)`** has **no** matching **unique / exclusion** constraint on **`name`** on the **target** |
| **Resume** import | **Expected:** **OK** | **Expected:** **same** class of failure until **target** has a **unique constraint** on **`name`** |
| After **target** **`UNIQUE (name)`** + resume | **Expected:** **OK** | **Expected:** **OK** — backlog + **new** events |

#### Observed

- No immediate crash right after the **source-only** PK DDL; **`control_t`** events continued to apply.
- First **`subject_t`** insert after the source PK changed failed on import with:
  `error executing batch on channel 76: error executing batch: error preparing statements for events in batch (2:2) or when executing event with vsn(2): ERROR: there is no unique or exclusion constraint matching the ON CONFLICT specification (SQLSTATE 42P10)`
- **Resume / restart import** did not clear it — the same **`42P10`** repeated (stuck batch replay).
- **Exporter** continued to run and **exported** both events.
- **Recovery that worked:** on **Yugabyte**, dropping and adding a new **primary key** (instead of adding a separate **`UNIQUE(name)`**) allowed import to **resume cleanly**, and **all queued + new** events went through.

#### Why (from code)

- For **YugabyteDB** / **PostgreSQL** targets, **`getPreparedInsertStmt`** appends **`ON CONFLICT (`** + **sorted `event.Key` columns** + **`) DO NOTHING`** — **`event.Key`** comes from **CDC** (aligned with **source** replica identity / **primary key**), **not** from **`TargetYugabyteDB.GetPrimaryKeyColumns`** on this path:

```349:374:yb-voyager/src/tgtdb/event.go
func (event *Event) getPreparedInsertStmt(tdb TargetDB, targetDBType string) (string, error) {
	// ...
	if targetDBType == POSTGRESQL || targetDBType == YUGABYTEDB {
		keyColumns := utils.GetMapKeysSorted(event.Key)
		// ...
		stmt = fmt.Sprintf("%s ON CONFLICT (%s) DO NOTHING", stmt, strings.Join(keyColumns, ","))
	}
	return stmt, nil
}
```

- If the **target** has **no** **`UNIQUE`** or **`PRIMARY KEY`** that includes those **`ON CONFLICT`** columns, the server rejects the statement (**PostgreSQL-style** message: *there is no unique or exclusion constraint matching the ON CONFLICT specification*, often surfaced through **`ExecuteBatch`** like **G**/**N**).

#### Notes

- **Failure (observed):** **PK mismatch** — CDC keys lead to **`ON CONFLICT`** on columns that have **no matching** **unique / exclusion** constraint on the target → **`42P10`**; **`resume`** alone does **not** rewrite events.
- **Workaround (observed):** On **Yugabyte**, make the **target** enforce uniqueness on the conflict columns — either **`UNIQUE(name)`** (minimal), or switching the **target PK** to include the conflict columns (what we did: drop + add a new PK) — then **resume import**.

### Final notes

**Scenarios A–S** (where **Findings → Observed** is filled): one line each — **Failure** · **Workaround** (what actually fixed it, or that **nothing** in the playbook did).

**A:** **Failure:** CDC referenced **`from_source`** but Yugabyte **`subject_t`** lacked it → import **panic**. **Workaround:** **`ALTER TABLE public.subject_t ADD COLUMN from_source TEXT`** on the **target**, then **resume import** — worked.  
**B:** **Failure:** After source had **`from_target`**, import hit **stale Debezium schema JSON** (column missing from snapshot). **Workaround:** **quit + resume import** again after export refreshed schema files — worked.  
**C:** **Failure:** **`side_t`** was never in export capture → **only `control_t`** replicated; **target alignment** (`CREATE TABLE` + resume) and **`--table-list … side_t`** did **not** fix it. **Workaround:** mid-migration surgery (edit `<export-dir>/metainfo/conf/application.properties`, `<export-dir>/metainfo/name_registry.json`, and patch `<export-dir>/metainfo/meta.db`) made new `side_t` changes stream, but **older `side_t` rows/events required backfill**.  
**D:** **Failure:** Early on, same “not in capture / `--table-list` rejected” pattern as **C**; after the same **mid-migration surgery** as **C** (minus sequences), **`export data` still exits** because **`public.nopk_side_t` has no PK** — `reportUnsupportedTablesForLiveMigration` in `exportData.go`. **Workaround:** **none** for live streaming without a PK; add a **PK** (or exclude the table from live migration and backfill separately).  
**E:** **Failure:** **`subject_t` → `subject_renamed_t`** left **MSR / export list** and **`dataFileDescriptor`** / **`import_data_state`** paths out of sync → **export reltuples** error, then **import registry** error, then **duplicate snapshot `COPY`** after descriptor fix. **Workaround:** patch **`meta.db`**, **`application.properties`**, **`name_registry.json`**, **`dataFileDescriptor.json`**, **target `RENAME`**; **`TRUNCATE`** + resume to clear snapshot collision; **one CDC change** still **lost** (offsets not rewound) — repair with **re-emit / manual row fix** if needed.  
**F:** **Failure:** Source **`RENAME COLUMN`** (`note` → `note_renamed`) while target kept **`note`** → live import **panic** in **`getPreparedInsertStmt`** / **`QuoteAttributeName`** (**`note_renamed` not found** among target columns **`[id note name]`**); **`control_t`** still worked. **Restart** alone **failed** again. **Workaround:** **`RENAME COLUMN`** on **Yugabyte** to match source, then resume — **OK**, **new events** OK.  
**G:** **Failure:** Source **`score`** **`INT` → `NUMERIC`** while target stayed **`INTEGER`** → first **`subject_t`** batch **`22P02`**: **`invalid input syntax for integer: "1.25"`**; message bundles **prepare vs execute** ambiguity (`ExecuteBatch` first `br.Exec`); **resume** repeated same error; **export** fine. **Workaround:** **`ALTER COLUMN score TYPE NUMERIC…`** on **Yugabyte**, resume — **all events** imported.  
**H:** **Failure:** Source **`NUMERIC` → `TEXT`** while target stayed **`NUMERIC`** — **numeric-looking** strings (**`'7.5'`**) **OK**; **`'not-a-number'`** → **`22P02`** **`invalid input syntax for type numeric: "not-a-number"`**; **resume** did **not** fix until **target `TEXT`**. **Workaround:** **`ALTER COLUMN score TYPE TEXT`** on **Yugabyte**, resume — **all events** through.  
**I:** **Failure:** Source **`DROP subject_t`** while MSR/Debezium/descriptors still captured it → export **`42P01`** on **`reltuples`**; **target `DROP`** did **not** fix; surgery on **`meta.db`** / files still left **`get sequence initial values`** until **`data/postdata.sql`** dropped **`setval`** for **`subject_t_id_seq`**. **Workaround:** full **step 5** playbook **including `postdata.sql`** (and correct **`SourceColumnToSequenceMapping`**) — **exporter** then **OK**.  
**J:** **Failure:** **None observed** — source dropped **nullable** **`note`** while target kept **`note`**; **`control_t`** / **`subject_t`** fine; **exit + resume** fine; **target `DROP COLUMN`** + resume fine. **Workaround:** **none** for this run; **target `DROP COLUMN`** for parity only.  
**K:** **Failure:** **None observed** — source dropped **`NOT NULL`** **`tag`** while target kept **`tag`** with **`DEFAULT 'k_seed'`**; **`control_t`** / **`subject_t`** fine; **resume** and **target `DROP COLUMN`** fine. **Why it worked:** **`INSERT`** from CDC **omitted** **`tag`**; Yugabyte applied the **column default** (not “null events” carrying **`k_seed`**). **Workaround:** **none**; skew **without** a target **default** is **Scenario L** (**`tag2`**).  
**L:** **Failure:** Source **`DROP COLUMN tag2`** while target kept **`NOT NULL tag2`** **without** **`DEFAULT`** → **`control_t`** OK; first **`subject_t`** batch **`23502`**: **`null value in column "tag2" violates not-null constraint`** (`ExecuteBatch` / **`yugabytedb.go`**); **resume** repeated **`23502`**. **Workaround:** **`DROP COLUMN tag2`** on **Yugabyte**, resume — **all events** through.  
**M:** **Failure:** Source **`DROP`** **`subject_t`** **PK** → **`control_t`** OK; **`subject_t`** import **`42601`** (**`ON CONFLICT ()`**); **export resume** **`reportUnsupportedTablesForLiveMigration`**; **`--exclude-table-list`** blocked; **partial surgery** → **import** **`schema registry: lookup subject_t`** until **`data/schemas/…`** removed (**I §5.4a**), then **`failed to unmarshal … lookup subject_t`** until **`data/queue/`** lines + **`queue_segment_meta`** fixed (**I §5.4b**). **Workaround:** **`ADD PRIMARY KEY (id)`** on **source** **or** full surgery **including** **`data/schemas/`** + **`data/queue/`** — **or** **new migration** (**I §5.4b**) if editing segments / **`size_committed`** is not worth it.  
**N:** **Failure:** Source **`ADD VALUE 'n_gamma'`** while target enum lacked **`n_gamma`** → **`control_t`** OK; **`subject_t`** **`22P02`** **`invalid input value for enum subject_phase_t: "n_gamma"`** (`ExecuteBatch` vsn(2)); **export** kept exporting both; **restart import** same error. **Workaround:** **`ALTER TYPE public.subject_phase_t ADD VALUE 'n_gamma'`** on **Yugabyte**, **exit + resume** export and import — backlog + **new** **`n_*`** events **OK**.  
**O:** **Failure:** **Source** **`PK(name)`** while **target** stays **`PK(id)`** → **`control_t`** OK; **`subject_t`** **`42P10`** (*no unique/exclusion constraint matching `ON CONFLICT`*) and **resume** repeats. **Workaround:** switch target PK to match the CDC key (e.g. **drop PK(id)** and add **`PRIMARY KEY (name)`**), then resume — worked.  
**P:** **Failure:** Source adds **new leaf partition** `public.part_t_p3` mid-run → exporter detects it and (if you continue) **ignores it**: `Detected new partition tables… These will not be considered during migration`. Rows routed to that partition are **not exported/imported**; target alignment does not help. **Workaround:** **new migration** with partition present up front (or risky mid-migration capture-set surgery) — see **Findings — P**.
**Q:** **Failure:** **None observed** — source-only default change replicated fine; exit+resume fine; target default alignment optional for parity. **Workaround:** align target default to avoid long-term functional divergence (see **Findings — Q**).  
**R:** **Failure:** **Target-only `NOT NULL`** on **`strict_col`** while **source** stays **nullable** → **`control_t`** OK; **`subject_t`** **`23502`** (**`null value in column "strict_col" violates not-null constraint`**); **restart import** repeats; **export** OK. **Workaround:** **`ALTER … DROP NOT NULL`** on **Yugabyte** (or add **`DEFAULT`**), resume — worked. (Overlaps **L**’s mechanism on the forward path; see **Findings — R**.)  
**S:** **Failure:** **Fallback** (YB→PG): **Yugabyte** relaxed **`fb_col`** (**`DROP NOT NULL`**) while **PostgreSQL** kept **`NOT NULL`** → **`subject_t`** import **`23502`** (**`null value in column "fb_col" … violates not-null constraint`**). **Workaround:** align **PostgreSQL** (**`DROP NOT NULL`** / **`DEFAULT`**) — worked (see **Findings — S**).

---

## Scenario P — **Add partition** on **source** only (target partition routing lag)

Create a **partitioned** table on both sides, then **add a new partition on the source only** and write rows that belong to that new partition. If the **target** is also partitioned but **does not** have the matching partition, inserts routed through the partitioned parent can fail with a **“no partition found for row”**-class error until the target partition exists.

We use a separate table **`public.part_t`** so the baseline `control_t` / `subject_t` scenarios stay intact. Literals prefixed **`p_`**.

### Prerequisite (both sides — baseline has no `part_t`)

Run on **PostgreSQL (source)** and **YugabyteDB (target)** *before starting the migration* (so the table is present in the initial table list). Keep the partition boundaries identical on both sides.

```sql
DROP TABLE IF EXISTS public.part_t CASCADE;

CREATE TABLE public.part_t (
	id BIGINT NOT NULL,
	day DATE NOT NULL,
	name TEXT NOT NULL,
	PRIMARY KEY (id, day)
) PARTITION BY RANGE (day);

CREATE TABLE public.part_t_p1 PARTITION OF public.part_t
	FOR VALUES FROM ('2026-01-01') TO ('2026-04-01');

CREATE TABLE public.part_t_p2 PARTITION OF public.part_t
	FOR VALUES FROM ('2026-04-01') TO ('2026-07-01');
```

**Source-only requirement (PostgreSQL):** voyager requires **`REPLICA IDENTITY FULL`** for tables in live migration. For partitioned tables it may check **partitions** too, so set it on the parent **and** each partition:

```sql
ALTER TABLE public.part_t REPLICA IDENTITY FULL;
ALTER TABLE public.part_t_p1 REPLICA IDENTITY FULL;
ALTER TABLE public.part_t_p2 REPLICA IDENTITY FULL;
```

Optional seed (source only):

```sql
INSERT INTO public.part_t (id, day, name) VALUES
	(1, '2026-01-10', 'p_seed_1'),
	(2, '2026-04-10', 'p_seed_2');
```

### Steps

**1. Start live migration** including **`public.part_t`** in the initial table list (along with `control_t` / `subject_t` as usual). Wait until snapshot import is done and CDC is flowing.

**2. DDL (source) only** — add a *new* partition that the target does not yet have:

```sql
CREATE TABLE public.part_t_p3 PARTITION OF public.part_t
	FOR VALUES FROM ('2026-07-01') TO ('2026-10-01');
```

**3. DML (source)** — write a row that belongs in the new partition:

```sql
INSERT INTO public.control_t (name) VALUES ('p_after_add_partition_ctl');
INSERT INTO public.part_t (id, day, name) VALUES (3, '2026-07-10', 'p_after_add_partition_row');
```

**4. If import fails**, record the exact error (expected: **no partition found for row** on the target for `part_t`).

**5. Target alignment (DDL on Yugabyte)** — add the same partition on the target:

```sql
CREATE TABLE public.part_t_p3 PARTITION OF public.part_t
	FOR VALUES FROM ('2026-07-01') TO ('2026-10-01');
```

**6. Exit + resume** `export data` and `import data`.

**7. DML (source) after alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('p_post_align_ctl');
INSERT INTO public.part_t (id, day, name) VALUES (4, '2026-07-11', 'p_post_align_row');
```

### Cleanup (after **P**)

On **both** sides:

```sql
DROP TABLE IF EXISTS public.part_t CASCADE;
```

### Findings — P (partition routing drift, source ahead)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After **source-only** `part_t_p3` is created and rows are inserted into that new partition | **Exporter warns** about **new leaf partitions** and will **not consider** them in the migration | **No apply failure** — the `part_t_p3` row is **not exported**, so it cannot be imported |
| Restart `export data` | Prompts: **Detected new partition tables… These will not be considered during migration** (leaf: `public.part_t_p3`) | `control_t` continues; `part_t_p3` rows remain **missing** |
| After creating `part_t_p3` on target | **Still ignored** (table list/capture does not expand mid-run) | Still **missing** — target alignment alone does not help if export ignores the partition |

#### Observed

- No immediate failure right after creating **`part_t_p3`** on the **source**; **`control_t`** events continued to replicate.
- The `part_t` row that belongs to **`part_t_p3`** was **neither exported nor imported**.
- On restarting **`export data`**, voyager printed and prompted:
  - `Detected new partition tables for the following partitioned tables. These will not be considered during migration: Root table: public.part_t, new leaf partitions: public.part_t_p3`
  - If you continue (**Yes**), the new partition is **ignored**.
- Creating the same partition on the **target** did **not** change this outcome (no events to apply).

#### Why (from code)

- Live migration’s **table list / capture set** is effectively **fixed** after the initial run. When the exporter detects **new leaf partitions**, it chooses to **not expand** the capture set mid-flight; instead it logs the warning/prompt above and proceeds **without** those partitions. Result: rows that land in the new leaf partition never enter the event queue, so import has nothing to apply.

#### Notes

- **Failure (observed):** adding a **new leaf partition** mid-live-migration results in **export ignoring that partition** (explicit warning). Rows routed to that partition are **not exported/imported**.
- **Workaround (observed):** mid-migration “force include leaf partition” surgery can make **new** events for the leaf partition flow, but **does not recover** rows inserted before the surgery:
  - Stop `export data` + `import data`; **backup** `meta.db` + `name_registry.json`.
  - **`name_registry.json`**: add `part_t_p3` under both `SourceDBTableNames.public[]` and `YBTableNames.public[]`.
  - **`meta.db` → `json_objects` → `migration_status`**:
    - Add `public.part_t_p3` to `SourceExportedTableListWithLeafPartitions`.
    - Add `SourceRenameTablesMap["public.part_t_p3"] = "public.part_t"`.
  - **PostgreSQL publication** (critical): add the new leaf table to the publication voyager uses (from MSR `PGPublicationName`):
    - `ALTER PUBLICATION <publication> ADD TABLE public.part_t_p3;`
  - Ensure **source** `REPLICA IDENTITY FULL` on `part_t_p3`, and create `part_t_p3` on the target (DDL parity).
  - Resume export/import; **new** `part_t_p3` events should stream.
- **Data loss note (observed):** any rows inserted into `part_t_p3` **before** the capture set/publication included it are **not backfilled** by CDC. To recover them you must **re-emit** (e.g. `UPDATE` the rows so they generate CDC) or do a **manual backfill** / **fresh migration**.

---

## Scenario Q — Default value change on **source** only

Change a column **`DEFAULT`** on the **source** while the **target** keeps the old default. This is usually **not** a live-CDC apply problem because Debezium events carry **explicit column values**; defaults are applied only when a column is **omitted** from an `INSERT` on that database.

We use a new column **`flag`** on `public.subject_t`. Literals prefixed **`q_`**.

### Prerequisite (both sides)

After a clean baseline, add the column on **both** sides with the same default:

```sql
ALTER TABLE public.subject_t
  ADD COLUMN flag TEXT NOT NULL DEFAULT 'q_old';
```

### Steps

**1. DDL (source) only** — change the default:

```sql
ALTER TABLE public.subject_t
  ALTER COLUMN flag SET DEFAULT 'q_new';
```

**2. DML (source)** — omit `flag` so the **source default** is used:

```sql
INSERT INTO public.control_t (name) VALUES ('q_after_src_default_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('q_after_src_default_sub', NULL);
```

**3. Exit + resume** `export data` and `import data`.

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('q_post_resume_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('q_post_resume_sub', NULL);
```

**5. Target alignment (optional, for parity)** — match the default on Yugabyte:

```sql
ALTER TABLE public.subject_t
  ALTER COLUMN flag SET DEFAULT 'q_new';
```

**6. Exit + resume** export and import again.

### Cleanup (after **Q**)

On **both** sides:

```sql
ALTER TABLE public.subject_t DROP COLUMN IF EXISTS flag;
```

### Findings — Q (default drift, source ahead)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After source-only default change, target default still old | OK | OK |
| Exit + resume export/import | OK | OK |
| Target default alignment | OK | OK (parity only) |

#### Observed

- No failures after changing the default on the **source** only.
- **`control_t`** and **`subject_t`** events both replicated successfully.
- **Exit + resume** worked; events continued to flow.
- Target alignment (updating the default on Yugabyte) also worked, but was **not required** to keep CDC apply healthy.

#### Why (from code)

- For inserts, voyager builds SQL from **CDC `event.Fields`**. The target default is not consulted if the event includes `flag` (which it should when the source applied a default during the insert).

#### Notes

- **Key takeaway (observed):** default drift is primarily a **schema parity / application semantics** issue (the default that matters is whichever database you will write to after cutover). It did **not** break live migration because the CDC stream carried explicit row values for `flag` when inserts omitted it on the source.

---

## Scenario R — **Target-only `NOT NULL`** (**`subject_t.strict_col`**)

Make **`public.subject_t.strict_col`** **`NOT NULL`** on **YugabyteDB** only while **PostgreSQL** keeps the column **nullable** and **without** a **`DEFAULT`**. Then **`INSERT`** on the **source** **without** **`strict_col`** so the CDC payload may **omit** the column (implicit **null** on apply). **`public.control_t`** is the control ping. Literals prefixed **`r_`**.

This differs from **Scenario L** (**source** dropped the column while the target kept **`NOT NULL`**): here both sides still have the column, but the **target** is **stricter**. **Practical note:** the failure mode is the same **`23502`** “implicit null vs **`NOT NULL`**” pattern as **L**; **R** exists mainly to document **target-first** nullability skew on the **forward** path. For the **more interesting** reverse-direction case (fallback **Yugabyte → PostgreSQL**), see **Scenario S**.

### Prerequisite (both sides)

Use a clean **`subject_t`** (re-baseline if **`flag`** from **Q**, **`tag`/`tag2`** from **K**/**L**, **`score`** from **G**/**H**, **`phase`** from **N**, etc. remain). Add a nullable column on **PostgreSQL** and **YugabyteDB**:

```sql
ALTER TABLE public.subject_t ADD COLUMN strict_col TEXT;
```

Backfill so **existing** rows are non-null before you tighten the **target** (snapshot rows may otherwise block **`SET NOT NULL`**):

```sql
UPDATE public.subject_t SET strict_col = 'r_init' WHERE strict_col IS NULL;
```

Run the same **`UPDATE`** on **Yugabyte** if snapshot/import left any **`NULL`**.

**1. DDL (Yugabyte / target) only** — enforce **`NOT NULL`** **without** adding a **`DEFAULT`**:

```sql
ALTER TABLE public.subject_t ALTER COLUMN strict_col SET NOT NULL;
```

**Do not** run this on **PostgreSQL** until **alignment** (step **5**), or reset via **Cleanup**.

**2. DML (source)** — omit **`strict_col`**:

```sql
INSERT INTO public.control_t (name) VALUES ('r_after_tgt_nn_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('r_after_tgt_nn_sub', NULL);
```

**3. Exit + resume** `export data` and `import data` (both).

**4. DML (source) after resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('r_post_resume_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('r_post_resume_sub', NULL);
```

**5. Target alignment (DDL on Yugabyte)** — relax or default the column:

```sql
ALTER TABLE public.subject_t ALTER COLUMN strict_col DROP NOT NULL;
```

(Alternative: **`SET DEFAULT 'r_init'`** then **`SET NOT NULL`** if you want to keep **`NOT NULL`** but allow omitted CDC fields.)

**6. Exit + resume** export and import again.

**7. DML (source) after alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('r_post_align_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('r_post_align_sub', NULL);
```

### Cleanup (after **R**)

On **both** sides:

```sql
ALTER TABLE public.subject_t DROP COLUMN IF EXISTS strict_col;
```

### Findings — R (target stricter nullability)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After **target-only** **`SET NOT NULL`** (no default), source inserts omit **`strict_col`** | OK | **`control_t`** OK; **`subject_t`** → **`23502`** |
| **Restart / resume** import | OK | **Same `23502`** (stuck batch replay) |
| After **`DROP NOT NULL`** on **Yugabyte** + resume | OK | OK |

#### Observed

- **`control_t`** events applied; first failing **`subject_t`** batch:  
  `error executing batch on channel 17: error executing batch: error preparing statements for events in batch (2:2) or when executing event with vsn(2): ERROR: null value in column "strict_col" violates not-null constraint (SQLSTATE 23502)`
- **Exporter** remained healthy.
- **Restart / resume import** did **not** clear the failure — **same `23502`** until **`ALTER TABLE public.subject_t ALTER COLUMN strict_col DROP NOT NULL`** on **Yugabyte**, then resume — **OK**.

#### Why (from code)

- Same family as **L**: live apply uses **`ExecuteBatch`** (`tgtdb/yugabytedb.go`); if the event does not supply a value for **`strict_col`**, the insert path behaves like **NULL** for that column, which violates **`NOT NULL`** on the target → **`SQLSTATE 23502`**.

#### Notes

- **Failure (observed):** **target stricter nullability** than what the CDC payload can satisfy for omitted columns → **`23502`**; **`resume`** repeats until the **target** is relaxed or given a compatible **`DEFAULT`**.
- **Workaround (observed):** **`ALTER COLUMN … DROP NOT NULL`** on **Yugabyte**, then **resume import**.

---

## Scenario S — **Fallback** (YB → PG): **target** relaxes **`NOT NULL`**, **source** stays strict

**Context:** After **cut over to Yugabyte** and starting **live migration fallback**, **`export data`** streams changes **from the target (Yugabyte)** and **`import data`** applies them **into PostgreSQL** (the “source” in product terms is now the **importer** for this direction). This scenario is about **nullability skew** on that **reverse** path.

This section does **not** spell every **`yb-voyager`** CLI flag for cutover/fallback — follow the product docs for your version — it only records the **DDL skew** and the **expected apply-time error**.

We add **`public.subject_t.fb_col`** on **both** sides, initially **`NOT NULL`** (with a one-time backfill so **`SET NOT NULL`** succeeds). Then relax **`fb_col`** on **Yugabyte only** to **nullable** while **PostgreSQL** keeps **`NOT NULL`**. **`INSERT`** on **Yugabyte** **without** **`fb_col`** (or with explicit **`NULL`**) should produce a change event that **cannot** be applied on **PostgreSQL**. Literals prefixed **`s_`**.

### Prerequisite (both sides)

Re-baseline or remove conflicting columns from other scenarios. Add **`fb_col`** and enforce **`NOT NULL`** on **PostgreSQL** and **Yugabyte**:

```sql
ALTER TABLE public.subject_t ADD COLUMN fb_col TEXT;

UPDATE public.subject_t SET fb_col = 's_init' WHERE fb_col IS NULL;

ALTER TABLE public.subject_t ALTER COLUMN fb_col SET NOT NULL;
```

### Lab setup (high level)

1. Run **live migration** through **cutover to Yugabyte** per docs.
2. Start **fallback** so streaming runs **Yugabyte → PostgreSQL** (target exporter / source-side importer roles as in your runbook).

### Steps (after fallback streaming is up)

**1. DDL (Yugabyte / target) only** — relax nullability:

```sql
ALTER TABLE public.subject_t ALTER COLUMN fb_col DROP NOT NULL;
```

**Do not** run this on **PostgreSQL** until **alignment** (step **4**), or reset via **Cleanup**.

**2. DML (Yugabyte / target)** — omit **`fb_col`** so the row stores **`NULL`** on the relaxed target:

```sql
INSERT INTO public.control_t (name) VALUES ('s_after_relax_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('s_after_relax_sub', NULL);
```

**3. Exit + resume** `export data` and `import data` as required for your fallback workflow.

**4. Source alignment (DDL on PostgreSQL)** — pick one:

```sql
-- Option A: match the relaxed target
ALTER TABLE public.subject_t ALTER COLUMN fb_col DROP NOT NULL;

-- Option B: keep NOT NULL but allow omitted CDC fields (if acceptable)
-- ALTER TABLE public.subject_t ALTER COLUMN fb_col SET DEFAULT 's_init';
```

**5. Exit + resume** export and import again.

**6. DML (Yugabyte) after alignment + resume:**

```sql
INSERT INTO public.control_t (name) VALUES ('s_post_align_ctl');
INSERT INTO public.subject_t (name, note) VALUES ('s_post_align_sub', NULL);
```

### Cleanup (after **S**)

On **both** sides:

```sql
ALTER TABLE public.subject_t DROP COLUMN IF EXISTS fb_col;
```

### Findings — S (fallback: source stricter nullability)

#### At a glance

| When | Export | Import |
|------|--------|--------|
| After **target-only** **`DROP NOT NULL`**, target inserts omit / null **`fb_col`** | OK | **`control_t`** OK; **`subject_t`** → **`23502`** on **PostgreSQL** apply |
| **Resume** import | OK | **Same `23502`** until **PostgreSQL** aligned |
| After **`DROP NOT NULL`** on **PostgreSQL** + resume | OK | OK |

#### Observed

- **`subject_t`** apply failed on **PostgreSQL** (fallback import path):  
  `error executing batch on channel 13: error executing batch: error preparing statements for events in batch (3:3) or when executing event with vsn(3): ERROR: null value in column "fb_col" of relation "subject_t" violates not-null constraint (SQLSTATE 23502)`
- **Post source alignment** (relax **`NOT NULL`** / add **`DEFAULT`** on **PostgreSQL** per scenario step **4**) — everything went through fine afterward.

#### Why (from code)

- Same **`23502`** family as **L** / **R**, but the **importer catalog** is now **PostgreSQL** while events originate from **Yugabyte**.

#### Notes

- **Failure (observed):** **PostgreSQL (importer)** stayed **`NOT NULL`** on **`fb_col`** while **Yugabyte** events carried **`NULL`** / omitted **`fb_col`** → **`23502`**.
- **Workaround (observed):** **source-side** schema alignment (scenario **step 4**), then **resume** — catch-up OK.

---

## Scenario T — **`DETACH PARTITION`** on **source** only (leaf becomes standalone table)

**Context:** Scenario **P** covered **adding** a new leaf partition mid-run. **T** is the mirror case: **detaching** an existing leaf partition on the **source** while the **target** still has it attached. Once a migration is running, voyager captures each leaf partition individually — Debezium's **`table.include.list`** names each leaf (e.g. **`public.part_t_p2`**), and voyager's **`SourceRenameTablesMap`** maps the leaf name back to the **root** (`public.part_t_p2 → public.part_t`) so target apply routes through the parent. After **`DETACH PARTITION`** on the **source**, the leaf is **no longer a child** of the root on the source — but voyager's capture set, publication, and rename map are unchanged. Events from the now-standalone leaf continue to stream, still rewritten to the root on the target. The **semantics** of that rename are broken.

We reuse **`public.part_t`** from scenario **P**. Literals prefixed **`t_`**.

### Prerequisite (both sides — baseline: `part_t` with `p1`, `p2`)

If scenario **P** was already run and cleaned up, re-create `part_t` fresh on both sides **before** starting the migration (leaf partitions must be in the initial capture set):

```sql
DROP TABLE IF EXISTS public.part_t CASCADE;

CREATE TABLE public.part_t (
	id BIGINT NOT NULL,
	day DATE NOT NULL,
	name TEXT NOT NULL,
	PRIMARY KEY (id, day)
) PARTITION BY RANGE (day);

CREATE TABLE public.part_t_p1 PARTITION OF public.part_t
	FOR VALUES FROM ('2026-01-01') TO ('2026-04-01');

CREATE TABLE public.part_t_p2 PARTITION OF public.part_t
	FOR VALUES FROM ('2026-04-01') TO ('2026-07-01');
```

**Source-only (PostgreSQL):** voyager requires **`REPLICA IDENTITY FULL`** on the parent **and** each leaf:

```sql
ALTER TABLE public.part_t REPLICA IDENTITY FULL;
ALTER TABLE public.part_t_p1 REPLICA IDENTITY FULL;
ALTER TABLE public.part_t_p2 REPLICA IDENTITY FULL;
```

Optional seed (source only):

```sql
INSERT INTO public.part_t (id, day, name) VALUES
	(101, '2026-01-15', 't_seed_p1'),
	(102, '2026-04-15', 't_seed_p2');
```

### Steps

**1. Start live migration** including **`public.part_t`** in the initial table list. Wait until snapshot import is done and CDC is flowing. Confirm a row routed through the root reaches the target correctly:

```sql
-- source
INSERT INTO public.part_t (id, day, name) VALUES (103, '2026-04-20', 't_pre_detach_row');
```

**2. DDL (source) only** — detach the p2 leaf:

```sql
ALTER TABLE public.part_t DETACH PARTITION public.part_t_p2;
```

After this, on the **source**:
- `public.part_t_p2` is a **standalone** table. It still exists, still holds its rows, still has `REPLICA IDENTITY FULL`, and is **still in the publication** voyager created.
- `public.part_t` no longer has a partition for the `2026-04-01` → `2026-07-01` range — inserts through the parent for that range will fail with `no partition of relation "part_t" found for row`.

**3. DML (source) — probe A**: insert directly into the now-standalone **`part_t_p2`**. Debezium will still emit a change event (table is in `table.include.list`); voyager will still rename it to `public.part_t` on apply:

```sql
INSERT INTO public.control_t (name) VALUES ('t_after_detach_ctl');
INSERT INTO public.part_t_p2 (id, day, name) VALUES (104, '2026-04-25', 't_after_detach_direct_leaf');
```

**4. DML (source) — probe B**: try to route through the root for the old p2 range (expected to fail **on source** because p2 is no longer attached):

```sql
-- expected to fail on PostgreSQL with:
--   ERROR: no partition of relation "part_t" found for row
INSERT INTO public.part_t (id, day, name) VALUES (105, '2026-05-05', 't_after_detach_via_root');
```

**5. Exit + resume** `export data` and `import data`. Record whether voyager emits any warning about the leaf no longer being attached on the source (analogous to scenario **P**'s "Detected new partition tables…" prompt, but on the **drop** side).

**6. DML (source) — probe C**: insert more rows into standalone **`part_t_p2`** after the resume and check where they land on the target:

```sql
INSERT INTO public.control_t (name) VALUES ('t_post_resume_ctl');
INSERT INTO public.part_t_p2 (id, day, name) VALUES (106, '2026-04-28', 't_post_resume_direct_leaf');
```

**7. On target — observe**:
- Does the row appear in `public.part_t` (root → routed into target's still-attached `part_t_p2`)? Or does it land nowhere?
- Query both sides:

```sql
-- on both source and target
SELECT 'source'   AS site, id, day, name FROM public.part_t_p2 ORDER BY id;
SELECT 'combined' AS site, id, day, name FROM public.part_t   ORDER BY id;
```

**8. Target alignment attempt (DDL on Yugabyte)** — also detach p2 on the target so the two sides match structurally:

```sql
ALTER TABLE public.part_t DETACH PARTITION public.part_t_p2;
```

After target-side detach, CDC events for the leaf are still rewritten to the root on apply by `SourceRenameTablesMap`. On the target, the root no longer has p2 attached either — record whether apply fails (expected: `no partition of relation "part_t" found for row` on the **target** now, **`23514`**-class or partition routing error).

**9. Exit + resume** and record the behavior. Note that mid-migration "remove leaf from capture set" surgery is the mirror of **P**'s "add leaf" surgery — neither is documented for end-users.

### Cleanup (after **T**)

Re-attach `p2` on both sides (or drop and re-baseline `part_t`):

```sql
-- on each side that was detached
ALTER TABLE public.part_t ATTACH PARTITION public.part_t_p2
	FOR VALUES FROM ('2026-04-01') TO ('2026-07-01');

-- or nuke and restart:
DROP TABLE IF EXISTS public.part_t CASCADE;
```

### Findings — T (detach partition, source ahead)

> **Status: not yet run — fill `Observed` from an actual lab run before trusting `Why` / `Notes` below.** `Why` and `Notes` are **expectations** based on code paths (`SourceRenameTablesMap`, `SourceExportedTableListWithLeafPartitions`, Debezium `table.include.list`) and on the behavior recorded in scenario **P**.

#### At a glance (expected)

| When | Export | Import |
|------|--------|--------|
| After **source-only** `DETACH` of `part_t_p2` | Debezium still streams from standalone `part_t_p2` (unchanged include-list + publication) | Events get renamed to `part_t` by `SourceRenameTablesMap` → target routes through still-attached `part_t_p2` → **data lands in target `part_t_p2` even though source treats it as standalone** (silent semantic divergence) |
| Source insert through **root** for p2 range | Fails on **source** (`no partition of relation "part_t" found for row`) → no event generated | Nothing to apply |
| After **restart `export data`** | No current code path reports a **removed** leaf (mirror of the "new leaf" detector doesn't exist for detach). No warning expected. | Same as above |
| After **target** also `DETACH`s `part_t_p2` | Export still OK | Apply fails: root `part_t` on target no longer has a partition for the p2 range → `no partition of relation "part_t" found for row` / partition routing error |

#### Observed

- _Not yet observed — pending test run._

#### Why (from code)

- Voyager builds the capture set + rename map **once** at migration setup: `exportData.go` stores `SourceExportedTableListWithLeafPartitions` and `SourceRenameTablesMap` in MSR, and feeds the leaf names into Debezium's `table.include.list`. Publication is created with the **leaf tables** added individually.
- **`DETACH PARTITION`** on the source is a **pure catalog-level** change on PostgreSQL. It does **not** drop the table, does **not** remove it from the publication, does **not** change its `REPLICA IDENTITY`, and does **not** break logical replication.
- Result: Debezium keeps streaming from the (now-standalone) leaf, voyager keeps renaming events to the root, and the target (still partitioned) silently routes them correctly. **Functionally invisible** on the happy path, but the **source-side semantics diverged** the moment detach happened: the source no longer considers the leaf part of `part_t`, while the pipeline does.
- The "new leaf" detector at `detectAndReportNewLeafPartitionsOnPartitionedTables` in `exportData.go` has **no counterpart** for removed/detached leaves. So unlike scenario **P**, the user gets **no warning** on resume.

#### Notes (expected)

- **Failure (expected):** primarily **silent semantic drift** rather than an apply-time crash — data from a source-standalone table continues to flow into a target partition of the (unrelated on source) root. Crash only manifests when the target side also detaches, or when the source-side app switches to routing through the root (whose p2 range no longer exists on the source).
- **Workaround (expected, mirrors P):** mid-migration surgery to remove the leaf from capture — `ALTER PUBLICATION <pub> DROP TABLE public.part_t_p2;`, remove it from `SourceExportedTableListWithLeafPartitions`, remove its entry from `SourceRenameTablesMap`, and update `name_registry.json`. End-users **cannot realistically do this**.
- **Realistic workaround:** **restart the migration** with the desired partition structure. Or, if the detach was unintentional, **re-attach** `part_t_p2` on the source to restore semantics.
- **Relation to other scenarios:** this is P0 in the same sense as scenario **P** (add partition) — both break voyager's partition capture inventory in ways that the exporter cannot self-heal and the user cannot fix without metadata surgery or a restart.
