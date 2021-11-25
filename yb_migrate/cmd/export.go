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

	"github.com/spf13/cobra"
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export has various sub-commands to extract schema, data and generate migration report",
	Long: `Export has various sub-commands to extract schema, data and generate migration report.
`,

	Run: func(cmd *cobra.Command, args []string) {
		//fmt.Printf("export called with command use = %s and args[0] = %s\n", cmd.Use, args[0])
		//exportGenerateReportCmd.Run(cmd, args)
		//exportSchemaCmd.Run(cmd, args)
		//exportDataCmd.Run(cmd, args)
		//time.Sleep(3 * time.Second)
		fmt.Printf("export called with source data type = %s\n", source.DBType)
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.PersistentFlags().StringVar(&source.DBType, "source-db-type", "",
		"source database type (Oracle/PostgreSQL/MySQL)")
	exportCmd.MarkPersistentFlagRequired("source-db-type")
	exportCmd.PersistentFlags().StringVar(&source.Host, "source-db-host", "localhost",
		"The host on which the source database is running")
	exportCmd.MarkPersistentFlagRequired("source-db-host")

	// TODO How to change defaults with the db type
	exportCmd.PersistentFlags().StringVar(&source.Port, "source-db-port", "",
		"The port on which the source database is running")

	exportCmd.PersistentFlags().StringVar(&source.User, "source-db-user", "",
		"The user with which the connection will be made to the source database")
	exportCmd.MarkPersistentFlagRequired("source-db-user")

	// TODO All sensitive parameters can be taken from the environment variable
	exportCmd.PersistentFlags().StringVar(&source.Password, "source-db-password", "",
		"The user with which the connection will be made to the source database")
	exportCmd.MarkPersistentFlagRequired("source-db-password")

	exportCmd.PersistentFlags().StringVar(&source.DBName, "source-db-name", "",
		"The source database which needs to be migrated to YugabyteDB")
	exportCmd.MarkPersistentFlagRequired("source-db-name")

	//out of schema and db-name one should be mandatory(oracle vs others)

	exportCmd.PersistentFlags().StringVar(&source.Schema, "source-db-schema", "",
		"The source schema which needs to be migrated to YugabyteDB")
	// exportCmd.MarkPersistentFlagRequired("source-db-schema")

	// TODO SSL related more args will come. Explore them later.
	exportCmd.PersistentFlags().StringVar(&source.SSLCert, "source-ssl-cert", "",
		"source database type (Oracle/PostgreSQL/MySQL")

	exportCmd.PersistentFlags().StringVar(&source.SSLMode, "source-ssl-mode", "disable",
		"source database type (Oracle/PostgreSQL/MySQL")

}
