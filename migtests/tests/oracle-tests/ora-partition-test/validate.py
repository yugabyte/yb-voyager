#!/usr/bin/env python3

import yb

def main():
	yb.run_checks({
		"MIGRATION_COMPLETED": migration_completed_checks,
	})

EXPECTED_ROW_COUNT = {
	'ACCOUNTS_LIST_PARTITIONED': 0,
	'ACCOUNTS_LIST_PARTITIONED_P_NORTHCENTRAL': 153926,
	'ACCOUNTS_LIST_PARTITIONED_P_NORTHEAST': 230330,
	'ACCOUNTS_LIST_PARTITIONED_P_NORTHWEST': 115509,
	'ACCOUNTS_LIST_PARTITIONED_P_SOUTHCENTRAL': 115687,
	'ACCOUNTS_LIST_PARTITIONED_P_SOUTHEAST': 153907,
	'ACCOUNTS_LIST_PARTITIONED_P_SOUTHWEST': 230641,
	'ORDERS_INTERVAL_PARTITION': 0,
	'ORDERS_INTERVAL_PARTITION_INTERVAL_PARTITION_LESS_THAN_2015': 1,
	'ORDERS_INTERVAL_PARTITION_INTERVAL_PARTITION_LESS_THAN_2016': 13,
    'ORDERS_INTERVAL_PARTITION_INTERVAL_PARTITION_LESS_THAN_2015': 79,
	'ORDERS_INTERVAL_PARTITION_INTERVAL_PARTITION_LESS_THAN_2016': 12,
    'ORDER_ITEMS_RANGE_PARTITIONED': 0,
	'ORDER_ITEMS_RANGE_PARTITIONED_P1': 49,
	'ORDER_ITEMS_RANGE_PARTITIONED_P2': 20,
    'ORDER_ITEMS_RANGE_PARTITIONED_P3': 10,
	'SALES_HASH': 0,
    'SALES_HASH_P1': 315607,
    'SALES_HASH_P2': 526679,
    'SALES_HASH_P3': 473453,
    'SALES_HASH_P4': 684261
}

def migration_completed_checks(tgt, tag):
    table_list = tgt.get_table_names("public")
	print("table_list:", table_list)
	assert len(table_list) == 21
    
    returned_row_count = tgt.row_count_of_all_tables("public")
	for table_name, row_count in EXPECTED_ROW_COUNT.items():
		print(f"table_name: {table_name}, row_count: {returned_row_count[table_name]}")
		assert row_count == returned_row_count[table_name]

if __name__ == "__main__":
	main()
