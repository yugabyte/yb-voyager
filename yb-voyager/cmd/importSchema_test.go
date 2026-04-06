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
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestImportSchemaWithYBSpecificSyntax(t *testing.T) {
	tempExportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(tempExportDir)

	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start postgres container: %v", err)
	}
	defer postgresContainer.Stop(context.Background())

	yugabyteContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err = yugabyteContainer.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start yugabyte container: %v", err)
	}
	defer yugabyteContainer.Stop(context.Background())

	postgresContainer.ExecuteSqls(
		`CREATE SCHEMA test_schema;`,
		`CREATE TABLE test_schema.test_table (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INT
		);`,
	)
	defer postgresContainer.ExecuteSqls(`DROP SCHEMA test_schema CASCADE;`)

	// Step 1: Export schema from PostgreSQL source
	_, err = testutils.RunVoyagerCommand(postgresContainer, "export schema", []string{
		"--source-db-schema", "test_schema",
		"--export-dir", tempExportDir,
		"--yes",
	}, nil, false)
	if err != nil {
		t.Fatalf("Failed to export schema: %v", err)
	}

	// Step 2: Modify table.sql to inject YB-specific SPLIT INTO clause
	tableSqlFilePath := filepath.Join(tempExportDir, "schema", "tables", "table.sql")
	if !utils.FileOrFolderExists(tableSqlFilePath) {
		t.Fatalf("table.sql does not exist at: %s", tableSqlFilePath)
	}

	content, err := os.ReadFile(tableSqlFilePath)
	if err != nil {
		t.Fatalf("Failed to read table.sql: %v", err)
	}

	expectedTablets := 3
	re := regexp.MustCompile(`(?s)(CREATE TABLE test_schema\.test_table\b.*?)\);`)
	original := string(content)
	modifiedContent := re.ReplaceAllString(original, fmt.Sprintf("${1}) SPLIT INTO %d TABLETS;", expectedTablets))
	if modifiedContent == original {
		t.Fatalf("Failed to modify table.sql: could not find CREATE TABLE test_schema.test_table DDL to modify")
	}

	err = os.WriteFile(tableSqlFilePath, []byte(modifiedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write modified table.sql: %v", err)
	}

	// Step 3: Import the modified schema into YugabyteDB
	_, err = testutils.RunVoyagerCommand(yugabyteContainer, "import schema", []string{
		"--export-dir", tempExportDir,
		"--yes",
		"--start-clean", "true",
	}, func() {
		time.Sleep(10 * time.Second)
	}, true)
	if err != nil {
		t.Fatalf("Failed to import schema: %v", err)
	}

	// Step 4: Verify the table was created with the expected number of tablets
	rows, err := yugabyteContainer.Query(
		"SELECT num_tablets FROM yb_table_properties('test_schema.test_table'::regclass)")
	if err != nil {
		t.Fatalf("Failed to query yb_table_properties: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatalf("No rows returned from yb_table_properties")
	}

	var numTablets int
	err = rows.Scan(&numTablets)
	if err != nil {
		t.Fatalf("Failed to scan num_tablets: %v", err)
	}

	assert.Equal(t, expectedTablets, numTablets,
		"Table should have %d tablets from SPLIT INTO clause, got %d", expectedTablets, numTablets)
}
