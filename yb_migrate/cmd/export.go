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
		fmt.Printf("Parent Export Commnad called with source data type = %s\n", source.DBType)
		exportSchemaAndData()
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)

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

	exportCmd.PersistentFlags().StringVar(&exportMode, "export mode", "offline",
		"offline | online")
}

func exportSchemaAndData() {

	switch source.DBType {
	case "oracle":
		exportSchema()
		exportData()
	case "postgres":
		exportSchema()
		exportData()
	case "mysql":
		exportSchema()
		exportData()
	default:
		fmt.Printf("Invalid source database type for export\n")
	}

}
