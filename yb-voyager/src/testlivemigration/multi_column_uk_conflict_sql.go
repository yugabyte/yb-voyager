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

import "fmt"

const multiColumnUKConflictTargetIDOffset = 2004

// multiColumnUniqueIndexConflictDeltaSQL generates source (idOffset=0) or target-side
// (idOffset=multiColumnUKConflictTargetIDOffset) delta SQL for multi-column UK conflict tests.
// When withRegion is true, inserts include region='r1' for partitioned tables.
func multiColumnUniqueIndexConflictDeltaSQL(tableName string, withRegion bool, idOffset int) []string {
	anchor20 := 20 + idOffset
	anchor521 := 521 + idOffset
	anchor1022 := 1022 + idOffset
	anchor1523 := 1523 + idOffset
	loop1Start := 21 + idOffset
	loop1End := 520 + idOffset
	loop2Start := 522 + idOffset
	loop2End := 1021 + idOffset
	loop3Start := 1023 + idOffset
	loop3End := 1522 + idOffset
	loop4Start := 1524 + idOffset
	loop4End := 2023 + idOffset

	insertCols := "id, id1, id2, updated_at"
	regionPrefix := ""
	loop3LastInsertValues := "i-1, i, i, now()"
	loop4LastInsertValues := "i-1, i, i, now()"
	loop2InsertValues := "i, i, NULL, now()"
	loop2LastInsertValues := "i-1, i-1, NULL, now()"
	if withRegion {
		insertCols = "id, region, id1, id2, updated_at"
		regionPrefix = "'r1', "
		loop2InsertValues = "i, 'r1', i, NULL, now()"
		loop2LastInsertValues = "i-1, 'r1', i-1, NULL, now()"
		loop3LastInsertValues = "i-1, 'r1', i, i, now()"
		loop4LastInsertValues = "i-1, 'r1', i, i, now()"
	}

	blocks := []string{
		fmt.Sprintf(` DO $$
		    DECLARE
				i INTEGER;
			BEGIN
				FOR i IN %d..%d LOOP
						UPDATE %s SET updated_at = now() + interval '1 second' WHERE id = i - 1;
						INSERT INTO %s(%s) VALUES(i, %s%d, i, now());

						DELETE FROM %s WHERE id = i;
						DELETE FROM %s WHERE id = i-1;
						INSERT INTO %s(%s) VALUES(i, %s%d, i, now());

						INSERT INTO %s(%s) VALUES(i-1, %s%d, i-1, now());
				END LOOP;
       		END
		$$;`, loop1Start, loop1End, tableName, tableName, insertCols, regionPrefix, anchor20, tableName, tableName, tableName, insertCols, regionPrefix, anchor20, tableName, insertCols, regionPrefix, anchor20),
		fmt.Sprintf(`INSERT INTO %s(%s) VALUES(%d, %s%d, NULL, now());`, tableName, insertCols, anchor521, regionPrefix, anchor521),
		fmt.Sprintf(` DO $$
		    DECLARE
				i INTEGER;
			BEGIN
				FOR i IN %d..%d LOOP
						UPDATE %s SET updated_at = now() + interval '1 second' WHERE id = i - 1;
						INSERT INTO %s(%s) VALUES(%s);

						DELETE FROM %s WHERE id = i;
						DELETE FROM %s WHERE id = i-1;
						INSERT INTO %s(%s) VALUES(%s);

						INSERT INTO %s(%s) VALUES(%s);

				END LOOP;
       		END
		$$;`, loop2Start, loop2End, tableName, tableName, insertCols, loop2InsertValues, tableName, tableName, tableName, insertCols, loop2InsertValues, tableName, insertCols, loop2LastInsertValues),
		fmt.Sprintf(`INSERT INTO %s(%s) VALUES(%d, %s%d, %d, now());`, tableName, insertCols, anchor1022, regionPrefix, anchor1022, anchor1022),
		fmt.Sprintf(` DO $$
		    DECLARE
				i INTEGER;
			BEGIN
				FOR i IN %d..%d LOOP
						UPDATE %s SET id1=i, id2=i WHERE id = i - 1;
						INSERT INTO %s(%s) VALUES(i, %s%d, %d, now());

						DELETE FROM %s WHERE id = i;
						UPDATE %s SET id1=%d, id2=%d WHERE id = i-1;

						DELETE FROM %s WHERE id = i-1;
						INSERT INTO %s(%s) VALUES(i, %s%d, %d, now());

						INSERT INTO %s(%s) VALUES(%s);

				END LOOP;
       		END
		$$;`, loop3Start, loop3End, tableName, tableName, insertCols, regionPrefix, anchor1022, anchor1022, tableName, tableName, anchor1022, anchor1022, tableName, tableName, insertCols, regionPrefix, anchor1022, anchor1022, tableName, insertCols, loop3LastInsertValues),
		fmt.Sprintf(`INSERT INTO %s(%s) VALUES(%d, %sNULL, NULL, now());`, tableName, insertCols, anchor1523, regionPrefix),
		fmt.Sprintf(` DO $$
		    DECLARE
				i INTEGER;
			BEGIN
				FOR i IN %d..%d LOOP
						UPDATE %s SET id1=i, id2=i WHERE id = i - 1;
						INSERT INTO %s(%s) VALUES(i, %sNULL, NULL, now());

						DELETE FROM %s WHERE id = i;
						UPDATE %s SET id1=NULL, id2=NULL WHERE id = i-1;

						DELETE FROM %s WHERE id = i-1;
						INSERT INTO %s(%s) VALUES(i, %sNULL, NULL, now());

						INSERT INTO %s(%s) VALUES(%s);


				END LOOP;
       		END
		$$;`, loop4Start, loop4End, tableName, tableName, insertCols, regionPrefix, tableName, tableName, tableName, tableName, insertCols, regionPrefix, tableName, insertCols, loop4LastInsertValues),
	}

	if idOffset > 0 {
		leadingInsert := fmt.Sprintf(`INSERT INTO %s(%s) VALUES(%d, %s%d, %d, now());`,
			tableName, insertCols, anchor20, regionPrefix, anchor20, anchor20)
		return append([]string{leadingInsert}, blocks...)
	}
	return blocks
}

// multiColumnUKFalsePositiveDeltaSQL returns delta SQL blocks that should not trigger
// unique-key conflict detection (loops 21-1021).
func multiColumnUKFalsePositiveDeltaSQL(tableName string, withRegion bool, idOffset int) []string {
	all := multiColumnUniqueIndexConflictDeltaSQL(tableName, withRegion, idOffset)
	return all[0:3]
}

// multiColumnUKTruePositiveDeltaSQL returns delta SQL blocks that should trigger
// unique-key conflict detection (loops 1023+).
func multiColumnUKTruePositiveDeltaSQL(tableName string, withRegion bool, idOffset int) []string {
	all := multiColumnUniqueIndexConflictDeltaSQL(tableName, withRegion, idOffset)
	return all[3:]
}

const (
	multiColumnUKFalsePositiveChangesInserts = 3001
	multiColumnUKFalsePositiveChangesUpdates = 1000
	multiColumnUKFalsePositiveChangesDeletes = 2000
)
