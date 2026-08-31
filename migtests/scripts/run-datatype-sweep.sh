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
export YB_VERSION="${YB_VERSION:-2025.2.1.0}"

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
# Run one `go test` selector, appending to the run log. A failing mode must not stop the
# other modes: a sweep's output IS its verdicts, and a mode that fails still produced them.
# ---------------------------------------------------------------------------
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
    fi
    return 0
}

generate_catalog() {
    log "generating the probe catalog (the report's static half)"
    ( cd "${MODULE_DIR}" && SWEEP_CATALOG_OUT="${CATALOG_JSON}" \
        VOYAGER_COMMIT="$(git -C "${REPO_ROOT}" rev-parse --short HEAD 2>/dev/null || echo unknown)" \
        go test -tags integration_live_migration ./src/testlivemigration/ \
        -run 'TestDatatypeSweepCatalog|TestDatatypeReportCoversEveryProbe' -v ) 2>&1 | tee -a "${RUN_LOG}"
}

collect_results() {
    log "collecting results"
    ( cd "${MODULE_DIR}" && go run ./src/testlivemigration/sweepreport collect \
        -log "${RUN_LOG}" \
        -out "${RESULTS_CSV}" \
        -catalog "${CATALOG_JSON}" \
        -commit "$(git -C "${REPO_ROOT}" rev-parse --short HEAD 2>/dev/null || echo unknown)" \
        -pg-version "${PG_VERSION}" \
        -yb-version "${YB_VERSION}" )

    log "generating the report rows from those results"
    ( cd "${MODULE_DIR}" && go run ./src/testlivemigration/sweepreport report \
        -results "${RESULTS_CSV}" \
        -catalog "${CATALOG_JSON}" \
        -out "${RESULTS_DIR}/report-rows.json" \
        -csv "${RESULTS_DIR}/report-rows.csv" )

    # The control gate: a run whose known-good int/text controls did not come out WORKS
    # is an INVALID run and none of its verdicts may be published.
    if grep -q '^PROBE-RUN-INVALID' "${RUN_LOG}"; then
        log "CONTROL GATE FAILED - these batches produced unusable verdicts:"
        grep '^PROBE-RUN-INVALID' "${RUN_LOG}" || true
        echo "Their rows are marked run_status=INVALID in ${RESULTS_CSV} and are excluded from diffs."
    fi
    if grep -q '^PROBE-RUN-FLAKE' "${RUN_LOG}"; then
        log "flaked batches (re-run these before recording anything):"
        grep '^PROBE-RUN-FLAKE' "${RUN_LOG}" || true
    fi

    log "artefacts"
    echo "  log      ${RUN_LOG}"
    echo "  results  ${RESULTS_CSV}"
    echo "  catalog  ${CATALOG_JSON}"
    echo "  report   ${RESULTS_DIR}/report-rows.json (+ .csv)"
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
    # that actually succeeded.
    if [ "${what}" = "coverage" ]; then
        log "coverage guard only - no probe results to collect"
        return 0
    fi

    collect_results
}

main "$@"
