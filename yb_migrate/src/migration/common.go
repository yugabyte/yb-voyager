/*
Copyright (c) YugaByte, Inc.

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
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"

	"github.com/go-sql-driver/mysql"
	_ "github.com/godror/godror"
	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
)

func UpdateFilePaths(source *utils.Source, exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	var requiredMap map[string]string

	// TODO: handle the case if table name has double quotes/case sensitive

	sortedKeys := utils.GetSortedKeys(&tablesProgressMetadata)
	if source.DBType == "postgresql" {
		requiredMap = getMappingForTableNameVsTableFileName(exportDir + "/data")
		for _, key := range sortedKeys {
			tableName := tablesProgressMetadata[key].TableName
			fullTableName := tablesProgressMetadata[key].FullTableName

			if _, ok := requiredMap[fullTableName]; ok { // checking if toc/dump has data file for table
				tablesProgressMetadata[key].InProgressFilePath = exportDir + "/data/" + requiredMap[fullTableName]
				if tablesProgressMetadata[key].TableSchema == "public" {
					tablesProgressMetadata[key].FinalFilePath = exportDir + "/data/" + tableName + "_data.sql"
				} else {
					tablesProgressMetadata[key].FinalFilePath = exportDir + "/data/" + fullTableName + "_data.sql"
				}
			} else {
				log.Infof("deleting an entry from tablesProgressMetadata: ", fullTableName)
				delete(tablesProgressMetadata, key)
			}
		}
	} else if source.DBType == "oracle" || source.DBType == "mysql" {
		for _, key := range sortedKeys {
			targetTableName := tablesProgressMetadata[key].TableName
			// required if PREFIX_PARTITION is set in ora2pg.conf file
			// if tablesProgressMetadata[key].IsPartition {
			// 	targetTableName = tablesProgressMetadata[key].ParentTable + "_" + targetTableName
			// }
			tablesProgressMetadata[key].InProgressFilePath = exportDir + "/data/tmp_" + targetTableName + "_data.sql"
			tablesProgressMetadata[key].FinalFilePath = exportDir + "/data/" + targetTableName + "_data.sql"
		}
	}

	logMsg := "After updating data file paths, TablesProgressMetadata:"
	for _, key := range sortedKeys {
		logMsg += fmt.Sprintf("%+v\n", tablesProgressMetadata[key])
	}
	log.Infof(logMsg)
}

func UpdateTableRowCount(source *utils.Source, exportDir string, tablesProgressMetadata map[string]*utils.TableProgressMetadata) {
	fmt.Println("calculating num of rows to export for each table...")
	if !source.VerboseMode {
		go utils.Wait()
	}

	utils.PrintIfTrue(fmt.Sprintf("+%s+\n", strings.Repeat("-", 65)), source.VerboseMode)
	utils.PrintIfTrue(fmt.Sprintf("| %30s | %30s |\n", "Table", "Row Count"), source.VerboseMode)

	sortedKeys := utils.GetSortedKeys(&tablesProgressMetadata)
	for _, key := range sortedKeys {
		utils.PrintIfTrue(fmt.Sprintf("|%s|\n", strings.Repeat("-", 65)), source.VerboseMode)
		// fullTableName := tablesProgressMetadata[key].FullTableName

		utils.PrintIfTrue(fmt.Sprintf("| %30s ", key), source.VerboseMode)

		if source.VerboseMode {
			go utils.Wait()
		}

		queryFrom := tablesProgressMetadata[key].FullTableName
		rowCount := SelectCountStarFromTable(queryFrom, source)

		if source.VerboseMode {
			utils.WaitChannel <- 0
			<-utils.WaitChannel
		}

		tablesProgressMetadata[key].CountTotalRows = rowCount
		utils.PrintIfTrue(fmt.Sprintf("| %30d |\n", rowCount), source.VerboseMode)
	}
	utils.PrintIfTrue(fmt.Sprintf("+%s+\n", strings.Repeat("-", 65)), source.VerboseMode)
	if !source.VerboseMode {
		utils.WaitChannel <- 0
		<-utils.WaitChannel
	}

	log.Tracef("After updating total row count, TablesProgressMetadata: %+v", tablesProgressMetadata)
}

func GetTableRowCount(filePath string) map[string]int64 {
	tableRowCountMap := make(map[string]int64)

	fileBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		utils.ErrExit("read file %q: %s", filePath, err)
	}

	lines := strings.Split(strings.Trim(string(fileBytes), "\n"), "\n")

	for _, line := range lines {
		tableName := strings.Split(line, ",")[0]
		rowCount := strings.Split(line, ",")[1]
		rowCountInt64, _ := strconv.ParseInt(rowCount, 10, 64)

		tableRowCountMap[tableName] = rowCountInt64
	}

	log.Infof("tableRowCountMap: %v", tableRowCountMap)
	return tableRowCountMap
}

func SelectCountStarFromTable(tableName string, source *utils.Source) int64 {
	var rowCount int64 = -1
	dbConnStr := GetDriverConnStr(source)
	query := fmt.Sprintf("select count(*) from %s", tableName)

	//just querying each source type using corresponding drivers
	switch source.DBType {
	case "oracle":
		db, err := sql.Open("godror", dbConnStr)
		if err != nil {
			utils.WaitChannel <- 0 //stop waiting
			<-utils.WaitChannel
			errMsg := fmt.Sprintf("error while count(*) query from %q: %v\n", tableName, err)
			utils.ErrExit(errMsg)
		}
		defer db.Close()

		err = db.QueryRow(query).Scan(&rowCount)
		if err != nil {
			utils.WaitChannel <- 0
			<-utils.WaitChannel
			errMsg := fmt.Sprintf("error while scanning count(*) rows from %q: %v\n", tableName, err)
			utils.ErrExit(errMsg)
		}
	case "mysql":
		db, err := sql.Open("mysql", dbConnStr)
		if err != nil {
			utils.WaitChannel <- 0
			<-utils.WaitChannel
			errMsg := fmt.Sprintf("error while count(*) query from %q: %v\n", tableName, err)
			utils.ErrExit(errMsg)
		}
		defer db.Close()

		err = db.QueryRow(query).Scan(&rowCount)
		if err != nil {
			utils.WaitChannel <- 0
			<-utils.WaitChannel
			errMsg := fmt.Sprintf("error while scanning count(*) rows from %q: %v\n", tableName, err)
			utils.ErrExit(errMsg)
		}
	case "postgresql":
		conn, err := pgx.Connect(context.Background(), dbConnStr)
		if err != nil {
			utils.WaitChannel <- 0
			<-utils.WaitChannel
			errMsg := fmt.Sprintf("error while count(*) query from %q: %v\n", tableName, err)
			utils.ErrExit(errMsg)
		}
		defer conn.Close(context.Background())

		err = conn.QueryRow(context.Background(), query).Scan(&rowCount)
		if err != nil {
			utils.WaitChannel <- 0
			<-utils.WaitChannel
			errMsg := fmt.Sprintf("error while scanning count(*) rows from %q: %v\n", tableName, err)
			utils.ErrExit(errMsg)
		}
	}

	if rowCount == -1 { // if var is still not updated
		errMsg := fmt.Sprintf("couldn't fetch row count for table: %q\n", tableName)
		utils.ErrExit(errMsg)
	}

	return rowCount
}

func GetDriverConnStr(source *utils.Source) string {
	var connStr string
	switch source.DBType {
	//TODO:Discuss and set a priority order for checks in the case of Oracle
	case "oracle":
		if source.DBSid != "" {
			connStr = fmt.Sprintf("%s/%s@(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%d))(CONNECT_DATA=(SID=%s)))",
				source.User, source.Password, source.Host, source.Port, source.DBSid)
		} else if source.TNSAlias != "" {
			connStr = fmt.Sprintf("%s/%s@%s", source.User, source.Password, source.TNSAlias)
		} else if source.DBName != "" {
			connStr = fmt.Sprintf("%s/%s@(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%d))(CONNECT_DATA=(SERVICE_NAME=%s)))",
				source.User, source.Password, source.Host, source.Port, source.DBName)
		}
	case "mysql":
		parseSSLString(source)
		var tlsString string
		switch source.SSLMode {
		case "disable":
			tlsString = "tls=false"
			break
		case "prefer":
			tlsString = "tls=preferred"
			break
		case "require":
			tlsString = "tls=skip-verify"
			break
		case "verify-ca", "verify-full":
			tlsConf := createTLSConf(source)
			err := mysql.RegisterTLSConfig("custom", &tlsConf)
			if err != nil {
				fmt.Println(err)
				log.Fatal(err)
			}
			tlsString = "tls=custom"
			break
		default:
			errMsg := "Incorrect SSL Mode Provided. Please enter a valid sslmode."
			utils.ErrExit(errMsg)
		}
		connStr = fmt.Sprintf("%s:%s@(%s:%d)/%s?%s", source.User, source.Password,
			source.Host, source.Port, source.DBName, tlsString)

	case "postgresql":
		if source.Uri == "" {
			connStr = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?%s", source.User, source.Password,
				source.Host, source.Port, source.DBName, generateSSLQueryStringIfNotExists(source))
		} else {
			connStr = source.Uri
		}
	}
	return connStr
}

func PrintSourceDBVersion(source *utils.Source) string {
	dbConnStr := GetDriverConnStr(source)
	version := SelectVersionQuery(source.DBType, dbConnStr)

	if !source.GenerateReportMode {
		fmt.Printf("%s Version: %s\n", strings.ToUpper(source.DBType), version)
	}

	return version
}

func SelectVersionQuery(dbType string, dbConnStr string) string {
	var version string

	switch dbType {
	case "oracle":
		db, err := sql.Open("godror", dbConnStr)
		if err != nil {
			utils.ErrExit("connect to oracle source DB: %s", err)
		}
		defer db.Close()

		query := "SELECT VERSION FROM V$INSTANCE"
		err = db.QueryRow(query).Scan(&version)
		if err != nil {
			utils.ErrExit("run query %q on source: %s", query, err)
		}
	case "mysql":
		db, err := sql.Open("mysql", dbConnStr)
		if err != nil {
			utils.ErrExit("connect to source mysql db: %s", err)
		}
		defer db.Close()

		query := "SELECT VERSION()"
		err = db.QueryRow(query).Scan(&version)
		if err != nil {
			utils.ErrExit("run query %q on source: %s", query, err)
		}
	case "postgresql", "yugabytedb":
		conn, err := pgx.Connect(context.Background(), dbConnStr)
		if err != nil {
			utils.ErrExit("connect to %s db: %s", dbType, err)
		}
		defer conn.Close(context.Background())

		query := "SELECT setting from pg_settings where name = 'server_version'"
		err = conn.QueryRow(context.Background(), query).Scan(&version)
		if err != nil {
			utils.ErrExit("run query %q on %s: %s", query, dbType, err)
		}
	}

	log.Infof("%s version: %q", dbType, version)
	return version
}

func ExportDataPostProcessing(source *utils.Source, exportDir string, tablesProgressMetadata *map[string]*utils.TableProgressMetadata) {
	if source.DBType == "oracle" || source.DBType == "mysql" {
		// empty - in case of oracle and mysql, the renaming is handled by tool(ora2pg)
	} else if source.DBType == "postgresql" {
		renameDataFiles(tablesProgressMetadata)
	}

	saveExportedRowCount(exportDir, tablesProgressMetadata)
}

func renameDataFiles(tablesProgressMetadata *map[string]*utils.TableProgressMetadata) {
	for _, tableProgressMetadata := range *tablesProgressMetadata {
		oldFilePath := tableProgressMetadata.InProgressFilePath
		newFilePath := tableProgressMetadata.FinalFilePath
		if utils.FileOrFolderExists(oldFilePath) {
			err := os.Rename(oldFilePath, newFilePath)
			if err != nil {
				utils.ErrExit("renaming data file for table %q after data export: %v", tableProgressMetadata.TableName, err)
			}
		}
	}
}

func saveExportedRowCount(exportDir string, tablesMetadata *map[string]*utils.TableProgressMetadata) {
	filePath := exportDir + "/metainfo/flags/tablesrowcount"
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	fmt.Println("exported num of rows for each table")
	fmt.Printf("+%s+\n", strings.Repeat("-", 65))
	fmt.Printf("| %30s | %30s |\n", "Table", "Row Count")
	sortedKeys := utils.GetSortedKeys(tablesMetadata)
	for _, key := range sortedKeys {
		tableMetadata := (*tablesMetadata)[key]
		fmt.Printf("|%s|\n", strings.Repeat("-", 65))

		targetTableName := strings.TrimSuffix(filepath.Base(tableMetadata.FinalFilePath), "_data.sql")
		actualRowCount := tableMetadata.CountLiveRows
		line := targetTableName + "," + strconv.FormatInt(actualRowCount, 10) + "\n"
		file.WriteString(line)
		fmt.Printf("| %30s | %30d |\n", key, actualRowCount)
	}
	fmt.Printf("+%s+\n", strings.Repeat("-", 65))
}

func MySQLGetAllTableNames(source *utils.Source) []string {
	dbConnStr := GetDriverConnStr(source)
	db, err := sql.Open("mysql", dbConnStr)
	if err != nil {
		errMsg := fmt.Sprintf("error in opening connections to database: %v\n", err)
		utils.ErrExit(errMsg)
	}
	defer db.Close()

	var tableNames []string
	query := fmt.Sprintf("SELECT table_name FROM information_schema.tables "+
		"WHERE table_schema = '%s' && table_type = 'BASE TABLE'", source.DBName)
	rows, err := db.Query(query)
	if err != nil {
		errMsg := fmt.Sprintf("error in querying source database for table names: %v\n", err)
		utils.ErrExit(errMsg)
	}
	defer rows.Close()
	for rows.Next() {
		var tableName string
		err = rows.Scan(&tableName)
		if err != nil {
			errMsg := fmt.Sprintf("error in scanning query rows for table names: %v\n", err)
			utils.ErrExit(errMsg)
		}
		tableNames = append(tableNames, tableName)
	}
	return tableNames
}

func CheckSourceDBAccessibility(source *utils.Source) {
	dbConnStr := GetDriverConnStr(source)

	switch source.DBType {
	case "oracle":
		db, err := sql.Open("godror", dbConnStr)
		if err != nil {
			errMsg := fmt.Sprintf("error in opening connections to database: %v\n", err)
			utils.ErrExit(errMsg)
		}
		db.Close()
	case "mysql":
		db, err := sql.Open("mysql", dbConnStr)
		if err != nil {
			errMsg := fmt.Sprintf("error in opening connections to database: %v\n", err)
			utils.ErrExit(errMsg)
		}
		db.Close()
	case "postgresql":
		conn, err := pgx.Connect(context.Background(), dbConnStr)
		if err != nil {
			errMsg := fmt.Sprintf("error in opening connections to database: %v\n", err)
			utils.ErrExit(errMsg)
		}
		conn.Close(context.Background())
	}

}
