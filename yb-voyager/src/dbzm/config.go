package dbzm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ExportDir string

	Host     string
	Port     int
	Username string
	Password string

	DatabaseName string
	SchemaNames  string
	TableList    []string
}

var configTemplate = `
debezium.format.value=json
debezium.sink.type=file
debezium.sink.filesink.dataDir=%s
debezium.source.max.batch.size=100
debezium.source.connector.class=io.debezium.connector.postgresql.PostgresConnector
debezium.source.offset.storage.file.filename=%s
debezium.source.offset.flush.interval.ms=0
debezium.source.database.hostname=%s
debezium.source.database.port=%d
debezium.source.database.user=%s
debezium.source.database.password=%s
debezium.source.database.dbname=%s
ddebezium.source.database.server.name=voyager
debezium.source.schema.include.list=%s
debezium.source.table.include.list=%s
debezium.source.plugin.name=pgoutput
debezium.source.snapshot.mode=initial_only

debezium.source.topic.prefix=voyager
quarkus.log.console.json=false
`

func (c *Config) WriteToFile(filePath string) error {
	dataDir := filepath.Join(c.ExportDir, "data")
	schemaNames := strings.Join(strings.Split(c.SchemaNames, "|"), ",")
	config := fmt.Sprintf(configTemplate,
		dataDir,
		filepath.Join(dataDir, "offsets.dat"),
		c.Host, c.Port, c.Username, c.Password,
		c.DatabaseName,
		schemaNames,
		strings.Join(c.TableList, ","))
	err := os.WriteFile(filePath, []byte(config), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file %s: %v", filePath, err)
	}
	return nil
}
