package migration

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"yb_migrate/src/utils"
)

var commandNotFoundRegexp *regexp.Regexp = regexp.MustCompile(`(?i)not[ ]+found[ ]+in[ ]+\$PATH`)

// TODO: check for pgdump and psql/ysqlsh - installed/set-in-path
func CheckToolsRequiredForPostgresExport() {
	toolsRequired := []string{"pg_dump", "strings"}

	for _, tool := range toolsRequired {
		checkToolPresenceCommand := exec.Command(tool, "--version")

		err := checkToolPresenceCommand.Run()

		if err != nil {
			if commandNotFoundRegexp.MatchString(err.Error()) {
				log.Fatalf("%s command not found. Check if %s is installed and included in PATH variable", tool, tool)
			} else {
				panic(err)
			}
		}
	}

	fmt.Printf("[Debug] Required tools for export are present...\n")
}

// TODO: fill it, how to using the tools? [psql -c "Select * from version()"]
func PrintPostgresSourceDBVersion(source *utils.Source, exportDir string) {
}

func PgDumpExtractSchema(source *utils.Source, exportDir string) {
	fmt.Printf("Exporting Postgres schema started...\n")
	projectDirPath := utils.GetProjectDirPath(source, exportDir)

	prepareYsqldumpCommandString := fmt.Sprintf(`pg_dump "user=%s password=%s host=%s port=%s dbname=%s sslmode=%s sslrootcert=%s" `+
		`--schema-only > %s/metainfo/schema/schema.sql`, source.User, source.Password, source.Host,
		source.Port, source.DBName, source.SSLMode, source.SSLCertPath, projectDirPath)

	preparedYsqldumpCommand := exec.Command("/bin/bash", "-c", prepareYsqldumpCommandString)

	fmt.Printf("Executing command: %s\n", preparedYsqldumpCommand)

	err := preparedYsqldumpCommand.Run()
	utils.CheckError(err, prepareYsqldumpCommandString, "Retry, dump didn't happen", true)

	//Parsing the single file to generate multiple database object files
	parseSchemaFile(source, exportDir)

	fmt.Println("Export Schema Done!!!")
}

//NOTE: This is for case when --schema-only option is provided with ysql_dump[Data shouldn't be there]
func parseSchemaFile(source *utils.Source, exportDir string) {
	fmt.Printf("Parsing the schema file...\n")

	projectDirPath := utils.GetProjectDirPath(source, exportDir)
	schemaFilePath := projectDirPath + "/metainfo/schema" + "/schema.sql"

	//CHOOSE - bufio vs ioutil(Memory vs Performance)?
	schemaFileData, err := ioutil.ReadFile(schemaFilePath)

	utils.CheckError(err, "", "File not read", true)

	schemaFileLines := strings.Split(string(schemaFileData), "\n")
	numLines := len(schemaFileLines)

	sessionVariableStartPattern, err := regexp.Compile("-- Dumped by pg_dump.*")
	if err != nil {
		panic(err)
	}

	//For example: -- Name: address address_city_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
	sqlTypeInfoCommentPattern, err := regexp.Compile("--.*Type:.*")
	if err != nil {
		panic(err)
	}

	utils.CheckError(err, "", "Couldn't generate the schema", true)

	var createTableSqls, createFkConstraintSqls, createFunctionSqls, createTriggerSqls,
		createIndexSqls, createTypeSqls, createSequenceSqls, createDomainSqls,
		createRuleSqls, createAggregateSqls, createViewSqls, uncategorizedSqls,
		createSchemaSqls, createProcedureSqls, setSessionVariables strings.Builder

	var isPossibleFlag bool = true

	for i := 0; i < numLines; i++ {
		if sqlTypeInfoCommentPattern.MatchString(schemaFileLines[i]) {
			sqlType := extractSqlTypeFromSqlInfoComment(schemaFileLines[i])

			i += 2 //jumping to start of sql statement
			sqlStatement := extractSqlStatements(schemaFileLines, &i)

			//Missing: PARTITION, PROCEDURE, MVIEW, TABLESPACE, ROLE, GRANT ...
			switch sqlType {
			case "TABLE", "DEFAULT", "CONSTRAINT":
				createTableSqls.WriteString(sqlStatement)
			case "FK CONSTRAINT":
				createFkConstraintSqls.WriteString(sqlStatement)
			case "INDEX":
				createIndexSqls.WriteString(sqlStatement)

			case "FUNCTION":
				createFunctionSqls.WriteString(sqlStatement)

			case "PROCEDURE":
				createProcedureSqls.WriteString(sqlStatement)

			case "TRIGGER":
				createTriggerSqls.WriteString(sqlStatement)

			case "TYPE":
				createTypeSqls.WriteString(sqlStatement)
			case "DOMAIN":
				createDomainSqls.WriteString(sqlStatement)

			case "AGGREGATE":
				createAggregateSqls.WriteString(sqlStatement)
			case "RULE":
				createRuleSqls.WriteString(sqlStatement)
			case "SEQUENCE":
				createSequenceSqls.WriteString(sqlStatement)
			case "VIEW":
				createViewSqls.WriteString(sqlStatement)
			case "SCHEMA":
				createSchemaSqls.WriteString(sqlStatement)
			default:
				uncategorizedSqls.WriteString(sqlStatement)
			}
		} else if isPossibleFlag && sessionVariableStartPattern.MatchString(schemaFileLines[i]) {
			i++

			setSessionVariables.WriteString("-- setting variables for current session")
			sqlStatement := extractSqlStatements(schemaFileLines, &i)
			setSessionVariables.WriteString(sqlStatement)

			isPossibleFlag = false
		}
	}

	//TODO: convert below code into a for-loop

	//writing to .sql files in project
	ioutil.WriteFile(projectDirPath+"/schema/tables/table.sql", []byte(setSessionVariables.String()+createTableSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/tables/FKEYS_table.sql", []byte(setSessionVariables.String()+createFkConstraintSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/tables/INDEXES_table.sql", []byte(setSessionVariables.String()+createIndexSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/functions/function.sql", []byte(setSessionVariables.String()+createFunctionSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/procedures/procedure.sql", []byte(setSessionVariables.String()+createProcedureSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/triggers/trigger.sql", []byte(setSessionVariables.String()+createTriggerSqls.String()), 0644)

	//to keep the project structure consistent
	// ioutil.WriteFile(projectDirPath+"/schema/types/type.sql", []byte(setSessionVariables.String() + createTypeSqls.String()+createDomainSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/types/type.sql", []byte(setSessionVariables.String()+createTypeSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/domains/domain.sql", []byte(setSessionVariables.String()+createDomainSqls.String()), 0644)

	ioutil.WriteFile(projectDirPath+"/schema/aggregates/aggregate.sql", []byte(setSessionVariables.String()+createAggregateSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/rules/rule.sql", []byte(setSessionVariables.String()+createRuleSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/sequences/sequence.sql", []byte(setSessionVariables.String()+createSequenceSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/views/view.sql", []byte(setSessionVariables.String()+createViewSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/uncategorized.sql", []byte(setSessionVariables.String()+uncategorizedSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/schemas/schema.sql", []byte(setSessionVariables.String()+createSchemaSqls.String()), 0644)

}

func extractSqlStatements(schemaFileLines []string, index *int) string {
	// fmt.Println("extracting sql statement started...")
	var sqlStatement strings.Builder

	for (*index) < len(schemaFileLines) {
		// fmt.Println((*index), " , ", schemaFileLines[(*index)])
		if isSqlComment(schemaFileLines[(*index)]) {
			break
		} else {
			sqlStatement.WriteString(schemaFileLines[(*index)] + "\n")
		}

		(*index)++
	}

	// fmt.Println("extracting sql statement done...")
	return sqlStatement.String()
}

func isSqlComment(line string) bool {
	return len(line) >= 2 && line[:2] == "--"
}

func extractSqlTypeFromSqlInfoComment(sqlInfoComment string) string {
	sqlInfoCommentSlice := strings.Split(sqlInfoComment, "; ")
	var sqlType strings.Builder
	for _, info := range sqlInfoCommentSlice {
		if info[:4] == "Type" {
			typeInfo := strings.Split(info, ": ")
			typeInfoValue := typeInfo[1]

			for i := 0; i < len(typeInfoValue) && typeInfoValue[i] != ';'; i++ {
				sqlType.WriteByte(typeInfoValue[i])
			}
		}
	}

	return sqlType.String()
}

func PgDumpExportDataOffline(source *utils.Source, exportDir string) {
	CheckToolsRequiredForPostgresExport()

	utils.CreateMigrationProjectIfNotExists(source, exportDir)

	projectDirPath := utils.GetProjectDirPath(source, exportDir)
	dataDirPath := projectDirPath + "/data"

	//using pgdump for exporting data in directory format
	//example: pg_dump postgresql://postgres:postgres@127.0.0.1:5432/sakila?sslmode=disable --verbose --compress=0 --data-only -Fd -f sakila-data-dir
	pgdumpDataExportCommandString := fmt.Sprintf("pg_dump postgresql://%s:%s@%s:%s/%s?"+
		"sslmode=%s --compress=0 --data-only -Fd --file %s --jobs %d", source.User, source.Password,
		source.Host, source.Port, source.DBName, source.SSLMode, dataDirPath, source.NumConnections)

	fmt.Printf("[Debug] Command: %s\n", pgdumpDataExportCommandString)

	pgdumpDataExportCommand := exec.Command("/bin/bash", "-c", pgdumpDataExportCommandString)

	err := pgdumpDataExportCommand.Run()

	utils.CheckError(err, pgdumpDataExportCommandString,
		"Exporting of data failed, retry exporting it", true)

	//Parsing the main toc.dat file
	parseTocFileCommand := exec.Command("strings", dataDirPath+"/toc.dat")

	cmdOutput, err := parseTocFileCommand.CombinedOutput()

	utils.CheckError(err, parseTocFileCommand.String(), string(cmdOutput), true)

	//Put the data into a toc.txt file
	tocTextFilePath := dataDirPath + "/toc.txt"
	tocTextFile, err := os.Create(tocTextFilePath)
	if err != nil {
		panic(err)
	}

	writer := bufio.NewWriter(tocTextFile)
	writer.Write(cmdOutput)

	writer.Flush()
	tocTextFile.Close()

	//TODO: write the mapping creation function
	requiredMap := getMappingForTableFileNameVsTableName(dataDirPath)

	for fileName, tableName := range requiredMap {
		oldFileName := dataDirPath + "/" + fileName
		newFileName := dataDirPath + "/" + tableName + "_data.sql"
		fmt.Printf("Renaming: %s -> %s\n", fileName, tableName+"_data.sql")
		os.Rename(oldFileName, newFileName)
	}

	fmt.Printf("Data  export complete... \n")
}

//The function might be error prone rightnow, will need to verify with other possible toc files. Testing needs to be done
func getMappingForTableFileNameVsTableName(dataDirPath string) map[string]string {
	tocTextFilePath := dataDirPath + "/toc.txt"
	tocTextFileDataBytes, err := ioutil.ReadFile(tocTextFilePath)

	utils.CheckError(err, "", "", true)

	tocTextFileData := strings.Split(string(tocTextFileDataBytes), "\n")
	numLines := len(tocTextFileData)

	var sequencesPostData strings.Builder
	fileNameVsTableNameMap := make(map[string]string)

	for i := 0; i < numLines; i++ {
		if tocTextFileData[i] == "TABLE DATA" {
			fileNameVsTableNameMap[tocTextFileData[i+5]] = tocTextFileData[i-1]
		} else if tocTextFileData[i] == "SEQUENCE SET" {
			sequencesPostData.WriteString(tocTextFileData[i+1])
			sequencesPostData.WriteString("\n")
		}
	}

	//extracted SQL for setval() and put it into a post_data.sql file
	//TODO: May also need to add TRIGGERS ENABLE, FOREIGN KEYS enable
	ioutil.WriteFile(dataDirPath+"/post_data.sql", []byte(sequencesPostData.String()), 0644)

	return fileNameVsTableNameMap
}
