#!/usr/bin/env bash
# Runs golangci-lint over every build-tag group, mirroring .github/workflows/lint.yml.
# Run from anywhere; lints the yb-voyager module. Requires golangci-lint on PATH
# (pin: versions/ci-config.json -> golangci_lint). Install:
#   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
#     | sh -s -- -b "$(go env GOPATH)/bin" "$(jq -r .versions.golangci_lint versions/ci-config.json)"
set -u

cd "$(dirname "$0")/.."

TAG_GROUPS=(
  ""
  "unit"
  "integration"
  "integration_voyager_command"
  "integration_live_migration"
  "issues_integration"
  "failpoint_export"
  "failpoint_import"
  "failpoint_cutover"
  "cdc_benchmark"
  "manual"
  "yb_version_latest_stable"
  "connector_latest_stable"
)

failed=0
for tags in "${TAG_GROUPS[@]}"; do
  echo "==> golangci-lint run ${tags:+--build-tags $tags}"
  golangci-lint run --max-issues-per-linter 0 --max-same-issues 0 ${tags:+--build-tags "$tags"} ./... || failed=1
done

exit $failed
