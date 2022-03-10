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

	"github.com/yugabyte/ybm/yb_migrate/src/utils"

	"github.com/spf13/cobra"
)

var importMode string
var target utils.Target

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import",
	Short: "import schema and data from compatible source database(Oracle, Mysql, Postgres)",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().PersistentPreRun(cmd.Parent(), args)
		// fmt.Println("Parent Import PersistentPreRun")
		checkOrSetDefaultTargetSSLMode()
		// if URI is not given then these flags are required, otherwise just use URI
		if target.Uri == "" {
			cmd.MarkPersistentFlagRequired("target-db-user")
			// TODO: All sensitive parameters should be taken from the environment variable
			cmd.MarkPersistentFlagRequired("target-db-password")
		} else {
			//TODO: else we can have a check for the uri pattern

			target.ParseURI()
		}

		if source.TableList != "" {
			checkTableListFlag()
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		importSchema()
		importData()
	},
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.PersistentFlags().StringVarP(&exportDir, "export-dir", "e", ".",
		"export directory (default is current working directory") //default value is current dir

	importCmd.PersistentFlags().StringVar(&target.Host, "target-db-host", "127.0.0.1",
		"Host on which the YugabyteDB server is running")

	importCmd.PersistentFlags().StringVar(&target.Port, "target-db-port", YUGABYTEDB_DEFAULT_PORT,
		"Port on which the YugabyteDB database is running")

	importCmd.PersistentFlags().StringVar(&target.User, "target-db-user", "",
		"Username with which to connect to the target YugabyteDB server")

	importCmd.PersistentFlags().StringVar(&target.Password, "target-db-password", "",
		"Password with which to connect to the target YugabyteDB server")

	importCmd.PersistentFlags().StringVar(&target.DBName, "target-db-name", YUGABYTEDB_DEFAULT_DATABASE,
		"Name of the database on the target YugabyteDB server on which import needs to be done")

	importCmd.PersistentFlags().StringVar(&target.Uri, "target-db-uri", "",
		"Complete connection uri to the target YugabyteDB server")

	// Might not be needed for import in yugabytedb
	/*
		importCmd.PersistentFlags().StringVar(&target.Schema, "target-db-schema", "",
			"target schema name which needs to be migrated to YugabyteDB")
		importCmd.MarkPersistentFlagRequired("target-db-schema")
	*/

	// TODO: SSL related more args might come. Need to explore SSL part completely.
	importCmd.PersistentFlags().StringVar(&target.SSLCertPath, "target-ssl-cert", "",
		"provide target SSL Certificate Path")

	importCmd.PersistentFlags().StringVar(&target.SSLMode, "target-ssl-mode", "prefer",
		"specify the target SSL mode out of - disable, allow, prefer, require, verify-ca, verify-full")

	importCmd.PersistentFlags().StringVar(&target.SSLKey, "target-ssl-key", "",
		"provide SSL Key Path")

	importCmd.PersistentFlags().StringVar(&target.SSLRootCert, "target-ssl-root-cert", "",
		"provide SSL Root Certificate Path")

	importCmd.PersistentFlags().StringVar(&target.SSLCRL, "target-ssl-crl", "",
		"provide SSL Root Certificate Revocation List (CRL)")

	importCmd.PersistentFlags().StringVar(&migrationMode, "migration-mode", "offline",
		"mode can be offline | online(applicable only for data migration)")

	importCmd.PersistentFlags().BoolVar(&startClean, "start-clean", false,
		"delete all the existing objects and start fresh")

	importCmd.PersistentFlags().Int64Var(&numLinesInASplit, "batch-size", 100000,
		"Maximum size of each batch import ")
	importCmd.PersistentFlags().IntVar(&parallelImportJobs, "parallel-jobs", -1,
		"Number of parallel copy command jobs. default: -1 means number of servers in the Yugabyte cluster")

	importCmd.PersistentFlags().BoolVar(&target.ImportIndexesAfterData, "import-indexes-after-data", true,
		"false - import indexes before data\n"+
			"true - create index after data i.e. index backfill")

	importCmd.PersistentFlags().BoolVar(&target.VerboseMode, "verbose", false,
		"verbose mode for some extra details during execution of command")

	importCmd.PersistentFlags().BoolVar(&target.ContinueOnError, "continue-on-error", false,
		"false - stop the execution in case of errors(default false)\n"+
			"true - to ignore errors and continue")

	importCmd.PersistentFlags().BoolVar(&target.IgnoreIfExists, "ignore-exist", false,
		"true - to ignore errors if object already exists\n"+
			"false - throw those errors to the standard output (default false)")

	importCmd.PersistentFlags().StringVar(&target.TableList, "table-list", "",
		"list of the tables to import data(Note: works only for import data command)")
}

func checkOrSetDefaultTargetSSLMode() {
	if target.Uri == "" {
		if target.SSLMode == "" {
			target.SSLMode = "prefer"
		} else if target.SSLMode != "disable" && target.SSLMode != "prefer" && target.SSLMode != "require" && target.SSLMode != "verify-ca" && target.SSLMode != "verify-full" {
			fmt.Printf("Invalid sslmode\nValid sslmodes are: disable, allow, prefer, require, verify-ca, verify-full")
			os.Exit(1)
		}
	}
}
