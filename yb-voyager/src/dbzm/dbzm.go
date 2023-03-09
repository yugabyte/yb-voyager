package dbzm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var DEBEZIUM_DIST_DIR, DEBEZIUM_CONF_DIR, DEBEZIUM_CONF_FILEPATH string

type Debezium struct {
	*Config
	cmd  *exec.Cmd
	err  error
	done bool
}

func NewDebezium(config *Config) *Debezium {
	return &Debezium{Config: config}
}

func initVars() {
	// take value of DEBEZIUM_DIST_DIR value from environment variables
	if envVal := os.Getenv("DEBEZIUM_DIST_DIR"); envVal != "" {
		DEBEZIUM_DIST_DIR = envVal
	} else {
		utils.ErrExit("DEBEZIUM_DIST_DIR environment variable is not set")
	}

	DEBEZIUM_CONF_DIR = filepath.Join(DEBEZIUM_DIST_DIR, "conf")
	DEBEZIUM_CONF_FILEPATH = filepath.Join(DEBEZIUM_CONF_DIR, "application.properties")
}

func (d *Debezium) Start() error {
	initVars()
	err := d.Config.WriteToFile(DEBEZIUM_CONF_FILEPATH)
	if err != nil {
		return err
	}

	utils.PrintAndLog("starting streaming changes from source DB...")
	logFile, _ := filepath.Abs(filepath.Join(d.ExportDir, "debezium.log"))
	log.Infof("debezium logfile path: %s\n", logFile)

	cmdStr := fmt.Sprintf("cd %q; %s > %s 2>&1", DEBEZIUM_DIST_DIR, filepath.Join(DEBEZIUM_DIST_DIR, "run.sh"), logFile)
	log.Infof("running command: %s\n", cmdStr)
	d.cmd = exec.Command("/bin/bash", "-c", cmdStr)
	err = d.cmd.Start()
	if err != nil {
		return fmt.Errorf("Error starting debezium: %v", err)
	}

	log.Infof("Debezium started successfully")
	go func() {
		d.err = d.cmd.Wait()
		d.done = true
		if d.err != nil {
			log.Errorf("Debezium exited with: %v", d.err)
		}
	}()
	return nil
}

func (d *Debezium) IsRunning() bool {
	return d.cmd.Process != nil && !d.done
}

func (d *Debezium) Error() error {
	return d.err
}

func (d *Debezium) GetExportStatus() (*ExportStatus, error) {
	statusFilePath := filepath.Join(d.ExportDir, "data", "export_status.json")
	return ReadExportStatus(statusFilePath)
}
