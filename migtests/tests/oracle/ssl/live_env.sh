export SOURCE_DB_TYPE="oracle"
export SOURCE_DB_ORACLE_CDB_TNS_ALIAS="ORCLCDBSSL"
export SOURCE_DB_SCHEMA=${SOURCE_DB_SCHEMA:-"TEST_SCHEMA"}
export TARGET_DB_NAME=${TARGET_DB_NAME:-"ssl_test"}
export SOURCE_REPLICA_DB_ORACLE_TNS_ALIAS="ORCLPDB1SSL"