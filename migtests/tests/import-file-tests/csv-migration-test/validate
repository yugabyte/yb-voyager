#!/usr/bin/env python3

import yb

def main():
	yb.run_checks({
		"FILE_IMPORT_DONE": file_import_done_checks,
	})


def file_import_done_checks(tgt, tag):
	row_count = tgt.get_row_count("csv_table")
	print(f"row_count: {row_count}")
	assert row_count == 41715

if __name__ == "__main__":
	main()
