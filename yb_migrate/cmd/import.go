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
	"yb_migrate/migrationutil"

	"github.com/spf13/cobra"
)

var importMode string
var target migrationutil.Target

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import",
	Short: "import schema and data from compatible source database(Oracle, Mysql, Postgres)",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		if StartClean != "NO" && StartClean != "YES" {
			fmt.Printf("Invalid value of flag start-clean as '%s'\n", StartClean)
			os.Exit(1)
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Parent Import Command called")
		importSchema()
		// importData()
	},
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.PersistentFlags().StringVarP(&ExportDir, "export-dir", "e", ".",
		"export directory (default is current working directory") //default value is current dir

	importCmd.PersistentFlags().StringVar(&target.Host, "target-db-host", "localhost",
		"target database server host")

	importCmd.PersistentFlags().StringVar(&target.Port, "target-db-port", "5433",
		"target database server port number")

	importCmd.PersistentFlags().StringVar(&target.User, "target-db-user", "",
		"connect to target database as specified user")
	importCmd.MarkPersistentFlagRequired("target-db-user")

	// TODO: All sensitive parameters should be taken from the environment variable
	importCmd.PersistentFlags().StringVar(&target.Password, "target-db-password", "",
		"connect to target as specified user")
	importCmd.MarkPersistentFlagRequired("target-db-password")

	importCmd.PersistentFlags().StringVar(&target.DBName, "target-db-name", "",
		"target database name to be migrated to YugabyteDB")
	importCmd.MarkPersistentFlagRequired("target-db-name")

	// Might not be needed for import in yugabytedb
	/*
		importCmd.PersistentFlags().StringVar(&target.Schema, "target-db-schema", "",
			"target schema name which needs to be migrated to YugabyteDB")
		importCmd.MarkPersistentFlagRequired("target-db-schema")
	*/

	// TODO: SSL related more args might come. Need to explore SSL part completely.
	importCmd.PersistentFlags().StringVar(&target.SSLCertPath, "target-ssl-cert", "",
		"provide target SSL Certificate Path")

	importCmd.PersistentFlags().StringVar(&target.SSLMode, "target-ssl-mode", "disable",
		"specify the target SSL mode out of - disable, allow, prefer, require, verify-ca, verify-full")

	importCmd.PersistentFlags().StringVar(&MigrationMode, "migration-mode", "offline",
		"mode can be offline | online(applicable only for data migration)")

	importCmd.PersistentFlags().StringVar(&StartClean, "start-clean", "",
		"delete all the existing objects and start fresh")
}
