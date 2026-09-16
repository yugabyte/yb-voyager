#!/usr/bin/env python3
"""Build the published Datatype Survival Map from the sweep's own output.

    python3 build_page.py report-rows.json out.html page_template.html \
        [all.csv] [excluded.csv] [fall-back-skip.txt]

The page is a VIEW over report-rows.json. Two rules hold absolutely:

  * Every label shown was measured. Nothing is inferred from a type's name, its
    category, or how a similar type behaved.
  * A cell only makes a claim ABOUT THE TYPE when the measurement can be pinned
    on that type. Otherwise it says, in words, why it cannot.

That second rule is the whole reason this file is careful. Three separate bugs
in earlier versions all had the same shape - a label asserting a cause the run
never established:

  1. "Cutover fails" was shown as an outcome for 87 types. Every one of those
     runs had ALSO killed the known-good `int` and `text` controls, so the run
     never reached cutover for reasons that had nothing to do with the type in
     that row. Checking the cutover branch BEFORE the control gate is what let
     a spoiled run masquerade as a finding.
  2. The controls themselves were published as failing in all three modes,
     i.e. the report claimed PostgreSQL integers do not migrate. A control is
     instrumentation, never a subject; it belongs in a self-check, not in the
     type table.
  3. Notes pasted raw log lines. A SQLSTATE is meaningful, but only once it is
     translated - "22P02" says nothing, "the target could not parse the value"
     says the whole thing.

So: gate first, then the type's own result, and every note is a sentence.

Three optional trailing arguments extend what the page can say, without
changing the required three-argument call:

  * `all.csv`       - the flat per-run results. Only its distinct
                       `voyager_commit` values are used here, to say when the
                       corpus was stitched together from more than one run.
  * `excluded.csv`   - probes left out of shared batch runs by design
                       (`probe_id,mode,reason`). Used to mark a type's name
                       with why it never ran alongside the others.
  * a SKIP file       - lines of the shape
                       `SKIP <id> FALL-BACK :: not reachable, live import died
                       (<verdict>, <sqlstate>)`, one per probe whose fall-back
                       leg was never attempted because live already failed.
                       Most such probes already say so in their own evidence
                       (`cutover ... did not complete`); this file catches the
                       remainder, which carry no evidence at all
                       (`verdict: NOT_TESTED`).
"""
import json, sys, html, datetime, re, csv

# ---------------------------------------------------------------------------
# THE VOCABULARY
#
# Two tiers, and the tier is the point. Tier 1 says what happened to the data.
# Tier 2 says we could not find out, and why. A reader must never have to guess
# which kind of statement a cell is making.
#
# Tier 1 labels name the PROCESS that fails, because "blocked" did not say
# whether the importer or the exporter dies, and those are different problems
# for whoever is on call.
# ---------------------------------------------------------------------------
T1 = {
    "WORKS":            ("Works",             "v-works"),
    "QUIET_DROP":       ("Column dropped",    "v-drop"),
    "EXCLUDED_TOLD":    ("Column dropped, you were asked", "v-told"),
    "SILENT_WRONG":     ("Wrong value",       "v-wrong"),
    "SILENT_LOSS":      ("Data lost",         "v-wrong"),
    "BLOCKS":           ("Import stops",      "v-imp"),
    "STUCK":            ("Import stops",      "v-imp"),
    "IMPORTER_STOPS":   ("Import stops",      "v-imp"),
    "EXPORTER_CRASHES": ("Export crashes",    "v-exp"),
}

# SQLSTATE is the only part of an import failure log line that carries meaning a
# reader can act on. The raw line is a timestamp, a file:line and a truncated
# batch identifier; the code says what the target actually objected to.
SQLSTATE = {
    "22P02": "the target could not parse the value",
    "42601": "the value was pasted into the SQL statement instead of being passed as a parameter, producing invalid SQL",
    "0A000": "YugabyteDB does not support this operation",
    "42704": "the object named inside the value does not exist on the target",
    "42883": "the function named inside the value does not exist on the target",
    "3F000": "the schema named inside the value does not exist on the target",
    "42P01": "the table named inside the value does not exist on the target",
}

FAILED = {"BLOCKS", "STUCK", "IMPORTER_STOPS", "EXPORTER_CRASHES"}
MODE_NAME = {"offline": "offline", "live": "live", "fall_back": "fall-back",
             "fall_forward": "fall-forward"}
TIER2_LABELS = {"Not reachable", "Not measured", "No result", "Not run"}

# The fixed tooltip for a fall-back cell that was never attempted because its
# own live leg had already failed. Rule (1) in the page spec: this applies
# whether we know that from a `SKIP ... FALL-BACK` line (no measurement at
# all was ever recorded) or because the LIVE cell itself reads "Import stops"
# / "Export crashes".
NOT_REACHABLE_LIVE_NOTE = (
    "Fall-back was not run: the live import for this type fails first, so "
    "there is nothing to fall back from."
)

# The operation tags the harness writes inside `[...]` in a value-comparison
# detail, turned into plain English. Anything not listed falls back to a
# generic un-jargoned rendering (see humanize_op).
OP_LABELS = {
    "update-this-column":  "an update to this column",
    "update-other-column": "an update to a different column in the same row",
    "NULL->value":         "changing the value from NULL to a value",
    "value->NULL":         "changing the value to NULL",
    "insert":              "an insert",
    "delete":              "a delete",
}

# `[op] id=N source="X" destination="Y"` - the one place a per-cell value pair
# is recorded against a specific operation and row. This is the ONLY source
# used for a Wrong value / Data lost tooltip's source/target pair; the flat
# `source_value` / `target_value` fields (on this cell, or in all.csv) are
# deliberately never read for this, because for a fall-back row they hold the
# value AFTER the operation on the return trip, not the before/after pair the
# reader needs to see.
OPVAL_RE = re.compile(
    r'\[([A-Za-z0-9_.>-]+)\]\s+id=(\d+)\s+source="([^"]*)"\s+destination="([^"]*)"'
)


def strip_note(ev):
    """Drop our own bracketed annotation - it is commentary, not evidence."""
    return re.sub(r"\s+", " ", re.sub(r"\[[^\]]*\]", "", ev or "")).strip()


def humanize_op(op):
    if op in OP_LABELS:
        return OP_LABELS[op]
    return op.replace("->", " to ").replace("_", " ").replace("-", " ")


def fmt_value(v):
    return f'"{v}"' if v != "" else "an empty string"


# ---------------------------------------------------------------------------
# Shortening long hex payloads and file paths in tooltip text. A short hex
# span (a handful of bytes) IS the finding - e.g. a cursor name that arrived
# as `\x6f746865725f637572736f72` - and stays untouched. A long one (a whole
# serialized statistics object, hundreds of hex digits) is not itself
# readable and only clutters the tooltip, so past a threshold it is
# collapsed to its first few digits plus a byte count.
# ---------------------------------------------------------------------------
_HEX_RE = re.compile(r'(\\x|0x)([0-9a-fA-F]+)')
_PATH_RE = re.compile(r'(?:/[\w.\-]+){3,}')


def shorten_hex(text, threshold=32, keep=12):
    def repl(m):
        prefix, digits = m.groups()
        if len(digits) <= threshold:
            return m.group(0)
        return f"{prefix}{digits[:keep]}…({len(digits)} hex digits)"
    return _HEX_RE.sub(repl, text or "")


def shorten_paths(text, threshold=40):
    def repl(m):
        s = m.group(0)
        if len(s) <= threshold:
            return s
        parts = [p for p in s.split("/") if p]
        return "/" + parts[0] + "/…/" + parts[-1]
    return _PATH_RE.sub(repl, text or "")


def shorten(text):
    """Apply both shortenings. Safe to call on any tooltip text - a no-op
    when there is nothing long enough to collapse."""
    return shorten_paths(shorten_hex(text or ""))


def sqlstate_of(ev):
    m = re.search(r"SQLSTATE\s+([0-9A-Za-z]{5})", ev or "")
    return m.group(1).upper() if m else ""


def is_cutover_abort(ev):
    e = (ev or "").lower()
    return "cutover" in e and "not complete" in e


def forward_leg_failure_detail(ev):
    """Rule (3): a detail of the shape `fall-back not reached: forward leg
    failed (...)` means fall-back is unreachable for a reason that is fully
    explained by the forward leg's own failure - never a claim about the
    return trip. Returns the parenthesised forward detail (possibly empty),
    or None if the phrase is not present at all."""
    e = strip_note(ev)
    m = re.search(r"not reached:\s*forward leg failed\s*\((.*)\)\s*$", e, re.IGNORECASE)
    if m:
        return m.group(1).strip()
    if re.search(r"not reached:\s*forward leg failed", e, re.IGNORECASE):
        return ""
    return None


def explain(mode_key, verdict, ev, import_error=""):
    """One plain sentence saying what was observed. No log lines, no jargon."""
    e = strip_note(ev).lower()
    where = MODE_NAME.get(mode_key, mode_key)

    if verdict == "WORKS":
        # What "checked" means is different in each mode, and a reader who did not run
        # the sweep has no way to guess that from the word "Works" alone.
        if mode_key == "offline":
            return ("The row was copied once, during the snapshot, and matched the source "
                    "exactly. Offline migration only copies data once, so there are no "
                    "later changes to check.")
        if mode_key == "live":
            return ("Every change was checked and matched: an insert, an update, a delete, "
                    "and setting the value to NULL and back again, all streamed to the "
                    "target correctly.")
        if mode_key == "fall_back":
            return ("The same checks ran in reverse: an insert, an update, a delete, and a "
                    "NULL change, streamed from YugabyteDB back to PostgreSQL, all matched "
                    "correctly.")
        return "The value arrived unchanged."

    if verdict == "EXCLUDED_TOLD":
        return ("The column was left out of the change stream and voyager asked before "
                "continuing (or --yes answered for you).")

    if verdict == "QUIET_DROP":
        if "no value can be lost" in e:
            return ("This type accepts only NULL in PostgreSQL 17.8. The column never "
                    "appears in the change stream, so nothing can be lost, but a real "
                    "value could not be tested.")
        return ("The column is left out of every change event, so later updates to it "
                "never reach the target. Nothing is logged as an error. The first copy "
                "still carries the old values, so the column looks populated while "
                "quietly going out of date.")

    if verdict in ("SILENT_WRONG", "SILENT_LOSS"):
        lead = ("A different value arrived." if verdict == "SILENT_WRONG"
                else "The value never arrived.")
        m = OPVAL_RE.search(ev or "")
        if m:
            op, rowid, src, dst = m.groups()
            sentence = f" The operation was {humanize_op(op)} (row {rowid})"
            if "target->source" in (ev or ""):
                sentence += ", measured in the reverse direction (YugabyteDB back to PostgreSQL)"
            sentence += f": source {fmt_value(src)}, target {fmt_value(dst)}."
            return shorten(lead + sentence + " No error was printed anywhere.")
        if "absent" in e and "column" in e:
            return (lead + " The column never appeared in the change stream at all, so "
                    "there was no value to compare side by side. No error was printed "
                    "anywhere.")
        return lead + " No error was printed anywhere."

    if verdict in FAILED:
        code = sqlstate_of(ev)
        why = SQLSTATE.get(code, "")
        if verdict == "EXPORTER_CRASHES":
            return ("The export process died, usually before any row moved. Its only "
                    "message is that export failed and to check the logs — it does not "
                    "name the table, the column or the type.")
        # "Import stops" always means the same thing: a SQL error on this specific
        # value, and the importer exiting rather than retrying past it.
        base = "The importer hit a SQL error on this type and exited."
        if why:
            base += f" The target's complaint was that {why} (SQLSTATE {code})."
        elif code:
            base += f" The target reported SQLSTATE {code}."
        else:
            base += " The error and SQLSTATE are shown below."
        # The full `ERROR: ... (SQLSTATE xxxxx)` line, wherever it can be found -
        # the classified import_error field first, falling back to the same span
        # inside the evidence itself - with any long hex payload or file path
        # collapsed so the tooltip stays readable.
        line = import_error or ""
        if not line:
            m = re.search(r"ERROR:.*?\(SQLSTATE\s+[0-9A-Za-z]{5}\)", ev or "")
            if m:
                line = m.group(0)
        if line:
            base += f" Error: {line}"
        return shorten(base)

    if verdict == "INCONCLUSIVE":
        if "exporter died" in e:
            return ("The export process crashed before this type was reached, so nothing "
                    "was ever produced for it and no claim can be made about it.")
        return ("The run timed out before any change for this type was seen. Nothing can "
                "be claimed either way.")

    return strip_note(ev) or "No detail was recorded."


# The catalog's own batch names are written for the harness author, not a
# reader of the page, and one of them ("poison") is exactly the jargon word
# rule (9) bans. Group names are otherwise shown verbatim (as a filter value,
# a group header and a table column), so this is the one place to relabel
# them in plain language.
GROUP_LABELS = {
    "poison": "known crash cases",
}


def plain_group(g):
    return GROUP_LABELS.get(g, g)


def skipped_cell(ev):
    """A skip is a RESULT: we tried, and the column cannot exist. Which side
    refused matters — 'the target cannot hold this type' and 'we never got to
    it' mean entirely different things to whoever reads this."""
    e = (ev or "").lower()
    if "extension unavailable" in e:
        return ("Target rejects type", "v-reject",
                "The extension that provides this type is not installed on YugabyteDB, "
                "so a column of this type cannot exist on the target and nothing can "
                "migrate. This is a finding, not an untested gap.")
    if "ddl rejected" in e or "on target" in e or "target:" in e:
        return ("Target rejects type", "v-reject",
                "YugabyteDB refuses to create a column of this type at all, so nothing "
                "can migrate. This is a finding, not an untested gap.")
    if "on source" in e or "source:" in e:
        return ("Source rejects value", "v-reject",
                "PostgreSQL itself refuses every literal we could write for this type, so "
                "there is no value to migrate. The column can exist; it cannot be filled.")
    return ("Column cannot exist", "v-reject",
            "The probe could not be set up at all, for a reason that does not fit the "
            "usual two shapes above. " + (strip_note(ev) or "No detail was recorded."))


def cell(mode_key, mode, live_verdict, live_ok, probe_id="", fallback_skip_ids=None):
    """One mode's cell: (label, css, note).

    Order is load-bearing and must not be rearranged:
      1. Nothing recorded            -> Not run, unless this is a FALL-BACK
                                         cell that we independently know was
                                         never reachable (a SKIP line, or the
                                         LIVE cell already failed) -> Not
                                         reachable instead.
      2. Column could not exist      -> a setup-time fact, true regardless of the gate.
      3. Forward leg failed so badly that fall-back/fall-forward never got a
         return leg to measure          -> Not reachable, never a claim about
                                            the type.
      4. Cutover never finished      -> NEVER a claim about the type, ALWAYS "Not
                                         reachable" for fall-back (see below — it does
                                         not matter what the live cell says).
      5. Run's controls died         -> not attributable to this type.
      6. Only now, the type's own measured result.
    Putting 4 or 6 before 5 is exactly the bug that published 87 spoiled runs
    as findings.
    """
    fallback_skip_ids = fallback_skip_ids or set()
    v = (mode.get("verdict") or "").upper() if isinstance(mode, dict) else ""

    if not isinstance(mode, dict) or v in ("", "NOT_TESTED"):
        if mode_key == "fall_back":
            live_already_failed = live_verdict in FAILED  # STUCK/BLOCKS/IMPORTER_STOPS/EXPORTER_CRASHES
            if probe_id in fallback_skip_ids or live_already_failed:
                return ("Not reachable", "v-none", NOT_REACHABLE_LIVE_NOTE)
        return ("Not run", "v-none",
                "This combination of type and migration mode has not been attempted yet.")

    ev = mode.get("evidence") or ""
    ok = (mode.get("run_status") or "OK").upper() in ("", "OK", "ATTRIBUTED", "POISON")
    import_error = mode.get("import_error") or ""

    if v == "SKIPPED":
        return skipped_cell(ev)

    # A forward leg that failed badly enough that cutover, and therefore the
    # return trip, never happened. The forward failure fully explains why
    # this mode has nothing of its own to report.
    fwd = forward_leg_failure_detail(ev)
    if fwd is not None:
        note = NOT_REACHABLE_LIVE_NOTE
        if fwd:
            note += f" The forward leg's own failure: {shorten(fwd)}."
        return ("Not reachable", "v-none", note)

    # Fall-back only exists after a successful cutover. If cutover never finished, the
    # return trip never started for THIS type — full stop. Earlier this only said "Not
    # reachable" when the live cell itself had already failed, and fell back to "Not
    # measured" (implying some other type in the run was to blame) whenever the live
    # cell looked fine. But a fall-back row carrying this detail already tells us why
    # fall-back never ran for this type: its own forward migration never reached
    # cutover. That is always "Not reachable", regardless of what the live column says.
    if is_cutover_abort(ev):
        return ("Not reachable", "v-none",
                "Fall-back was never reached: the live import for this type had already "
                "failed.")

    if not ok:
        if live_ok and live_verdict in FAILED:
            return ("Not reachable", "v-none",
                    "This type stops the forward migration (see the live column), so this "
                    "mode was never reachable for it.")
        return ("Not measured", "v-disc",
                "Another datatype sharing this run broke the migration first, so nothing "
                "measured here can be pinned on this type. The known-good control types "
                "in the same run failed too, which is how we know. Needs a re-run on its own.")

    if v == "INCONCLUSIVE":
        return ("No result", "v-incon", explain(mode_key, v, ev, import_error))

    label, css = T1.get(v, (v.replace("_", " ").capitalize(), "v-none"))
    return (label, css, explain(mode_key, v, ev, import_error))


# How bad each label is, worst first. Used only to choose which of the three
# modes the one-line summary column describes.
SEVERITY = ["Data lost", "Wrong value", "Export crashes", "Import stops",
            "Column dropped", "Column dropped, you were asked",
            "Target rejects type", "Source rejects value",
            "Works", "Not reachable", "No result", "Not measured", "Not run"]


def summary_note(*cells):
    """The note for the worst MEASURED outcome across the three modes.

    A row whose live cell says the value is silently corrupted must not summarise
    itself as "the run was aborted before cutover" just because its fall-back cell
    happens to be the last one checked.
    """
    ranked = sorted(cells, key=lambda c: SEVERITY.index(c[0]) if c[0] in SEVERITY else 99)
    return ranked[0][2] if ranked else ""


# ---------------------------------------------------------------------------
# Optional inputs
# ---------------------------------------------------------------------------

_SKIP_RE = re.compile(r'^SKIP\s+(\S+)\s+FALL-BACK\s+::\s*not reachable', re.IGNORECASE)


def load_fallback_skip_ids(path):
    """Parse a `SKIP <id> FALL-BACK :: not reachable, ...` file into the set
    of probe ids it names. Any line not matching that shape is ignored -
    this file is a log, not a strict format."""
    ids = set()
    if not path:
        return ids
    with open(path) as f:
        for line in f:
            m = _SKIP_RE.match(line.strip())
            if m:
                ids.add(m.group(1))
    return ids


def humanize_exclusion_reason(reason):
    """excluded.csv's reason column is written for the harness's own author,
    not a reader of the page - it says "POISON" and assumes familiarity with
    the batching model. Say the same thing in plain terms."""
    r = re.sub(r"(?i)^\s*POISON:\s*", "", reason or "").strip()
    r = re.sub(r"(?i)deterministic BLOCKS in LIVE",
                "it reliably makes the importer stop during live migration", r)
    r = re.sub(r"(?i)Must be run solo\.?",
                "It is tested on its own, not in a shared batch.", r)
    return shorten(r)


def load_excluded_reasons(path):
    """probe_id -> plain-language reason(s) it is excluded from shared batch
    runs. A probe can appear once per mode in the file; reasons are usually
    identical across modes, so duplicates are folded together."""
    reasons = {}
    if not path:
        return reasons
    with open(path, newline="") as f:
        for row in csv.DictReader(f):
            pid = row.get("probe_id") or ""
            reason = humanize_exclusion_reason(row.get("reason") or "")
            if not pid or not reason:
                continue
            seen = reasons.setdefault(pid, [])
            if reason not in seen:
                seen.append(reason)
    return {pid: " / ".join(rs) for pid, rs in reasons.items()}


def load_commits_from_csv(path):
    """Distinct harness commits that stamped a run, oldest run first."""
    first_seen = {}
    if not path:
        return []
    with open(path, newline="") as f:
        for row in csv.DictReader(f):
            c = row.get("voyager_commit") or ""
            if not c:
                continue
            t = row.get("run_timestamp") or ""
            if c not in first_seen or (t and t < first_seen[c]):
                first_seen[c] = t
    return sorted(first_seen, key=lambda c: first_seen[c])


# ---------------------------------------------------------------------------
# Coverage: how many cells, per mode, carry each kind of outcome. Replaces
# any hand-typed count of measured/unreachable/untested cells - this is
# computed straight from the same `cell()` classification the table uses, so
# the two can never drift apart.
# ---------------------------------------------------------------------------

COVERAGE_MODES = [("offline", "Offline"), ("live", "Live"),
                   ("fall_back", "Fall-back"), ("fall_forward", "Fall-forward")]


def build_coverage(rows, fallback_skip_ids):
    cov = {mk: {"trusted": 0, "not_reachable": 0, "not_run": 0,
                "no_result": 0, "not_measured": 0, "total": 0}
           for mk, _ in COVERAGE_MODES}
    for r in rows:
        if (r.get("group") or "") == "controls":
            continue
        lv = ((r.get("live") or {}).get("verdict") or "").upper()
        lok = ((r.get("live") or {}).get("run_status") or "OK").upper() in ("", "OK", "ATTRIBUTED", "POISON")
        pid = r.get("probe_id") or ""
        for mk, _ in COVERAGE_MODES:
            label, _, _ = cell(mk, r.get(mk), lv, lok, pid, fallback_skip_ids)
            bucket = {"Not reachable": "not_reachable", "Not run": "not_run",
                      "No result": "no_result", "Not measured": "not_measured"}.get(label, "trusted")
            cov[mk][bucket] += 1
            cov[mk]["total"] += 1
    return cov


def render_coverage_html(cov, ntypes):
    rows_html = []
    grand = {"trusted": 0, "not_reachable": 0, "not_run": 0, "no_result": 0,
             "not_measured": 0, "total": 0}
    for mk, label in COVERAGE_MODES:
        c = cov[mk]
        for k in grand:
            grand[k] += c[k]
        rows_html.append(
            "<tr><td>{label}</td><td><strong>{trusted}</strong></td>"
            "<td>{not_reachable}</td><td>{not_run}</td><td>{no_result}</td>"
            "<td>{not_measured}</td><td>{total}</td></tr>".format(label=html.escape(label), **c)
        )
    table = (
        '<table class="cov">'
        "<thead><tr><th>Mode</th><th>Trusted verdict</th><th>Not reachable</th>"
        "<th>Not run</th><th>No result</th><th>Not measured</th><th>Cells</th></tr></thead>"
        "<tbody>" + "".join(rows_html) +
        "<tr><td><strong>All modes</strong></td><td><strong>{trusted}</strong></td>"
        "<td>{not_reachable}</td><td>{not_run}</td><td>{no_result}</td>"
        "<td>{not_measured}</td><td><strong>{total}</strong></td></tr>"
        "</tbody></table>".format(**grand)
    )
    prose = (
        f"<p class=\"note\">{ntypes} types across {len(COVERAGE_MODES)} modes is "
        f"{grand['total']} cells. {grand['trusted']} of them carry a trusted, "
        f"measured verdict. {grand['not_reachable']} are marked not reachable "
        f"(an earlier mode for that type already failed), {grand['not_run']} were "
        f"never attempted, {grand['no_result']} timed out with nothing to report, and "
        f"{grand['not_measured']} were spoiled by another type breaking the same run "
        f"and still need a solo re-run.</p>"
    )
    return table + prose


def format_provenance(header_commit, csv_commits, pg_version, yb_version):
    # Run commits first, in run order; the catalogue's own commit last if it never ran.
    commits = list(csv_commits)
    if header_commit and header_commit not in commits:
        commits.append(header_commit)
    # Every commit that stamped a run is a test-harness commit; the voyager code under
    # test did not change between them, so none is "the" commit and none is ranked.
    if not commits:
        commit_str = "unknown"
    else:
        commit_str = ", ".join(commits)
    return commit_str, pg_version or "unknown", yb_version or "unknown"


def main(src, dst, tmpl, all_csv=None, excluded_csv=None, fallback_skip_file=None):
    with open(src) as f:
        data = json.load(f)
    rows = data.get("rows", []) if isinstance(data, dict) else data

    fallback_skip_ids = load_fallback_skip_ids(fallback_skip_file)
    excluded_reasons = load_excluded_reasons(excluded_csv)
    csv_commits = load_commits_from_csv(all_csv)

    out, counts, controls = [], {}, []
    for r in rows:
        # The controls are the harness's own known-answer check. They are not
        # datatypes under audit, and showing them as rows told readers that `int`
        # and `text` do not migrate. They get their own self-check line instead.
        if (r.get("group") or "") == "controls":
            for k in ("offline", "live", "fall_back"):
                m = r.get(k) or {}
                controls.append({"t": r.get("type_name"), "m": MODE_NAME[k],
                                 "v": (m.get("verdict") or "").upper()})
            continue

        pid = r.get("probe_id") or ""
        lv = ((r.get("live") or {}).get("verdict") or "").upper()
        lok = ((r.get("live") or {}).get("run_status") or "OK").upper() in ("", "OK", "ATTRIBUTED", "POISON")

        o = cell("offline",   r.get("offline"),   lv, lok, pid, fallback_skip_ids)
        l = cell("live",      r.get("live"),      lv, lok, pid, fallback_skip_ids)
        f = cell("fall_back", r.get("fall_back"), lv, lok, pid, fallback_skip_ids)

        out.append({
            "t": r.get("type_name", r.get("probe_id", "?")),
            "p": pid,
            "g": plain_group(r.get("group", "other")),
            "k": r.get("kind", ""),
            "o": [o[0], o[1]], "l": [l[0], l[1]], "f": [f[0], f[1]],
            "a":  r.get("reported_by_assess") or "No",
            "n":  r.get("reported_by_analyze") or "No",
            "gr": r.get("guardrail_action") or "No",
            "d":  r.get("reported_by_docs") or "No",
            # The summary column can only carry one of the three modes, so it carries
            # the WORST MEASURED one. Preferring the fall-back cell instead buried
            # every real finding under "the run was aborted" boilerplate, which is the
            # least informative thing on the row.
            # Live first, so that among equally-severe cells the summary describes the
            # mode that exercises the most machinery rather than the snapshot-only one.
            "e":  summary_note(l, f, o),
            "eo": o[2], "el": l[2], "ef": f[2],
            # Rule (7): a probe excluded from shared batch runs by design gets a
            # marker next to its name, with the reason in the marker's tooltip.
            "xr": excluded_reasons.get(pid, ""),
        })
        for lbl in (o[0], l[0], f[0]):
            counts[lbl] = counts.get(lbl, 0) + 1

    ctrl_pass = sum(1 for c in controls if c["v"] == "WORKS")
    ctrl_line = (f"The harness checks itself on every run with two known-good types, "
                 f"<code>int</code> and <code>text</code>. Both migrate correctly in all "
                 f"three modes ({ctrl_pass} of {len(controls)} checks passed). When either "
                 f"one fails, that run is discarded rather than reported — which is why "
                 f"some cells below read <em>Not measured</em>.")

    header_commit = data.get("voyager_commit") if isinstance(data, dict) else None
    pg_version = data.get("pg_version") if isinstance(data, dict) else None
    yb_version = data.get("yb_version") if isinstance(data, dict) else None
    commit_str, pg_str, yb_str = format_provenance(header_commit, csv_commits, pg_version, yb_version)

    cov = build_coverage(rows, fallback_skip_ids)
    coverage_html = render_coverage_html(cov, len(out))

    with open(tmpl) as f:
        page = f.read()
    page = page.replace("/*__ROWS__*/[]", json.dumps(out, ensure_ascii=False))
    page = page.replace("__CONTROLCHECK__", ctrl_line)
    page = page.replace("__GENERATED__",
                        datetime.datetime.now(datetime.timezone.utc).strftime("%d %B %Y"))
    page = page.replace("__NTYPES__", str(len(out)))
    page = page.replace("__VOYAGER_COMMIT__", html.escape(commit_str))
    page = page.replace("__PG_VERSION__", html.escape(pg_str))
    page = page.replace("__YB_VERSION__", html.escape(yb_str))
    page = page.replace("__COVERAGE_TABLE__", coverage_html)
    page = page.replace("__PROVENANCE_LINE__",
                        html.escape(f"yb-voyager {commit_str} · PostgreSQL {pg_str} "
                                    f"→ YugabyteDB {yb_str}"))
    with open(dst, "w") as f:
        f.write(page)

    print(f"wrote {dst}: {len(out)} type rows ({len(controls)} control checks kept out of the table)")
    for k, v in sorted(counts.items(), key=lambda x: -x[1]):
        print(f"   {k:20} {v}")


if __name__ == "__main__":
    args = sys.argv[1:]
    if len(args) < 3:
        sys.exit("usage: build_page.py rows.json out.html page_template.html "
                  "[all.csv] [excluded.csv] [fallback-skip.txt]")
    src, dst, tmpl = args[0], args[1], args[2]
    all_csv = args[3] if len(args) > 3 else None
    excluded_csv = args[4] if len(args) > 4 else None
    fallback_skip_file = args[5] if len(args) > 5 else None
    main(src, dst, tmpl, all_csv, excluded_csv, fallback_skip_file)
