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
	"os/exec"
	"regexp"
	"strings"
	"yb_migrate/migration"
	"yb_migrate/migrationutil"

	"github.com/spf13/cobra"
)

// importSchemaCmd represents the importSchema command
var importSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "This command imports schema into the destination YugabyteDB database",
	Long:  `Long version This command imports schema into the destination YUgabyteDB database.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		if startClean != "NO" && startClean != "YES" {
			fmt.Printf("Invalid value of flag start-clean as '%s'\n", startClean)
			os.Exit(1)
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Import Schema Command called")
		importSchema()
	},
}

func init() {
	importCmd.AddCommand(importSchemaCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// importSchemaCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// importSchemaCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func importSchema() {
	migrationutil.CheckToolsRequiredInstalledOrNot("yugabytedb")

	targetConnectionURIWithGivenDB := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		target.User, target.Password, target.Host, target.Port, target.DBName, source.SSLMode)
	targetConnectionURIWithDefaultDB := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		target.User, target.Password, target.Host, target.Port, "yugabyte", source.SSLMode)

	checkDatabaseExistenceCommand := exec.Command("psql", targetConnectionURIWithGivenDB,
		"-Atc", fmt.Sprintf("SELECT datname FROM pg_database where datname='%s';", target.DBName))

	cmdOutputBytes, _ := checkDatabaseExistenceCommand.CombinedOutput()

	// migrationutil.CheckError(err, checkDatabaseExistenceCommand.String(), string(cmdOutputBytes), true)

	existingDatabaseName := strings.Trim(string(cmdOutputBytes), "\n")

	fmt.Printf("[Info]: %s\n", checkDatabaseExistenceCommand)

	//TODO: add options for setting client_encoding
	//TODO: Check if DROP DATABASE in slow in YugabyteDB vs Postgresql?
	dropDatabaseSql := "DROP DATABASE " + target.DBName + ";"
	createDatabaseSql := "CREATE DATABASE " + target.DBName + ";"

	dropDatabaseCommand := exec.Command("psql", targetConnectionURIWithDefaultDB, "-c", dropDatabaseSql)
	createDatabaseCommand := exec.Command("psql", targetConnectionURIWithDefaultDB, "-c", createDatabaseSql)

	if existingDatabaseName == target.DBName {
		if target.StartClean || migrationutil.AskPrompt("Drop and create a new database named: ", target.DBName) {
			//dropping existing database
			fmt.Printf("dropping %s database...\n", target.DBName)
			cmdOutputBytes, err := dropDatabaseCommand.CombinedOutput()
			migrationutil.CheckError(err, dropDatabaseCommand.String(), string(cmdOutputBytes), true)

			//creating required database
			fmt.Printf("creating %s database...\n", target.DBName)
			cmdOutputBytes, err = createDatabaseCommand.CombinedOutput()
			migrationutil.CheckError(err, createDatabaseCommand.String(), string(cmdOutputBytes), true)

		} // else Use the exisitng Database

	} else if patternMatch, _ := regexp.MatchString("database.*does[ ]+not[ ]+exist", existingDatabaseName); patternMatch {
		fmt.Printf("[Info] %s\n", existingDatabaseName)
		if migrationutil.AskPrompt("Create a new database named:", target.DBName) {
			fmt.Printf("creating %s database...\n", target.DBName)

			cmdOutputBytes, err := createDatabaseCommand.CombinedOutput()
			migrationutil.CheckError(err, createDatabaseCommand.String(), string(cmdOutputBytes), true)

		} else {
			//Database neither exists nor created, so exit the process
			fmt.Println("Import Schema Aborted!!")
			os.Exit(126)
		}
	} else {
		fmt.Println("Import Schema Aborted!!")
		os.Exit(126)
	}

	migration.YugabyteDBImportSchema(&target, ExportDir)
}
