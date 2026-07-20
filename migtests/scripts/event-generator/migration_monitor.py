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

Usage
-----
    python3 migration_monitor.py --export-dir /path/to/export-dir \
        --exporter-role target_db_exporter_fb \
        --import-dsn "host=... port=... dbname=... user=... password=..." \
        --interval 5 --duration 3600 --out migration-throughput.csv

    python3 migration_monitor.py --selftest

Output CSV columns
-------------------
    epoch, t_seconds, exported_cum, export_evps, imported_cum, import_evps, lag
"""

import argparse
import csv
import os
import sys
import time
import sqlite3

try:
    import psycopg2
    import psycopg2.errorcodes
except ImportError:  # pragma: no cover - psycopg2 is required for --import-dsn
    psycopg2 = None


DEFAULT_EXPORTER_ROLE = "target_db_exporter_fb"
EXPORT_DB_SUBPATH = os.path.join("metainfo", "meta.db")


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


SLOT_LAG_QUERY = (
    "SELECT COALESCE(MAX(pg_wal_lsn_diff(pg_current_wal_lsn(), confirmed_flush_lsn)), 0) "
    "FROM pg_replication_slots"
)


def fetch_slot_lag_bytes(dsn):
    """Connect to the CDC-source DB (where the replication slot lives) and return
    how far the exporter's slot is behind the WAL, in bytes.

    - Connection failure -> None (skip this sample's slot field; one bad
      connection shouldn't kill the monitor).
    - No slot yet (empty pg_replication_slots) -> 0.0 (valid early-run state).
    - Other query errors -> None with a warning.
    """
    if psycopg2 is None:
        return None
    try:
        conn = psycopg2.connect(dsn)
    except Exception as e:
        print("[warn] slot-lag DB connection failed: %s" % e, file=sys.stderr)
        return None
    try:
        conn.autocommit = True
        cur = conn.cursor()
        try:
            cur.execute(SLOT_LAG_QUERY)
            row = cur.fetchone()
            return float(row[0]) if row and row[0] is not None else 0.0
        except psycopg2.Error as e:
            print("[warn] slot-lag query failed: %s" % e, file=sys.stderr)
            return None
        finally:
            cur.close()
    finally:
        conn.close()


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


# --------------------------------------------------------------------------
# Main monitor loop
# --------------------------------------------------------------------------

def run_monitor(export_dir, role, import_dsn, interval, duration, out_path, slot_dsn=None):
    db_path = os.path.join(export_dir, EXPORT_DB_SUBPATH)

    print("Monitoring Voyager CDC throughput:")
    print("  export metadb : %s (role=%s)" % (db_path, role))
    print("  import target : %s" % (import_dsn if import_dsn else "(not tracked)"))
    print("  slot-lag DB   : %s" % (slot_dsn if slot_dsn else "(not tracked)"))
    print("  interval      : %ss, duration: %ss" % (interval, duration))
    print("  output CSV    : %s" % out_path)

    f = open(out_path, "w", newline="")
    writer = csv.writer(f)
    writer.writerow(
        ["epoch", "t_seconds", "exported_cum", "export_evps", "imported_cum", "import_evps", "lag",
         "slot_lag_bytes"]
    )
    f.flush()

    start_wall = time.time()
    start_mono = time.monotonic()

    export_prev_cum = None
    export_prev_time = None
    import_prev_cum = None
    import_prev_time = None

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

            slot_lag_str = ""
            if slot_dsn:
                slot_lag = fetch_slot_lag_bytes(slot_dsn)
                if slot_lag is not None:
                    slot_lag_str = "%.0f" % slot_lag

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
                    slot_lag_str,
                ]
            )
            f.flush()

            print(
                "[t=%6.1fs] export_cum=%.0f export_evps=%.2f imported_cum=%s import_evps=%s lag=%s slot_lag_bytes=%s"
                % (
                    t_seconds,
                    export_cum,
                    export_evps,
                    imported_cum_str or "n/a",
                    import_evps_str or "n/a",
                    lag_str or "n/a",
                    slot_lag_str or "n/a",
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

        def test_slot_lag_absent_connection_returns_none(self):
            if psycopg2 is None:
                self.skipTest("psycopg2 not installed")
            bad_dsn = "host=127.0.0.1 port=1 dbname=nope connect_timeout=1"
            result = fetch_slot_lag_bytes(bad_dsn)
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
        "--slot-dsn",
        default=None,
        help="psycopg2 DSN for the CDC-source DB (where the replication slot lives); "
        "when given, each sample also records how many bytes the exporter's slot is "
        "behind the WAL (pg_wal_lsn_diff vs confirmed_flush_lsn). "
        "If omitted, slot lag is not tracked.",
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
        slot_dsn=args.slot_dsn,
    )


if __name__ == "__main__":
    main()
