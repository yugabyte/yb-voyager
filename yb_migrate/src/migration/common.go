package migration

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/yugabyte/ybm/yb_migrate/src/utils"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/godror/godror"
	"github.com/jackc/pgx/v4"
)

var log = utils.GetLogger()

func UpdateFilePaths(source *utils.Source, exportDir string, tablesMetadata []utils.TableProgressMetadata) {
	var requiredMap map[string]string

	// TODO: handle the case if table name has double quotes/case sensitive

	if source.DBType == "postgresql" {
		requiredMap = getMappingForTableNameVsTableFileName(exportDir + "/data")
		for i := 0; i < len(tablesMetadata); i++ {
			tableName := tablesMetadata[i].TableName
			tablesMetadata[i].InProgressFilePath = exportDir + "/data/" + requiredMap[tableName]
			tablesMetadata[i].FinalFilePath = exportDir + "/data/" + tablesMetadata[i].TableName + "_data.sql"
		}
	} else { //for Oracle and MySQL
		for i := 0; i < len(tablesMetadata); i++ {
			tableName := tablesMetadata[i].TableName
			fileName := "tmp_" + strings.ToUpper(tableName) + "_data.sql"
			tablesMetadata[i].InProgressFilePath = exportDir + "/data/" + fileName
			tablesMetadata[i].FinalFilePath = exportDir + "/data/" + strings.ToUpper(tableName) + "_data.sql"
		}
	}

	// fmt.Println("After updating datafilepath")
	// fmt.Printf("TableMetadata: %v\n\n", tablesMetadata)
}

func UpdateTableRowCount(source *utils.Source, exportDir string, tablesMetadata []utils.TableProgressMetadata) {
	fmt.Println("calculating num of rows for each table...")
	fmt.Printf("+%s+\n", strings.Repeat("-", 65))
	fmt.Printf("| %30s | %30s |\n", "Table", "Row Count")
	for i := 0; i < len(tablesMetadata); i++ {
		fmt.Printf("|%s|\n", strings.Repeat("-", 65))
		tableName := tablesMetadata[i].TableName
		fmt.Printf("| %30s ", tableName)

		go utils.Wait()

		rowCount := SelectCountStarFromTable(tableName, source)

		utils.WaitChannel <- 0

		tablesMetadata[i].CountTotalRows = rowCount
		fmt.Printf("| %30d |\n", rowCount)
	}
	fmt.Printf("+%s+\n", strings.Repeat("-", 65))
	// fmt.Println("After updating total row count")
	// fmt.Printf("TableMetadata: %v\n\n", tablesMetadata)
}

func GetTableList(exportDir string) []string {
	var tableList []string
	tableSqlFilePath := exportDir + "/schema/tables/table.sql"

	tableSqlFileData, err := ioutil.ReadFile(tableSqlFilePath)

	errorMsg := fmt.Sprintf("couldn't read file: %s\n", tableSqlFilePath)
	if err != nil {
		log.Printf(errorMsg)
		panic(err)
	}

	tableSqls := strings.Split(string(tableSqlFileData), "\n")
	//Temporary, assuming tools dump SQLs in sophisticated manner
	tableNameRegex := regexp.MustCompile(`CREATE[ ]+TABLE[ ]+(\S+)[ (]+`)

	for _, line := range tableSqls {
		tablenameMatches := tableNameRegex.FindAllStringSubmatch(line, -1)
		// fmt.Println(tablenameMatches)
		for _, match := range tablenameMatches {
			tableList = append(tableList, match[1])
		}

	}

	return tableList
}

//temp function, will change based on report generation part
func GetTableRowCount(filePath string) map[string]int64 {
	tableRowCountMap := make(map[string]int64)

	fileBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	lines := strings.Split(strings.Trim(string(fileBytes), "\n"), "\n")

	for _, line := range lines {
		tableName := strings.Split(line, ",")[0]
		rowCount := strings.Split(line, ",")[1]
		rowCountInt64, _ := strconv.ParseInt(rowCount, 10, 64)

		tableRowCountMap[tableName] = rowCountInt64
	}

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
			fmt.Println(err)
			os.Exit(1)
		}
		defer db.Close()

		err = db.QueryRow(query).Scan(&rowCount)
		if err != nil {
			utils.WaitChannel <- 0
			fmt.Println(err)
			os.Exit(1)
		}
	case "mysql":
		db, err := sql.Open("mysql", dbConnStr)
		if err != nil {
			utils.WaitChannel <- 0
			fmt.Println(err)
			os.Exit(1)
		}
		defer db.Close()

		err = db.QueryRow(query).Scan(&rowCount)
		if err != nil {
			utils.WaitChannel <- 0
			fmt.Println(err)
			os.Exit(1)
		}
	case "postgresql":
		conn, err := pgx.Connect(context.Background(), dbConnStr)
		if err != nil {
			utils.WaitChannel <- 0
			fmt.Println(err)
			os.Exit(1)
		}
		defer conn.Close(context.Background())

		err = conn.QueryRow(context.Background(), query).Scan(&rowCount)
		if err != nil {
			utils.WaitChannel <- 0
			fmt.Println(err)
			os.Exit(1)
		}
	}

	if rowCount == -1 { // if var is still not updated
		fmt.Println("couldn't fetch row count of table: " + tableName)
		os.Exit(1)
	}

	return rowCount
}

func GetDriverConnStr(source *utils.Source) string {
	var connStr string
	switch source.DBType {
	case "oracle":
		connStr = fmt.Sprintf("%s/%s@(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%s))(CONNECT_DATA=(SID=%s)))",
			source.User, source.Password, source.Host, source.Port, source.DBName)
	case "mysql":
		connStr = fmt.Sprintf("%s:%s@(%s:%s)/%s", source.User, source.Password,
			source.Host, source.Port, source.DBName)
	case "postgresql":
		connStr = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s", source.User, source.Password,
			source.Host, source.Port, source.DBName, source.SSLMode)
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
			fmt.Println(err)
			os.Exit(1)
		}
		defer db.Close()

		err = db.QueryRow("SELECT VERSION FROM V$INSTANCE").Scan(&version)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "mysql":
		db, err := sql.Open("mysql", dbConnStr)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer db.Close()

		err = db.QueryRow("SELECT VERSION()").Scan(&version)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "postgresql":
		conn, err := pgx.Connect(context.Background(), dbConnStr)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer conn.Close(context.Background())

		err = conn.QueryRow(context.Background(), "SELECT VERSION()").Scan(&version)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "yugabytedb":
		conn, err := pgx.Connect(context.Background(), dbConnStr)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer conn.Close(context.Background())

		err = conn.QueryRow(context.Background(), "SELECT VERSION()").Scan(&version)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	return version
}

func ExportDataPostProcessing(exportDir string, tablesMetadata *[]utils.TableProgressMetadata) {
	dataDirPath := exportDir + "/data"
	// in case of ora2pg the renaming is not required hence will for loop will do nothing
	for _, tableMetadata := range *tablesMetadata {
		oldFilePath := tableMetadata.InProgressFilePath
		newFilePath := dataDirPath + "/" + tableMetadata.TableName + "_data.sql"
		if utils.FileOrFolderExists(oldFilePath) {
			// fmt.Printf("Renaming: %s -> %s\n", filepath.Base(oldFilePath), filepath.Base(newFilePath))
			os.Rename(oldFilePath, newFilePath)
		}
	}

	saveExportedRowCount(exportDir, tablesMetadata)
}

func saveExportedRowCount(exportDir string, tablesMetadata *[]utils.TableProgressMetadata) {
	filePath := exportDir + "/metainfo/flags/tablesrowcount"
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	fmt.Println("actual exported num of rows for each table")
	fmt.Printf("+%s+\n", strings.Repeat("-", 65))
	fmt.Printf("| %30s | %30s |\n", "Table", "Row Count")
	for _, tableMetadata := range *tablesMetadata {
		fmt.Printf("|%s|\n", strings.Repeat("-", 65))
		tableName := tableMetadata.TableName
		actualRowCount := tableMetadata.CountLiveRows
		line := tableName + "," + strconv.FormatInt(actualRowCount, 10) + "\n"
		file.WriteString(line)
		fmt.Printf("| %30s | %30d |\n", tableName, actualRowCount)
	}
	fmt.Printf("+%s+\n", strings.Repeat("-", 65))
}

func CheckSourceDBAccessibility(source *utils.Source) {
	dbConnStr := GetDriverConnStr(source)

	switch source.DBType {
	case "oracle":
		db, err := sql.Open("godror", dbConnStr)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		db.Close()
	case "mysql":
		db, err := sql.Open("mysql", dbConnStr)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		db.Close()
	case "postgresql":
		conn, err := pgx.Connect(context.Background(), dbConnStr)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		conn.Close(context.Background())
	}

	// fmt.Printf("source '%s' database is accessible\n", source.DBType)
}
