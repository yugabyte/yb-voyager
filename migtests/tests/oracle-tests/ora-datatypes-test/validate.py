#!/usr/bin/env python3

import yb

def main():
	yb.run_checks({
		"MIGRATION_COMPLETED": migration_completed_checks,
	})


def migration_completed_checks(tgt, tag):
    tables = tgt.row_count_of_all_tables()
    assert tables.get("numeric_types_number") == 4
    assert tables.get("numeric_types_float") == 5
    assert tables.get("numeric_types_binary_float") == 5
    assert tables.get("numeric_types_binary_double") == 5
    assert tables.get("numeric_types") == 8
    assert tables.get("date_time_types") == 10
    assert tables.get("interval_types") == 4
    assert tables.get("CHAR_TYPES") == 6
    assert tables.get("LONG_TYPE") == 3
    assert tables.get("RAW_TYPE") == 6

if __name__ == "__main__":
	main()
