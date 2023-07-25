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
package tgtdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type TargetOracleDB struct {
	sync.Mutex
	tconf    *TargetConf
	conn_    *sql.DB
	connPool *ConnectionPool // Connection pool needs to be implemented for Oracle
}

func newTargetOracleDB(tconf *TargetConf) TargetDB {
	return &TargetOracleDB{tconf: tconf}
}

func (db *TargetOracleDB) connect() error {
	conn, err := sql.Open("godror", db.getConnectionUri(db.tconf))
	db.conn_ = conn
	return err
}

func (db *TargetOracleDB) disconnect() {
	if db.conn_ != nil {
		log.Infof("No connection to the target database to close")
	}

	err := db.conn_.Close()
	if err != nil {
		log.Errorf("Failed to close connection to the target database: %v", err)
	}
}

func (db *TargetOracleDB) reconnect() error {
	db.Mutex.Lock()
	defer db.Mutex.Unlock()

	var err error
	db.disconnect()
	for attempt := 1; attempt < 5; attempt++ {
		err = db.connect()
		if err == nil {
			return nil
		}
		log.Infof("Failed to reconnect to the target database: %s", err)
		time.Sleep(time.Duration(attempt*2) * time.Second)
		// Retry.
	}
	return fmt.Errorf("reconnect to target db: %w", err)
}

func (db *TargetOracleDB) GetConnection() *sql.DB {
	if db.conn_ == nil {
		utils.ErrExit("Called target db GetConnection() before Init()")
	}
	return db.conn_
}

func (db *TargetOracleDB) Init() error {
	return db.connect()
}

func (db *TargetOracleDB) Finalize() {
	db.disconnect()
}

func (db *TargetOracleDB) getTargetSchemaName(tableName string) string {
	parts := strings.Split(tableName, ".")
	if len(parts) == 2 {
		return parts[0]
	}
	return db.tconf.Schema // default set to "public"
}

func (db *TargetOracleDB) CleanFileImportState(filePath, tableName string) error {
	// // Delete all entries from ${BATCH_METADATA_TABLE_NAME} for the given file.
	// schemaName := db.getTargetSchemaName(tableName)
	// cmd := fmt.Sprintf(
	// 	`DELETE FROM %s WHERE data_file_name = '%s' AND schema_name = '%s' AND table_name = '%s';`,
	// 	BATCH_METADATA_TABLE_NAME, filePath, schemaName, tableName)
	// res, err := db.conn_.ExecContext(context.Background(), cmd)
	// if err != nil {
	// 	return fmt.Errorf("remove %q related entries from %s: %w", tableName, BATCH_METADATA_TABLE_NAME, err)
	// }
	// rowsAffected, _ := res.RowsAffected()
	// log.Infof("query: [%s] => rows affected %v", cmd, rowsAffected)
	// return nil
	return nil
}

func (db *TargetOracleDB) GetVersion() string {
	var version string
	query := "SELECT BANNER FROM V$VERSION"
	// query sample output: Oracle Database 19c Enterprise Edition Release 19.0.0.0.0 - Production
	err := db.conn_.QueryRow(query).Scan(&version)
	if err != nil {
		utils.ErrExit("run query %q on source: %s", query, err)
	}
	return version
}

func (db *TargetOracleDB) CreateVoyagerSchema() error {
	// 	createUserQuery := fmt.Sprintf("CREATE USER %s IDENTIFIED BY password;", BATCH_METADATA_TABLE_SCHEMA)
	// 	grantQuery := fmt.Sprintf("GRANT CONNECT, RESOURCE TO %s;", BATCH_METADATA_TABLE_SCHEMA)
	// 	alterQuery := fmt.Sprintf("ALTER USER %s QUOTA UNLIMITED ON USERS;", BATCH_METADATA_TABLE_SCHEMA)

	// 	cmds := []string{
	// 		createUserQuery,
	// 		grantQuery,
	// 		alterQuery,
	// 		fmt.Sprintf(`CREATE TABLE %s (
	// 			data_file_name VARCHAR2(250),
	// 			batch_number NUMBER(10),
	// 			schema_name VARCHAR2(250),
	// 			table_name VARCHAR2(250),
	// 			rows_imported NUMBER(19),
	// 			PRIMARY KEY (data_file_name, batch_number, schema_name, table_name)
	// 		);`, BATCH_METADATA_TABLE_NAME),
	// 	}

	// 	maxAttempts := 12
	// 	var err error

	// outer:
	// 	for _, cmd := range cmds {
	// 		for attempt := 1; attempt <= maxAttempts; attempt++ {
	// 			log.Infof("Executing on target: [%s]", cmd)
	// 			conn := db.GetConnection()
	// 			_, err = conn.ExecContext(context.Background(), cmd)
	// 			if err == nil {
	// 				// No error. Move on to the next command.
	// 				continue outer
	// 			}
	// 			log.Warnf("Error while running [%s] attempt %d: %s", cmd, attempt, err)
	// 			time.Sleep(5 * time.Second)
	// 			err = db.reconnect()
	// 			if err != nil {
	// 				break
	// 			}
	// 		}
	// 		if err != nil {
	// 			return fmt.Errorf("create ybvoyager schema on target: %w", err)
	// 		}
	// 	}
	return nil
}

func (db *TargetOracleDB) GetNonEmptyTables(tables []string) []string {
	// result := []string{}

	// for _, table := range tables {
	// 	log.Infof("Checking if table %s is empty", table)
	// 	tmp := false
	// 	stmt := fmt.Sprintf("SELECT 1 FROM %s WHERE ROWNUM <= 1", table)
	// 	err := db.conn_.QueryRow(stmt).Scan(&tmp)
	// 	if err != nil {
	// 		log.Errorf("Failed to check if table %s is empty: %v", table, err)
	// 		continue
	// 	}
	// 	if tmp {
	// 		result = append(result, table)
	// 	}
	// }

	// return result
	return nil
}

func (db *TargetOracleDB) IsNonRetryableCopyError(err error) bool {
	return false
}

func (db *TargetOracleDB) ImportBatch(batch Batch, args *ImportBatchArgs, exportDir string) (int64, error) {
	// var rowsAffected int64
	// var err error
	// copyFn := func(conn *sql.Conn) (bool, error) {
	// 	rowsAffected, err = db.importBatch(conn, batch, args, exportDir)
	// 	return false, err
	// }
	// err = db.WithConn(copyFn)
	// return rowsAffected, err
	return 0, nil
}

func (db *TargetOracleDB) WithConn(fn func(*sql.Conn) (bool, error)) error {
	var err error
	if db.conn_ == nil {
		err = db.reconnect()
		if err != nil {
			return err
		}
	}
	retry := true
	for retry {
		conn, err := db.conn_.Conn(context.Background())
		if err != nil {
			return fmt.Errorf("get connection from target db: %w", err)
		}
		retry, err = fn(conn)
		if err != nil {
			// On err, drop the connection.
			conn.Close()
		}
	}

	return err
}

func (db *TargetOracleDB) importBatch(conn *sql.Conn, batch Batch, args *ImportBatchArgs, exportDir string) (rowsAffected int64, err error) {
	// var file *os.File
	// file, err = batch.Open()
	// if err != nil {
	// 	return 0, fmt.Errorf("open batch file %q: %w", batch.GetFilePath(), err)
	// }
	// defer file.Close()

	// //setting the schema so that the table is created in the correct schema
	// db.setTargetSchema(conn)

	// ctx := context.Background()
	// var tx *sql.Tx
	// tx, err = conn.BeginTx(ctx, &sql.TxOptions{})
	// if err != nil {
	// 	return 0, fmt.Errorf("begin transaction: %w", err)
	// }
	// defer func() {
	// 	var err2 error
	// 	if err != nil {
	// 		err2 = tx.Rollback(ctx)
	// 		if err2 != nil {
	// 			rowsAffected = 0
	// 			err = fmt.Errorf("rollback transaction: %w (while processing %s)", err2, err)
	// 		}
	// 	} else {
	// 		err2 = tx.Commit(ctx)
	// 		if err2 != nil {
	// 			rowsAffected = 0
	// 			err = fmt.Errorf("commit transaction: %w", err2)
	// 		}
	// 	}
	// }()

	// // Check if split is already imported
	// var alreadyImported bool
	// alreadyImported, rowsAffected, err = db.isBatchAlreadyImported(tx, batch)
	// if err != nil {
	// 	return 0, err
	// }
	// if alreadyImported {
	// 	return rowsAffected, nil
	// }

	// // Import the split using sqldr
	// sqlldrControlFile := args.GetSqlLdrControlFile()
	// // Create sqlldr control file at export-dir/sqlldr
	// // Create folder if it does not exist
	// if _, err := os.Stat(fmt.Sprintf("%s/sqlldr", exportDir)); os.IsNotExist(err) {
	// 	os.Mkdir(fmt.Sprintf("%s/sqlldr", exportDir), 0755)
	// }
	// sqlldrControlFilePath := fmt.Sprintf("%s/sqlldr/%s.ctl", exportDir, batch.GetBatchId())
	return 0, nil
}

func (db *TargetOracleDB) isBatchAlreadyImported(tx *sql.Tx, batch Batch) (bool, int64, error) {
	// var rowsImported int64
	// query := batch.GetQueryIsBatchAlreadyImported()
	// err := tx.QueryRowContext(context.Background(), query).Scan(&rowsImported)
	// if err == nil {
	// 	log.Infof("%v rows from %q are already imported", rowsImported, batch.GetFilePath())
	// 	return true, rowsImported, nil
	// }
	// if err == sql.ErrNoRows {
	// 	log.Infof("%q is not imported yet", batch.GetFilePath())
	// 	return false, 0, nil
	// }
	// return false, 0, fmt.Errorf("check if %s is already imported: %w", batch.GetFilePath(), err)
	return false, 0, nil
}

func (db *TargetOracleDB) setTargetSchema(conn *sql.Conn) {
	// // Set the target schema.
	// checkSchemaExistsQuery := fmt.Sprintf(
	// 	"SELECT 1 FROM ALL_USERS WHERE USERNAME = '%s'",
	// 	db.tconf.Schema)
	// var cntSchemaName int

	// if err := conn.QueryRowContext(context.Background(), checkSchemaExistsQuery).Scan(&cntSchemaName); err != nil {
	// 	utils.ErrExit("run query %q on target %q to check schema exists: %s", checkSchemaExistsQuery, db.tconf.Host, err)
	// } else if cntSchemaName == 0 {
	// 	utils.ErrExit("schema '%s' does not exist in target", db.tconf.Schema)
	// }

	// setSchemaQuery := fmt.Sprintf("ALTER SESSION SET CURRENT_SCHEMA = %s", db.tconf.Schema)
	// _, err := conn.ExecContext(context.Background(), setSchemaQuery)
	// if err != nil {
	// 	utils.ErrExit("run query %q on target %q to set schema: %s", setSchemaQuery, db.tconf.Host, err)
	// }
	fmt.Printf("setTargetSchema() not implemented")
}

func (db *TargetOracleDB) IfRequiredQuoteColumnNames(tableName string, columns []string) ([]string, error) {
	return nil, nil
}

func (db *TargetOracleDB) ExecuteBatch(batch []*Event) error {
	return nil
}

func (db *TargetOracleDB) InitConnPool() error {
	return nil
}

func (db *TargetOracleDB) getConnectionUri(tconf *TargetConf) string {
	if tconf.Uri != "" {
		return tconf.Uri
	}

	switch true {
	case tconf.DBSid != "":
		tconf.Uri = fmt.Sprintf(`user="%s" password="%s" connectString="(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%d))(CONNECT_DATA=(SID=%s)))"`,
			tconf.User, tconf.Password, tconf.Host, tconf.Port, tconf.DBSid)

	case tconf.TNSAlias != "":
		tconf.Uri = fmt.Sprintf(`user="%s" password="%s" connectString="%s"`, tconf.User, tconf.Password, tconf.TNSAlias)

	case tconf.DBName != "":
		tconf.Uri = fmt.Sprintf(`user="%s" password="%s" connectString="(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%d))(CONNECT_DATA=(SERVICE_NAME=%s)))"`,
			tconf.User, tconf.Password, tconf.Host, tconf.Port, tconf.DBName)
	}

	return tconf.Uri
}
