//go:build issues_integration

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
package queryissue

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/modules/yugabytedb"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

var (
	testYugabytedbContainer *yugabytedb.Container
	testYugabytedbConnStr   string
	testYbVersion           *ybversion.YBVersion
)

func getConn() (*pgx.Conn, error) {
	ctx := context.Background()
	var connStr string
	var err error
	if testYugabytedbConnStr != "" {
		connStr = testYugabytedbConnStr
	} else {
		connStr, err = testYugabytedbContainer.YSQLConnectionString(ctx, "sslmode=disable")
		if err != nil {
			return nil, err
		}
	}

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// TODO maybe we don't need the version check for different error msgs, just check if any of the error msgs is found in the error
func assertErrorCorrectlyThrownForIssueForYBVersion(t *testing.T, execErr error, expectedError string, issue issue.Issue) {
	isFixed, err := issue.IsFixedIn(testYbVersion)
	testutils.FatalIfError(t, err)

	if isFixed {
		assert.NoError(t, execErr)
	} else {
		assert.ErrorContains(t, execErr, expectedError)
	}
}

func testStoredGeneratedFunctionsIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
		CREATE TABLE rectangles (
		id SERIAL PRIMARY KEY,
		length NUMERIC NOT NULL,
		width NUMERIC NOT NULL,
		area NUMERIC GENERATED ALWAYS AS (length * width) STORED
	)`)
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "syntax error", generatedColumnsIssue)
}

func testUnloggedTableIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, "CREATE UNLOGGED TABLE unlogged_table (a int)")

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "UNLOGGED database object not supported yet", unloggedTableIssue)
}

func testAlterTableAddPKOnPartitionIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
	CREATE TABLE orders2 (
	order_id bigint NOT NULL,
	order_date timestamp
	) PARTITION BY RANGE (order_date);
	ALTER TABLE orders2 ADD PRIMARY KEY (order_id,order_date)`)

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "changing primary key of a partitioned table is not yet implemented", alterTableAddPKOnPartitionIssue)
}

func testSetAttributeIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
	CREATE TABLE public.event_search (
    event_id text,
    room_id text,
    sender text,
    key text,
    vector tsvector,
    origin_server_ts bigint,
    stream_ordering bigint
	);
	ALTER TABLE ONLY public.event_search ALTER COLUMN room_id SET (n_distinct=-0.01)`)

	var errMsg string
	switch {
	case testYbVersion.ReleaseType() == ybversion.V2_25_0_0.ReleaseType() && testYbVersion.GreaterThanOrEqual(ybversion.V2_25_0_0),
		testYbVersion.ReleaseType() == ybversion.V2024_2_1_0.ReleaseType() && testYbVersion.GreaterThanOrEqual(ybversion.V2024_2_1_0):
		errMsg = `ALTER action ALTER COLUMN ... SET not supported yet`
	default:
		errMsg = "ALTER TABLE ALTER column not supported yet"
	}
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, errMsg, setColumnAttributeIssue)
}

func testClusterOnIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
	CREATE TABLE test(ID INT PRIMARY KEY NOT NULL,
	Name TEXT NOT NULL,
	Age INT NOT NULL,
	Address CHAR(50), 
	Salary REAL);

	CREATE UNIQUE INDEX test_age_salary ON public.test USING btree (age ASC NULLS LAST, salary ASC NULLS LAST);

	ALTER TABLE public.test CLUSTER ON test_age_salary`)

	var errMsg string
	switch {
	case testYbVersion.ReleaseType() == ybversion.V2_25_0_0.ReleaseType() && testYbVersion.GreaterThanOrEqual(ybversion.V2_25_0_0),
		testYbVersion.ReleaseType() == ybversion.V2024_2_1_0.ReleaseType() && testYbVersion.GreaterThanOrEqual(ybversion.V2024_2_1_0):
		errMsg = "ALTER action CLUSTER ON not supported yet"
	default:
		errMsg = "ALTER TABLE CLUSTER not supported yet"
	}
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, errMsg, alterTableClusterOnIssue)
}

func testDisableRuleIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
	create table trule (a int);

	create rule trule_rule as on update to trule do instead nothing;

	ALTER TABLE trule DISABLE RULE trule_rule`)

	var errMsg string
	switch {
	case testYbVersion.ReleaseType() == ybversion.V2_25_0_0.ReleaseType() && testYbVersion.GreaterThanOrEqual(ybversion.V2_25_0_0),
		testYbVersion.ReleaseType() == ybversion.V2024_2_1_0.ReleaseType() && testYbVersion.GreaterThanOrEqual(ybversion.V2024_2_1_0):
		errMsg = "ALTER action DISABLE RULE not supported yet"
	default:
		errMsg = "ALTER TABLE DISABLE RULE not supported yet"
	}
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, errMsg, alterTableDisableRuleIssue)
}

func testStorageParameterIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
	CREATE TABLE public.example (
	name         text,
	email        text,
	new_id       integer NOT NULL,
	id2          integer NOT NULL,
	CONSTRAINT example_name_check CHECK ((char_length(name) > 3))
	);

	ALTER TABLE ONLY public.example
	ADD CONSTRAINT example_email_key UNIQUE (email) WITH (fillfactor = 70);`)

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "unrecognized parameter", storageParameterIssue)
}

func testLoDatatypeIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
	CREATE TABLE image (title text, raster lo);`)

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "does not exist", loDatatypeIssue)
}

func testMultiRangeDatatypeIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	queries := []string{
		`CREATE TABLE int_multirange_table (
			id SERIAL PRIMARY KEY,
			value_ranges int4multirange
		);`,
		`CREATE TABLE bigint_multirange_table (
			id SERIAL PRIMARY KEY,
			value_ranges int8multirange
		);`,
		`CREATE TABLE numeric_multirange_table (
			id SERIAL PRIMARY KEY,
			price_ranges nummultirange
		);`,
		`CREATE TABLE timestamp_multirange_table (
			id SERIAL PRIMARY KEY,
			event_times tsmultirange
		);`,
		`CREATE TABLE timestamptz_multirange_table (
			id SERIAL PRIMARY KEY,
			global_event_times tstzmultirange
		);`,
		`CREATE TABLE date_multirange_table (
			id SERIAL PRIMARY KEY,
			project_dates datemultirange
		);`,
	}

	for _, query := range queries {
		_, err = conn.Exec(ctx, query)
		assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "does not exist", multiRangeDatatypeIssue)
	}
}

func testSecurityInvokerView(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
	CREATE TABLE public.employees (
		employee_id SERIAL PRIMARY KEY,
		first_name VARCHAR(100),
		last_name VARCHAR(100),
		department VARCHAR(50)
	);

	CREATE VIEW public.view_explicit_security_invoker
	WITH (security_invoker = true) AS
	SELECT employee_id, first_name
	FROM public.employees;`)

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "unrecognized parameter", securityInvokerViewIssue)
}

func testDeterministicCollationIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
	CREATE COLLATION case_insensitive (provider = icu, locale = 'und-u-ks-level2', deterministic = false);`)

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, `collation attribute "deterministic" not recognized`, deterministicOptionCollationIssue)
}

func testForeignKeyReferencesPartitionedTableIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
	CREATE TABLE abc1(id int PRIMARY KEY, val text) PARTITION BY RANGE (id);
	CREATE TABLE abc_fk(id int PRIMARY KEY, abc_id INT REFERENCES abc1(id), val text) ;`)

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, `cannot reference partitioned table "abc1"`, foreignKeyReferencesPartitionedTableIssue)
}

func testSQLBodyInFunctionIssue(t *testing.T) {
	sqls := map[string]string{
		`CREATE OR REPLACE FUNCTION asterisks(n int)
  RETURNS text
  LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
RETURN repeat('*', n);`: `syntax error at or near "RETURN"`,
		`CREATE OR REPLACE FUNCTION asterisks1(n int)
  RETURNS SETOF text
  LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
BEGIN ATOMIC
SELECT repeat('*', g) FROM generate_series (1, n) g;
END;`: `syntax error at or near "BEGIN"`,
	}
	for sql, errMsg := range sqls {
		ctx := context.Background()
		conn, err := getConn()
		assert.NoError(t, err)

		defer conn.Close(context.Background())
		_, err = conn.Exec(ctx, sql)
		assertErrorCorrectlyThrownForIssueForYBVersion(t, err, errMsg, sqlBodyInFunctionIssue)
	}
}

func testUniqueNullsNotDistinctIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `CREATE TABLE public.products (
    id INTEGER PRIMARY KEY,
    product_name VARCHAR(100),
    serial_number TEXT,
    UNIQUE NULLS NOT DISTINCT (product_name, serial_number)
	);`)

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "syntax error", uniqueNullsNotDistinctIssue)
}

func testBeforeRowTriggerOnPartitionedTable(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
CREATE TABLE sales_region (id int, amount int, branch text, region text, PRIMARY KEY(id, region)) PARTITION BY LIST (region);

CREATE OR REPLACE FUNCTION public.check_sales_region()
RETURNS TRIGGER AS $$
BEGIN

    IF NEW.amount < 0 THEN
        RAISE EXCEPTION 'Amount cannot be negative';
    END IF;

    IF NEW.branch IS NULL OR NEW.branch = '' THEN
        RAISE EXCEPTION 'Branch name cannot be null or empty';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER before_sales_region_insert_update
BEFORE INSERT OR UPDATE ON public.sales_region
FOR EACH ROW
EXECUTE FUNCTION public.check_sales_region();`)

	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, `"sales_region" is a partitioned table`, beforeRowTriggerOnPartitionTableIssue)
}

func testDatabaseOptions(t *testing.T) {
	sqlsforPG15 := []string{
		` CREATE DATABASE locale_example
    WITH LOCALE = 'en_US.UTF-8'
         TEMPLATE = template0;`,
		`CREATE DATABASE locale_provider_example
    WITH ICU_LOCALE = 'en_US'
         LOCALE_PROVIDER = 'icu'
         TEMPLATE = template0;`,
		`CREATE DATABASE oid_example
    WITH OID = 123456;`,
		`CREATE DATABASE collation_version_example
    WITH COLLATION_VERSION = '153.128';`,
		`CREATE DATABASE strategy_example
    WITH STRATEGY = 'wal_log';`,
	}
	sqlsForPG17 := []string{
		`CREATE DATABASE icu_rules_example
    WITH ICU_RULES = '&a < b < c';`,
		`CREATE DATABASE builtin_locale_example
    WITH BUILTIN_LOCALE = 'C';`,
	}

	for _, sql := range sqlsforPG15 {
		ctx := context.Background()
		conn, err := getConn()
		assert.NoError(t, err)

		defer conn.Close(context.Background())
		_, err = conn.Exec(ctx, sql)
		switch {
		case testYbVersion.ReleaseType() == ybversion.V2_25_0_0.ReleaseType() && testYbVersion.GreaterThanOrEqual(ybversion.V2_25_0_0):
			assert.NoError(t, err)
			//Database options works on pg15 but not supported actually and hence not marking this as supported
			assertErrorCorrectlyThrownForIssueForYBVersion(t, fmt.Errorf(""), "", databaseOptionsPG15Issue)
		default:
			assertErrorCorrectlyThrownForIssueForYBVersion(t, err, `not recognized`, databaseOptionsPG15Issue)
		}
	}
	for _, sql := range sqlsForPG17 {
		ctx := context.Background()
		conn, err := getConn()
		assert.NoError(t, err)

		defer conn.Close(context.Background())
		_, err = conn.Exec(ctx, sql)
		assertErrorCorrectlyThrownForIssueForYBVersion(t, err, `not recognized`, databaseOptionsPG17Issue)
	}

}

func testNonDeterministicCollationIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `
	CREATE COLLATION case_insensitive_accent_sensitive (
    provider = icu,
    locale = 'en-u-ks-level2',
    deterministic = false
);
CREATE TABLE collation_ex (
    id SERIAL PRIMARY KEY,
    name TEXT COLLATE case_insensitive_accent_sensitive
);
INSERT INTO collation_ex (name) VALUES
('André'),
('andre'),
('ANDRE'),
('Ándre'),
('andrÉ');
	;`)
	switch {
	case testYbVersion.ReleaseType() == ybversion.V2_25_0_0.ReleaseType() && testYbVersion.GreaterThanOrEqual(ybversion.V2_25_0_0):
		assert.NoError(t, err)
		rows, err := conn.Query(context.Background(), `SELECT name
FROM collation_ex
ORDER BY name;`)
		assert.NoError(t, err)

		var names []string

		for rows.Next() {
			var name string
			err := rows.Scan(&name)
			assert.NoError(t, err)
			names = append(names, name)
		}

		/*
			GH Issue for the support - https://github.com/yugabyte/yugabyte-db/issues/25541
			order of the name column is depending on non-deterministic collations, example is of case-insensitive and accent-sensitive collation
			and output is different from PG - which means functionality is not proper
			postgres=# SELECT name
				FROM collation_ex
				order by name;
				name
				-------
				andre
				ANDRE
				André
				andrÉ
				Ándre
				(5 rows)
		*/

		assert.Equal(t, []string{"andre", "ANDRE", "andrÉ", "André", "Ándre"}, names)

		assertErrorCorrectlyThrownForIssueForYBVersion(t, fmt.Errorf(""), "", nonDeterministicCollationIssue)
	default:
		assertErrorCorrectlyThrownForIssueForYBVersion(t, err, `collation attribute "deterministic" not recognized`, deterministicOptionCollationIssue)
	}

}

func testCompressionClauseIssue(t *testing.T) {
	ctx := context.Background()
	conn, err := getConn()
	assert.NoError(t, err)

	defer conn.Close(context.Background())

	_, err = conn.Exec(ctx, `CREATE TABLE tbl_comp1(id int, v text COMPRESSION pglz);`)
	//CREATE works on 2.25 without errors or warning but not supported actually

	var errMsg string
	switch {
	case testYbVersion.ReleaseType() == ybversion.V2_25_0_0.ReleaseType() && testYbVersion.GreaterThanOrEqual(ybversion.V2_25_0_0):
		assert.NoError(t, err)
		err = fmt.Errorf("")
		errMsg = ""
	default:
		errMsg = `syntax error at or near "COMPRESSION"`
	}
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, errMsg, compressionClauseForToasting)

	_, err = conn.Exec(ctx, `
	CREATE TABLE tbl_comp(id int, v text);
	ALTER TABLE ONLY public.tbl_comp ALTER COLUMN v SET COMPRESSION pglz;`)
	//ALTER not supported in 2.25
	switch {
	case testYbVersion.ReleaseType() == ybversion.V2_25_0_0.ReleaseType() && testYbVersion.GreaterThanOrEqual(ybversion.V2_25_0_0):
		errMsg = "This ALTER TABLE command is not yet supported."
	default:
		errMsg = `syntax error at or near "COMPRESSION"`
	}
	//TODO maybe we don't need the version check for different error msgs, just check if any of the error msgs is found in the error
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, errMsg, compressionClauseForToasting)

}

func TestDDLIssuesInYBVersion(t *testing.T) {
	var err error
	ybVersion := os.Getenv("YB_VERSION")
	if ybVersion == "" {
		panic("YB_VERSION env variable is not set. Set YB_VERSION=2024.1.3.0-b105 for example")
	}

	ybVersionWithoutBuild := strings.Split(ybVersion, "-")[0]
	testYbVersion, err = ybversion.NewYBVersion(ybVersionWithoutBuild)
	testutils.FatalIfError(t, err)

	testYugabytedbConnStr = os.Getenv("YB_CONN_STR")
	if testYugabytedbConnStr == "" {
		// spawn yugabytedb container
		var err error
		ctx := context.Background()
		testYugabytedbContainer, err = yugabytedb.Run(
			ctx,
			"yugabytedb/yugabyte:"+ybVersion,
		)
		assert.NoError(t, err)
		defer testYugabytedbContainer.Terminate(context.Background())
	}

	// run tests
	var success bool

	success = t.Run(fmt.Sprintf("%s-%s", "stored generated functions", ybVersion), testStoredGeneratedFunctionsIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "unlogged table", ybVersion), testUnloggedTableIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "alter table add PK on partition", ybVersion), testAlterTableAddPKOnPartitionIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "set attribute", ybVersion), testSetAttributeIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "cluster on", ybVersion), testClusterOnIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "disable rule", ybVersion), testDisableRuleIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "storage parameter", ybVersion), testStorageParameterIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "lo datatype", ybVersion), testLoDatatypeIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "multi range datatype", ybVersion), testMultiRangeDatatypeIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "security invoker view", ybVersion), testSecurityInvokerView)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "deterministic attribute in collation", ybVersion), testDeterministicCollationIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "non-deterministic collations", ybVersion), testNonDeterministicCollationIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "foreign key referenced partitioned table", ybVersion), testForeignKeyReferencesPartitionedTableIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "sql body in function", ybVersion), testSQLBodyInFunctionIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "unique nulls not distinct", ybVersion), testUniqueNullsNotDistinctIssue)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "before row triggers on partitioned table", ybVersion), testBeforeRowTriggerOnPartitionedTable)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "compression clause", ybVersion), testCompressionClauseIssue)
	assert.True(t, success)
	success = t.Run(fmt.Sprintf("%s-%s", "database options", ybVersion), testDatabaseOptions)
	assert.True(t, success)

}
