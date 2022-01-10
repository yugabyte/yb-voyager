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
	"yb_migrate/src/migration"
	"yb_migrate/src/utils"

	"github.com/spf13/cobra"
)

// importSchemaCmd represents the importSchema command
var importSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "This command imports schema into the destination YugabyteDB database",
	Long:  `Long version This command imports schema into the destination YUgabyteDB database.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().PersistentPreRun(cmd.Parent(), args)
		// fmt.Println("Import Schema PersistentPreRun")
	},

	Run: func(cmd *cobra.Command, args []string) {
		importSchema()
	},
}

func init() {
	importCmd.AddCommand(importSchemaCmd)
}

func importSchema() {
	fmt.Printf("\nimport of schema in '%s' database started\n", target.DBName)
	utils.CheckToolsRequiredInstalledOrNot("yugabytedb")

	// targetConnectionURIWithGivenDB := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
	// 	target.User, target.Password, target.Host, target.Port, target.DBName, target.SSLMode)
	targetConnectionURIWithDefaultDB := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s", target.User,
		target.Password, target.Host, target.Port, YUGABYTEDB_DEFAULT_DATABASE, target.SSLMode)

	//TODO: Explore if DROP DATABASE vs DROP command for all objects
	dropDatabaseSql := "DROP DATABASE " + target.DBName + ";"
	createDatabaseSql := "CREATE DATABASE " + target.DBName + ";"
	dropDatabaseCommand := exec.Command("psql", targetConnectionURIWithDefaultDB, "-c", dropDatabaseSql)
	createDatabaseCommand := exec.Command("psql", targetConnectionURIWithDefaultDB, "-c", createDatabaseSql)

	checkDatabaseExistCommand := exec.Command("psql", targetConnectionURIWithDefaultDB,
		"-Atc", fmt.Sprintf("SELECT datname FROM pg_database where datname='%s';", target.DBName))

	cmdOutputBytes, err := checkDatabaseExistCommand.CombinedOutput()

	// fmt.Printf("[Debug]: %s\n", checkDatabaseExistCommand)
	// fmt.Printf("[Info] Command Output: %s\n", cmdOutputBytes)
	// utils.CheckError(err, "", "", true)

	//removing newline character from the end
	requiredDatabaseName := strings.Trim(string(cmdOutputBytes), "\n")

	//TODO: make error matching/handling better
	if patternMatch, _ := regexp.MatchString("database.*does[ ]+not[ ]+exist", requiredDatabaseName); patternMatch {
		// above if-condition is only true if default database - "yugabyte" doesn't exists

		fmt.Printf("Default database '%s' does not exists, please create it and continue!!\n", YUGABYTEDB_DEFAULT_DATABASE)
		os.Exit(1)

	} else if requiredDatabaseName == target.DBName {
		if startClean && target.DBName != YUGABYTEDB_DEFAULT_DATABASE {
			//dropping existing database
			fmt.Printf("dropping '%s' database...\n", target.DBName)

			cmdOutputBytes, err := dropDatabaseCommand.CombinedOutput()
			utils.CheckError(err, "", string(cmdOutputBytes), true)

			//creating required database
			fmt.Printf("creating '%s' database...\n", target.DBName)
			cmdOutputBytes, err = createDatabaseCommand.CombinedOutput()
			utils.CheckError(err, "", string(cmdOutputBytes), true)

		} else {
			fmt.Println("[Debug] Using the target database directly wihtout cleaning")
		}

	} else if requiredDatabaseName == "" {
		//above if-condition is true if target DB does not exists

		fmt.Printf("Required Database doesn't exists, creating '%s' database...\n", target.DBName)

		err = createDatabaseCommand.Run()
		utils.CheckError(err, "", "couldn't create the target database", true)
	} else { //cases like user, password are invalid
		fmt.Println("Import Schema could not proceed, Abort!!")
		if err != nil {
			log.Fatal(err.Error())
		} else {
			os.Exit(126)
		}
	}

	migration.YugabyteDBImportSchema(&target, exportDir)
}
