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
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestOracle_ReportUnsupportedIndexTypes(t *testing.T) {
	ctx := context.Background()

	// Create a temporary export directory.
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	oracleContainer := testcontainers.NewTestContainer("oracle", nil)
	if err := oracleContainer.Start(ctx); err != nil {
		utils.ErrExit("Failed to start Oracle container: %v", err)
	}

	// Export schema from Oracle
	_, err := testutils.RunVoyagerCommand(oracleContainer, "export schema", []string{
		"--export-dir", exportDir,
		"--source-db-schema", "YBVOYAGER",
		"--yes",
	}, nil, false)

	if err != nil {
		t.Fatalf("Failed to export schema from Oracle: %v", err)
	}

	//  Run analyze schema command
	_, err = testutils.RunVoyagerCommand(oracleContainer, "analyze-schema", []string{
		"--export-dir", exportDir,
		"--yes",
	}, nil, false)
	if err != nil {
		t.Fatalf("Failed to analyze schema: %v", err)
	}

	// verify the generated report in exportDir/reports/schema_analysis_report.json
	reportFilePath := fmt.Sprintf("%s/reports/schema_analysis_report.json", exportDir)
	if _, err := os.Stat(reportFilePath); os.IsNotExist(err) {
		t.Fatalf("Schema analysis report file does not exist: %s", reportFilePath)
	}

	// parse json in analyze schema struct
	reportPath := fmt.Sprintf("%s/reports/schema_analysis_report.json", exportDir)
	report, err := ParseJsonToAnalyzeSchemaReport(reportPath)
	if err != nil {
		t.Fatalf("Failed to parse schema analysis report: %v", err)
	}

	// Check if the report contains unsupported index types
	expectedJsonString := `Indexes which are neither exported by yb-voyager as they are unsupported in YB and needs to be handled manually:
		Index Name=IDX_ADDRESS_TEXT, Index Type=DOMAIN
		Index Name=IDX_PHONE_REVERSE, Index Type=NORMAL/REV

There are some GIN indexes present in the schema, but GIN indexes are partially supported in YugabyteDB as mentioned in (https://github.com/yugabyte/yugabyte-db/issues/7850) so take a look and modify them if not supported.`

	var actualString string
	for _, dbObject := range report.SchemaSummary.DBObjects {
		if dbObject.ObjectType == constants.INDEX {
			actualString = dbObject.Details
		}
	}

	assert.Equal(t, expectedJsonString, actualString, "The unsupported index types in the report do not match the expected output")
}

func Test_ContainerResumption(t *testing.T) {
	mysqlContainer := testcontainers.NewTestContainer(testcontainers.MYSQL, nil)
	if err := mysqlContainer.Start(context.Background()); err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	// create table and insert 1 row
	mysqlContainer.ExecuteSqls(
		`CREATE TABLE test_table (id INT PRIMARY KEY, name VARCHAR(50));`,
		`INSERT INTO test_table (id, name) VALUES (9999, 'test_name');`)

	color.Red("Stopping PostgreSQL container to test resumption...")
	// Stop the container
	if err := mysqlContainer.Stop(context.Background()); err != nil {
		t.Fatalf("Failed to stop PostgreSQL container: %v", err)
	}

	// Restart the container
	color.Yellow("Restarting PostgreSQL container to test resumption...")
	if err := mysqlContainer.Start(context.Background()); err != nil {
		t.Fatalf("Failed to restart PostgreSQL container: %v", err)
	}

	color.Green("PostgreSQL container restarted successfully, now checking if data is intact...")
	rows, err := mysqlContainer.Query("SELECT * FROM test_table;", nil)
	if err != nil {
		t.Fatalf("Failed to query PostgreSQL container: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatalf("Expected to find a row in the table, but found none")
	}
	var id int
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		t.Fatalf("Failed to scan row: %v", err)
	}
	if id != 9999 || name != "test_name" {
		t.Fatalf("Expected row (1, 'test_name'), but got (%d, '%s')", id, name)
	}
	utils.PrintAndLogf("PostgreSQL container resumed successfully with data intact")
}
