//go:build integration_live_migration

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
package testlivemigration

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestLiveMigrationWithUniqueKeyConflictWithUniqueIndexOnlyOnLeafPartitions(t *testing.T) {
	t.Parallel()
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test9",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test9",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_partitions (
				id int,
				name TEXT,
				region TEXT,
				branch TEXT,
				PRIMARY KEY(id, region)
			) PARTITION BY LIST (region);

			CREATE TABLE test_schema.test_partitions_part1 PARTITION OF test_schema.test_partitions FOR VALUES IN ('London');
			CREATE TABLE test_schema.test_partitions_part2 PARTITION OF test_schema.test_partitions FOR VALUES IN ('Sydney');
			CREATE TABLE test_schema.test_partitions_part3 PARTITION OF test_schema.test_partitions FOR VALUES IN ('Boston');
			CREATE UNIQUE INDEX idx_1 ON test_schema.test_partitions_part1 (branch); -- This is the unique index only on part1
			CREATE UNIQUE INDEX idx_2 ON test_schema.test_partitions_part2 (branch); -- This is the unique index only on part2
			CREATE UNIQUE INDEX idx_3 ON test_schema.test_partitions_part3 (branch); -- This is the unique index only on part3`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_partitions (id, name, region, branch)
	SELECT i, md5(random()::text), CASE WHEN i%3=1 THEN 'London' WHEN i%3=2 THEN 'Sydney' ELSE 'Boston' END, 'Branch ' || i FROM generate_series(1, 20) as i;`,
		},
		SourceSetupSchemaSQL: []string{
			"ALTER TABLE test_schema.test_partitions REPLICA IDENTITY FULL;",
			"ALTER TABLE test_schema.test_partitions_part1 REPLICA IDENTITY FULL;",
			"ALTER TABLE test_schema.test_partitions_part2 REPLICA IDENTITY FULL;",
			"ALTER TABLE test_schema.test_partitions_part3 REPLICA IDENTITY FULL;",
		},
		SourceDeltaSQL: []string{
			/*
				conflict events
				1 London Branch1
				2 Sydney Branch2
				3 Boston Branch3
				...
				20 Sydney Branch20
				i=21
				UI conflict
				U 20 Sydney Branch20->Branch 21
				I 21 Boston Branch20

				U 21 Boston Branch20->Branch 521
				UU conflict
				U 20 Sydney Branch21->Branch 20
				U 21 Boston Branch521->Branch 21

				DU conflict
				D 20 Sydney Branch20
				U 21 Boston Branch21->Branch 20

				DI conflict
				D 21 Boston Branch21
				I 20 Sydney Branch 21

				U 20 Sydney Branch21->Branch 20
				I 21 Boston Branch20->Branch 21

				..so on since the branch is same for all the events it will be conflict with each other
			*/
			`
		DO $$
		DECLARE
		i INTEGER;
		BEGIN
			FOR i IN 21..520 LOOP
				UPDATE test_schema.test_partitions SET branch = 'Branch ' || i WHERE id = i - 1;
				INSERT INTO test_schema.test_partitions(id, name, region, branch)
				SELECT i, md5(random()::text), 'London', 'Branch ' || i-1;
		
				UPDATE test_schema.test_partitions SET branch = 'Branch ' || i+500 WHERE id = i;
		
				UPDATE test_schema.test_partitions SET branch = 'Branch ' || i-1 WHERE id = i - 1;
				UPDATE test_schema.test_partitions SET branch = 'Branch ' || i WHERE id = i;
		
				DELETE FROM test_schema.test_partitions WHERE id = i-1;
				UPDATE test_schema.test_partitions SET branch = 'Branch ' || i-1 WHERE id = i;
		
				DELETE FROM test_schema.test_partitions WHERE id = i;
				INSERT INTO test_schema.test_partitions(id, name, region, branch)
				SELECT i-1, md5(random()::text), 'London', 'Branch ' || i;
		
				UPDATE test_schema.test_partitions SET branch = 'Branch ' || i-1 WHERE id = i - 1;
				INSERT INTO test_schema.test_partitions(id, name, region, branch)
				SELECT i, md5(random()::text), 'London', 'Branch ' || i;
		
			END LOOP;
		END $$;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})

	defer liveMigrationTest.Cleanup()

	err := liveMigrationTest.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = liveMigrationTest.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = liveMigrationTest.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = liveMigrationTest.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	err = liveMigrationTest.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_partitions"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`"test_schema"."test_partitions"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = liveMigrationTest.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_partitions"`: {
			Inserts: 1500,
			Updates: 3000,
			Deletes: 1000,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	//streaming events 10000 events
	err = liveMigrationTest.ValidateDataConsistency([]string{`"test_schema"."test_partitions"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	// Perform cutover
	err = liveMigrationTest.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = liveMigrationTest.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}

func TestLiveMigrationWithUniqueKeyConflictWithExpressionIndexOnPartitions(t *testing.T) {
	t.Parallel()
	liveMigrationTest := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test11",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test11",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;
			CREATE TABLE test_schema.test_partitions(
		id int,
		region text,
		created_at date,
		email text,
		username text,
		status text,
		PRIMARY KEY(id, region)
	) PARTITION BY LIST (region);
	 
	CREATE TABLE test_schema.test_partitions_l PARTITION OF test_schema.test_partitions FOR VALUES IN ('London');
	CREATE TABLE test_schema.test_partitions_s PARTITION OF test_schema.test_partitions FOR VALUES IN ('Sydney');
	CREATE TABLE test_schema.test_partitions_b PARTITION OF test_schema.test_partitions FOR VALUES IN ('Boston');
	CREATE TABLE test_schema.test_partitions_t PARTITION OF test_schema.test_partitions FOR VALUES IN ('Tokyo');
	
	CREATE UNIQUE INDEX idx_test_partitions_email_l ON test_schema.test_partitions_l (lower(email));
	CREATE UNIQUE INDEX idx_test_partitions_email_s ON test_schema.test_partitions_s (lower(email));
	CREATE UNIQUE INDEX idx_test_partitions_email_b ON test_schema.test_partitions_b (lower(email));
	CREATE UNIQUE INDEX idx_test_partitions_email_t ON test_schema.test_partitions_t (lower(email));
	CREATE UNIQUE INDEX idx_test_expression_index_partitions_username_t ON test_schema.test_partitions_t (upper(username));`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_partitions REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_partitions_l REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_partitions_s REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_partitions_b REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.test_partitions_t REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_partitions (id, region, email, username, created_at, status)
	SELECT i, 
		CASE 
			WHEN i%4 = 0 THEN 'London'
			WHEN i%4 = 1 THEN 'Sydney'
			WHEN i%4 = 2 THEN 'Boston'
			ELSE 'Tokyo'
		END,
		'email_' || i || '@example.com',
		'user_' || i,
		now() + (i || ' days')::interval,
		CASE WHEN i%2 = 0 THEN 'active' ELSE 'inactive' END
	FROM generate_series(1, 20) as i;`,
		},
		SourceDeltaSQL: []string{
			/*
				1  Sydney email_1@example.com user_1 2021-01-01 active
				2  Boston email_2@example.com user_2 2021-01-02 active
				...
				20 London email_20@example.com user_20 2021-01-20 active


				changes
				UI
				U 20 email_20@example.com -> Email_21@example.com
				I 21 email_20@example.com user_21 2021-01-21 active

				UU
				U 21 email_20@example.com -> Email_521@example.com
				U 20 Email_21@example.com -> Email_20@example.com

				DU
				D 20 Email_20@example.com
				U 21 Email_521@example.com -> email_20@example.com

				DI
				D 21 email_20@example.com
				I 20 Email_20@example.com user_20 2021-01-20 active

				U 20 email_20@example.com -> Email_21@example.com
				I 21 email_20@example.com user_21 2021-01-21 active

			*/
			`DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 21..520 LOOP
        UPDATE test_schema.test_partitions SET email = 'Email_' || i || '@example.com' WHERE id = i - 1;
		INSERT INTO test_schema.test_partitions(id, region, email, username, created_at, status) VALUES 
		(i, 'Sydney', 'email_' || 20 || '@example.com', 'user_' || i, now() + (i || ' days')::interval, 'active');

		UPDATE test_schema.test_partitions SET email = 'Email_' || 500+i || '@example.com' WHERE id = i;
		UPDATE test_schema.test_partitions SET email = 'Email_' || 20 || '@example.com' WHERE id = i - 1;

		DELETE FROM test_schema.test_partitions WHERE id = i-1;
		UPDATE test_schema.test_partitions SET email = 'email_' || 20 || '@example.com' WHERE id = i;

		DELETE FROM test_schema.test_partitions WHERE id = i;
		INSERT INTO test_schema.test_partitions(id, region, email, username, created_at, status) VALUES 
		(i-1, 'London', 'Email_' || 20 || '@example.com', 'user_' || i-1, now() + ((i-1) || ' days')::interval, 'active');

		UPDATE test_schema.test_partitions SET email = 'Email_' || i || '@example.com' WHERE id = i - 1;
		INSERT INTO test_schema.test_partitions(id, region, email, username, created_at, status) VALUES 
		(i, 'Sydney', 'email_' || 20 || '@example.com', 'user_' || i, now() + (i || ' days')::interval, 'active');

    END LOOP;
END $$;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})

	defer liveMigrationTest.Cleanup()

	err := liveMigrationTest.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = liveMigrationTest.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = liveMigrationTest.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = liveMigrationTest.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	err = liveMigrationTest.WaitForSnapshotComplete(map[string]int64{
		`"test_schema"."test_partitions"`: 20,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`"test_schema"."test_partitions"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = liveMigrationTest.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = liveMigrationTest.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"test_schema"."test_partitions"`: {
			Inserts: 1500,
			Updates: 2500,
			Deletes: 1000,
		},
	}, 120, 5)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = liveMigrationTest.ValidateDataConsistency([]string{`"test_schema"."test_partitions"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	// Perform cutover
	err = liveMigrationTest.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = liveMigrationTest.WaitForCutoverComplete(0, 50)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

}

func getLiveMigrationTestBasicForPartitionedTableWithChildPK(t *testing.T, databaseName string) *LiveMigrationTest {
	return NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: databaseName,
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: databaseName,
		},
		SchemaNames: []string{"public"},
		SchemaSQL: []string{
			// Create partitioned table with NO PRIMARY KEY on root
			`CREATE TABLE public.orders (
				id SERIAL,
				region TEXT NOT NULL,
				amount bigint
			) PARTITION BY LIST (region);`,

			// Create child partitions WITH primary keys
			`CREATE TABLE public.orders_us PARTITION OF public.orders FOR VALUES IN ('US');`,
			`ALTER TABLE public.orders_us ADD PRIMARY KEY (id);`,

			`CREATE TABLE public.orders_eu PARTITION OF public.orders FOR VALUES IN ('EU');`,
			`ALTER TABLE public.orders_eu ADD PRIMARY KEY (id);`,

			`CREATE TABLE public.orders_apac PARTITION OF public.orders FOR VALUES IN ('APAC');`,
			`ALTER TABLE public.orders_apac ADD PRIMARY KEY (id);`,
		},
		SourceSetupSchemaSQL: []string{
			// Set replica identity for CDC on root and child partitions
			`ALTER TABLE public.orders REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.orders_us REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.orders_eu REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.orders_apac REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			// Insert initial data into different partitions
			`INSERT INTO public.orders (id, region, amount) VALUES (1, 'US', 100);`,
			`INSERT INTO public.orders (id, region, amount) VALUES (2, 'US', 200);`,
			`INSERT INTO public.orders (id, region, amount) VALUES (3, 'EU', 150);`,
			`INSERT INTO public.orders (id, region, amount) VALUES (4, 'EU', 250);`,
			`INSERT INTO public.orders (id, region, amount) VALUES (5, 'APAC', 300);`,
		},
		SourceDeltaSQL: []string{
			// CDC operations on different partitions
			`
			DO $$
			DECLARE
			BEGIN
				FOR i IN 1..100 LOOP
					INSERT INTO public.orders (id, region, amount) VALUES (i+5, 'US', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+105, 'EU', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+205, 'APAC', i * 100);

					UPDATE public.orders SET amount = amount + 1 WHERE id = i+5;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+105;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+205;

					IF i % 2 = 0 THEN
						DELETE FROM public.orders WHERE id = i+5;
					END IF;
				END LOOP;
			END $$;`,
		},
		TargetDeltaSQL: []string{
			`
			DO $$
			DECLARE
			BEGIN
				FOR i IN 301..400 LOOP
					INSERT INTO public.orders (id, region, amount) VALUES (i+5, 'US', i * 100);	
					INSERT INTO public.orders (id, region, amount) VALUES (i+105, 'EU', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+205, 'APAC', i * 100);

					UPDATE public.orders SET amount = amount + 1 WHERE id = i+5;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+105;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+205;

					IF i % 2 = 0 THEN
						DELETE FROM public.orders WHERE id = i+5;
					END IF;
				END LOOP;
			END $$;`,
		},
		CleanupSQL: []string{
			`DROP TABLE IF EXISTS public.orders CASCADE;`,
		},
	})
}

// TestLiveMigrationPartitionedTableWithChildPK tests live migration with partitioned tables
// where the root table has no primary key but child partitions do.
// This uses the '--use-partition-root false' flag to insert directly into child partitions.
//
// Schema:
//   - orders: root table partitioned by LIST on region, NO PRIMARY KEY
//   - orders_us: partition for 'US' with PRIMARY KEY (id)
//   - orders_eu: partition for 'EU' with PRIMARY KEY (id)
//   - orders_apac: partition for 'APAC' with PRIMARY KEY (id)
/*
INSERT events in all partitions
UPDATE partitions
	- change amount of the partitions
DELETE rows for partition
*/
func TestLiveMigrationPartitionedTableWithChildPK(t *testing.T) {
	t.Parallel()

	lm := getLiveMigrationTestBasicForPartitionedTableWithChildPK(t, "test_partition_with_child_pk")

	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	// This relaxes the PK check for root table since all child partitions have PKs
	// CDC events will contain partition_table_name for original partition
	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	// Start import with '--use-partition-root false'
	// This causes SQL to target partition tables using partition_table_name from events
	err = lm.StartImportData(true, map[string]string{
		"--use-partition-root": "false",
	})
	testutils.FatalIfError(t, err, "failed to start import data")

	// Wait for snapshot to complete - 5 initial rows
	// Snapshot data is tracked by root table name (standard behavior)
	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"public"."orders"`: 5,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ValidateDataConsistency([]string{`"public"."orders"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	// Execute CDC operations
	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	// Wait for streaming to complete
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"public"."orders"`: {
			Inserts: 300,
			Updates: 300,
			Deletes: 50,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	// Validate data consistency after CDC
	err = lm.ValidateDataConsistency([]string{`"public"."orders"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	// Perform cutover
	err = lm.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	err = lm.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		`"public"."orders"`: {
			Inserts: 300,
			Updates: 300,
			Deletes: 50,
		},
	}, 60, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = lm.ValidateDataConsistency([]string{`"public"."orders"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = lm.WaitForCutoverSourceComplete(0, 160)
	testutils.FatalIfError(t, err, "failed to wait for cutover source complete")

	err = lm.ValidateDataConsistency([]string{`"public"."orders"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

}

/*
Test schema:
Root table: public.customers
Child tables: public.customers_active, public.customers_other PK(id)
Child tables of active: public.customers_arr_small,public.customers_arr_large,
Child tables of arr_small: public.customers_part11 PK(id), public.customers_part12 PK(id)
Child tables of arr_large: public.customers_part21 PK(id), public.customers_part22 PK(id)

loading data using use-partition-root false

INSERT events in all customer partitions 400
UPDATE customers
  - change email of the customers 400
  - change status (partition key) for the partitions to change the partitions that gives out a DELETE (row with id and partition key) and INSERT (row with same id and new partition key) event
  - DELETE - 400
  - INSERT - 400

DELETE rows for partition - 100
*/
func TestLiveMigrationWithMultiLevelPartitioningWithChildTablesHasPK(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_multi_level_partitioning_with_child_tables_has_pk",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_multi_level_partitioning_with_child_tables_has_pk",
		},
		SchemaNames: []string{"public"},
		SchemaSQL: []string{
			//create a table with multi level partitioning
			`CREATE TABLE public.customers(
				id SERIAL,
				name TEXT NOT NULL,
				email TEXT NOT NULL,
				status TEXT NOT NULL,
				arr NUMERIC NOT NULL
			) PARTITION BY LIST (status);`,
			`CREATE TABLE public.customers_active PARTITION OF public.customers FOR VALUES IN ('ACTIVE', 'RECURRING','REACTIVATED') PARTITION BY RANGE(arr);`,
			`CREATE TABLE public.customers_other PARTITION OF public.customers DEFAULT;`,
			`CREATE TABLE public.customers_arr_small PARTITION OF public.customers_active FOR VALUES FROM (MINVALUE) TO (101) PARTITION BY HASH(id);`,
			`CREATE TABLE public.customers_part11 PARTITION OF public.customers_arr_small FOR VALUES WITH (modulus 2, remainder 0);`,
			`CREATE TABLE public.customers_part12 PARTITION OF public.customers_arr_small FOR VALUES WITH (modulus 2, remainder 1);`,
			`CREATE TABLE public.customers_arr_large PARTITION OF public.customers_active FOR VALUES FROM (101) TO (MAXVALUE) PARTITION BY HASH(id);`,
			`CREATE TABLE public.customers_part21 PARTITION OF public.customers_arr_large FOR VALUES WITH (modulus 2, remainder 0);`,
			`CREATE TABLE public.customers_part22 PARTITION OF public.customers_arr_large FOR VALUES WITH (modulus 2, remainder 1);`,
			`ALTER TABLE public.customers_other ADD PRIMARY KEY (id);`,
			`ALTER TABLE public.customers_part11 ADD PRIMARY KEY (id);`,
			`ALTER TABLE public.customers_part12 ADD PRIMARY KEY (id);`,
			`ALTER TABLE public.customers_part21 ADD PRIMARY KEY (id);`,
			`ALTER TABLE public.customers_part22 ADD PRIMARY KEY (id);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE public.customers REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.customers_other REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.customers_active REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.customers_arr_small REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.customers_part11 REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.customers_part12 REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.customers_arr_large REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.customers_part21 REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.customers_part22 REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO public.customers (id, name, email, status, arr) VALUES (1, 'ACTIVE', 'active@example.com', 'ACTIVE', 100	);`,
			`INSERT INTO public.customers (id, name, email, status, arr) VALUES (2, 'RECURRING', 'recurring@example.com', 'RECURRING', 105);`,
			`INSERT INTO public.customers (id, name, email, status, arr) VALUES (3, 'REACTIVATED', 'reactivated@example.com', 'REACTIVATED', 85);`,
			`INSERT INTO public.customers (id, name, email, status, arr) VALUES (4, 'OTHER', 'other@example.com', 'OTHER', 150);`,
			`INSERT INTO public.customers (id, name, email, status, arr) VALUES (5, 'ACTIVE', 'active@example.com', 'ACTIVE', 60);`,
			`INSERT INTO public.customers (id, name, email, status, arr) VALUES (6, 'RECURRING', 'recurring@example.com', 'RECURRING', 160);`,
			`INSERT INTO public.customers (id, name, email, status, arr) VALUES (7, 'REACTIVATED', 'reactivated@example.com', 'REACTIVATED', 50);`,
		},
		SourceDeltaSQL: []string{
			`DO $$
			DECLARE
			BEGIN
				FOR i IN 1..100 LOOP
					INSERT INTO public.customers (id, name, email, status, arr) VALUES (i+7, 'ABC', 'active@example.com', 'ACTIVE', 100);
					INSERT INTO public.customers (id, name, email, status, arr) VALUES (i+107, 'DEF', 'recurring@example.com', 'RECURRING', 105);
					INSERT INTO public.customers (id, name, email, status, arr) VALUES (i+207, 'OTHER', 'other@example.com', 'OTHER', 150);
					INSERT INTO public.customers (id, name, email, status, arr) VALUES (i+307, 'GHI', 'reactivated@example.com', 'REACTIVATED', 85);

					UPDATE public.customers SET status = 'REACTIVATED', arr = 105 WHERE id = i+7 AND arr = 100;
					UPDATE public.customers SET status = 'OTHER', arr = 50 WHERE id = i+107 AND arr = 105;
					UPDATE public.customers SET status = 'ACTIVE', arr = 60 WHERE id = i+207 AND arr = 150;
					UPDATE public.customers SET status = 'RECURRING', arr = 160 WHERE id = i+307 AND arr = 85;

					UPDATE public.customers SET email = 'activeabc@example.com' WHERE id = i+7 AND arr = 105;
					UPDATE public.customers SET email = 'recurringdef@example.com' WHERE id = i+107 AND arr = 50;
					UPDATE public.customers SET email = 'reactivatedghi@example.com' WHERE id = i+207 AND arr = 60;
					UPDATE public.customers SET email = 'otherjkl@example.com' WHERE id = i+307 AND arr = 160;

					IF i % 2 = 0 THEN
						DELETE FROM public.customers WHERE id = i+7;
					END IF;
				END LOOP;
			END $$;`,
		},
		TargetDeltaSQL: []string{
			`DO $$
			DECLARE
			BEGIN
				FOR i IN 401..500 LOOP
					INSERT INTO public.customers (id, name, email, status, arr) VALUES (i+7, 'ABC', 'active@example.com', 'ACTIVE', 100);
					INSERT INTO public.customers (id, name, email, status, arr) VALUES (i+107, 'OTHER', 'other@example.com', 'OTHER', 150);
					INSERT INTO public.customers (id, name, email, status, arr) VALUES (i+207, 'DEF', 'recurring@example.com', 'RECURRING', 160);
					INSERT INTO public.customers (id, name, email, status, arr) VALUES (i+307, 'GHI', 'reactivated@example.com', 'REACTIVATED', 85);

					UPDATE public.customers SET status = 'REACTIVATED', arr = 105 WHERE id = i+7 AND arr = 100;
					UPDATE public.customers SET status = 'RECURRING', arr = 50 WHERE id = i+107 AND arr = 150;
					UPDATE public.customers SET status = 'ACTIVE', arr = 60 WHERE id = i+207 AND arr = 160;
					UPDATE public.customers SET status = 'OTHER', arr = 160 WHERE id = i+307 AND arr = 85;

					UPDATE public.customers SET email = 'activeabc@example.com' WHERE id = i+7 AND arr = 105;
					UPDATE public.customers SET email = 'recurringdef@example.com' WHERE id = i+107 AND arr = 50;
					UPDATE public.customers SET email = 'reactivatedghi@example.com' WHERE id = i+207 AND arr = 60;
					UPDATE public.customers SET email = 'otherjkl@example.com' WHERE id = i+307 AND arr = 160;

					IF i % 2 = 0 THEN
						DELETE FROM public.customers WHERE id = i+7;
					END IF;
				END LOOP;
			END $$;`,
		},
		CleanupSQL: []string{
			`DROP TABLE IF EXISTS public.customers CASCADE;`,
		},
	})

	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportData(true, map[string]string{
		"--use-partition-root": "false",
	})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"public"."customers"`: 7,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"public"."customers"`: {
			Inserts: 800,
			Updates: 400,
			Deletes: 450,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = lm.ValidateDataConsistency([]string{`"public"."customers"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	err = lm.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		`"public"."customers"`: {
			Inserts: 800,
			Updates: 400,
			Deletes: 450,
		},
	}, 60, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = lm.ValidateDataConsistency([]string{`"public"."customers"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = lm.WaitForCutoverSourceComplete(0, 160)
	testutils.FatalIfError(t, err, "failed to wait for cutover source complete")

	err = lm.ValidateDataConsistency([]string{`"public"."customers"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")
}

/*
Schema:
public.orders - partitioned table with child partitions in different schemas
  - no primary key on root table
  - test_schema.orders_us - partition for 'US' with primary key (id)
  - test_schema.orders_eu - partition for 'EU' with primary key (id)
  - test_schema.orders_apac - partition for 'APAC' with primary key (id)

public.customers - partitioned table with child partitions in different schemas
  - no primary key on root table
  - test_schema.customers_other - default partition with primary key (id)
  - test_schema.customers_active - partition for 'ACTIVE', 'RECURRING','REACTIVATED' with primary key (id)
  - test_schema.customers_arr_small - partition for 'ACTIVE', 'RECURRING','REACTIVATED' with primary key (id)

CDC events:
orders:
  - INSERT events in all partitions - 600
  - UPDATE partitions - 300
  - DELETE rows for partition - 350

customers:
  - INSERT events in all partitions - 3
  - UPDATE partitions - 2 (1 - updating email, 1 - updating status within same partition), (another UpdAte to change the partition from some status to OTHER default status so it changes the partition)
  - DELETE rows for partition - 2

In the starting not creating PK on some partitions of both the tables - test_schema.orders_eu, test_schema.customers_other, public.customers_part21
Checking if export data fails with the Primary key guardrail error.
if it fails, assert the error message contains the table names without a Primary key.
and then create the PK on the tables and then runs the rest of the flow
*/
func TestLiveMigrationPartitionedWithChildPKAndPartitionsAcrossDifferentSchemas(t *testing.T) {
	t.Parallel()
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_partition_across_schemas",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_partition_across_schemas",
		},
		SchemaNames: []string{"public", "test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS public;`,
			`CREATE SCHEMA IF NOT EXISTS test_schema;`,
			`CREATE TABLE public.orders (
				id SERIAL,
				region TEXT NOT NULL,
				amount bigint
			) PARTITION BY LIST (region);`,
			`CREATE TABLE test_schema.orders_us PARTITION OF public.orders FOR VALUES IN ('US');`,
			`ALTER TABLE test_schema.orders_us ADD PRIMARY KEY (id);`,
			`CREATE TABLE test_schema.orders_eu PARTITION OF public.orders FOR VALUES IN ('EU');`,
			`CREATE TABLE test_schema.orders_apac PARTITION OF public.orders FOR VALUES IN ('APAC');`,
			`ALTER TABLE test_schema.orders_apac ADD PRIMARY KEY (id);`,
			`CREATE TABLE public.customers(
				id SERIAL,
				name TEXT NOT NULL,
				email TEXT NOT NULL,
				status TEXT NOT NULL,
				arr NUMERIC NOT NULL
			) PARTITION BY LIST (status);`,
			`CREATE TABLE public.customers_active PARTITION OF public.customers FOR VALUES IN ('ACTIVE', 'RECURRING','REACTIVATED') PARTITION BY RANGE(arr);`,
			`CREATE TABLE test_schema.customers_other PARTITION OF public.customers DEFAULT;`,
			`CREATE TABLE public.customers_arr_small PARTITION OF public.customers_active FOR VALUES FROM (MINVALUE) TO (101) PARTITION BY HASH(id);`,
			`CREATE TABLE test_schema.customers_part11 PARTITION OF public.customers_arr_small FOR VALUES WITH (modulus 2, remainder 0);`,
			`CREATE TABLE public.customers_part12 PARTITION OF public.customers_arr_small FOR VALUES WITH (modulus 2, remainder 1);`,
			`CREATE TABLE test_schema.customers_arr_large PARTITION OF public.customers_active FOR VALUES FROM (101) TO (MAXVALUE) PARTITION BY HASH(id);`,
			`CREATE TABLE public.customers_part21 PARTITION OF test_schema.customers_arr_large FOR VALUES WITH (modulus 2, remainder 0);`,
			`CREATE TABLE test_schema.customers_part22 PARTITION OF test_schema.customers_arr_large FOR VALUES WITH (modulus 2, remainder 1);`,
			`ALTER TABLE test_schema.customers_part11 ADD PRIMARY KEY (id);`,
			`ALTER TABLE public.customers_part12 ADD PRIMARY KEY (id);`,
			`ALTER TABLE test_schema.customers_part22 ADD PRIMARY KEY (id);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE public.orders REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.orders_us REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.orders_eu REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.orders_apac REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.customers REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.customers_other REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.customers_active REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.customers_arr_small REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.customers_part11 REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.customers_part12 REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.customers_arr_large REPLICA IDENTITY FULL;`,
			`ALTER TABLE public.customers_part21 REPLICA IDENTITY FULL;`,
			`ALTER TABLE test_schema.customers_part22 REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO public.orders (id, region, amount) VALUES (1, 'US', 100);`,
			`INSERT INTO public.orders (id, region, amount) VALUES (2, 'US', 200);`,
			`INSERT INTO public.orders (id, region, amount) VALUES (3, 'EU', 150);`,
			`INSERT INTO public.orders (id, region, amount) VALUES (4, 'EU', 250);`,
			`INSERT INTO public.orders (id, region, amount) VALUES (5, 'APAC', 300);`,
			`INSERT INTO public.customers (id, name, email, status, arr) VALUES (1, 'RECURRING', 'recurring@example.com', 'RECURRING', 106);`,
			`INSERT INTO public.customers (id, name, email, status, arr) VALUES (2, 'REACTIVATED', 'reactivated@example.com', 'OTHER', 85);`,
		},
		SourceDeltaSQL: []string{
			`
			DO $$
			DECLARE
			BEGIN
				FOR i IN 1..100 LOOP
					INSERT INTO public.orders (id, region, amount) VALUES (i+5, 'US', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+105, 'EU', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+205, 'APAC', i * 100);

					UPDATE public.orders SET amount = amount + 1 WHERE id = i+5;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+105;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+205;

					UPDATE public.orders SET region = 'EU' WHERE id = i+5;
					UPDATE public.orders SET region = 'APAC' WHERE id = i+105;
					UPDATE public.orders SET region = 'US' WHERE id = i+205;

					IF i % 2 = 0 THEN
						DELETE FROM public.orders WHERE id = i+5;
					END IF;
				END LOOP;
			END $$;`,
			`INSERT INTO public.customers (id, name, email, status, arr) VALUES (3, 'ABC', 'active@example.com', 'ACTIVE', 100);`,
			`INSERT INTO public.customers (id, name, email, status, arr) VALUES (4, 'DEF', 'recurring@example.com', 'RECURRING', 105);`,
			`UPDATE public.customers SET status = 'OTHER' WHERE id = 4 AND arr = 105;`,
			`UPDATE public.customers SET status = 'RECURRING' WHERE id = 3 AND arr = 100;`,
			`UPDATE public.customers SET email = 'other@example.com' WHERE id = 4 AND arr = 105;`,
			`DELETE FROM public.customers WHERE id = 3;`,
		},
		TargetDeltaSQL: []string{
			`
			DO $$
			DECLARE
			BEGIN
				FOR i IN 301..400 LOOP
					INSERT INTO public.orders (id, region, amount) VALUES (i+5, 'US', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+105, 'EU', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+205, 'APAC', i * 100);

					UPDATE public.orders SET amount = amount + 1 WHERE id = i+5;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+105;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+205;

					UPDATE public.orders SET region = 'EU' WHERE id = i+5;
					UPDATE public.orders SET region = 'APAC' WHERE id = i+105;
					UPDATE public.orders SET region = 'US' WHERE id = i+205;

					IF i % 2 = 0 THEN
						DELETE FROM public.orders WHERE id = i+5;
					END IF;
				END LOOP;
			END $$;`,

			`INSERT INTO public.customers (id, name, email, status, arr) VALUES (5, 'ABC', 'active@example.com', 'ACTIVE', 100);`,
			`INSERT INTO public.customers (id, name, email, status, arr) VALUES (6, 'DEF', 'recurring@example.com', 'RECURRING', 105);`,
			`UPDATE public.customers SET status = 'OTHER' WHERE id = 6 AND arr = 105;`,
			`UPDATE public.customers SET status = 'REACTIVATED' WHERE id = 5 AND arr = 100;`,
			`UPDATE public.customers SET email = 'reactive@example.com' WHERE id = 5 AND arr = 100;`,
			`DELETE FROM public.customers WHERE id = 5;`,
		},
		CleanupSQL: []string{
			`DROP TABLE IF EXISTS public.orders CASCADE;`,
			`DROP TABLE IF EXISTS test_schema.orders_us CASCADE;`,
			`DROP TABLE IF EXISTS test_schema.orders_eu CASCADE;`,
			`DROP TABLE IF EXISTS test_schema.orders_apac CASCADE;`,
			`DROP TABLE IF EXISTS public.customers CASCADE;`,
			`DROP TABLE IF EXISTS test_schema.customers_other CASCADE;`,
			`DROP TABLE IF EXISTS public.customers_active CASCADE;`,
			`DROP TABLE IF EXISTS public.customers_arr_small CASCADE;`,
			`DROP TABLE IF EXISTS test_schema.customers_part11 CASCADE;`,
			`DROP TABLE IF EXISTS public.customers_part12 CASCADE;`,
			`DROP TABLE IF EXISTS test_schema.customers_arr_large CASCADE;`,
			`DROP TABLE IF EXISTS public.customers_part21 CASCADE;`,
			`DROP TABLE IF EXISTS test_schema.customers_part22 CASCADE;`,
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})

	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(false, nil)
	assert.Error(t, err)

	exportStderr := lm.GetExportCommandStderr()
	require.Contains(t, exportStderr, "Currently voyager does not support live-migration for tables without a primary key.\nYou can exclude these tables using the --exclude-table-list argument")

	//Validation for the Primary key guardrail in export data
	exportStdout := lm.GetExportCommandStdout()
	require.Contains(t, exportStdout, "Table names without a Primary key: [public.customers public.customers_part21 public.orders test_schema.customers_other test_schema.orders_eu]")

	err = lm.WithSourceTargetConn(func(sourceConn *sql.DB, targetConn *sql.DB) error {
		ddls := []string{
			`ALTER TABLE test_schema.orders_eu ADD PRIMARY KEY (id);`,
			`ALTER TABLE test_schema.customers_other ADD PRIMARY KEY (id);`,
			`ALTER TABLE public.customers_part21 ADD PRIMARY KEY (id);`,
		}
		for _, ddl := range ddls {
			_, err := sourceConn.Exec(ddl)
			if err != nil {
				return err
			}
			_, err = targetConn.Exec(ddl)
			if err != nil {
				return err
			}
		}
		return nil
	})

	testutils.FatalIfError(t, err, "failed to add primary keys to source and target")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportData(true, map[string]string{
		"--use-partition-root": "false",
	})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"public"."orders"`:    5,
		`"public"."customers"`: 2,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"public"."orders"`: {
			Inserts: 600,
			Updates: 300,
			Deletes: 350,
		},
		`"public"."customers"`: {
			Inserts: 3,
			Updates: 2,
			Deletes: 2,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = lm.ValidateDataConsistency([]string{`"public"."orders"`, `"public"."customers"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	err = lm.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		`"public"."orders"`: {
			Inserts: 600,
			Updates: 300,
			Deletes: 350,
		},
		`"public"."customers"`: {
			Inserts: 3,
			Updates: 2,
			Deletes: 2,
		},
	}, 60, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = lm.ValidateDataConsistency([]string{`"public"."orders"`, `"public"."customers"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToSource(nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = lm.WaitForCutoverSourceComplete(0, 160)
	testutils.FatalIfError(t, err, "failed to wait for cutover source complete")

	err = lm.ValidateDataConsistency([]string{`"public"."orders"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")
}

func TestLiveMigrationWithIterationsOnPartitionedTableWithChildPK(t *testing.T) {
	t.Parallel()

	lm := getLiveMigrationTestBasicForPartitionedTableWithChildPK(t, "test_iterations_on_partitioned_table_with_child_pk")

	// defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportData(true, map[string]string{
		"--use-partition-root": "false",
	})
	testutils.FatalIfError(t, err, "failed to start import data")

	err = lm.WaitForSnapshotComplete(map[string]int64{
		`"public"."orders"`: 5,
	}, 30)
	testutils.FatalIfError(t, err, "failed to wait for snapshot complete")

	err = lm.ValidateDataConsistency([]string{`"public"."orders"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	var forwardInserts int64 = 300
	var forwardUpdates int64 = 300
	var forwardDeletes int64 = 50
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"public"."orders"`: {
			Inserts: forwardInserts,
			Updates: forwardUpdates,
			Deletes: forwardDeletes,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = lm.ValidateDataConsistency([]string{`"public"."orders"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = lm.WaitForCutoverComplete(0, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	err = lm.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	var fallbackInserts int64 = 300
	var fallbackUpdates int64 = 300
	var fallbackDeletes int64 = 50
	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		`"public"."orders"`: {
			Inserts: fallbackInserts,
			Updates: fallbackUpdates,
			Deletes: fallbackDeletes,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for fallback streaming complete")

	err = lm.ValidateDataConsistency([]string{`"public"."orders"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	//iteration-1
	err = lm.InitiateCutoverToSource(map[string]string{
		"--restart-data-migration-source-target": "true",
	})
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = lm.WaitForNextIterationInitialized(0, 100)
	testutils.FatalIfError(t, err, "failed to wait for next iteration initialized")

	err = lm.WaitForCutoverSourceComplete(0, 100)
	testutils.FatalIfError(t, err, "failed to wait for cutover source complete")

	//source delta for iteration-1
	lm.config.SourceDeltaSQL = []string{
		`
		DO $$
			DECLARE
			BEGIN
				FOR i IN 601..700 LOOP
					INSERT INTO public.orders (id, region, amount) VALUES (i+5, 'US', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+105, 'EU', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+205, 'APAC', i * 100);

					UPDATE public.orders SET amount = amount + 1 WHERE id = i+5;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+105;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+205;

					IF i % 2 = 0 THEN
						DELETE FROM public.orders WHERE id = i+5;
					END IF;
				END LOOP;
			END $$;
		`,
	}

	//target delta for iteration-1
	lm.config.TargetDeltaSQL = []string{
		`
		DO $$
			DECLARE
			BEGIN
				FOR i IN 901..1000 LOOP
					INSERT INTO public.orders (id, region, amount) VALUES (i+5, 'US', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+105, 'EU', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+205, 'APAC', i * 100);

					UPDATE public.orders SET amount = amount + 1 WHERE id = i+5;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+105;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+205;

					IF i % 2 = 0 THEN
						DELETE FROM public.orders WHERE id = i+5;
					END IF;
				END LOOP;
			END $$;
		`,
	}

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	forwardInserts += 300
	forwardUpdates += 300
	forwardDeletes += 50
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"public"."orders"`: {
			Inserts: forwardInserts,
			Updates: forwardUpdates,
			Deletes: forwardDeletes,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = lm.ValidateDataConsistency([]string{`"public"."orders"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToTarget(true, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = lm.WaitForCutoverComplete(1, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	err = lm.ExecuteTargetDelta()
	testutils.FatalIfError(t, err, "failed to execute target delta")

	fallbackInserts += 300
	fallbackUpdates += 300
	fallbackDeletes += 50
	err = lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
		`"public"."orders"`: {
			Inserts: fallbackInserts,
			Updates: fallbackUpdates,
			Deletes: fallbackDeletes,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for fallback streaming complete")

	err = lm.ValidateDataConsistency([]string{`"public"."orders"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	//iteration-2
	err = lm.InitiateCutoverToSource(map[string]string{
		"--restart-data-migration-source-target": "true",
	})
	testutils.FatalIfError(t, err, "failed to initiate cutover to source")

	err = lm.WaitForNextIterationInitialized(1, 100)
	testutils.FatalIfError(t, err, "failed to wait for next iteration initialized")

	err = lm.WaitForCutoverSourceComplete(1, 100)
	testutils.FatalIfError(t, err, "failed to wait for cutover source complete")

	//source delta for iteration-2
	lm.config.SourceDeltaSQL = []string{
		`
		DO $$
			DECLARE
			BEGIN
				FOR i IN 1201..1300 LOOP
					INSERT INTO public.orders (id, region, amount) VALUES (i+5, 'US', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+105, 'EU', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+205, 'APAC', i * 100);

					UPDATE public.orders SET amount = amount + 1 WHERE id = i+5;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+105;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+205;

					IF i % 2 = 0 THEN
						DELETE FROM public.orders WHERE id = i+5;
					END IF;
				END LOOP;
			END $$;
		`,
	}

	//target delta for iteration-2
	lm.config.TargetDeltaSQL = []string{
		`
		DO $$
			DECLARE
			BEGIN
				FOR i IN 1501..1600 LOOP
					INSERT INTO public.orders (id, region, amount) VALUES (i+5, 'US', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+105, 'EU', i * 100);
					INSERT INTO public.orders (id, region, amount) VALUES (i+205, 'APAC', i * 100);

					UPDATE public.orders SET amount = amount + 1 WHERE id = i+5;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+105;
					UPDATE public.orders SET amount = amount + 1 WHERE id = i+205;

					IF i % 2 = 0 THEN
						DELETE FROM public.orders WHERE id = i+5;
					END IF;
				END LOOP;
			END $$;
		`,
	}

	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	forwardInserts += 300
	forwardUpdates += 300
	forwardDeletes += 50
	err = lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		`"public"."orders"`: {
			Inserts: forwardInserts,
			Updates: forwardUpdates,
			Deletes: forwardDeletes,
		},
	}, 30, 1)
	testutils.FatalIfError(t, err, "failed to wait for streaming complete")

	err = lm.ValidateDataConsistency([]string{`"public"."orders"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

	err = lm.InitiateCutoverToTarget(false, nil)
	testutils.FatalIfError(t, err, "failed to initiate cutover to target")

	err = lm.WaitForCutoverComplete(2, 30)
	testutils.FatalIfError(t, err, "failed to wait for cutover complete")

	err = lm.ValidateDataConsistency([]string{`"public"."orders"`}, "id")
	testutils.FatalIfError(t, err, "failed to validate data consistency")

}
