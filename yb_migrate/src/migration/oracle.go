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
	"database/sql"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/srcdb"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

func Ora2PgExtractSchema(source *srcdb.Source, exportDir string) {
	schemaDirPath := exportDir + "/schema"
	configFilePath := exportDir + "/temp/.ora2pg.conf"
	populateOra2pgConfigFile(configFilePath, source)

	exportObjectList := utils.GetSchemaObjectList(source.DBType)

	for _, exportObject := range exportObjectList {
		if exportObject == "INDEX" {
			continue // INDEX are exported along with TABLE in ora2pg
		}

		fmt.Printf("exporting %10s %5s", exportObject, "")

		go utils.Wait(fmt.Sprintf("%10s\n", "done"), fmt.Sprintf("%10s\n", "error!"))

		exportObjectFileName := utils.GetObjectFileName(schemaDirPath, exportObject)
		exportObjectDirPath := utils.GetObjectDirPath(schemaDirPath, exportObject)

		var exportSchemaObjectCommand *exec.Cmd
		if source.DBType == "oracle" {
			exportSchemaObjectCommand = exec.Command("ora2pg", "-p", "-t", exportObject, "-o",
				exportObjectFileName, "-b", exportObjectDirPath, "-c", configFilePath)
			log.Infof("Executing command: %s", exportSchemaObjectCommand.String())
		} else if source.DBType == "mysql" {
			exportSchemaObjectCommand = exec.Command("ora2pg", "-p", "-m", "-t", exportObject, "-o",
				exportObjectFileName, "-b", exportObjectDirPath, "-c", configFilePath)
			log.Infof("Executing command: %s", exportSchemaObjectCommand.String())
		}

		stdout, _ := exportSchemaObjectCommand.StdoutPipe()
		stderr, _ := exportSchemaObjectCommand.StderrPipe()

		go func() { //command output scanner goroutine
			outScanner := bufio.NewScanner(stdout)
			for outScanner.Scan() {
				line := strings.ToLower(outScanner.Text())
				if strings.Contains(line, "error") {
					utils.WaitChannel <- 1 //stop waiting with exit code 1
					<-utils.WaitChannel
					log.Infof("ERROR in output scanner goroutine: %s", line)
					runtime.Goexit()
				}
				log.Infof("ora2pg STDOUT: %s", outScanner.Text())
			}
		}()

		go func() { //command error scanner goroutine
			errScanner := bufio.NewScanner(stderr)
			for errScanner.Scan() {
				line := strings.ToLower(errScanner.Text())
				if strings.Contains(line, "error") {
					utils.WaitChannel <- 1 //stop waiting with exit code 1
					<-utils.WaitChannel
					log.Infof("ERROR in error scanner goroutine: %s", line)
					runtime.Goexit()
				}
			}
		}()

		err := exportSchemaObjectCommand.Start()
		if err != nil {
			utils.PrintAndLog("Error while starting export: %v", err)
			exportSchemaObjectCommand.Process.Kill()
			continue
		}

		err = exportSchemaObjectCommand.Wait()
		if err != nil {
			utils.PrintAndLog("Error while waiting for export command exit: %v", err)
			exportSchemaObjectCommand.Process.Kill()
			continue
		} else {
			utils.WaitChannel <- 0 //stop waiting with exit code 0
			<-utils.WaitChannel
		}
	}
}

//go:embed data/sample-ora2pg.conf
var SampleOra2pgConfigFile string

func populateOra2pgConfigFile(configFilePath string, source *srcdb.Source) {
	sourceDSN := getSourceDSN(source)

	lines := strings.Split(string(SampleOra2pgConfigFile), "\n")

	for i, line := range lines {
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
	if err != nil {
		utils.ErrExit("unable to update config file %q: %v\n", configFilePath, err)
	}
}

func updateOra2pgConfigFileForExportData(configFilePath string, source *srcdb.Source, tableList []string) {
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
	if err != nil {
		utils.ErrExit("unable to update config file %q: %v\n", configFilePath, err)
	}
}

func Ora2PgExportDataOffline(ctx context.Context, source *srcdb.Source, exportDir string, tableList []string, quitChan chan bool, exportDataStart chan bool) {
	defer utils.WaitGroup.Done()

	projectDirPath := exportDir
	configFilePath := projectDirPath + "/temp/.ora2pg.conf"
	populateOra2pgConfigFile(configFilePath, source)

	updateOra2pgConfigFileForExportData(configFilePath, source, tableList)

	exportDataCommandString := fmt.Sprintf("ora2pg -t COPY -P %d -o data.sql -b %s/data -c %s",
		source.NumConnections, projectDirPath, configFilePath)

	//Exporting all the tables in the schema
	exportDataCommand := exec.CommandContext(ctx, "/bin/bash", "-c", exportDataCommandString)
	log.Infof("Executing command: %s", exportDataCommandString)
	var outbuf bytes.Buffer
	var errbuf bytes.Buffer

	exportDataCommand.Stdout = &outbuf
	exportDataCommand.Stderr = &errbuf

	err := exportDataCommand.Start()
	fmt.Println("starting ora2pg for data export...")
	if outbuf.String() != "" {
		log.Infof("ora2pg STDOUT: %s", outbuf.String())
	}
	if err != nil {
		utils.ErrExit("Error while starting ora2pg for data export: %v\n%s", err, errbuf.String())
	}

	exportDataStart <- true

	err = exportDataCommand.Wait()
	if err != nil {
		utils.ErrExit("Error while waiting for ora2pg to exit: %v\n%s", err, errbuf.String())
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

func getSourceDSN(source *srcdb.Source) string {
	var sourceDSN string

	if source.DBType == "oracle" {
		if source.DBName != "" {
			sourceDSN = fmt.Sprintf("dbi:Oracle:host=%s;service_name=%s;port=%d", source.Host, source.DBName, source.Port)
		} else if source.DBSid != "" {
			sourceDSN = fmt.Sprintf("dbi:Oracle:host=%s;sid=%s;port=%d", source.Host, source.DBSid, source.Port)
		} else {
			sourceDSN = fmt.Sprintf("dbi:Oracle:%s", source.TNSAlias) //this option is ideal for ssl connectivity, provide in documentation if needed
		}
	} else if source.DBType == "mysql" {
		parseSSLString(source)
		sourceDSN = fmt.Sprintf("dbi:mysql:host=%s;database=%s;port=%d", source.Host, source.DBName, source.Port)
		sourceDSN = extrapolateDSNfromSSLParams(source, sourceDSN)
	} else {
		utils.ErrExit("Invalid Source DB Type.")
	}

	log.Infof("Source DSN used for export: %s", sourceDSN)
	return sourceDSN
}

func OracleGetAllPartitionNames(source *srcdb.Source, tableName string) []string {
	dbConnStr := GetDriverConnStr(source)
	db, err := sql.Open("godror", dbConnStr)
	if err != nil {
		utils.ErrExit("error in opening connections to database: %v", err)
	}
	defer db.Close()

	query := fmt.Sprintf("SELECT partition_name FROM all_tab_partitions "+
		"WHERE table_name = '%s' AND table_owner = '%s' ORDER BY partition_name ASC",
		tableName, source.Schema)
	rows, err := db.Query(query)
	if err != nil {
		utils.ErrExit("error in query table %q for partition names: %v", tableName, err)
	}
	defer rows.Close()

	var partitionNames []string
	for rows.Next() {
		var partitionName string
		err = rows.Scan(&partitionName)
		if err != nil {
			utils.ErrExit("error in scanning query rows: %v", err)
		}
		partitionNames = append(partitionNames, partitionName)

		// TODO: Support subpartition(find subparititions for each partition)
	}

	log.Infof("Partition Names for parent table %q: %q", tableName, partitionNames)
	return partitionNames
}
