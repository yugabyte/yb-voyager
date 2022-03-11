package cmd

const (
	FOUR_MB                     = 4 * 1024 * 1024
	SPLIT_FILE_CHANNEL_SIZE     = 200
	META_INFO_DIR_NAME          = "metainfo"
	NEWLINE                     = '\n'
	GET_SERVERS_QUERY           = "SELECT * FROM yb_servers()"
	ORACLE_DEFAULT_PORT         = "1521"
	MYSQL_DEFAULT_PORT          = "3306"
	POSTGRES_DEFAULT_PORT       = "5432"
	YUGABYTEDB_DEFAULT_PORT     = "5433"
	YUGABYTEDB_DEFAULT_DATABASE = "yugabyte"
	ORACLE                      = "oracle"
	MYSQL                       = "mysql"
	POSTGRESQL                  = "postgresql"
	LAST_SPLIT_NUM              = 0
)

var IMPORT_SESSION_SETTERS = []string{"" +
	"SET client_encoding TO 'UTF8';",
	"SET yb_disable_transactional_writes to true;", //to enable Single Row Transactions which results in high-performance with YugabyteDB
	"SET session_replication_role TO replica;",     //disable triggers or fkeys constraint checks
}

var supportedSourceDBTypes = []string{ORACLE, MYSQL, POSTGRESQL}
