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

// TestImportSchemaWithPercentType covers PL/pgSQL functions whose body uses
// unqualified `%TYPE` references against (a) a relation imported earlier in
// the object order — table — and (b) a relation imported later — view. Both
// paths previously failed under pg_dump's empty `search_path` preamble; the
// fix injects `SET search_path = <schemas>;` into function.sql at export time
// and broadens the import-side defer classifier to catch 42601 + %TYPE.
func TestImportSchemaWithPercentType(t *testing.T) {
	tempExportDir := testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(tempExportDir)

	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	if err := postgresContainer.Start(context.Background()); err != nil {
		t.Fatalf("Failed to start postgres container: %v", err)
	}
	defer postgresContainer.Stop(context.Background())

	yugabyteContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	if err := yugabyteContainer.Start(context.Background()); err != nil {
		t.Fatalf("Failed to start yugabyte container: %v", err)
	}
	defer yugabyteContainer.Stop(context.Background())

	postgresContainer.ExecuteSqls(
		`CREATE SCHEMA pct;`,
		// Source-side search_path must include pct so PL/pgSQL validation at
		// CREATE FUNCTION time can resolve the body's unqualified refs. The
		// bug under test is target-side; source just needs to accept the DDL.
		`SET search_path = pct, public;`,
		`CREATE TABLE pct.employees (employee_id int PRIMARY KEY, salary numeric);`,
		`INSERT INTO pct.employees VALUES (1, 50000), (2, 75000);`,
		// Case A: unqualified %TYPE against an existing table — exercises the
		// export-side SET search_path injection (first-pass success).
		`CREATE FUNCTION pct.get_employee_salary(emp_id integer) RETURNS numeric
		    LANGUAGE plpgsql AS $$
		DECLARE emp_salary employees.salary%TYPE;
		BEGIN
		    SELECT salary INTO emp_salary FROM employees WHERE employee_id = emp_id;
		    RETURN emp_salary;
		END;
		$$;`,
		`CREATE VIEW pct.v_sal AS SELECT employee_id, salary FROM pct.employees;`,
		// Case B: unqualified %TYPE against a VIEW (imported AFTER functions in
		// voyager's object order) — exercises the import-side defer classifier.
		`CREATE FUNCTION pct.get_view_salary(p_id integer) RETURNS numeric
		    LANGUAGE plpgsql AS $$
		DECLARE v v_sal.salary%TYPE;
		BEGIN
		    SELECT salary INTO v FROM v_sal WHERE employee_id = p_id;
		    RETURN v;
		END;
		$$;`,
	)
	defer postgresContainer.ExecuteSqls(`DROP SCHEMA pct CASCADE;`)

	if _, err := testutils.RunVoyagerCommand(postgresContainer, "export schema", []string{
		"--source-db-schema", "pct",
		"--export-dir", tempExportDir,
		"--yes",
	}, nil, false); err != nil {
		t.Fatalf("Failed to export schema: %v", err)
	}

	if _, err := testutils.RunVoyagerCommand(yugabyteContainer, "import schema", []string{
		"--export-dir", tempExportDir,
		"--yes",
		"--start-clean", "true",
	}, func() { time.Sleep(10 * time.Second) }, true); err != nil {
		t.Fatalf("Failed to import schema: %v", err)
	}

	// Verify both functions end up created in YB and resolve their unqualified
	// %TYPE refs correctly. This covers both cases: get_employee_salary references
	// a table imported earlier in the object order, and get_view_salary
	// forward-references a view imported later. The DO blocks below invoke each
	// function; a call to a function that was never created fails with a query
	// error caught by t.Fatalf, so a missing object is a test failure.

	// Function bodies use unqualified %TYPE refs that re-resolve at every call
	// via the caller's search_path. Pin one physical connection (sql.DB is a
	// pool — SET on one conn doesn't carry to the next) so the SET persists
	// across the DO blocks. Each DO block invokes a function and RAISEs on a
	// wrong return, surfacing as a query error caught by t.Fatalf — keeping
	// deferred container teardown alive, unlike container.ExecuteSqls which
	// would utils.ErrExit on failure and kill the test process.
	db, err := yugabyteContainer.GetConnection()
	if err != nil {
		t.Fatalf("get YB connection: %v", err)
	}
	defer db.Close()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("pin YB conn: %v", err)
	}
	defer conn.Close()
	for _, stmt := range []string{
		`INSERT INTO pct.employees VALUES (3, 99000);`,
		`SET search_path = pct, public;`,
		`DO $$ DECLARE r numeric;
		 BEGIN
		   r := get_employee_salary(1);
		   IF r != 50000 THEN RAISE 'get_employee_salary(1)=% want 50000', r; END IF;
		 END $$;`,
		`DO $$ DECLARE r numeric;
		 BEGIN
		   r := get_view_salary(3);
		   IF r != 99000 THEN RAISE 'get_view_salary(3)=% want 99000', r; END IF;
		 END $$;`,
	} {
		if _, err := conn.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
}
