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
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryissue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestMain(m *testing.M) {
	// set logging level to WARN
	// to avoid info level logs flooding the test output
	log.SetLevel(log.WarnLevel)

	exitCode := m.Run()

	// cleaning up all the running containers
	testcontainers.TerminateAllContainers()

	os.Exit(exitCode)
}

func Test_AssessMigration(t *testing.T) {
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
	SELECT md5(random()::text) FROM generate_series(1, 100000);`,
		`CREATE TABLE test_schema.test_data2 (
			id SERIAL,
			value TEXT
		);`,
		`INSERT INTO test_schema.test_data2 (value)
		SELECT md5(random()::text) FROM generate_series(1, 100000);`)

	defer postgresContainer.ExecuteSqls(
		`DROP SCHEMA test_schema CASCADE;`)

	doDuringCmd := func() {
		log.Infof("Performing sequential reads on the table")
		// sequential reads on the table with delay to ensure IOPS capture window overlaps
		postgresContainer.ExecuteSqls(`
DO $$ 
DECLARE i INT;
BEGIN
	FOR i IN 1..200 LOOP
		PERFORM COUNT(*) FROM test_schema.test_data;
		PERFORM COUNT(*) FROM test_schema.test_data2;
		PERFORM pg_sleep(0.05); -- Small delay to spread queries over time
	END LOOP;
END $$;`)
		if err != nil {
			t.Errorf("Failed to perform sequential reads on the table: %v", err)
		}
	}

	expectedColocatedTables := []string(nil)
	expectedSizingAssessmentReport := migassessment.SizingAssessmentReport{
		SizingRecommendation: migassessment.SizingRecommendation{
			ColocatedTables:                 nil,
			ColocatedReasoning:              "Recommended instance type with 4 vCPU and 16 GiB memory could fit 2 objects (2 tables/materialized views and 0 explicit/implicit indexes) with 13.03 MB size and throughput requirement of 20 reads/sec and 0 writes/sec as sharded. Non leaf partition tables/indexes and unsupported tables/indexes were not considered.",
			ShardedTables:                   []string{"test_schema.test_data", "test_schema.test_data2"},
			NumNodes:                        3,
			VCPUsPerInstance:                4,
			MemoryPerInstance:               16,
			OptimalSelectConnectionsPerNode: 10,
			OptimalInsertConnectionsPerNode: 12,
			EstimatedTimeInMinForImport:     1,
			EstimatedTimeInMinForImportWithoutRedundantIndexes: 1,
		},
		FailureReasoning: "",
	}
	expectedTableIndexStats := []migassessment.TableIndexStats{
		{
			SchemaName:      "test_schema",
			ObjectName:      "test_data",
			RowCount:        int64Ptr(100000),
			ColumnCount:     int64Ptr(2),
			ReadsPerSecond:  int64Ptr(10),
			WritesPerSecond: int64Ptr(0),
			IsIndex:         false,
			ObjectType:      "table",
			ParentTableName: nil,
			SizeInBytes:     int64Ptr(6832128),
		},
		{
			SchemaName:      "test_schema",
			ObjectName:      "test_data2",
			RowCount:        int64Ptr(100000),
			ColumnCount:     int64Ptr(2),
			ReadsPerSecond:  int64Ptr(10),
			WritesPerSecond: int64Ptr(0),
			IsIndex:         false,
			ObjectType:      "table",
			ParentTableName: nil,
			SizeInBytes:     int64Ptr(6832128),
		},
		{
			SchemaName:      "test_schema",
			ObjectName:      "test_data_pkey",
			RowCount:        nil,
			ColumnCount:     int64Ptr(1),
			ReadsPerSecond:  int64Ptr(0),
			WritesPerSecond: int64Ptr(0),
			IsIndex:         true,
			ObjectType:      "primary key",
			ParentTableName: stringPtr("test_schema.test_data"),
			SizeInBytes:     int64Ptr(2260992),
		},
	}

	// running the command
	_, err = testutils.RunVoyagerCommand(postgresContainer, "assess-migration", []string{
		"--source-db-schema", "test_schema", // overriding the flag value
		"--iops-capture-interval", "20",     // Increased for better test reliability
		"--export-dir", exportDir,
		"--yes",
	}, doDuringCmd, false)
	if err != nil {
		t.Errorf("Failed to run assess-migration command: %v", err)
	}

	assessmentReportPath := filepath.Join(exportDir, "assessment", "reports", fmt.Sprintf("%s.json", ASSESSMENT_FILE_NAME))
	report, err := ParseJSONToAssessmentReport(assessmentReportPath)
	if err != nil {
		t.Errorf("failed to parse json report file %q: %v", assessmentReportPath, err)
	}

	colocatedTables, err := report.GetColocatedTablesRecommendation()
	if err != nil {
		t.Errorf("failed to get colocated tables recommendation: %v", err)
	}

	log.Infof("Colocated tables: %v\n", colocatedTables)
	log.Infof("Sizing recommendations: %+v\n", report.Sizing)
	log.Infof("TableIndexStats: %s\n", spew.Sdump(report.TableIndexStats))

	// assert all these three info
	testutils.AssertEqualStringSlices(t, expectedColocatedTables, colocatedTables)
	assert.Equal(t, expectedSizingAssessmentReport, *report.Sizing)
	assert.Equal(t, expectedTableIndexStats, *report.TableIndexStats)
}

func int64Ptr(i int64) *int64 {
	return &i
}

func stringPtr(s string) *string {
	return &s
}

func Test_AssessMigration_UsageCategory(t *testing.T) {
	// create temp export dir and setting global exportDir variable
	exportDir = testutils.CreateTempExportDir()
	// defer testutils.RemoveTempExportDir(exportDir)

	// setting up source test container and source params for assessment
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(context.Background())
	if err != nil {
		utils.ErrExit("Failed to start postgres container: %v", err)
	}
	/*
		test_data: scans 50013, writes 100000 ( PK hotspot, index on particular value) FREQUENT
		idx_test_data_value ON test_schema.test_data
		test_data2: scans 60000, writes 0 ( FK with  datatype mismatch, Missing index on foreign key columns) FREQUENT
		test_partitions_l: scans 49000, writes 25000 ( Index hotspot, ) MODERATE
		test_partitions_s: scans 1, writes 25000 ( Index hotspot) MODERATE
		test_partitions_b: scans 1, writes 10 ( Index hotspot) RARE
		test_partitions:  MODERATE combined usage of all the partitions
		test_unused: scans 3, writes 0 ( PK if UNIQUE NOT NULL column present) UNUSED
	*/
	postgresContainer.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		//PK Hotspot with timestamp type
		`CREATE TABLE test_schema.test_data (
			id bigint UNIQUE,
			created_at timestamp with time zone PRIMARY KEY,
			value TEXT
		);`,
		//index on a particular value
		`CREATE INDEX idx_test_data_value ON test_schema.test_data (value);`,
		`INSERT INTO test_schema.test_data
			SELECT i, 
				now() + (i || ' days')::interval, 
				'a particular value' 
			FROM generate_series(1, 100000) AS i;`,
		//table with no indexes
		//FK with  datatype mismatch
		//Missing index on foreign key columns
		`CREATE TABLE test_schema.test_data2 (
			id int,
			val text,
			FOREIGN KEY (id) REFERENCES test_schema.test_data (id)
		);`,
		//PK not present but UNIQUE NOT NULL column present
		//partition table
		//Index hotspot
		`CREATE TABLE test_schema.test_partitions(
			id int,
			region text,
			created_at date,
			FOREIGN KEY (id) REFERENCES test_schema.test_data (id)
		) PARTITION BY LIST (region);`,
		`CREATE TABLE test_schema.test_partitions_l PARTITION OF test_schema.test_partitions FOR VALUES IN ('London');`,
		`CREATE TABLE test_schema.test_partitions_s PARTITION OF test_schema.test_partitions FOR VALUES IN ('Sydney');`,
		`CREATE TABLE test_schema.test_partitions_b PARTITION OF test_schema.test_partitions FOR VALUES IN ('Boston');`,
		`INSERT INTO test_schema.test_partitions (id, region, created_at) 
		SELECT i, 
			CASE 
				WHEN i%2 =0 THEN 'London'
				ELSE 'Sydney'
			END,
			now() + (i || ' days')::interval FROM generate_series(1, 50000) as i;`,
		`INSERT INTO test_schema.test_partitions (id, region, created_at) 
		SELECT i, 'Boston', now() + (i || ' days')::interval  FROM generate_series(50001, 50010) as i;`,
		`CREATE INDEX idx_test_partitions_created_at ON test_schema.test_partitions (created_at);`,
		`CREATE TABLE test_schema.test_unused (id int UNIQUE NOT NULL,value text);`,
		`DO $$ 
DECLARE i INT;
BEGIN
	FOR i IN 1..3 LOOP
		PERFORM COUNT(*) FROM test_schema.test_unused;
	END LOOP;
END $$;`,
		`DO $$ 
DECLARE i INT;
BEGIN
	FOR i IN 1..60000 LOOP
		PERFORM * FROM test_schema.test_data2 LIMIT 1;
	END LOOP;
END $$;`,
		`DO $$
DECLARE i INT;
BEGIN
	FOR i IN 1..48990 LOOP
		PERFORM COUNT(*) FROM test_schema.test_partitions where region='London' and created_at = CURRENT_DATE;
	END LOOP;
END $$;`,
		`ANALYZE;`)

	defer postgresContainer.ExecuteSqls(
		`DROP SCHEMA test_schema CASCADE;`)

	_, err = testutils.RunVoyagerCommand(postgresContainer, "assess-migration", []string{
		"--source-db-schema", "test_schema", // overriding the flag value
		"--iops-capture-interval", "10",
		"--export-dir", exportDir,
		"--yes",
	}, nil, false)

	testutils.FatalIfError(t, err, "Failed to run assess-migration command")

	assessmentReportPath := filepath.Join(exportDir, "assessment", "reports", fmt.Sprintf("%s.json", ASSESSMENT_FILE_NAME))
	report, err := ParseJSONToAssessmentReport(assessmentReportPath)
	if err != nil {
		t.Errorf("failed to parse json report file %q: %v", assessmentReportPath, err)
	}

	assert.NotNil(t, report)
	assert.Equal(t, 10, len(report.Issues))
	expectedObjectToUsageCategory := map[string]string{
		"test_schema.test_data":                                             "FREQUENT", //3 scan 100000 writes
		"test_schema.test_data2":                                            "FREQUENT", //6000 scans 0 writes
		"idx_test_data_value ON test_schema.test_data":                      "FREQUENT", //0 scans 100000 writes
		"test_partitions_l_created_at_idx ON test_schema.test_partitions_l": "FREQUENT", //4899 scans 25000 writes
		"test_schema.test_partitions_l":                                     "FREQUENT", //4900 scans 25000 writes
		"test_schema.test_partitions":                                       "FREQUENT", // combined usage of all the partitions
		"test_schema.test_partitions_s":                                     "MODERATE", //4597 scans 25000 writes
		"test_partitions_s_created_at_idx ON test_schema.test_partitions_s": "MODERATE", //0 scans 25000 writes
		"test_schema.test_partitions_b":                                     "RARE",     //1 scan 10 writes
		"test_partitions_b_created_at_idx ON test_schema.test_partitions_b": "RARE",     //0 scans 10 writes
		"test_schema.test_unused":                                           "UNUSED",   // 0 scans 0 writes
		"idx_test_partitions_created_at ON test_schema.test_partitions":     "UNUSED",   // 0 scans 0 writes
	}
	for _, issue := range report.Issues {
		assert.Equal(t, issue.Category, PERFORMANCE_OPTIMIZATIONS_CATEGORY)
		expectedUsageCategory, ok := expectedObjectToUsageCategory[issue.ObjectName]
		if !ok {
			t.Errorf("unexpected object name: %s", issue.ObjectName)
		}
		assert.Equal(t, expectedUsageCategory, issue.ObjectUsage, "expected usage category: %s, actual usage category: %s for object: %s", expectedUsageCategory, issue.ObjectUsage, issue.ObjectName)
	}
}

func Test_AssessMigration_RecommendedSQL_Datatypes(t *testing.T) {
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(context.Background())
	if err != nil {
		utils.ErrExit("Failed to start postgres container: %v", err)
	}

	yugabyteContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err = yugabyteContainer.Start(context.Background())
	if err != nil {
		utils.ErrExit("Failed to start yugabyte container: %v", err)
	}

	createSchemaAndTableSQL := `CREATE SCHEMA test_schema;
		CREATE TABLE test_schema.test_data_types (
			id SERIAL PRIMARY KEY,
			c_text TEXT NOT NULL,
			c_char CHARACTER(10) NOT NULL,
			c_varchar VARCHAR(20) NOT NULL,
			c_int INTEGER NOT NULL,
			c_int2 INT2 NOT NULL,
			c_int8 INT8 NOT NULL,
			c_numeric NUMERIC(10,2) NOT NULL,
			c_real REAL NOT NULL,
			c_float8 FLOAT8 NOT NULL,
			c_bool BOOLEAN NOT NULL,
			c_date DATE NOT NULL,
			c_timestamp TIMESTAMP NOT NULL,
			c_timestamptz TIMESTAMPTZ NOT NULL,
			c_uuid UUID NOT NULL,
			c_jsonb JSONB NOT NULL,
			c_bytea BYTEA NOT NULL,
			c_money MONEY NOT NULL,
			c_text_array TEXT[] NOT NULL,
			c_time TIME NOT NULL,
			c_serial SERIAL NOT NULL,
			c_smallserial SMALLSERIAL NOT NULL,
			c_bigserial BIGSERIAL NOT NULL
		);`

	postgresContainer.ExecuteSqls(
		createSchemaAndTableSQL,
		`INSERT INTO test_schema.test_data_types (
			c_text, c_char, c_varchar, c_int, c_int2, c_int8, c_numeric, c_real, c_float8, c_bool, 
			c_date, c_timestamp, c_timestamptz, c_uuid, c_jsonb, c_bytea, c_money, c_text_array,
			c_time, c_serial, c_smallserial, c_bigserial
		)
		SELECT
			CASE WHEN i <= 80 THEN 'fallback_val' ELSE 'fallback_val_' || i END,
			CASE WHEN i <= 80 THEN 'ABCDE'::CHARACTER(10) ELSE lpad(i::TEXT, 10, '0')::CHARACTER(10) END,
			CASE WHEN i <= 80 THEN 'active' ELSE 'inactive_' || i END,
			CASE WHEN i <= 80 THEN 7 ELSE i END,
			CASE WHEN i <= 80 THEN 2::INT2 ELSE (i % 100)::INT2 END,
			CASE WHEN i <= 80 THEN 9876543210::INT8 ELSE (9876543210 + i)::INT8 END,
			CASE WHEN i <= 80 THEN 9.99::NUMERIC ELSE (i::NUMERIC / 10) END,
			CASE WHEN i <= 80 THEN 0.25::REAL ELSE (i::REAL / 100) END,
			CASE WHEN i <= 80 THEN 0.125::FLOAT8 ELSE (i::FLOAT8 / 1000) END,
			CASE WHEN i <= 80 THEN true ELSE false END,
			CASE WHEN i <= 80 THEN DATE '2024-01-01' ELSE (DATE '2024-01-01' + i) END,
			CASE WHEN i <= 80 THEN TIMESTAMP '2024-01-01 00:00:00' ELSE (TIMESTAMP '2024-01-01 00:00:00' + (i || ' seconds')::INTERVAL) END,
			CASE WHEN i <= 80 THEN TIMESTAMPTZ '2024-01-01 00:00:00+00' ELSE (TIMESTAMPTZ '2024-01-01 00:00:00+00' + (i || ' seconds')::INTERVAL) END,
			CASE WHEN i <= 80 THEN '550e8400-e29b-41d4-a716-446655440000'::UUID ELSE ('00000000-0000-0000-0000-' || lpad(i::TEXT, 12, '0'))::UUID END,
			CASE WHEN i <= 80 THEN '{"k":"v"}'::JSONB ELSE jsonb_build_object('k', i) END,
			CASE WHEN i <= 80 THEN E'\\xDEADBEEF'::BYTEA ELSE decode(lpad(to_hex(i), 8, '0'), 'hex') END,
			CASE WHEN i <= 80 THEN '$99.95'::MONEY ELSE (i::NUMERIC)::MONEY END,
			CASE WHEN i <= 80 THEN ARRAY['a', 'b']::TEXT[] ELSE ARRAY['x', i::TEXT]::TEXT[] END,
			CASE WHEN i <= 80 THEN '12:00:00'::TIME ELSE ('12:00:00'::TIME + (i || ' minutes')::INTERVAL) END,
			CASE WHEN i <= 80 THEN 42 ELSE i + 1000 END,
			CASE WHEN i <= 80 THEN 5::SMALLINT ELSE (i % 100 + 200)::SMALLINT END,
			CASE WHEN i <= 80 THEN 123456789::BIGINT ELSE (123456789 + i)::BIGINT END
		FROM generate_series(1, 100) AS i;`,
		`CREATE INDEX idx_mf_c_text ON test_schema.test_data_types (c_text);`,
		`CREATE INDEX idx_mf_c_char ON test_schema.test_data_types (c_char);`,
		`CREATE INDEX idx_mf_c_varchar ON test_schema.test_data_types (c_varchar);`,
		`CREATE INDEX idx_mf_c_int ON test_schema.test_data_types (c_int);`,
		`CREATE INDEX idx_mf_c_int2 ON test_schema.test_data_types (c_int2);`,
		`CREATE INDEX idx_mf_c_int8 ON test_schema.test_data_types (c_int8);`,
		`CREATE INDEX idx_mf_c_numeric ON test_schema.test_data_types (c_numeric);`,
		`CREATE INDEX idx_mf_c_real ON test_schema.test_data_types (c_real);`,
		`CREATE INDEX idx_mf_c_float8 ON test_schema.test_data_types (c_float8);`,
		`CREATE INDEX idx_mf_c_bool ON test_schema.test_data_types (c_bool);`,
		`CREATE INDEX idx_mf_c_date ON test_schema.test_data_types (c_date);`,
		`CREATE INDEX idx_mf_c_timestamp ON test_schema.test_data_types (c_timestamp);`,
		`CREATE INDEX idx_mf_c_timestamptz ON test_schema.test_data_types (c_timestamptz);`,
		`CREATE INDEX idx_mf_c_uuid ON test_schema.test_data_types (c_uuid);`,
		`CREATE INDEX idx_mf_c_jsonb ON test_schema.test_data_types (c_jsonb);`,
		`CREATE INDEX idx_mf_c_bytea ON test_schema.test_data_types (c_bytea);`,
		`CREATE INDEX idx_mf_c_money ON test_schema.test_data_types (c_money);`,
		`CREATE INDEX idx_mf_c_text_array ON test_schema.test_data_types (c_text_array);`,
		`CREATE INDEX idx_mf_c_time ON test_schema.test_data_types (c_time);`,
		`CREATE INDEX idx_mf_c_serial ON test_schema.test_data_types (c_serial);`,
		`CREATE INDEX idx_mf_c_smallserial ON test_schema.test_data_types (c_smallserial);`,
		`CREATE INDEX idx_mf_c_bigserial ON test_schema.test_data_types (c_bigserial);`,
		`ANALYZE test_schema.test_data_types;`,
	)
	defer postgresContainer.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

	_, err = testutils.RunVoyagerCommand(postgresContainer, "assess-migration", []string{
		"--source-db-schema", "test_schema",
		"--iops-capture-interval", "0",
		"--export-dir", exportDir,
		"--yes",
	}, nil, false)
	testutils.FatalIfError(t, err, "Failed to run assess-migration command")

	assessmentReportPath := filepath.Join(exportDir, "assessment", "reports", fmt.Sprintf("%s.json", ASSESSMENT_FILE_NAME))
	report, err := ParseJSONToAssessmentReport(assessmentReportPath)
	testutils.FatalIfError(t, err, "failed to parse assessment report")

	indexToDataType := map[string]string{
		"idx_mf_c_text":        "text",
		"idx_mf_c_char":        "character",
		"idx_mf_c_varchar":     "character varying",
		"idx_mf_c_int":         "integer",
		"idx_mf_c_int2":        "int2",
		"idx_mf_c_int8":        "int8",
		"idx_mf_c_numeric":     "numeric",
		"idx_mf_c_real":        "real",
		"idx_mf_c_float8":      "float8",
		"idx_mf_c_bool":        "boolean",
		"idx_mf_c_date":        "date",
		"idx_mf_c_timestamp":   "timestamp",
		"idx_mf_c_timestamptz": "timestamptz",
		"idx_mf_c_uuid":        "uuid",
		"idx_mf_c_jsonb":       "jsonb",
		"idx_mf_c_bytea":       "bytea",
		"idx_mf_c_money":       "money",
		"idx_mf_c_text_array":  "text[]",
		"idx_mf_c_time":        "time without time zone",
		"idx_mf_c_serial":      "integer",
		"idx_mf_c_smallserial": "smallint",
		"idx_mf_c_bigserial":   "bigint",
	}

	recommendedSQLByObject := map[string]string{}
	for _, issue := range report.Issues {
		if issue.Type != queryissue.MOST_FREQUENT_VALUE_INDEXES || !strings.HasPrefix(issue.ObjectName, "idx_mf_") {
			continue
		}
		recommendedSQL, ok := getRecommendedSQLFromAssessmentIssue(issue)
		assert.Truef(t, ok, "expected RecommendedSQL in issue details for %s", issue.ObjectName)
		if ok {
			recommendedSQLByObject[issue.ObjectName] = recommendedSQL
		}
	}

	assert.Equal(t, len(indexToDataType), len(recommendedSQLByObject),
		"expected recommended SQL for all datatype test indexes")

	yugabyteContainer.ExecuteSqls(createSchemaAndTableSQL)
	defer yugabyteContainer.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

	ybConn, err := yugabyteContainer.GetConnection()
	testutils.FatalIfError(t, err, "failed to get yugabyte connection")
	defer ybConn.Close()

	for idx, dataType := range indexToDataType {
		objectName := fmt.Sprintf("%s ON test_schema.test_data_types", idx)
		recommendedSQL, ok := recommendedSQLByObject[objectName]
		if !assert.Truef(t, ok, "missing recommended SQL for %s", objectName) {
			continue
		}

		_, err = ybConn.Exec(recommendedSQL)
		if isUnsupportedIndexDatatype(dataType) {
			if assert.Errorf(t, err, "expected recommended SQL to fail for unsupported datatype index %s: %s", objectName, recommendedSQL) {
				assert.Contains(t, strings.ToLower(err.Error()), "not yet supported", "unexpected error for unsupported datatype index %s", objectName)
			}
		} else {
			assert.NoErrorf(t, err, "recommended SQL failed to compile in YugabyteDB for %s: %s", objectName, recommendedSQL)
		}
	}
}

func getRecommendedSQLFromAssessmentIssue(issue AssessmentIssue) (string, bool) {
	if issue.Details == nil {
		return "", false
	}
	val, ok := issue.Details[queryissue.RECOMMENDED_SQL]
	if !ok {
		return "", false
	}
	sql, ok := val.(string)
	return sql, ok
}

func isUnsupportedIndexDatatype(dataType string) bool {
	if strings.HasSuffix(dataType, "[]") {
		return true
	}
	return slices.Contains(queryissue.UnsupportedIndexDatatypes, dataType)
}