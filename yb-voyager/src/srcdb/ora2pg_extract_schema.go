/*
Copyright (c) YugabyteDB, Inc.

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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func ora2pgExtractSchema(source *Source, exportDir string, schemaDir string) {
	configFilePath := filepath.Join(exportDir, "temp", ".ora2pg.conf")
	populateOra2pgConfigFile(configFilePath, getDefaultOra2pgConfig(source))

	for _, exportObject := range source.ExportObjectTypeList {
		if exportObject == "INDEX" {
			continue // INDEX are exported along with TABLE in ora2pg
		}

		fmt.Printf("exporting %10s %5s", exportObject, "")

		go utils.Wait(fmt.Sprintf("%10s\n", "done"), fmt.Sprintf("%10s\n", "error!"))

		exportObjectFileName := utils.GetObjectFileName(schemaDir, exportObject)
		exportObjectDirPath := utils.GetObjectDirPath(schemaDir, exportObject)

		var exportSchemaObjectCommand *exec.Cmd
		if source.DBType == "oracle" {
			exportSchemaObjectCommand = exec.Command("ora2pg", "-p", "-q", "-t", exportObject, "-o",
				exportObjectFileName, "-b", exportObjectDirPath, "-c", configFilePath, "--no_header")
			log.Infof("Executing command: %s", exportSchemaObjectCommand.String())
		} else if source.DBType == "mysql" {
			exportSchemaObjectCommand = exec.Command("ora2pg", "-p", "-m", "-q", "-t", exportObject, "-o",
				exportObjectFileName, "-b", exportObjectDirPath, "-c", configFilePath, "--no_header")
			log.Infof("Executing command: %s", exportSchemaObjectCommand.String())
		}
		exportSchemaObjectCommand.Env = append(os.Environ(), "ORA2PG_PASSWD="+source.Password)
		var outbuf bytes.Buffer
		var errbuf bytes.Buffer
		exportSchemaObjectCommand.Stdout = &outbuf
		exportSchemaObjectCommand.Stderr = &errbuf

		err := exportSchemaObjectCommand.Start()
		if err != nil {
			utils.PrintAndLog("Error while starting export: %v", err)
			utils.WaitChannel <- 1 //stop execution of command with exit code 1
			<-utils.WaitChannel
			continue
		}

		err = exportSchemaObjectCommand.Wait()
		if outbuf.String() != "" {
			log.Infof(`ora2pg STDOUT: "%s"`, outbuf.String())
		}
		if errbuf.String() != "" {
			log.Errorf(`ora2pg STDERR in export of %s : "%s"`, exportObject, errbuf.String())
		}
		if err != nil {
			utils.PrintAndLog("Error while waiting for export command exit: %v", err)
			utils.WaitChannel <- 1 //stop waiting with exit code 1
			<-utils.WaitChannel
			continue
		} else {
			if strings.Contains(strings.ToLower(errbuf.String()), "error") || strings.Contains(strings.ToLower(outbuf.String()), "error") {
				utils.WaitChannel <- 1 //stop waiting with exit code 1
				<-utils.WaitChannel
			} else {
				utils.WaitChannel <- 0 //stop waiting with exit code 0
				<-utils.WaitChannel
			}
		}
		if err := processImportDirectives(utils.GetObjectFilePath(schemaDir, exportObject)); err != nil {
			utils.ErrExit("failed to process import directives during export schema: %v", err.Error())
		}
		if exportObject == "SYNONYM" {
			if err := stripSourceSchemaNames(utils.GetObjectFilePath(schemaDir, exportObject), source.Schema); err != nil {
				utils.ErrExit("failed to strip schema names for SYNONYM object during export schema: %v", err.Error())
			}
		}
		if exportObject == "TABLE" {
			if err := removeReduntantAlterTable(utils.GetObjectFilePath(schemaDir, exportObject)); err != nil {
				utils.ErrExit("failed to remove redundant alter table during export schema: %v", err.Error())
			}
		}
	}
	fmt.Println()
	if source.DBType == "oracle" {
		if err := ora2pgAssessmentReport(source, configFilePath, schemaDir); err != nil {
			utils.ErrExit("failed to save ora2pg oracle assessment report during export schema: %v", err.Error())
		}
	}
}

func ora2pgAssessmentReport(source *Source, configFilePath string, schemaDir string) error {
	// ora2pg_report_cmd="ora2pg -t show_report --estimate_cost -c $OUTPUT_FILE_PATH --dump_as_sheet > $assessment_metadata_dir/schema/ora2pg_report.csv"
	reportCommand := exec.Command("ora2pg", "-t", "show_report", "--estimate_cost", "-c", configFilePath, "--dump_as_sheet")
	outfile, err := os.Create(filepath.Join(schemaDir, "ora2pg_report.csv"))
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer outfile.Close()
	var errbuf bytes.Buffer
	reportCommand.Stdout = outfile
	reportCommand.Stderr = &errbuf
	reportCommand.Env = append(os.Environ(), "ORA2PG_PASSWD="+source.Password)

	log.Info("Executing command to get ora2pg report: ", reportCommand.String())
	err = reportCommand.Start()
	if err != nil {
		return fmt.Errorf("starting ora2pg command: %w", err)
	}

	err = reportCommand.Wait()
	if err != nil {
		return fmt.Errorf("waiting for ora2pg command exit: %w", err)
	}
	if errbuf.String() != "" {
		log.Errorf(`ora2pg STDERR in getting assessment report : "%s"`, errbuf.String())
	}

	if strings.Contains(strings.ToLower(errbuf.String()), "error") {
		return fmt.Errorf("error in stdout of ora2pg assessment report: %s", errbuf.String())
	}
	return nil
}
