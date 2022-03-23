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

	"github.com/yugabyte/ybm/yb_migrate/src/utils"
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

func PgDumpExtractSchema(source *utils.Source, exportDir string) {
	if source.GenerateReportMode {
		fmt.Printf("scanning the schema %10s", "")
	} else {
		fmt.Printf("exporting the schema %10s", "")
	}
	go utils.Wait("done\n", "error\n")

	SSLQueryString := generateSSLQueryStringIfNotExists(source)
	prepareYsqldumpCommandString := ""

	if source.Uri != "" {
		prepareYsqldumpCommandString = fmt.Sprintf(`pg_dump "%s" --schema-only --no-owner -f %s/temp/schema.sql`, source.Uri, exportDir)
	} else {
		prepareYsqldumpCommandString = fmt.Sprintf(`pg_dump "postgresql://%s:%s@%s:%d/%s?%s" --schema-only --no-owner -f %s/temp/schema.sql`, source.User, source.Password, source.Host,
			source.Port, source.DBName, SSLQueryString, exportDir)
	}

	preparedYsqldumpCommand := exec.Command("/bin/bash", "-c", prepareYsqldumpCommandString)

	// fmt.Printf("Executing command: %s\n", preparedYsqldumpCommand)

	err := preparedYsqldumpCommand.Run()
	if err != nil {
		utils.WaitChannel <- 1
		utils.CheckError(err, prepareYsqldumpCommandString, "Retry, dump didn't happen", true)
	}

	//Parsing the single file to generate multiple database object files
	parseSchemaFile(source, exportDir)

	// utils.PrintIfTrue("export of schema done!!!", !source.GenerateReportMode)
	utils.WaitChannel <- 0
}

//NOTE: This is for case when --schema-only option is provided with pg_dump[Data shouldn't be there]
func parseSchemaFile(source *utils.Source, exportDir string) {
	// utils.PrintIfTrue("Parsing the schema file...\n", !source.GenerateReportMode)

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

	ioutil.WriteFile(schemaDirPath+"/types/type.sql", []byte(setSessionVariables.String()+createTypeSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/domains/domain.sql", []byte(setSessionVariables.String()+createDomainSqls.String()), 0644)

	ioutil.WriteFile(schemaDirPath+"/aggregates/aggregate.sql", []byte(setSessionVariables.String()+createAggregateSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/rules/rule.sql", []byte(setSessionVariables.String()+createRuleSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/sequences/sequence.sql", []byte(setSessionVariables.String()+createSequenceSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/views/view.sql", []byte(setSessionVariables.String()+createViewSqls.String()), 0644)

	if uncategorizedSqls.Len() > 0 {
		ioutil.WriteFile(schemaDirPath+"/uncategorized.sql", []byte(setSessionVariables.String()+uncategorizedSqls.String()), 0644)
	}

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

	dataDirPath := exportDir + "/data"

	tableListPatterns := createTableListPatterns(tableList)
	// fmt.Printf("[Debug] createTableListPatterns = %+v\n", tableListPatterns)

	SSLQueryString := generateSSLQueryStringIfNotExists(source)

	//using pgdump for exporting data in directory format
	// pgdumpDataExportCommandArgsString := fmt.Sprintf(`pg_dump "postgresql://%s:%s@%s:%s/%s?%s" --data-only --compress=0 %s -Fd --file %s --jobs %d`, source.User, source.Password,
	// 	source.Host, source.Port, source.DBName, SSLQueryString, tableListPatterns, dataDirPath, source.NumConnections)
	pgdumpDataExportCommandArgsString := ""
	if source.Uri != "" {
		pgdumpDataExportCommandArgsString = fmt.Sprintf(`pg_dump "%s" --data-only --compress=0 %s -Fd --file %s --jobs %d`, source.Uri, tableListPatterns, dataDirPath, source.NumConnections)
	} else {
		pgdumpDataExportCommandArgsString = fmt.Sprintf(`pg_dump "postgresql://%s:%s@%s:%d/%s?%s" --data-only --compress=0 %s -Fd --file %s --jobs %d`, source.User, source.Password,
			source.Host, source.Port, source.DBName, SSLQueryString, tableListPatterns, dataDirPath, source.NumConnections)
	}

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
	// waitingFlag := 0
	for !utils.FileOrFolderExists(tocTextFilePath) {
		// fmt.Printf("Waiting for toc.text file = %s to be created\n", tocTextFilePath)
		// waitingFlag = 1
		time.Sleep(time.Second * 1)
	}

	// if waitingFlag == 1 {
	// 	fmt.Println("toc.txt file got created !!")
	// }

	pgRestoreCmd := exec.Command("pg_restore", "-l", dataDirPath)
	stdOut, err := pgRestoreCmd.Output()
	if err != nil {
		fmt.Println("couldn't parse the TOC to collect the tablenames for data files", err)
		os.Exit(1)
	}

	tableNameVsFileNameMap := make(map[string]string)
	var sequencesPostData strings.Builder

	lines := strings.Split(string(stdOut), "\n")
	for _, line := range lines {
		// example of line: 3725; 0 16594 TABLE DATA public categories ds2
		parts := strings.Split(line, " ")

		if len(parts) < 8 { // those lines don't contain table/sequences related info
			continue
		} else if parts[3] == "TABLE" && parts[4] == "DATA" {
			fileName := strings.Trim(parts[0], ";") + ".dat"
			fullTableName := parts[5] + "." + parts[6]
			tableNameVsFileNameMap[fullTableName] = fileName
		}
	}

	tocTextFileDataBytes, err := ioutil.ReadFile(tocTextFilePath)
	utils.CheckError(err, "", "", true)

	tocTextFileData := strings.Split(string(tocTextFileDataBytes), "\n")
	numLines := len(tocTextFileData)
	setvalRegex := regexp.MustCompile("(?i)SELECT.*setval")

	for i := 0; i < numLines; i++ {
		if setvalRegex.MatchString(tocTextFileData[i]) {
			sequencesPostData.WriteString(tocTextFileData[i])
			sequencesPostData.WriteString("\n")
		}
	}

	//extracted SQL for setval() and put it into a postexport.sql file
	ioutil.WriteFile(dataDirPath+"/postdata.sql", []byte(sequencesPostData.String()), 0644)
	return tableNameVsFileNameMap
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

func createTableListPatterns(tableList []string) string {
	var tableListPattern string

	for _, table := range tableList {
		tableListPattern += fmt.Sprintf("-t %s ", table)
	}

	return tableListPattern
}

func generateSSLQueryStringIfNotExists(s *utils.Source) string {

	if s.Uri == "" {
		SSLQueryString := ""
		if s.SSLQueryString == "" {

			if s.SSLMode == "disable" || s.SSLMode == "allow" || s.SSLMode == "prefer" || s.SSLMode == "require" || s.SSLMode == "verify-ca" || s.SSLMode == "verify-full" {
				SSLQueryString = "sslmode=" + s.SSLMode
				if s.SSLMode == "require" || s.SSLMode == "verify-ca" || s.SSLMode == "verify-full" {
					SSLQueryString = fmt.Sprintf("sslmode=%s", s.SSLMode)
					if s.SSLCertPath != "" {
						SSLQueryString += "&sslcert=" + s.SSLCertPath
					}
					if s.SSLKey != "" {
						SSLQueryString += "&sslkey=" + s.SSLKey
					}
					if s.SSLRootCert != "" {
						SSLQueryString += "&sslrootcert=" + s.SSLRootCert
					}
					if s.SSLCRL != "" {
						SSLQueryString += "&sslcrl=" + s.SSLCRL
					}
				}
			} else {
				fmt.Println("Invalid sslmode entered")
			}
		} else {
			SSLQueryString = s.SSLQueryString
		}
		return SSLQueryString
	} else {
		return ""
	}
}
