package dbzm

import (
	"path/filepath"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const DEBEZIUM_DIST_DIR = "/home/amit.jambure/debezium-server/"
const DEBEZIUM_CONF_DIR = DEBEZIUM_DIST_DIR + "conf/"
const DEBEZIUM_CONF_FILEPATH = DEBEZIUM_CONF_DIR + "application.properties"

type Debezium struct {
	*Config
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
	return nil
}

func (d *Debezium) Stop() error {
	return nil
}

var count = 0

func (d *Debezium) IsRunning() bool {
	count++
	return count < 5
}

func (d *Debezium) Error() error {
	return nil
}

func (d *Debezium) GetExportStatus() (*ExportStatus, error) {
	statusFilePath := filepath.Join(d.ExportDir, "data", "export_status.json")
	return ReadExportStatus(statusFilePath)
}
