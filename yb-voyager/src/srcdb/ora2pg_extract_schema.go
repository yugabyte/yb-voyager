package srcdb

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func ora2pgExtractSchema(source *Source, exportDir string) {
	schemaDirPath := filepath.Join(exportDir, "schema")
	configFilePath := filepath.Join(exportDir, "temp", ".ora2pg.conf")
	source.PopulateOra2pgConfigFile(configFilePath, source.getDefaultOra2pgConfig())

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
			exportSchemaObjectCommand = exec.Command("ora2pg", "-p", "-q", "-t", exportObject, "-o",
				exportObjectFileName, "-b", exportObjectDirPath, "-c", configFilePath, "--no_header")
			log.Infof("Executing command: %s", exportSchemaObjectCommand.String())
		} else if source.DBType == "mysql" {
			exportSchemaObjectCommand = exec.Command("ora2pg", "-p", "-m", "-q", "-t", exportObject, "-o",
				exportObjectFileName, "-b", exportObjectDirPath, "-c", configFilePath, "--no_header")
			log.Infof("Executing command: %s", exportSchemaObjectCommand.String())
		}

		var outbuf bytes.Buffer
		var errbuf bytes.Buffer
		exportSchemaObjectCommand.Stdout = &outbuf
		exportSchemaObjectCommand.Stderr = &errbuf

		err := exportSchemaObjectCommand.Start()
		if err != nil {
			utils.PrintAndLog("Error while starting export: %v", err)
			exportSchemaObjectCommand.Process.Kill()
			continue
		}

		err = exportSchemaObjectCommand.Wait()
		var errMsgs []string
		if err != nil {
			utils.PrintAndLog("Error while waiting for export command exit: %v", err)
			exportSchemaObjectCommand.Process.Kill()
			continue
		} else {
			for _, line := range strings.Split(outbuf.String(), "\n") {
				if strings.Contains(strings.ToLower(line), "error") {
					errMsgs = append(errMsgs, line)
				}
			}
			for _, line := range strings.Split(errbuf.String(), "\n") {
				if line != "" {
					errMsgs = append(errMsgs, line)
				}
			}
			if len(errMsgs) > 0 {
				utils.WaitChannel <- 1 //stop waiting with exit code 1
				<-utils.WaitChannel
			} else {
				utils.WaitChannel <- 0 //stop waiting with exit code 0
				<-utils.WaitChannel
			}
		}
		if outbuf.String() != "" {
			log.Infof(`ora2pg STDOUT: "%s"`, outbuf.String())
		}
		for _, errMsg := range errMsgs {
			log.Errorf(`ora2pg STDERR in export of %s : "%s"`, exportObject, errMsg)
			fmt.Printf("ora2pg STDERR in export of %s : %s \n", exportObject, errMsg)
		}
		if err := processImportDirectives(utils.GetObjectFilePath(schemaDirPath, exportObject)); err != nil {
			utils.ErrExit(err.Error())
		}
		if exportObject == "SYNONYM" {
			if err := stripSourceSchemaNames(utils.GetObjectFilePath(schemaDirPath, exportObject), source.Schema); err != nil {
				utils.ErrExit(err.Error())
			}
		}
	}

}
