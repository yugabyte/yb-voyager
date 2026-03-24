# Iteration Feature -- Issues Report

## Previously Raised (3)

1. **Iteration cutover number + log in fallback case** -- e.g., logs should say "starting fallback for iteration 3"
2. **Warning should be added if cutover-to-source with restart runs before cutover-to-target**
3. **Resumption during cutover -- continuity logs needed**

---

## Found During Testing (2)

### Issue 4: `get data-migration-report` suggests wrong flag name for iteration details

**Severity:** Medium (UX)
**Found by:** Manual testing

**Description:**
After running `yb-voyager get data-migration-report --export-dir <parent-dir>`, the output shows:
```
To see the detailed report with all the iterations, run the command with the --all-iterations true flag.
```
But `--all-iterations` does not exist. Running it gives:
```
Error: unknown flag: --all-iterations
```
The actual flag is `--include-detailed-iterations-stats` (visible in `--help` output).

**Expected:** The message should say:
```
To see the detailed report with all the iterations, run the command with the --include-detailed-iterations-stats true flag.
```

---

### Issue 5: New table on source causes crash during iteration restart -- no guardrail

**Severity:** High (Bug)
**Found by:** Manual testing (TC18/TC30/TC38)
**Confirmed by:** Dev team

**Steps to reproduce:**
1. Start live migration with fallback on tables `employees`, `orders`
2. Complete iteration-2 (forward + fallback + restart)
3. Create a new table on source: `CREATE TABLE guardrail_test (id serial PRIMARY KEY, val text);`
4. Run cutover to target + cutover to source with `--restart-data-migration-source-target true`

**Actual behavior:**
- Voyager allows the restart, creates iteration-3, starts export
- Table list correctly filters to original 2 tables: `[public.employees public.orders]`
- Crashes with:
```
get sequence initial values: iterate over sequence last value map: lookup for sequence name
"public"."guardrail_test_id_seq": lookup source table name [."guardrail_test_id_seq"]:
table name not found: guardrail_test_id_seq
```
- Import to target gets stuck at `Initializing streaming phase...` because exporter is dead

**Expected behavior:**
Voyager should either block the restart with a clear validation error about schema change, or filter sequences to only migrated tables so the new table is fully ignored.

---

## Closed / Not Reproducible

### Issue 6: `import data` panics with nil pointer in SchemaRegistry during iteration-2 start

**Status:** CLOSED -- not a product bug. Caused by stale automation state.

**Root cause:** The crash only occurred when reusing a `test_db` and `export-dir` that had accumulated corrupted metadata from previous failed automation runs (multiple kill/restart cycles, partial iterations, leftover event generator data). When tested with a clean setup using the same 11-table complex schema (json, bit, array, tsvector, bytea, enum), iteration-2 import ran without any crash.

**Resolution:** Ensure automation always starts with a fresh DB and export dir. No product fix needed.
