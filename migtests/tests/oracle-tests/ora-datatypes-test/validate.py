#!/usr/bin/env python3

import yb

def main():
	yb.run_checks({
		"FILE_IMPORT_DONE": file_import_done_checks,
	})


def file_import_done_checks(tgt, tag):
	row_count_numeric = tgt.get_row_count("numeric_types_number")
    row_count_float = tgt.get_row_count("numeric_types_float")
    row_count_binary_float = tgt.get_row_count("numeric_types_binary_float")
    row_count_numeric_binary_double = tgt.get_row_count("numeric_types_binary_double")
    row_count_numeric_types = tgt.get_row_count("numeric_types")
    row_count_date_time = tgt.get_row_count("date_time_types")
    row_count_interval_types = tgt.get_row_count("interval_types")
    row_count_char_types = tgt.get_row_count("CHAR_TYPES")
    row_count_long_type = tgt.get_row_count("LONG_TYPE")
    row_count_raw_type = tgt.get_row_count("RAW_TYPE")
    assert row_count_numeric == 5
    assert row_count_float == 5
    assert row_count_binary_float == 5
    assert row_count_numeric_binary_double == 5
    assert row_count_numeric_types == 8
    assert row_count_date_time == 10
    assert row_count_interval_types == 4
    assert row_count_char_types == 5
    assert row_count_long_type == 5
    assert row_count_raw_type == 6

if __name__ == "__main__":
	main()
