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
	"os"
	"strings"

	"github.com/yugabyte/ybm/yb_migrate/src/utils"

	"github.com/spf13/cobra"
)

// source struct will be populated by CLI arguments parsing
var source utils.Source

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "export schema and data from compatible source database(Oracle, Mysql, Postgres)",
	Long: `Export has various sub-commands to extract schema, data and generate migration report.
`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().PersistentPreRun(cmd.Parent(), args)

		checkSourceDBType()
		setSourceDefaultPort() //will set only if required
		checkOrSetDefaultSSLMode()

		//marking flags as required based on conditions
		cmd.MarkPersistentFlagRequired("source-db-type")
		if source.Uri == "" { //if uri is not given
			cmd.MarkPersistentFlagRequired("source-db-user")
			cmd.MarkPersistentFlagRequired("source-db-password")
			if source.DBType != ORACLE {
				cmd.MarkPersistentFlagRequired("source-db-name")
			} else if source.DBType == ORACLE {
				cmd.MarkPersistentFlagRequired("source-db-schema")
				// in oracle, object names are stored in UPPER CASE by default(case insensitive)
				if !utils.InDoubleQuotes(source.Schema) {
					source.Schema = strings.ToUpper(source.Schema)
				}

				//TODO: set up an internal priority order in case 2 or more flags are specified for Oracle
				if source.DBName != "" || source.DBSid != "" {
					//no issues, continue with export of schema/data
				} else {
					cmd.MarkPersistentFlagRequired("oracle-tns-admin")
				}
			}
		} else {
			//check and parse the source
			source.ParseURI()
		}

		if source.TableList != "" {
			checkTableListFlag()
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		// log.Infof("parent export command called with source data type = %s", source.DBType)

		exportSchema()
		exportData()
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.PersistentFlags().StringVarP(&exportDir, "export-dir", "e", ".",
		"export directory (default is current working directory") //default value is current dir

	exportCmd.PersistentFlags().StringVar(&source.DBType, "source-db-type", "",
		fmt.Sprintf("source database type: %s\n", supportedSourceDBTypes))

	exportCmd.PersistentFlags().StringVar(&source.Host, "source-db-host", "localhost",
		"source database server host")

	exportCmd.PersistentFlags().StringVar(&source.Port, "source-db-port", "",
		"source database server port number")

	exportCmd.PersistentFlags().StringVar(&source.User, "source-db-user", "",
		"connect to source database as specified user")

	// TODO: All sensitive parameters can be taken from the environment variable
	exportCmd.PersistentFlags().StringVar(&source.Password, "source-db-password", "",
		"connect to source as specified user")

	exportCmd.PersistentFlags().StringVar(&source.DBName, "source-db-name", "",
		"source database name to be migrated to YugabyteDB")

	exportCmd.PersistentFlags().StringVar(&source.DBSid, "oracle-db-sid", "",
		"[For Oracle Only] Oracle System Identifier (SID) that you wish to use while exporting data from Oracle instances")

	exportCmd.PersistentFlags().StringVar(&source.OracleHome, "oracle-home", "",
		"[For Oracle Only] Path to set $ORACLE_HOME environment variable. tnsnames.ora is found in $ORACLE_HOME/network/admin")

	exportCmd.PersistentFlags().StringVar(&source.TNSAlias, "oracle-tns-alias", "",
		"[For Oracle Only] Name of TNS Alias you wish to use to connect to Oracle instance. Refer to documentation to learn more about configuring tnsnames.ora and aliases")

	//out of schema and db-name one should be mandatory(oracle vs others)

	exportCmd.PersistentFlags().StringVar(&source.Schema, "source-db-schema", "",
		"source schema name which needs to be migrated to YugabyteDB")

	// TODO SSL related more args will come. Explore them later.
	exportCmd.PersistentFlags().StringVar(&source.SSLCertPath, "source-ssl-cert", "",
		"provide Source SSL Certificate Path")

	exportCmd.PersistentFlags().StringVar(&source.SSLMode, "source-ssl-mode", "prefer",
		"specify the source SSL mode out of - disable, allow, prefer, require, verify-ca, verify-full. \nMySQL does not support 'allow' sslmode, and Oracle does not use explicit sslmode paramters.")

	exportCmd.PersistentFlags().StringVar(&source.SSLKey, "source-ssl-key", "",
		"provide SSL Key Path")

	exportCmd.PersistentFlags().StringVar(&source.SSLRootCert, "source-ssl-root-cert", "",
		"provide SSL Root Certificate Path")

	exportCmd.PersistentFlags().StringVar(&source.SSLCRL, "source-ssl-crl", "",
		"provide SSL Root Certificate Revocation List (CRL)")

	exportCmd.PersistentFlags().StringVar(&migrationMode, "migration-mode", "offline",
		"mode can be offline | online(applicable only for data migration)")

	exportCmd.PersistentFlags().IntVar(&source.NumConnections, "parallel-jobs", 1,
		"number of Parallel Jobs to extract data from source database[Note: applicable only for export data command]")

	exportCmd.PersistentFlags().StringVar(&source.Uri, "source-db-uri", "",
		`URI for connecting to the source database
		format:
			1. Oracle:	user/password@//host:port:SID	OR
					user/password@//host:port/service_name	OR
					user/password@TNS_alias
			2. MySQL:	mysql://[user[:[password]]@]host[:port][/dbname][?sslmode=mode&sslcert=cert_path...]
			3. PostgreSQL:	postgresql://[user[:[password]]@]host[:port][/dbname][?sslmode=mode&sslcert=cert_path...]
			
		`)

	exportCmd.PersistentFlags().BoolVar(&startClean, "start-clean", false,
		"clean the project's data directory for already existing files before start(Note: works only for export data command)")

	exportCmd.PersistentFlags().StringVar(&source.TableList, "table-list", "",
		"list of the tables to export data(Note: works only for export data command)")
}

func checkSourceDBType() {
	if source.DBType == "" {
		fmt.Printf("requried flag: source-db-type\n")
		fmt.Printf("supported source db types are: %s\n", supportedSourceDBTypes)
		os.Exit(1)
	}
	for _, sourceDBType := range supportedSourceDBTypes {
		if sourceDBType == source.DBType {
			return //if matches any allowed type
		}
	}

	fmt.Printf("invalid source db type: %s\n", source.DBType)
	fmt.Printf("supported source db types are: %s\n", supportedSourceDBTypes)
	os.Exit(1)
}

func setSourceDefaultPort() {
	if source.Port == "" {
		switch source.DBType {
		case ORACLE:
			source.Port = ORACLE_DEFAULT_PORT
		case POSTGRESQL:
			source.Port = POSTGRES_DEFAULT_PORT
		case MYSQL:
			source.Port = MYSQL_DEFAULT_PORT
		}
	}
}

func checkOrSetDefaultSSLMode() {

	switch source.DBType {
	case MYSQL:
		if source.SSLMode == "disable" || source.SSLMode == "prefer" || source.SSLMode == "require" || source.SSLMode == "verify-ca" || source.SSLMode == "verify-full" {
			return
		} else if source.SSLMode == "" {
			source.SSLMode = "prefer"
		} else {
			fmt.Printf("Invalid sslmode\nValid sslmodes are: disable, prefer, require, verify-ca, verify-full\n")
			os.Exit(1)
		}
	case POSTGRESQL:
		if source.SSLMode == "disable" || source.SSLMode == "allow" || source.SSLMode == "prefer" || source.SSLMode == "require" || source.SSLMode == "verify-ca" || source.SSLMode == "verify-full" {
			return
		} else if source.SSLMode == "" {
			source.SSLMode = "prefer"
		} else {
			fmt.Printf("Invalid sslmode\nValid sslmodes are: disable, allow, prefer, require, verify-ca, verify-full\n")
			os.Exit(1)
		}
	}
}
