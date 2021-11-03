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

var sourceDBType string
var sourceHost string
var sourcePort string
var sourceUser string
var sourceDBPasswd string
var sourceDatabase string
var sourceSchema string
var sourceSSLCert string

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export has various sub-commands to extract schema, data and generate migration report",
	Long: `Export has various sub-commands to extract schema, data and generate migration report.
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("export called")
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.PersistentFlags().StringVar(&sourceDBType, "source-db-type", "",
		"source database type (Oracle/PostgreSQL/MySQL)")
	exportCmd.PersistentFlags().StringVar(&sourceHost, "source-host", "localhost",
		"The host on which the source database is running")
	// TODO How to change defaults with the db type
	exportCmd.PersistentFlags().StringVar(&sourcePort, "source-port", "",
		"The port on which the source database is running")
	exportCmd.PersistentFlags().StringVar(&sourceUser, "source-user", "",
		"The user with which the connection will be made to the source database")
	// TODO All sensitive parameters can be taken from the environment variable
	exportCmd.PersistentFlags().StringVar(&sourceDBPasswd, "source-db-password", "",
		"The user with which the connection will be made to the source database")
	exportCmd.PersistentFlags().StringVar(&sourceDatabase, "source-database", "",
		"The source database which needs to be migrated to YugabyteDB")
	exportCmd.PersistentFlags().StringVar(&sourceSchema, "source-schema", "",
		"The source schema which needs to be migrated to YugabyteDB")
	// TODO SSL related more args will come. Explore them later.
	exportCmd.PersistentFlags().StringVar(&sourceSSLCert, "source-ssl-cert", "",
		"source database type (Oracle/PostgreSQL/MySQL")


}
