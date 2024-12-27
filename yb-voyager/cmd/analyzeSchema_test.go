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
		t.Errorf("Expected %d SQL statements for %s, got %d",len(expectedSqlInfoArr),  objType, len(sqlInfoArr))
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
		t.Errorf("Expected %d SQL statements for %s, got %d",len(expectedSqlInfoArr), objType, len(sqlInfoArr))
	}

	for i, expectedSqlInfo := range expectedSqlInfoArr {
		assert.Equal(t, expectedSqlInfo.objName, sqlInfoArr[i].objName)
		assert.Equal(t, expectedSqlInfo.stmt, sqlInfoArr[i].stmt)
		assert.Equal(t, expectedSqlInfo.formattedStmt, sqlInfoArr[i].formattedStmt)
	}

}
