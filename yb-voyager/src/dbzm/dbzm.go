package dbzm

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/tebeka/atexit"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"

	// "syscall"

	log "github.com/sirupsen/logrus"
)

var DEBEZIUM_DIST_DIR, DEBEZIUM_CONF_FILEPATH string

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
		DEBEZIUM_DIST_DIR = "/opt/yb-voyager/debezium-server"
	}
}

func NewDebezium(config *Config) *Debezium {
	return &Debezium{Config: config}
}

func (d *Debezium) Start() error {
	DEBEZIUM_CONF_FILEPATH = filepath.Join(d.ExportDir, "metainfo", "conf", "application.properties")
	err := d.Config.WriteToFile(DEBEZIUM_CONF_FILEPATH)
	if err != nil {
		return err
	}

	log.Infof("starting debezium...")
	logFile, _ := filepath.Abs(filepath.Join(d.ExportDir, "logs/debezium.log"))
	log.Infof("debezium logfile path: %s\n", logFile)

	// cmdStr := fmt.Sprintf("%s %s > %s 2>&1", filepath.Join(DEBEZIUM_DIST_DIR, "run.sh"), DEBEZIUM_CONF_FILEPATH, logFile)
	// log.Infof("running command: %s\n", cmdStr)
	// d.cmd = exec.Command("/bin/bash", "-c", cmdStr)
	d.cmd = exec.Command(filepath.Join(DEBEZIUM_DIST_DIR, "run.sh"), DEBEZIUM_CONF_FILEPATH)
	// open the out file for writing
	outfile, err := os.Create(logFile)
	if err != nil {
		panic(err)
	}
	defer outfile.Close()
	d.cmd.Stdout = outfile
	d.cmd.Stderr = outfile
	// d.cmd.SysProcAttr = &syscall.SysProcAttr{
	// 	Pdeathsig: syscall.SIGKILL, // kill the debezium process if the parent process dies
	// }
	d.registerExitHandlers()
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

func (d *Debezium) registerExitHandlers() {
	atexit.Register(func() {
		d.Stop()
	})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		utils.PrintAndLog("Received signal %s. Exiting...", sig)
		atexit.Exit(0)
	}()
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
		err := d.cmd.Process.Signal(syscall.SIGTERM)
		if err != nil {
			return fmt.Errorf("Error stopping debezium: %v", err)
		}
		d.cmd.Wait()
		log.Info("Stopped debezium.")
	}
	return nil
}
