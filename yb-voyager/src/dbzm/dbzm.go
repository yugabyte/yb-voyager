package dbzm

import (
	"fmt"
	"os/exec"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const DEBEZIUM_DIST_DIR = "/home/amit.jambure/debezium-server/"
const DEBEZIUM_CONF_DIR = DEBEZIUM_DIST_DIR + "conf/"
const DEBEZIUM_CONF_FILEPATH = DEBEZIUM_CONF_DIR + "application.properties"

type Debezium struct {
	*Config
	cmd  *exec.Cmd
	err  error
	done bool
}

func NewDebezium(config *Config) *Debezium {
	return &Debezium{Config: config}
}

func (d *Debezium) Start() error {
	err := d.Config.WriteToFile(DEBEZIUM_CONF_FILEPATH)
	if err != nil {
		return err
	}
	utils.PrintAndLog("Starting debezium...")
	logFile := filepath.Join(d.ExportDir, "debezium.log")
	cmdStr := fmt.Sprintf("cd %q; %s > %s 2>&1", DEBEZIUM_DIST_DIR, filepath.Join(DEBEZIUM_DIST_DIR, "run.sh"), logFile)
	log.Infof("Running command: %s\n", cmdStr)
	d.cmd = exec.Command("/bin/bash", "-c", cmdStr)
	err = d.cmd.Start()
	if err != nil {
		return fmt.Errorf("Error starting debezium: %v", err)
	}
	utils.PrintAndLog("Debezium started successfully")
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
