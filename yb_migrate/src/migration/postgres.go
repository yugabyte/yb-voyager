package migration

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
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
func PrintPostgresSourceDBVersion(source *utils.Source) {
}

func PgDumpExtractSchema(source *utils.Source, exportDir string) {
	utils.PrintIfTrue("Exporting Postgres schema started...\n", !source.GenerateReportMode)

	prepareYsqldumpCommandString := fmt.Sprintf(`pg_dump "user=%s password=%s host=%s port=%s dbname=%s sslmode=%s sslrootcert=%s" `+
		`--schema-only -f %s/temp/schema.sql`, source.User, source.Password, source.Host,
		source.Port, source.DBName, source.SSLMode, source.SSLCertPath, exportDir)

	preparedYsqldumpCommand := exec.Command("/bin/bash", "-c", prepareYsqldumpCommandString)

	// fmt.Printf("Executing command: %s\n", preparedYsqldumpCommand)

	err := preparedYsqldumpCommand.Run()
	utils.CheckError(err, prepareYsqldumpCommandString, "Retry, dump didn't happen", true)

	//Parsing the single file to generate multiple database object files
	parseSchemaFile(source, exportDir)

	utils.PrintIfTrue("export of schema done!!!", !source.GenerateReportMode)
}

//NOTE: This is for case when --schema-only option is provided with ysql_dump[Data shouldn't be there]
func parseSchemaFile(source *utils.Source, exportDir string) {
	utils.PrintIfTrue("Parsing the schema file...\n", !source.GenerateReportMode)

	schemaFilePath := exportDir + "/temp" + "/schema.sql"
	var schemaDirPath string
	if source.GenerateReportMode {
		schemaDirPath = exportDir + "/temp/schema"
	} else {
		schemaDirPath = exportDir + "/schema"
	}

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

	var createTableSqls, createFunctionSqls, createTriggerSqls,
		createIndexSqls, createTypeSqls, createSequenceSqls, createDomainSqls,
		createRuleSqls, createAggregateSqls, createViewSqls, uncategorizedSqls,
		createSchemaSqls, createExtensionSqls, createProcedureSqls, setSessionVariables strings.Builder

	var isPossibleFlag bool = true

	for i := 0; i < numLines; i++ {
		if sqlTypeInfoCommentPattern.MatchString(schemaFileLines[i]) {
			sqlType := extractSqlTypeFromSqlInfoComment(schemaFileLines[i])

			i += 2 //jumping to start of sql statement
			sqlStatement := extractSqlStatements(schemaFileLines, &i)

			//Missing: PARTITION, PROCEDURE, MVIEW, TABLESPACE, ROLE, GRANT ...
			switch sqlType {
			case "TABLE", "DEFAULT", "CONSTRAINT", "FK CONSTRAINT":
				createTableSqls.WriteString(sqlStatement)
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
			case "EXTENSION":
				createExtensionSqls.WriteString(sqlStatement)
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
	ioutil.WriteFile(schemaDirPath+"/tables/table.sql", []byte(setSessionVariables.String()+createTableSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/tables/INDEXES_table.sql", []byte(setSessionVariables.String()+createIndexSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/functions/function.sql", []byte(setSessionVariables.String()+createFunctionSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/procedures/procedure.sql", []byte(setSessionVariables.String()+createProcedureSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/triggers/trigger.sql", []byte(setSessionVariables.String()+createTriggerSqls.String()), 0644)

	//to keep the project structure consistent
	// ioutil.WriteFile(projectDirPath+"/types/type.sql", []byte(setSessionVariables.String() + createTypeSqls.String()+createDomainSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/types/type.sql", []byte(setSessionVariables.String()+createTypeSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/domains/domain.sql", []byte(setSessionVariables.String()+createDomainSqls.String()), 0644)

	ioutil.WriteFile(schemaDirPath+"/aggregates/aggregate.sql", []byte(setSessionVariables.String()+createAggregateSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/rules/rule.sql", []byte(setSessionVariables.String()+createRuleSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/sequences/sequence.sql", []byte(setSessionVariables.String()+createSequenceSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/views/view.sql", []byte(setSessionVariables.String()+createViewSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/uncategorized.sql", []byte(setSessionVariables.String()+uncategorizedSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/schemas/schema.sql", []byte(setSessionVariables.String()+createSchemaSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/extensions/extension.sql", []byte(setSessionVariables.String()+createExtensionSqls.String()), 0644)

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

func PgDumpExportDataOffline(ctx context.Context, source *utils.Source, exportDir string, tableList []string, quitChan chan bool, exportDataStart chan bool) {
	defer utils.WaitGroup.Done()
	CheckToolsRequiredForPostgresExport()

	utils.CreateMigrationProjectIfNotExists(source, exportDir)

	dataDirPath := exportDir + "/data"

	tableListRegex := createTableListRegex(tableList)

	//using pgdump for exporting data in directory format
	pgdumpDataExportCommandArgsString := fmt.Sprintf("pg_dump postgresql://%s:%s@%s:%s/%s?"+
		"sslmode=%s --compress=0 --data-only -t '%s' -Fd --file %s --jobs %d", source.User, source.Password,
		source.Host, source.Port, source.DBName, source.SSLMode, tableListRegex, dataDirPath, source.NumConnections)

	// fmt.Printf("[Debug] Command: %s\n", pgdumpDataExportCommandArgsString)

	pgdumpDataExportCommand := exec.CommandContext(ctx, "/bin/bash", "-c", pgdumpDataExportCommandArgsString)

	var stderrBuffer bytes.Buffer
	pgdumpDataExportCommand.Stderr = &stderrBuffer
	pgdumpDataExportCommand.Stdout = &stderrBuffer

	err := pgdumpDataExportCommand.Start()
	fmt.Println("pg_dump for data export started")
	if err != nil {
		fmt.Printf("%s\n%s\n", stderrBuffer.String(), err)
		quitChan <- true
		runtime.Goexit()
	}
	exportDataStart <- true

	//Parsing the main toc.dat file in parallel
	go parseAndCreateTocTextFile(dataDirPath)

	//Wait for pg_dump to complete before renaming of data files
	err = pgdumpDataExportCommand.Wait()
	if err != nil {
		fmt.Printf("%s\n%s\n", stderrBuffer.String(), err)
		quitChan <- true
		runtime.Goexit()
	}

}

//The function might be error prone rightnow, will need to verify with other possible toc files. Testing needs to be done
func getMappingForTableNameVsTableFileName(dataDirPath string) map[string]string {
	tocTextFilePath := dataDirPath + "/toc.txt"
	waitingFlag := 0
	for !utils.FileOrFolderExists(tocTextFilePath) {
		// fmt.Printf("Waiting for toc.text file = %s to be created\n", tocTextFilePath)
		waitingFlag = 1
		time.Sleep(time.Second * 3)
	}

	if waitingFlag == 1 {
		// fmt.Println("toc.txt file got created !!")
	}

	tocTextFileDataBytes, err := ioutil.ReadFile(tocTextFilePath)

	utils.CheckError(err, "", "", true)

	tocTextFileData := strings.Split(string(tocTextFileDataBytes), "\n")
	numLines := len(tocTextFileData)

	var sequencesPostData strings.Builder
	fileNameVsTableNameMap := make(map[string]string)

	for i := 0; i < numLines; i++ {
		if tocTextFileData[i] == "TABLE DATA" {
			fileNameVsTableNameMap[tocTextFileData[i-1]] = tocTextFileData[i+5]
		} else if tocTextFileData[i] == "SEQUENCE SET" {
			sequencesPostData.WriteString(tocTextFileData[i+1])
			sequencesPostData.WriteString("\n")
		}
	}

	//extracted SQL for setval() and put it into a postexport.sql file
	//TODO: May also need to add TRIGGERS ENABLE, FOREIGN KEYS enable
	ioutil.WriteFile(dataDirPath+"/postexport.sql", []byte(sequencesPostData.String()), 0644)

	return fileNameVsTableNameMap
}

func parseAndCreateTocTextFile(dataDirPath string) {
	tocFilePath := dataDirPath + "/toc.dat"
	waitingFlag := 0
	for !utils.FileOrFolderExists(tocFilePath) {
		// fmt.Printf("Waiting for toc.dat file = %s to be created\n", tocFilePath)
		waitingFlag = 1
		time.Sleep(time.Second * 3)
	}

	if waitingFlag == 1 {
		// fmt.Println("toc.dat file got created !!")
	}

	parseTocFileCommand := exec.Command("strings", tocFilePath)

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
}

func createTableListRegex(tableList []string) string {
	var tableListRegex string

	for _, table := range tableList {
		if strings.ContainsRune(table, '.') {
			tableListRegex += strings.Split(table, ".")[1] + "|"
		} else {
			tableListRegex += table + "|"
		}
	}

	if len(tableList) > 0 { //removing last '|'
		tableListRegex = tableListRegex[0 : len(tableListRegex)-1]
	}
	return tableListRegex
}
