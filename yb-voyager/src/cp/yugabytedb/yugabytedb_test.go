//go:build integration

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
package yugabytedb

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cp "github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

// controlPlaneSchemaDDL is the schema the control plane owns. Voyager never
// creates it, so the test sets it up before using the control plane.
var controlPlaneSchemaDDL = []string{
	`CREATE SCHEMA voyager_core`,
	`CREATE TABLE voyager_core.migration_state (
		id           bigserial PRIMARY KEY,
		migration_id uuid NOT NULL,
		name         text NOT NULL,
		scope        text NOT NULL DEFAULT '',
		value        boolean NOT NULL DEFAULT true,
		detail       jsonb,
		set_at       timestamptz NOT NULL DEFAULT now()
	)`,
	`CREATE INDEX migration_state_latest_idx
		ON voyager_core.migration_state (migration_id, name, scope, set_at DESC, id DESC)`,
	`CREATE TABLE voyager_core.metadata (
		id              bigserial PRIMARY KEY,
		migration_id    uuid NOT NULL,
		kind            text NOT NULL,
		payload         jsonb NOT NULL,
		payload_version text NOT NULL,
		generated_at    timestamptz NOT NULL DEFAULT now()
	)`,
	`CREATE INDEX metadata_latest_idx
		ON voyager_core.metadata (migration_id, kind, generated_at DESC, id DESC)`,
	`CREATE TABLE voyager_core.table_data_progress (
		migration_id            uuid NOT NULL,
		db_role                 text NOT NULL,
		direction               text NOT NULL,
		schema_name             text NOT NULL,
		table_name              text NOT NULL,
		status                  text NOT NULL DEFAULT 'not_started',
		snapshot_rows_estimated bigint,
		snapshot_rows           bigint NOT NULL DEFAULT 0,
		snapshot_rows_errored   bigint NOT NULL DEFAULT 0,
		cdc_inserts             bigint NOT NULL DEFAULT 0,
		cdc_updates             bigint NOT NULL DEFAULT 0,
		cdc_deletes             bigint NOT NULL DEFAULT 0,
		details                 jsonb,
		updated_at              timestamptz NOT NULL DEFAULT now(),
		PRIMARY KEY (migration_id, db_role, direction, schema_name, table_name)
	)`,
}

func TestYugabyteDBControlPlaneWrites(t *testing.T) {
	ctx := context.Background()

	yugabyteDBContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err := yugabyteDBContainer.Start(ctx)
	if err != nil {
		utils.ErrExit("Failed to start yugabytedb container: %v", err)
	}
	defer testcontainers.TerminateAllContainers()
	assert.NoError(t, err, "Failed to start YugabyteDB container")

	// Connect to the database
	dsn := yugabyteDBContainer.GetConnectionString()
	db, err := sql.Open("pgx", dsn)
	assert.NoError(t, err)
	defer db.Close()

	// Wait for the database to be ready
	err = testutils.WaitForDBToBeReady(db)
	assert.NoError(t, err)
	err = os.Setenv(DB_CONN_STRING_ENV_VAR, dsn)
	assert.NoError(t, err)

	exportDir := filepath.Join(os.TempDir(), "yugabytedb")

	// Create a temporary export directory for testing
	err = os.MkdirAll(exportDir, 0755)
	assert.NoError(t, err, "Failed to create temporary export directory")
	// Ensure the directory is removed after the test
	defer func() {
		err := os.RemoveAll(exportDir)
		assert.NoError(t, err, "Failed to remove temporary export directory")
	}()

	controlPlane := New(exportDir)

	// The control plane must not create any schema objects, so without them
	// Init fails.
	err = controlPlane.Init()
	assert.ErrorContains(t, err, "table voyager_core.migration_state does not exist")
	var schemaCount int
	err = db.QueryRow(`SELECT count(*) FROM information_schema.schemata WHERE schema_name = $1`,
		CONTROL_PLANE_SCHEMA).Scan(&schemaCount)
	assert.NoError(t, err)
	assert.Equal(t, 0, schemaCount, "control plane must not create its schema")

	for _, ddl := range controlPlaneSchemaDDL {
		_, err = db.Exec(ddl)
		assert.NoError(t, err, "Failed to create control plane schema: %s", ddl)
	}
	err = controlPlane.Init()
	require.NoError(t, err, "Failed to initialize the control plane")

	migrationUUID := uuid.New()
	baseEvent := func(eventType string) cp.BaseEvent {
		return cp.BaseEvent{
			EventType:     eventType,
			MigrationUUID: migrationUUID,
			DBType:        "postgresql",
			DatabaseName:  "testdb",
			SchemaNames:   []string{"public", "sales"},
			DBIP:          []string{"10.0.0.1"},
			Port:          5432,
			DBVersion:     "17.2",
		}
	}
	rowCountEvent := func(eventType, table, status string, total, completed int64) cp.BaseUpdateRowCountEvent {
		return cp.BaseUpdateRowCountEvent{
			BaseEvent:         cp.BaseEvent{EventType: eventType, MigrationUUID: migrationUUID, SchemaNames: []string{"public"}},
			TableName:         table,
			Status:            status,
			TotalRowCount:     total,
			CompletedRowCount: completed,
		}
	}
	flush := func() { controlPlane.waitGroup.Wait() }

	// First assessment, then a re-run of it: the re-run must unset the done entry.
	controlPlane.MigrationAssessmentStarted(&cp.MigrationAssessmentStartedEvent{BaseEvent: baseEvent("ASSESS MIGRATION")})
	flush()
	controlPlane.MigrationAssessmentCompleted(&cp.MigrationAssessmentCompletedEvent{
		BaseEvent: baseEvent("ASSESS MIGRATION"),
		Report:    `{"PayloadVersion":"1.8","MigrationComplexity":"LOW"}`,
	})
	flush()
	controlPlane.MigrationAssessmentStarted(&cp.MigrationAssessmentStartedEvent{BaseEvent: baseEvent("ASSESS MIGRATION")})
	flush()

	controlPlane.ExportSchemaStarted(&cp.ExportSchemaStartedEvent{BaseEvent: baseEvent("EXPORT SCHEMA")})
	flush()
	controlPlane.ExportSchemaCompleted(&cp.ExportSchemaCompletedEvent{BaseEvent: baseEvent("EXPORT SCHEMA")})
	flush()

	controlPlane.SchemaAnalysisIterationCompleted(&cp.SchemaAnalysisIterationCompletedEvent{
		BaseEvent:      baseEvent("ANALYZE SCHEMA"),
		AnalysisReport: utils.SchemaReport{VoyagerVersion: "2026.11.1"},
	})
	flush()

	controlPlane.UpdateExportedRowCount([]*cp.UpdateExportedRowCountEvent{
		{BaseUpdateRowCountEvent: rowCountEvent("EXPORT DATA", "orders", "IN PROGRESS", 100, 40)},
		{BaseUpdateRowCountEvent: rowCountEvent("EXPORT DATA", "items", "NOT STARTED", 0, 0)},
	})
	flush()
	controlPlane.UpdateExportedRowCount([]*cp.UpdateExportedRowCountEvent{
		{BaseUpdateRowCountEvent: rowCountEvent("EXPORT DATA", "orders", "COMPLETED", 100, 120)},
	})
	flush()
	controlPlane.UpdateImportedRowCount([]*cp.UpdateImportedRowCountEvent{
		{BaseUpdateRowCountEvent: rowCountEvent("IMPORT DATA", "customers", "IN PROGRESS", 120, 60)},
		{BaseUpdateRowCountEvent: rowCountEvent("IMPORT DATA", `o'brien`, "IN PROGRESS", 10, 5)},
	})
	flush()

	// Events the control plane cannot map are rejected whole: nothing is
	// written for them, under a migration id of their own.
	rejectedUUID := uuid.New()
	rejected := func(eventType string) cp.BaseEvent {
		return cp.BaseEvent{EventType: eventType, MigrationUUID: rejectedUUID}
	}
	controlPlane.ExportSchemaStarted(&cp.ExportSchemaStartedEvent{BaseEvent: rejected("NOT A PHASE")})
	// A document-producing phase must not be marked done without its document,
	// or with a document that carries no version.
	controlPlane.MigrationAssessmentCompleted(&cp.MigrationAssessmentCompletedEvent{
		BaseEvent: rejected("ASSESS MIGRATION"), Report: 42})
	controlPlane.MigrationAssessmentCompleted(&cp.MigrationAssessmentCompletedEvent{
		BaseEvent: rejected("ASSESS MIGRATION"), Report: ""})
	controlPlane.MigrationAssessmentCompleted(&cp.MigrationAssessmentCompletedEvent{
		BaseEvent: rejected("ASSESS MIGRATION"), Report: `{"MigrationComplexity":"LOW"}`})
	controlPlane.UpdateImportedRowCount([]*cp.UpdateImportedRowCountEvent{
		{BaseUpdateRowCountEvent: cp.BaseUpdateRowCountEvent{
			BaseEvent: rejected("IMPORT DATA"), TableName: "bad_status", Status: "PAUSED"}},
	})
	controlPlane.UpdateImportedRowCount([]*cp.UpdateImportedRowCountEvent{
		{BaseUpdateRowCountEvent: cp.BaseUpdateRowCountEvent{
			BaseEvent: rejected("EXPORT SCHEMA"), TableName: "bad_event_type", Status: "IN PROGRESS"}},
	})
	flush()

	t.Run("rejections name their cause", func(t *testing.T) {
		migrationEvent := func(eventType, status, payload string) MigrationEvent {
			return MigrationEvent{MigrationUUID: rejectedUUID, EventType: eventType, Status: status, Payload: payload}
		}
		for _, tc := range []struct {
			event   MigrationEvent
			wantErr string
		}{
			{migrationEvent("NOT A PHASE", STATUS_IN_PROGRESS, ""), `unknown migration event type "NOT A PHASE"`},
			{migrationEvent("ASSESS MIGRATION", "PAUSED", ""), `unknown migration event status "PAUSED"`},
			{migrationEvent("ASSESS MIGRATION", STATUS_COMPLETED, ""), "ASSESS MIGRATION completed without its assessment document"},
			{migrationEvent("ASSESS MIGRATION", STATUS_COMPLETED, `{"MigrationComplexity":"LOW"}`), "payload has no PayloadVersion"},
			{migrationEvent("ASSESS MIGRATION", STATUS_COMPLETED, `not json`), "parse payload"},
		} {
			assert.ErrorContains(t, controlPlane.sendMigrationEvent(tc.event), tc.wantErr)
		}
		for _, tc := range []struct {
			progress TableProgress
			wantErr  string
		}{
			{TableProgress{MigrationUUID: rejectedUUID, EventType: "IMPORT DATA", TableName: "bad_status", Status: "PAUSED"},
				`unknown row count status "PAUSED" for table bad_status`},
			{TableProgress{MigrationUUID: rejectedUUID, EventType: "EXPORT SCHEMA", TableName: "bad_event_type", Status: STATUS_IN_PROGRESS},
				`unknown row count event type "EXPORT SCHEMA" for table bad_event_type`},
		} {
			assert.ErrorContains(t, controlPlane.sendTableDataProgress([]TableProgress{tc.progress}), tc.wantErr)
		}
	})

	// The entry names are spelled out here, not read from migrationPhases, so a
	// typo in that map fails.
	t.Run("state entries per phase", func(t *testing.T) {
		for _, tc := range []struct {
			eventType, payload, startedEntry, doneEntry string
		}{
			{"ASSESS MIGRATION", `{"PayloadVersion":"1.8"}`, "assessment.started", "assessment.completed"},
			{"EXPORT SCHEMA", "", "schema.export_started", "schema.exported"},
			{"ANALYZE SCHEMA", `{}`, "schema.analysis_started", "schema.analyzed"},
			{"EXPORT DATA", "", "data.export_started", "data.exported"},
			{"IMPORT SCHEMA", "", "schema.import_started", "schema.imported"},
			{"IMPORT DATA", "", "data.import_started", "data.imported"},
		} {
			phaseUUID := uuid.New()
			require.NoError(t, controlPlane.sendMigrationEvent(MigrationEvent{
				MigrationUUID: phaseUUID, EventType: tc.eventType, Status: STATUS_IN_PROGRESS}))
			require.NoError(t, controlPlane.sendMigrationEvent(MigrationEvent{
				MigrationUUID: phaseUUID, EventType: tc.eventType, Status: STATUS_COMPLETED, Payload: tc.payload}))
			var names string
			err := db.QueryRow(`SELECT string_agg(name, ',' ORDER BY set_at, id)
				FROM voyager_core.migration_state WHERE migration_id = $1 AND value`, phaseUUID).Scan(&names)
			require.NoError(t, err)
			assert.Equal(t, tc.startedEntry+","+tc.doneEntry, names, tc.eventType)
		}
	})

	// A statement that fails on every attempt must surface its error.
	err = controlPlane.exec(`SELECT 1/0`)
	assert.ErrorContains(t, err, "division by zero")

	controlPlane.Finalize()

	type stateEntry struct {
		name  string
		value bool
	}

	t.Run("rejected events write nothing", func(t *testing.T) {
		for _, table := range []string{"migration_state", "metadata", "table_data_progress"} {
			var count int
			err := db.QueryRow(`SELECT count(*) FROM voyager_core.`+table+` WHERE migration_id = $1`, rejectedUUID).Scan(&count)
			assert.NoError(t, err)
			assert.Equal(t, 0, count, "rows in %s for a rejected event", table)
		}
	})

	t.Run("migration_state history", func(t *testing.T) {
		rows, err := db.Query(`SELECT name, value, scope, detail IS NULL
			FROM voyager_core.migration_state WHERE migration_id = $1 ORDER BY set_at, id`, migrationUUID)
		assert.NoError(t, err)
		defer rows.Close()
		var got []stateEntry
		for rows.Next() {
			var e stateEntry
			var scope string
			var detailIsNull bool
			assert.NoError(t, rows.Scan(&e.name, &e.value, &scope, &detailIsNull))
			assert.Equal(t, "", scope)
			assert.True(t, detailIsNull, "no detail is reported for %s", e.name)
			got = append(got, e)
		}
		assert.NoError(t, rows.Err())
		if !assert.Len(t, got, 7) {
			return
		}
		assert.Equal(t, []stateEntry{
			{"assessment.started", true},
			{"assessment.completed", true},
		}, got[:2])
		// The re-run's started and unset rows come from one statement, so they
		// share a set_at and their relative order is not defined.
		assert.ElementsMatch(t, []stateEntry{
			{"assessment.started", true},
			{"assessment.completed", false},
		}, got[2:4])
		assert.Equal(t, []stateEntry{
			{"schema.export_started", true},
			{"schema.exported", true},
			{"schema.analyzed", true},
		}, got[4:])
	})

	t.Run("migration_state current", func(t *testing.T) {
		rows, err := db.Query(`SELECT DISTINCT ON (name, scope) name, value
			FROM voyager_core.migration_state WHERE migration_id = $1
			ORDER BY name, scope, set_at DESC, id DESC`, migrationUUID)
		assert.NoError(t, err)
		defer rows.Close()
		got := map[string]bool{}
		for rows.Next() {
			var name string
			var value bool
			assert.NoError(t, rows.Scan(&name, &value))
			got[name] = value
		}
		assert.NoError(t, rows.Err())
		assert.Equal(t, map[string]bool{
			"assessment.started":    true,
			"assessment.completed":  false,
			"schema.export_started": true,
			"schema.exported":       true,
			"schema.analyzed":       true,
		}, got)
	})

	t.Run("metadata", func(t *testing.T) {
		rows, err := db.Query(`SELECT kind, payload_version, payload->>'MigrationComplexity'
			FROM voyager_core.metadata WHERE migration_id = $1 ORDER BY generated_at, id`, migrationUUID)
		assert.NoError(t, err)
		defer rows.Close()
		var got [][3]string
		for rows.Next() {
			var kind, version string
			var complexity sql.NullString
			assert.NoError(t, rows.Scan(&kind, &version, &complexity))
			got = append(got, [3]string{kind, version, complexity.String})
		}
		assert.NoError(t, rows.Err())
		assert.Equal(t, [][3]string{
			{"assessment", "1.8", "LOW"},
			{"schema_analysis", SCHEMA_ANALYSIS_PAYLOAD_VERSION, ""},
		}, got)
	})

	t.Run("table_data_progress", func(t *testing.T) {
		rows, err := db.Query(`SELECT db_role, direction, schema_name, table_name, status, snapshot_rows_estimated, snapshot_rows
			FROM voyager_core.table_data_progress WHERE migration_id = $1
			ORDER BY direction, table_name`, migrationUUID)
		assert.NoError(t, err)
		defer rows.Close()
		type progress struct {
			role, direction, schema, table, status string
			estimated                              sql.NullInt64
			rows                                   int64
		}
		var got []progress
		for rows.Next() {
			var p progress
			assert.NoError(t, rows.Scan(&p.role, &p.direction, &p.schema, &p.table, &p.status, &p.estimated, &p.rows))
			got = append(got, p)
		}
		assert.NoError(t, rows.Err())
		assert.Equal(t, []progress{
			{"source", "export", "public", "items", "not_started", sql.NullInt64{Int64: 0, Valid: true}, 0},
			{"source", "export", "public", "orders", "completed", sql.NullInt64{Int64: 100, Valid: true}, 120},
			{"target", "import", "public", "customers", "in_progress", sql.NullInt64{Int64: 120, Valid: true}, 60},
			{"target", "import", "public", `o'brien`, "in_progress", sql.NullInt64{Int64: 10, Valid: true}, 5},
		}, got)
	})
}

func TestYugabyteDBStructs(t *testing.T) {
	// Test the structs used in YugabyteDB

	expectedVoyagerInstance := struct {
		IP                 string
		OperatingSystem    string
		DiskSpaceAvailable uint64
		ExportDirectory    string
	}{}

	t.Run("Validate VoyagerInstance Struct Definition", func(t *testing.T) {
		testutils.CompareStructs(t, reflect.TypeOf(cp.VoyagerInstance{}), reflect.TypeOf(expectedVoyagerInstance), "VoyagerInstance")
	})

	expectedMigrationEvent := struct {
		MigrationUUID uuid.UUID `json:"migration_uuid"`
		EventType     string    `json:"event_type"`
		Payload       string    `json:"payload"`
		Status        string    `json:"status"`
	}{}

	t.Run("Validate MigrationEvent Struct Definition", func(t *testing.T) {
		testutils.CompareStructs(t, reflect.TypeOf(MigrationEvent{}), reflect.TypeOf(expectedMigrationEvent), "MigrationEvent")
	})

	expectedTableProgress := struct {
		MigrationUUID     uuid.UUID
		EventType         string
		SchemaName        string
		TableName         string
		Status            string
		TotalRowCount     int64
		CompletedRowCount int64
	}{}

	t.Run("Validate TableProgress Struct Definition", func(t *testing.T) {
		testutils.CompareStructs(t, reflect.TypeOf(TableProgress{}), reflect.TypeOf(expectedTableProgress), "TableProgress")
	})

	expectedYugabyteDB := struct {
		sync.Mutex
		waitGroup               sync.WaitGroup
		eventChan               chan (MigrationEvent)
		rowCountUpdateEventChan chan ([]TableProgress)
		connMutex               sync.Mutex
		connPool                *pgxpool.Pool
		lastRowCountUpdate      map[string]time.Time
	}{}

	t.Run("Validate YugabyteDB Struct Definition", func(t *testing.T) {
		testutils.CompareStructs(t, reflect.TypeOf(&YugabyteDB{}).Elem(), reflect.TypeOf(&expectedYugabyteDB).Elem(), "YugabyteDB")
	})
}
