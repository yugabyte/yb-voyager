/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

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
	"yb_migrate/src/utils"

	"github.com/spf13/cobra"
)

var source utils.Source

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "export schema and data from compatible source database(Oracle, Mysql, Postgres)",
	Long: `Export has various sub-commands to extract schema, data and generate migration report.
`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().PersistentPreRun(cmd.Parent(), args)

		checkSourceDBType()
		setSourceDefaultPort() //will set only if required

		//marking flags as required based on conditions
		cmd.MarkPersistentFlagRequired("source-db-type")
		if source.Uri == "" { //if uri is not given
			cmd.MarkPersistentFlagRequired("source-db-user")
			cmd.MarkPersistentFlagRequired("source-db-password")
			cmd.MarkPersistentFlagRequired("source-db-name")
			if source.DBType == ORACLE { //default is public
				cmd.MarkPersistentFlagRequired("source-db-schema")
			}
		} else {
			//check and parse the source
			source.ParseURI()
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
		fmt.Sprintf("source database type: %s\n", allowedSourceDBTypes))

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

	//out of schema and db-name one should be mandatory(oracle vs others)

	exportCmd.PersistentFlags().StringVar(&source.Schema, "source-db-schema", "public",
		"source schema name which needs to be migrated to YugabyteDB")

	// TODO SSL related more args will come. Explore them later.
	exportCmd.PersistentFlags().StringVar(&source.SSLCertPath, "source-ssl-cert", "",
		"provide Source SSL Certificate Path")

	exportCmd.PersistentFlags().StringVar(&source.SSLMode, "source-ssl-mode", "disable",
		"specify the source SSL mode out of - disable, allow, prefer, require, verify-ca, verify-full")

	exportCmd.PersistentFlags().StringVar(&migrationMode, "migration-mode", "offline",
		"mode can be offline | online(applicable only for data migration)")

	exportCmd.PersistentFlags().IntVar(&source.NumConnections, "parallel-jobs", 1,
		"number of Parallel Jobs to extract data from source database[Note: this is only for export data command not export schema command]")

	exportCmd.PersistentFlags().BoolVar(&startClean, "start-clean", false,
		"delete all existing files inside the project if present and start fresh")

	exportCmd.PersistentFlags().StringVar(&source.Uri, "source-db-uri", "",
		`URI for connecting to the source database
		format:
			1. Oracle:	oracle://User=username;Password=password@hostname:port/SID
			2. MySQL:	mysql://user:password@host:port/database
			3. PostgreSQL:	postgresql://user:password@host:port/dbname?sslmode=mode
		`)

}

func checkSourceDBType() {
	if source.DBType == "" {
		fmt.Printf("requried flag: source-db-type\n")
		os.Exit(1)
	}
	for _, sourceDBType := range allowedSourceDBTypes {
		if sourceDBType == source.DBType {
			return //if matches any allowed type
		}
	}

	fmt.Printf("invalid source db type: %s\n", source.DBType)
	fmt.Printf("allowed source db types are: %s\n", allowedSourceDBTypes)
	os.Exit(1)
}

func setSourceDefaultPort() {
	if source.Port != "" {
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
