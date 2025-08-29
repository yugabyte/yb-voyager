#!/bin/bash

# Wait for Debezium queue to stabilize after applying changes
# This ensures all changes are captured before proceeding with cutover
# Usage: wait_for_debezium_queue_to_stabilize.sh <export_dir> <exporter_role> [timeout_seconds]

export_dir="$1"
exporter_role="$2"
timeout_seconds="${3:-60}"

if [ -z "$export_dir" ] || [ -z "$exporter_role" ]; then
    echo "Usage: $0 <export_dir> <exporter_role> [timeout_seconds]"
    exit 1
fi

queue_dir="${export_dir}/data/queue"
start_time=$(date +%s)
last_event_count=0
stable_iterations=0
required_stable_iterations=3  # Must be stable for 3 checks (9 seconds)

echo "Waiting for Debezium queue to stabilize for ${exporter_role}..."

while true; do
    current_time=$(date +%s)
    elapsed=$((current_time - start_time))
    
    if [ $elapsed -ge $timeout_seconds ]; then
        echo "Timeout reached (${timeout_seconds}s). Queue may not be fully stable."
        exit 0
    fi
    
    # Count events from the specific exporter role
    current_event_count=0
    if [ -d "$queue_dir" ]; then
        for file in "$queue_dir"/segment.*.ndjson; do
            if [ -f "$file" ] && [ -s "$file" ]; then
                # Use grep with explicit handling of empty results
                count=$(grep -c "\"exporter_role\":\"${exporter_role}\"" "$file" 2>/dev/null)
                if [ $? -eq 0 ]; then
                    # grep found matches or returned 0 (no matches but file processed)
                    current_event_count=$((current_event_count + count))
                fi
            fi
        done
    fi
    
    echo "Current event count for ${exporter_role}: ${current_event_count}"
    
    # Check if event count is stable
    if [ $current_event_count -eq $last_event_count ] && [ $current_event_count -gt 0 ]; then
        stable_iterations=$((stable_iterations + 1))
        echo "Queue stable for ${stable_iterations} iterations..."
        
        if [ $stable_iterations -ge $required_stable_iterations ]; then
            echo "Queue has stabilized with ${current_event_count} events from ${exporter_role}"
            exit 0
        fi
    else
        stable_iterations=0
        last_event_count=$current_event_count
    fi
    
    sleep 3
done
