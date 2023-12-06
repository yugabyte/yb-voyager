export SOURCE_DB_TYPE="oracle"
export SOURCE_DB_SCHEMA=${SOURCE_DB_SCHEMA:-"TEST_SCHEMA"}
export TARGET_DB_NAME=${TARGET_DB_NAME:-"table_list_test"}
export EXPORT_TABLE_LIST=${EXPORT_TABLE_LIST:-'session_log,session_log?,"group","check",test*,"*Case*",c*'}
export EXPORT_EX_TABLE_LIST=${EXPORT_EX_TABLE_LIST:-'session_log2,reserved_column'}
export IMPORT_TABLE_LIST=${IMPORT_TABLE_LIST:-'session_log,session_log?,"group",test*,*case*,c,c?'}
export IMPORT_EX_TABLE_LIST=${IMPORT_EX_TABLE_LIST:-'session_log1,"check",c1'}