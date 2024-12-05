package testcontainers

import (
	_ "embed"
)

const (
	DEFAULT_PG_PORT     = "5432"
	DEFAULT_YB_PORT     = "5433"
	DEFAULT_ORACLE_PORT = "1521"
	DEFAULT_MYSQL_PORT  = "3306"

	POSTGRESQL = "postgresql"
	YUGABYTEDB = "yugabytedb"
	ORACLE     = "oracle"
	MYSQL      = "mysql"
)

//go:embed test_schemas/postgresql_schema.sql
var postgresInitSchemaFile []byte

//go:embed test_schemas/oracle_schema.sql
var oracleInitSchemaFile []byte

//go:embed test_schemas/mysql_schema.sql
var mysqlInitSchemaFile []byte

//go:embed test_schemas/yugabytedb_schema.sql
var yugabytedbInitSchemaFile []byte
