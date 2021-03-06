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

import (
	"fmt"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/exp/slices"

	"github.com/spf13/cobra"
)

// source struct will be populated by CLI arguments parsing
var source srcdb.Source

// to disable progress bar during data export and import
var disablePb bool

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "export schema and data from compatible source database(Oracle, Mysql, Postgres)",
	Long: `Export has various sub-commands to extract schema, data and generate migration report.
`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().PersistentPreRun(cmd.Parent(), args)
		setExportFlagsDefaults()
		validateExportFlags()
		markFlagsRequired(cmd)
	},

	Run: func(cmd *cobra.Command, args []string) {
		// log.Infof("parent export command called with source data type = %s", source.DBType)
		checkDataDirs()
		exportSchema()
		exportData()
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)

	registerCommonExportFlags(exportCmd)

	exportCmd.PersistentFlags().StringVar(&source.TableList, "table-list", "",
		"list of the tables to export data(Note: works only for export data command)")

	exportCmd.PersistentFlags().StringVar(&migrationMode, "migration-mode", "offline",
		"mode can be offline | online(applicable only for data migration)")

	exportCmd.PersistentFlags().IntVar(&source.NumConnections, "parallel-jobs", 1,
		"number of Parallel Jobs to extract data from source database")

	exportCmd.Flags().BoolVar(&disablePb, "disable-pb", false,
		"true - to disable progress bar during data export (default false)")
}

func registerCommonExportFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&source.DBType, "source-db-type", "",
		fmt.Sprintf("source database type: %s\n", supportedSourceDBTypes))

	cmd.PersistentFlags().StringVar(&source.Host, "source-db-host", "localhost",
		"source database server host")

	cmd.PersistentFlags().IntVar(&source.Port, "source-db-port", -1,
		"source database server port number")

	cmd.PersistentFlags().StringVar(&source.User, "source-db-user", "",
		"connect to source database as specified user")

	// TODO: All sensitive parameters can be taken from the environment variable
	cmd.PersistentFlags().StringVar(&source.Password, "source-db-password", "",
		"connect to source as specified user")

	cmd.PersistentFlags().StringVar(&source.DBName, "source-db-name", "",
		"source database name to be migrated to YugabyteDB")

	cmd.PersistentFlags().StringVar(&source.DBSid, "oracle-db-sid", "",
		"[For Oracle Only] Oracle System Identifier (SID) that you wish to use while exporting data from Oracle instances")

	cmd.PersistentFlags().StringVar(&source.OracleHome, "oracle-home", "",
		"[For Oracle Only] Path to set $ORACLE_HOME environment variable. tnsnames.ora is found in $ORACLE_HOME/network/admin")

	cmd.PersistentFlags().StringVar(&source.TNSAlias, "oracle-tns-alias", "",
		"[For Oracle Only] Name of TNS Alias you wish to use to connect to Oracle instance. Refer to documentation to learn more about configuring tnsnames.ora and aliases")

	//out of schema and db-name one should be mandatory(oracle vs others)

	cmd.PersistentFlags().StringVar(&source.Schema, "source-db-schema", "",
		"source schema name which needs to be migrated to YugabyteDB (valid for Oracle, PostgreSQL)\n"+
			"Note: in case of PostgreSQL, it can be a single or comma separated list of schemas")

	// TODO SSL related more args will come. Explore them later.
	cmd.PersistentFlags().StringVar(&source.SSLCertPath, "source-ssl-cert", "",
		"provide Source SSL Certificate Path")

	cmd.PersistentFlags().StringVar(&source.SSLMode, "source-ssl-mode", "prefer",
		"specify the source SSL mode out of - disable, allow, prefer, require, verify-ca, verify-full. \nMySQL does not support 'allow' sslmode, and Oracle does not use explicit sslmode paramters.")

	cmd.PersistentFlags().StringVar(&source.SSLKey, "source-ssl-key", "",
		"provide SSL Key Path")

	cmd.PersistentFlags().StringVar(&source.SSLRootCert, "source-ssl-root-cert", "",
		"provide SSL Root Certificate Path")

	cmd.PersistentFlags().StringVar(&source.SSLCRL, "source-ssl-crl", "",
		"provide SSL Root Certificate Revocation List (CRL)")

	cmd.PersistentFlags().BoolVar(&startClean, "start-clean", false,
		"clean the project's data directory for already existing files before start(Note: works only for export data command)")
}

func setExportFlagsDefaults() {
	setSourceDefaultPort() //will set only if required
	setDefaultSSLMode()
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

func validateExportFlags() {
	validateExportDirFlag()
	validateSourceDBType()
	validateSourceSchema()
	validatePortRange()
	validateSSLMode()
	validateOracleParams()
	validateTableListFlag(source.TableList)

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

func validateSourceDBType() {
	if source.DBType == "" {
		utils.ErrExit("Error: required flag \"source-db-type\" not set")
	}

	source.DBType = strings.ToLower(source.DBType)
	if !slices.Contains(supportedSourceDBTypes, source.DBType) {
		utils.ErrExit("Error: Invalid source-db-type: %q. Supported source db types are: %s", source.DBType, supportedSourceDBTypes)
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

}

func markFlagsRequired(cmd *cobra.Command) {
	// mandatory for all
	cmd.MarkPersistentFlagRequired("source-db-type")
	cmd.MarkPersistentFlagRequired("source-db-user")
	cmd.MarkPersistentFlagRequired("source-db-password")

	switch source.DBType {
	case POSTGRESQL, ORACLE: // schema and database names are mandatory
		cmd.MarkPersistentFlagRequired("source-db-name")
		cmd.MarkPersistentFlagRequired("source-db-schema")
	case MYSQL:
		cmd.MarkPersistentFlagRequired("source-db-name")
	}
}
