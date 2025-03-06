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
package sqltransformer

import (
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

/*
	Test cases covered
	1. Basic test for merge constraints
	2. Merge constraints with all possible constraints type
	3. Merge constraints with different casing of table names
	4. Single Alter Table with multiple subcommands in it
	5. Foreign key circular dependency case
	6. Quoted column names
	7. Error when CREATE TABLE is not present for a alter
	8. Miscellaneous - comments, large check constraint, can there variations in the ALTER for above cases(?)
	9. [Extra] Exclude constraint (omission of USING btree by parser)
*/

func TestMergeConstraints_Basic(t *testing.T) {
	sqlFileContent := `
	CREATE TABLE test_table1 (
		id INT,
		name VARCHAR(255)
	);

	CREATE TABLE test_table2 (
		id INT,
		name VARCHAR(255),
		email VARCHAR(255)
	);

	ALTER TABLE test_table1 ADD CONSTRAINT test_table_pk PRIMARY KEY (id);
	ALTER TABLE test_table2 ADD CONSTRAINT test_table_fk FOREIGN KEY (id) REFERENCES test_table1 (id);
	ALTER TABLE test_table2 ADD CONSTRAINT test_table_uk UNIQUE (email);
	`

	expectedSqls := []string{
		`CREATE TABLE test_table1 (id int, name varchar(255), CONSTRAINT test_table_pk PRIMARY KEY (id));`,
		`CREATE TABLE test_table2 (id int, name varchar(255), email varchar(255), CONSTRAINT test_table_uk UNIQUE (email));`,
		`ALTER TABLE test_table2 ADD CONSTRAINT test_table_fk FOREIGN KEY (id) REFERENCES test_table1 (id);`,
	}

	tempFilePath, err := testutils.CreateTempFile("/tmp", sqlFileContent, "sql")
	testutils.FatalIfError(t, err)

	stmts, err := queryparser.ParseSqlFile(tempFilePath)
	testutils.FatalIfError(t, err)

	transformer := NewTransformer()
	transformedStmts, err := transformer.MergeConstraints(stmts.Stmts)
	testutils.FatalIfError(t, err)

	finalSqlStmts, err := queryparser.DeparseRawStmts(transformedStmts)
	testutils.FatalIfError(t, err)

	testutils.AssertEqualStringSlices(t, expectedSqls, finalSqlStmts)
}

func TestMergeConstraints_AllSupportedConstraintTypes(t *testing.T) {
	sqlFileContent := `
    CREATE TABLE all_constraints (
        col1 INT,
        col2 TEXT
    );

	-- CHECK Constraint
    ALTER TABLE all_constraints
        ADD CONSTRAINT c_ck CHECK (col1 > 0);

    -- PRIMARY KEY Constraint
    ALTER TABLE all_constraints
        ADD CONSTRAINT c_pk PRIMARY KEY (col1);

    -- UNIQUE Constraint
    ALTER TABLE all_constraints
        ADD CONSTRAINT c_uk UNIQUE (col2);

	-- FOREIGN Key Constraint
	ALTER TABLE all_constraints
		ADD CONSTRAINT c_fk FOREIGN KEY (col2) REFERENCES some_table(txt);

    -- EXCLUSION Constraint
    ALTER TABLE all_constraints
        ADD CONSTRAINT c_excl EXCLUDE USING gist (col2 WITH &&);

    -- NOT NULL
    ALTER TABLE all_constraints ALTER COLUMN col1 SET NOT NULL;

    -- DEFAULT
    ALTER TABLE all_constraints ALTER COLUMN col1 SET DEFAULT 100;

    -- IDENTITY
    ALTER TABLE all_constraints ALTER COLUMN col1 ADD GENERATED ALWAYS AS IDENTITY;

    -- Some deferrability attributes, usually seen as part of FK constraints, e.g.:
    ALTER TABLE all_constraints
        ADD CONSTRAINT c_fk_deferrable FOREIGN KEY (col1) REFERENCES some_table(id)
        DEFERRABLE INITIALLY DEFERRED;
	`

	expectedSqls := []string{
		`CREATE TABLE all_constraints (col1 int, col2 text, CONSTRAINT c_ck CHECK (col1 > 0), CONSTRAINT c_pk PRIMARY KEY (col1), CONSTRAINT c_uk UNIQUE (col2));`,
		`ALTER TABLE all_constraints ALTER COLUMN col1 SET NOT NULL;`,
		`ALTER TABLE all_constraints ALTER COLUMN col1 SET DEFAULT 100;`,
		`ALTER TABLE all_constraints ALTER col1 ADD GENERATED ALWAYS AS IDENTITY;`,
		`ALTER TABLE all_constraints ADD CONSTRAINT c_excl EXCLUDE USING gist (col2 WITH &&);`,
		`ALTER TABLE all_constraints ADD CONSTRAINT c_fk FOREIGN KEY (col2) REFERENCES some_table (txt);`,
		`ALTER TABLE all_constraints ADD CONSTRAINT c_fk_deferrable FOREIGN KEY (col1) REFERENCES some_table (id) DEFERRABLE INITIALLY DEFERRED;`,
	}

	// The rest is standard test boilerplate:
	tempFilePath, err := testutils.CreateTempFile("/tmp", sqlFileContent, "sql")
	testutils.FatalIfError(t, err)

	stmts, err := queryparser.ParseSqlFile(tempFilePath)
	testutils.FatalIfError(t, err)

	transformer := NewTransformer()
	transformedStmts, err := transformer.MergeConstraints(stmts.Stmts)
	testutils.FatalIfError(t, err)

	finalSqlStmts, err := queryparser.DeparseRawStmts(transformedStmts)
	testutils.FatalIfError(t, err)

	testutils.AssertEqualStringSlices(t, expectedSqls, finalSqlStmts)
}

func TestMergeConstraints_DifferentCasing(t *testing.T) {
	sqlFileContent := `
        CREATE TABLE "MixedCaseTable" (
            ID INT,
            VALUE TEXT
        );

        ALTER TABLE "MixedCaseTable" ADD CONSTRAINT pk_mixed PRIMARY KEY (ID);
    `
	// If merging is successful, we expect the PK inline:
	expectedSqls := []string{
		`CREATE TABLE "MixedCaseTable" (id int, value text, CONSTRAINT pk_mixed PRIMARY KEY (id));`,
	}

	tempFilePath, err := testutils.CreateTempFile("/tmp", sqlFileContent, "sql")
	testutils.FatalIfError(t, err)

	stmts, err := queryparser.ParseSqlFile(tempFilePath)
	testutils.FatalIfError(t, err)

	transformer := NewTransformer()
	transformedStmts, err := transformer.MergeConstraints(stmts.Stmts)
	testutils.FatalIfError(t, err)

	finalSqlStmts, err := queryparser.DeparseRawStmts(transformedStmts)
	testutils.FatalIfError(t, err)

	testutils.AssertEqualStringSlices(t, expectedSqls, finalSqlStmts)
}

func TestMergeConstraints_MultipleConstraintsInSingleStmt(t *testing.T) {
	sqlFileContent := `
        CREATE TABLE tbl_multi (
            id INT,
            val INT
        );

        ALTER TABLE tbl_multi
            ADD CONSTRAINT multi_pk PRIMARY KEY (id),
            ADD CONSTRAINT multi_ck CHECK (val > 0),
            ADD CONSTRAINT multi_uk UNIQUE (val);
    `

	expectedSqls := []string{
		`CREATE TABLE tbl_multi (id int, val int);`,
		`ALTER TABLE tbl_multi ADD CONSTRAINT multi_pk PRIMARY KEY (id), ADD CONSTRAINT multi_ck CHECK (val > 0), ADD CONSTRAINT multi_uk UNIQUE (val);`,
	}

	tempFilePath, err := testutils.CreateTempFile("/tmp", sqlFileContent, "sql")
	testutils.FatalIfError(t, err)

	stmts, err := queryparser.ParseSqlFile(tempFilePath)
	testutils.FatalIfError(t, err)

	transformer := NewTransformer()
	transformedStmts, err := transformer.MergeConstraints(stmts.Stmts)
	testutils.FatalIfError(t, err)

	finalSqlStmts, err := queryparser.DeparseRawStmts(transformedStmts)
	testutils.FatalIfError(t, err)

	testutils.AssertEqualStringSlices(t, expectedSqls, finalSqlStmts)
}

func TestMergeConstraints_CircularDependencyWithSeparateFK(t *testing.T) {
	sqlFileContent := `
    -- Table t1 has primary key on (id)
    CREATE TABLE t1 (
		id INT PRIMARY KEY,
		t2_id INT
    );

    -- Table t2 has primary key on (id2)
    CREATE TABLE t2 (
		id2 INT PRIMARY KEY,
		t1_id INT
    );

    -- Now we add FKs referencing each other (circular):
    ALTER TABLE t1 ADD CONSTRAINT fk1 FOREIGN KEY (t2_id) REFERENCES t2 (id2);
    ALTER TABLE t2 ADD CONSTRAINT fk2 FOREIGN KEY (t1_id) REFERENCES t1 (id);
    `

	expectedSqls := []string{
		`CREATE TABLE t1 (id int PRIMARY KEY, t2_id int);`,
		`CREATE TABLE t2 (id2 int PRIMARY KEY, t1_id int);`,
		`ALTER TABLE t1 ADD CONSTRAINT fk1 FOREIGN KEY (t2_id) REFERENCES t2 (id2);`,
		`ALTER TABLE t2 ADD CONSTRAINT fk2 FOREIGN KEY (t1_id) REFERENCES t1 (id);`,
	}

	tempFilePath, err := testutils.CreateTempFile("/tmp", sqlFileContent, "sql")
	testutils.FatalIfError(t, err)

	stmts, err := queryparser.ParseSqlFile(tempFilePath)
	testutils.FatalIfError(t, err)

	transformer := NewTransformer()
	transformedStmts, err := transformer.MergeConstraints(stmts.Stmts)
	testutils.FatalIfError(t, err)

	finalSqlStmts, err := queryparser.DeparseRawStmts(transformedStmts)
	testutils.FatalIfError(t, err)

	testutils.AssertEqualStringSlices(t, expectedSqls, finalSqlStmts)
}

func TestMergeConstraints_QuotedColumnNames(t *testing.T) {
	sqlFileContent := `
	CREATE TABLE "Employee Data" (
		"Emp ID" INT,
		"Full Name" TEXT
	);

	ALTER TABLE "Employee Data"
		ADD CONSTRAINT "Emp_pk" PRIMARY KEY("Emp ID");
	`

	expectedSqls := []string{
		`CREATE TABLE "Employee Data" ("Emp ID" int, "Full Name" text, CONSTRAINT "Emp_pk" PRIMARY KEY ("Emp ID"));`,
	}

	tempFilePath, err := testutils.CreateTempFile("/tmp", sqlFileContent, "sql")
	testutils.FatalIfError(t, err)

	stmts, err := queryparser.ParseSqlFile(tempFilePath)
	testutils.FatalIfError(t, err)

	transformer := NewTransformer()
	transformedStmts, err := transformer.MergeConstraints(stmts.Stmts)
	testutils.FatalIfError(t, err)

	finalSqlStmts, err := queryparser.DeparseRawStmts(transformedStmts)
	testutils.FatalIfError(t, err)

	testutils.AssertEqualStringSlices(t, expectedSqls, finalSqlStmts)
}

func TestMergeConstraints_AlterWithoutCreateTableError(t *testing.T) {
	sqlFileContent := `
	CREATE TABLE some_table (
		id INT,
		name VARCHAR(255)
	);

	ALTER TABLE missing_table
		ADD CONSTRAINT missing_pk PRIMARY KEY (id);
	`

	tempFilePath, err := testutils.CreateTempFile("/tmp", sqlFileContent, "sql")
	testutils.FatalIfError(t, err)

	stmts, err := queryparser.ParseSqlFile(tempFilePath)
	testutils.FatalIfError(t, err)

	transformer := NewTransformer()
	_, transformErr := transformer.MergeConstraints(stmts.Stmts)
	if transformErr == nil {
		t.Fatalf("expected an error because CREATE TABLE is missing, but got no error")
	}

	// Check if the error message is as expected
	expectedErrMsg := "CREATE TABLE stmt not found for table missing_table"
	if transformErr.Error() != expectedErrMsg {
		t.Fatalf("expected error: %v, got: %v", expectedErrMsg, transformErr.Error())
	}
}

/*
	jFYI: For EXCLUDE constraint, the USING btree is omitted by parser during deparsing.

	Before: ALTER TABLE ONLY public.test_exclude_basic ADD CONSTRAINT no_same_name_address EXCLUDE USING btree (name WITH =, address WITH =);
	After:  ALTER TABLE ONLY public.test_exclude_basic ADD CONSTRAINT no_same_name_address EXCLUDE (name WITH =, address WITH =);
*/
// It covers exclusion constraint where parser omits USING clause for btree index method
func TestMergeConstraints_ExcludeConstraintType(t *testing.T) {
	sqlFileContent := `
	-- Create a simple table with two columns:
	CREATE TABLE ex_test (a int, b text);

	-- Alter the table to add an exclusion constraint
	ALTER TABLE ex_test ADD CONSTRAINT ex_foo EXCLUDE USING btree (a WITH =, b WITH =);

	CREATE TABLE ex_test_ts (
		ts tsrange
	);

	ALTER TABLE ex_test_ts ADD CONSTRAINT ex_ts_1 EXCLUDE USING gist (ts WITH &&);
	`

	expectedSqls := []string{
		`CREATE TABLE ex_test (a int, b text);`,
		`ALTER TABLE ex_test ADD CONSTRAINT ex_foo EXCLUDE (a WITH =, b WITH =);`,
		`CREATE TABLE ex_test_ts (ts tsrange);`,
		`ALTER TABLE ex_test_ts ADD CONSTRAINT ex_ts_1 EXCLUDE USING gist (ts WITH &&);`,
	}

	tempFilePath, err := testutils.CreateTempFile("/tmp", sqlFileContent, "sql")
	testutils.FatalIfError(t, err)

	stmts, err := queryparser.ParseSqlFile(tempFilePath)
	testutils.FatalIfError(t, err)

	transformer := NewTransformer()
	transformedStmts, err := transformer.MergeConstraints(stmts.Stmts)
	testutils.FatalIfError(t, err)

	finalSqlStmts, err := queryparser.DeparseRawStmts(transformedStmts)
	testutils.FatalIfError(t, err)

	testutils.AssertEqualStringSlices(t, expectedSqls, finalSqlStmts)
}

// In some case pg_query golang based pg parser removes default values from the sql statements
// above TestMergeConstraints_ExcludeConstraintType is one example; this function will just document such known cases
func Test_RemovalOfDefaultValuesByParser(t *testing.T) {

	sqlFileContent := `
		-- Case: Foreign Key Constraint with deferrability attributes default values
		ALTER TABLE inventory ADD CONSTRAINT inventory_product_id_fk FOREIGN KEY (product_id) REFERENCES products(product_id)
			ON DELETE NO ACTION
			NOT DEFERRABLE
			INITIALLY IMMEDIATE;

		-- Case: Foreign Key Constraint with deferrability attributes default values
		ALTER TABLE t1 ADD CONSTRAINT fk1 FOREIGN KEY (t2_id) REFERENCES t2 (id2)
			ON UPDATE NO ACTION
			ON DELETE NO ACTION
			DEFERRABLE INITIALLY DEFERRED;

		ALTER TABLE t1 ADD CONSTRAINT fk1 FOREIGN KEY (t2_id) REFERENCES t2 (id2);
	`

	expectedSqls := []string{
		`ALTER TABLE inventory ADD CONSTRAINT inventory_product_id_fk FOREIGN KEY (product_id) REFERENCES products (product_id);`,
		`ALTER TABLE t1 ADD CONSTRAINT fk1 FOREIGN KEY (t2_id) REFERENCES t2 (id2) DEFERRABLE INITIALLY DEFERRED;`,
		`ALTER TABLE t1 ADD CONSTRAINT fk1 FOREIGN KEY (t2_id) REFERENCES t2 (id2);`,
	}

	tempFilePath, err := testutils.CreateTempFile("/tmp", sqlFileContent, "sql")
	testutils.FatalIfError(t, err)

	stmts, err := queryparser.ParseSqlFile(tempFilePath)
	testutils.FatalIfError(t, err)

	transformer := NewTransformer()
	transformedStmts, err := transformer.MergeConstraints(stmts.Stmts)
	testutils.FatalIfError(t, err)

	finalSqlStmts, err := queryparser.DeparseRawStmts(transformedStmts)
	testutils.FatalIfError(t, err)

	testutils.AssertEqualStringSlices(t, expectedSqls, finalSqlStmts)
}
