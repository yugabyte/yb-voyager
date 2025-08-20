//go:build integration_voyager_command

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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestShardingRecommendations(t *testing.T) {
	sqlInfo_mview1 := sqlInfo{
		objName:       "m1",
		stmt:          "CREATE MATERIALIZED VIEW m1 AS SELECT * FROM t1 WHERE a = 3",
		formattedStmt: "CREATE MATERIALIZED VIEW m1 AS SELECT * FROM t1 WHERE a = 3",
		fileName:      "",
	}
	sqlInfo_mview2 := sqlInfo{
		objName:       "m1",
		stmt:          "CREATE MATERIALIZED VIEW m1 AS SELECT * FROM t1 WHERE a = 3 with no data;",
		formattedStmt: "CREATE MATERIALIZED VIEW m1 AS SELECT * FROM t1 WHERE a = 3 with no data;",
		fileName:      "",
	}
	sqlInfo_mview3 := sqlInfo{
		objName:       "m1",
		stmt:          "CREATE MATERIALIZED VIEW m1 WITH (fillfactor=70) AS SELECT * FROM t1 WHERE a = 3 with no data",
		formattedStmt: "CREATE MATERIALIZED VIEW m1 WITH (fillfactor=70) AS SELECT * FROM t1 WHERE a = 3 with no data",
		fileName:      "",
	}
	source.DBType = POSTGRESQL
	modifiedSqlStmt, match, _, _, _ := applyShardingRecommendationIfMatching(&sqlInfo_mview1, []string{"m1"}, MVIEW)
	assert.Equal(t, strings.ToLower(modifiedSqlStmt),
		strings.ToLower("create materialized view m1 with (colocation=false) as select * from t1 where a = 3;"))
	assert.Equal(t, match, true)

	modifiedSqlStmt, match, _, _, _ = applyShardingRecommendationIfMatching(&sqlInfo_mview2, []string{"m1"}, MVIEW)
	assert.Equal(t, strings.ToLower(modifiedSqlStmt),
		strings.ToLower("create materialized view m1 with (colocation=false) as select * from t1 where a = 3 with no data;"))
	assert.Equal(t, match, true)

	modifiedSqlStmt, match, _, _, _ = applyShardingRecommendationIfMatching(&sqlInfo_mview2, []string{"m1_notfound"}, MVIEW)
	assert.Equal(t, modifiedSqlStmt, sqlInfo_mview2.stmt)
	assert.Equal(t, match, false)

	modifiedSqlStmt, match, _, _, _ = applyShardingRecommendationIfMatching(&sqlInfo_mview3, []string{"m1"}, MVIEW)
	assert.Equal(t, strings.ToLower(modifiedSqlStmt),
		strings.ToLower("create materialized view m1 with (fillfactor=70, colocation=false) "+
			"as select * from t1 where a = 3 with no data;"))
	assert.Equal(t, match, true)

	sqlInfo_table1 := sqlInfo{
		objName:       "m1",
		stmt:          "create table a (a int, b int)",
		formattedStmt: "create table a (a int, b int)",
		fileName:      "",
	}
	sqlInfo_table2 := sqlInfo{
		objName:       "m1",
		stmt:          "create table a (a int, b int) WITH (fillfactor=70);",
		formattedStmt: "create table a (a int, b int) WITH (fillfactor=70);",
		fileName:      "",
	}
	sqlInfo_table3 := sqlInfo{
		objName:       "m1",
		stmt:          "alter table a add col text;",
		formattedStmt: "alter table a add col text;",
		fileName:      "",
	}
	modifiedTableStmt, matchTable, _, _, _ := applyShardingRecommendationIfMatching(&sqlInfo_table1, []string{"a"}, TABLE)
	assert.Equal(t, strings.ToLower(modifiedTableStmt),
		strings.ToLower("create table a (a int, b int) WITH (colocation=false);"))
	assert.Equal(t, matchTable, true)

	modifiedTableStmt, matchTable, _, _, _ = applyShardingRecommendationIfMatching(&sqlInfo_table2, []string{"a"}, TABLE)
	assert.Equal(t, strings.ToLower(modifiedTableStmt),
		strings.ToLower("create table a (a int, b int) WITH (fillfactor=70, colocation=false);"))
	assert.Equal(t, matchTable, true)

	modifiedSqlStmt, matchTable, _, _, _ = applyShardingRecommendationIfMatching(&sqlInfo_table2, []string{"m1_notfound"}, TABLE)
	assert.Equal(t, modifiedSqlStmt, sqlInfo_table2.stmt)
	assert.Equal(t, matchTable, false)

	modifiedTableStmt, matchTable, _, _, _ = applyShardingRecommendationIfMatching(&sqlInfo_table3, []string{"a"}, TABLE)
	assert.Equal(t, strings.ToLower(modifiedTableStmt),
		strings.ToLower(sqlInfo_table3.stmt))
	assert.Equal(t, matchTable, false)
}

// Test export schema after running assessment internally - case when assess-migration is run before export-schema
// Expectation: export-schema should export with no internal assess-migration cmd invokation
func TestExportSchemaRunningAssessmentInternally_ExportAfterAssessCmd(t *testing.T) {
	// create temp export dir and setting global exportDir variable
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// setting up source test container and source params for assessment
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(context.Background())
	if err != nil {
		utils.ErrExit("Failed to start postgres container: %v", err)
	}

	// create table and initial data in it
	postgresContainer.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		`CREATE TABLE test_schema.test_data (
		id SERIAL PRIMARY KEY,
		value TEXT
	);`,
		`INSERT INTO test_schema.test_data (value)
	SELECT md5(random()::text) FROM generate_series(1, 100000);`)
	if err != nil {
		t.Errorf("Failed to create test table: %v", err)
	}
	defer postgresContainer.ExecuteSqls(`
	DROP SCHEMA test_schema CASCADE;`)

	// running the command
	_, err = testutils.RunVoyagerCommand(postgresContainer, "assess-migration", []string{
		"--iops-capture-interval", "0",
		"--source-db-schema", "test_schema",
		"--export-dir", exportDir,
		"--yes",
	}, nil, false)
	if err != nil {
		t.Errorf("Failed to run assess-migration command: %v", err)
	}

	// verify the MSR.MigrationAssessmentDone flag is set to true
	metaDB = initMetaDB(exportDir)
	res, err := IsMigrationAssessmentDoneDirectly(metaDB)
	if err != nil {
		t.Errorf("Failed to check MigrationAssessmentDoneViaExportSchema flag: %v", err)
	}
	assert.True(t, res, "Expected MigrationAssessmentDone flag to be true")

	// verify the MSR.MigrationAssessmentDoneViaExportSchema flag is set to false
	metaDB = initMetaDB(exportDir)
	res, err = IsMigrationAssessmentDoneViaExportSchema()
	if err != nil {
		t.Errorf("Failed to check MigrationAssessmentDoneViaExportSchema flag: %v", err)
	}
	assert.False(t, res, "Expected MigrationAssessmentDoneViaExportSchema flag to be false")

	_, err = testutils.RunVoyagerCommand(postgresContainer, "export schema", []string{
		"--source-db-schema", "test_schema",
		"--export-dir", exportDir,
		"--yes",
	}, nil, false)
	if err != nil {
		t.Errorf("Failed to run export schema command: %v", err)
	}

	// doing the same check in MSR to ensure nothing has changed after export schema
	metaDB = initMetaDB(exportDir)
	res, err = IsMigrationAssessmentDoneDirectly(metaDB)
	if err != nil {
		t.Errorf("Failed to check MigrationAssessmentDoneViaExportSchema flag: %v", err)
	}
	assert.True(t, res, "Expected MigrationAssessmentDone flag to be true")

	metaDB = initMetaDB(exportDir)
	res, err = IsMigrationAssessmentDoneViaExportSchema()
	if err != nil {
		t.Errorf("Failed to check MigrationAssessmentDoneViaExportSchema flag: %v", err)
	}
	assert.False(t, res, "Expected MigrationAssessmentDoneViaExportSchema flag to be false")

	// check if report from assessment
	reportFilePath := filepath.Join(exportDir, "assessment", "reports", "migration_assessment_report.json")
	if !utils.FileOrFolderExists(reportFilePath) {
		t.Errorf("Expected assessment report file does not exist: %s", reportFilePath)
	}

	// check table.sql from export schema
	tableSqlFilePath := filepath.Join(exportDir, "schema", "tables", "table.sql")
	if !utils.FileOrFolderExists(tableSqlFilePath) {
		t.Errorf("Expected table.sql file does not exist: %s", tableSqlFilePath)
	}
}

func TestExportSchemaRunningAssessmentInternally_ExportSchemaThenAssessCmd(t *testing.T) {
	// create temp export dir and setting global exportDir variable
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// setting up source test container and source params for assessment
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(context.Background())
	if err != nil {
		utils.ErrExit("Failed to start postgres container: %v", err)
	}

	// create table and initial data in it
	postgresContainer.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		`CREATE TABLE test_schema.test_data (
		id SERIAL PRIMARY KEY,
		value TEXT
	);`,
		`INSERT INTO test_schema.test_data (value)
	SELECT md5(random()::text) FROM generate_series(1, 100000);`)
	if err != nil {
		t.Errorf("Failed to create test table: %v", err)
	}
	defer postgresContainer.ExecuteSqls(`
	DROP SCHEMA test_schema CASCADE;`)

	_, err = testutils.RunVoyagerCommand(postgresContainer, "export schema", []string{
		"--source-db-schema", "test_schema",
		"--export-dir", exportDir,
		"--yes",
	}, nil, false)
	if err != nil {
		t.Errorf("Failed to run export schema command: %v", err)
	}

	// verify the MSR.MigrationAssessmentDoneViaExportSchema flag is set to true
	metaDB = initMetaDB(exportDir)
	res, err := IsMigrationAssessmentDoneViaExportSchema()
	if err != nil {
		t.Errorf("Failed to check MigrationAssessmentDoneViaExportSchema flag: %v", err)
	}
	assert.True(t, res, "Expected MigrationAssessmentDoneViaExportSchema flag to be true")

	// verify if table.sql exists or not
	tableSqlFilePath := filepath.Join(exportDir, "schema", "tables", "table.sql")
	if !utils.FileOrFolderExists(tableSqlFilePath) {
		t.Errorf("Expected table.sql file does not exist: %s", tableSqlFilePath)
	}

	_, err = testutils.RunVoyagerCommand(postgresContainer, "assess-migration", []string{
		"--source-db-schema", "test_schema",
		"--iops-capture-interval", "0",
		"--export-dir", exportDir,
		"--start-clean", "true",
		"--yes",
	}, nil, false)
	if err != nil {
		t.Errorf("Failed to run assess-migration command: %v", err)
	}

	reportFilePath := filepath.Join(exportDir, "assessment", "reports", "migration_assessment_report.json")
	if !utils.FileOrFolderExists(reportFilePath) {
		t.Errorf("Expected assessment report file does not exist: %s", reportFilePath)
	}

	// verify the MSR.MigrationAssessmentDone flag is set to true
	metaDB = initMetaDB(exportDir)
	res, err = IsMigrationAssessmentDoneDirectly(metaDB)
	if err != nil {
		t.Errorf("Failed to check MigrationAssessmentDoneViaExportSchema flag: %v", err)
	}
	assert.True(t, res, "Expected MigrationAssessmentDone flag to be true")

	// verify the MSR.MigrationAssessmentDoneViaExportSchema flag is set to false
	metaDB = initMetaDB(exportDir)
	res, err = IsMigrationAssessmentDoneViaExportSchema()
	if err != nil {
		t.Errorf("Failed to check MigrationAssessmentDoneViaExportSchema flag: %v", err)
	}
	assert.False(t, res, "Expected MigrationAssessmentDoneViaExportSchema flag to be false")
}

// Test: export schema after running assessment internally - case when --assess-schema-before-export flag is set to false
// Expectation: export-schema should export with no internal assess-migration cmd invokation
func TestExportSchemaRunningAssessmentInternally_DisableFlag(t *testing.T) {
	// create temp export dir and setting global exportDir variable
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// setting up source test container and source params for assessment
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(context.Background())
	if err != nil {
		utils.ErrExit("Failed to start postgres container: %v", err)
	}

	// create table and initial data in it
	postgresContainer.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		`CREATE TABLE test_schema.test_data (
		id SERIAL PRIMARY KEY,
		value TEXT
	);`,
		`INSERT INTO test_schema.test_data (value)
	SELECT md5(random()::text) FROM generate_series(1, 100000);`)
	if err != nil {
		t.Errorf("Failed to create test table: %v", err)
	}
	defer postgresContainer.ExecuteSqls(`
	DROP SCHEMA test_schema CASCADE;`)

	_, err = testutils.RunVoyagerCommand(postgresContainer, "export schema", []string{
		"--assess-schema-before-export", "false",
		"--source-db-schema", "test_schema",
		"--export-dir", exportDir,
		"--yes",
	}, nil, false)
	if err != nil {
		t.Errorf("Failed to run export schema command: %v", err)
	}

	res, err := IsMigrationAssessmentDoneViaExportSchema()
	if err != nil {
		t.Errorf("Failed to check MigrationAssessmentDoneViaExportSchema flag: %v", err)
	}
	assert.False(t, res, "Expected MigrationAssessmentDoneViaExportSchema flag to be false")

	reportFilePath := filepath.Join(exportDir, "assessment", "reports", "migration_assessment_report.json")
	if utils.FileOrFolderExists(reportFilePath) {
		t.Errorf("Expected assessment report file does exist: %s", reportFilePath)
	}
}

// Add test for Schema optimization report json format in the export schema command
func TestExportSchemaSchemaOptimizationReportAndRedundantIndexAutofix(t *testing.T) {
	// create temp export dir and setting global exportDir variable
	tempExportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(tempExportDir)

	// setting up source test container and source params for assessment
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(context.Background())
	if err != nil {
		utils.ErrExit("Failed to start postgres container: %v", err)
	}
	defer postgresContainer.Stop(context.Background())

	yugabyteContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err = yugabyteContainer.Start(context.Background())
	if err != nil {
		utils.ErrExit("Failed to start yugabyte container: %v", err)
	}
	defer yugabyteContainer.Stop(context.Background())

	// create table and initial data in it
	postgresContainer.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		`CREATE TABLE test_schema.test_data (
			id SERIAL PRIMARY KEY,
			value TEXT,
			value_2 TEXT,
			id1 int
		);`,
		`CREATE INDEX idx_test_data_value ON test_schema.test_data (value);`,
		`CREATE INDEX idx_test_data_value_2 ON test_schema.test_data (value_2);`,
		`CREATE INDEX idx_test_data_value_3 ON test_schema.test_data (value, value_2);`,
		`CREATE INDEX idx_test_data_id1 ON test_schema.test_data (value_2, id1);`,
	)
	if err != nil {
		t.Errorf("Failed to create test table: %v", err)
	}
	defer postgresContainer.ExecuteSqls(`
		DROP SCHEMA test_schema CASCADE;`)

	_, err = testutils.RunVoyagerCommand(postgresContainer, "export schema", []string{
		"--source-db-schema", "test_schema",
		"--export-dir", tempExportDir,
		"--yes",
	}, nil, false)
	if err != nil {
		t.Errorf("Failed to run export schema command: %v", err)
	}

	// check if schema optimization report json file exists
	schemaOptimizationReportFilePath := filepath.Join(exportDir, "reports", "schema_optimization_report.json")
	if !utils.FileOrFolderExists(schemaOptimizationReportFilePath) {
		t.Errorf("Expected schema optimization report file does not exist: %s", schemaOptimizationReportFilePath)
	}

	jsonFile := jsonfile.NewJsonFile[SchemaOptimizationReport](schemaOptimizationReportFilePath)
	schemaOptimizationReport, err := jsonFile.Read()
	if err != nil {
		t.Errorf("Failed to read schema optimization report file: %v", err)
	}
	assert.NotNil(t, schemaOptimizationReport)
	assert.NotNil(t, schemaOptimizationReport.RedundantIndexChange)
	assert.Nil(t, schemaOptimizationReport.TableShardingRecommendation)
	assert.Nil(t, schemaOptimizationReport.MviewShardingRecommendation)
	assert.Equal(t, 1, len(schemaOptimizationReport.RedundantIndexChange.TableToRemovedIndexesMap))
	assert.Equal(t, 2, len(schemaOptimizationReport.RedundantIndexChange.TableToRemovedIndexesMap["test_schema.test_data"]))

	_, err = testutils.RunVoyagerCommand(yugabyteContainer, "import schema", []string{
		"--export-dir", tempExportDir,
	}, func() {
		time.Sleep(10 * time.Second)
	}, true)
	if err != nil {
		t.Errorf("Failed to run import schema command: %v", err)
	}
	//GEt all indexes from yugabyte container
	rows, err := yugabyteContainer.Query("select indexname from pg_indexes where tablename = 'test_data' and schemaname = 'test_schema' ;")
	if err != nil {
		t.Errorf("Failed to get indexes: %v", err)
	}
	indexes := []string{}
	for rows.Next() {
		var indexName string
		err = rows.Scan(&indexName)
		if err != nil {
			t.Errorf("Failed to scan index: %v", err)
		}
		indexes = append(indexes, indexName)
	}

	assert.Equal(t, 3, len(indexes))
	expectedIndexes := []string{
		"idx_test_data_value_3",
		"idx_test_data_id1",
		"test_data_pkey", //PK index
	}
	testutils.AssertEqualStringSlices(t, expectedIndexes, indexes)
}

func TestExportSchemaSchemaOptimizationReportAndRangeShardedAutofix(t *testing.T) {
	// create temp export dir and setting global exportDir variable
	tempExportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(tempExportDir)

	// setting up source test container and source params for assessment
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(context.Background())
	if err != nil {
		utils.ErrExit("Failed to start postgres container: %v", err)
	}
	defer postgresContainer.Stop(context.Background())

	yugabyteContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err = yugabyteContainer.Start(context.Background())
	if err != nil {
		utils.ErrExit("Failed to start yugabyte container: %v", err)
	}
	defer yugabyteContainer.Stop(context.Background())

	// create table and initial data in it
	postgresContainer.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		`CREATE TABLE test_schema.test_data (
				id SERIAL PRIMARY KEY,
				value TEXT,
				value_2 TEXT,
				id1 int
			);`,
		`CREATE INDEX idx_test_data_value_3 ON test_schema.test_data (value, value_2);`,
		`CREATE INDEX idx_test_data_id1 ON test_schema.test_data (value_2 DESC, id1);`,
	)
	if err != nil {
		t.Errorf("Failed to create test table: %v", err)
	}
	defer postgresContainer.ExecuteSqls(`
			DROP SCHEMA test_schema CASCADE;`)

	_, err = testutils.RunVoyagerCommand(postgresContainer, "export schema", []string{
		"--source-db-schema", "test_schema",
		"--export-dir", tempExportDir,
		"--yes",
	}, nil, false)
	if err != nil {
		t.Errorf("Failed to run export schema command: %v", err)
	}

	// check if schema optimization report json file exists
	schemaOptimizationReportFilePath := filepath.Join(exportDir, "reports", "schema_optimization_report.json")
	if !utils.FileOrFolderExists(schemaOptimizationReportFilePath) {
		t.Errorf("Expected schema optimization report file does not exist: %s", schemaOptimizationReportFilePath)
	}

	jsonFile := jsonfile.NewJsonFile[SchemaOptimizationReport](schemaOptimizationReportFilePath)
	schemaOptimizationReport, err := jsonFile.Read()
	if err != nil {
		t.Errorf("Failed to read schema optimization report file: %v", err)
	}
	assert.NotNil(t, schemaOptimizationReport)
	assert.Nil(t, schemaOptimizationReport.RedundantIndexChange)
	assert.Nil(t, schemaOptimizationReport.TableShardingRecommendation)
	assert.Nil(t, schemaOptimizationReport.MviewShardingRecommendation)
	assert.NotNil(t, schemaOptimizationReport.TableShard)

	_, err = testutils.RunVoyagerCommand(yugabyteContainer, "import schema", []string{
		"--export-dir", tempExportDir,
	}, func() {
		time.Sleep(10 * time.Second)
	}, true)
	if err != nil {
		t.Errorf("Failed to run import schema command: %v", err)
	}
	//GEt all indexes from yugabyte container
	rows, err := yugabyteContainer.Query("select indexname from pg_indexes where tablename = 'test_data' and schemaname = 'test_schema' ;")
	if err != nil {
		t.Errorf("Failed to get indexes: %v", err)
	}
	indexes := []string{}
	for rows.Next() {
		var indexName string
		err = rows.Scan(&indexName)
		if err != nil {
			t.Errorf("Failed to scan index: %v", err)
		}
		indexes = append(indexes, indexName)
	}

	assert.Equal(t, 3, len(indexes))
	expectedIndexes := []string{
		"idx_test_data_value_3",
		"idx_test_data_id1",
		"test_data_pkey", //PK index
	}
	testutils.AssertEqualStringSlices(t, expectedIndexes, indexes)
}
