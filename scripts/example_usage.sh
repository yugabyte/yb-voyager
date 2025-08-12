#!/bin/bash

# Example usage of the YugabyteDB tablet information script
# You can configure the cluster parameters in two ways:
# 1. Set environment variables (recommended for production)
# 2. Modify the default values below (for testing)

echo "YugabyteDB Tablet Information Script - Example Usage"
echo "=================================================="
echo ""
echo "Using cluster parameters:"
echo "  Host: $HOST (set YB_HOST to override)"
echo "  Port: $PORT (set YB_PORT to override)"
echo "  Database: $DATABASE (set YB_DATABASE to override)"
echo "  Schema: $SCHEMA (set YB_SCHEMA to override)"
echo "  Table: $TABLE (set YB_TABLE to override)"
echo ""

# Set your cluster parameters here
# You can override these by setting environment variables
# Example: export YB_HOST="192.168.1.100" && export YB_TABLE="users" && ./example_usage.sh
HOST="${YB_HOST:-127.0.0.1}"
PORT="${YB_PORT:-5433}"
USERNAME="${YB_USERNAME:-yugabyte}"
PASSWORD="${YB_PASSWORD:-yugabyte}"
DATABASE="${YB_DATABASE:-test}"
SCHEMA="${YB_SCHEMA:-public}"
TABLE="${YB_TABLE:-table_name}"

# Run the script
echo "Running tablet info script for table: $TABLE"
echo ""

python3 get_tablet_info.py \
    --host "$HOST" \
    --port "$PORT" \
    --username "$USERNAME" \
    --password "$PASSWORD" \
    --database "$DATABASE" \
    --schema "$SCHEMA" \
    --table "$TABLE" \
    --output "tablet_info_${TABLE}.csv"

echo ""
echo "Script completed. Check the output above and the CSV file if specified." 