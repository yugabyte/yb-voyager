package migration

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
	"yb_migrate/src/utils"
)

func UpdateDataFilePath(source *utils.Source, exportDir string, tablesMetadata []utils.ExportTableMetadata) {
	var requiredMap map[string]string

	if source.DBType == "postgresql" {
		requiredMap = getMappingForTableNameVsTableFileName(exportDir + "/data")
	} else {
		//TODO: Implement for Oracle and Mysql
	}

	for i := 0; i < len(tablesMetadata); i++ {
		tablesMetadata[i].DataFilePath = exportDir + "/data/" + requiredMap[tablesMetadata[i].TableName]
	}

	// fmt.Println("After updating datafilepath")
	// fmt.Printf("TableMetadata: %v\n\n", tablesMetadata)
}

func UpdateTableRowCount(source *utils.Source, exportDir string, tablesMetadata []utils.ExportTableMetadata) {
	//TODO: Change this once report generation code is inplace, rightnow this is hardcoded

	rowCountFilePath := "/home/centos/yb_migrate_projects/"
	if source.DBType == "oracle" {
		rowCountFilePath += source.Schema + "_rc.txt"
	} else {
		rowCountFilePath += source.DBName + "_rc.txt"
	}

	tableRowCountMap := GetTableRowCount(rowCountFilePath)
	for i := 0; i < len(tablesMetadata); i++ {
		tablesMetadata[i].CountTotalRows = tableRowCountMap[tablesMetadata[i].TableName]
	}

	fmt.Println("After updating total row count")
	fmt.Printf("TableMetadata: %v\n\n", tablesMetadata)
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
		tableName := strings.Split(line, "=")[0]
		rowCount := strings.Split(line, "=")[1]
		rowCountInt64, _ := strconv.ParseInt(rowCount, 10, 64)

		tableRowCountMap[tableName] = rowCountInt64
	}

	return tableRowCountMap
}
