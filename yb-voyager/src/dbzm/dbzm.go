package dbzm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

var DEBEZIUM_DIST_DIR, DEBEZIUM_CONF_DIR, DEBEZIUM_CONF_FILEPATH string

type Debezium struct {
	*Config
	cmd  *exec.Cmd
	err  error
	done bool
}

func init() {
	if distDir := os.Getenv("DEBEZIUM_DIST_DIR"); distDir != "" {
		DEBEZIUM_DIST_DIR = distDir
	} else {
		DEBEZIUM_DIST_DIR = "/etc/yb-voyager/debezium"
	}

	DEBEZIUM_CONF_DIR = filepath.Join(DEBEZIUM_DIST_DIR, "conf")
	DEBEZIUM_CONF_FILEPATH = filepath.Join(DEBEZIUM_CONF_DIR, "application.properties")
}

func NewDebezium(config *Config) *Debezium {
	return &Debezium{Config: config}
}

func (d *Debezium) Start() error {
	err := d.Config.WriteToFile(DEBEZIUM_CONF_FILEPATH)
	if err != nil {
		return err
	}

	log.Infof("starting debezium...")
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

// stops debezium process if it is running
func (d *Debezium) Stop() error {
	if d.IsRunning() {
		log.Infof("Stopping debezium...")
		err := d.cmd.Process.Kill()
		if err != nil {
			return fmt.Errorf("Error stopping debezium: %v", err)
		}
	}
	return nil
}
