//go:build integration_live_migration

package testlivemigration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// ---------------------------------------------------------------------------
// TestRangeTypeSpike is an EVIDENCE-GATHERING test (a spike), not a pass/fail
// assertion test. It answers:
//  1. does `export data` exclude BUILT-IN range columns (int4range, ...) and
//     USER-DEFINED range columns from CDC?
//  2. is the user warned, and with what exact text? is a prompt printed?
//  3. does --yes auto-accept the prompt and continue?
//  4. does the SNAPSHOT still carry range values (pg_dump is not column
//     filtered) while CDC changes to those columns are lost?
//  5. what happens to an UPDATE that touches ONLY a range column?
//
// Everything is reported through t.Logf with a "RANGE-FINDING:" prefix. Only
// genuine harness errors fail the test.
// ---------------------------------------------------------------------------

const rangeSpikeTable = `"test_schema"."test_ranges"`

// column order used by rangeSpikeQuery (id excluded)
var rangeSpikeCols = []string{
	"filler", "r_int4", "r_int8", "r_num", "r_ts", "r_tstz", "r_date", "r_custom", "r_int4_arr",
}

const rangeSpikeQuery = `SELECT id,
	filler,
	r_int4::text, r_int8::text, r_num::text, r_ts::text, r_tstz::text,
	r_date::text, r_custom::text, r_int4_arr::text
FROM test_schema.test_ranges ORDER BY id`

func TestRangeTypeSpike(t *testing.T) {
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "rangespike",
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "rangespike",
		},
		SchemaNames: []string{"test_schema"},
		SchemaSQL: []string{
			`CREATE SCHEMA IF NOT EXISTS test_schema;`,
			`CREATE TYPE test_schema.pricerange AS RANGE (subtype = numeric);`,
			`CREATE TABLE test_schema.test_ranges (
				id         int PRIMARY KEY,
				filler     text,
				r_int4     int4range,
				r_int8     int8range,
				r_num      numrange,
				r_ts       tsrange,
				r_tstz     tstzrange,
				r_date     daterange,
				r_custom   test_schema.pricerange,
				r_int4_arr int4range[]
			);`,
		},
		SourceSetupSchemaSQL: []string{
			`ALTER TABLE test_schema.test_ranges REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO test_schema.test_ranges VALUES
				(1, 'snap-one',   '[1,10)',   '[100,200)',     '[1.5,9.5)',
				 '[2024-01-01 00:00:00,2024-02-01 00:00:00)',
				 '[2024-01-01 00:00:00+00,2024-02-01 00:00:00+00)',
				 '[2024-01-01,2024-02-01)', '[10.50,99.99)',
				 ARRAY['[1,5)'::int4range,'[7,9)'::int4range]),
				(2, 'snap-two',   '[11,20)',  '[300,400)',     '[2.5,8.5)',
				 '[2024-02-01 00:00:00,2024-03-01 00:00:00)',
				 '[2024-02-01 00:00:00+00,2024-03-01 00:00:00+00)',
				 '[2024-02-01,2024-03-01)', '[20.50,89.99)',
				 ARRAY['[11,15)'::int4range]),
				(3, 'snap-three', '[21,30)',  '[500,600)',     '[3.5,7.5)',
				 '[2024-03-01 00:00:00,2024-04-01 00:00:00)',
				 '[2024-03-01 00:00:00+00,2024-04-01 00:00:00+00)',
				 '[2024-03-01,2024-04-01)', '[30.50,79.99)',
				 ARRAY['[21,25)'::int4range]);`,
		},
		SourceDeltaSQL: []string{
			// (a) INSERT a brand new row carrying range values
			`INSERT INTO test_schema.test_ranges VALUES
				(4, 'delta-insert', '[40,50)', '[4000,5000)', '[4.5,5.5)',
				 '[2024-04-01 00:00:00,2024-05-01 00:00:00)',
				 '[2024-04-01 00:00:00+00,2024-05-01 00:00:00+00)',
				 '[2024-04-01,2024-05-01)', '[40.40,50.50)',
				 ARRAY['[40,45)'::int4range]);`,
			// (b) UPDATE only a NON-range column
			`UPDATE test_schema.test_ranges SET filler = 'filler-only-update' WHERE id = 1;`,
			// (c) UPDATE only range columns (nothing else changes)
			`UPDATE test_schema.test_ranges
				SET r_int4 = '[999,1000)', r_num = '[99.9,199.9)',
				    r_custom = '[900.00,999.00)', r_date = '[2030-01-01,2030-02-01)'
				WHERE id = 2;`,
			// (c2) UPDATE only the ARRAY-of-range column (does the array bypass the filter?)
			`UPDATE test_schema.test_ranges
				SET r_int4_arr = ARRAY['[777,888)'::int4range] WHERE id = 3;`,
			// (d) DELETE a row
			`DELETE FROM test_schema.test_ranges WHERE id = 1;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS test_schema CASCADE;`,
		},
	})

	defer lm.Cleanup()

	err := lm.SetupContainers(context.Background())
	testutils.FatalIfError(t, err, "failed to setup containers")

	err = lm.SetupSchema()
	testutils.FatalIfError(t, err, "failed to setup schema")

	err = lm.StartExportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start export data")

	err = lm.StartImportData(true, nil)
	testutils.FatalIfError(t, err, "failed to start import data")

	// ---- Q2/Q3: what did export data print about unsupported columns? -----
	rangeSpikeDumpExportOutput(t, lm)

	// ---- snapshot ---------------------------------------------------------
	snapErr := lm.WaitForSnapshotComplete(map[string]int64{rangeSpikeTable: 3}, 90)
	if snapErr != nil {
		t.Logf("RANGE-FINDING: snapshot did not reach 3/3 rows: %v", snapErr)
	}
	rangeSpikeReport(t, lm, "AFTER-SNAPSHOT")
	rangeSpikeCompare(t, lm, "AFTER-SNAPSHOT")

	// ---- delta + streaming ------------------------------------------------
	err = lm.ExecuteSourceDelta()
	testutils.FatalIfError(t, err, "failed to execute source delta")

	// expected if nothing is dropped: 1 insert, 3 updates, 1 delete
	streamErr := lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		rangeSpikeTable: {Inserts: 1, Updates: 3, Deletes: 1},
	}, 90, 2)
	if streamErr != nil {
		t.Logf("RANGE-FINDING: streaming never reached inserts=1/updates=3/deletes=1: %v", streamErr)
	} else {
		t.Logf("RANGE-FINDING: streaming reached the full expected counts inserts=1/updates=3/deletes=1")
	}
	rangeSpikeReport(t, lm, "AFTER-STREAMING")
	rangeSpikeCompare(t, lm, "AFTER-STREAMING")

	// ---- what is actually inside the CDC event queue? ---------------------
	rangeSpikeDumpQueue(t, lm)

	// re-print export output at the end in case more was flushed
	rangeSpikeDumpExportOutput(t, lm)
}

// rangeSpikeDumpExportOutput greps export data's stdout/stderr for the
// unsupported-column warning and the prompt.
func rangeSpikeDumpExportOutput(t *testing.T, lm *LiveMigrationTest) {
	for _, s := range []struct {
		name string
		body string
	}{
		{"stdout", lm.GetExportCommandStdout()},
		{"stderr", lm.GetExportCommandStderr()},
	} {
		t.Logf("RANGE-FINDING: ===== export data %s (%d bytes) =====\n%s\n===== end %s =====",
			s.name, len(s.body), s.body, s.name)
		for _, line := range strings.Split(s.body, "\n") {
			l := strings.ToLower(line)
			if strings.Contains(l, "unsupported") ||
				strings.Contains(l, "ignoring") ||
				strings.Contains(l, "[y/n]") ||
				strings.Contains(l, "do you want to continue") ||
				strings.Contains(l, "null constraint") ||
				strings.Contains(l, "r_int4") ||
				strings.Contains(l, "r_custom") {
				t.Logf("RANGE-FINDING: MATCH(%s): %q", s.name, line)
			}
		}
	}
}

// rangeSpikeReport prints the exported/imported counters.
func rangeSpikeReport(t *testing.T, lm *LiveMigrationTest, phase string) {
	report, err := lm.GetDataMigrationReport()
	if err != nil {
		t.Logf("RANGE-FINDING: [%s] could not fetch data-migration-report: %v", phase, err)
		return
	}
	for _, row := range report.RowData {
		t.Logf("RANGE-FINDING: [%s] REPORT table=%s db=%s expSnap=%d impSnap=%d "+
			"expI/U/D=%d/%d/%d impI/U/D=%d/%d/%d final=%d",
			phase, row.TableName, row.DBType, row.ExportedSnapshotRows, row.ImportedSnapshotRows,
			row.ExportedInserts, row.ExportedUpdates, row.ExportedDeletes,
			row.ImportedInserts, row.ImportedUpdates, row.ImportedDeletes, row.FinalRowCount)
	}
}

func rangeSpikeFetch(db *sql.DB) (map[int64][]string, error) {
	rows, err := db.Query(rangeSpikeQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64][]string)
	for rows.Next() {
		var id int64
		vals := make([]sql.NullString, len(rangeSpikeCols))
		ptrs := []any{&id}
		for i := range vals {
			ptrs = append(ptrs, &vals[i])
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		strs := make([]string, len(vals))
		for i, v := range vals {
			if v.Valid {
				strs[i] = v.String
			} else {
				strs[i] = "<NULL>"
			}
		}
		out[id] = strs
	}
	return out, rows.Err()
}

// rangeSpikeCompare prints source vs target values, column by column, per row.
func rangeSpikeCompare(t *testing.T, lm *LiveMigrationTest, phase string) {
	err := lm.WithSourceTargetConn(func(source, target *sql.DB) error {
		src, err := rangeSpikeFetch(source)
		if err != nil {
			return fmt.Errorf("source query: %w", err)
		}
		tgt, err := rangeSpikeFetch(target)
		if err != nil {
			return fmt.Errorf("target query: %w", err)
		}
		ids := map[int64]bool{}
		for id := range src {
			ids[id] = true
		}
		for id := range tgt {
			ids[id] = true
		}
		sorted := []int64{}
		for id := range ids {
			sorted = append(sorted, id)
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		t.Logf("RANGE-FINDING: [%s] source rows=%d target rows=%d", phase, len(src), len(tgt))
		for _, id := range sorted {
			s, sOK := src[id]
			g, gOK := tgt[id]
			if !sOK {
				t.Logf("RANGE-FINDING: [%s] id=%d MISSING-ON-SOURCE (present on target: %v)", phase, id, g)
				continue
			}
			if !gOK {
				t.Logf("RANGE-FINDING: [%s] id=%d MISSING-ON-TARGET (present on source: %v)", phase, id, s)
				continue
			}
			for i, col := range rangeSpikeCols {
				verdict := "SAME"
				if s[i] != g[i] {
					verdict = "**MISMATCH**"
				}
				t.Logf("RANGE-FINDING: [%s] id=%d col=%-10s src=%-55q tgt=%-55q %s",
					phase, id, col, s[i], g[i], verdict)
			}
		}
		return nil
	})
	if err != nil {
		t.Logf("RANGE-FINDING: [%s] compare failed: %v", phase, err)
	}
}

// rangeSpikeDumpQueue inspects the exported CDC event queue segments.
func rangeSpikeDumpQueue(t *testing.T, lm *LiveMigrationTest) {
	queueDir := filepath.Join(lm.GetCurrentExportDir(), "data", "queue")
	entries, err := os.ReadDir(queueDir)
	if err != nil {
		t.Logf("RANGE-FINDING: cannot read queue dir %s: %v", queueDir, err)
		return
	}
	var all []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(queueDir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			t.Logf("RANGE-FINDING: cannot read segment %s: %v", p, err)
			continue
		}
		t.Logf("RANGE-FINDING: queue segment %s (%d bytes)", e.Name(), len(b))
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			all = append(all, line)
		}
	}
	t.Logf("RANGE-FINDING: total queue event lines = %d", len(all))

	// only look at events for our table
	var mine []string
	for _, l := range all {
		if strings.Contains(l, "test_ranges") {
			mine = append(mine, l)
		}
	}
	t.Logf("RANGE-FINDING: queue event lines mentioning test_ranges = %d", len(mine))

	joined := strings.Join(mine, "\n")
	for _, col := range append([]string{"id"}, rangeSpikeCols...) {
		key := `"` + col + `"`
		present := strings.Contains(joined, key)
		count := strings.Count(joined, key)
		t.Logf("RANGE-FINDING: CDC column key %-12s present=%-5v occurrences=%d", key, present, count)
	}

	for i, l := range mine {
		if i >= 8 {
			t.Logf("RANGE-FINDING: (%d more event lines not printed)", len(mine)-i)
			break
		}
		t.Logf("RANGE-FINDING: SAMPLE EVENT[%d]: %s", i, l)
	}
}
