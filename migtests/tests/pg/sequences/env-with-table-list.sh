export SOURCE_DB_TYPE="postgresql"
export SOURCE_DB_NAME=${SOURCE_DB_NAME:-"pg_sequences"}
export SOURCE_DB_SCHEMA="public,schema1,schema3,schema4"
export EXPORT_TABLE_LIST="sequence_check1,sequence_check3,multiple_serial_columns,Case_Sensitive_Seq,schema1.sequence_check1,schema1.sequence_check2,schema1.multiple_identity_columns,foo,bar,schema3.foo,schema4.bar,London,sydney,users"