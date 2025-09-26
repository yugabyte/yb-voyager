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
	"testing"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
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
	SELECT md5(random()::text) FROM generate_series(1, 100000);`)
	defer postgresContainer.ExecuteSqls(
		`DROP SCHEMA test_schema CASCADE;`)

	doDuringCmd := func() {
		log.Infof("Performing sequential reads on the table")
		// sequential reads on the table
		postgresContainer.ExecuteSqls(`
DO $$ 
DECLARE i INT;
BEGIN
	FOR i IN 1..100 LOOP
		PERFORM COUNT(*) FROM test_schema.test_data;
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
			ColocatedReasoning:              "Recommended instance type with 4 vCPU and 16 GiB memory could fit 1 objects (1 tables/materialized views and 0 explicit/implicit indexes) with 6.52 MB size and throughput requirement of 10 reads/sec and 0 writes/sec as sharded. Non leaf partition tables/indexes and unsupported tables/indexes were not considered.",
			ShardedTables:                   []string{"test_schema.test_data"},
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
		"--iops-capture-interval", "10",
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
