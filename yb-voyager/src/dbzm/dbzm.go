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
package dbzm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tebeka/atexit"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var DEBEZIUM_DIST_DIR, DEBEZIUM_CONF_FILEPATH string

// These versions need to be changed at the time of a release
const DEBEZIUM_VERSION = "2.2.0-1.3.0"

type Debezium struct {
	*Config
	cmd  *exec.Cmd
	err  error
	done bool
}

func findDebeziumDistribution(sourceDBType string) error {
	if distDir := os.Getenv("DEBEZIUM_DIST_DIR"); distDir != "" {
		DEBEZIUM_DIST_DIR = distDir
	} else {
		var possiblePaths []string
		if sourceDBType == "yugabytedb" {
			possiblePaths = []string{
				"/opt/homebrew/Cellar/debezium@" + DEBEZIUM_VERSION + "/" + DEBEZIUM_VERSION + "/debezium-server/debezium-server-fall-forward",
				"/usr/local/Cellar/debezium@" + DEBEZIUM_VERSION + "/" + DEBEZIUM_VERSION + "/debezium-server/debezium-server-fall-forward",
				"/opt/yb-voyager/debezium-server/debezium-server-fall-forward"}
		} else {
			possiblePaths = []string{
				"/opt/homebrew/Cellar/debezium@" + DEBEZIUM_VERSION + "/" + DEBEZIUM_VERSION + "/debezium-server",
				"/usr/local/Cellar/debezium@" + DEBEZIUM_VERSION + "/" + DEBEZIUM_VERSION + "/debezium-server",
				"/opt/yb-voyager/debezium-server"}
		}
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

func GenerateAndStoreYBStreamID(config Config) (string, error) {
	err := findDebeziumDistribution(config.SourceDBType)
	if err != nil {
		return "", fmt.Errorf("error in finding debezium distribution: %s", err)
	}
	YB_CLIENT_WRAPPER_JAR := fmt.Sprintf("%s/yb-client-cdc-stream-wrapper.jar", DEBEZIUM_DIST_DIR) //$DEBEZIUM_DIST/yb-client-cdc-stream-wrapper.jar
	tableName := strings.Split(config.TableList[0], ".")[1]                                        //any table name in the database is required by yb-client createCDCStream(...) API
	command := fmt.Sprintf("java -jar %s -master_addresses %s -table_name %s -db_name %s ", YB_CLIENT_WRAPPER_JAR, config.YBServers, tableName, config.DatabaseName)

	if config.SSLRootCert != "" {
		command += fmt.Sprintf(" -ssl_cert_file %s", config.SSLRootCert)
	}

	cmd := exec.CommandContext(context.Background(), "/bin/bash", "-c", command)
	var outbuf bytes.Buffer
	var errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf
	err = cmd.Start()
	if err != nil {
		if outbuf.String() != "" {
			log.Infof("Output of the command %s: %s", command, outbuf.String())
		}
		log.Infof("Failed to start command: %s, error: %s", command, err)
		return "", err
	}
	err = cmd.Wait()
	if err != nil {
		if outbuf.String() != "" {
			log.Infof("Output of the command %s: %s", command, outbuf.String())
		}
		log.Infof("Failed to wait for command: %s , error: %s", command, err)
		return "", err
	}
	//output of yb-admin command - CDC Stream ID: <stream_id>
	rgx := regexp.MustCompile(`CDC Stream ID: ([a-zA-Z0-9]+)`)
	matches := rgx.FindStringSubmatch(outbuf.String())
	if len(matches) != 2 {
		return "", fmt.Errorf("error in parsing output of command: %s, output: %s", command, outbuf.String())
	}
	streamID := matches[1]
	//save streamID in a file
	streamIDFile := filepath.Join(config.ExportDir, "metainfo", "yb_cdc_stream_id.txt")
	file, err := os.Create(streamIDFile)
	if err != nil {
		return "", fmt.Errorf(" creating file: %s, error: %s", streamIDFile, err)
	}
	defer file.Close()
	_, err = file.WriteString(streamID)
	if err != nil {
		return "", fmt.Errorf(" writing to file: %s, error: %s", streamIDFile, err)
	}
	return streamID, nil
}

func ReadYBStreamID(exportDir string) (string, error) {
	streamIDFilePath := filepath.Join(exportDir, "metainfo", "yb_cdc_stream_id.txt")
	if utils.FileOrFolderExists(streamIDFilePath) {
		file, err := os.ReadFile(streamIDFilePath)
		if err != nil {
			return "", fmt.Errorf("failed to read stream id file: %w", err)
		}
		return string(file), nil
	}
	return "", fmt.Errorf("yugabytedb cdc stream id not found at %s. Please use --start-clean to start the streaming", streamIDFilePath)

}

func NewDebezium(config *Config) *Debezium {
	return &Debezium{Config: config}
}

func (d *Debezium) Start() error {
	err := findDebeziumDistribution(d.Config.SourceDBType)
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
	d.cmd.Env = os.Environ()
	// $TNS_ADMIN is used to set jdbc property oracle.net.tns_admin which will enable using TNS alias
	d.cmd.Env = append(d.cmd.Env, fmt.Sprintf("TNS_ADMIN=%s", d.Config.TNSAdmin))
	log.Infof("Setting TNS_ADMIN=%s", d.Config.TNSAdmin)
	if !d.Config.OracleJDBCWalletLocationSet {
		// only specify the default value of this property if it is not already set in $TNS_ADMIN/ojdbc.properties.
		// This is because the property set in the command seems to take precedence.
		d.cmd.Env = append(d.cmd.Env, fmt.Sprintf("JAVA_OPTS=-Doracle.net.wallet_location=file:%s", d.Config.TNSAdmin))
		log.Infof("Setting oracle wallet location=%s", d.Config.TNSAdmin)
	}
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
