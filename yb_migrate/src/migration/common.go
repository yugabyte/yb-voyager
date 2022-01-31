package migration

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"yb_migrate/src/utils"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/godror/godror"
	_ "github.com/lib/pq"
)

func UpdateDataFilePath(source *utils.Source, exportDir string, tablesMetadata []utils.TableProgressMetadata) {
	var requiredMap map[string]string

	// TODO: handle the case if table name has double quotes/case sensitive

	if source.DBType == "postgresql" {
		requiredMap = getMappingForTableNameVsTableFileName(exportDir + "/data")
		for i := 0; i < len(tablesMetadata); i++ {
			tablesMetadata[i].DataFilePath = exportDir + "/data/" + requiredMap[tablesMetadata[i].TableName]
		}
	} else { //for Oracle and MySQL
		for i := 0; i < len(tablesMetadata); i++ {
			fileName := "tmp_" + strings.ToUpper(tablesMetadata[i].TableName) + "_data.sql"
			tablesMetadata[i].DataFilePath = exportDir + "/data/" + fileName
		}

	}

	// fmt.Println("After updating datafilepath")
	// fmt.Printf("TableMetadata: %v\n\n", tablesMetadata)
}

func UpdateTableRowCount(source *utils.Source, exportDir string, tablesMetadata []utils.TableProgressMetadata) {
	fmt.Println("calculating num of rows in all tables...")
	for i := 0; i < len(tablesMetadata); i++ {
		tableName := tablesMetadata[i].TableName
		rowCount := SelectCountStarFromTable(tableName, source)
		tablesMetadata[i].CountTotalRows = rowCount
		fmt.Printf("row count of %s = %d\n", tableName, rowCount)
	}

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
	var driverName string

	if source.DBType == "oracle" {
		driverName = "godror"
	} else if source.DBType == "postgresql" {
		driverName = "postgres" //postgresql keyword is not accepted
	} else if source.DBType == "mysql" {
		driverName = "mysql"
	}

	dbConnStr := getDriverConnStr(source)
	db, err := sql.Open(driverName, dbConnStr)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	row, err := db.Query(fmt.Sprintf("select count(*) from %s", tableName))
	if err != nil {
		panic(err)
	}
	defer row.Close()

	if row.Next() {
		var rowValue string
		row.Scan(&rowValue)
		rowCount, _ = strconv.ParseInt(rowValue, 10, 64)
	}

	if rowCount == -1 { // if var is still not updated
		panic("couldn't fetch row count of table: " + tableName)
	}

	return rowCount
}

func getDriverConnStr(source *utils.Source) string {
	var connStr string
	switch source.DBType {
	case "oracle":
		connStr = fmt.Sprintf("%s/%s@(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%s))(CONNECT_DATA=(SID=%s)))",
			source.User, source.Password, source.Host, source.Port, source.DBName)
	case "mysql":
		connStr = fmt.Sprintf("%s:%s@(%s:%s)/%s", source.User, source.Password,
			source.Host, source.Port, source.DBName)
	case "postgresql":
		connStr = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", source.User, source.Password,
			source.Host, source.Port, source.DBName)
	}
	return connStr
}

func PrintSourceDBVersion(source *utils.Source) {
	if source.DBType == "oracle" {
		PrintOracleSourceDBVersion(source)
	} else if source.DBType == "mysql" {
		PrintMySQLSourceDBVersion(source)
	} else if source.DBType == "postgresql" {
		PrintPostgresSourceDBVersion(source)
	}
}

func ExportDataPostProcessing(exportDir string, tablesMetadata *[]utils.TableProgressMetadata) {
	dataDirPath := exportDir + "/data"
	// in case of ora2pg the renaming is not required hence will for loop will do nothing
	for _, tableMetadata := range *tablesMetadata {
		oldFilePath := tableMetadata.DataFilePath
		newFilePath := dataDirPath + "/" + tableMetadata.TableName + "_data.sql"
		if utils.FileOrFolderExists(oldFilePath) {
			// fmt.Printf("Renaming: %s -> %s\n", filepath.Base(oldFilePath), filepath.Base(newFilePath))
			os.Rename(oldFilePath, newFilePath)
		}
	}

	saveExportedRowCount(exportDir, tablesMetadata)
}

func saveExportedRowCount(exportDir string, tablesMetadata *[]utils.TableProgressMetadata) {
	filePath := exportDir + "/metainfo/data/tablesrowcount.csv"
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	for _, tableMetadata := range *tablesMetadata {
		tableName := tableMetadata.TableName
		actualRowCount := tableMetadata.CountLiveRows

		line := tableName + "," + strconv.FormatInt(actualRowCount, 10) + "\n"
		file.WriteString(line)
	}
}
