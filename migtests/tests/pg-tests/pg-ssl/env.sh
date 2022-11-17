export SOURCE_DB_TYPE="postgresql"
export SOURCE_DB_NAME=${SOURCE_DB_NAME:-"pg_ssl"}
export SOURCE_DB_SCHEMA="public"
export SOURCE_SSL_MODE=${SOURCE_SSL_MODE:-"require"}    # Only for PG SSL testing.