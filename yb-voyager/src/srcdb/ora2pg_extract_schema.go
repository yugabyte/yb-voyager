package srcdb

import (
	"bufio"
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

		stdout, _ := exportSchemaObjectCommand.StdoutPipe()
		stderr, _ := exportSchemaObjectCommand.StderrPipe()

		var errMsgs []string
		go func() { //command output scanner goroutine
			outScanner := bufio.NewScanner(stdout)
			for outScanner.Scan() {
				line := strings.ToLower(outScanner.Text())
				if strings.Contains(line, "error") {
					errMsgs = append(errMsgs, outScanner.Text())
				} else {
					log.Infof("ora2pg STDOUT: \"%s\"", outScanner.Text())
				}
			}
		}()

		go func() { //command error scanner goroutine
			errScanner := bufio.NewScanner(stderr)
			for errScanner.Scan() {
				errMsgs = append(errMsgs, errScanner.Text())
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
			if len(errMsgs) > 0 {
				utils.WaitChannel <- 1 //stop waiting with exit code 1
				<-utils.WaitChannel
			} else {
				utils.WaitChannel <- 0 //stop waiting with exit code 0
				<-utils.WaitChannel
			}
		}

		for _, err := range errMsgs {
			utils.PrintAndLog("ora2pg STDERR: \"%s\"", err)
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
