export SOURCE_DB_TYPE="mysql"
export SOURCE_DB_NAME=${SOURCE_DB_NAME:-"table_list_file_path"}
export TARGET_DB_SCHEMA="table_list_file_path"
export EXPORT_TABLE_LIST_FILE_PATH=${EXPORT_TABLE_LIST_FILE_PATH:-$TEST_DIR/export-table-list.txt}
export EXPORT_EX_TABLE_LIST_FILE_PATH=${EXPORT_EX_TABLE_LIST_FILE_PATH:-$TEST_DIR/export-ex-table-list.txt}
export IMPORT_TABLE_LIST_FILE_PATH=${IMPORT_TABLE_LIST_FILE_PATH:-$TEST_DIR/import-table-list.txt}
export IMPORT_EX_TABLE_LIST_FILE_PATH=${IMPORT_EX_TABLE_LIST_FILE_PATH:-$TEST_DIR/import-ex-table-list.txt}