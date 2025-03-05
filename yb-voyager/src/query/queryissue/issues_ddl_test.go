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

	query := `CREATE TABLE int_multirange_table (
			id SERIAL PRIMARY KEY,
			value_ranges int4multirange
		);`
	_, err = conn.Exec(ctx, query)
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "does not exist", int4MultirangeDatatypeIssue)

	query = `CREATE TABLE bigint_multirange_table (
			id SERIAL PRIMARY KEY,
			value_ranges int8multirange
		);`
	_, err = conn.Exec(ctx, query)
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "does not exist", int8MultirangeDatatypeIssue)

	query = `CREATE TABLE numeric_multirange_table (
			id SERIAL PRIMARY KEY,
			price_ranges nummultirange
		);`
	_, err = conn.Exec(ctx, query)
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "does not exist", numMultirangeDatatypeIssue)

	query = `CREATE TABLE timestamp_multirange_table (
			id SERIAL PRIMARY KEY,
			event_times tsmultirange
		);`
	_, err = conn.Exec(ctx, query)
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "does not exist", tsMultirangeDatatypeIssue)

	query = `CREATE TABLE timestamptz_multirange_table (
			id SERIAL PRIMARY KEY,
			global_event_times tstzmultirange
		);`
	_, err = conn.Exec(ctx, query)
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "does not exist", tstzMultirangeDatatypeIssue)

	query = `CREATE TABLE date_multirange_table (
			id SERIAL PRIMARY KEY,
			project_dates datemultirange
		);`
	_, err = conn.Exec(ctx, query)
	assertErrorCorrectlyThrownForIssueForYBVersion(t, err, "does not exist", dateMultirangeDatatypeIssue)
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

func testIndexOnComplexDataType(t *testing.T) {
	// We have to create indexes on the tables to check if the index creation is supported or not
	// We will create indexes on the unsupported datatypes and check if the index creation fails

	type testIndexOnComplexDataTypeTests struct {
		sql             string
		errMsgBase      string
		errMsgv2_25_0_0 string
		Issue           issue.Issue
	}

	testCases := []testIndexOnComplexDataTypeTests{
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE citext_table (id int, name CITEXT);
			CREATE INDEX citext_index ON citext_table (name);`,
			errMsgBase:      "ERROR: type \"citext\" does not exist (SQLSTATE 42704)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnCitextDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE tsvector_table (id int, name TSVECTOR);
			CREATE INDEX tsvector_index ON tsvector_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'TSVECTOR' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnTsVectorDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE tsquery_table (id int, name TSQUERY);
			CREATE INDEX tsquery_index ON tsquery_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'TSQUERY' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnTsQueryDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE jsonb_table (id int, name JSONB);
			CREATE INDEX jsonb_index ON jsonb_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'JSONB' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnJsonbDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE inet_table (id int, name INET);
			CREATE INDEX inet_index ON inet_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'INET' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnInetDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE json_table (id int, name JSON);
			CREATE INDEX json_index ON json_table (name);`,
			errMsgBase:      "ERROR: data type json has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnJsonDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE macaddr_table (id int, name MACADDR);
			CREATE INDEX macaddr_index ON macaddr_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'MACADDR' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnMacaddrDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE macaddr8_table (id int, name MACADDR8);
			CREATE INDEX macaddr8_index ON macaddr8_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'MACADDR8' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnMacaddr8DatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE cidr_table (id int, name CIDR);
			CREATE INDEX cidr_index ON cidr_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'CIDR' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnCidrDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE bit_table (id int, name BIT(10));
			CREATE INDEX bit_index ON bit_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'BIT' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnBitDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE varbit_table (id int, name VARBIT(10));
			CREATE INDEX varbit_index ON varbit_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'VARBIT' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnVarbitDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE daterange_table (id int, name DATERANGE);
			CREATE INDEX daterange_index ON daterange_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'user_defined_type' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "ERROR: INDEX on column of type 'DATERANGE' not yet supported (SQLSTATE 0A000)",
			Issue:           indexOnDaterangeDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE tsrange_table (id int, name TSRANGE);
			CREATE INDEX tsrange_index ON tsrange_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'user_defined_type' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "ERROR: INDEX on column of type 'TSRANGE' not yet supported (SQLSTATE 0A000)",
			Issue:           indexOnTsrangeDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE tstzrange_table (id int, name TSTZRANGE);
			CREATE INDEX tstzrange_index ON tstzrange_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'user_defined_type' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "ERROR: INDEX on column of type 'TSTZRANGE' not yet supported (SQLSTATE 0A000)",
			Issue:           indexOnTstzrangeDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE numrange_table (id int, name NUMRANGE);
			CREATE INDEX numrange_index ON numrange_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'user_defined_type' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "ERROR: INDEX on column of type 'NUMRANGE' not yet supported (SQLSTATE 0A000)",
			Issue:           indexOnNumrangeDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE int4range_table (id int, name INT4RANGE);
			CREATE INDEX int4range_index ON int4range_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'INT4RANGE' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnInt4rangeDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE int8range_table (id int, name INT8RANGE);
			CREATE INDEX int8range_index ON int8range_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'user_defined_type' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "ERROR: INDEX on column of type 'INT8RANGE' not yet supported (SQLSTATE 0A000)",
			Issue:           indexOnInt8rangeDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE interval_table (id int, name INTERVAL);
			CREATE INDEX interval_index ON interval_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'INTERVAL' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnIntervalDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE circle_table (id int, name CIRCLE);
			CREATE INDEX circle_index ON circle_table (name);`,
			errMsgBase:      "ERROR: data type circle has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnCircleDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE box_table (id int, name BOX);
			CREATE INDEX box_index ON box_table (name);`,
			errMsgBase:      "ERROR: data type box has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnBoxDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE line_table (id int, name LINE);
			CREATE INDEX line_index ON line_table (name);`,
			errMsgBase:      "ERROR: data type line has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnLineDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE lseg_table (id int, name LSEG);
			CREATE INDEX lseg_index ON lseg_table (name);`,
			errMsgBase:      "ERROR: data type lseg has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnLsegDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE point_table (id int, name POINT);
			CREATE INDEX point_index ON point_table (name);`,
			errMsgBase:      "ERROR: data type point has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnPointDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE path_table (id int, name PATH);
			CREATE INDEX path_index ON path_table (name);`,
			errMsgBase:      "ERROR: data type path has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnPathDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE polygon_table (id int, name POLYGON);
			CREATE INDEX polygon_index ON polygon_table (name);`,
			errMsgBase:      "ERROR: data type polygon has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnPolygonDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE txid_snapshot_table (id int, name TXID_SNAPSHOT);
			CREATE INDEX txid_snapshot_index ON txid_snapshot_table (name);`,
			errMsgBase:      "ERROR: data type txid_snapshot has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnTxidSnapshotDatatypeIssue,
		},
		// One with array datatype
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE array_table (id int, name int[]);
			CREATE INDEX array_index ON array_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'INT4ARRAY' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnArrayDatatypeIssue,
		},
		// One with UDT datatype
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TYPE my_udt AS (a int, b text);
			CREATE TABLE udt_table (id int, name my_udt);
			CREATE INDEX udt_index ON udt_table (name);`,
			errMsgBase:      "ERROR: INDEX on column of type 'user_defined_type' not yet supported (SQLSTATE 0A000)",
			errMsgv2_25_0_0: "",
			Issue:           indexOnUserDefinedDatatypeIssue,
		},
		testIndexOnComplexDataTypeTests{
			sql: `CREATE TABLE pg_lsn_table (id int, name PG_LSN);
			CREATE INDEX pg_lsn_index ON pg_lsn_table (name);`,
			errMsgBase:      "", // It is possible to create index on pg_lsn datatype but operations on it throw error in YB
			errMsgv2_25_0_0: "",
			Issue:           indexOnPgLsnDatatypeIssue,
		},
	}

	for _, testCase := range testCases {
		ctx := context.Background()
		conn, err := getConn()
		assert.NoError(t, err)

		var errMsg string
		if testYbVersion.ReleaseType() == ybversion.V2_25_0_0.ReleaseType() && testYbVersion.GreaterThanOrEqual(ybversion.V2_25_0_0) && testCase.errMsgv2_25_0_0 != "" {
			errMsg = testCase.errMsgv2_25_0_0
		} else {
			errMsg = testCase.errMsgBase
		}

		_, err = conn.Exec(ctx, testCase.sql)
		if errMsg == "" { // For the pg_lsn datatype, we are not throwing any error but the operations on it throw error
			assert.NoError(t, err)
			assertErrorCorrectlyThrownForIssueForYBVersion(t, fmt.Errorf(""), errMsg, testCase.Issue)
		} else {
			assertErrorCorrectlyThrownForIssueForYBVersion(t, err, errMsg, testCase.Issue)
		}

		conn.Close(ctx)
	}
}

func testPKandUKONComplexDataType(t *testing.T) {
	// var UnsupportedIndexDatatypes = []string{
	// 	"citext",
	// 	"tsvector",
	// 	"tsquery",
	// 	"jsonb",
	// 	"inet",
	// 	"json",
	// 	"macaddr",
	// 	"macaddr8",
	// 	"cidr",
	// 	"bit",    // for BIT (n)
	// 	"varbit", // for BIT varying (n)
	// 	"daterange",
	// 	"tsrange",
	// 	"tstzrange",
	// 	"numrange",
	// 	"int4range",
	// 	"int8range",
	// 	"interval", // same for INTERVAL YEAR TO MONTH and INTERVAL DAY TO SECOND
	// 	//Below ones are not supported on PG as well with atleast btree access method. Better to have in our list though
	// 	//Need to understand if there is other method or way available in PG to have these index key [TODO]
	// 	"circle",
	// 	"box",
	// 	"line",
	// 	"lseg",
	// 	"point",
	// 	"pg_lsn",
	// 	"path",
	// 	"polygon",
	// 	"txid_snapshot",
	// 	// array as well but no need to add it in the list as fetching this type is a different way TODO: handle better with specific types
	// }

	type testPKandUKOnComplexDataTypeTests struct {
		sql        string
		errMsgBase string
		Issue      issue.Issue
	}

	testCases := []testPKandUKOnComplexDataTypeTests{
		{
			sql:        `CREATE TABLE citext_table_pk (id int, name CITEXT, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: type \"citext\" does not exist (SQLSTATE 42704)",
			Issue:      primaryOrUniqueConstraintOnCitextDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE tsvector_table_pk (id int, name TSVECTOR, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'TSVECTOR' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnTsVectorDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE tsquery_table_pk (id int, name TSQUERY, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'TSQUERY' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnTsQueryDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE jsonb_table_pk (id int, name JSONB, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'JSONB' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnJsonbDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE inet_table_pk (id int, name INET, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'INET' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnInetDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE json_table_pk (id int, name JSON, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'JSON' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnJsonDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE macaddr_table_pk (id int, name MACADDR, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'MACADDR' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnMacaddrDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE macaddr8_table_pk (id int, name MACADDR8, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'MACADDR8' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnMacaddr8DatatypeIssue,
		},
		{
			sql:        `CREATE TABLE cidr_table_pk (id int, name CIDR, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'CIDR' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnCidrDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE bit_table_pk (id int, name BIT(10), PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'BIT' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnBitDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE varbit_table_pk (id int, name VARBIT(10), PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'VARBIT' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnVarbitDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE daterange_table_pk (id int, name DATERANGE, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'DATERANGE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnDaterangeDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE tsrange_table_pk (id int, name TSRANGE, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'TSRANGE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnTsrangeDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE tstzrange_table_pk (id int, name TSTZRANGE, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'TSTZRANGE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnTstzrangeDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE numrange_table_pk (id int, name NUMRANGE, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'NUMRANGE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnNumrangeDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE int4range_table_pk (id int, name INT4RANGE, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'INT4RANGE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnInt4rangeDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE int8range_table_pk (id int, name INT8RANGE, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'INT8RANGE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnInt8rangeDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE interval_table_pk (id int, name INTERVAL, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'INTERVAL' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnIntervalDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE circle_table_pk (id int, name CIRCLE, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'CIRCLE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnCircleDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE box_table_pk (id int, name BOX, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'BOX' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnBoxDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE line_table_pk (id int, name LINE, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'LINE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnLineDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE lseg_table_pk (id int, name LSEG, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'LSEG' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnLsegDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE point_table_pk (id int, name POINT, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'POINT' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnPointDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE path_table_pk (id int, name PATH, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'PATH' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnPathDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE polygon_table_pk (id int, name POLYGON, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'POLYGON' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnPolygonDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE txid_snapshot_table_pk (id int, name TXID_SNAPSHOT, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'TXID_SNAPSHOT' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnTxidSnapshotDatatypeIssue,
		},
		{
			sql: `CREATE TYPE my_udt_pk AS (a int, b text);
			CREATE TABLE udt_table_pk (id int, name my_udt_pk, PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'user_defined_type' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnUserDefinedTypeIssue,
		},
		{
			sql:        `CREATE TABLE pg_lsn_table_pk (id int, name PG_LSN, PRIMARY KEY (name));`,
			errMsgBase: "", // It is possible to create PK on pg_lsn datatype but operations on it throw error in YB
			Issue:      primaryOrUniqueConstraintOnPgLsnDatatypeIssue,
		},
		// One for array datatype
		{
			sql:        `CREATE TABLE array_table_pk (id int, name int[], PRIMARY KEY (name));`,
			errMsgBase: "ERROR: PRIMARY KEY containing column of type 'INT4ARRAY' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnArrayDatatypeIssue,
		},

		// Now Unique Key
		{
			sql:        `CREATE TABLE citext_table_uk (id int, name CITEXT, UNIQUE (name));`,
			errMsgBase: "ERROR: type \"citext\" does not exist (SQLSTATE 42704)",
			Issue:      primaryOrUniqueConstraintOnCitextDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE tsvector_table_uk (id int, name TSVECTOR, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'TSVECTOR' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnTsVectorDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE tsquery_table_uk (id int, name TSQUERY, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'TSQUERY' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnTsQueryDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE jsonb_table_uk (id int, name JSONB, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'JSONB' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnJsonbDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE inet_table_uk (id int, name INET, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'INET' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnInetDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE json_table_uk (id int, name JSON, UNIQUE (name));`,
			errMsgBase: "ERROR: data type json has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			Issue:      primaryOrUniqueConstraintOnJsonDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE macaddr_table_uk (id int, name MACADDR, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'MACADDR' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnMacaddrDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE macaddr8_table_uk (id int, name MACADDR8, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'MACADDR8' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnMacaddr8DatatypeIssue,
		},
		{
			sql:        `CREATE TABLE cidr_table_uk (id int, name CIDR, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'CIDR' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnCidrDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE bit_table_uk (id int, name BIT(10), UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'BIT' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnBitDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE varbit_table_uk (id int, name VARBIT(10), UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'VARBIT' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnVarbitDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE daterange_table_uk (id int, name DATERANGE, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'DATERANGE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnDaterangeDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE tsrange_table_uk (id int, name TSRANGE, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'TSRANGE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnTsrangeDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE tstzrange_table_uk (id int, name TSTZRANGE, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'TSTZRANGE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnTstzrangeDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE numrange_table_uk (id int, name NUMRANGE, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'NUMRANGE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnNumrangeDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE int4range_table_uk (id int, name INT4RANGE, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'INT4RANGE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnInt4rangeDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE int8range_table_uk (id int, name INT8RANGE, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'INT8RANGE' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnInt8rangeDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE interval_table_uk (id int, name INTERVAL, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'INTERVAL' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnIntervalDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE circle_table_uk (id int, name CIRCLE, UNIQUE (name));`,
			errMsgBase: "ERROR: data type circle has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			Issue:      primaryOrUniqueConstraintOnCircleDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE box_table_uk (id int, name BOX, UNIQUE (name));`,
			errMsgBase: "ERROR: data type box has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			Issue:      primaryOrUniqueConstraintOnBoxDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE line_table_uk (id int, name LINE, UNIQUE (name));`,
			errMsgBase: "ERROR: data type line has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			Issue:      primaryOrUniqueConstraintOnLineDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE lseg_table_uk (id int, name LSEG, UNIQUE (name));`,
			errMsgBase: "ERROR: data type lseg has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			Issue:      primaryOrUniqueConstraintOnLsegDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE point_table_uk (id int, name POINT, UNIQUE (name));`,
			errMsgBase: "ERROR: data type point has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			Issue:      primaryOrUniqueConstraintOnPointDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE path_table_uk (id int, name PATH, UNIQUE (name));`,
			errMsgBase: "ERROR: data type path has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			Issue:      primaryOrUniqueConstraintOnPathDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE polygon_table_uk (id int, name POLYGON, UNIQUE (name));`,
			errMsgBase: "ERROR: data type polygon has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			Issue:      primaryOrUniqueConstraintOnPolygonDatatypeIssue,
		},
		{
			sql:        `CREATE TABLE txid_snapshot_table_uk (id int, name TXID_SNAPSHOT, UNIQUE (name));`,
			errMsgBase: "ERROR: data type txid_snapshot has no default operator class for access method \"lsm\" (SQLSTATE 42704)",
			Issue:      primaryOrUniqueConstraintOnTxidSnapshotDatatypeIssue,
		},
		{
			sql: `CREATE TYPE my_udt_uk AS (a int, b text);
			CREATE TABLE udt_table_uk (id int, name my_udt_uk, UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'user_defined_type' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnUserDefinedTypeIssue,
		},
		{
			sql:        `CREATE TABLE pg_lsn_table_uk (id int, name PG_LSN, UNIQUE (name));`,
			errMsgBase: "", // It is possible to create UK on pg_lsn datatype but operations on it throw error in YB
			Issue:      primaryOrUniqueConstraintOnPgLsnDatatypeIssue,
		},
		// One for array datatype
		{
			sql:        `CREATE TABLE array_table_uk (id int, name int[], UNIQUE (name));`,
			errMsgBase: "ERROR: INDEX on column of type 'INT4ARRAY' not yet supported (SQLSTATE 0A000)",
			Issue:      primaryOrUniqueConstraintOnArrayDatatypeIssue,
		},
	}

	for _, testCase := range testCases {
		ctx := context.Background()
		conn, err := getConn()
		assert.NoError(t, err)

		var errMsg = testCase.errMsgBase
		_, err = conn.Exec(ctx, testCase.sql)
		if errMsg == "" { // For the pg_lsn datatype, we are not throwing any error but the operations on it throw error
			assert.NoError(t, err)
			assertErrorCorrectlyThrownForIssueForYBVersion(t, fmt.Errorf(""), errMsg, testCase.Issue)
		} else {
			assertErrorCorrectlyThrownForIssueForYBVersion(t, err, errMsg, testCase.Issue)
		}

		conn.Close(ctx)
	}
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

	success = t.Run(fmt.Sprintf("%s-%s", "index on complex data type", ybVersion), testIndexOnComplexDataType)
	assert.True(t, success)

	success = t.Run(fmt.Sprintf("%s-%s", "PK and UK on complex data type", ybVersion), testPKandUKONComplexDataType)
	assert.True(t, success)

}
