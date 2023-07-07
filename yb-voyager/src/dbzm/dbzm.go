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
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

func findDebeziumDistribution() error {
	if distDir := os.Getenv("DEBEZIUM_DIST_DIR"); distDir != "" {
		DEBEZIUM_DIST_DIR = distDir
	} else {
		possiblePaths := []string{
			"/opt/homebrew/Cellar/debezium@" + DEBEZIUM_VERSION + "/" + DEBEZIUM_VERSION + "/debezium-server",
			"/usr/local/Cellar/debezium@" + DEBEZIUM_VERSION + "/" + DEBEZIUM_VERSION + "/debezium-server",
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

// TODO : refactor this to an interface for different targetDBs
type VariableScaleDecimal struct {
	Scale int64
	Value string
}

func TransformValue(columnSchema Schema, columnValue string) (string, error) {
	logicalType := columnSchema.ColDbzSchema.Name
	if logicalType != "" {
		switch logicalType {
		case "io.debezium.time.Date":
			epochDays, err := strconv.ParseUint(columnValue, 10, 64)
			fmt.Printf("epochDays: %v", epochDays)
			if err != nil {
				return columnValue, fmt.Errorf("Error parsing epoch seconds: %v", err)
			}
			epochSecs := epochDays * 24 * 60 * 60
			return time.Unix(int64(epochSecs), 0).Local().Format(time.DateOnly), nil
		case "io.debezium.time.Timestamp":
			epochMilliSecs, err := strconv.ParseInt(columnValue, 10, 64)
			if err != nil {
				return columnValue, fmt.Errorf("Error parsing epoch milliseconds: %v", err)
			}
			epochSecs := epochMilliSecs / 1000
			return time.Unix(epochSecs, 0).Local().Format(time.DateTime), nil
		case "io.debezium.time.MicroTimestamp":
			epochMicroSecs, err := strconv.ParseInt(columnValue, 10, 64)
			if err != nil {
				return columnValue, fmt.Errorf("Error parsing epoch microseconds: %v", err)
			}
			epochSeconds := epochMicroSecs / 1000000
			epochNanos := (epochMicroSecs % 1000000) * 1000
			microTimeStamp, err := time.Parse(time.RFC3339Nano, time.Unix(epochSeconds, epochNanos).Local().Format(time.RFC3339Nano))
			if err != nil {
				return columnValue, err
			}
			return strings.TrimSuffix(microTimeStamp.String(), " +0000 UTC"), nil
		case "io.debezium.time.NanoTimestamp":
			epochNanoSecs, err := strconv.ParseInt(columnValue, 10, 64)
			if err != nil {
				return columnValue, fmt.Errorf("Error parsing epoch nanoseconds: %v", err)
			}
			epochSeconds := epochNanoSecs / 1000000000
			epochNanos := epochNanoSecs % 1000000000
			nanoTimeStamp, err := time.Parse(time.RFC3339Nano, time.Unix(epochSeconds, epochNanos).Local().Format(time.RFC3339Nano))
			if err != nil {
				return columnValue, err
			}
			return strings.TrimSuffix(nanoTimeStamp.String(), " +0000 UTC"), nil
		case "io.debezium.time.Time":
			epochMilliSecs, err := strconv.ParseInt(columnValue, 10, 64)
			if err != nil {
				return columnValue, fmt.Errorf("Error parsing epoch milliseconds: %v", err)
			}
			epochSecs := epochMilliSecs / 1000
			return time.Unix(epochSecs, 0).Local().Format(time.TimeOnly), nil
		case "io.debezium.time.MicroTime":
			epochMicroSecs, err := strconv.ParseInt(columnValue, 10, 64)
			if err != nil {
				return columnValue, fmt.Errorf("Error parsing epoch microseconds: %v", err)
			}
			epochSeconds := epochMicroSecs / 1000000
			epochNanos := (epochMicroSecs % 1000000) * 1000
			MICRO_TIME_FORMAT := "15:04:05.000000"
			return time.Unix(epochSeconds, epochNanos).Local().Format(MICRO_TIME_FORMAT), nil
		case "io.debezium.data.Bits":
			bytes, err := base64.StdEncoding.DecodeString(columnValue)
			fmt.Printf("bytes: %v", bytes)
			if err != nil {
				return columnValue, fmt.Errorf("Error decoding variable scale decimal in base64: %v", err)
			}
			var data uint64
			if len(bytes) >= 8 {
				data = binary.LittleEndian.Uint64(bytes[:8])
			} else {
				for i, b := range bytes {
					data |= uint64(b) << (8 * i)
				}
			}
			return fmt.Sprintf("%b", data), nil
		case "io.debezium.data.geometry.Point":
			// TODO: figure out if we want to represent it as a postgres native point or postgis geometry point.
		case "io.debezium.data.geometry.Geometry":
		case "io.debezium.data.geometry.Geography":
			//TODO: figure out if we want to represent it as a postgres native geography or postgis geometry geography.
		case "org.apache.kafka.connect.data.Decimal":
			return columnValue, nil //to skip it to be transformed by BYTES case
		case "io.debezium.data.VariableScaleDecimal":
			variableScaleDecimal := VariableScaleDecimal{}
			err := json.Unmarshal([]byte(columnValue), &variableScaleDecimal)
			if err != nil {
				return columnValue, fmt.Errorf("Error parsing variable scale decimal: %v", err)
			}
			data, err := base64.StdEncoding.DecodeString(variableScaleDecimal.Value)
			if err != nil {
				return columnValue, fmt.Errorf("Error decoding variable scale decimal in base64: %v", err)
			}
			var value int32
			for idx, b := range data {
				i := len(data) - idx - 1
				value |= int32(b) << (8 * i)
			}
			scale := math.Pow(10, float64(variableScaleDecimal.Scale))
			// Convert the int32 value to the decimal value
			var decimal float64
			if value < 0 { //neg decimal case
				value = -value
				decimal = float64(value) / scale
				decimal = -decimal
			} else {
				decimal = float64(value) / scale
			}
			decimalData := new(big.Float).SetFloat64(decimal)
			return decimalData.String(), nil
		}
	}

	dbzSchemaType := columnSchema.ColDbzSchema.Type
	switch dbzSchemaType {
	case "BYTES":
		//decode base64 string to bytes
		decodedBytes, err := base64.StdEncoding.DecodeString(columnValue) //e.g.`////wv==` -> `[]byte{0x00, 0x00, 0x00, 0x00}`
		if err != nil {
			return columnValue, fmt.Errorf("Error decoding base64 string: %v", err)
		}
		//convert bytes to hex string e.g. `[]byte{0x00, 0x00, 0x00, 0x00}` -> `\\x00000000`
		hexString := `\\x` //extra backslash is needed to escape the backslash in the hex string
		for _, b := range decodedBytes {
			hexString += fmt.Sprintf("%02x", b)
		}
		return string(hexString), nil
	case "MAP":
		mapValue := make(map[string]interface{})
		err := json.Unmarshal([]byte(columnValue), &mapValue)
		if err != nil {
			return columnValue, fmt.Errorf("Error parsing map: %v", err)
		}
		var transformedMapValue string
		for key, value := range mapValue {
			transformedMapValue = transformedMapValue + fmt.Sprintf("\"%s\"=>\"%s\",", key, value)
		}
		return strings.TrimSuffix(transformedMapValue, ","), nil //remove last comma
	}
	return columnValue, nil
}

func TransformDataRow(dataRow string, tableSchema TableSchema) (string, error) {
	columnValues := strings.Split(dataRow, "\t")
	var transformedValues []string
	for i, columnValue := range columnValues {
		columnSchema := tableSchema.Columns[i]
		if columnValue != "\\N" {
			fmt.Printf("columnValue: %v\n", columnValue)
			transformedValue, err := TransformValue(columnSchema, columnValue)
			if err != nil {
				return dataRow, err
			}
			fmt.Printf("transformedValue: %v\n", transformedValue)
			transformedValues = append(transformedValues, transformedValue)
		} else {
			transformedValues = append(transformedValues, columnValue)
		}

	}
	fmt.Printf("transformedValues: %v\n", transformedValues)
	transformedRow := strings.Join(transformedValues, "\t")
	return transformedRow, nil
}
