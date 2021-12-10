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

var reportName string
var destYBVersion string

// exportGenerateReportCmd represents the exportGenerateReport command
var exportGenerateReportCmd = &cobra.Command{
	Use:   "report",
	Short: "This command gives a report of the database objects to be migrated to YugabyteDB",
	Long: `This command gives a report of the database objects of th source database to be migrated
to YugabyteDB. It includes the list of objects to be migrated, objects which cannot be migrated and
also for some of the database objects for which direct support is not there in YugabyteDB it suggests workarounds.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("generate report called")
	},
}

func init() {
	rootCmd.AddCommand(exportGenerateReportCmd)

	exportGenerateReportCmd.PersistentFlags().StringVar(&source.DBType, "source-db-type", "",
		"source database type (Oracle/PostgreSQL/MySQL)")
	exportGenerateReportCmd.PersistentFlags().StringVar(&source.Host, "source-db-host", "localhost",
		"The host on which the source database is running")
	// TODO How to change defaults with the db type
	exportGenerateReportCmd.PersistentFlags().StringVar(&source.Port, "source-db-port", "",
		"The port on which the source database is running")
	exportGenerateReportCmd.PersistentFlags().StringVar(&source.User, "source-db-user", "",
		"The user with which the connection will be made to the source database")
	// TODO All sensitive parameters can be taken from the environment variable
	exportGenerateReportCmd.PersistentFlags().StringVar(&source.Password, "source-db-password", "",
		"The user with which the connection will be made to the source database")
	exportGenerateReportCmd.PersistentFlags().StringVar(&source.DBName, "source-db-name", "",
		"The source database which needs to be migrated to YugabyteDB")
	exportGenerateReportCmd.PersistentFlags().StringVar(&source.Schema, "source-db-schema", "",
		"The source schema which needs to be migrated to YugabyteDB")
	// TODO SSL related more args will come. Explore them later.
	exportGenerateReportCmd.PersistentFlags().StringVar(&source.SSLCertPath, "source-ssl-cert", "",
		"source database type (Oracle/PostgreSQL/MySQL")

	exportGenerateReportCmd.PersistentFlags().StringVar(&destYBVersion, "dest-yb-version", "localhost",
		"The version of the destination YugabyteDB database")
	exportGenerateReportCmd.Flags().StringVar(&reportName, "report-name", "",
		"The file where the report will be written into")
	exportGenerateReportCmd.Flags().BoolP("useExtensions", "E", false,
		"The port on which the source database is running")
}
