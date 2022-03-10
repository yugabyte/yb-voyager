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
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/yugabyte/ybm/yb_migrate/src/utils"

	"github.com/jackc/pgx/v4"
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
	fmt.Printf("import of schema in '%s' database started\n", target.DBName)
	utils.CheckToolsRequiredInstalledOrNot(&utils.Source{DBType: "yugabytedb"})

	PrintTargetYugabyteDBVersion(&target)

	/*
		targetConnectionURIWithGivenDB := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		target.User, target.Password, target.Host, target.Port, target.DBName, target.SSLMode)
	*/
	targetConnectionURIWithDefaultDB := generateDefaultTargetDBUri(&target)

	//TODO: Explore if DROP DATABASE vs DROP command for all objects
	dropDatabaseQuery := "DROP DATABASE " + target.DBName + ";"
	createDatabaseQuery := "CREATE DATABASE " + target.DBName + ";"

	bgCtx := context.Background()

	// taking assumption that "yugabyte" database should be present in user's setup
	conn, err := pgx.Connect(bgCtx, targetConnectionURIWithDefaultDB)
	if err != nil {
		if res, _ := regexp.MatchString("database.*does[ ]+not[ ]+exist", err.Error()); res {
			fmt.Printf("default database '%s' does not exists, please create it and continue!!\n", YUGABYTEDB_DEFAULT_DATABASE)
			os.Exit(1)
		} else {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	defer conn.Close(bgCtx)

	var fetchedDBName string
	checkDatabaseExistQuery := fmt.Sprintf("SELECT datname FROM pg_database where datname='%s';", target.DBName)
	err = conn.QueryRow(bgCtx, checkDatabaseExistQuery).Scan(&fetchedDBName)

	if err != nil && (strings.Contains(err.Error(), "no rows in result set") && fetchedDBName == "") {
		fmt.Printf("required database '%s' doesn't exists, creating...\n", target.DBName)
		_, err := conn.Exec(bgCtx, createDatabaseQuery)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else if err != nil {
		panic(err)
	} else if target.DBName == fetchedDBName {
		if startClean && target.DBName != YUGABYTEDB_DEFAULT_DATABASE {
			//dropping existing database
			fmt.Printf("dropping '%s' database...\n", target.DBName)

			_, err := conn.Exec(bgCtx, dropDatabaseQuery)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			//creating required database
			fmt.Printf("creating '%s' database...\n", target.DBName)
			_, err = conn.Exec(bgCtx, createDatabaseQuery)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		} else if startClean && target.DBName == YUGABYTEDB_DEFAULT_DATABASE {
			fmt.Printf("can't drop default database: %s, exiting...\n", YUGABYTEDB_DEFAULT_DATABASE)
			fmt.Printf("please clean it manually before starting again!\n")
			os.Exit(1)
		} else {
			fmt.Printf("database '%s' already exists, using it without cleaning\n", target.DBName)
		}
	}

	YugabyteDBImportSchema(&target, exportDir)
}

func generateSSLQueryStringIfNotExists(t *utils.Target) string {
	SSLQueryString := ""
	if t.SSLMode == "" {
		t.SSLMode = "prefer"
	}
	if t.SSLQueryString == "" {

		if t.SSLMode == "disable" || t.SSLMode == "allow" || t.SSLMode == "prefer" || t.SSLMode == "require" || t.SSLMode == "verify-ca" || t.SSLMode == "verify-full" {
			SSLQueryString = "sslmode=" + t.SSLMode
			if t.SSLMode == "require" || t.SSLMode == "verify-ca" || t.SSLMode == "verify-full" {
				SSLQueryString = fmt.Sprintf("sslmode=%s", t.SSLMode)
				if t.SSLCertPath != "" {
					SSLQueryString += "&sslcert=" + t.SSLCertPath
				}
				if t.SSLKey != "" {
					SSLQueryString += "&sslkey=" + t.SSLKey
				}
				if t.SSLRootCert != "" {
					SSLQueryString += "&sslrootcert=" + t.SSLRootCert
				}
				if t.SSLCRL != "" {
					SSLQueryString += "&sslcrl=" + t.SSLCRL
				}
			}
		} else {
			fmt.Println("Invalid sslmode entered")
		}
	} else {
		SSLQueryString = t.SSLQueryString
	}
	return SSLQueryString
}

func generateDefaultTargetDBUri(t *utils.Target) string {
	var defaultDBUri string
	if t.Uri != "" {
		if strings.Count(t.Uri, "/") > 2 {
			//if there is a non-default dbname
			skimindex := strings.Index(t.Uri, "//") + 2
			startindex := strings.Index(t.Uri[skimindex:], "/") + skimindex + 1

			if strings.Count(t.Uri, "?") > 0 {
				//if there are additional params (ssl related, etc)
				endindex := strings.Index(t.Uri, "?")
				defaultDBUri = t.Uri[:startindex] + YUGABYTEDB_DEFAULT_DATABASE + t.Uri[endindex:]
			} else {
				//if there are no additional arguments
				defaultDBUri = t.Uri[:startindex] + YUGABYTEDB_DEFAULT_DATABASE
			}
		} else {
			//if there is an implicit default dbname
			if strings.Count(t.Uri, "?") > 0 {
				//if there are additional params (ssl related, etc)
				endindex := strings.Index(t.Uri, "?")
				defaultDBUri = t.Uri[:endindex] + "/" + YUGABYTEDB_DEFAULT_DATABASE + t.Uri[endindex:]
			} else {
				//if there are additional arguments
				defaultDBUri = t.Uri + "/" + YUGABYTEDB_DEFAULT_DATABASE
			}
		}
	} else {
		defaultDBUri = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?%s", target.User,
			target.Password, target.Host, target.Port, YUGABYTEDB_DEFAULT_DATABASE, generateSSLQueryStringIfNotExists(&target))
	}
	return defaultDBUri
}
