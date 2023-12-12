export SOURCE_DB_TYPE="mysql"
export SOURCE_DB_NAME=${SOURCE_DB_NAME:-"table_list_test"}
export TARGET_DB_SCHEMA="table_list_test"
export EXPORT_TABLE_LIST=${EXPORT_TABLE_LIST:-'"order","user","group","check",reserved_column,"*Case*",session_log*'}
export EXPORT_EX_TABLE_LIST=${EXPORT_EX_TABLE_LIST:-'session_log4,"check"'}
export IMPORT_TABLE_LIST=${IMPORT_TABLE_LIST:-'"*er","group",reserved_column,*case*,session_log*'}
export IMPORT_EX_TABLE_LIST=${IMPORT_EX_TABLE_LIST:-'"user",session_log3'}
