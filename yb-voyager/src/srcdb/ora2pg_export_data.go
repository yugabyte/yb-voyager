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
package srcdb

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func updateOra2pgConfigFileForExportData(configFilePath string, source *Source, tableList []string) {
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
		} else if strings.HasPrefix(line, "DISABLE_PARTITION") {
			lines[i] = "DISABLE_PARTITION 1"
		}
	}

	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile(configFilePath, []byte(output), 0644)
	if err != nil {
		utils.ErrExit("unable to update config file %q: %v\n", configFilePath, err)
	}
}

func ora2pgExportDataOffline(ctx context.Context, source *Source, exportDir string, tableList []string, quitChan chan bool, exportDataStart chan bool, exportSuccessChan chan bool) {
	defer utils.WaitGroup.Done()

	configFilePath := filepath.Join(exportDir, "temp", ".ora2pg.conf")
	tempDirPath := filepath.Join(exportDir, "temp", "ora2pg_temp_dir")
	source.PopulateOra2pgConfigFile(configFilePath)

	updateOra2pgConfigFileForExportData(configFilePath, source, tableList)

	exportDataCommandString := fmt.Sprintf("ora2pg -q -t COPY -P %d -o data.sql -b %s/data -c %s --no_header -T %s",
		source.NumConnections, exportDir, configFilePath, tempDirPath)

	//Exporting all the tables in the schema
	log.Infof("Executing command: %s", exportDataCommandString)
	exportDataCommand := exec.CommandContext(ctx, "/bin/bash", "-c", exportDataCommandString)

	var outbuf bytes.Buffer
	var errbuf bytes.Buffer
	exportDataCommand.Stdout = &outbuf
	exportDataCommand.Stderr = &errbuf

	err := exportDataCommand.Start()
	if err != nil {
		utils.ErrExit("Failed to initiate data export: %v\n%s", err, errbuf.String())
	}
	fmt.Println("Data export started.")
	exportDataStart <- true

	err = exportDataCommand.Wait()
	log.Infof(`ora2pg STDOUT: "%s"`, outbuf.String())
	log.Errorf(`ora2pg STDERR: "%s"`, errbuf.String())
	if err != nil {
		utils.ErrExit("Data export failed: %v\n%s", err, errbuf.String())
	}

	// move to ALTER SEQUENCE commands to postdata.sql file
	extractAlterSequenceStatements(exportDir)
	exportSuccessChan <- true
}

func extractAlterSequenceStatements(exportDir string) {
	alterSequenceRegex := regexp.MustCompile(`(?)ALTER SEQUENCE`)
	filePath := filepath.Join(exportDir, "data", "data.sql")
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

	ioutil.WriteFile(filepath.Join(exportDir, "data", "postdata.sql"), []byte(requiredLines.String()), 0644)
}
