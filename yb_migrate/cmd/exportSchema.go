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
	"yb_migrate/migration"
	"yb_migrate/migrationutil"

	"github.com/spf13/cobra"
)

var schemaName string

// exportSchemaCmd represents the exportSchema command
var exportSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("exportSchema called")
		// if sourceStruct and exportDir etc are nil or undefined
		// then read the config file and create source struct from config file values
		// possibly export dir value as well and then call export Schema with the created arguments
		// else call the exportSchema directly
		if len(cfgFile) == 0 {
			exportSchema(source, exportDir)
		} else {
			// read from config // prepare the structs and then call exportSchema
			fmt.Printf("Config path called")
		}
	},
}

func exportSchema(source Source, exportDir string) {

	switch source.sourceDBType {
	case "oracle":
		fmt.Printf("Prepare Ora2Pg for schema export from Oracle\n")
		oracleSchemaExport()
	case "postgres":
		fmt.Printf("Prepare pgdump for schema export from PG\n")
	case "mysql":
		fmt.Printf("Prepare Ora2Pg for schema export from MySQL\n")
	}
}

func init() {
	exportCmd.AddCommand(exportSchemaCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// exportSchemaCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// exportSchemaCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func oracleSchemaExport() {
	migration.CheckOra2pgInstalled()

	migration.CreateMigrationProject(exportDir, source.sourceDBSchema)

	//This is temporary function. In future, there can be someother general function
	//which can just check the source connectivity separately
	migrationutil.CheckSourceDBConnectivity(source.sourceDBHost, source.sourceDBPort,
		source.sourceDBSchema, source.sourceDBUser, source.sourceDBPassword)

	migration.PrintSourceDBVersion(source.sourceDBHost, source.sourceDBPort,
		source.sourceDBSchema, source.sourceDBUser, source.sourceDBPassword,
		source.sourceDBName, exportDir)

	migration.ExportSchema(source.sourceDBHost, source.sourceDBPort,
		source.sourceDBSchema, source.sourceDBUser, source.sourceDBPassword,
		source.sourceDBName, exportDir)

}
