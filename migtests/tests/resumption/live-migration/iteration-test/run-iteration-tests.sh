#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load environment variables
if [ -f "$SCRIPT_DIR/.env" ]; then
    set -a
    source "$SCRIPT_DIR/.env"
    set +a
fi

ORCHESTRATOR="$SCRIPT_DIR/../../../../scripts/live-migration-resumption/orchestrator.py"
SCENARIO="$SCRIPT_DIR/scenario.yaml"

echo "=== Running iteration tests ==="
echo "Orchestrator: $ORCHESTRATOR"
echo "Scenario: $SCENARIO"
echo ""

stdbuf -oL python3 -u "$ORCHESTRATOR" "$SCENARIO"
