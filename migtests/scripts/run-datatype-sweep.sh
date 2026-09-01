#!/usr/bin/env bash
#
# Copyright (c) YugabyteDB, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# ---------------------------------------------------------------------------
# run-datatype-sweep.sh - one command to run the PG -> YB datatype sweep.
#
#   migtests/scripts/run-datatype-sweep.sh                 # coverage guard + offline + live
#   migtests/scripts/run-datatype-sweep.sh live            # one mode
#   migtests/scripts/run-datatype-sweep.sh all             # every mode, incl. fall-back/-forward
#   migtests/scripts/run-datatype-sweep.sh coverage        # only the catalogue coverage guard
#   migtests/scripts/run-datatype-sweep.sh probe HSTORE-001 LIVE   # one probe, isolated
#
# It always ends by writing a results CSV and the generated report rows, so a run is
# comparable with any other run:
#
#   $RESULTS_DIR/datatype-sweep-<stamp>.csv    one row per (probe, mode)
#   $RESULTS_DIR/probe-catalog.json            the static per-type half of the report
#   $RESULTS_DIR/report-rows.json|.csv         the published report's row data
#
# Compare two runs:
#   go run ./src/testlivemigration/sweepreport diff -old old.csv -new new.csv
#
# See yb-voyager/src/testlivemigration/DATATYPE_SWEEP.md for the full story.
# ---------------------------------------------------------------------------
set -euo pipefail

# ---------------------------------------------------------------------------
# Environment. Every one of these can be overridden from the caller's shell.
# ---------------------------------------------------------------------------

# PG_VERSION  - source PostgreSQL image tag. MUST NOT be newer than the local pg_dump:
#               `export data` shells out to pg_dump, and a server newer than the client
#               is refused outright. 17.8-ext is the custom image carrying postgis and
#               pgvector; the postgis/pgvector probes report SKIPPED without it.
export PG_VERSION="${PG_VERSION:-17.8}"

# YB_VERSION  - target YugabyteDB image tag.
# Must be a tag that actually exists on Docker Hub, including its build suffix —
# yugabytedb/yugabyte publishes 2026.1.0.0-b118, not a bare 2026.1.0.0. A tag with
# no build number fails at container start with "manifest unknown", which the
# harness reports as a container-setup failure on every probe in the run.
export YB_VERSION="${YB_VERSION:-2026.1.0.0-b118}"

# DEBEZIUM_DIST_DIR - where the live-migration framework finds the Debezium server
#               distribution. Live, fall-back and fall-forward modes cannot run without it.
export DEBEZIUM_DIST_DIR="${DEBEZIUM_DIST_DIR:-/opt/yb-voyager/debezium-server}"

# KUBECONFIG  - HARD-WON: an EXPIRED Teleport kubeconfig kills voyager's Debezium JVM at
#               startup with no error in any voyager log (an ANSI escape byte in
#               ~/.kube/config breaks snakeyaml before the connector starts). It presents
#               as a flaky run that exports zero events. Point KUBECONFIG at an empty file
#               so nothing can read a stale one.
if [ -z "${KUBECONFIG:-}" ] || [ ! -s "${KUBECONFIG}" ]; then
    NEUTRAL_KUBECONFIG="$(mktemp -t voyager-sweep-kubeconfig.XXXXXX)"
    export KUBECONFIG="${NEUTRAL_KUBECONFIG}"
fi

# Telemetry off for a test run.
export YB_VOYAGER_SEND_DIAGNOSTICS="${YB_VOYAGER_SEND_DIAGNOSTICS:-0}"

# PROBE_ID / PROBE_MODE - consumed by TestDatatypeSweepSuspect for a single-probe run.
#               The `probe` subcommand sets them for you.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MODULE_DIR="${REPO_ROOT}/yb-voyager"

RESULTS_DIR="${RESULTS_DIR:-${REPO_ROOT}/datatype-sweep-results}"
TEST_TIMEOUT="${TEST_TIMEOUT:-6h}"
SKIP_BUILD="${SKIP_BUILD:-0}"

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_LOG="${RESULTS_DIR}/run-${STAMP}.log"
CATALOG_JSON="${RESULTS_DIR}/probe-catalog.json"
RESULTS_CSV="${RESULTS_DIR}/datatype-sweep-${STAMP}.csv"

mkdir -p "${RESULTS_DIR}"

log() { printf '\n=== %s\n' "$*"; }

# ---------------------------------------------------------------------------
# Build the voyager binary from THIS worktree and put it FIRST on PATH.
#
# HARD-WON: several sweep phases shell out to `yb-voyager`. Without this an older
# INSTALLED binary from /usr/local/bin is used instead and the run silently measures the
# wrong build - the verdicts look plausible and are about someone else's code.
# ---------------------------------------------------------------------------
build_voyager() {
    if [ "${SKIP_BUILD}" = "1" ]; then
        log "SKIP_BUILD=1, using whatever yb-voyager is already on PATH: $(command -v yb-voyager || echo none)"
        return
    fi
    local bindir="${RESULTS_DIR}/bin"
    mkdir -p "${bindir}"
    log "building yb-voyager from ${MODULE_DIR} into ${bindir}"
    ( cd "${MODULE_DIR}" && go build -o "${bindir}/yb-voyager" . )
    export PATH="${bindir}:${PATH}"
    log "yb-voyager on PATH: $(command -v yb-voyager)"
}

# ---------------------------------------------------------------------------
# FAILED_RUNS records every `go test` invocation that did not pass, so that a failing
# mode does not stop the other modes AND the script still exits non-zero at the end.
#
# Both halves matter. Continuing is right because a sweep's output IS its verdicts, and a
# mode that fails still produced them. But swallowing the exit code outright is how a
# guard whose entire purpose is to fail ends up reporting success: with `run_go_test`
# returning 0 unconditionally, `run-datatype-sweep.sh coverage` exited 0 even when the
# coverage guard failed listing missing types, so the cheap per-PR job would have gone
# green on exactly the failure it exists to catch. Record, continue, then propagate.
# ---------------------------------------------------------------------------
# A counter plus a newline-joined string rather than an array: `set -u` and bash 3.2
# (still the default /bin/bash on macOS) disagree about expanding an empty array.
FAILED_COUNT=0
FAILED_LIST=""

run_go_test() {
    local name="$1" selector="$2"
    log "running ${name}  (-run '${selector}')"
    set +e
    ( cd "${MODULE_DIR}" && go test -tags integration_live_migration ./src/testlivemigration/ \
        -run "${selector}" -timeout "${TEST_TIMEOUT}" -v ) 2>&1 | tee -a "${RUN_LOG}"
    local rc=${PIPESTATUS[0]}
    set -e
    if [ "${rc}" -ne 0 ]; then
        log "${name} exited ${rc} (verdicts are still recorded; check the control gate below)"
        FAILED_COUNT=$((FAILED_COUNT + 1))
        FAILED_LIST="${FAILED_LIST}  ${name} (exit ${rc})
"
    fi
    return 0
}

generate_catalog() {
    log "generating the probe catalog (the report's static half)"
    export SWEEP_CATALOG_OUT="${CATALOG_JSON}"
    export VOYAGER_COMMIT="$(git -C "${REPO_ROOT}" rev-parse --short HEAD 2>/dev/null || echo unknown)"
    run_go_test "probe catalog + report round-trip" \
        'TestDatatypeSweepCatalog|TestDatatypeReportCoversEveryProbe'
}

# finish is the single exit path. It runs after the artefacts are written, so a failing
# run still leaves its log, results and report behind to look at.
finish() {
    if [ "${FAILED_COUNT}" -eq 0 ]; then
        log "all runs passed"
        return 0
    fi
    log "FAILED (${FAILED_COUNT}):"
    printf '%s' "${FAILED_LIST}"
    echo
    echo "Artefacts are still in ${RESULTS_DIR}; the full output is in ${RUN_LOG}."
    return 1
}

collect_results() {
    log "collecting results"
    # Record-and-continue rather than abort: when a mode has already failed there may be
    # no PROBE-RESULT lines to collect, and dying here would hide the failure summary
    # that says WHY. The exit code is carried by finish() either way.
    if ! ( cd "${MODULE_DIR}" && go run ./src/testlivemigration/sweepreport collect \
        -log "${RUN_LOG}" \
        -out "${RESULTS_CSV}" \
        -catalog "${CATALOG_JSON}" \
        -commit "$(git -C "${REPO_ROOT}" rev-parse --short HEAD 2>/dev/null || echo unknown)" \
        -pg-version "${PG_VERSION}" \
        -yb-version "${YB_VERSION}" ); then
        FAILED_COUNT=$((FAILED_COUNT + 1))
        FAILED_LIST="${FAILED_LIST}  collecting results
"
        return 0
    fi

    log "generating the report rows from those results"
    if ! ( cd "${MODULE_DIR}" && go run ./src/testlivemigration/sweepreport report \
        -results "${RESULTS_CSV}" \
        -catalog "${CATALOG_JSON}" \
        -out "${RESULTS_DIR}/report-rows.json" \
        -csv "${RESULTS_DIR}/report-rows.csv" ); then
        FAILED_COUNT=$((FAILED_COUNT + 1))
        FAILED_LIST="${FAILED_LIST}  generating report rows
"
    fi

    # The control gate: a run whose known-good int/text controls did not come out WORKS
    # is an INVALID run and none of its verdicts may be published.
    if grep -qa '^PROBE-RUN-INVALID' "${RUN_LOG}"; then
        log "CONTROL GATE FAILED - these batches produced unusable verdicts:"
        grep -a '^PROBE-RUN-INVALID' "${RUN_LOG}" || true
        echo "Their rows are marked run_status=INVALID in ${RESULTS_CSV} and are excluded from diffs."
    fi
    if grep -qa '^PROBE-RUN-FLAKE' "${RUN_LOG}"; then
        log "flaked batches (re-run these before recording anything):"
        grep -a '^PROBE-RUN-FLAKE' "${RUN_LOG}" || true
    fi

    # A probe caught crash-looping the import channel. Its batch-mates are collateral:
    # their events were stuck behind it, so they were never measured and have to be
    # re-run WITHOUT it. The line names the probe and the command that measures it alone.
    if grep -qa '^PROBE-RUN-QUARANTINE' "${RUN_LOG}"; then
        log "QUARANTINED probes - each one wedged its batch; re-run the batch without it:"
        grep -a '^PROBE-RUN-QUARANTINE' "${RUN_LOG}" || true
    fi

    # The exporter died. Nothing migrated at all in that run, and `initiate cutover`
    # would have hung forever - the most severe outcome in the audit, and the one that
    # used to be reported as "inconclusive". The line quotes the cause and, when the
    # failure names a probe, which probe to measure on its own.
    if grep -qa '^PROBE-RUN-EXPORT-DIED' "${RUN_LOG}"; then
        log "EXPORTER DIED - nothing was produced in these batches:"
        grep -a '^PROBE-RUN-EXPORT-DIED' "${RUN_LOG}" || true
    fi

    # What every bounded wait actually cost, and why it ended. This is where the
    # crash-loop detector's saving is visible: a "repeating-error" line says how much of
    # the budget was not spent, and a "timeout" line is a stall that logged nothing.
    if grep -qa '^PROBE-WAIT' "${RUN_LOG}"; then
        log "wait accounting (elapsed / budget / why it ended):"
        grep -a '^PROBE-WAIT' "${RUN_LOG}" || true
    fi

    # Render the shareable page from those same rows. The page is a VIEW over the
    # results: if a verdict is not in the CSV it cannot appear on the page, and a
    # verdict from a run that failed its control gate is shown as discarded rather
    # than as a result. Python is optional - the JSON/CSV above are the real output.
    local page_dir="${MODULE_DIR}/src/testlivemigration/sweepreport/page"
    if command -v python3 >/dev/null 2>&1 && [ -f "${page_dir}/build_page.py" ]; then
        log "rendering the report page"
        if ! python3 "${page_dir}/build_page.py" \
            "${RESULTS_DIR}/report-rows.json" \
            "${RESULTS_DIR}/datatype-survival-map.html" \
            "${page_dir}/page_template.html"; then
            FAILED_COUNT=$((FAILED_COUNT + 1))
            FAILED_LIST="${FAILED_LIST}  rendering the report page
"
        fi
    else
        log "python3 not found - skipping the HTML page (JSON and CSV are still written)"
    fi

    log "artefacts"
    echo "  log      ${RUN_LOG}"
    echo "  results  ${RESULTS_CSV}"
    echo "  catalog  ${CATALOG_JSON}"
    echo "  report   ${RESULTS_DIR}/report-rows.json (+ .csv)"
    echo "  page     ${RESULTS_DIR}/datatype-survival-map.html"
}

usage() {
    sed -n '18,40p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    exit 2
}

# ---------------------------------------------------------------------------
main() {
    local what="${1:-default}"

    log "PG_VERSION=${PG_VERSION}  YB_VERSION=${YB_VERSION}"
    log "DEBEZIUM_DIST_DIR=${DEBEZIUM_DIST_DIR}  KUBECONFIG=${KUBECONFIG}"

    case "${what}" in
        -h|--help|help) usage ;;
    esac

    build_voyager
    generate_catalog

    case "${what}" in
        coverage)
            run_go_test "coverage guard" 'TestDatatypeCatalogCoverage'
            ;;
        offline)  run_go_test "offline sweep" 'TestDatatypeSweepOffline' ;;
        live)     run_go_test "live sweep" 'TestDatatypeSweepLive' ;;
        fallback|fall-back)
            run_go_test "fall-back sweep" 'TestDatatypeSweepFallback' ;;
        fallforward|fall-forward)
            run_go_test "fall-forward sweep" 'TestDatatypeSweepFallForward' ;;
        probe)
            local id="${2:-${PROBE_ID:-}}" mode="${3:-${PROBE_MODE:-LIVE}}"
            if [ -z "${id}" ]; then
                echo "usage: $0 probe <PROBE_ID> [OFFLINE|LIVE|FALL-BACK|FALL-FORWARD]" >&2
                exit 2
            fi
            export PROBE_ID="${id}" PROBE_MODE="${mode}"
            run_go_test "single probe ${id} in ${mode}" 'TestDatatypeSweepSuspect'
            ;;
        default)
            # The cheap, always-useful pair, plus the guard that keeps them honest.
            run_go_test "coverage guard" 'TestDatatypeCatalogCoverage'
            run_go_test "offline sweep" 'TestDatatypeSweepOffline'
            run_go_test "live sweep" 'TestDatatypeSweepLive'
            ;;
        all)
            run_go_test "coverage guard" 'TestDatatypeCatalogCoverage'
            run_go_test "offline sweep" 'TestDatatypeSweepOffline'
            run_go_test "live sweep" 'TestDatatypeSweepLive'
            run_go_test "fall-back sweep" 'TestDatatypeSweepFallback'
            run_go_test "fall-forward sweep" 'TestDatatypeSweepFallForward'
            ;;
        *)
            echo "unknown mode ${what}" >&2
            usage
            ;;
    esac

    # The coverage guard is a catalogue check, not a probe run: it never emits
    # PROBE-RESULT lines, so asking the collector for results would fail a run
    # that actually succeeded. Skip the collector - but still go through finish(),
    # or a guard that failed listing missing types would report success.
    if [ "${what}" = "coverage" ]; then
        log "coverage guard only - no probe results to collect"
        finish
        return
    fi

    collect_results
    finish
}

main "$@"
