#!/usr/bin/env python3

import argparse
import psycopg2
import requests
from bs4 import BeautifulSoup
import pandas as pd
import sys
from typing import List, Dict, Any
import re
from urllib.parse import urljoin
import time
import urllib3

# Disable SSL warnings since we're using verify=False
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)


class YugabyteTabletInfo:
    def __init__(self, host: str, port: int, username: str, password: str, database: str, schema: str):
        self.host = host
        self.port = port
        self.username = username
        self.password = password
        self.database = database
        self.schema = schema
        self.connection = None

    def connect(self):
        """Establish connection to YugabyteDB"""
        try:
            self.connection = psycopg2.connect(
                host=self.host,
                port=self.port,
                user=self.username,
                password=self.password,
                database=self.database
            )
            print(f"Connected to YugabyteDB at {self.host}:{self.port}")
        except Exception as e:
            print(f"Error connecting to database: {e}")
            sys.exit(1)

    def get_all_nodes(self) -> List[str]:
        """Get all node IPs from the cluster"""
        try:
            cursor = self.connection.cursor()
            cursor.execute("SELECT host FROM yb_servers();")
            nodes = [row[0] for row in cursor.fetchall()]
            cursor.close()
            print(f"Found {len(nodes)} nodes in the cluster: {nodes}")
            return nodes
        except Exception as e:
            print(f"Error getting nodes: {e}")
            return []

    def get_table_tablets(self, table_name: str) -> List[Dict[str, Any]]:
        """Get tablet information for a specific table from database system tables"""
        try:
            cursor = self.connection.cursor()
            
            # Note: This is a placeholder. Actual tablet information is obtained via UI scraping
            # as YugabyteDB doesn't expose tablet details through standard SQL queries
            cursor.close()
            return []
        except Exception as e:
            print(f"Error getting tablet information: {e}")
            return []

    def scrape_tablet_ui(self, node_ip: str, table_name: str) -> List[Dict[str, Any]]:
        """Scrape tablet information from the tablet server UI"""
        try:
            url = f"http://{node_ip}:9000/tablets"
            print(f"Scraping tablet UI from {url}")
            
            # Disable SSL verification and use HTTP
            response = requests.get(url, timeout=10, verify=False)
            response.raise_for_status()
            
            soup = BeautifulSoup(response.content, 'html.parser')
            tablets = []
            
            # Find the table containing tablet information
            table = soup.find('table')
            if not table:
                print(f"No table found on {url}")
                return tablets
            
            rows = table.find_all('tr')[1:]  # Skip header row
            
            for row in rows:
                cells = row.find_all('td')
                if len(cells) >= 11:  # Ensure we have all expected columns
                    # Extract tablet information from the row
                    namespace = cells[0].get_text(strip=True)
                    table_name_cell = cells[1].get_text(strip=True)
                    tablet_id_link = cells[3].find('a')
                    tablet_id = tablet_id_link.get_text(strip=True) if tablet_id_link else cells[3].get_text(strip=True)
                    partition_info = cells[4].get_text(strip=True)
                    state = cells[5].get_text(strip=True)
                    size_info = cells[8]  # On-disk size column
                    raft_config = cells[9]  # RaftConfig column
                    
                    # Only process tablets for the specified table (exact match)
                    if table_name.lower() == table_name_cell.lower():
                        # Check if this node is the leader for this tablet
                        is_leader = self.is_tablet_leader(raft_config, node_ip)
                        
                        # Only include if this node is the leader
                        if is_leader:
                            # Parse partition information
                            partition_type, partition_keys = self.parse_partition_info(partition_info)
                            
                                                    # Parse size information
                        size_str = self.parse_size_info(size_info)
                        
                        tablet_info = {
                                'node_host': node_ip,
                                'tablet_id': tablet_id,
                                'partition_type': partition_type,
                                'partition_keys': partition_keys,
                                'size': size_str,  # Use original size string instead of bytes
                                'state': state
                            }
                        tablets.append(tablet_info)
            
            return tablets
        except requests.exceptions.RequestException as e:
            print(f"Error scraping {node_ip}:9000/tablets: {e}")
            return []
        except Exception as e:
            print(f"Error parsing tablet UI from {node_ip}: {e}")
            return []

    def parse_partition_info(self, partition_info: str) -> tuple[str, str]:
        """Parse partition information to extract type and keys"""
        if not partition_info:
            return "unknown", ""
        
        # Examples from the HTML:
        # "hash_split: [0xB555, 0xB8E2]"
        # "range: [<start>, DocKey([], [0]))"
        
        if "hash_split:" in partition_info:
            partition_type = "hash"
            # Extract the range part after "hash_split:"
            keys_part = partition_info.split("hash_split:", 1)[1].strip()
            partition_keys = keys_part
        elif "range:" in partition_info:
            partition_type = "range"
            # Extract the range part after "range:"
            keys_part = partition_info.split("range:", 1)[1].strip()
            partition_keys = keys_part
        else:
            partition_type = "unknown"
            partition_keys = partition_info
        
        return partition_type, partition_keys

    def is_tablet_leader(self, raft_config_element, node_ip: str) -> bool:
        """Check if the given node is the leader for this tablet"""
        try:
            # Look for the LEADER entry in the RaftConfig
            leader_li = raft_config_element.find('li', string=lambda text: text and 'LEADER:' in text)
            if leader_li:
                leader_text = leader_li.get_text(strip=True)
                # Extract the IP from "LEADER: <ip>"
                leader_ip = leader_text.split('LEADER:', 1)[1].strip()
                return leader_ip == node_ip
            return False
        except Exception as e:
            print(f"Error checking leader status: {e}")
            return False

    def parse_size_info(self, size_element) -> str:
        """Parse the complex size information from the On-disk size column"""
        try:
            # The size information is in a <ul> with multiple <li> elements
            # We want the "Total:" value
            total_li = size_element.find('li', string=lambda text: text and 'Total:' in text)
            if total_li:
                total_text = total_li.get_text(strip=True)
                # Extract the size part after "Total:"
                size_str = total_text.split('Total:', 1)[1].strip()
                print(f"DEBUG: Found size '{size_str}'")
                return size_str
            else:
                # Fallback: try to get any text content
                size_text = size_element.get_text(strip=True)
                print(f"DEBUG: Size element text: {size_text}")
                # Look for patterns like "Total: 6.27M" in the text
                import re
                total_match = re.search(r'Total:\s*([\d.]+[KMGT]?B?)', size_text)
                if total_match:
                    size_str = total_match.group(1)
                    print(f"DEBUG: Fallback found size '{size_str}'")
                    return size_str
                return "0B"
        except Exception as e:
            print(f"Error parsing size info: {e}")
            return "0B"

    def parse_size_to_bytes(self, size_str: str) -> int:
        """Convert human-readable size string to bytes"""
        if not size_str or size_str.strip() == '':
            return 0
        
        size_str = size_str.strip().upper()
        
        # Extract number and unit
        match = re.match(r'([\d.]+)\s*([KMGT]?B?)', size_str)
        if not match:
            return 0
        
        number = float(match.group(1))
        unit = match.group(2)
        
        multipliers = {
            'B': 1,
            'KB': 1024,
            'MB': 1024**2,
            'GB': 1024**3,
            'TB': 1024**4
        }
        
        return int(number * multipliers.get(unit, 1))

    def get_tablet_info(self, table_name: str) -> pd.DataFrame:
        """Get comprehensive tablet information for a table"""
        print(f"Getting tablet information for table: {table_name}")
        
        # Get all nodes in the cluster
        nodes = self.get_all_nodes()
        if not nodes:
            print("No nodes found in the cluster")
            return pd.DataFrame()
        
        all_tablets = []
        
        # Scrape tablet information from each node
        for node_ip in nodes:
            print(f"Processing node: {node_ip}")
            tablets = self.scrape_tablet_ui(node_ip, table_name)
            all_tablets.extend(tablets)
            
            # Small delay to avoid overwhelming the servers
            time.sleep(0.5)
        
        # All tablet information comes from UI scraping
        combined_tablets = all_tablets
        
        # Create DataFrame
        df = pd.DataFrame(combined_tablets)
        
        if not df.empty:
            # Sort by tablet_id for better readability
            df = df.sort_values('tablet_id').reset_index(drop=True)
            # Add serial number column
            df.insert(0, 'serial_no', range(1, len(df) + 1))
        
        return df

    def close(self):
        """Close database connection"""
        if self.connection:
            self.connection.close()
            print("Database connection closed")


def main():
    parser = argparse.ArgumentParser(description='Get tablet information from YugabyteDB cluster')
    parser.add_argument('--host', required=True, help='YugabyteDB node host IP')
    parser.add_argument('--port', type=int, default=5433, help='YugabyteDB port (default: 5433)')
    parser.add_argument('--username', required=True, help='Database username')
    parser.add_argument('--password', required=True, help='Database password')
    parser.add_argument('--database', required=True, help='Database name')
    parser.add_argument('--schema', default='public', help='Schema name (default: public)')
    parser.add_argument('--table', required=True, help='Table name (or index name)')
    parser.add_argument('--output', help='Output file path (CSV format)')
    
    args = parser.parse_args()
    
    # Create YugabyteTabletInfo instance
    yb_info = YugabyteTabletInfo(
        host=args.host,
        port=args.port,
        username=args.username,
        password=args.password,
        database=args.database,
        schema=args.schema
    )
    
    try:
        # Connect to database
        yb_info.connect()
        
        # Get tablet information
        df = yb_info.get_tablet_info(args.table)
        
        if df.empty:
            print(f"No tablet information found for table: {args.table}")
        else:
            print(f"\nFound {len(df)} tablets for table: {args.table}")
            print("\nTablet Information:")
            print("=" * 80)
            print(df.to_string(index=False))
            
            # Save to CSV if output file specified
            if args.output:
                df.to_csv(args.output, index=False)
                print(f"\nTablet information saved to: {args.output}")
    
    except KeyboardInterrupt:
        print("\nOperation cancelled by user")
    except Exception as e:
        print(f"Error: {e}")
    finally:
        yb_info.close()


if __name__ == "__main__":
    main() 