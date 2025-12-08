//go:build unit

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
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setupFile(objType string, sqlContent string) (*os.File, error) {
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("%s.sql", strings.ToLower(objType)))
	if err != nil {
		return nil, err
	}
	// Write the SQL content to the file
	if err := os.WriteFile(tmpFile.Name(), []byte(sqlContent), 0644); err != nil {
		return nil, err
	}

	return tmpFile, nil
}

func TestTableSQLFile(t *testing.T) {
	tableFileContent := `-- This is a comment
CREATE TABLE test_table (
    id INT PRIMARY KEY,
    name VARCHAR(100)
);

-- Another comment
CREATE TABLE another_table (
    id INT PRIMARY KEY,
    description TEXT
);`
	objType := "TABLE"
	expectedSqlInfoArr := []sqlInfo{
		sqlInfo{
			objName: "test_table",
			stmt:    "CREATE TABLE test_table (     id INT PRIMARY KEY,     name VARCHAR(100) ); ",
			formattedStmt: `CREATE TABLE test_table (
    id INT PRIMARY KEY,
    name VARCHAR(100)
);`,
		},
		sqlInfo{
			objName: "another_table",
			stmt:    "CREATE TABLE another_table (     id INT PRIMARY KEY,     description TEXT ); ",
			formattedStmt: `CREATE TABLE another_table (
    id INT PRIMARY KEY,
    description TEXT
);`,
		},
	}

	sqlFile, err := setupFile(objType, tableFileContent)
	if err != nil {
		t.Errorf("Error creating file for the objType %s: %v", objType, err)
	}

	defer os.Remove(sqlFile.Name())

	sqlInfoArr := parseSqlFileForObjectType(sqlFile.Name(), objType)
	// Validate the number of SQL statements found
	if len(sqlInfoArr) != len(expectedSqlInfoArr) {
		t.Errorf("Expected %d SQL statements for %s, got %d", len(expectedSqlInfoArr), objType, len(sqlInfoArr))
	}

	for i, expectedSqlInfo := range expectedSqlInfoArr {
		assert.Equal(t, expectedSqlInfo.objName, sqlInfoArr[i].objName)
		assert.Equal(t, expectedSqlInfo.stmt, sqlInfoArr[i].stmt)
		assert.Equal(t, expectedSqlInfo.formattedStmt, sqlInfoArr[i].formattedStmt)
	}

}

func TestProcedureSQLFile(t *testing.T) {
	procedureFileContent := `-- Procedure to add a new record
CREATE PROCEDURE add_record() AS $$
BEGIN
    INSERT INTO test_table (id, name) VALUES (1, 'Test');
END;
$$ LANGUAGE plpgsql;
    
-- Another procedure
CREATE PROCEDURE delete_record() AS $$
BEGIN
    DELETE FROM test_table WHERE id = 1;
END;
$$ LANGUAGE plpgsql;`

	expectedSqlInfoArr := []sqlInfo{
		sqlInfo{
			objName: "add_record",
			stmt:    "CREATE PROCEDURE add_record() AS $$ BEGIN     INSERT INTO test_table (id, name) VALUES (1, 'Test'); END; $$ LANGUAGE plpgsql; ",
			formattedStmt: `CREATE PROCEDURE add_record() AS $$
BEGIN
    INSERT INTO test_table (id, name) VALUES (1, 'Test');
END;
$$ LANGUAGE plpgsql;`,
		},
		sqlInfo{
			objName: "delete_record",
			stmt:    "CREATE PROCEDURE delete_record() AS $$ BEGIN     DELETE FROM test_table WHERE id = 1; END; $$ LANGUAGE plpgsql; ",
			formattedStmt: `CREATE PROCEDURE delete_record() AS $$
BEGIN
    DELETE FROM test_table WHERE id = 1;
END;
$$ LANGUAGE plpgsql;`,
		},
	}
	objType := "PROCEDURE"
	sqlFile, err := setupFile(objType, procedureFileContent)
	if err != nil {
		t.Errorf("Error creating file for the objType %s: %v", objType, err)
	}

	defer os.Remove(sqlFile.Name())

	sqlInfoArr := parseSqlFileForObjectType(sqlFile.Name(), objType)

	// Validate the number of SQL statements found
	if len(sqlInfoArr) != len(expectedSqlInfoArr) {
		t.Errorf("Expected %d SQL statements for %s, got %d", len(expectedSqlInfoArr), objType, len(sqlInfoArr))
	}

	for i, expectedSqlInfo := range expectedSqlInfoArr {
		assert.Equal(t, expectedSqlInfo.objName, sqlInfoArr[i].objName)
		assert.Equal(t, expectedSqlInfo.stmt, sqlInfoArr[i].stmt)
		assert.Equal(t, expectedSqlInfo.formattedStmt, sqlInfoArr[i].formattedStmt)
	}

}

func TestFunctionSQLFile(t *testing.T) {
	functionFileContent := `CREATE FUNCTION public.asterisks(n integer) RETURNS SETOF text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    BEGIN ATOMIC
 SELECT repeat('*'::text, g.g) AS repeat
    FROM generate_series(1, asterisks.n) g(g);
END;

CREATE OR REPLACE FUNCTION copy_high_earners(threshold NUMERIC) RETURNS VOID AS $$
DECLARE
    temp_salary employees.salary%TYPE;
BEGIN
    CREATE TEMP TABLE temp_high_earners AS
    SELECT * FROM employees WHERE salary > threshold;
    FOR temp_salary IN SELECT salary FROM temp_high_earners LOOP
        RAISE NOTICE 'High earner salary: %', temp_salary;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION add(int, int) RETURNS int IMMUTABLE PARALLEL SAFE BEGIN ATOMIC; SELECT $1 + $2; END;

CREATE FUNCTION add(int, int) RETURNS int IMMUTABLE PARALLEL SAFE 
BEGIN ATOMIC; SELECT $1 + $2; END;

CREATE FUNCTION public.case_sensitive_test(n integer) RETURNS SETOF text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    begin atomic
 SELECT repeat('*'::text, g.g) AS repeat
    FROM generate_series(1, asterisks.n) g(g);
end;

CREATE FUNCTION public.asterisks1(n integer) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    RETURN repeat('*'::text, n);

CREATE FUNCTION add(integer, integer) RETURNS integer
    AS 'select test;'
    LANGUAGE SQL
    IMMUTABLE
    RETURNS NULL ON NULL INPUT;

CREATE OR REPLACE FUNCTION increment(i integer) RETURNS integer AS $$
        BEGIN
                RETURN i + 1;
        END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION public.dup(integer, OUT f1 integer, OUT f2 text) RETURNS record
    LANGUAGE sql
    AS $_$ SELECT $1, CAST($1 AS text) || ' is text' $_$;

CREATE FUNCTION check_password(uname TEXT, pass TEXT)
RETURNS BOOLEAN AS $$
DECLARE passed BOOLEAN;
BEGIN
        SELECT  (pwd = $2) INTO passed
        FROM    pwds
        WHERE   username = $1;

        RETURN passed;
END;
$$  LANGUAGE plpgsql
    SECURITY DEFINER
    -- Set a secure search_path: trusted schema(s), then 'pg_temp'.
    SET search_path = admin, pg_temp;`

	expectedSqlInfoArr := []sqlInfo{
		sqlInfo{
			objName: "public.asterisks",
			stmt:    "CREATE FUNCTION public.asterisks(n integer) RETURNS SETOF text     LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE     BEGIN ATOMIC  SELECT repeat('*'::text, g.g) AS repeat     FROM generate_series(1, asterisks.n) g(g); END; ",
			formattedStmt: `CREATE FUNCTION public.asterisks(n integer) RETURNS SETOF text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    BEGIN ATOMIC
 SELECT repeat('*'::text, g.g) AS repeat
    FROM generate_series(1, asterisks.n) g(g);
END;`,
		},
		sqlInfo{
			objName: "copy_high_earners",
			stmt:    "CREATE OR REPLACE FUNCTION copy_high_earners(threshold NUMERIC) RETURNS VOID AS $$ DECLARE     temp_salary employees.salary%TYPE; BEGIN     CREATE TEMP TABLE temp_high_earners AS     SELECT * FROM employees WHERE salary > threshold;     FOR temp_salary IN SELECT salary FROM temp_high_earners LOOP         RAISE NOTICE 'High earner salary: %', temp_salary;     END LOOP; END; $$ LANGUAGE plpgsql; ",
			formattedStmt: `CREATE OR REPLACE FUNCTION copy_high_earners(threshold NUMERIC) RETURNS VOID AS $$
DECLARE
    temp_salary employees.salary%TYPE;
BEGIN
    CREATE TEMP TABLE temp_high_earners AS
    SELECT * FROM employees WHERE salary > threshold;
    FOR temp_salary IN SELECT salary FROM temp_high_earners LOOP
        RAISE NOTICE 'High earner salary: %', temp_salary;
    END LOOP;
END;
$$ LANGUAGE plpgsql;`,
		},
		sqlInfo{
			objName:       "add",
			stmt:          "CREATE FUNCTION add(int, int) RETURNS int IMMUTABLE PARALLEL SAFE BEGIN ATOMIC; SELECT $1 + $2; END; ",
			formattedStmt: `CREATE FUNCTION add(int, int) RETURNS int IMMUTABLE PARALLEL SAFE BEGIN ATOMIC; SELECT $1 + $2; END;`,
		},
		sqlInfo{
			objName:       "add",
			stmt:          "CREATE FUNCTION add(int, int) RETURNS int IMMUTABLE PARALLEL SAFE BEGIN ATOMIC; SELECT $1 + $2; END; ",
			formattedStmt: "CREATE FUNCTION add(int, int) RETURNS int IMMUTABLE PARALLEL SAFE\nBEGIN ATOMIC; SELECT $1 + $2; END;",
		},
		sqlInfo{
			objName: "public.case_sensitive_test",
			stmt:    "CREATE FUNCTION public.case_sensitive_test(n integer) RETURNS SETOF text     LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE     begin atomic  SELECT repeat('*'::text, g.g) AS repeat     FROM generate_series(1, asterisks.n) g(g); end; ",
			formattedStmt: `CREATE FUNCTION public.case_sensitive_test(n integer) RETURNS SETOF text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    begin atomic
 SELECT repeat('*'::text, g.g) AS repeat
    FROM generate_series(1, asterisks.n) g(g);
end;`,
		},
		sqlInfo{
			objName: "public.asterisks1",
			stmt:    "CREATE FUNCTION public.asterisks1(n integer) RETURNS text     LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE     RETURN repeat('*'::text, n); ",
			formattedStmt: `CREATE FUNCTION public.asterisks1(n integer) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    RETURN repeat('*'::text, n);`,
		},
		sqlInfo{
			objName: "add",
			stmt:    "CREATE FUNCTION add(integer, integer) RETURNS integer     AS 'select test;'     LANGUAGE SQL     IMMUTABLE     RETURNS NULL ON NULL INPUT; ",
			formattedStmt: `CREATE FUNCTION add(integer, integer) RETURNS integer
    AS 'select test;'
    LANGUAGE SQL
    IMMUTABLE
    RETURNS NULL ON NULL INPUT;`,
		},
		sqlInfo{
			objName: "increment",
			stmt:    "CREATE OR REPLACE FUNCTION increment(i integer) RETURNS integer AS $$         BEGIN                 RETURN i + 1;         END; $$ LANGUAGE plpgsql; ",
			formattedStmt: `CREATE OR REPLACE FUNCTION increment(i integer) RETURNS integer AS $$
        BEGIN
                RETURN i + 1;
        END;
$$ LANGUAGE plpgsql;`,
		},
		sqlInfo{
			objName: "public.dup",
			stmt:    "CREATE FUNCTION public.dup(integer, OUT f1 integer, OUT f2 text) RETURNS record     LANGUAGE sql     AS $_$ SELECT $1, CAST($1 AS text) || ' is text' $_$; ",
			formattedStmt: `CREATE FUNCTION public.dup(integer, OUT f1 integer, OUT f2 text) RETURNS record
    LANGUAGE sql
    AS $_$ SELECT $1, CAST($1 AS text) || ' is text' $_$;`,
		},
		sqlInfo{
			objName: "check_password",
			stmt:    "CREATE FUNCTION check_password(uname TEXT, pass TEXT) RETURNS BOOLEAN AS $$ DECLARE passed BOOLEAN; BEGIN         SELECT  (pwd = $2) INTO passed         FROM    pwds         WHERE   username = $1;         RETURN passed; END; $$  LANGUAGE plpgsql     SECURITY DEFINER     -- Set a secure search_path: trusted schema(s), then 'pg_temp'.     SET search_path = admin, pg_temp; ",
			formattedStmt: `CREATE FUNCTION check_password(uname TEXT, pass TEXT)
RETURNS BOOLEAN AS $$
DECLARE passed BOOLEAN;
BEGIN
        SELECT  (pwd = $2) INTO passed
        FROM    pwds
        WHERE   username = $1;
        RETURN passed;
END;
$$  LANGUAGE plpgsql
    SECURITY DEFINER
    -- Set a secure search_path: trusted schema(s), then 'pg_temp'.
    SET search_path = admin, pg_temp;`,
		},
	}
	objType := "FUNCTION"
	sqlFile, err := setupFile(objType, functionFileContent)
	if err != nil {
		t.Errorf("Error creating file for the objType %s: %v", objType, err)
	}

	defer os.Remove(sqlFile.Name())

	sqlInfoArr := parseSqlFileForObjectType(sqlFile.Name(), objType)

	// Validate the number of SQL statements found
	if len(sqlInfoArr) != len(expectedSqlInfoArr) {
		t.Errorf("Expected %d SQL statements for %s, got %d", len(expectedSqlInfoArr), objType, len(sqlInfoArr))
	}

	for i, expectedSqlInfo := range expectedSqlInfoArr {
		assert.Equal(t, expectedSqlInfo.objName, sqlInfoArr[i].objName)
		assert.Equal(t, expectedSqlInfo.stmt, sqlInfoArr[i].stmt)
		assert.Equal(t, expectedSqlInfo.formattedStmt, sqlInfoArr[i].formattedStmt)
	}

}
