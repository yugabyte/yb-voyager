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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	goerrors "github.com/go-errors/errors"

	log "github.com/sirupsen/logrus"
	"github.com/tebeka/atexit"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var DEBEZIUM_DIST_DIR, DEBEZIUM_CONF_FILEPATH string

// These versions need to be changed at the time of a release
const DEBEZIUM_VERSION = "2.5.2-2026.5.1"

type Debezium struct {
	*Config
	cmd  *exec.Cmd
	err  error
	done bool
}

func FindDebeziumDistribution(sourceDBType string, useYBgRPCConnector bool) error {
	if distDir := os.Getenv("DEBEZIUM_DIST_DIR"); distDir != "" {
		DEBEZIUM_DIST_DIR = distDir
	} else {
		// depending on OS add the paths to check
		currentOS := runtime.GOOS
		possiblePaths := []string{"/opt/yb-voyager/debezium-server"}
		if currentOS == "darwin" {
			possiblePaths = append(possiblePaths, fmt.Sprintf("/opt/homebrew/Cellar/debezium@%s/%s/debezium-server", DEBEZIUM_VERSION, DEBEZIUM_VERSION),
				fmt.Sprintf("/usr/local/Cellar/debezium@%s/%s/debezium-server", DEBEZIUM_VERSION, DEBEZIUM_VERSION))
		}

		for _, path := range possiblePaths {
			if utils.FileOrFolderExists(path) {
				DEBEZIUM_DIST_DIR = path
				break
			}
		}
		if DEBEZIUM_DIST_DIR == "" {
			err := goerrors.Errorf("Debezium: not found in path(s) %v", possiblePaths)
			return err
		}
	}

	if sourceDBType == "yugabytedb" && useYBgRPCConnector {
		pathSuffix := "debezium-server-1.9.5"
		DEBEZIUM_DIST_DIR = filepath.Join(DEBEZIUM_DIST_DIR, pathSuffix)
	}
	return nil
}

// logicalConnectorYBVersionRegex captures the connector version token after "yb." up to
// the next "-" (e.g. "2025.2.3"):
//   - only the first two segments matter (the YB series); the rest is a connector-internal
//     counter, reduced later via ybversion.SeriesVersion.
//   - the full token is kept so distinct jars can be told apart.
//   - the required leading digit rejects the gRPC tag ("yb.grpc.<ver>").
var logicalConnectorYBVersionRegex = regexp.MustCompile(`yb\.([0-9]+\.[0-9]+[^-]*)`)

// ErrMultipleLogicalConnectorVersions means the yb-connector directory holds jars for
// more than one connector version. run.sh puts every jar on the classpath, so which one
// runs is undefined — callers treat this as fatal rather than guessing.
var ErrMultipleLogicalConnectorVersions = errors.New("multiple logical connector versions found in distribution")

// ParseLogicalConnectorYBVersion returns the connector version token from a jar name
// (e.g. ".yb.2025.2.3-..." → "2025.2.3"):
//   - only the first two segments (the YB series) matter for compatibility; the rest is a
//     connector-internal counter.
//   - the full token is returned so callers can distinguish distinct jars.
//   - errors if the name has no "yb.<series>..." token (dependency jars, gRPC connector).
func ParseLogicalConnectorYBVersion(jarName string) (string, error) {
	match := logicalConnectorYBVersionRegex.FindStringSubmatch(jarName)
	if len(match) < 2 {
		return "", goerrors.Errorf("unable to extract a YugabyteDB connector version from jar name %q; expected a 'yb.<series>...' token", jarName)
	}
	return match[1], nil
}

// GetLogicalConnectorYBVersion returns the connector version token from the resolved
// Debezium distribution (FindDebeziumDistribution must have run). run.sh puts every jar in
// yb-connector on the classpath, so jars for multiple distinct tokens are a fatal
// ambiguity → ErrMultipleLogicalConnectorVersions.
func GetLogicalConnectorYBVersion() (string, error) {
	if DEBEZIUM_DIST_DIR == "" {
		return "", goerrors.Errorf("debezium distribution directory is not resolved")
	}
	connectorDir := filepath.Join(DEBEZIUM_DIST_DIR, "yb-connector")
	jars, err := filepath.Glob(filepath.Join(connectorDir, "*.jar"))
	if err != nil {
		return "", goerrors.Errorf("listing logical connector jars in %s: %w", connectorDir, err)
	}

	// Collect distinct connector tokens (ignoring non-connector jars).
	seen := make(map[string]bool)
	var versions []string
	for _, jar := range jars {
		token, err := ParseLogicalConnectorYBVersion(filepath.Base(jar))
		if err != nil {
			// Not a connector jar (e.g. a dependency jar); ignore.
			continue
		}
		if !seen[token] {
			seen[token] = true
			versions = append(versions, token)
		}
	}

	switch len(versions) {
	case 0:
		return "", goerrors.Errorf("no logical connector jar found in %s", connectorDir)
	case 1:
		return versions[0], nil
	default:
		sort.Strings(versions)
		return "", fmt.Errorf("found multiple logical connector jars with different versions %v in %s: %w", versions, connectorDir, ErrMultipleLogicalConnectorVersions)
	}
}

func NewDebezium(config *Config) *Debezium {
	return &Debezium{Config: config}
}

func (d *Debezium) Start() error {
	err := FindDebeziumDistribution(d.Config.SourceDBType, d.Config.UseYBgRPCConnector)
	if err != nil {
		// Addding suggestion to install debezium-server if it is not found
		return goerrors.Errorf("%w. Either install debezium-server or provide its path in the DEBEZIUM_DIST_DIR env variable", err)
	}
	DEBEZIUM_CONF_FILEPATH = filepath.Join(d.ExportDir, "metainfo", "conf", "application.properties")
	err = d.Config.WriteToFile(DEBEZIUM_CONF_FILEPATH)
	if err != nil {
		return err
	}

	schemasPath := filepath.Join(d.ExportDir, "data", "schemas", d.ExporterRole)
	err = os.MkdirAll(schemasPath, 0755)
	if err != nil {
		return goerrors.Errorf("Error creating schemas directory: %w", err)
	}

	var YB_OR_PG_CONNECTOR_PATH string
	if isTargetDBExporter(d.ExporterRole) {
		if !d.Config.UseYBgRPCConnector {
			// In case of logical replication connector we need the path /opt/yb-voyager/debezium-server/yb-connector
			YB_OR_PG_CONNECTOR_PATH = filepath.Join(DEBEZIUM_DIST_DIR, "yb-connector")
		} else {
			// In case of gRPC connector the DEBEZIUM_DIST_DIR is set to debezium-server-1.9.5 and the connector is in debezium-server-1.9.5/yb-grpc-connector
			//This is done to load this jar at the end in the classpath to avoid classpath issues with the jar
			// Faced an issue with error `java.sql.SQLException: No suitable driver found for jdbc:sqlite`
			// the grpc connector has a service java.sql.Driver which has com.yugabyte.Driver implementation but the class wasn't found in the built jar
			// because of which it errors out and doesn't load rest of the dependencies and sqlite driver is not loaded and hence it errored out
			YB_OR_PG_CONNECTOR_PATH = filepath.Join(DEBEZIUM_DIST_DIR, "yb-grpc-connector")
		}
	} else {
		// In case of source db exporter we need the path /opt/yb-voyager/debezium-server/pg-connector
		YB_OR_PG_CONNECTOR_PATH = filepath.Join(DEBEZIUM_DIST_DIR, "pg-connector")
	}

	log.Infof("starting debezium...")
	d.cmd = exec.Command(filepath.Join(DEBEZIUM_DIST_DIR, "run.sh"), DEBEZIUM_CONF_FILEPATH, YB_OR_PG_CONNECTOR_PATH)
	d.cmd.Env = os.Environ()
	// $TNS_ADMIN is used to set jdbc property oracle.net.tns_admin which will enable using TNS alias
	d.cmd.Env = append(d.cmd.Env, fmt.Sprintf("TNS_ADMIN=%s", d.Config.TNSAdmin))
	if d.Config.Password != "" {
		d.cmd.Env = append(d.cmd.Env, "DEBEZIUM_SOURCE_DATABASE_PASSWORD="+d.Config.Password)
	}
	log.Infof("Setting TNS_ADMIN=%s", d.Config.TNSAdmin)
	if !d.Config.OracleJDBCWalletLocationSet {
		// only specify the default value of this property if it is not already set in $TNS_ADMIN/ojdbc.properties.
		// This is because the property set in the command seems to take precedence.
		d.cmd.Env = append(d.cmd.Env, fmt.Sprintf("JAVA_OPTS=-Doracle.net.wallet_location=file:%s", d.Config.TNSAdmin))
		log.Infof("Setting oracle wallet location=%s", d.Config.TNSAdmin)
	}
	err = d.setupLogFile()
	if err != nil {
		return goerrors.Errorf("Error setting up logging for debezium: %w", err)
	}
	d.registerExitHandlers()
	log.Debugf("debezium command: %v", d.cmd)

	err = d.cmd.Start()
	if err != nil {
		return goerrors.Errorf("Error starting debezium: %w", err)
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
	logFilePath, err := filepath.Abs(filepath.Join(d.ExportDir, "logs", fmt.Sprintf("debezium-%s.log", d.ExporterRole)))
	if err != nil {
		return goerrors.Errorf("failed to create absolute path:%w", err)
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
			return goerrors.Errorf("Error sending signal to SIGTERM: %w", err)
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
		_ = d.cmd.Wait() // reaping after a deliberate stop; a non-zero exit is expected here
		d.done = true
		log.Info("Stopped debezium.")
	}
	return nil
}

func GetPIDOfDebeziumOnExportDir(exportDir string, exporterRole string) (string, error) {
	dbzmLockFile := filepath.Join(exportDir, fmt.Sprintf(".debezium_%s.lck", exporterRole))
	_, err := os.Stat(dbzmLockFile)
	if err != nil {
		return "", err
	}
	//read the lock file to get the pid of the process
	pid, err := os.ReadFile(dbzmLockFile)
	if err != nil {
		return "", goerrors.Errorf("read debezium lock file: %w", err)
	}
	pidStr := strings.TrimSuffix(string(pid), "\n")
	return pidStr, nil
}
