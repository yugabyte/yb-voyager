# We use an already deployed Oracle instance on docker for testing.

export SOURCE_DB_PORT=${SOURCE_DB_PORT:-1521}
export SOURCE_DB_USER=${SOURCE_DB_USER:-"sakila"}
export SOURCE_DB_PASSWORD=${SOURCE_DB_PASSWORD:-"password"}
export SOURCE_DB_NAME=${SOURCE_DB_NAME:-"DMS"}
export SOURCE_DB_HOST=${SOURCE_DB_HOST:-"ssinghal-dms-oracle.cbtcvpszcgdq.us-west-2.rds.amazonaws.com"}
export SOURCE_DB_USER_SCHEMA_OWNER=${SOURCE_DB_USER_SCHEMA_OWNER:-"dt"}
export SOURCE_DB_USER_SCHEMA_OWNER_PASSWORD=${SOURCE_DB_USER_SCHEMA_OWNER_PASSWORD:-"dt"}
export SOURCE_DB_USER_SYS=${SOURCE_DB_USER_SYS:-"SYS"}
export SOURCE_DB_USER_SYS_PASSWORD=${SOURCE_DB_USER_SYS_PASSWORD:-"Password1"}
