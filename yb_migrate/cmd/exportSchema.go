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
		if source.sourceDBPort == "" {
			source.sourceDBPort = "1521"
		}
		oracleSchemaExport()
	case "postgres":
		fmt.Printf("Prepare ysql_dump for schema export from PG\n")
		if source.sourceDBPort == "" {
			source.sourceDBPort = "5432"
		}
		postgresSchemaExport()
	case "mysql":
		fmt.Printf("Prepare Ora2Pg for schema export from MySQL\n")
		mysqlSchemaExport()
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
	//TODO: make it a general function under migrationutil to check for given source-db-type
	//function may not be needed if things change going forward.
	migration.CheckOracleToolsInstalled()
	// migrationutil.CheckRequiredToolsInstalled(source.sourceDBType)

	// Temporary. TODO: One function for checksourcedbendpoint + dbuserpassword + dbversionprint
	migrationutil.CheckSourceDBConnectivity(source.sourceDBHost, source.sourceDBPort,
		source.sourceDBSchema, source.sourceDBUser, source.sourceDBPassword)

	//TODO: make this general for every source db type | [Optional Function]
	migration.PrintOracleSourceDBVersion(source.sourceDBHost, source.sourceDBPort,
		source.sourceDBSchema, source.sourceDBUser, source.sourceDBPassword,
		source.sourceDBName, exportDir)

	//[Self] It should be source.sourceDBName for other db-source-type
	projectDirName := "project-" + source.sourceDBSchema + "-migration"
	migrationutil.CreateMigrationProject(exportDir, projectDirName, source.sourceDBSchema)

	migration.ExportOracleSchema(source.sourceDBHost, source.sourceDBPort,
		source.sourceDBSchema, source.sourceDBUser, source.sourceDBPassword,
		source.sourceDBName, exportDir, projectDirName)

}

func postgresSchemaExport() {
	migration.CheckPostgresToolsInstalled()

	migrationutil.CheckSourceDBConnectivity(source.sourceDBHost, source.sourceDBPort,
		source.sourceDBSchema, source.sourceDBUser, source.sourceDBPassword)

	migration.PrintPostgresSourceDBVersion(source.sourceDBHost, source.sourceDBPort,
		source.sourceDBSchema, source.sourceDBUser, source.sourceDBPassword,
		source.sourceDBName, exportDir)

	//[Self] It should be source.sourceDBName for other db-source-type
	projectDirName := "project-" + source.sourceDBName + "-migration"
	migrationutil.CreateMigrationProject(exportDir, projectDirName, source.sourceDBName)

	migration.ExportPostgresSchema(source.sourceDBHost, source.sourceDBPort,
		source.sourceDBSchema, source.sourceDBUser, source.sourceDBPassword,
		source.sourceDBName, exportDir, projectDirName)
}

func mysqlSchemaExport() {

}
