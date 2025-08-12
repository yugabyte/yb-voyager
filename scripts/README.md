# YugabyteDB Tablet Information Script

This script retrieves detailed tablet leader information from a YugabyteDB cluster for a specific table or index.

## Features

- Connects to YugabyteDB cluster and discovers all nodes
- Scrapes tablet server UI (port 9000) from each node
- Retrieves tablet information from database system tables
- Combines information from multiple sources for comprehensive results
- Outputs results in a formatted table and optionally saves to CSV

## Requirements

- Python 3.7+
- Access to YugabyteDB cluster
- Network access to tablet server UI endpoints (port 9000)

## Installation

1. Install the required dependencies:
```bash
pip install -r requirements.txt
```

## Usage

### Basic Usage

```bash
python get_tablet_info.py \
    --host <node_ip> \
    --username <username> \
    --password <password> \
    --database <database_name> \
    --table <table_name>
```

### Complete Example

```bash
python get_tablet_info.py \
    --host 192.168.1.100 \
    --port 5433 \
    --username yugabyte \
    --password mypassword \
    --database mydb \
    --schema public \
    --table users \
    --output tablet_info.csv
```

### Parameters

- `--host`: YugabyteDB node host IP (required)
- `--port`: YugabyteDB port (default: 5433)
- `--username`: Database username (required)
- `--password`: Database password (required)
- `--database`: Database name (required)
- `--schema`: Schema name (default: public)
- `--table`: Table name or index name (required)
- `--output`: Output file path for CSV export (optional)

## Output

The script outputs a table with the following columns:

- **serial_no**: Sequential number for easy counting
- **node_host**: The IP address of the node hosting the tablet
- **tablet_id**: Unique identifier for the tablet (extracted from the tablet ID link)
- **partition_type**: Type of partitioning (hash/range) - parsed from partition column
- **partition_keys**: Partition key range for the tablet (e.g., "[0xB555, 0xB8E2]")
- **size**: Total size of data in the tablet (e.g., "6.76G", "1.00M") - extracted from "Total:" field
- **state**: Current state of the tablet (e.g., "RUNNING")

## How It Works

1. **Node Discovery**: Connects to the specified YugabyteDB node and queries `yb_servers()` to get all cluster nodes
2. **UI Scraping**: Visits `http://<node_ip>:9000/tablets` for each node to scrape tablet information
3. **Data Processing**: Processes and formats the scraped HTML data
4. **Output**: Displays results in a formatted table and optionally saves to CSV

## Notes

- The script requires network access to port 9000 on all cluster nodes
- Tablet information may vary between nodes due to replication
- The script includes a small delay between requests to avoid overwhelming the servers
- Size information is converted from human-readable format (KB, MB, GB) to bytes

## Troubleshooting

- **Connection errors**: Verify the host IP, port, and credentials
- **UI scraping errors**: Ensure port 9000 is accessible on all nodes
- **No results**: Verify the table name exists and is accessible
- **Permission errors**: Ensure the user has access to system tables and tablet server UI 