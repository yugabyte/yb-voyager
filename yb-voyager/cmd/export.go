/*
Copyright (c) YugabyteDB, Inc.

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

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// source struct will be populated by CLI arguments parsing
var source srcdb.Source

// to disable progress bar during data export and import
var disablePb utils.BoolStr
var exportType string
var useDebezium bool
var runId string
var excludeTableListFilePath string
var tableListFilePath string

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "export schema and data from compatible source database(Oracle, MySQL, PostgreSQL)",
	Long:  `Export has various sub-commands i.e. export schema and export data to export from various compatible source databases(Oracle, MySQL, PostgreSQL).`,
}

func init() {
	rootCmd.AddCommand(exportCmd)
}

func registerCommonExportFlags(cmd *cobra.Command) {
	BoolVar(cmd.Flags(), &startClean, "start-clean", false,
		"cleans up the project directory for schema or data files depending on the export command")

	source.VerboseMode = bool(VerboseMode)
}

func registerSourceDBConnFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&source.DBType, "source-db-type", "",
		"source database type: (oracle, mysql, postgresql)\n")

	cmd.Flags().StringVar(&source.Host, "source-db-host", "localhost",
		"source database server host")

	cmd.Flags().IntVar(&source.Port, "source-db-port", -1,
		"source database server port number. Default: Oracle(1521), MySQL(3306), PostgreSQL(5432)")

	cmd.Flags().StringVar(&source.User, "source-db-user", "",
		"connect to source database as the specified user")

	// TODO: All sensitive parameters can be taken from the environment variable
	cmd.Flags().StringVar(&source.Password, "source-db-password", "",
		"source password to connect as the specified user")

	cmd.Flags().StringVar(&source.DBName, "source-db-name", "",
		"source database name to be migrated to YugabyteDB")

	cmd.Flags().StringVar(&source.DBSid, "oracle-db-sid", "",
		"[For Oracle Only] Oracle System Identifier (SID) that you wish to use while exporting data from Oracle instances")

	cmd.Flags().StringVar(&source.OracleHome, "oracle-home", "",
		"[For Oracle Only] Path to set $ORACLE_HOME environment variable. tnsnames.ora is found in $ORACLE_HOME/network/admin")

	cmd.Flags().StringVar(&source.TNSAlias, "oracle-tns-alias", "",
		"[For Oracle Only] Name of TNS Alias you wish to use to connect to Oracle instance. Refer to documentation to learn more about configuring tnsnames.ora and aliases")

	cmd.Flags().StringVar(&source.CDBName, "oracle-cdb-name", "",
		"[For Oracle Only] Oracle Container Database Name in case you are using a multitenant container database. Note: This is only required for live migration.")

	cmd.Flags().StringVar(&source.CDBSid, "oracle-cdb-sid", "",
		"[For Oracle Only] Oracle System Identifier (SID) of the Container Database that you wish to use while exporting data from Oracle instances.  Note: This is only required for live migration.")

	cmd.Flags().StringVar(&source.CDBTNSAlias, "oracle-cdb-tns-alias", "",
		"[For Oracle Only] Name of TNS Alias you wish to use to connect to Oracle Container Database in case you are using a multitenant container database. Refer to documentation to learn more about configuring tnsnames.ora and aliases. Note: This is only required for live migration.")

	cmd.Flags().StringVar(&source.Schema, "source-db-schema", "",
		"source schema name to export (valid for Oracle, PostgreSQL)\n"+
			"Note: in case of PostgreSQL, it can be a single or comma separated list of schemas")

	// TODO SSL related more args will come. Explore them later.
	cmd.Flags().StringVar(&source.SSLCertPath, "source-ssl-cert", "",
		"source SSL Certificate Path")

	cmd.Flags().StringVar(&source.SSLMode, "source-ssl-mode", "prefer",
		"specify the source SSL mode out of - disable, allow, prefer, require, verify-ca, verify-full. \nMySQL does not support 'allow' sslmode, and Oracle does not use explicit sslmode paramters.")

	cmd.Flags().StringVar(&source.SSLKey, "source-ssl-key", "",
		"source SSL Key Path")

	cmd.Flags().StringVar(&source.SSLRootCert, "source-ssl-root-cert", "",
		"source SSL Root Certificate Path")

	cmd.Flags().StringVar(&source.SSLCRL, "source-ssl-crl", "",
		"source SSL Root Certificate Revocation List (CRL)")
}

func registerTargetDBAsSourceConnFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&source.Host, "target-db-host", "127.0.0.1",
		"host on which the YugabyteDB server is running")

	cmd.Flags().IntVar(&source.Port, "target-db-port", -1,
		"port on which the YugabyteDB YSQL API is running")

	cmd.Flags().StringVar(&source.User, "target-db-user", "",
		"username with which to connect to the target YugabyteDB server")
	cmd.MarkFlagRequired("target-db-user")

	cmd.Flags().StringVar(&source.Password, "target-db-password", "",
		"password with which to connect to the target YugabyteDB server")

	cmd.Flags().StringVar(&source.DBName, "target-db-name", "",
		"name of the database on the target YugabyteDB server on which import needs to be done")

	cmd.Flags().StringVar(&source.Schema, "target-db-schema", "",
		"target schema name in YugabyteDB")

	// TODO: SSL related more args might come. Need to explore SSL part completely.
	cmd.Flags().StringVar(&source.SSLCertPath, "target-ssl-cert", "",
		"provide target SSL Certificate Path")

	cmd.Flags().StringVar(&source.SSLMode, "target-ssl-mode", "prefer",
		"specify the target SSL mode out of - disable, allow, prefer, require, verify-ca, verify-full")

	cmd.Flags().StringVar(&source.SSLKey, "target-ssl-key", "",
		"target SSL Key Path")

	cmd.Flags().StringVar(&source.SSLRootCert, "target-ssl-root-cert", "",
		"target SSL Root Certificate Path")

	cmd.Flags().StringVar(&source.SSLCRL, "target-ssl-crl", "",
		"target SSL Root Certificate Revocation List (CRL)")

	source.VerboseMode = bool(VerboseMode)
}

func setExportFlagsDefaults() {
	setSourceDefaultPort() //will set only if required
	setDefaultSSLMode()

	val, ok := os.LookupEnv("BETA_FAST_DATA_EXPORT")
	if ok {
		useDebezium = (val == "true" || val == "1" || val == "yes")
	}
}

func setSourceDefaultPort() {
	if source.Port != -1 {
		return
	}
	switch source.DBType {
	case ORACLE:
		source.Port = ORACLE_DEFAULT_PORT
	case POSTGRESQL:
		source.Port = POSTGRES_DEFAULT_PORT
	case YUGABYTEDB:
		source.Port = YUGABYTEDB_YSQL_DEFAULT_PORT
	case MYSQL:
		source.Port = MYSQL_DEFAULT_PORT
	}
}

func setDefaultSSLMode() {
	if source.SSLMode != "" {
		return
	}
	switch source.DBType {
	case MYSQL, POSTGRESQL:
		source.SSLMode = "prefer"
	}
}

func validateExportFlags(cmd *cobra.Command, exporterRole string) {
	validateExportDirFlag()
	validateSourceDBType()
	validateSourceSchema()
	validatePortRange()
	validateSSLMode()
	validateOracleParams()

	validateConflictsBetweenTableListFlags(source.TableList, source.ExcludeTableList)

	validateTableListFlag(source.TableList, "table-list")
	validateTableListFlag(source.ExcludeTableList, "exclude-table-list")

	if source.TableList == "" {
		source.TableList = validateAndExtractTableListFilePathFlag(tableListFilePath, "table-list-file-path")
	}
	if source.ExcludeTableList == "" {
		source.ExcludeTableList = validateAndExtractTableListFilePathFlag(excludeTableListFilePath, "exclude-table-list-file-path")
	}

	switch exporterRole {
	case SOURCE_DB_EXPORTER_ROLE:
		getAndStoreSourceDBPasswordInSourceConf(cmd)
	case TARGET_DB_EXPORTER_ROLE:
		getAndStoreTargetDBPasswordInSourceConf(cmd)
	}

	// checking if wrong flag is given used for a db type
	if source.DBType != ORACLE {
		if source.DBSid != "" {
			utils.ErrExit("Error: --oracle-db-sid flag is only valid for 'oracle' db type")
		}
		if source.OracleHome != "" {
			utils.ErrExit("Error: --oracle-home flag is only valid for 'oracle' db type")
		}
		if source.TNSAlias != "" {
			utils.ErrExit("Error: --oracle-tns-alias flag is only valid for 'oracle' db type")
		}
	}
}

func registerExportDataFlags(cmd *cobra.Command) {
	BoolVar(cmd.Flags(), &disablePb, "disable-pb", false,
		"true - to disable progress bar during data export and stats printing during streaming phase (default false)")

	cmd.Flags().StringVar(&source.ExcludeTableList, "exclude-table-list", "",
		"list of tables to exclude while exporting data (ignored if --table-list/--table-list-file-path is used)")

	cmd.Flags().StringVar(&source.TableList, "table-list", "",
		"list of the tables to export data")

	cmd.Flags().StringVar(&excludeTableListFilePath, "exclude-table-list-file-path", "",
		"path of the file for list of tables to exclude while exporting data (ignored if --table-list/--table-list-file-path is used)")

	cmd.Flags().StringVar(&tableListFilePath, "table-list-file-path", "",
		"path of the file for list of tables to export data")

	cmd.Flags().IntVar(&source.NumConnections, "parallel-jobs", 4,
		"number of Parallel Jobs to extract data from source database")

	cmd.Flags().StringVar(&exportType, "export-type", SNAPSHOT_ONLY,
		fmt.Sprintf("export type: %s, %s[TECH PREVIEW]", SNAPSHOT_ONLY, SNAPSHOT_AND_CHANGES))
}

func validateSourceDBType() {
	if source.DBType == "" {
		utils.ErrExit("Error: required flag \"source-db-type\" not set")
	}

	source.DBType = strings.ToLower(source.DBType)
	if !slices.Contains(supportedSourceDBTypes, source.DBType) {
		utils.ErrExit("Error: Invalid source-db-type: %q. Supported source db types are: (postgresql, oracle, mysql)", source.DBType)
	}
}

func validateConflictsBetweenTableListFlags(tableList string, excludeTableList string) {
	if tableList != "" && excludeTableList != "" {
		utils.ErrExit("Error: Only one of --table-list and --exclude-table-list are allowed")
	}

	if tableList != "" && tableListFilePath != "" {
		utils.ErrExit("Error: Only one of --table-list and --table-list-file-path are allowed")
	}
	if excludeTableList != "" && excludeTableListFilePath != "" {
		utils.ErrExit("Error: Only one of --exclude-table-list and --exclude-table-list-file-path are allowed")
	}

	if tableListFilePath != "" && excludeTableListFilePath != "" {
		utils.ErrExit("Error: Only one of --table-list-file-path and --exclude-table-list-file-path are allowed")
	}

	if tableList != "" && excludeTableListFilePath != "" {
		utils.ErrExit("Error: Only one of --table-list and --exclude-table-list-file-path are allowed")
	}

	if excludeTableList != "" && tableListFilePath != "" {
		utils.ErrExit("Error: Only one of --exclude-table-list and --table-list-file-path are allowed")
	}
}

func validateSourceSchema() {
	if source.Schema == "" {
		return
	}

	schemaList := utils.CsvStringToSlice(source.Schema)
	switch source.DBType {
	case MYSQL:
		utils.ErrExit("Error: --source-db-schema flag is not valid for 'MySQL' db type")
	case ORACLE:
		if len(schemaList) > 1 {
			utils.ErrExit("Error: single schema at a time is allowed to export from oracle. List of schemas provided: %s", schemaList)
		}
	case POSTGRESQL:
		// In PG, its supported to export more than one schema
		source.Schema = strings.Join(schemaList, "|") // clean and correct formatted for pg
	}
}

func validatePortRange() {
	if source.Port < 0 || source.Port > 65535 {
		utils.ErrExit("Error: Invalid port number %d. Valid range is 0-65535", source.Port)
	}
}

func validateSSLMode() {
	if source.DBType == ORACLE || slices.Contains(validSSLModes[source.DBType], source.SSLMode) {
		return
	} else {
		utils.ErrExit("Error: Invalid sslmode: %q. Valid SSL modes are %v", validSSLModes[source.DBType])
	}
}

func validateOracleParams() {
	if source.DBType != ORACLE {
		return
	}

	// in oracle, object names are stored in UPPER CASE by default(case insensitive)
	if !utils.IsQuotedString(source.Schema) {
		source.Schema = strings.ToUpper(source.Schema)
	}
	if source.DBName == "" && source.DBSid == "" && source.TNSAlias == "" {
		utils.ErrExit(`Error: one flag required out of "oracle-tns-alias", "source-db-name", "oracle-db-sid" required.`)
	} else if source.TNSAlias != "" {
		//Priority order for Oracle: oracle-tns-alias > source-db-name > oracle-db-sid
		utils.PrintAndLog("Using TNS Alias for export.")
		source.DBName = ""
		source.DBSid = ""
	} else if source.DBName != "" {
		utils.PrintAndLog("Using DB Name for export.")
		source.DBSid = ""
	} else if source.DBSid != "" {
		utils.PrintAndLog("Using SID for export.")
	}
	if source.IsOracleCDBSetup() {
		//Priority order for Oracle: oracle-tns-alias > source-db-name > oracle-db-sid
		if source.CDBTNSAlias != "" {
			//Priority order for Oracle: oracle-tns-alias > source-db-name > oracle-db-sid
			utils.PrintAndLog("Using CDB TNS Alias for export.")
			source.CDBName = ""
			source.CDBSid = ""
		} else if source.CDBName != "" {
			utils.PrintAndLog("Using CDB Name for export.")
			source.CDBSid = ""
		} else if source.CDBSid != "" {
			utils.PrintAndLog("Using CDB SID for export.")
		}
		if source.DBName == "" {
			utils.ErrExit(`Error: When using Container DB setup, specifying PDB via oracle-tns-alias or oracle-db-sid is not allowed. Please specify PDB name via source-db-name`)
		}
	}

}

func getAndStoreSourceDBPasswordInSourceConf(cmd *cobra.Command) {
	var err error
	source.Password, err = getPassword(cmd, "source-db-password", "SOURCE_DB_PASSWORD")
	if err != nil {
		utils.ErrExit("error in getting source-db-password: %v", err)
	}
}

func getAndStoreTargetDBPasswordInSourceConf(cmd *cobra.Command) {
	var err error
	source.Password, err = getPassword(cmd, "target-db-password", "TARGET_DB_PASSWORD")
	if err != nil {
		utils.ErrExit("error in getting target-db-password: %v", err)
	}
}

func markFlagsRequired(cmd *cobra.Command) {
	// mandatory for all
	cmd.MarkFlagRequired("source-db-type")
	cmd.MarkFlagRequired("source-db-user")

	switch source.DBType {
	case POSTGRESQL, ORACLE: // schema and database names are mandatory
		cmd.MarkFlagRequired("source-db-name")
		cmd.MarkFlagRequired("source-db-schema")
	case MYSQL:
		cmd.MarkFlagRequired("source-db-name")
	}
}

func validateExportTypeFlag() {
	exportType = strings.ToLower(exportType)
	if !slices.Contains(validExportTypes, exportType) {
		utils.ErrExit("Error: Invalid export-type: %q. Supported export types are: %s", exportType, validExportTypes)
	}
}

func saveExportTypeInMetaDB() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ExportType = exportType
	})
	if err != nil {
		utils.ErrExit("error while updating export type in meta db: %v", err)
	}
}
