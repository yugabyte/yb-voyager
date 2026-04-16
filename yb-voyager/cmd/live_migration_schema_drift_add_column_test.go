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

package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newAddColumnDriftLM(t *testing.T, dbSuffix string, controlTable, subjectTable string) *LiveMigrationTest {
	const schema = "test_schema"
	return NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: "test_addcol_drift_" + dbSuffix,
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: "test_addcol_drift_" + dbSuffix,
		},
		SchemaNames: []string{schema},
		SchemaSQL: []string{
			fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s;
CREATE TABLE %s (
	id SERIAL PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s (
	id SERIAL PRIMARY KEY,
	name TEXT NOT NULL
);`, schema, controlTable, subjectTable),
		},
		SourceSetupSchemaSQL: []string{
			fmt.Sprintf(`ALTER TABLE %s REPLICA IDENTITY FULL;`, controlTable),
			fmt.Sprintf(`ALTER TABLE %s REPLICA IDENTITY FULL;`, subjectTable),
		},
		InitialDataSQL: []string{
			fmt.Sprintf(`INSERT INTO %s (name) SELECT 'row_' || g::text FROM generate_series(1,5) g;`, controlTable),
			fmt.Sprintf(`INSERT INTO %s (name) SELECT 'row_' || g::text FROM generate_series(1,5) g;`, subjectTable),
		},
		CleanupSQL: []string{
			fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE;`, schema),
		},
	})
}

// TestLiveMigrationAddColumnDuringCDC_BeforeDMLWorkaround aligns target DDL before inserts
// (operator fixes catalog ahead of rows), using RunSchemaDriftPhase.
func TestLiveMigrationAddColumnDuringCDC_BeforeDMLWorkaround(t *testing.T) {
	t.Parallel()

	const schema = "test_schema"
	controlFQ := fmt.Sprintf(`"%s"."control_addcol"`, schema)
	subjectFQ := fmt.Sprintf(`"%s"."subject_addcol"`, schema)

	lm := newAddColumnDriftLM(t, "align", controlFQ, subjectFQ)
	defer lm.Cleanup()

	require.NoError(t, lm.SetupContainers(context.Background()))
	require.NoError(t, lm.SetupSchema())
	require.NoError(t, lm.StartExportData(true, nil))
	require.NoError(t, lm.StartImportData(true, nil))
	require.NoError(t, lm.WaitForSnapshotComplete(map[string]int64{
		controlFQ: 5,
		subjectFQ: 5,
	}, 60))
	require.NoError(t, lm.ValidateDataConsistency([]string{controlFQ, subjectFQ}, "id"))
	require.NoError(t, lm.WaitForStreamingMode(120*time.Second, 2*time.Second))

	res, err := lm.RunSchemaDriftPhase(SchemaDriftPhaseConfig{
		SourceDDL: []string{
			fmt.Sprintf(`ALTER TABLE %s.subject_addcol ADD COLUMN extra TEXT;`, schema),
		},
		SourceDML: []string{
			fmt.Sprintf(`INSERT INTO %s.control_addcol (name) VALUES ('post_ddl_control');`, schema),
			fmt.Sprintf(`INSERT INTO %s.subject_addcol (name, extra) VALUES ('post_ddl_subject', 'extra_val');`, schema),
		},
		TargetWorkaroundSQL: []string{
			fmt.Sprintf(`ALTER TABLE %s.subject_addcol ADD COLUMN extra TEXT;`, schema),
		},
		WorkaroundMode: SchemaDriftWorkaroundBeforeSourceDML,
		ReplicationWaits: []SchemaDriftReplicationWait{
			{Name: "control", Expected: map[string]ChangesCount{controlFQ: {Inserts: 1}}, Timeout: 60 * time.Second, Poll: 2 * time.Second, RequireSuccess: true},
			{Name: "subject", Expected: map[string]ChangesCount{subjectFQ: {Inserts: 1}}, Timeout: 90 * time.Second, Poll: 2 * time.Second, RequireSuccess: true},
		},
	})
	require.NoError(t, err, SchemaDriftStderrTail(lm, 2000))
	if ce := res.AllStreamingCycleErrors(); len(ce) > 0 {
		t.Logf("stop/resume cycle errors (inspect if expected): %v", ce)
	}

	require.NoError(t, lm.WithSourceTargetConn(func(src, tgt *sql.DB) error {
		var srcCtl, tgtCtl int
		if err := src.QueryRow(fmt.Sprintf(`SELECT count(*) FROM %s.control_addcol`, schema)).Scan(&srcCtl); err != nil {
			return fmt.Errorf("count source control: %w", err)
		}
		if err := tgt.QueryRow(fmt.Sprintf(`SELECT count(*) FROM %s.control_addcol`, schema)).Scan(&tgtCtl); err != nil {
			return fmt.Errorf("count target control: %w", err)
		}
		if srcCtl != 6 || tgtCtl != 6 {
			return fmt.Errorf("control row counts want 6/6 got source=%d target=%d", srcCtl, tgtCtl)
		}
		var srcSub, tgtSub int
		if err := src.QueryRow(fmt.Sprintf(`SELECT count(*) FROM %s.subject_addcol`, schema)).Scan(&srcSub); err != nil {
			return fmt.Errorf("count source subject: %w", err)
		}
		if err := tgt.QueryRow(fmt.Sprintf(`SELECT count(*) FROM %s.subject_addcol`, schema)).Scan(&tgtSub); err != nil {
			return fmt.Errorf("count target subject: %w", err)
		}
		if srcSub != 6 || tgtSub != 6 {
			return fmt.Errorf("subject row counts want 6/6 got source=%d target=%d", srcSub, tgtSub)
		}
		var extra sql.NullString
		if err := tgt.QueryRow(fmt.Sprintf(`SELECT extra FROM %s.subject_addcol WHERE name = 'post_ddl_subject'`, schema)).Scan(&extra); err != nil {
			return fmt.Errorf("read extra on target: %w", err)
		}
		if !extra.Valid || extra.String != "extra_val" {
			return fmt.Errorf("target extra want extra_val got valid=%v s=%q", extra.Valid, extra.String)
		}
		return nil
	}))

	require.NoError(t, lm.StopExportData())
	require.NoError(t, lm.StopImportData())
}

// TestLiveMigrationAddColumnDuringCDC_NoWorkaroundObservesSubjectFailure runs a full
// import+export stop/resume after each wait; subject replication is not required to succeed.
func TestLiveMigrationAddColumnDuringCDC_NoWorkaroundObservesSubjectFailure(t *testing.T) {
	t.Parallel()

	const schema = "test_schema"
	controlFQ := fmt.Sprintf(`"%s"."control_addcol2"`, schema)
	subjectFQ := fmt.Sprintf(`"%s"."subject_addcol2"`, schema)

	lm := newAddColumnDriftLM(t, "obs", controlFQ, subjectFQ)
	defer lm.Cleanup()

	require.NoError(t, lm.SetupContainers(context.Background()))
	require.NoError(t, lm.SetupSchema())
	require.NoError(t, lm.StartExportData(true, nil))
	require.NoError(t, lm.StartImportData(true, nil))
	require.NoError(t, lm.WaitForSnapshotComplete(map[string]int64{
		controlFQ: 5,
		subjectFQ: 5,
	}, 60))
	require.NoError(t, lm.WaitForStreamingMode(120*time.Second, 2*time.Second))

	res, err := lm.RunSchemaDriftPhase(SchemaDriftPhaseConfig{
		SourceDDL: []string{
			fmt.Sprintf(`ALTER TABLE %s.subject_addcol2 ADD COLUMN extra2 TEXT;`, schema),
		},
		SourceDML: []string{
			fmt.Sprintf(`INSERT INTO %s.control_addcol2 (name) VALUES ('c2');`, schema),
			fmt.Sprintf(`INSERT INTO %s.subject_addcol2 (name, extra2) VALUES ('s2', 'e2');`, schema),
		},
		WorkaroundMode: SchemaDriftNoWorkaround,
		ReplicationWaits: []SchemaDriftReplicationWait{
			{Name: "control", Expected: map[string]ChangesCount{controlFQ: {Inserts: 1}}, Timeout: 60 * time.Second, Poll: 2 * time.Second, RequireSuccess: true},
			{Name: "subject", Expected: map[string]ChangesCount{subjectFQ: {Inserts: 1}}, Timeout: 45 * time.Second, Poll: 2 * time.Second, RequireSuccess: false},
		},
	})
	require.NoError(t, err, SchemaDriftStderrTail(lm, 2000))
	if ce := res.AllStreamingCycleErrors(); len(ce) > 0 {
		t.Logf("stop/resume cycle errors (inspect if expected): %v", ce)
	}
	require.Error(t, res.LastAttemptError("subject"), "expected subject wait error without target column; %s", SchemaDriftStderrTail(lm, 2000))

	require.NoError(t, lm.StopExportData())
	require.NoError(t, lm.StopImportData())
}

// TestLiveMigrationAddColumnDuringCDC_WorkaroundAfterFirstFailure applies target DDL only after
// the first failing replication wait, then retries that wait (full stop+resume after every wait).
func TestLiveMigrationAddColumnDuringCDC_WorkaroundAfterFirstFailure(t *testing.T) {
	t.Parallel()

	const schema = "test_schema"
	controlFQ := fmt.Sprintf(`"%s"."control_addcol3"`, schema)
	subjectFQ := fmt.Sprintf(`"%s"."subject_addcol3"`, schema)

	lm := newAddColumnDriftLM(t, "afterfail", controlFQ, subjectFQ)
	defer lm.Cleanup()

	require.NoError(t, lm.SetupContainers(context.Background()))
	require.NoError(t, lm.SetupSchema())
	require.NoError(t, lm.StartExportData(true, nil))
	require.NoError(t, lm.StartImportData(true, nil))
	require.NoError(t, lm.WaitForSnapshotComplete(map[string]int64{
		controlFQ: 5,
		subjectFQ: 5,
	}, 60))
	require.NoError(t, lm.WaitForStreamingMode(120*time.Second, 2*time.Second))

	res, err := lm.RunSchemaDriftPhase(SchemaDriftPhaseConfig{
		SourceDDL: []string{
			fmt.Sprintf(`ALTER TABLE %s.subject_addcol3 ADD COLUMN extra3 TEXT;`, schema),
		},
		SourceDML: []string{
			fmt.Sprintf(`INSERT INTO %s.control_addcol3 (name) VALUES ('c3');`, schema),
			fmt.Sprintf(`INSERT INTO %s.subject_addcol3 (name, extra3) VALUES ('s3', 'e3');`, schema),
		},
		TargetWorkaroundSQL: []string{
			fmt.Sprintf(`ALTER TABLE %s.subject_addcol3 ADD COLUMN extra3 TEXT;`, schema),
		},
		WorkaroundMode: SchemaDriftWorkaroundAfterFirstWaitFailure,
		ReplicationWaits: []SchemaDriftReplicationWait{
			{Name: "control", Expected: map[string]ChangesCount{controlFQ: {Inserts: 1}}, Timeout: 60 * time.Second, Poll: 2 * time.Second, RequireSuccess: true},
			{Name: "subject", Expected: map[string]ChangesCount{subjectFQ: {Inserts: 1}}, Timeout: 45 * time.Second, Poll: 2 * time.Second, RequireSuccess: true},
		},
	})
	require.NoError(t, err, SchemaDriftStderrTail(lm, 2000))
	if ce := res.AllStreamingCycleErrors(); len(ce) > 0 {
		t.Logf("stop/resume cycle errors (inspect if expected): %v", ce)
	}

	require.NoError(t, lm.WithSourceTargetConn(func(_ *sql.DB, tgt *sql.DB) error {
		var extra sql.NullString
		if err := tgt.QueryRow(fmt.Sprintf(`SELECT extra3 FROM %s.subject_addcol3 WHERE name = 's3'`, schema)).Scan(&extra); err != nil {
			return fmt.Errorf("read extra3 on target: %w", err)
		}
		if !extra.Valid || extra.String != "e3" {
			return fmt.Errorf("target extra3 want e3 got valid=%v s=%q", extra.Valid, extra.String)
		}
		return nil
	}))

	require.NoError(t, lm.StopExportData())
	require.NoError(t, lm.StopImportData())
}
