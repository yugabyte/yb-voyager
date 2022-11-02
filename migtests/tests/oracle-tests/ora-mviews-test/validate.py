#!/usr/bin/env python3

import yb

def main():
	yb.run_checks(migration_completed_checks)

#=============================================================================

def migration_completed_checks(tgt):
	table_list = tgt.get_table_names("public")
	print("table_list:", table_list)
	assert len(table_list) == 1

	mv_list = tgt.get_objects_of_type_materialized_views("public")
	print("mv_list:", mv_list)
	assert len(mv_list) == 1

	returned_row_count = tgt.get_row_count("public")
	assert returned_row_count == 998

if __name__ == "__main__":
	main()
