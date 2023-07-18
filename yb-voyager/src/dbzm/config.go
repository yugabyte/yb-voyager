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
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

	DatabaseName                string
	PDBName                     string
	SchemaNames                 string
	TableList                   []string
	ColumnSequenceMap           []string
	ColumnList                  []string
	Uri                         string
	TNSAdmin                    string
	OracleJDBCWalletLocationSet bool

	SSLMode               string
	SSLCertPath           string
	SSLKey                string
	SSLRootCert           string
	SSLKeyStore           string
	SSLKeyStorePassword   string
	SSLTrustStore         string
	SSLTrustStorePassword string

	SnapshotMode string
}

var baseConfigTemplate = `
debezium.format.value=connect
debezium.format.key=connect
quarkus.log.console.json=false
quarkus.log.level=info
`

var baseSrcConfigTemplate = `
debezium.source.database.user=%s
debezium.source.database.password=%s

debezium.source.snapshot.mode=%s
debezium.source.offset.storage.file.filename=%s
debezium.source.offset.flush.interval.ms=0

debezium.source.table.include.list=%s
debezium.source.interval.handling.mode=string
debezium.source.include.unknown.datatypes=true
debezium.source.datatype.propagate.source.type=.*BOX.*,.*LINE.*,.*LSEG.*,.*PATH.*,.*POLYGON.*,.*CIRCLE.*
debezium.source.tombstones.on.delete=false

debezium.source.topic.naming.strategy=io.debezium.server.ybexporter.DummyTopicNamingStrategy
debezium.source.topic.prefix=yb-voyager
debezium.source.database.server.name=yb-voyager
`
var baseSinkConfigTemplate = `
debezium.sink.type=ybexporter
debezium.sink.ybexporter.dataDir=%s
debezium.sink.ybexporter.column_sequence.map=%s
`

var postgresSrcConfigTemplate = `
debezium.source.database.hostname=%s
debezium.source.database.port=%d
debezium.source.connector.class=io.debezium.connector.postgresql.PostgresConnector
debezium.source.database.dbname=%s
debezium.source.schema.include.list=%s
debezium.source.plugin.name=pgoutput
debezium.source.hstore.handling.mode=map
debezium.source.converters=postgres_to_yb_converter
debezium.source.postgres_to_yb_converter.type=io.debezium.server.ybexporter.PostgresToYbValueConverter
`

var postgresSSLConfigTemplate = `
debezium.source.database.sslmode=%s
debezium.source.database.sslcert=%s
debezium.source.database.sslkey=%s
debezium.source.database.sslpassword=
debezium.source.database.sslrootcert=%s
`

var postgresConfigTemplate = baseConfigTemplate +
	baseSrcConfigTemplate +
	postgresSrcConfigTemplate +
	baseSinkConfigTemplate

var oracleSrcConfigTemplate = `
debezium.source.database.url=%s
debezium.source.connector.class=io.debezium.connector.oracle.OracleConnector
debezium.source.database.dbname=PLACEHOLDER
#debezium.source.database.pdb.name=ORCLPDB1
debezium.source.schema.include.list=%s
debezium.source.hstore.handling.mode=map
debezium.source.database.history=io.debezium.relational.history.FileDatabaseHistory
debezium.source.database.history.file.filename=%s
debezium.source.schema.history.internal=io.debezium.storage.file.history.FileSchemaHistory
debezium.source.schema.history.internal.file.filename=%s
debezium.source.schema.history.internal.skip.unparseable.ddl=true
debezium.source.schema.history.internal.store.only.captured.tables.ddl=true
debezium.source.schema.history.internal.store.only.captured.databases.ddl=true
debezium.source.include.schema.changes=false
#debezium.sink.ybexporter.queueSegmentMaxBytes=10485760 # 10MB
`

var oracleSrcPDBConfigTemplate = `
debezium.source.database.pdb.name=%s
`

var oracleConfigTemplate = baseConfigTemplate +
	baseSrcConfigTemplate +
	oracleSrcConfigTemplate +
	baseSinkConfigTemplate

var mysqlSrcConfigTemplate = `
debezium.source.database.hostname=%s
debezium.source.database.port=%d
debezium.source.database.include.list=%s
debezium.source.database.server.id=%d
debezium.source.connector.class=io.debezium.connector.mysql.MySqlConnector

debezium.source.schema.history.internal=io.debezium.storage.file.history.FileSchemaHistory
debezium.source.schema.history.internal.file.filename=%s
debezium.source.include.schema.changes=false
`

var mysqlConfigTemplate = baseConfigTemplate +
	baseSrcConfigTemplate +
	mysqlSrcConfigTemplate +
	baseSinkConfigTemplate

var mysqlSSLConfigTemplate = `
debezium.source.database.ssl.mode=%s
`

var mysqlSSLKeyStoreConfigTemplate = `
debezium.source.database.ssl.keystore=%s
debezium.source.database.ssl.keystore.password=%s
`

var mysqlSSLTrustStoreConfigTemplate = `
debezium.source.database.ssl.truststore=%s
debezium.source.database.ssl.truststore.password=%s
`

func (c *Config) String() string {
	dataDir := filepath.Join(c.ExportDir, "data")
	offsetFile := filepath.Join(dataDir, "offsets.dat")
	schemaNames := strings.Join(strings.Split(c.SchemaNames, "|"), ",")
	var conf string
	switch c.SourceDBType {
	case "postgresql":
		conf = fmt.Sprintf(postgresConfigTemplate,
			c.Username,
			c.Password,
			c.SnapshotMode,
			offsetFile,
			strings.Join(c.TableList, ","),

			c.Host, c.Port,
			c.DatabaseName,
			schemaNames,

			dataDir,
			strings.Join(c.ColumnSequenceMap, ","))
		sslConf := fmt.Sprintf(postgresSSLConfigTemplate,
			c.SSLMode,
			c.SSLCertPath,
			c.SSLKey,
			c.SSLRootCert)
		conf = conf + sslConf

	case "oracle":
		conf = fmt.Sprintf(oracleConfigTemplate,
			c.Username,
			c.Password,
			c.SnapshotMode,
			offsetFile,
			strings.Join(c.TableList, ","),

			c.Uri,
			schemaNames,
			filepath.Join(c.ExportDir, "data", "history.dat"),
			filepath.Join(c.ExportDir, "data", "schema_history.json"),

			dataDir,
			strings.Join(c.ColumnSequenceMap, ","))
		if c.PDBName != "" {
			// cdb setup.
			conf = conf + fmt.Sprintf(oracleSrcPDBConfigTemplate, c.PDBName)
		}

	case "mysql":
		conf = fmt.Sprintf(mysqlConfigTemplate,
			c.Username,
			c.Password,
			c.SnapshotMode,
			offsetFile,
			strings.Join(c.TableList, ","),

			c.Host, c.Port,
			c.DatabaseName,
			getDatabaseServerID(),
			filepath.Join(c.ExportDir, "data", "schema_history.json"),

			dataDir,
			strings.Join(c.ColumnSequenceMap, ","))
		sslConf := fmt.Sprintf(mysqlSSLConfigTemplate, c.SSLMode)
		if c.SSLKeyStore != "" {
			sslConf += fmt.Sprintf(mysqlSSLKeyStoreConfigTemplate,
				c.SSLKeyStore,
				c.SSLKeyStorePassword)
		}
		if c.SSLTrustStore != "" {
			sslConf += fmt.Sprintf(mysqlSSLTrustStoreConfigTemplate,
				c.SSLTrustStore,
				c.SSLTrustStorePassword)
		}
		conf = conf + sslConf
	default:
		panic(fmt.Sprintf("unknown source db type %s", c.SourceDBType))
	}

	if c.ColumnList != nil {
		conf += fmt.Sprintf("\ndebezium.source.column.include.list=%s", strings.Join(c.ColumnList, ","))
	}

	return conf
}

func (c *Config) WriteToFile(filePath string) error {
	config := c.String()
	err := os.WriteFile(filePath, []byte(config), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file %s: %v", filePath, err)
	}
	return nil
}

// read config file DEBEZIUM_CONF_FILEPATH into a string
func readConfigFile() (string, error) {
	config, err := os.ReadFile(DEBEZIUM_CONF_FILEPATH)
	if err != nil {
		return "", fmt.Errorf("failed to read config file %s: %w", DEBEZIUM_CONF_FILEPATH, err)
	}

	return string(config), nil
}

// generate/fetch the value for 'debezium.source.database.server.id' property for MySQL
func getDatabaseServerID() int {
	databaseServerId := rand.Intn(math.MaxInt-10000) + 10000
	log.Infof("randomly generated database server id: %d", databaseServerId)
	config, err := readConfigFile()
	if err != nil {
		log.Errorf("failed to read config file: %v", err)
		return databaseServerId
	}

	// if config file exists, read the value of 'debezium.source.database.server.id' property
	if strings.Contains(config, "debezium.source.database.server.id") {
		re := regexp.MustCompile(`(?m)^debezium.source.database.server.id=(\d+)$`)
		matches := re.FindStringSubmatch(config)
		if len(matches) == 2 {
			databaseServerId, err = strconv.Atoi(matches[1])
			if err != nil {
				log.Errorf("failed to convert database server id to int: %v", err)
				return databaseServerId
			}
		}
	}
	log.Infof("final database server id: %d", databaseServerId)
	return databaseServerId
}
