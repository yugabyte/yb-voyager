package srcdb

import (
	"bytes"
	"fmt"
	"os/exec"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

func ora2pgExtractSchema(source *Source, exportDir string) {
	schemaDirPath := exportDir + "/schema"
	configFilePath := exportDir + "/temp/.ora2pg.conf"
	source.PopulateOra2pgConfigFile(configFilePath)

	exportObjectList := utils.GetSchemaObjectList(source.DBType)

	for _, objectType := range exportObjectList {
		if objectType == "INDEX" {
			continue // INDEX are exported along with TABLE in ora2pg
		}

		fmt.Printf("exporting %10s %5s", objectType, "")
		go utils.Wait(fmt.Sprintf("%10s\n", "done"), fmt.Sprintf("%10s\n", "error!"))

		outFile := utils.GetObjectFileName(schemaDirPath, objectType)
		outDir := utils.GetObjectDirPath(schemaDirPath, objectType)

		args := []string{
			"-p", "-q",
			"-t", objectType,
			"-o", outFile,
			"-b", outDir,
			"-c", configFilePath,
			"--no_header",
		}
		if source.DBType == "mysql" {
			args = append(args, "-m")
		}
		cmd := exec.Command("ora2pg", args...)
		log.Infof("Executing command: %s", cmd.String())

		var outbuf bytes.Buffer
		var errbuf bytes.Buffer
		cmd.Stdout = &outbuf
		cmd.Stderr = &errbuf

		err := cmd.Start()
		if err != nil {
			utils.WaitChannel <- 1 //stop waiting with exit code 1
			<-utils.WaitChannel
			utils.ErrExit("Failed to initiate %s export: %v\n%s", objectType, err, errbuf.String())
		}

		err = cmd.Wait()
		log.Infof("ora2pg STDOUT: %s", outbuf.String())
		log.Errorf("ora2pg STDERR: %s", errbuf.String())
		if err != nil {
			utils.WaitChannel <- 1 //stop waiting with exit code 1
			<-utils.WaitChannel
			utils.ErrExit("%s export failed: %v\n%s", objectType, err, errbuf.String())
		}
		utils.WaitChannel <- 0 //stop waiting with exit code 0
		<-utils.WaitChannel
	}
}
