#!/usr/bin/env python3
"""Unit tests for build_page.py.

Run with `python3 -m pytest build_page_test.py` or plain
`python3 build_page_test.py` - both work, no dependencies beyond the
standard library (pytest is used only if present).
"""
import csv
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import build_page as bp


# The full set of labels the legend in page_template.html defines. Any label
# `cell()` can produce that is not in this set is a page bug: a reader would
# see a pill with no definition anywhere on the page.
LEGEND_LABELS = {
    "Works", "Column dropped", "Column dropped, you were asked",
    "Wrong value", "Data lost", "Import stops", "Export crashes",
    "Target rejects type", "Source rejects value", "Column cannot exist",
    "Not reachable", "Not measured", "No result", "Not run",
}

BANNED_WORDS = ["wedged", "poison", "attributed", "carve-out"]


def assert_no_jargon(text):
    low = (text or "").lower()
    for w in BANNED_WORDS:
        assert w not in low, f"banned word {w!r} found in: {text!r}"


class Rule1NotReachableViaSkipOrLiveFailure(unittest.TestCase):
    """A FALL-BACK cell with NO measurement is "Not run" UNLESS its probe is
    named in the SKIP file, or its LIVE cell already failed - either one
    makes it "Not reachable" instead."""

    def test_no_measurement_no_skip_no_live_failure_is_not_run(self):
        label, css, note = bp.cell("fall_back", {"verdict": "NOT_TESTED"}, "WORKS", True,
                                    "SOME-001", set())
        self.assertEqual(label, "Not run")
        assert_no_jargon(note)

    def test_no_measurement_with_skip_marker_is_not_reachable(self):
        label, css, note = bp.cell("fall_back", {"verdict": "NOT_TESTED"}, "WORKS", True,
                                    "DOM-003", {"DOM-003"})
        self.assertEqual(label, "Not reachable")
        self.assertIn("nothing to fall back from", note)
        assert_no_jargon(note)

    def test_no_measurement_with_live_import_stops_is_not_reachable(self):
        label, css, note = bp.cell("fall_back", {"verdict": "NOT_TESTED"}, "STUCK", True,
                                    "REG-001", set())
        self.assertEqual(label, "Not reachable")
        self.assertIn("nothing to fall back from", note)

    def test_no_measurement_with_live_export_crashes_is_not_reachable(self):
        label, css, note = bp.cell("fall_back", {"verdict": "NOT_TESTED"}, "EXPORTER_CRASHES",
                                    True, "SOME-002", set())
        self.assertEqual(label, "Not reachable")

    def test_missing_mode_dict_entirely_behaves_the_same_as_not_tested(self):
        label, css, note = bp.cell("fall_back", None, "BLOCKS", True, "SOME-003", set())
        self.assertEqual(label, "Not reachable")

    def test_only_applies_to_fall_back_mode(self):
        # The gate is fall-back specific; a live cell with no measurement is
        # just "Not run" even if some other mode "failed".
        label, css, note = bp.cell("live", {"verdict": "NOT_TESTED"}, "STUCK", True,
                                    "REG-001", {"REG-001"})
        self.assertEqual(label, "Not run")


class Rule2ExcludedTold(unittest.TestCase):
    def test_label_and_tooltip(self):
        mode = {"verdict": "EXCLUDED_TOLD",
                "evidence": "column \"v\" absent; export printed the exclusion notice and "
                            "asked before continuing",
                "run_status": "OK"}
        label, css, note = bp.cell("fall_back", mode, "WORKS", True, "MISC-001", set())
        self.assertEqual(label, "Column dropped, you were asked")
        self.assertIn("voyager asked before continuing", note)
        self.assertIn("--yes", note)

    def test_label_is_in_legend(self):
        self.assertIn("Column dropped, you were asked", LEGEND_LABELS)


class Rule3ForwardLegFailed(unittest.TestCase):
    def test_forward_leg_failure_renders_not_reachable_with_forward_detail(self):
        mode = {"verdict": "STUCK",
                "evidence": "fall-back not reached: forward leg failed (SQLSTATE 22P02: "
                            "ERROR: invalid input syntax (SQLSTATE 22P02))",
                "run_status": "OK"}
        label, css, note = bp.cell("fall_back", mode, "STUCK", True, "X-001", set())
        self.assertEqual(label, "Not reachable")
        self.assertIn("nothing to fall back from", note)
        self.assertIn("forward leg's own failure", note)
        self.assertIn("22P02", note)

    def test_fall_forward_variant_also_matches(self):
        detail = bp.forward_leg_failure_detail(
            "fall-forward not reached: forward leg failed (cutover never completed)")
        self.assertEqual(detail, "cutover never completed")

    def test_no_match_returns_none(self):
        self.assertIsNone(bp.forward_leg_failure_detail("cutover to target did not complete"))


class Rule4QuietDropNullOnly(unittest.TestCase):
    def test_no_value_can_be_lost_gets_specific_tooltip(self):
        mode = {"verdict": "QUIET_DROP",
                "evidence": "column \"v\" absent from all 3 exported events for this table; "
                            "column never appears in the change stream; the type stores only "
                            "NULL so no value can be lost, but a non-NULL value could not be "
                            "tested [NULL-only: PG 17.8 refuses every literal]",
                "run_status": "OK"}
        label, css, note = bp.cell("live", mode, "WORKS", True, "IDXKEY-002", set())
        self.assertEqual(label, "Column dropped")
        self.assertIn("accepts only NULL", note)
        self.assertIn("nothing can be lost", note)
        self.assertNotIn("data loss", note.lower())
        self.assertNotIn("data lost", note.lower())

    def test_ordinary_quiet_drop_keeps_generic_text(self):
        mode = {"verdict": "QUIET_DROP",
                "evidence": "column absent from every change event",
                "run_status": "OK"}
        label, css, note = bp.cell("live", mode, "WORKS", True, "RANGE-001", set())
        self.assertEqual(label, "Column dropped")
        self.assertNotIn("accepts only NULL", note)


class Rule5WrongValueTooltip(unittest.TestCase):
    def test_forward_direction_uses_detail_span_not_flat_fields(self):
        mode = {
            "verdict": "SILENT_WRONG",
            "evidence": 'streaming source->target: [update-this-column] id=1 '
                        'source="other_cursor" destination="\\x6f746865725f637572736f72"',
            # Deliberately wrong/irrelevant flat fields - the tooltip must
            # ignore these and read the detail's own span instead.
            "source_value": "SHOULD NOT APPEAR",
            "target_value": "SHOULD NOT APPEAR EITHER",
            "run_status": "OK",
        }
        label, css, note = bp.cell("live", mode, "WORKS", True, "CAT-003", set())
        self.assertEqual(label, "Wrong value")
        self.assertIn("an update to this column", note)
        self.assertIn("row 1", note)
        self.assertIn('source "other_cursor"', note)
        self.assertNotIn("SHOULD NOT APPEAR", note)

    def test_reverse_direction_is_named_in_plain_language(self):
        mode = {
            "verdict": "SILENT_WRONG",
            "evidence": 'streaming target->source: [update-this-column] id=101 '
                        'source="other_cursor" destination="\\x6f746865725f637572736f72"; '
                        "queue scanned from the cutover mark only",
            "source_value": "SHOULD NOT APPEAR",
            "target_value": "SHOULD NOT APPEAR EITHER",
            "run_status": "OK",
        }
        label, css, note = bp.cell("fall_back", mode, "WORKS", True, "CAT-003", set())
        self.assertEqual(label, "Wrong value")
        self.assertIn("reverse direction (YugabyteDB back to PostgreSQL)", note)
        self.assertIn("row 101", note)

    def test_data_lost_with_no_comparable_value_does_not_invent_one(self):
        mode = {
            "verdict": "SILENT_LOSS",
            "evidence": 'column "v" absent from all 3 exported events for this table; '
                        "no exclusion warning in export stdout/stderr",
            "source_value": "SHOULD NOT APPEAR",
            "target_value": "SHOULD NOT APPEAR EITHER",
            "run_status": "OK",
        }
        label, css, note = bp.cell("live", mode, "WORKS", True, "CATSTAT-006", set())
        self.assertEqual(label, "Data lost")
        self.assertNotIn("SHOULD NOT APPEAR", note)


class Rule6ImportStopsTooltip(unittest.TestCase):
    def test_base_sentence_and_sqlstate_translation(self):
        mode = {"verdict": "STUCK",
                "evidence": "SQLSTATE 22P02: ERROR: invalid input syntax (SQLSTATE 22P02)",
                "import_error": "ERROR: invalid input syntax (SQLSTATE 22P02)",
                "run_status": "OK"}
        label, css, note = bp.cell("live", mode, "WORKS", True, "SYS-002", set())
        self.assertEqual(label, "Import stops")
        self.assertIn("The importer hit a SQL error on this type and exited.", note)
        self.assertIn("could not parse the value", note)
        self.assertIn("SQLSTATE 22P02", note)

    def test_long_hex_payload_is_shortened(self):
        long_hex = "a1b2c3d4e5f6" * 6  # 72 hex digits, well past the threshold
        mode = {"verdict": "STUCK",
                "evidence": f'ERROR: cannot accept value "\\x{long_hex}" (SQLSTATE 22P02)',
                "import_error": f'ERROR: cannot accept value "\\x{long_hex}" (SQLSTATE 22P02)',
                "run_status": "OK"}
        label, css, note = bp.cell("live", mode, "WORKS", True, "CATSTAT-005", set())
        self.assertNotIn(long_hex, note)
        self.assertIn("hex digits", note)

    def test_short_hex_payload_is_left_alone(self):
        # A short hex span (a handful of bytes) IS the finding and must stay
        # readable, not collapsed away.
        text = 'the field held \\x6f746865725f637572736f72 verbatim'
        self.assertEqual(bp.shorten_hex(text), text)

    def test_export_crashes_has_its_own_sentence(self):
        mode = {"verdict": "EXPORTER_CRASHES",
                "evidence": "export data itself dies",
                "run_status": "OK"}
        label, css, note = bp.cell("live", mode, "WORKS", True, "DOM-005", set())
        self.assertEqual(label, "Export crashes")
        self.assertIn("export process died", note)


class Rule7ExcludedFromBatchRuns(unittest.TestCase):
    def test_poison_prefix_is_removed(self):
        reason = "POISON: deterministic BLOCKS in LIVE (import: syntax error, SQLSTATE 42601)"
        out = bp.humanize_exclusion_reason(reason)
        assert_no_jargon(out)
        self.assertNotIn("POISON", out)
        self.assertIn("SQLSTATE 42601", out)

    def test_must_be_run_solo_is_reworded(self):
        reason = "POISON: some finding. Must be run solo."
        out = bp.humanize_exclusion_reason(reason)
        self.assertNotIn("solo.", out.lower().replace("not", ""))  # no bare jargon leftover
        self.assertIn("on its own", out)

    def test_load_excluded_reasons_keys_by_probe_id(self):
        with tempfile.NamedTemporaryFile("w", suffix=".csv", delete=False, newline="") as f:
            w = csv.writer(f)
            w.writerow(["probe_id", "mode", "reason"])
            w.writerow(["DOM-003", "LIVE", "POISON: deterministic BLOCKS in LIVE (x)"])
            w.writerow(["DOM-003", "FALL-BACK", "POISON: deterministic BLOCKS in LIVE (x)"])
            path = f.name
        try:
            reasons = bp.load_excluded_reasons(path)
            self.assertIn("DOM-003", reasons)
            assert_no_jargon(reasons["DOM-003"])
        finally:
            os.unlink(path)


class Rule8Coverage(unittest.TestCase):
    def test_build_coverage_buckets_match_cell_labels(self):
        rows = [
            {"probe_id": "A", "group": "misc",
             "offline": {"verdict": "WORKS", "run_status": "OK"},
             "live": {"verdict": "WORKS", "run_status": "OK"},
             "fall_back": {"verdict": "WORKS", "run_status": "OK"}},
            {"probe_id": "B", "group": "misc",
             "offline": {"verdict": "WORKS", "run_status": "OK"},
             "live": {"verdict": "STUCK", "run_status": "OK", "evidence": "x"},
             "fall_back": {"verdict": "NOT_TESTED"}},
            {"probe_id": "C", "group": "controls",  # must be excluded entirely
             "offline": {"verdict": "WORKS", "run_status": "OK"},
             "live": {"verdict": "WORKS", "run_status": "OK"},
             "fall_back": {"verdict": "WORKS", "run_status": "OK"}},
        ]
        cov = bp.build_coverage(rows, fallback_skip_ids=set())
        # Row A: three trusted cells (offline/live/fall_back all WORKS).
        # Row B: offline trusted, live trusted (Import stops), fall_back
        # "Not reachable" (no measurement, live already failed).
        self.assertEqual(cov["offline"]["trusted"], 2)
        self.assertEqual(cov["live"]["trusted"], 2)
        self.assertEqual(cov["fall_back"]["trusted"], 1)
        self.assertEqual(cov["fall_back"]["not_reachable"], 1)
        # Controls never counted.
        self.assertEqual(cov["offline"]["total"], 2)
        self.assertEqual(cov["fall_forward"]["not_run"], 2)  # both rows: NOT_TESTED

    def test_render_coverage_html_has_no_leftover_hardcoded_counts(self):
        cov = bp.build_coverage(
            [{"probe_id": "A", "group": "misc",
              "offline": {"verdict": "WORKS", "run_status": "OK"},
              "live": {"verdict": "WORKS", "run_status": "OK"},
              "fall_back": {"verdict": "WORKS", "run_status": "OK"}}],
            fallback_skip_ids=set())
        out = bp.render_coverage_html(cov, ntypes=1)
        self.assertNotIn("223", out)
        self.assertIn("<table", out)


class Rule9PlainLanguage(unittest.TestCase):
    SAMPLE_MODES = [
        ("WORKS", {"verdict": "WORKS", "run_status": "OK"}),
        ("QUIET_DROP", {"verdict": "QUIET_DROP", "evidence": "column absent", "run_status": "OK"}),
        ("EXCLUDED_TOLD", {"verdict": "EXCLUDED_TOLD", "evidence": "asked before continuing",
                            "run_status": "OK"}),
        ("SILENT_WRONG", {"verdict": "SILENT_WRONG",
                           "evidence": 'streaming source->target: [update-this-column] id=1 '
                                       'source="1" destination="2"', "run_status": "OK"}),
        ("SILENT_LOSS", {"verdict": "SILENT_LOSS", "evidence": "value never arrived",
                          "run_status": "OK"}),
        ("STUCK", {"verdict": "STUCK", "evidence": "SQLSTATE 22P02: ERROR: bad (SQLSTATE 22P02)",
                   "run_status": "OK"}),
        ("BLOCKS", {"verdict": "BLOCKS", "evidence": "SQLSTATE 0A000: ERROR: nope (SQLSTATE 0A000)",
                    "run_status": "OK"}),
        ("EXPORTER_CRASHES", {"verdict": "EXPORTER_CRASHES", "evidence": "crash",
                               "run_status": "OK"}),
        ("SKIPPED-target-ext", {"verdict": "SKIPPED", "evidence": "extension unavailable: postgis",
                                 "run_status": "OK"}),
        ("SKIPPED-target-ddl", {"verdict": "SKIPPED", "evidence": "ddl rejected on target",
                                 "run_status": "OK"}),
        ("SKIPPED-source", {"verdict": "SKIPPED", "evidence": "refused on source",
                             "run_status": "OK"}),
        ("SKIPPED-other", {"verdict": "SKIPPED", "evidence": "something else entirely",
                            "run_status": "OK"}),
        ("INCONCLUSIVE", {"verdict": "INCONCLUSIVE", "evidence": "timed out", "run_status": "OK"}),
        ("NOT_TESTED", {"verdict": "NOT_TESTED"}),
        ("cutover-abort", {"verdict": "BLOCKS", "evidence": "cutover to target did not complete",
                            "run_status": "OK"}),
        ("spoiled-run", {"verdict": "STUCK", "evidence": "x", "run_status": "INVALID"}),
    ]

    def test_every_produced_label_is_in_the_legend(self):
        for name, mode in self.SAMPLE_MODES:
            for mode_key, live_verdict in (("offline", "WORKS"), ("live", "WORKS"),
                                            ("fall_back", "WORKS"), ("fall_back", "STUCK")):
                label, css, note = bp.cell(mode_key, mode, live_verdict, True,
                                            "PID-1", {"PID-1"} if name == "NOT_TESTED" else set())
                self.assertIn(label, LEGEND_LABELS,
                               f"label {label!r} for {name}/{mode_key} not in legend")

    def test_no_banned_jargon_in_any_tooltip(self):
        for name, mode in self.SAMPLE_MODES:
            for mode_key in ("offline", "live", "fall_back"):
                label, css, note = bp.cell(mode_key, mode, "STUCK", True, "PID-1", set())
                assert_no_jargon(note)

    def test_skipped_fallback_catchall_label_in_legend(self):
        label, css, note = bp.skipped_cell("nothing matches either known shape")
        self.assertEqual(label, "Column cannot exist")
        self.assertIn(label, LEGEND_LABELS)

    def test_batch_group_names_are_relabeled_in_plain_language(self):
        # The catalog names one batch "poison" (probes expected to crash the
        # exporter or importer) - that word is banned from the page itself,
        # even though it is a legitimate internal batch name in rows.json.
        out = bp.plain_group("poison")
        assert_no_jargon(out)
        self.assertNotEqual(out, "poison")

    def test_ordinary_group_names_pass_through(self):
        self.assertEqual(bp.plain_group("ranges"), "ranges")


class MainEndToEnd(unittest.TestCase):
    """main() must still work with just the original three positional
    arguments, and the extra arguments must be genuinely optional."""

    def _write(self, path, content):
        with open(path, "w") as f:
            f.write(content)

    def test_three_argument_call_shape_still_works(self):
        with tempfile.TemporaryDirectory() as d:
            rows_path = os.path.join(d, "rows.json")
            out_path = os.path.join(d, "out.html")
            tmpl_path = os.path.join(d, "tmpl.html")
            self._write(rows_path, json.dumps({
                "voyager_commit": "abc123", "pg_version": "17.8", "yb_version": "2026.1",
                "rows": [
                    {"probe_id": "P1", "type_name": "sometype", "group": "misc",
                     "offline": {"verdict": "WORKS", "run_status": "OK"},
                     "live": {"verdict": "WORKS", "run_status": "OK"},
                     "fall_back": {"verdict": "WORKS", "run_status": "OK"}},
                ],
            }))
            self._write(tmpl_path,
                        "<title>t</title>__VOYAGER_COMMIT__ __PG_VERSION__ __YB_VERSION__ "
                        "__NTYPES__ __GENERATED__ __CONTROLCHECK__ __COVERAGE_TABLE__ "
                        "__PROVENANCE_LINE__ const ROWS = /*__ROWS__*/[];")
            bp.main(rows_path, out_path, tmpl_path)  # no optional args
            with open(out_path) as f:
                out = f.read()
            self.assertIn("abc123", out)
            self.assertIn("17.8", out)
            self.assertNotIn("__ROWS__", out)
            self.assertNotIn("__COVERAGE_TABLE__", out)

    def test_optional_args_add_commits_and_exclusion_markers(self):
        with tempfile.TemporaryDirectory() as d:
            rows_path = os.path.join(d, "rows.json")
            out_path = os.path.join(d, "out.html")
            tmpl_path = os.path.join(d, "tmpl.html")
            all_csv_path = os.path.join(d, "all.csv")
            excluded_csv_path = os.path.join(d, "excluded.csv")
            skip_path = os.path.join(d, "appended.txt")

            self._write(rows_path, json.dumps({
                "voyager_commit": "newcommit", "pg_version": "17.8", "yb_version": "2026.1",
                "rows": [
                    {"probe_id": "REG-001", "type_name": "regproc", "group": "regtypes",
                     "offline": {"verdict": "WORKS", "run_status": "OK"},
                     "live": {"verdict": "STUCK", "run_status": "OK", "evidence": "x"},
                     "fall_back": {"verdict": "NOT_TESTED"}},
                ],
            }))
            self._write(all_csv_path,
                        "voyager_commit,pg_version,yb_version\noldcommit,17.8,2026.1\n")
            self._write(excluded_csv_path,
                        'probe_id,mode,reason\nREG-001,LIVE,"POISON: reg* family travels as '
                        'raw bytes. Must be run solo."\n')
            self._write(skip_path, "SKIP REG-001 FALL-BACK :: not reachable, live import died "
                                    "(STUCK, 42883)\n")
            self._write(tmpl_path,
                        "<title>t</title>__VOYAGER_COMMIT__ __NTYPES__ __GENERATED__ "
                        "__CONTROLCHECK__ __COVERAGE_TABLE__ __PROVENANCE_LINE__ "
                        "__PG_VERSION__ __YB_VERSION__ const ROWS = /*__ROWS__*/[];")

            bp.main(rows_path, out_path, tmpl_path, all_csv_path, excluded_csv_path, skip_path)
            with open(out_path) as f:
                out = f.read()
            self.assertIn("newcommit", out)
            self.assertIn("oldcommit", out)  # the older commit is surfaced too

            rows_start = out.index("const ROWS = ") + len("const ROWS = ")
            rows_json = out[rows_start:out.rindex(";")]
            rows = json.loads(rows_json)
            row = rows[0]
            self.assertEqual(row["f"][0], "Not reachable")
            self.assertTrue(row["xr"])
            self.assertNotIn("POISON", row["xr"])


if __name__ == "__main__":
    unittest.main()
