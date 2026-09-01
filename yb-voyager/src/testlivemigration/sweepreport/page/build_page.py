#!/usr/bin/env python3
"""Build the published Datatype Survival Map from the sweep's own output.

    python3 build_report.py report-rows.json out.html

The page is a view over report-rows.json: every row shown was measured, and a
verdict whose run failed its control gate is never displayed as a result.
"""
import json, sys, html, datetime

# Every label has to answer two questions at a glance: did we probe it, and what
# happened. "Not tested" was doing the work of three different situations —
# never attempted, attempted but the column cannot exist, and attempted but the
# measurement was thrown away — which made the report look less thorough than it
# was and hid the ones that genuinely still need running.
# The vocabulary is deliberately two-tier and grammatically parallel, because a
# reader scans a column vertically and should not have to switch frame of
# reference on every row.
#
#   Tier 1 - WHAT HAPPENED TO THE DATA. Seven labels, all outcomes.
#     Works · Dropped · Silently wrong · Silently lost · Import fails ·
#     Export fails · Rejected by target
#     "Silently wrong" and "Silently lost" are worded as a pair on purpose: they
#     are the same family (no error is raised) and the silence is the finding.
#     "Import fails" and "Export fails" are a pair for the same reason - same
#     shape, and the only difference is which process dies.
#
#   Tier 2 - WHAT WE KNOW. Three labels, all about the measurement, never the
#     product. One word each so they cannot be mistaken for an outcome.
#     Discarded · Inconclusive · Not run
VERDICT = {                       # suite verdict -> (label shown, css class)
    "WORKS":            ("Works",              "v-works"),
    "QUIET_DROP":       ("Dropped",            "v-drop"),
    "IMPORTER_STOPS":   ("Import fails",       "v-imp"),
    "BLOCKS":           ("Import fails",       "v-imp"),
    "STUCK":            ("Import fails",       "v-imp"),
    "EXPORTER_CRASHES": ("Export fails",       "v-exp"),
    "SILENT_WRONG":     ("Silently wrong",     "v-wrong"),
    "SILENT_LOSS":      ("Silently lost",      "v-wrong"),
    "TARGET_REJECTS":   ("Rejected by target", "v-reject"),
    "INCONCLUSIVE":     ("Inconclusive",       "v-incon"),
    "NOT_TESTED":       ("Not run",            "v-none"),
    "":                 ("Not run",            "v-none"),
}

def skipped_label(ev):
    """A skip is a RESULT, not a gap: we probed it and the column cannot exist.
    Name the side that refused — 'the target cannot hold this type' and 'we
    never got to it' mean completely different things to a reader."""
    e = (ev or "").lower()
    if "on target" in e or "target:" in e:
        return "Rejected by target", "v-reject"
    if "on source" in e or "source:" in e:
        return "Rejected by source", "v-reject"
    return "Rejected", "v-reject"

def cell(mode):
    """One mode's verdict, refusing anything whose run failed the control gate."""
    if not isinstance(mode, dict):
        return VERDICT[""][0], VERDICT[""][1], ""
    v = (mode.get("verdict") or "").upper()
    status = (mode.get("run_status") or "OK").upper()
    ev = mode.get("evidence") or ""
    # A skip is decided at setup — the column could not be created — so it holds
    # regardless of whether the run's control probes later passed.
    if v == "SKIPPED":
        label, css = skipped_label(ev)
        return label, css, ev
    # A fall-back run that never got past cutover is not a lost measurement: it is
    # the finding. Fall-back only exists after a successful cutover, so a type that
    # kills the forward migration makes the return trip UNREACHABLE. Reporting that
    # as "discarded" would hide the most important thing about it - the safety net
    # is missing exactly when you would need it.
    if "cutover" in (ev or "").lower() and "not complete" in (ev or "").lower():
        return ("Cutover fails", "v-imp",
                "the forward migration never reached cutover, so the return trip "
                "never existed. " + (ev or ""))
    if status not in ("", "OK"):
        # We DID probe this. The run's known-good control probes failed, so the
        # verdict it produced cannot be trusted — but saying "not tested" would
        # both understate the work and hide that this one needs re-running.
        return ("Discarded", "v-disc",
                f"a verdict of {v.replace('_',' ').lower()} was produced but the run's "
                f"control probes failed ({status.lower()}), so it is not reported. "
                f"Needs a clean re-run. " + (ev or ""))
    label, css = VERDICT.get(v, (v.replace("_", " ").title(), "v-none"))
    return label, css, ev

def main(src, dst, tmpl):
    rows = json.load(open(src))
    if isinstance(rows, dict):
        rows = rows.get("rows", [])

    out, counts = [], {}
    for r in rows:
        off_l, off_c, off_e = cell(r.get("offline"))
        liv_l, liv_c, liv_e = cell(r.get("live"))
        fb_l,  fb_c,  fb_e  = cell(r.get("fall_back"))
        evidence = r.get("note") or liv_e or off_e or fb_e or ""
        out.append({
            "t":  r.get("type_name", r.get("probe_id", "?")),
            "g":  r.get("group", "other"),
            "k":  r.get("kind", ""),
            "o":  [off_l, off_c], "l": [liv_l, liv_c], "f": [fb_l, fb_c],
            "a":  r.get("reported_by_assess") or "No",
            "n":  r.get("reported_by_analyze") or "No",
            "gr": r.get("guardrail_action") or "No",
            "d":  r.get("reported_by_docs") or "No",
            "e":  evidence,
        })
        for lbl in (off_l, liv_l, fb_l):
            counts[lbl] = counts.get(lbl, 0) + 1

    page = open(tmpl).read()
    page = page.replace("/*__ROWS__*/[]", json.dumps(out, ensure_ascii=False))
    page = page.replace("__GENERATED__",
                        datetime.datetime.now(datetime.timezone.utc).strftime("%d %B %Y"))
    page = page.replace("__NTYPES__", str(len(out)))
    open(dst, "w").write(page)
    print(f"wrote {dst}: {len(out)} type rows")
    for k, v in sorted(counts.items(), key=lambda x: -x[1]):
        print(f"   {k:24} {v}")

if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2], sys.argv[3])
