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
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type YugabyteDBCDCClient struct {
	exportDir string
	ybServers string
	dbName    string
	//Any one table name in the database is required by yb-client createCDCStream(...) API
	tableName          string
	sslRootCert        string
	ybCdcClientJarPath string
	ybMasterNodes      string
	metaDB             *metadb.MetaDB
}

func NewYugabyteDBCDCClient(exportDir, ybServers, sslRootCert, dbName, tableName string, metaDB *metadb.MetaDB) *YugabyteDBCDCClient {
	return &YugabyteDBCDCClient{
		exportDir:   exportDir,
		ybServers:   ybServers,
		dbName:      dbName,
		tableName:   tableName,
		sslRootCert: sslRootCert,
		metaDB:      metaDB,
	}
}

func (ybc *YugabyteDBCDCClient) Init() error {
	err := findDebeziumDistribution("yugabytedb")
	if err != nil {
		return fmt.Errorf("error in finding debezium distribution: %s", err)
	}
	ybc.ybCdcClientJarPath = fmt.Sprintf("%s/yb-client-cdc-stream-wrapper.jar", DEBEZIUM_DIST_DIR)
	return nil
}

func (ybc *YugabyteDBCDCClient) GetStreamID() (string, error) {
	streamID, err := ybc.readYBStreamID()
	if err != nil && strings.Contains(err.Error(), "stream id not found") {
		streamID, err = ybc.GenerateAndStoreStreamID()
		if err != nil {
			return "", fmt.Errorf("failed to generate and store stream id: %w", err)
		}
		utils.PrintAndLog("Generated YugabyteDB CDC stream-id: %s", streamID)
		return streamID, nil
	} else if err != nil {
		return "", fmt.Errorf("failed to read stream id: %w", err)
	}
	utils.PrintAndLog("Using YugabyteDB CDC stream-id: %s", streamID)
	return streamID, nil
}

func (ybc *YugabyteDBCDCClient) GenerateAndStoreStreamID() (string, error) {
	args := fmt.Sprintf("-create -master_addresses %s -table_name %s -db_name %s ", ybc.ybMasterNodes, ybc.tableName, ybc.dbName)

	if ybc.sslRootCert != "" {
		args += fmt.Sprintf(" -ssl_cert_file %s", ybc.sslRootCert)
	}

	stdout, err := ybc.runCommand(args)
	if err != nil {
		return "", fmt.Errorf("running command with args: %s, error: %s", args, err)
	}
	//stdout - CDC Stream ID: <stream_id>
	streamID := strings.Trim(strings.Split(stdout, ":")[1], " \n")
	err = ybc.metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.YBCDCStreamID = streamID
	})
	if err != nil {
		return "", fmt.Errorf("failed to update migration status record: %w", err)
	}
	return streamID, nil
}

func (ybc *YugabyteDBCDCClient) readYBStreamID() (string, error) {
	msr, err := ybc.metaDB.GetMigrationStatusRecord()
	if err != nil {
		return "", fmt.Errorf("failed to get migration status record: %w", err)
	}
	if msr != nil && msr.YBCDCStreamID != "" {
		return msr.YBCDCStreamID, nil
	}
	return "", fmt.Errorf("yugabytedb cdc stream id not found in msr" )
}

func (ybc *YugabyteDBCDCClient) DeleteStreamID() error {
	streamID, err := ybc.readYBStreamID()
	if err != nil && strings.Contains(err.Error(), "stream id not found") {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to read stream id: %w", err)
	}
	args := fmt.Sprintf("-delete_stream %s -master_addresses %s ", streamID, ybc.ybMasterNodes)

	if ybc.sslRootCert != "" {
		args += fmt.Sprintf(" -ssl_cert_file %s", ybc.sslRootCert)
	}

	_, err = ybc.runCommand(args)
	if err != nil {
		return fmt.Errorf("running command with args: %s, error: %s", args, err)
	}
	err = ybc.metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.YBCDCStreamID = ""
	})
	if err != nil {
		return fmt.Errorf("failed to update migration status record: %w", err)
	}
	utils.PrintAndLog("Deleted YugabyteDB CDC stream-id: %s", streamID)
	return nil
}

func (ybc *YugabyteDBCDCClient) ListMastersNodes() (string, error) {
	tserverPort := "9100" //TODO: make it internally handled by yb-client
	if os.Getenv("YB_TSERVER_PORT") != "" {
		tserverPort = os.Getenv("YB_TSERVER_PORT")
	}
	args := fmt.Sprintf("-list_masters -master_addresses %s -tserver_port %s", ybc.ybServers, tserverPort)

	if ybc.sslRootCert != "" {
		args += fmt.Sprintf(" -ssl_cert_file %s", ybc.sslRootCert)
	}

	stdout, err := ybc.runCommand(args)
	if err != nil {
		return "", fmt.Errorf("running command with args: %s, error: %s", args, err)
	}
	//stdout - Master Addresses: <comma_separated_list_addresses>
	masterAddresses := strings.Trim(strings.Split(stdout, ": ")[1], " \n")
	ybc.ybMasterNodes = masterAddresses
	return masterAddresses, nil
}

func (ybc *YugabyteDBCDCClient) runCommand(args string) (string, error) {
	command := fmt.Sprintf("java -jar %s %s", ybc.ybCdcClientJarPath, args)

	cmd := exec.CommandContext(context.Background(), "/bin/bash", "-c", command)
	var outbuf bytes.Buffer
	var errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf
	err := cmd.Start()
	if err != nil {
		if outbuf.String() != "" {
			log.Infof("Output of the command %s: %s", command, outbuf.String())
		}
		log.Errorf("Failed to start command: %s, error: %s", command, errbuf.String())
		return outbuf.String(), fmt.Errorf("failed to start command: %s, error: %w", command, err)
	}
	err = cmd.Wait()
	if err != nil {
		if outbuf.String() != "" {
			log.Infof("Output of the command %s: %s", command, outbuf.String())
		}
		log.Errorf("Failed to wait for command: %s , error: %s", command, errbuf.String())
		return outbuf.String(), fmt.Errorf("failed to wait for command: %s , error: %w", command, err)
	}

	return outbuf.String(), nil
}
