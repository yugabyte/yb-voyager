package dbzm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tebeka/atexit"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"gopkg.in/natefinch/lumberjack.v2"
)

var DEBEZIUM_DIST_DIR, DEBEZIUM_CONF_FILEPATH string

// These versions need to be changed at the time of a release
const DEBEZIUM_VERSION = "2.2.0-1.3.0"
const BASE_DEBEZIUM_VERSION = "2.2.0"

type Debezium struct {
	*Config
	cmd  *exec.Cmd
	err  error
	done bool
}

func findDebeziumDistribution() error {
	if distDir := os.Getenv("DEBEZIUM_DIST_DIR"); distDir != "" {
		DEBEZIUM_DIST_DIR = distDir
	} else {
		possiblePaths := []string{
			"/opt/homebrew/Cellar/debezium@" + BASE_DEBEZIUM_VERSION + "/" + DEBEZIUM_VERSION + "/debezium-server",
			"/usr/localCellar/debezium@" + BASE_DEBEZIUM_VERSION + "/" + DEBEZIUM_VERSION + "/debezium-server",
			"/opt/yb-voyager/debezium-server"}
		for _, path := range possiblePaths {
			if utils.FileOrFolderExists(path) {
				DEBEZIUM_DIST_DIR = path
				break
			}
		}
		if DEBEZIUM_DIST_DIR == "" {
			err := fmt.Errorf("could not find debezium-server directory in any of %v. Either install debezium-server or provide its path in the DEBEZIUM_DIST_DIR env variable", possiblePaths)
			return err
		}
	}
	return nil
}

func NewDebezium(config *Config) *Debezium {
	return &Debezium{Config: config}
}

func (d *Debezium) Start() error {
	err := findDebeziumDistribution()
	if err != nil {
		return err
	}
	DEBEZIUM_CONF_FILEPATH = filepath.Join(d.ExportDir, "metainfo", "conf", "application.properties")
	err = d.Config.WriteToFile(DEBEZIUM_CONF_FILEPATH)
	if err != nil {
		return err
	}

	log.Infof("starting debezium...")
	d.cmd = exec.Command(filepath.Join(DEBEZIUM_DIST_DIR, "run.sh"), DEBEZIUM_CONF_FILEPATH)
	err = d.setupLogFile()
	if err != nil {
		return fmt.Errorf("Error setting up logging for debezium: %v", err)
	}
	d.registerExitHandlers()
	err = d.cmd.Start()
	if err != nil {
		return fmt.Errorf("Error starting debezium: %v", err)
	}
	log.Infof("Debezium started successfully with pid = %d", d.cmd.Process.Pid)

	// wait for process to end.
	go func() {
		d.err = d.cmd.Wait()
		d.done = true
		if d.err != nil {
			log.Errorf("Debezium exited with: %v", d.err)
		}
	}()
	return nil
}

func (d *Debezium) setupLogFile() error {
	logFilePath, err := filepath.Abs(filepath.Join(d.ExportDir, "logs", "debezium.log"))
	if err != nil {
		return fmt.Errorf("failed to create absolute path:%v", err)
	}

	logRotator := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    200, // 200 MB log size before rotation
		MaxBackups: 10,  // Allow upto 10 logs at once before deleting oldest logs.
	}
	d.cmd.Stdout = logRotator
	d.cmd.Stderr = logRotator
	return nil
}

// Registers an atexit handlers to ensure that debezium is shut down gracefully in the
// event that voyager exits either due to some error.
func (d *Debezium) registerExitHandlers() {
	atexit.Register(func() {
		err := d.Stop()
		if err != nil {
			log.Errorf("Error stopping debezium: %v", err)
		}
	})
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

// stops debezium process gracefully if it is running
func (d *Debezium) Stop() error {
	if d.IsRunning() {
		log.Infof("Stopping debezium...")
		err := d.cmd.Process.Signal(syscall.SIGTERM)
		if err != nil {
			return fmt.Errorf("Error sending signal to SIGTERM: %v", err)
		}
		go func() {
			// wait for a certain time for debezium to shut down before force killing the process.
			sigtermTimeout := 100
			time.Sleep(time.Duration(sigtermTimeout) * time.Second)
			if d.IsRunning() {
				log.Warnf("Waited %d seconds for debezium process to stop. Force killing it now.", sigtermTimeout)
				err = d.cmd.Process.Kill()
				if err != nil {
					log.Errorf("Error force-stopping debezium: %v", err)
					os.Exit(1) // not calling atexit.Exit here because this func is called from within an atexit handler
				}
			}
		}()
		d.cmd.Wait()
		d.done = true
		log.Info("Stopped debezium.")
	}
	return nil
}
