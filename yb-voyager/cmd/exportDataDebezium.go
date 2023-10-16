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
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gosuri/uilive"
	"github.com/magiconair/properties"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func prepareDebeziumConfig(tableList []*sqlname.SourceName, tablesColumnList map[*sqlname.SourceName][]string) (*dbzm.Config, map[string]int64, error) {
	runId = time.Now().String()
	absExportDir, err := filepath.Abs(exportDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get absolute path for export dir: %v", err)
	}

	var snapshotMode string
	switch exportType {
	case SNAPSHOT_AND_CHANGES:
		snapshotMode = "initial"
	case CHANGES_ONLY:
		snapshotMode = "never"
	case SNAPSHOT_ONLY:
		snapshotMode = "initial_only"
	default:
		return nil, nil, fmt.Errorf("invalid export type %s", exportType)
	}

	tableList = filterTablePartitions(tableList)
	fmt.Printf("num tables to export: %d\n", len(tableList))
	utils.PrintAndLog("table list for data export: %v", tableList)
	tableNameToApproxRowCountMap := getTableNameToApproxRowCountMap(tableList)

	var dbzmTableList, dbzmColumnList []string
	for _, table := range tableList {
		dbzmTableList = append(dbzmTableList, table.Qualified.Unquoted)
	}
	if exporterRole == SOURCE_DB_EXPORTER_ROLE && changeStreamingIsEnabled(exportType) {
		err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
			record.TableListExportedFromSource = dbzmTableList
		})
		if err != nil {
			utils.ErrExit("error while updating fall forward db exists in meta db: %v", err)
		}
	}

	for tableName, columns := range tablesColumnList {
		for _, column := range columns {
			columnName := fmt.Sprintf("%s.%s", tableName.Qualified.Unquoted, column)
			if column == "*" {
				dbzmColumnList = append(dbzmColumnList, columnName) //for all columns <schema>.<table>.*
				break
			}
			dbzmColumnList = append(dbzmColumnList, columnName) // if column is PK, then data for it will come from debezium
		}
	}

	var columnSequenceMap []string
	colToSeqMap := source.DB().GetColumnToSequenceMap(tableList)
	for column, sequence := range colToSeqMap {
		columnSequenceMap = append(columnSequenceMap, fmt.Sprintf("%s:%s", column, sequence))
	}
	err = prepareSSLParamsForDebezium(absExportDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare ssl params for debezium: %w", err)
	}

	config := &dbzm.Config{
		RunId:          runId,
		SourceDBType:   source.DBType,
		ExporterRole:   exporterRole,
		ExportDir:      absExportDir,
		MetadataDBPath: metadb.GetMetaDBPath(absExportDir),
		Host:           source.Host,
		Port:           source.Port,
		Username:       source.User,
		Password:       source.Password,

		DatabaseName:      source.DBName,
		SchemaNames:       source.Schema,
		TableList:         dbzmTableList,
		ColumnList:        dbzmColumnList,
		ColumnSequenceMap: columnSequenceMap,

		SSLMode:               source.SSLMode,
		SSLCertPath:           source.SSLCertPath,
		SSLKey:                source.SSLKey,
		SSLRootCert:           source.SSLRootCert,
		SSLKeyStore:           source.SSLKeyStore,
		SSLKeyStorePassword:   source.SSLKeyStorePassword,
		SSLTrustStore:         source.SSLTrustStore,
		SSLTrustStorePassword: source.SSLTrustStorePassword,
		SnapshotMode:          snapshotMode,
	}
	if source.DBType == "oracle" {
		jdbcConnectionStringPrefix := "jdbc:oracle:thin:@"
		if source.IsOracleCDBSetup() {
			// uri = cdb uri
			connectionString := srcdb.GetOracleConnectionString(source.Host, source.Port, source.CDBName, source.CDBSid, source.CDBTNSAlias)
			config.Uri = fmt.Sprintf("%s%s", jdbcConnectionStringPrefix, connectionString)
			config.PDBName = source.DBName
		} else {
			connectionString := srcdb.GetOracleConnectionString(source.Host, source.Port, source.DBName, source.DBSid, source.TNSAlias)
			config.Uri = fmt.Sprintf("%s%s", jdbcConnectionStringPrefix, connectionString)
		}

		config.TNSAdmin, err = getTNSAdmin(source)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get tns admin: %w", err)
		}
		config.OracleJDBCWalletLocationSet, err = isOracleJDBCWalletLocationSet(source)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to determine if Oracle JDBC wallet location is set: %v", err)
		}
	} else if source.DBType == "yugabytedb" {
		if exportType == CHANGES_ONLY {
			ybServers := source.DB().GetServers()
			masterPort := "7100"
			if os.Getenv("YB_MASTER_PORT") != "" {
				masterPort = os.Getenv("YB_MASTER_PORT")
			}
			ybServers = lo.Map(ybServers, (func(s string, _ int) string {
				return fmt.Sprintf("%s:%s", s, masterPort)
			}),
			)
			ybCDCClient := dbzm.NewYugabyteDBCDCClient(exportDir, strings.Join(ybServers, ","), config.SSLRootCert, config.DatabaseName, config.TableList[0])
			err := ybCDCClient.Init()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to initialize YugabyteDB CDC client: %w", err)
			}
			config.YBMasterNodes, err = ybCDCClient.ListMastersNodes()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to list master nodes: %w", err)
			}
			if startClean {
				err = ybCDCClient.DeleteStreamID()
				if err != nil {
					return nil, nil, fmt.Errorf("failed to delete stream id: %w", err)
				}
				config.YBStreamID, err = ybCDCClient.GenerateAndStoreStreamID()
				if err != nil {
					return nil, nil, fmt.Errorf("failed to generate stream id: %w", err)
				}
				utils.PrintAndLog("Generated new YugabyteDB CDC stream-id: %s", config.YBStreamID)
			} else {
				config.YBStreamID, err = ybCDCClient.GetStreamID()
				if err != nil {
					return nil, nil, fmt.Errorf("failed to get stream id: %w", err)
				}
			}
		}
	}
	return config, tableNameToApproxRowCountMap, nil
}

// required only for postgresql since GetAllTables() returns all tables and partitions
func filterTablePartitions(tableList []*sqlname.SourceName) []*sqlname.SourceName {
	if source.DBType != POSTGRESQL || source.TableList != "" {
		return tableList
	}

	filteredTableList := []*sqlname.SourceName{}
	for _, table := range tableList {
		if !source.DB().IsTablePartition(table) {
			filteredTableList = append(filteredTableList, table)
		}
	}
	return filteredTableList
}

func prepareSSLParamsForDebezium(exportDir string) error {
	switch source.DBType {
	case "postgresql", "yugabytedb": //TODO test for yugabytedb
		if source.SSLKey != "" {
			targetSslKeyPath := filepath.Join(exportDir, "metainfo", "ssl", ".key.der")
			err := dbzm.WritePKCS8PrivateKeyPEMasDER(source.SSLKey, targetSslKeyPath)
			if err != nil {
				return fmt.Errorf("could not write private key PEM as DER: %w", err)
			}
			utils.PrintAndLog("Converted SSL key from PEM to DER format. File saved at %s", targetSslKeyPath)
			source.SSLKey = targetSslKeyPath
		}
	case "mysql":
		switch source.SSLMode {
		case "disable":
			source.SSLMode = "disabled"
		case "prefer":
			source.SSLMode = "preferred"
		case "require":
			source.SSLMode = "required"
		case "verify-ca":
			source.SSLMode = "verify_ca"
		case "verify-full":
			source.SSLMode = "verify_identity"
		}
		if source.SSLKey != "" {
			keyStorePath := filepath.Join(exportDir, "metainfo", "ssl", ".keystore.jks")
			keyStorePassword := utils.GenerateRandomString(8)
			err := dbzm.WritePKCS8PrivateKeyCertAsJavaKeystore(source.SSLKey, source.SSLCertPath, "mysqlclient", keyStorePassword, keyStorePath)
			if err != nil {
				return fmt.Errorf("failed to write java keystore for debezium: %w", err)
			}
			utils.PrintAndLog("Converted SSL key, cert to java keystore. File saved at %s", keyStorePath)
			source.SSLKeyStore = keyStorePath
			source.SSLKeyStorePassword = keyStorePassword
		}
		if source.SSLRootCert != "" {
			trustStorePath := filepath.Join(exportDir, "metainfo", "ssl", ".truststore.jks")
			trustStorePassword := utils.GenerateRandomString(8)
			err := dbzm.WriteRootCertAsJavaTrustStore(source.SSLRootCert, "MySQLCACert", trustStorePassword, trustStorePath)
			if err != nil {
				return fmt.Errorf("failed to write java truststore for debezium: %w", err)
			}
			utils.PrintAndLog("Converted SSL root cert to java truststore. File saved at %s", trustStorePath)
			source.SSLTrustStore = trustStorePath
			source.SSLTrustStorePassword = trustStorePassword
		}
	case "oracle":
	}
	return nil
}

// https://www.orafaq.com/wiki/TNS_ADMIN
// default is $ORACLE_HOME/network/admin
func getTNSAdmin(s srcdb.Source) (string, error) {
	if s.DBType != "oracle" {
		return "", fmt.Errorf("invalid source db type %s for getting TNS_ADMIN", s.DBType)
	}
	tnsAdminEnvVar, present := os.LookupEnv("TNS_ADMIN")
	if present {
		return tnsAdminEnvVar, nil
	} else {
		return filepath.Join(s.GetOracleHome(), "network", "admin"), nil
	}
}

// oracle wallet location can be optionally set in $TNS_ADMIN/ojdbc.properties as
// oracle.net.wallet_location=<>
func isOracleJDBCWalletLocationSet(s srcdb.Source) (bool, error) {
	if s.DBType != "oracle" {
		return false, fmt.Errorf("invalid source db type %s for checking jdbc wallet location", s.DBType)
	}
	tnsAdmin, err := getTNSAdmin(s)
	if err != nil {
		return false, fmt.Errorf("failed to get tns admin: %w", err)
	}
	ojdbcPropertiesFilePath := filepath.Join(tnsAdmin, "ojdbc.properties")
	if _, err := os.Stat(ojdbcPropertiesFilePath); errors.Is(err, os.ErrNotExist) {
		// file does not exist
		return false, nil
	}
	ojdbcProperties := properties.MustLoadFile(ojdbcPropertiesFilePath, properties.UTF8)
	walletLocationKey := "oracle.net.wallet_location"
	_, present := ojdbcProperties.Get(walletLocationKey)
	return present, nil
}

// ---------------------------------------------- Export Data ---------------------------------------//

func debeziumExportData(ctx context.Context, config *dbzm.Config, tableNameToApproxRowCountMap map[string]int64) error {
	progressTracker := NewProgressTracker(tableNameToApproxRowCountMap)
	debezium := dbzm.NewDebezium(config)
	err := debezium.Start()
	if err != nil {
		return fmt.Errorf("failed to start debezium: %w", err)
	}

	var status *dbzm.ExportStatus
	snapshotComplete := false
	for debezium.IsRunning() {
		status, err = debezium.GetExportStatus()
		if err != nil {
			return fmt.Errorf("failed to read export status: %w", err)
		}
		if status == nil {
			time.Sleep(2 * time.Second)
			continue
		}
		progressTracker.UpdateProgress(status)
		if !snapshotComplete {
			snapshotComplete, err = checkAndHandleSnapshotComplete(status, progressTracker)
			if err != nil {
				return fmt.Errorf("failed to check if snapshot is complete: %w", err)
			}
		}
		time.Sleep(time.Millisecond * 500)
	}
	if err := debezium.Error(); err != nil {
		return fmt.Errorf("debezium failed with error: %w", err)
	}
	// handle case where debezium finished before snapshot completion
	// was handled in above loop
	if !snapshotComplete {
		status, err = debezium.GetExportStatus()
		if err != nil {
			return fmt.Errorf("failed to read export status: %w", err)
		}
		snapshotComplete, err = checkAndHandleSnapshotComplete(status, progressTracker)
		if !snapshotComplete || err != nil {
			return fmt.Errorf("snapshot was not completed: %w", err)
		}
	}

	log.Info("Debezium exited normally.")
	return nil
}

func reportStreamingProgress() {
	tableWriter := uilive.New()
	headerWriter := tableWriter.Newline()
	separatorWriter := tableWriter.Newline()
	row1Writer := tableWriter.Newline()
	row2Writer := tableWriter.Newline()
	row3Writer := tableWriter.Newline()
	row4Writer := tableWriter.Newline()
	footerWriter := tableWriter.Newline()
	tableWriter.Start()
	for {
		totalEventCount, totalEventCountRun, err := metaDB.GetTotalExportedEvents(runId)
		if err != nil {
			utils.ErrExit("failed to get total exported count from metadb: %w", err)
		}
		throughputInLast3Min, err := metaDB.GetExportedEventsRateInLastNMinutes(runId, 3)
		if err != nil {
			utils.ErrExit("failed to get export rate from metadb: %w", err)
		}
		throughputInLast10Min, err := metaDB.GetExportedEventsRateInLastNMinutes(runId, 10)
		if err != nil {
			utils.ErrExit("failed to get export rate from metadb: %w", err)
		}
		fmt.Fprint(tableWriter, color.GreenString("| %-40s | %30s |\n", "---------------------------------------", "-----------------------------"))
		fmt.Fprint(headerWriter, color.GreenString("| %-40s | %30s |\n", "Metric", "Value"))
		fmt.Fprint(separatorWriter, color.GreenString("| %-40s | %30s |\n", "---------------------------------------", "-----------------------------"))
		fmt.Fprint(row1Writer, color.GreenString("| %-40s | %30s |\n", "Total Exported Events", strconv.FormatInt(totalEventCount, 10)))
		fmt.Fprint(row2Writer, color.GreenString("| %-40s | %30s |\n", "Total Exported Events (Current Run)", strconv.FormatInt(totalEventCountRun, 10)))
		fmt.Fprint(row3Writer, color.GreenString("| %-40s | %30s |\n", "Export Rate(Last 3 min)", strconv.FormatInt(throughputInLast3Min, 10)+"/sec"))
		fmt.Fprint(row4Writer, color.GreenString("| %-40s | %30s |\n", "Export Rate(Last 10 min)", strconv.FormatInt(throughputInLast10Min, 10)+"/sec"))
		fmt.Fprint(footerWriter, color.GreenString("| %-40s | %30s |\n", "---------------------------------------", "-----------------------------"))
		tableWriter.Flush()
		time.Sleep(10 * time.Second)
	}
}

func checkAndHandleSnapshotComplete(status *dbzm.ExportStatus, progressTracker *ProgressTracker) (bool, error) {
	if !status.SnapshotExportIsComplete() {
		return false, nil
	}
	progressTracker.Done(status)
	setDataIsExported()
	err := writeDataFileDescriptor(exportDir, status)
	if err != nil {
		return false, fmt.Errorf("failed to write data file descriptor: %w", err)
	}
	log.Infof("snapshot export is complete.")
	err = renameDbzmExportedDataFiles()
	if err != nil {
		return false, fmt.Errorf("failed to rename dbzm exported data files: %v", err)
	}
	if source.DBType != YUGABYTEDB {
		displayExportedRowCountSnapshot()
	}
	if changeStreamingIsEnabled(exportType) {
		color.Blue("streaming changes to a local queue file...")
		if !disablePb {
			go reportStreamingProgress()
		}
	}
	return true, nil
}

func writeDataFileDescriptor(exportDir string, status *dbzm.ExportStatus) error {
	dataFileList := make([]*datafile.FileEntry, 0)
	for _, table := range status.Tables {
		// TODO: TableName and FilePath must be quoted by debezium plugin.
		tableName := quoteIdentifierIfRequired(table.TableName)
		if source.DBType == POSTGRESQL && table.SchemaName != "public" {
			tableName = fmt.Sprintf("%s.%s", table.SchemaName, tableName)
		}
		fileEntry := &datafile.FileEntry{
			TableName: tableName,
			FilePath:  fmt.Sprintf("%s_data.sql", tableName),
			RowCount:  table.ExportedRowCountSnapshot,
			FileSize:  -1, // Not available.
		}
		dataFileList = append(dataFileList, fileEntry)
	}
	dfd := datafile.Descriptor{
		FileFormat:   datafile.CSV,
		Delimiter:    ",",
		HasHeader:    true,
		NullString:   utils.YB_VOYAGER_NULL_STRING,
		ExportDir:    exportDir,
		DataFileList: dataFileList,
	}
	dfd.Save()
	return nil
}

// handle renaming for tables having case sensitivity and reserved keywords
func renameDbzmExportedDataFiles() error {
	status, err := dbzm.ReadExportStatus(filepath.Join(exportDir, "data", "export_status.json"))
	if err != nil {
		return fmt.Errorf("failed to read export status during renaming exported data files: %w", err)
	}
	if status == nil {
		return fmt.Errorf("export status is empty during renaming exported data files")
	}

	for i := 0; i < len(status.Tables); i++ {
		tableName := status.Tables[i].TableName
		// either case sensitive(postgresql) or reserved keyword(any source db)
		if (!sqlname.IsAllLowercase(tableName) && source.DBType == POSTGRESQL) ||
			sqlname.IsReservedKeywordPG(tableName) {
			tableName = fmt.Sprintf("\"%s\"", status.Tables[i].TableName)
		}

		oldFilePath := filepath.Join(exportDir, "data", status.Tables[i].FileName)
		newFilePath := filepath.Join(exportDir, "data", tableName+"_data.sql")
		if status.Tables[i].SchemaName != "public" && source.DBType == POSTGRESQL {
			newFilePath = filepath.Join(exportDir, "data", status.Tables[i].SchemaName+"."+tableName+"_data.sql")
		}

		log.Infof("Renaming %s to %s", oldFilePath, newFilePath)
		err = os.Rename(oldFilePath, newFilePath)
		if err != nil {
			return fmt.Errorf("failed to rename dbzm exported data file: %w", err)
		}

		//rename table schema file as well
		oldTableSchemaFilePath := filepath.Join(exportDir, "data", "schemas", SOURCE_DB_EXPORTER_ROLE, strings.Replace(status.Tables[i].FileName, "_data.sql", "_schema.json", 1))
		newTableSchemaFilePath := filepath.Join(exportDir, "data", "schemas", SOURCE_DB_EXPORTER_ROLE, tableName+"_schema.json")
		if status.Tables[i].SchemaName != "public" && source.DBType == POSTGRESQL {
			newTableSchemaFilePath = filepath.Join(exportDir, "data", "schemas", SOURCE_DB_EXPORTER_ROLE, status.Tables[i].SchemaName+"."+tableName+"_schema.json")
		}
		log.Infof("Renaming %s to %s", oldTableSchemaFilePath, newTableSchemaFilePath)
		err = os.Rename(oldTableSchemaFilePath, newTableSchemaFilePath)
		if err != nil {
			return fmt.Errorf("failed to rename dbzm exported table schema file: %w", err)
		}
	}
	return nil
}
