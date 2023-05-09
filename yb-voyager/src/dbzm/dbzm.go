package dbzm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tebeka/atexit"

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
	d.cmd = exec.Command(filepath.Join(DEBEZIUM_DIST_DIR, "run.sh"), DEBEZIUM_CONF_FILEPATH)
	// capture stdout, stderr
	var outbuf bytes.Buffer
	var errbuf bytes.Buffer
	d.cmd.Stdout = &outbuf
	d.cmd.Stderr = &errbuf

	d.registerExitHandlers()
	err = d.cmd.Start()
	if err != nil {
		return fmt.Errorf("Error starting debezium: %v", err)
	}
	log.Infof("Debezium started successfully with pid = %d", d.cmd.Process.Pid)
	go func() {
		d.err = d.cmd.Wait()
		d.done = true
		if d.err != nil {
			log.Errorf("Debezium export failed:%s\nSTDOUT:%s\nSTDERR:%s", d.err, outbuf.String(), errbuf.String())
		}
	}()
	return nil
}

// Registers an atexit handlers to ensure that debezium is shut down gracefully in the
// event that voyager exits either due to some error.
func (d *Debezium) registerExitHandlers() {
	atexit.Register(func() {
		err := d.Stop()
		if err != nil {
			log.Errorf("Error stopping debezium: %v", err)
			os.Exit(1)
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
