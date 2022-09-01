/*
Copyright (c) YugaByte, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

const (
	FOUR_MB                     = 4 * 1024 * 1024
	SPLIT_FILE_CHANNEL_SIZE     = 20
	META_INFO_DIR_NAME          = "metainfo"
	NEWLINE                     = '\n'
	GET_SERVERS_QUERY           = "SELECT * FROM yb_servers()"
	ORACLE_DEFAULT_PORT         = 1521
	MYSQL_DEFAULT_PORT          = 3306
	POSTGRES_DEFAULT_PORT       = 5432
	YUGABYTEDB_DEFAULT_PORT     = 5433
	YUGABYTEDB_DEFAULT_DATABASE = "yugabyte"
	YUGABYTEDB_DEFAULT_SCHEMA   = "public"
	ORACLE                      = "oracle"
	MYSQL                       = "mysql"
	POSTGRESQL                  = "postgresql"
	LAST_SPLIT_NUM              = 0
	SPLIT_INFO_PATTERN          = "[0-9]*.[0-9]*.[0-9]*.[0-9]*"
	LAST_SPLIT_PATTERN          = "0.[0-9]*.[0-9]*.[0-9]*"
	COPY_MAX_RETRY_COUNT        = 10
	MAX_SLEEP_SECOND            = 60
)

// import session parameters
const (
	SET_CLIENT_ENCODING_TO_UTF8           = "SET client_encoding TO 'UTF8'"
	SET_SESSION_REPLICATE_ROLE_TO_REPLICA = "SET session_replication_role TO replica" //Disable triggers or fkeys constraint checks.
	SET_YB_ENABLE_UPSERT_MODE             = "SET yb_enable_upsert_mode to true"
	SET_YB_DISABLE_TRANSACTIONAL_WRITES   = "SET yb_disable_transactional_writes to true" // Disable transactions to improve ingestion throughput.
)

var supportedSourceDBTypes = []string{ORACLE, MYSQL, POSTGRESQL}

var NonRetryCopyErrors = []string{
	"Sending too long RPC message",
	"invalid input syntax",
	"violates unique constraint",
}
