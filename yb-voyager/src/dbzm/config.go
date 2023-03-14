package dbzm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

type Config struct {
	SourceDBType string
	ExportDir    string

	Host     string
	Port     int
	Username string
	Password string

	DatabaseName string
	SchemaNames  string
	TableList    []string
	SnapshotMode string
}

var postgresSrcConfigTemplate = `
debezium.format.value=connect
debezium.format.key=connect

debezium.sink.type=ybexporter
debezium.sink.ybexporter.dataDir=%s

debezium.source.connector.class=io.debezium.connector.postgresql.PostgresConnector
debezium.source.topic.naming.strategy=io.debezium.server.ybexporter.DummyTopicNamingStrategy
debezium.source.snapshot.mode=%s

debezium.source.offset.storage.file.filename=%s
debezium.source.offset.flush.interval.ms=0

debezium.source.database.hostname=%s
debezium.source.database.port=%d
debezium.source.database.user=%s
debezium.source.database.password=%s
debezium.source.database.dbname=%s
debezium.source.database.server.name=tutorial

debezium.source.schema.include.list=%s
debezium.source.table.include.list=%s
debezium.source.plugin.name=pgoutput
debezium.source.topic.prefix=tutorial

debezium.source.decimal.handling.mode=string
debezium.source.interval.handling.mode=string
debezium.source.hstore.handling.mode=map
debezium.source.include.unknown.datatypes=true
debezium.source.datatype.propagate.source.type=.*BOX.*,.*LINE.*,.*LSEG.*,.*PATH.*,.*POLYGON.*,.*CIRCLE.*

quarkus.log.console.json=false
quarkus.log.level=info`

var oracleSrcConfigTemplate = `
debezium.format.value=connect
debezium.format.key=connect

debezium.sink.type=ybexporter
debezium.sink.ybexporter.dataDir=%s

debezium.source.connector.class=io.debezium.connector.oracle.OracleConnector
debezium.source.topic.naming.strategy=io.debezium.server.ybexporter.DummyTopicNamingStrategy
debezium.source.snapshot.mode=%s

debezium.source.offset.storage.file.filename=%s
debezium.source.offset.flush.interval.ms=0

debezium.source.database.hostname=%s
debezium.source.database.port=%d
debezium.source.database.user=%s
debezium.source.database.password=%s
debezium.source.database.dbname=%s
#debezium.source.database.pdb.name=ORCLPDB1

debezium.source.schema.include.list=%s
debezium.source.table.include.list=%s
debezium.source.topic.prefix=tutorial

debezium.source.decimal.handling.mode=string
debezium.source.interval.handling.mode=string
debezium.source.hstore.handling.mode=map
debezium.source.include.unknown.datatypes=true
debezium.source.datatype.propagate.source.type=.*BOX.*,.*LINE.*,.*LSEG.*,.*PATH.*,.*POLYGON.*,.*CIRCLE.*

debezium.source.database.history=io.debezium.relational.history.FileDatabaseHistory
debezium.source.database.history.file.filename=%s
debezium.source.schema.history.internal=io.debezium.storage.file.history.FileSchemaHistory
debezium.source.schema.history.internal.file.filename=%s
debezium.source.include.schema.changes=false

quarkus.log.console.json=false
quarkus.log.level=info
debezium.source.tombstones.on.delete=false`

var mysqlSrcConfigTemplate = `
debezium.format.value=connect
debezium.format.key=connect

debezium.sink.type=ybexporter
debezium.sink.ybexporter.dataDir=%s

debezium.source.connector.class=io.debezium.connector.mysql.MySqlConnector
debezium.source.topic.naming.strategy=io.debezium.server.ybexporter.DummyTopicNamingStrategy
debezium.source.snapshot.mode=%s

debezium.source.offset.storage.file.filename=%s
debezium.source.offset.flush.interval.ms=0

debezium.source.database.hostname=%s
debezium.source.database.port=%d
debezium.source.database.user=%s
debezium.source.database.password=%s
debezium.source.database.include.list=%s
debezium.source.database.server.id=101

debezium.source.table.include.list=%s
debezium.source.topic.prefix=tutorial

debezium.source.decimal.handling.mode=string
debezium.source.interval.handling.mode=string
#debezium.source.include.unknown.datatypes=true
#debezium.source.datatype.propagate.source.type=.*BOX.*,.*LINE.*,.*LSEG.*,.*PATH.*,.*POLYGON.*,.*CIRCLE.*

debezium.source.schema.history.internal=io.debezium.storage.file.history.FileSchemaHistory
debezium.source.schema.history.internal.file.filename=%s
debezium.source.include.schema.changes=false

quarkus.log.console.json=false
quarkus.log.level=info`

func (c *Config) getConfigTemplate() string {
	dataDir := filepath.Join(c.ExportDir, "data")
	log.Infof("setting dataDir in debezium-server conf file: %s\n", dataDir)
	offsetFile := filepath.Join(dataDir, "offsets.dat")
	schemaNames := strings.Join(strings.Split(c.SchemaNames, "|"), ",")
	switch c.SourceDBType {
	case "postgresql":
		for i, table := range c.TableList {
			if !strings.Contains(table, ".") {
				c.TableList[i] = fmt.Sprintf("public.%s", table)
			}
		}
		return fmt.Sprintf(postgresSrcConfigTemplate,
			dataDir,
			c.SnapshotMode,
			offsetFile,
			c.Host, c.Port, c.Username, c.Password,
			c.DatabaseName,
			schemaNames,
			strings.Join(c.TableList, ","))

	case "oracle":
		for i, table := range c.TableList {
			if !strings.Contains(table, ".") {
				c.TableList[i] = fmt.Sprintf("%s.%s", c.SchemaNames, table)
			}
		}
		return fmt.Sprintf(oracleSrcConfigTemplate,
			dataDir,
			c.SnapshotMode,
			offsetFile,
			c.Host, c.Port, c.Username, c.Password,
			c.DatabaseName,
			schemaNames,
			strings.Join(c.TableList, ","),
			filepath.Join(c.ExportDir, "temp", "history.dat"),
			filepath.Join(c.ExportDir, "temp", "schema_history"))

	case "mysql":
		for i, table := range c.TableList {
			if !strings.Contains(table, ".") {
				c.TableList[i] = fmt.Sprintf("%s.%s", c.DatabaseName, table)
			}
		}
		return fmt.Sprintf(mysqlSrcConfigTemplate,
			dataDir,
			c.SnapshotMode,
			offsetFile,
			c.Host, c.Port, c.Username, c.Password,
			c.DatabaseName,
			strings.Join(c.TableList, ","),
			filepath.Join(c.ExportDir, "temp", "schema_history"))
	}
	return ""
}

func (c *Config) WriteToFile(filePath string) error {
	config := c.getConfigTemplate()
	err := os.WriteFile(filePath, []byte(config), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file %s: %v", filePath, err)
	}
	return nil
}
