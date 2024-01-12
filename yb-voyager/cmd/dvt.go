package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"golang.org/x/exp/slices"
)

var dvtColumnCommand = &cobra.Command{
	Use:   "gen-dvt-commands",
	Short: "gen-dvt-commands",

	PreRun: func(cmd *cobra.Command, args []string) {
		if utils.IsDirectoryEmpty(exportDir) {
			utils.ErrExit("export directory is empty")
		}
	},

	Run: dvtColumnCommandFn,
}

func dvtColumnCommandFn(cmd *cobra.Command, args []string) {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("error while getting migration status: %w\n", err)
	}
	if msr.SourceDBConf == nil {
		utils.ErrExit("msr SourceDBConf is empty")
	}
	if msr.TargetDBConf == nil {
		utils.ErrExit("msr TargetDBConf is empty")
	}
	if msr.FallForwardEnabled && msr.SourceReplicaDBConf == nil {
		utils.ErrExit("msr SourceReplicaDBConf is empty")
	}
	getTargetPassword(cmd)
	msr.TargetDBConf.Password = tconf.Password
	getSourceDBPassword(cmd)
	msr.SourceDBConf.Password = tconf.Password
	if msr.FallForwardEnabled {
		getSourceReplicaDBPassword(cmd)
		msr.SourceReplicaDBConf.Password = tconf.Password
	}

	sqlname.SourceDBType = msr.SourceDBConf.DBType

	// Connections
	utils.PrintAndLog("CONNECTIONS:")
	sourceConnName := "voyager_source"
	sourceDBConnCmd := fmt.Sprintf("python -m data_validation connections add --connection-name %s Postgres --host %s  --port %d --user %s --password %s --database %s",
		sourceConnName, msr.SourceDBConf.Host, msr.SourceDBConf.Port, msr.SourceDBConf.User, msr.SourceDBConf.Password, msr.SourceDBConf.DBName)

	utils.PrintAndLog(sourceDBConnCmd)

	targetConnName := "voyager_target"
	targetDBConnCmd := fmt.Sprintf("python -m data_validation connections add --connection-name %s Postgres --host %s  --port %d --user %s --password %s --database %s",
		targetConnName, msr.TargetDBConf.Host, msr.TargetDBConf.Port, msr.TargetDBConf.User, msr.TargetDBConf.Password, msr.TargetDBConf.DBName)

	utils.PrintAndLog(targetDBConnCmd)
	if msr.FallForwardEnabled {
		sourceReplicaConnName := "voyager_source_replica"
		sourceReplicaDBConnCmd := fmt.Sprintf("python -m data_validation connections add --connection-name %s Postgres --host %s  --port %d --user %s --password %s --database %s",
			sourceReplicaConnName, msr.SourceReplicaDBConf.Host, msr.SourceReplicaDBConf.Port, msr.SourceReplicaDBConf.User, msr.SourceReplicaDBConf.Password, msr.SourceReplicaDBConf.DBName)

		utils.PrintAndLog(sourceReplicaDBConnCmd)
	}

	tdb = tgtdb.NewTargetDB(msr.TargetDBConf)
	err = tdb.Init()
	if err != nil {
		utils.ErrExit("failed to initialize the target DB: %w", err)
	}
	defer tdb.Finalize()

	tableColumnListForColumn := map[string]string{}
	tableColumnListForRow := map[string]string{}
	tablePkListForRow := map[string]string{}

	sourceTableList := msr.TableListExportedFromSource
	for _, qualifiedTableName := range sourceTableList {
		tableName := sqlname.NewSourceNameFromQualifiedName(qualifiedTableName)

		columnListQuery := `
		select STRING_AGG(c.column_name,',') columnlist
		from information_schema.columns c  WHERE c.table_schema='%s' AND c.table_name='%s' 
		AND c.data_type != 'USER-DEFINED';
		`
		// utils.PrintAndLog("query-%s", fmt.Sprintf(columnListQuery, tableName.SchemaName.Unquoted, tableName.ObjectName.Unquoted))
		res := tdb.QueryRow(fmt.Sprintf(columnListQuery, tableName.SchemaName.Unquoted, tableName.ObjectName.Unquoted))
		var columnList string
		err := res.Scan(&columnList)
		if err != nil {
			utils.ErrExit("query column list: %v", err)
		}
		tableColumnListForColumn[qualifiedTableName] = columnList

		pkListQuery := `
		SELECT STRING_AGG(c.column_name,',')
		FROM information_schema.table_constraints tc
		JOIN information_schema.constraint_column_usage AS ccu USING (constraint_schema, constraint_name)
		JOIN information_schema.columns AS c ON c.table_schema = tc.constraint_schema
		  AND tc.table_name = c.table_name AND ccu.column_name = c.column_name
		WHERE constraint_type = 'PRIMARY KEY' and tc.table_schema='%s' AND tc.table_name = '%s';
		`
		res = tdb.QueryRow(fmt.Sprintf(pkListQuery, tableName.SchemaName.Unquoted, tableName.ObjectName.Unquoted))
		var pkList string
		err = res.Scan(&pkList)
		if err != nil {
			utils.ErrExit("query pk list: %v", err)
		}
		tablePkListForRow[qualifiedTableName] = pkList

		columnListQueryForRow := `
		select STRING_AGG(c.column_name,',') columnlist
		from information_schema.columns c  WHERE c.table_schema='%s' AND c.table_name='%s' 
		AND c.data_type NOT IN ('USER-DEFINED','json','jsonb');
		`
		// utils.PrintAndLog("query-%s", fmt.Sprintf(columnListQuery, tableName.SchemaName.Unquoted, tableName.ObjectName.Unquoted))
		res = tdb.QueryRow(fmt.Sprintf(columnListQueryForRow, tableName.SchemaName.Unquoted, tableName.ObjectName.Unquoted))
		var columnListForRow string
		err = res.Scan(&columnListForRow)
		if err != nil {
			utils.ErrExit("query column list for row: %v", err)
		}
		pkListColumns := strings.Split(pkList, ",")
		rowColumns := strings.Split(columnListForRow, ",")
		var finalColumnsForRow []string
		for _, columnName := range rowColumns {
			if slices.Contains(pkListColumns, columnName) {
				continue
			}
			finalColumnsForRow = append(finalColumnsForRow, columnName)
		}
		tableColumnListForRow[qualifiedTableName] = strings.Join(finalColumnsForRow, ",")
	}

	utils.PrintAndLog("########################################## SOURCE-TARGET #################################################")
	utils.PrintAndLog("")
	utils.PrintAndLog("VALIDATE COLUMN:")
	for tableName, columnList := range tableColumnListForColumn {
		validateColumnCmd := fmt.Sprintf("python -m data_validation validate column --source-conn voyager_source --target-conn voyager_target --tables-list '%s' --count '%s' --sum '%s' --wildcard-include-string-len --format csv",
			tableName, columnList, columnList)
		utils.PrintAndLog(validateColumnCmd)
	}

	utils.PrintAndLog("")
	utils.PrintAndLog("VALIDATE ROW:")
	for tableName, columnList := range tableColumnListForRow {
		validateRowCmd := fmt.Sprintf("python -m data_validation validate row --source-conn voyager_source --target-conn voyager_target --tables-list '%s'  -pk '%s' -comp-fields '%s'  --filter-status fail --format csv",
			tableName, tablePkListForRow[tableName], columnList)
		utils.PrintAndLog(validateRowCmd)
	}

	if msr.FallForwardEnabled {
		utils.PrintAndLog("########################################## SOURCE-SOURCE_REPLICA #################################################")
		utils.PrintAndLog("")
		utils.PrintAndLog("VALIDATE COLUMN:")
		for tableName, columnList := range tableColumnListForColumn {
			validateColumnCmd := fmt.Sprintf("python -m data_validation validate column --source-conn voyager_source --target-conn voyager_source_replica --tables-list '%s' --count '%s' --sum '%s' --wildcard-include-string-len --format csv",
				tableName, columnList, columnList)
			utils.PrintAndLog(validateColumnCmd)
		}

		utils.PrintAndLog("")
		utils.PrintAndLog("VALIDATE ROW:")
		for tableName, columnList := range tableColumnListForRow {
			validateRowCmd := fmt.Sprintf("python -m data_validation validate row --source-conn voyager_source --target-conn voyager_source_replica --tables-list '%s'  -pk '%s' -comp-fields '%s'  --filter-status fail --format csv",
				tableName, tablePkListForRow[tableName], columnList)
			utils.PrintAndLog(validateRowCmd)
		}

		utils.PrintAndLog("########################################## TARGET-SOURCE_REPLICA #################################################")
		utils.PrintAndLog("")
		utils.PrintAndLog("VALIDATE COLUMN:")
		for tableName, columnList := range tableColumnListForColumn {
			validateColumnCmd := fmt.Sprintf("python -m data_validation validate column --source-conn voyager_target --target-conn voyager_source_replica --tables-list '%s' --count '%s' --sum '%s' --wildcard-include-string-len --format csv",
				tableName, columnList, columnList)
			utils.PrintAndLog(validateColumnCmd)
		}

		utils.PrintAndLog("")
		utils.PrintAndLog("VALIDATE ROW:")
		for tableName, columnList := range tableColumnListForRow {
			validateRowCmd := fmt.Sprintf("python -m data_validation validate row --source-conn voyager_target --target-conn voyager_source_replica --tables-list '%s'  -pk '%s' -comp-fields '%s'  --filter-status fail --format csv",
				tableName, tablePkListForRow[tableName], columnList)
			utils.PrintAndLog(validateRowCmd)
		}
	}

}

func init() {
	rootCmd.AddCommand(dvtColumnCommand)
	registerExportDirFlag(dvtColumnCommand)
	dvtColumnCommand.Flags().StringVar(&sourceReplicaDbPassword, "source-replica-db-password", "",
		"password with which to connect to the target Source-Replica DB server. Alternatively, you can also specify the password by setting the environment variable SOURCE_REPLICA_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")

	dvtColumnCommand.Flags().StringVar(&sourceDbPassword, "source-db-password", "",
		"password with which to connect to the target source DB server. Alternatively, you can also specify the password by setting the environment variable SOURCE_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes")

	dvtColumnCommand.Flags().StringVar(&targetDbPassword, "target-db-password", "",
		"password with which to connect to the target YugabyteDB server. Alternatively, you can also specify the password by setting the environment variable TARGET_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")
}
