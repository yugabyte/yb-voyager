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
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/yugabyte/ybm/yb_migrate/src/utils"
)

func Ora2PgExtractSchema(source *utils.Source, exportDir string) {
	var schemaDirPath string
	if source.GenerateReportMode {
		schemaDirPath = exportDir + "/temp/schema"
	} else {
		schemaDirPath = exportDir + "/schema"
	}

	//[Internal]: Decide whether to keep ora2pg.conf file hidden or not
	configFilePath := exportDir + "/temp/.ora2pg.conf"
	populateOra2pgConfigFile(configFilePath, source)

	exportObjectList := utils.GetSchemaObjectList(source.DBType)

	for _, exportObject := range exportObjectList {
		time.Sleep(time.Second * 2)
		if exportObject == "INDEX" {
			continue // INDEX are exported along with TABLE in ora2pg
		}
		// utils.PrintIfTrue(fmt.Sprintf("starting export of %ss...\n", strings.ToLower(exportObject)), !source.GenerateReportMode)
		if source.GenerateReportMode {
			fmt.Printf("scanning %10s %5s", exportObject, "")
		} else {
			fmt.Printf("exporting %10s %5s", exportObject, "")
		}
		go utils.Wait(fmt.Sprintf("%10s\n", "done"), fmt.Sprintf("%10s\n", "error!"))

		exportObjectFileName := utils.GetObjectFileName(schemaDirPath, exportObject)
		exportObjectDirPath := utils.GetObjectDirPath(schemaDirPath, exportObject)

		var exportSchemaObjectCommand *exec.Cmd
		if source.DBType == "oracle" {
			exportSchemaObjectCommand = exec.Command("ora2pg", "-p", "-t", exportObject, "-o",
				exportObjectFileName, "-b", exportObjectDirPath, "-c", configFilePath)
		} else if source.DBType == "mysql" {
			exportSchemaObjectCommand = exec.Command("ora2pg", "-p", "-m", "-t", exportObject, "-o",
				exportObjectFileName, "-b", exportObjectDirPath, "-c", configFilePath)
		}

		stdout, _ := exportSchemaObjectCommand.StdoutPipe()
		stderr, _ := exportSchemaObjectCommand.StderrPipe()

		go func() { //command output scanner goroutine
			outScanner := bufio.NewScanner(stdout)
			for outScanner.Scan() {
				line := strings.ToLower(outScanner.Text())
				if strings.Contains(line, "error") {
					utils.WaitChannel <- 1 //stop waiting with exit code 1
					time.Sleep(time.Second * 2)
					fmt.Printf("ERROR: %s\n", line)
					runtime.Goexit()
				}
			}
		}()

		go func() { //command error scanner goroutine
			errScanner := bufio.NewScanner(stderr)
			for errScanner.Scan() {
				line := strings.ToLower(errScanner.Text())
				if strings.Contains(line, "error") {
					utils.WaitChannel <- 1 //stop waiting with exit code 1
					time.Sleep(time.Second * 2)
					fmt.Printf("ERROR: %s\n", line)
					runtime.Goexit()
				}
			}
		}()

		// fmt.Printf("[Debug] exportSchemaObjectCommand: %s\n", exportSchemaObjectCommand.String())
		err := exportSchemaObjectCommand.Start()
		if err != nil {
			fmt.Println(err.Error())
			exportSchemaObjectCommand.Process.Kill()
			continue
		}

		err = exportSchemaObjectCommand.Wait()
		if err != nil {
			fmt.Println(err.Error())
			exportSchemaObjectCommand.Process.Kill()
			continue
		} else {
			utils.WaitChannel <- 0 //stop waiting with exit code 0
			// utils.PrintIfTrue(fmt.Sprintf("export of %ss complete\n", strings.ToLower(exportObject)), !source.GenerateReportMode)
		}
	}
}

//go:embed data/sample-ora2pg.conf
var SampleOra2pgConfigFile string

func populateOra2pgConfigFile(configFilePath string, source *utils.Source) {
	sourceDSN := getSourceDSN(source)

	lines := strings.Split(string(SampleOra2pgConfigFile), "\n")

	//TODO: Add support for SSL Enable Connections
	for i, line := range lines {
		// fmt.Printf("[Debug]: %d %s\n", i, line)
		if strings.HasPrefix(line, "ORACLE_DSN") {
			lines[i] = "ORACLE_DSN	" + sourceDSN
		} else if strings.HasPrefix(line, "ORACLE_USER") {
			// fmt.Println(line)
			lines[i] = "ORACLE_USER	" + source.User
		} else if strings.HasPrefix(line, "ORACLE_HOME") && source.OracleHome != "" {
			// fmt.Println(line)
			lines[i] = "ORACLE_HOME	" + source.OracleHome
		} else if strings.HasPrefix(line, "ORACLE_PWD") {
			lines[i] = "ORACLE_PWD	" + source.Password
		} else if source.DBType == "oracle" && strings.HasPrefix(line, "SCHEMA") {
			if source.Schema != "" { // in oracle USER and SCHEMA are essentially the same thing
				lines[i] = "SCHEMA	" + source.Schema
			} else if source.User != "" {
				lines[i] = "SCHEMA	" + source.User
			}
		} else if strings.HasPrefix(line, "PARALLEL_TABLES") {
			lines[i] = "PARALLEL_TABLES " + strconv.Itoa(source.NumConnections)
		} else if strings.HasPrefix(line, "PG_VERSION") {
			lines[i] = "PG_VERSION " + strconv.Itoa(11) //TODO YugabyteDB compatible with postgres version ?
		}
	}

	output := strings.Join(lines, "\n")
	err := ioutil.WriteFile(configFilePath, []byte(output), 0644)

	utils.CheckError(err, "Not able to update the config file", "", true)
}

func updateOra2pgConfigFileForExportData(configFilePath string, source *utils.Source, tableList []string) {
	basicConfigFile, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		panic(err)
	}

	//ora2pg does accepts table names in format of SCHEMA_NAME.TABLE_NAME
	for i := 0; i < len(tableList); i++ {
		parts := strings.Split(tableList[i], ".")
		tableList[i] = parts[len(parts)-1] // tableList[i] = 'xyz.abc' then take only 'abc'
	}

	lines := strings.Split(string(basicConfigFile), "\n")

	for i, line := range lines {
		if strings.HasPrefix(line, "FILE_PER_TABLE") {
			lines[i] = "FILE_PER_TABLE " + "1"
		} else if strings.HasPrefix(line, "#ALLOW") {
			lines[i] = "ALLOW " + fmt.Sprintf("TABLE%v", tableList)
		}
	}

	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile(configFilePath, []byte(output), 0644)

	utils.CheckError(err, "Not able to update the config file", "", true)
}

func Ora2PgExportDataOffline(ctx context.Context, source *utils.Source, exportDir string, tableList []string, quitChan chan bool, exportDataStart chan bool) {
	defer utils.WaitGroup.Done()

	projectDirPath := exportDir

	//TODO: Decide where to keep this
	configFilePath := projectDirPath + "/temp/.ora2pg.conf"
	populateOra2pgConfigFile(configFilePath, source)

	updateOra2pgConfigFileForExportData(configFilePath, source, tableList)

	exportDataCommandString := fmt.Sprintf("ora2pg -t COPY -P %d -o data.sql -b %s/data -c %s",
		source.NumConnections, projectDirPath, configFilePath)

	//TODO: Exporting only those tables provided in tablelist

	//Exporting all the tables in the schema
	exportDataCommand := exec.Command("/bin/bash", "-c", exportDataCommandString)
	// log.Debugf("exportDataCommand: %s", exportDataCommandString)

	stdOutFile, err := os.OpenFile(exportDir+"/temp/export-data-stdout", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer stdOutFile.Close()

	stdErrFile, err := os.OpenFile(exportDir+"/temp/export-data-stderr", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer stdErrFile.Close()

	exportDataCommand.Stdout = stdOutFile
	exportDataCommand.Stderr = stdErrFile

	err = exportDataCommand.Start()
	fmt.Println("starting ora2pg for data export...")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	exportDataStart <- true

	err = exportDataCommand.Wait()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// move to ALTER SEQUENCE commands to postdata.sql file
	extractAlterSequenceStatements(exportDir)
}

func extractAlterSequenceStatements(exportDir string) {
	alterSequenceRegex := regexp.MustCompile(`(?)ALTER SEQUENCE`)
	filePath := exportDir + "/data/data.sql"
	var requiredLines strings.Builder

	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	lines := strings.Split(string(bytes), "\n")
	for _, line := range lines {
		if alterSequenceRegex.MatchString(line) {
			requiredLines.WriteString(line + "\n")
		}
	}

	ioutil.WriteFile(exportDir+"/data/postdata.sql", []byte(requiredLines.String()), 0644)
}

func getSourceDSN(source *utils.Source) string {
	var sourceDSN string

	if source.DBType == "oracle" {
		if source.DBName != "" {
			sourceDSN = "dbi:Oracle:" + "host=" + source.Host + ";service_name=" +
				source.DBName + ";port=" + source.Port
		} else if source.DBSid != "" {
			sourceDSN = "dbi:Oracle:" + "host=" + source.Host + ";sid=" +
				source.DBSid + ";port=" + source.Port
		} else {
			sourceDSN = "dbi:Oracle:" + source.TNSAlias //this option is ideal for ssl connectivity, provide in documentation if needed
		}
	} else if source.DBType == "mysql" {
		parseSSLString(source)
		sourceDSN = "dbi:mysql:" + "host=" + source.Host + ";database=" +
			source.DBName + ";port=" + source.Port
		sourceDSN = extrapolateDSNfromSSLParams(source, sourceDSN)
	} else {
		fmt.Println("Invalid Source DB Type!!")
		os.Exit(1)
	}

	return sourceDSN
}

func OracleGetAllTableNames(source *utils.Source) []string {
	dbConnStr := GetDriverConnStr(source)
	db, err := sql.Open("godror", dbConnStr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer db.Close()

	var tableNames []string
	query := fmt.Sprintf("SELECT table_name FROM dba_tables "+
		"WHERE owner = '%s' ORDER BY table_name ASC", source.Schema)
	rows, err := db.Query(query)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer rows.Close()
	for rows.Next() {
		var tableName string
		err = rows.Scan(&tableName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		tableNames = append(tableNames, tableName)
	}
	return tableNames
}
