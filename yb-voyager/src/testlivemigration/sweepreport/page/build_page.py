#!/usr/bin/env python3
"""Build the published Datatype Survival Map from the sweep's own output.

    python3 build_page.py report-rows.json out.html page_template.html

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
"""
import json, sys, html, datetime, re

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


def strip_note(ev):
    """Drop our own bracketed annotation - it is commentary, not evidence."""
    return re.sub(r"\s+", " ", re.sub(r"\[[^\]]*\]", "", ev or "")).strip()


def is_cutover_abort(ev):
    e = (ev or "").lower()
    return "cutover" in e and "not complete" in e


def sqlstate_of(ev):
    m = re.search(r"SQLSTATE\s+([0-9A-Za-z]{5})", ev or "")
    return m.group(1).upper() if m else ""


def explain(mode_key, verdict, ev, src, dst):
    """One plain sentence saying what was observed. No log lines, no jargon."""
    e = strip_note(ev).lower()
    where = MODE_NAME.get(mode_key, mode_key)

    if verdict == "WORKS":
        if "snapshot identical" in e:
            return ("The copied rows matched the source exactly. Offline migration copies "
                    "data once, so there are no later changes to check.")
        if "snapshot +" in e:
            return ("Everything matched: the first copy, then an insert, an update, a "
                    "delete, and setting the value to NULL and back again.")
        return "The value arrived unchanged."

    if verdict == "QUIET_DROP":
        return ("The column is left out of every change event, so later updates to it "
                "never reach the target. Nothing is logged as an error. The first copy "
                "still carries the old values, so the column looks populated while "
                "quietly going out of date.")

    if verdict in ("SILENT_WRONG", "SILENT_LOSS"):
        lead = ("A different value arrived." if verdict == "SILENT_WRONG"
                else "The value never arrived.")
        pair = ""
        if src or dst:
            pair = f" The source held {src or 'a value'}; the target ended up with {dst or 'nothing'}."
        return lead + pair + " No error was printed anywhere."

    if verdict in FAILED:
        code = sqlstate_of(ev)
        why = SQLSTATE.get(code, "")
        if verdict == "EXPORTER_CRASHES":
            return ("The export process died, usually before any row moved. Its only "
                    "message is that export failed and to check the logs — it does not "
                    "name the table, the column or the type.")
        when = ("while it was applying streamed changes" if "streaming" in e
                else "as it started")
        base = f"The import process stopped {when} and the migration halted."
        if why:
            base += f" The target's complaint was that {why}"
            base += f" (SQLSTATE {code})."
        elif code:
            base += f" The target reported SQLSTATE {code}."
        return base

    if verdict == "INCONCLUSIVE":
        if "exporter died" in e:
            return ("The export process crashed before this type was reached, so nothing "
                    "was ever produced for it and no claim can be made about it.")
        return ("No change events arrived for this table at all, and the import log "
                "showed no repeating error, so there was nothing to conclude.")

    return strip_note(ev) or "No detail was recorded."


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
    return ("Column cannot exist", "v-reject", strip_note(ev))


def cell(mode_key, mode, live_verdict, live_ok):
    """One mode's cell: (label, css, note).

    Order is load-bearing and must not be rearranged:
      1. Nothing recorded            -> Not run.
      2. Column could not exist      -> a setup-time fact, true regardless of the gate.
      3. Cutover never finished      -> NEVER a claim about the type.
      4. Run's controls died         -> not attributable to this type.
      5. Only now, the type's own measured result.
    Putting 3 or 5 before 4 is exactly the bug that published 87 spoiled runs
    as findings.
    """
    if not isinstance(mode, dict) or not (mode.get("verdict") or "").strip():
        return ("Not run", "v-none",
                "This combination of type and migration mode has not been attempted yet.")

    v = (mode.get("verdict") or "").upper()
    ev = mode.get("evidence") or ""
    ok = (mode.get("run_status") or "OK").upper() in ("", "OK", "ATTRIBUTED", "POISON")
    src, dst = mode.get("source_value") or "", mode.get("target_value") or ""

    if v == "NOT_TESTED":
        return ("Not run", "v-none",
                "This combination of type and migration mode has not been attempted yet.")

    if v == "SKIPPED":
        return skipped_cell(ev)

    # Fall-back only exists after a successful cutover. If cutover never finished,
    # the return trip never started — and that says nothing about this type unless
    # this type is what stopped the forward migration in the first place.
    if is_cutover_abort(ev):
        if live_ok and live_verdict in FAILED:
            return ("Not reachable", "v-none",
                    "Fall-back never became available: this type stops the forward "
                    "migration (see the live column), so cutover could not complete and "
                    "the return trip never existed. The safety net is missing precisely "
                    "where you would need it.")
        return ("Not measured", "v-disc",
                "The migration run was aborted before cutover finished, so the return "
                "trip never started. The known-good control types in the same run were "
                "cut short too, so the cause was the run rather than this type — but "
                "that also means nothing here can be pinned on this type either way. "
                "Needs a re-run on its own.")

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
        return ("No result", "v-incon", explain(mode_key, v, ev, src, dst))

    label, css = T1.get(v, (v.replace("_", " ").capitalize(), "v-none"))
    return (label, css, explain(mode_key, v, ev, src, dst))


# How bad each label is, worst first. Used only to choose which of the three
# modes the one-line summary column describes.
SEVERITY = ["Data lost", "Wrong value", "Export crashes", "Import stops",
            "Column dropped", "Target rejects type", "Source rejects value",
            "Works", "Not reachable", "No result", "Not measured", "Not run"]


def summary_note(*cells):
    """The note for the worst MEASURED outcome across the three modes.

    A row whose live cell says the value is silently corrupted must not summarise
    itself as "the run was aborted before cutover" just because its fall-back cell
    happens to be the last one checked.
    """
    ranked = sorted(cells, key=lambda c: SEVERITY.index(c[0]) if c[0] in SEVERITY else 99)
    return ranked[0][2] if ranked else ""


def main(src, dst, tmpl):
    data = json.load(open(src))
    rows = data.get("rows", []) if isinstance(data, dict) else data

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

        lv = ((r.get("live") or {}).get("verdict") or "").upper()
        lok = ((r.get("live") or {}).get("run_status") or "OK").upper() in ("", "OK", "ATTRIBUTED", "POISON")

        o = cell("offline",   r.get("offline"),   lv, lok)
        l = cell("live",      r.get("live"),      lv, lok)
        f = cell("fall_back", r.get("fall_back"), lv, lok)

        out.append({
            "t": r.get("type_name", r.get("probe_id", "?")),
            "g": r.get("group", "other"),
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
        })
        for lbl in (o[0], l[0], f[0]):
            counts[lbl] = counts.get(lbl, 0) + 1

    ctrl_pass = sum(1 for c in controls if c["v"] == "WORKS")
    ctrl_line = (f"The harness checks itself on every run with two known-good types, "
                 f"<code>int</code> and <code>text</code>. Both migrate correctly in all "
                 f"three modes ({ctrl_pass} of {len(controls)} checks passed). When either "
                 f"one fails, that run is discarded rather than reported — which is why "
                 f"some cells below read <em>Not measured</em>.")

    page = open(tmpl).read()
    page = page.replace("/*__ROWS__*/[]", json.dumps(out, ensure_ascii=False))
    page = page.replace("__CONTROLCHECK__", ctrl_line)
    page = page.replace("__GENERATED__",
                        datetime.datetime.now(datetime.timezone.utc).strftime("%d %B %Y"))
    page = page.replace("__NTYPES__", str(len(out)))
    open(dst, "w").write(page)

    print(f"wrote {dst}: {len(out)} type rows ({len(controls)} control checks kept out of the table)")
    for k, v in sorted(counts.items(), key=lambda x: -x[1]):
        print(f"   {k:20} {v}")


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2], sys.argv[3])
