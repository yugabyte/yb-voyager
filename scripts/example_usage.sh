#!/bin/bash

# Example usage of the YugabyteDB tablet information script
# Modify the parameters below according to your cluster configuration

echo "YugabyteDB Tablet Information Script - Example Usage"
echo "=================================================="

# Set your cluster parameters here
HOST="127.0.0.1"
PORT="5433"
USERNAME="yugabyte"
PASSWORD="yugabyte"
DATABASE="test"
SCHEMA="public"
TABLE="table_name"

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