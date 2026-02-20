export SOURCE_DB_TYPE="postgresql"
export SOURCE_DB_NAME=${SOURCE_DB_NAME:-"case_sensitive_reserved_words"}
export SOURCE_DB_SCHEMA="public,schema2,order,pg-schema,\"Schema\""
export SOURCE_DB_NAME=${SOURCE_DB_NAME:-"Case_Sensitive_Reserved_Words-src"}
export TARGET_DB_NAME=${TARGET_DB_NAME:-"case_sensitive_reserved_words-tgt"}
export SOURCE_REPLICA_DB_NAME=${SOURCE_REPLICA_DB_NAME:-"Case_Sensitive_Reserved_Words-src-replica"}