package cmd

const (
	FOUR_MB                 = 4 * 1024 * 1024
	SPLIT_FILE_CHANNEL_SIZE = 200
	META_INFO_DIR_NAME      = "metainfo"
	NEWLINE                 = '\n'
	YSQL                    = "/home/centos/code/yugabyte-db/bin/ysqlsh" //"/Users/kneeraj/code/yugabyte-db/bin/ysqlsh"
	GET_SERVERS_QUERY       = "SELECT * FROM yb_servers()"
)

var IMPORT_SESSION_SETTERS = []string{"" +
	"SET client_encoding TO 'UTF8';",
}
