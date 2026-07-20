#!/usr/bin/env python3
"""
migration_monitor.py

Standalone diagnostic (NOT part of the yb-voyager repo) that polls
YugabyteDB Voyager's own CDC counters during a live migration and writes a
CSV time series of export rate, import rate, and lag between them.

Data sources
------------
1. EXPORT side: the SQLite metadb at <export-dir>/metainfo/meta.db, table
   `exported_events_stats(run_id, exporter_role, timestamp, num_total,
   num_inserts, num_updates, num_deletes)`. Cumulative exported events for a
   role = SUM(num_total) over all its rows (rows accumulate for the whole
   run; `timestamp` is a unix-epoch floored to 10s buckets).

2. IMPORT side: in the import-target database, schema `ybvoyager_metadata`,
   table `ybvoyager_imported_event_count_by_table`, columns num_inserts /
   num_updates / num_deletes (cumulative, no timestamp). Cumulative imported
   events = SUM(num_inserts + num_updates + num_deletes) across rows.

3. PROMETHEUS side (optional, --prometheus-url): a YugabyteDB tserver's
   `:9000/prometheus-metrics` text endpoint. We read two metrics:
     - `cdcsdk_sent_lag_micros` (per tablet series) -> MAX across series,
       converted micros -> seconds. This is the time-based CDC replication
       lag and is the *correct* YB signal. (A byte-offset "slot lag" via
       pg_wal_lsn_diff/confirmed_flush_lsn is NOT valid on YugabyteDB - LSN
       there is not a byte offset and those functions are unsupported - so
       this monitor does not compute anything of that kind.)
     - `cdcsdk_change_event_count` (per tablet series, cumulative) -> SUM
       across series, then differenced against the previous poll to get a
       per-interval rate, as a cross-check against our own export/import
       rates.

Usage
-----
    python3 migration_monitor.py --export-dir /path/to/export-dir \
        --exporter-role target_db_exporter_fb \
        --import-dsn "host=... port=... dbname=... user=... password=..." \
        --prometheus-url http://10.9.101.88:9000/prometheus-metrics \
        --interval 5 --duration 3600 --out migration-throughput.csv

    python3 migration_monitor.py --selftest

Output CSV columns
-------------------
    epoch, t_seconds, exported_cum, export_evps, imported_cum, import_evps,
    lag, cdc_sent_lag_seconds, cdc_change_evps

The last two columns are blank whenever --prometheus-url is not given, or
whenever a given poll's fetch/parse fails (the monitor never crashes on a
bad Prometheus sample - it just leaves those two fields blank for that row).
"""

import argparse
import csv
import os
import sys
import time
import sqlite3
import urllib.request
import urllib.error

try:
    import psycopg2
    import psycopg2.errorcodes
except ImportError:  # pragma: no cover - psycopg2 is required for --import-dsn
    psycopg2 = None


DEFAULT_EXPORTER_ROLE = "target_db_exporter_fb"
EXPORT_DB_SUBPATH = os.path.join("metainfo", "meta.db")

# Prometheus metric names scraped from the tserver's :9000/prometheus-metrics
# endpoint. See module docstring, point 3.
CDC_SENT_LAG_METRIC = "cdcsdk_sent_lag_micros"
CDC_CHANGE_EVENT_COUNT_METRIC = "cdcsdk_change_event_count"


class ExportDBUnavailable(Exception):
    """Raised when the export metadb could not be read after retries."""


# --------------------------------------------------------------------------
# Pure / small, independently-testable query logic
# --------------------------------------------------------------------------

def compute_export_cumulative(cursor, role):
    """Given a sqlite3 cursor open on the export metadb, return the
    cumulative SUM(num_total) for the given exporter_role as a float.

    Returns 0.0 if the table doesn't exist yet (e.g. export hasn't started
    writing stats) or if there are no rows for the role.
    """
    try:
        cursor.execute(
            "SELECT COALESCE(SUM(num_total), 0) "
            "FROM exported_events_stats WHERE exporter_role = ?",
            (role,),
        )
    except sqlite3.OperationalError as e:
        if "no such table" in str(e).lower():
            return 0.0
        raise
    row = cursor.fetchone()
    if row is None or row[0] is None:
        return 0.0
    return float(row[0])


def compute_export_series(cursor, role):
    """Return a list of (timestamp, num_total_sum) tuples, one per 10s
    bucket, ordered by timestamp, for the given exporter_role. Useful for
    diagnostics/plotting; not required by the main CLI loop but kept here
    (and tested) per the documented data-source semantics.
    """
    try:
        cursor.execute(
            "SELECT timestamp, COALESCE(SUM(num_total), 0) "
            "FROM exported_events_stats WHERE exporter_role = ? "
            "GROUP BY timestamp ORDER BY timestamp",
            (role,),
        )
    except sqlite3.OperationalError as e:
        if "no such table" in str(e).lower():
            return []
        raise
    return [(int(ts), float(total)) for ts, total in cursor.fetchall()]


def compute_import_cumulative(cursor):
    """Given a psycopg2 cursor connected to the import-target DB, return the
    cumulative SUM(num_inserts+num_updates+num_deletes) across all rows of
    ybvoyager_metadata.ybvoyager_imported_event_count_by_table, as a float.
    Raises the underlying psycopg2 error if the schema/table is missing so
    the caller can decide how to treat that (fetch_import_cumulative treats
    it as "not started yet" -> 0.0).
    """
    cursor.execute(
        "SELECT COALESCE(SUM(num_inserts + num_updates + num_deletes), 0) "
        "FROM ybvoyager_metadata.ybvoyager_imported_event_count_by_table"
    )
    row = cursor.fetchone()
    if row is None or row[0] is None:
        return 0.0
    return float(row[0])


def compute_rate(prev_cum, cur_cum, elapsed_seconds):
    """Pure helper: events-per-second between two cumulative samples.
    Returns 0.0 if there's no valid previous sample or elapsed <= 0."""
    if prev_cum is None or elapsed_seconds is None or elapsed_seconds <= 0:
        return 0.0
    return (cur_cum - prev_cum) / elapsed_seconds


def parse_prometheus_metric(text, metric_name, agg="max"):
    """Pure parser: given the raw text body of a Prometheus text-exposition
    response, find every series for `metric_name` and aggregate their values.

    agg: 'max' -> return the maximum value across series (used for a gauge
         like a lag, where we want the worst tablet).
         'sum' -> return the sum of values across series (used for a
         cumulative counter, where we want the cluster-wide total).

    Handles:
      - '# HELP'/'# TYPE'/blank lines (skipped).
      - Series with labels, e.g. `metric{a="b",c="d"} 123 1699999999999`:
        the value is the first whitespace-separated token *after* the label
        block; a trailing unix-ms timestamp token (if present) is ignored.
        Labels are matched by splitting on the *last* '}' so label values
        that happen to contain spaces don't confuse the split.
      - Series with no labels, e.g. `metric 123`.
      - Other metric names present in the same text (ignored).

    Returns None if the metric has no matching series at all (e.g. it's
    absent from the response, or every value failed to parse as a float).
    """
    if agg not in ("max", "sum"):
        raise ValueError("agg must be 'max' or 'sum', got %r" % (agg,))

    values = []
    for raw_line in text.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue

        if "{" in line:
            name_part, _, rest = line.partition("{")
            if name_part != metric_name:
                continue
            if "}" not in rest:
                continue
            _, _, after_labels = rest.rpartition("}")
            tokens = after_labels.split()
            if not tokens:
                continue
            value_str = tokens[0]
        else:
            tokens = line.split()
            if len(tokens) < 2 or tokens[0] != metric_name:
                continue
            value_str = tokens[1]

        try:
            values.append(float(value_str))
        except ValueError:
            continue

    if not values:
        return None
    return max(values) if agg == "max" else sum(values)


# --------------------------------------------------------------------------
# I/O wrappers: retries, read-only access, graceful degradation
# --------------------------------------------------------------------------

def _connect_export_db_readonly(db_path, busy_timeout_ms=2000):
    uri = "file:%s?mode=ro" % db_path
    conn = sqlite3.connect(uri, uri=True, timeout=busy_timeout_ms / 1000.0)
    conn.execute("PRAGMA busy_timeout = %d" % busy_timeout_ms)
    return conn


def fetch_export_cumulative(db_path, role, retries=5, retry_delay=0.2):
    """Open the export metadb read-only and return the cumulative export
    count for `role`. Retries on transient 'database is locked' errors
    (the migration process writes this file concurrently). Raises
    ExportDBUnavailable if it still can't be read after `retries` attempts
    (e.g. the file doesn't exist yet) - the caller should treat that as
    "skip this sample" rather than crashing.
    """
    last_exc = None
    for _ in range(retries):
        try:
            conn = _connect_export_db_readonly(db_path)
            try:
                return compute_export_cumulative(conn.cursor(), role)
            finally:
                conn.close()
        except sqlite3.OperationalError as e:
            last_exc = e
            if "locked" in str(e).lower():
                time.sleep(retry_delay)
                continue
            # e.g. "unable to open database file" - db not created yet.
            time.sleep(retry_delay)
            continue
        except sqlite3.Error as e:
            last_exc = e
            time.sleep(retry_delay)
    raise ExportDBUnavailable(
        "could not read %s after %d attempts: %s" % (db_path, retries, last_exc)
    )


def fetch_import_cumulative(dsn):
    """Connect to the import-target DB via psycopg2 and return the
    cumulative imported-event count.

    - Connection failure -> returns None (caller should skip this sample's
      import fields; one bad connection shouldn't kill the monitor).
    - Schema/table not existing yet -> returns 0.0 (valid, early-run state).
    - Other query errors -> returns 0.0 and prints a warning.
    """
    if psycopg2 is None:
        print("[warn] psycopg2 not available; import-side tracking disabled", file=sys.stderr)
        return None

    try:
        conn = psycopg2.connect(dsn)
    except Exception as e:
        print("[warn] import DB connection failed: %s" % e, file=sys.stderr)
        return None

    try:
        conn.autocommit = True
        cur = conn.cursor()
        try:
            return compute_import_cumulative(cur)
        except psycopg2.Error as e:
            # Undefined table/schema (or any other query-time error) is
            # treated as "importer hasn't created its bookkeeping yet".
            try:
                conn.rollback()
            except Exception:
                pass
            code = getattr(e, "pgcode", None)
            undefined_codes = {
                psycopg2.errorcodes.UNDEFINED_TABLE,
                psycopg2.errorcodes.INVALID_SCHEMA_NAME,
            }
            if code not in undefined_codes:
                print("[warn] import query failed: %s" % e, file=sys.stderr)
            return 0.0
        finally:
            cur.close()
    finally:
        conn.close()


def fetch_prometheus_text(url, timeout=5):
    """Fetch the raw text body of a Prometheus metrics endpoint using only
    stdlib urllib. Returns None (and prints a one-line warning) on any
    connection/HTTP/decode failure - never raises, so a flaky tserver never
    takes the monitor down.
    """
    try:
        with urllib.request.urlopen(url, timeout=timeout) as resp:
            return resp.read().decode("utf-8", errors="replace")
    except (urllib.error.URLError, OSError, ValueError) as e:
        print("[warn] prometheus fetch failed (%s): %s" % (url, e), file=sys.stderr)
        return None


def fetch_cdc_sent_lag_seconds(url, timeout=5):
    """Fetch the endpoint and return MAX(cdcsdk_sent_lag_micros) across all
    tablet series, converted to seconds. Returns None on fetch failure or if
    the metric is absent (degrade gracefully - never raises).
    """
    text = fetch_prometheus_text(url, timeout=timeout)
    if text is None:
        return None
    lag_micros = parse_prometheus_metric(text, CDC_SENT_LAG_METRIC, agg="max")
    if lag_micros is None:
        return None
    return lag_micros / 1_000_000.0


def fetch_cdc_change_event_count(url, timeout=5):
    """Fetch the endpoint and return SUM(cdcsdk_change_event_count) across
    all tablet series (a cumulative counter; the caller differences it
    against a previous sample to get a rate). Returns None on fetch failure
    or if the metric is absent.
    """
    text = fetch_prometheus_text(url, timeout=timeout)
    if text is None:
        return None
    return parse_prometheus_metric(text, CDC_CHANGE_EVENT_COUNT_METRIC, agg="sum")


# --------------------------------------------------------------------------
# Main monitor loop
# --------------------------------------------------------------------------

def run_monitor(export_dir, role, import_dsn, interval, duration, out_path, prometheus_url=None):
    db_path = os.path.join(export_dir, EXPORT_DB_SUBPATH)

    print("Monitoring Voyager CDC throughput:")
    print("  export metadb : %s (role=%s)" % (db_path, role))
    print("  import target : %s" % (import_dsn if import_dsn else "(not tracked)"))
    print("  prometheus    : %s" % (prometheus_url if prometheus_url else "(not tracked)"))
    print("  interval      : %ss, duration: %ss" % (interval, duration))
    print("  output CSV    : %s" % out_path)

    f = open(out_path, "w", newline="")
    writer = csv.writer(f)
    writer.writerow(
        ["epoch", "t_seconds", "exported_cum", "export_evps", "imported_cum", "import_evps", "lag",
         "cdc_sent_lag_seconds", "cdc_change_evps"]
    )
    f.flush()

    start_wall = time.time()
    start_mono = time.monotonic()

    export_prev_cum = None
    export_prev_time = None
    import_prev_cum = None
    import_prev_time = None
    cdc_change_prev_cum = None
    cdc_change_prev_time = None

    try:
        next_tick = start_mono
        while True:
            now_mono = time.monotonic()
            if duration and (now_mono - start_mono) >= duration:
                break

            now_wall = time.time()

            try:
                export_cum = fetch_export_cumulative(db_path, role)
                export_evps = compute_rate(
                    export_prev_cum, export_cum,
                    (now_wall - export_prev_time) if export_prev_time else None,
                )
                export_prev_cum = export_cum
                export_prev_time = now_wall
            except ExportDBUnavailable as e:
                print("[warn] %s" % e, file=sys.stderr)
                export_cum = export_prev_cum if export_prev_cum is not None else 0.0
                export_evps = 0.0

            imported_cum_str = ""
            import_evps_str = ""
            lag_str = ""
            if import_dsn:
                import_cum = fetch_import_cumulative(import_dsn)
                if import_cum is not None:
                    import_evps = compute_rate(
                        import_prev_cum, import_cum,
                        (now_wall - import_prev_time) if import_prev_time else None,
                    )
                    import_prev_cum = import_cum
                    import_prev_time = now_wall
                    imported_cum_str = "%.2f" % import_cum
                    import_evps_str = "%.4f" % import_evps
                    lag_str = "%.2f" % (export_cum - import_cum)
                # else: connection failed this round - leave import fields blank,
                # keep previous state so the next successful sample's delta is
                # computed over the correct elapsed time.

            cdc_sent_lag_str = ""
            cdc_change_evps_str = ""
            if prometheus_url:
                sent_lag_seconds = fetch_cdc_sent_lag_seconds(prometheus_url)
                if sent_lag_seconds is not None:
                    cdc_sent_lag_str = "%.4f" % sent_lag_seconds

                change_cum = fetch_cdc_change_event_count(prometheus_url)
                if change_cum is not None:
                    change_evps = compute_rate(
                        cdc_change_prev_cum, change_cum,
                        (now_wall - cdc_change_prev_time) if cdc_change_prev_time else None,
                    )
                    cdc_change_prev_cum = change_cum
                    cdc_change_prev_time = now_wall
                    cdc_change_evps_str = "%.4f" % change_evps
                # else: fetch/parse failed this round - leave blank, keep
                # previous cumulative/time so the next good sample's delta
                # is computed over the correct elapsed time.

            t_seconds = now_wall - start_wall
            writer.writerow(
                [
                    int(now_wall),
                    "%.2f" % t_seconds,
                    "%.2f" % export_cum,
                    "%.4f" % export_evps,
                    imported_cum_str,
                    import_evps_str,
                    lag_str,
                    cdc_sent_lag_str,
                    cdc_change_evps_str,
                ]
            )
            f.flush()

            print(
                "[t=%6.1fs] export_cum=%.0f export_evps=%.2f imported_cum=%s import_evps=%s lag=%s "
                "cdc_sent_lag_s=%s cdc_change_evps=%s"
                % (
                    t_seconds,
                    export_cum,
                    export_evps,
                    imported_cum_str or "n/a",
                    import_evps_str or "n/a",
                    lag_str or "n/a",
                    cdc_sent_lag_str or "n/a",
                    cdc_change_evps_str or "n/a",
                )
            )

            next_tick += interval
            sleep_time = next_tick - time.monotonic()
            if sleep_time > 0:
                time.sleep(sleep_time)
            else:
                next_tick = time.monotonic()
    except KeyboardInterrupt:
        print("\nInterrupted; stopping cleanly.")
    finally:
        f.close()
        print("Wrote %s" % out_path)


# --------------------------------------------------------------------------
# Self-test
# --------------------------------------------------------------------------

def run_selftest():
    import tempfile
    import unittest

    SAMPLE_PROMETHEUS_TEXT = """\
# HELP cdcsdk_sent_lag_micros CDC SDK sent lag in microseconds.
# TYPE cdcsdk_sent_lag_micros gauge
cdcsdk_sent_lag_micros{table_id="t1",stream_id="s1",metric_type="tablet"} 500000 1699999999000
cdcsdk_sent_lag_micros{table_id="t2",stream_id="s1",metric_type="tablet"} 1500000 1699999999000
cdcsdk_sent_lag_micros{table_id="t3",stream_id="s1",metric_type="tablet"} 250000
# HELP cdcsdk_change_event_count CDC SDK change event count.
# TYPE cdcsdk_change_event_count counter
cdcsdk_change_event_count{table_id="t1",stream_id="s1",metric_type="tablet"} 1000
cdcsdk_change_event_count{table_id="t2",stream_id="s1",metric_type="tablet"} 2500 1699999999000

cdcsdk_change_event_count{table_id="t3",stream_id="s1",metric_type="tablet"} 500
# some unrelated metric that should never match
rocksdb_seek_count{table_id="t1"} 99999
"""

    class MonitorSelfTest(unittest.TestCase):
        def setUp(self):
            fd, self.db_path = tempfile.mkstemp(suffix=".db")
            os.close(fd)
            conn = sqlite3.connect(self.db_path)
            conn.execute(
                "CREATE TABLE exported_events_stats ("
                " run_id TEXT, exporter_role TEXT, timestamp INTEGER,"
                " num_total INTEGER, num_inserts INTEGER, num_updates INTEGER,"
                " num_deletes INTEGER)"
            )
            rows = [
                ("run1", "target_db_exporter_fb", 1000, 100, 90, 5, 5),
                ("run1", "target_db_exporter_fb", 1010, 150, 130, 10, 10),
                ("run1", "target_db_exporter_fb", 1020, 200, 180, 10, 10),
                # a different role's rows must NOT be counted
                ("run1", "source_db_exporter", 1000, 9999, 0, 0, 0),
            ]
            conn.executemany(
                "INSERT INTO exported_events_stats VALUES (?, ?, ?, ?, ?, ?, ?)",
                rows,
            )
            conn.commit()
            conn.close()

            fd2, self.empty_db_path = tempfile.mkstemp(suffix=".db")
            os.close(fd2)
            sqlite3.connect(self.empty_db_path).close()

        def tearDown(self):
            os.remove(self.db_path)
            os.remove(self.empty_db_path)

        def test_export_cumulative_sums_only_matching_role(self):
            conn = sqlite3.connect(self.db_path)
            cum = compute_export_cumulative(conn.cursor(), "target_db_exporter_fb")
            conn.close()
            self.assertEqual(cum, 100.0 + 150.0 + 200.0)

        def test_export_series_grouped_by_bucket(self):
            conn = sqlite3.connect(self.db_path)
            series = compute_export_series(conn.cursor(), "target_db_exporter_fb")
            conn.close()
            self.assertEqual(series, [(1000, 100.0), (1010, 150.0), (1020, 200.0)])

        def test_export_cumulative_missing_table_returns_zero(self):
            conn = sqlite3.connect(self.empty_db_path)
            cum = compute_export_cumulative(conn.cursor(), "target_db_exporter_fb")
            conn.close()
            self.assertEqual(cum, 0.0)

        def test_fetch_export_cumulative_readonly_via_file(self):
            cum = fetch_export_cumulative(self.db_path, "target_db_exporter_fb")
            self.assertEqual(cum, 450.0)

        def test_fetch_export_cumulative_unknown_role_is_zero(self):
            cum = fetch_export_cumulative(self.db_path, "no_such_role")
            self.assertEqual(cum, 0.0)

        def test_compute_rate(self):
            self.assertEqual(compute_rate(100.0, 150.0, 10.0), 5.0)
            self.assertEqual(compute_rate(None, 150.0, 10.0), 0.0)
            self.assertEqual(compute_rate(100.0, 150.0, 0), 0.0)
            self.assertEqual(compute_rate(100.0, 150.0, None), 0.0)

        def test_end_to_end_rate_over_two_polls(self):
            # Simulate two polls 10s apart against growing cumulative counts,
            # mirroring what run_monitor does with export_prev_cum/time.
            conn = sqlite3.connect(self.db_path)
            cur = conn.cursor()
            cum1 = compute_export_cumulative(cur, "target_db_exporter_fb")  # 450
            # simulate more events arriving
            conn.execute(
                "INSERT INTO exported_events_stats VALUES (?, ?, ?, ?, ?, ?, ?)",
                ("run1", "target_db_exporter_fb", 1030, 50, 40, 5, 5),
            )
            conn.commit()
            cum2 = compute_export_cumulative(cur, "target_db_exporter_fb")  # 500
            conn.close()
            rate = compute_rate(cum1, cum2, 10.0)
            self.assertEqual(cum1, 450.0)
            self.assertEqual(cum2, 500.0)
            self.assertEqual(rate, 5.0)

        def test_import_side_absent_connection_returns_none(self):
            if psycopg2 is None:
                self.skipTest("psycopg2 not installed")
            # Fast-failing DSN: unroutable port with a short connect_timeout,
            # standing in for "the import DB is not reachable".
            bad_dsn = "host=127.0.0.1 port=1 dbname=nope connect_timeout=1"
            result = fetch_import_cumulative(bad_dsn)
            self.assertIsNone(result)

        # ---- Prometheus parser tests (pure, no network) ----

        def test_parse_prometheus_metric_max_agg(self):
            # cdcsdk_sent_lag_micros series: 500000, 1500000, 250000 -> max
            val = parse_prometheus_metric(SAMPLE_PROMETHEUS_TEXT, CDC_SENT_LAG_METRIC, agg="max")
            self.assertEqual(val, 1500000.0)

        def test_parse_prometheus_metric_sum_agg(self):
            # cdcsdk_change_event_count series: 1000, 2500, 500 -> sum
            val = parse_prometheus_metric(
                SAMPLE_PROMETHEUS_TEXT, CDC_CHANGE_EVENT_COUNT_METRIC, agg="sum"
            )
            self.assertEqual(val, 4000.0)

        def test_parse_prometheus_metric_missing_metric_returns_none(self):
            val = parse_prometheus_metric(SAMPLE_PROMETHEUS_TEXT, "no_such_metric", agg="max")
            self.assertIsNone(val)

        def test_parse_prometheus_metric_empty_text_returns_none(self):
            self.assertIsNone(parse_prometheus_metric("", "anything", agg="sum"))

        def test_parse_prometheus_metric_ignores_comments_and_blank_lines(self):
            text = "\n".join(
                [
                    "# HELP foo bar",
                    "# TYPE foo gauge",
                    "",
                    "foo 42",
                    "",
                ]
            )
            self.assertEqual(parse_prometheus_metric(text, "foo", agg="max"), 42.0)

        def test_parse_prometheus_metric_no_labels(self):
            text = "foo 10\nfoo 20\n"
            self.assertEqual(parse_prometheus_metric(text, "foo", agg="sum"), 30.0)

        def test_parse_prometheus_metric_invalid_agg_raises(self):
            with self.assertRaises(ValueError):
                parse_prometheus_metric(SAMPLE_PROMETHEUS_TEXT, CDC_SENT_LAG_METRIC, agg="avg")

        def test_fetch_cdc_sent_lag_seconds_converts_micros_to_seconds(self):
            # 1,500,000 micros (the max series) == 1.5 seconds. Patch by
            # module object (not by dotted "migration_monitor.xxx" name) so
            # this also works when the file is run directly as __main__.
            import unittest.mock as mock

            this_module = sys.modules[__name__]
            with mock.patch.object(
                this_module, "fetch_prometheus_text", return_value=SAMPLE_PROMETHEUS_TEXT
            ):
                val = fetch_cdc_sent_lag_seconds("http://ignored/prometheus-metrics")
            self.assertAlmostEqual(val, 1.5)

        def test_fetch_prometheus_text_bad_url_returns_none(self):
            # Unroutable port with a short timeout - no real network access,
            # connection is refused/times out almost immediately.
            result = fetch_prometheus_text("http://127.0.0.1:1/prometheus-metrics", timeout=1)
            self.assertIsNone(result)

        def test_fetch_cdc_sent_lag_seconds_bad_url_returns_none(self):
            result = fetch_cdc_sent_lag_seconds("http://127.0.0.1:1/prometheus-metrics", timeout=1)
            self.assertIsNone(result)

        def test_fetch_cdc_change_event_count_bad_url_returns_none(self):
            result = fetch_cdc_change_event_count("http://127.0.0.1:1/prometheus-metrics", timeout=1)
            self.assertIsNone(result)

    suite = unittest.TestLoader().loadTestsFromTestCase(MonitorSelfTest)
    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(suite)
    return result.wasSuccessful()


# --------------------------------------------------------------------------
# CLI
# --------------------------------------------------------------------------

def build_arg_parser():
    p = argparse.ArgumentParser(
        description="Monitor Voyager live-migration CDC export/import throughput and save a CSV time series."
    )
    p.add_argument("--export-dir", help="Voyager export-dir (contains metainfo/meta.db)")
    p.add_argument(
        "--exporter-role",
        default=DEFAULT_EXPORTER_ROLE,
        help="exporter_role to track in exported_events_stats (default: %(default)s)",
    )
    p.add_argument(
        "--import-dsn",
        default=None,
        help='psycopg2 DSN for the import-target DB, e.g. "host=... port=... dbname=... user=... password=...". '
        "If omitted, only export-side rates are tracked.",
    )
    p.add_argument(
        "--prometheus-url",
        default=None,
        help="YugabyteDB tserver Prometheus metrics endpoint, e.g. "
        "http://10.9.101.88:9000/prometheus-metrics. When given, each sample also records "
        "cdc_sent_lag_seconds (max cdcsdk_sent_lag_micros across tablets, in seconds - the "
        "time-based CDC lag signal) and cdc_change_evps (per-interval rate of "
        "cdcsdk_change_event_count, summed across tablets, as a cross-check against the "
        "export/import rates). If omitted, both columns are left blank.",
    )
    p.add_argument("--interval", type=int, default=5, help="poll interval in seconds (default: %(default)s)")
    p.add_argument("--duration", type=int, default=3600, help="total run duration in seconds (default: %(default)s)")
    p.add_argument("--out", default="migration-throughput.csv", help="output CSV path (default: %(default)s)")
    p.add_argument("--selftest", action="store_true", help="run the built-in self-test and exit")
    return p


def main():
    args = build_arg_parser().parse_args()

    if args.selftest:
        ok = run_selftest()
        sys.exit(0 if ok else 1)

    if not args.export_dir:
        print("error: --export-dir is required (unless --selftest)", file=sys.stderr)
        sys.exit(2)

    run_monitor(
        export_dir=args.export_dir,
        role=args.exporter_role,
        import_dsn=args.import_dsn,
        interval=args.interval,
        duration=args.duration,
        out_path=args.out,
        prometheus_url=args.prometheus_url,
    )


if __name__ == "__main__":
    main()
