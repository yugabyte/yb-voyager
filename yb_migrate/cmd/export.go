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
	"yb_migrate/migrationutil"

	"github.com/spf13/cobra"
)

var source migrationutil.Source

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "export schema and data from compatible source database(Oracle, Mysql, Postgres)",
	Long: `Export has various sub-commands to extract schema, data and generate migration report.
`,

	Run: func(cmd *cobra.Command, args []string) {
		//fmt.Printf("export called with command use = %s and args[0] = %s\n", cmd.Use, args[0])
		//exportGenerateReportCmd.Run(cmd, args)
		//exportSchemaCmd.Run(cmd, args)
		//exportDataCmd.Run(cmd, args)
		//time.Sleep(3 * time.Second)
		fmt.Printf("Parent Export Commnad called with source data type = %s\n", source.DBType)
		exportSchema()
		exportData()
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.PersistentFlags().StringVarP(&ExportDir, "export-dir", "e", ".",
		"export directory (default is current working directory") //default value is current dir

	exportCmd.PersistentFlags().StringVar(&source.DBType, "source-db-type", "",
		"source database type (Oracle/PostgreSQL/MySQL)")
	exportCmd.MarkPersistentFlagRequired("source-db-type")
	exportCmd.PersistentFlags().StringVar(&source.Host, "source-db-host", "localhost",
		"source database server host")
	// exportCmd.MarkPersistentFlagRequired("source-db-host")

	exportCmd.PersistentFlags().StringVar(&source.Port, "source-db-port", "",
		"source database server port number")

	exportCmd.PersistentFlags().StringVar(&source.User, "source-db-user", "",
		"connect to source database as specified user")
	exportCmd.MarkPersistentFlagRequired("source-db-user")

	// TODO: All sensitive parameters can be taken from the environment variable
	exportCmd.PersistentFlags().StringVar(&source.Password, "source-db-password", "",
		"connect to source as specified user")
	exportCmd.MarkPersistentFlagRequired("source-db-password")

	exportCmd.PersistentFlags().StringVar(&source.DBName, "source-db-name", "",
		"source database name to be migrated to YugabyteDB")
	exportCmd.MarkPersistentFlagRequired("source-db-name")

	//out of schema and db-name one should be mandatory(oracle vs others)

	exportCmd.PersistentFlags().StringVar(&source.Schema, "source-db-schema", "",
		"source schema name which needs to be migrated to YugabyteDB")
	// exportCmd.MarkPersistentFlagRequired("source-db-schema")

	// TODO SSL related more args will come. Explore them later.
	exportCmd.PersistentFlags().StringVar(&source.SSLCert, "source-ssl-cert", "",
		"provide Source SSL Certificate Path")

	exportCmd.PersistentFlags().StringVar(&source.SSLMode, "source-ssl-mode", "disable",
		"specify the source SSL mode out of - disable, allow, prefer, require, verify-ca, verify-full")

	exportCmd.PersistentFlags().StringVar(&MigrationMode, "migration-mode", "offline",
		"mode can be offline | online(applicable only for data migration)")
}
