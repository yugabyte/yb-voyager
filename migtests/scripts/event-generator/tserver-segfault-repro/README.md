# TServer SIGSEGV repro — expression-pushdown decode on batched key reads

This folder reproduces a **YugabyteDB tserver crash (SIGSEGV)** that shows up when a
large, many-table OLTP workload is driven at high, sustained write rates against a
cluster with **expression pushdown enabled** (the default). It is a self-contained
harness built on the event generator in the parent directory.

## Crash signature

Under sustained load the tserver dies with a segfault whose symbolized stack decodes
a bad varlena pointer while evaluating a pushdown filter on a `string`/`varchar`
column during a **batched `ybctid` read**:

```
BigEndian::Load64
  <- PgValue::Vardata
  <- DocPgExprExecutor (pushdown filter decode)
  <- ExecuteBatchKeys        (batched ybctid read path)
```

- The faulting pointer is a small, clearly-invalid address (e.g. `0x13f`), i.e. a
  mis-decoded varlena header, **not** a null deref.
- It is **not** caused by background auto-analyze / auto-ANALYZE.
- The cluster **self-recovers** once the load is stopped and the tserver restarts.

Observed on `2025.2.2.2-b11` and `2025.2.4.0-b0`.

## What's in this folder

| File | Purpose |
|------|---------|
| `schema.sql` | Anonymized 340-table OLTP schema (`table_<hash>` / `col_<hash>`). Mixed `varchar` / `text` / `numeric` / `timestamp` / `jsonb` / `boolean` columns; single- and composite-PK tables. |
| `generator-config.yaml` | The exact generator config used to trigger the crash — table-weighted skew, INSERT/UPDATE/DELETE mix, and a 10k events/sec spike schedule. Fill in the `connection:` block. |
| `README.md` | This file. |

The generator itself lives one directory up (`../parallel_generator.py` and helpers).

## Prerequisites

1. **A YugabyteDB cluster** you can afford to crash (multi-tserver recommended — the
   crash lands on whichever tserver serves the hot tablets).
2. **Expression pushdown must be ON** — this is the default, so *do not* disable it.
   In particular, make sure the cluster was **not** started with
   `yb_enable_expression_pushdown=false` (that flag is the mitigation; see below).
3. Python deps for the generator:
   ```
   pip install psycopg2-binary Faker PyYAML
   ```

## Steps to reproduce

From this directory:

```bash
# 1. Create the target database and load the schema
ysqlsh -h <YB_TSERVER_HOST> -p 5433 -U <YSQL_USER> -c "CREATE DATABASE test_db;"
ysqlsh -h <YB_TSERVER_HOST> -p 5433 -U <YSQL_USER> -d test_db -f schema.sql

# 2. Point the generator at the cluster
#    edit the connection: block at the top of generator-config.yaml
#      host / port / database (test_db) / user / password

# 3. Drive the load (from the event-generator directory one level up)
cd ..
python3 parallel_generator.py \
    --config tserver-segfault-repro/generator-config.yaml \
    --rate-csv tserver-segfault-repro/rate.csv
```

The config runs a ~1,500 events/sec baseline and bursts to **10,000 events/sec for
5 minutes every 30 minutes**. The generator seeds each table first, then applies the
weighted INSERT/UPDATE/DELETE mix.

### What to expect

- One of the tservers segfaults under sustained spike load once the hot tables have
  grown and batched key reads are hitting pushdown-filtered `string` columns. In our
  runs this appeared within the first few hours; it is load-dependent, not immediate.
- Check `<tserver>/logs/yb-tserver.FATAL` (or the core) for the stack above.
- `parallel_generator.py` will show write errors / reconnects while the tserver is
  down, then resume once it recovers.

To shorten a run, lower `parallel.run_seconds` in the config; keep the spike schedule
intact — the crash needs the sustained high-rate bursts, not a one-off.

## Mitigation (for confirming the root cause)

Restarting the cluster with expression pushdown **off** eliminates the crash:

```
yb_enable_expression_pushdown=false
```

(set via the tserver `ysql_pg_conf_csv` gflag). With pushdown off, the same schema +
config runs indefinitely without the segfault — which is what pins the fault to the
pushdown filter-decode path rather than the generator or the schema.
