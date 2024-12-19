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
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var deferredSqlStmts []sqlInfo
var finalFailedSqlStmts []string

// The client message (NOTICE/WARNING) from psql is stored in this global variable.
// as part of the noticeHandler function for every query executed.
var notice *pgconn.Notice

func importSchemaInternal(exportDir string, importObjectList []string,
	skipFn func(string, string) bool) error {
	schemaDir := filepath.Join(exportDir, "schema")
	for _, importObjectType := range importObjectList {
		importObjectFilePath := utils.GetObjectFilePath(schemaDir, importObjectType)
		if !utils.FileOrFolderExists(importObjectFilePath) {
			continue
		}
		err := executeSqlFile(importObjectFilePath, importObjectType, skipFn)
		if err != nil {
			return err
		}
	}
	return nil
}

func isNotValidConstraint(stmt string) (bool, error) {
	parseTree, err := queryparser.Parse(stmt)
	if err != nil {
		return false, fmt.Errorf("error parsing the ddl[%s]: %v", stmt, err)
	}
	ddlObj, err := queryparser.ProcessDDL(parseTree)
	if err != nil {
		return false, fmt.Errorf("error in process DDL[%s]:%v", stmt, err)
	}
	alter, ok := ddlObj.(*queryparser.AlterTable)
	if !ok {
		return false, nil
	}
	if alter.IsAddConstraintType() && alter.ConstraintNotValid {
		return true, nil
	}
	return false, nil
}

func executeSqlFile(file string, objType string, skipFn func(string, string) bool) error {
	log.Infof("Execute SQL file %q on target %q", file, tconf.Host)
	conn := newTargetConn()

	defer func() {
		if conn != nil {
			conn.Close(context.Background())
		}
	}()

	sqlInfoArr := parseSqlFileForObjectType(file, objType)
	var err error
	for _, sqlInfo := range sqlInfoArr {
		if conn == nil {
			conn = newTargetConn()
		}

		setOrSelectStmt := strings.HasPrefix(strings.ToUpper(sqlInfo.stmt), "SET ") ||
			strings.HasPrefix(strings.ToUpper(sqlInfo.stmt), "SELECT ")
		if !setOrSelectStmt && skipFn != nil && skipFn(objType, sqlInfo.stmt) {
			continue
		}

		if objType == "TABLE" {
			// Check if the statement should be skipped
			skip, err := shouldSkipDDL(sqlInfo.stmt)
			if err != nil {
				return fmt.Errorf("error checking whether to skip DDL for statement [%s]: %v", sqlInfo.stmt, err)
			}
			if skip {
				log.Infof("Skipping DDL: %s", sqlInfo.stmt)
				continue
			}
		}

		err = executeSqlStmtWithRetries(&conn, sqlInfo, objType)
		if err != nil {
			return err
		}
	}
	return nil
}

func shouldSkipDDL(stmt string) (bool, error) {
	stmt = strings.ToUpper(stmt)

	// pg_dump generate `SET client_min_messages = 'warning';`, but we want to get
	// NOTICE severity as well (which is the default), hence skipping this.
	if strings.Contains(stmt, "SET CLIENT_MIN_MESSAGES") {
		return true, nil
	}
	skipReplicaIdentity := strings.Contains(stmt, "ALTER TABLE") && strings.Contains(stmt, "REPLICA IDENTITY")
	if skipReplicaIdentity {
		return true, nil
	}
	isNotValid, err := isNotValidConstraint(stmt)
	if err != nil {
		return false, fmt.Errorf("error checking whether stmt is to add not valid constraint: %v", err)
	}
	skipNotValidWithoutPostImport := isNotValid && !bool(flagPostSnapshotImport)
	skipOtherDDLsWithPostImport := (bool(flagPostSnapshotImport) && !isNotValid)
	if skipNotValidWithoutPostImport || // Skipping NOT VALID CONSTRAINT in import schema without post-snapshot-mode
		skipOtherDDLsWithPostImport { // Skipping other TABLE DDLs than the NOT VALID in post-snapshot-import mode
		return true, nil
	}
	return false, nil
}

func executeSqlStmtWithRetries(conn **pgx.Conn, sqlInfo sqlInfo, objType string) error {
	var err error
	var stmtNotice *pgconn.Notice
	log.Infof("On %s run query:\n%s\n", tconf.Host, sqlInfo.formattedStmt)
	for retryCount := 0; retryCount <= DDL_MAX_RETRY_COUNT; retryCount++ {
		if retryCount > 0 { // Not the first iteration.
			log.Infof("Sleep for 5 seconds before retrying for %dth time", retryCount)
			time.Sleep(time.Second * 5)
			log.Infof("RETRYING DDL: %q", sqlInfo.stmt)
		}

		if bool(flagPostSnapshotImport) && strings.Contains(objType, "INDEX") {
			err = beforeIndexCreation(sqlInfo, conn, objType)
			if err != nil {
				(*conn).Close(context.Background())
				*conn = nil
				return fmt.Errorf("before index creation: %w", err)
			}
		}
		stmtNotice, err = execStmtAndGetNotice(*conn, sqlInfo.formattedStmt)
		if err == nil {
			utils.PrintSqlStmtIfDDL(sqlInfo.stmt, utils.GetObjectFileName(filepath.Join(exportDir, "schema"), objType),
				getNoticeMessage(stmtNotice))
			return nil
		}

		log.Errorf("DDL Execution Failed for %q: %s", sqlInfo.formattedStmt, err)
		if strings.Contains(strings.ToLower(err.Error()), "conflicts with higher priority transaction") {
			// creating fresh connection
			(*conn).Close(context.Background())
			*conn = newTargetConn()
			continue
		} else if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(SCHEMA_VERSION_MISMATCH_ERR)) &&
			(objType == "INDEX" || objType == "PARTITION_INDEX") { // retriable error
			// creating fresh connection
			(*conn).Close(context.Background())
			*conn = newTargetConn()

			// Extract the schema name and add to the index name
			fullyQualifiedObjName, err := getIndexName(sqlInfo.stmt, sqlInfo.objName)
			if err != nil {
				(*conn).Close(context.Background())
				*conn = nil
				return fmt.Errorf("extract qualified index name from DDL [%v]: %v", sqlInfo.stmt, err)
			}

			// DROP INDEX in case INVALID index got created
			// `err` is already being used for retries, so using `err2`
			err2 := dropIdx(*conn, fullyQualifiedObjName)
			if err2 != nil {
				(*conn).Close(context.Background())
				*conn = nil
				return fmt.Errorf("drop invalid index %q: %w", fullyQualifiedObjName, err2)
			}
			continue
		} else if missingRequiredSchemaObject(err) {
			log.Infof("deffering execution of SQL: %s", sqlInfo.formattedStmt)
			deferredSqlStmts = append(deferredSqlStmts, sqlInfo)
		} else if isAlreadyExists(err.Error()) {
			// pg_dump generates `CREATE SCHEMA public;` in the schemas.sql. Because the `public`
			// schema already exists on the target YB db, the create schema statement fails with
			// "already exists" error. Ignore the error.
			if bool(tconf.IgnoreIfExists) || strings.EqualFold(strings.Trim(sqlInfo.stmt, " \n"), "CREATE SCHEMA public;") {
				err = nil
			}
		}
		break // no more iteration in case of non retriable error
	}
	if err != nil {
		(*conn).Close(context.Background())
		*conn = nil
		if missingRequiredSchemaObject(err) {
			// Do nothing for deferred case
		} else {
			utils.PrintSqlStmtIfDDL(sqlInfo.stmt, utils.GetObjectFileName(filepath.Join(exportDir, "schema"), objType),
				getNoticeMessage(stmtNotice))
			color.Red(fmt.Sprintf("%s\n", err.Error()))
			if tconf.ContinueOnError {
				log.Infof("appending stmt to failedSqlStmts list: %s\n", utils.GetSqlStmtToPrint(sqlInfo.stmt))
				errString := fmt.Sprintf("/*\n%s\nFile :%s\n*/\n", err.Error(), sqlInfo.fileName)
				finalFailedSqlStmts = append(finalFailedSqlStmts, errString+sqlInfo.formattedStmt)
			} else {
				return err
			}
		}
		return nil
	}
	return err
}

/*
Try re-executing each DDL from deferred list.
If fails, silently avoid the error.
Else remove from deferredSQLStmts list
At the end, add the unsuccessful ones to a failedSqlStmts list and report to the user
*/
func importDeferredStatements() {
	if len(deferredSqlStmts) == 0 {
		return
	}
	log.Infof("Number of statements in deferredSQLStmts list: %d\n", len(deferredSqlStmts))

	utils.PrintAndLog("\nExecuting the remaining SQL statements...\n\n")
	maxIterations := len(deferredSqlStmts)
	conn := newTargetConn()
	defer func() { conn.Close(context.Background()) }()

	var err error
	var finalFailedDeferredStmts []string
	// max loop iterations to remove all errors
	for i := 1; i <= maxIterations && len(deferredSqlStmts) > 0; i++ {
		beforeDeferredSqlCount := len(deferredSqlStmts)
		var failedSqlStmtInIthIteration []string
		for j := 0; j < len(deferredSqlStmts); j++ {
			var stmtNotice *pgconn.Notice
			stmtNotice, err = execStmtAndGetNotice(conn, deferredSqlStmts[j].formattedStmt)
			if err == nil {
				utils.PrintAndLog("%s\n", utils.GetSqlStmtToPrint(deferredSqlStmts[j].stmt))
				noticeMsg := getNoticeMessage(stmtNotice)
				if noticeMsg != "" {
					utils.PrintAndLog(color.YellowString("%s\n", noticeMsg))
				}
				// removing successfully executed SQL
				deferredSqlStmts = append(deferredSqlStmts[:j], deferredSqlStmts[j+1:]...)
				break
			} else {
				log.Infof("failed retry of deferred stmt: %s\n%v", utils.GetSqlStmtToPrint(deferredSqlStmts[j].stmt), err)
				errString := fmt.Sprintf("/*\n%s\nFile :%s\n*/\n", err.Error(), deferredSqlStmts[j].fileName)
				failedSqlStmtInIthIteration = append(failedSqlStmtInIthIteration, errString+deferredSqlStmts[j].formattedStmt)
				err = conn.Close(context.Background())
				if err != nil {
					log.Warnf("error while closing the connection due to failed deferred stmt: %v", err)
				}
				conn = newTargetConn()
			}
		}

		afterDeferredSqlCount := len(deferredSqlStmts)
		if afterDeferredSqlCount == 0 {
			log.Infof("all of the deferred statements executed successfully in the %d iteration", i)
		} else if beforeDeferredSqlCount == afterDeferredSqlCount {
			// no need for further iterations since the deferred list will remain same
			log.Infof("none of the deferred statements executed successfully in the %d iteration", i)
			finalFailedDeferredStmts = failedSqlStmtInIthIteration
			break
		}
	}
	finalFailedSqlStmts = append(finalFailedSqlStmts, finalFailedDeferredStmts...)
}

func applySchemaObjectFilterFlags(importObjectOrderList []string) []string {
	var finalImportObjectList []string
	excludeObjectList := utils.CsvStringToSlice(tconf.ExcludeImportObjects)
	for i, item := range excludeObjectList {
		excludeObjectList[i] = strings.ToUpper(item)
	}
	if tconf.ImportObjects != "" {
		includeObjectList := utils.CsvStringToSlice(tconf.ImportObjects)
		for i, item := range includeObjectList {
			includeObjectList[i] = strings.ToUpper(item)
		}
		if importObjectsInStraightOrder {
			// Import the objects in the same order as when listed by the user.
			for _, listedObject := range includeObjectList {
				if slices.Contains(importObjectOrderList, listedObject) {
					finalImportObjectList = append(finalImportObjectList, listedObject)
				}
			}
		} else {
			// Import the objects in the default order.
			for _, supportedObject := range importObjectOrderList {
				if slices.Contains(includeObjectList, supportedObject) {
					finalImportObjectList = append(finalImportObjectList, supportedObject)
				}
			}
		}
	} else {
		finalImportObjectList = utils.SetDifference(importObjectOrderList, excludeObjectList)
	}
	if sourceDBType == "postgresql" && !slices.Contains(finalImportObjectList, "SCHEMA") && !bool(flagPostSnapshotImport) { // Schema should be migrated by default.
		finalImportObjectList = append([]string{"SCHEMA"}, finalImportObjectList...)
	}

	if !flagPostSnapshotImport {
		finalImportObjectList = append(finalImportObjectList, []string{"UNIQUE INDEX"}...)
	}
	return finalImportObjectList
}

func getInvalidIndexes(conn **pgx.Conn) (map[string]bool, error) {
	var result = make(map[string]bool)
	// NOTE: this shouldn't fetch any predefined indexes of pg_catalog schema (assuming they can't be invalid) or indexes of other successful migrations
	query := "SELECT indexrelid::regclass FROM pg_index WHERE indisvalid = false"

	rows, err := (*conn).Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("querying invalid indexes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var fullyQualifiedIndexName string
		err := rows.Scan(&fullyQualifiedIndexName)
		if err != nil {
			return nil, fmt.Errorf("scanning row for invalid index name: %w", err)
		}
		// if schema is not provided by catalog table, then it is public schema
		if !strings.Contains(fullyQualifiedIndexName, ".") {
			fullyQualifiedIndexName = fmt.Sprintf("public.%s", fullyQualifiedIndexName)
		}
		result[fullyQualifiedIndexName] = true
	}
	return result, nil
}

// TODO: need automation tests for this, covering cases like schema(public vs non-public) or case sensitive names
func beforeIndexCreation(sqlInfo sqlInfo, conn **pgx.Conn, objType string) error {
	if !strings.Contains(strings.ToUpper(sqlInfo.stmt), "CREATE INDEX") {
		return nil
	}

	fullyQualifiedObjName, err := getIndexName(sqlInfo.stmt, sqlInfo.objName)
	if err != nil {
		return fmt.Errorf("extract qualified index name from DDL [%v]: %w", sqlInfo.stmt, err)
	}
	if invalidTargetIndexesCache == nil {
		invalidTargetIndexesCache, err = getInvalidIndexes(conn)
		if err != nil {
			return fmt.Errorf("failed to fetch invalid indexes: %w", err)
		}
	}

	// check index valid or not
	if invalidTargetIndexesCache[fullyQualifiedObjName] {
		log.Infof("index %q already exists but in invalid state, dropping it", fullyQualifiedObjName)
		err = dropIdx(*conn, fullyQualifiedObjName)
		if err != nil {
			return fmt.Errorf("drop invalid index %q: %w", fullyQualifiedObjName, err)
		}
	}

	// print the index name as index creation takes time and user can see the progress
	color.Yellow("creating index %s ...", fullyQualifiedObjName)
	return nil
}

func dropIdx(conn *pgx.Conn, idxName string) error {
	dropIdxQuery := fmt.Sprintf("DROP INDEX IF EXISTS %s", idxName)
	log.Infof("Dropping index: %q", dropIdxQuery)
	_, err := conn.Exec(context.Background(), dropIdxQuery)
	if err != nil {
		return fmt.Errorf("failed to drop index %q: %w", idxName, err)
	}
	return nil
}

func newTargetConn() *pgx.Conn {
	// save notice in global variable
	noticeHandler := func(conn *pgconn.PgConn, n *pgconn.Notice) {
		// ALTER TABLE .. ADD PRIMARY KEY throws the following notice in YugabyteDB.
		// unlogged=# ALTER TABLE ONLY public.ul     ADD CONSTRAINT ul_pkey PRIMARY KEY (id);
		// NOTICE:  table rewrite may lead to inconsistencies
		// DETAIL:  Concurrent DMLs may not be reflected in the new table.
		// HINT:  See https://github.com/yugabyte/yugabyte-db/issues/19860. Set 'ysql_suppress_unsafe_alter_notice' yb-tserver gflag to true to suppress this notice.

		// We ignore this notice because:
		// 1. This is an empty table at the time at which we are importing the schema
		//    and there is no concurrent DMLs
		// 2. This would unnecessarily clutter the output with NOTICES for every table,
		//    and scare the user
		noticesToIgnore := []string{
			"table rewrite may lead to inconsistencies",
		}

		if n != nil {
			if lo.Contains(noticesToIgnore, n.Message) {
				notice = nil
				return
			}
		}
		notice = n
	}
	errExit := func(err error) {
		if err != nil {
			utils.WaitChannel <- 1
			<-utils.WaitChannel
			utils.ErrExit("connect to target db: %s", err)
		}
	}

	conf, err := pgx.ParseConfig(tconf.GetConnectionUri())
	errExit(err)
	conf.OnNotice = noticeHandler

	conn, err := pgx.ConnectConfig(context.Background(), conf)
	errExit(err)

	setTargetSchema(conn)

	if sourceDBType == ORACLE && enableOrafce {
		setOrafceSearchPath(conn)
	}

	return conn
}

func getNoticeMessage(n *pgconn.Notice) string {
	if n == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", n.Severity, n.Message)
}

// TODO: Eventually get rid of this function in favour of TargetYugabyteDB.setTargetSchema().
func setTargetSchema(conn *pgx.Conn) {
	if sourceDBType == POSTGRESQL || tconf.Schema == YUGABYTEDB_DEFAULT_SCHEMA {
		// For PG, schema name is already included in the object name.
		// No need to set schema if importing in the default schema.
		return
	}
	checkSchemaExistsQuery := fmt.Sprintf("SELECT count(schema_name) FROM information_schema.schemata WHERE schema_name = '%s'", tconf.Schema)
	var cntSchemaName int

	if err := conn.QueryRow(context.Background(), checkSchemaExistsQuery).Scan(&cntSchemaName); err != nil {
		utils.ErrExit("run query %q on target %q to check schema exists: %s", checkSchemaExistsQuery, tconf.Host, err)
	} else if cntSchemaName == 0 {
		utils.ErrExit("schema '%s' does not exist in target", tconf.Schema)
	}

	setSchemaQuery := fmt.Sprintf("SET SCHEMA '%s'", tconf.Schema)
	_, err := conn.Exec(context.Background(), setSchemaQuery)
	if err != nil {
		utils.ErrExit("run query %q on target %q: %s", setSchemaQuery, tconf.Host, err)
	}
}

func setOrafceSearchPath(conn *pgx.Conn) {
	// append oracle schema in the search_path for orafce
	updateSearchPath := `SELECT set_config('search_path', current_setting('search_path') || ', oracle', false)`
	_, err := conn.Exec(context.Background(), updateSearchPath)
	if err != nil {
		utils.ErrExit("unable to update search_path for orafce extension: %v", err)
	}
}

func execStmtAndGetNotice(conn *pgx.Conn, stmt string) (*pgconn.Notice, error) {
	notice = nil // reset notice.
	_, err := conn.Exec(context.Background(), stmt)
	return notice, err
}
