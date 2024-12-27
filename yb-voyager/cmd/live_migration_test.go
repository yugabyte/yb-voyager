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
	"testing"

	"github.com/google/uuid"
	reporter "github.com/yugabyte/yb-voyager/yb-voyager/src/reporter/stats"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type mockYugabyteDB struct {
	tgtdb.TargetYugabyteDB // to satisfy interface
}

func (myb *mockYugabyteDB) ExecuteBatch(migrationUUID uuid.UUID, batch *tgtdb.EventBatch) error {
	return nil
}

func TestProcessEventsBasic(t *testing.T) {
	evChan := make(chan *tgtdb.Event, EVENT_CHANNEL_SIZE)
	lastAppliedVsn := int64(0)
	doneChan := make(chan bool, 1)
	statsReporter := &reporter.StreamImportStatsReporter{}
	state := NewImportDataState(exportDir)
	tdb = &mockYugabyteDB{}
	conflictDetectionCache = NewConflictDetectionCache(utils.NewStructMap[sqlname.NameTuple, []string](), []chan *tgtdb.Event{evChan}, POSTGRESQL)

	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	evChan <- &tgtdb.Event{
		Vsn: 1,
		Op:  "c",
		TableNameTup: sqlname.NameTuple{
			CurrentName: oname,
			TargetName:  oname,
		},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}
	evChan <- END_OF_QUEUE_SEGMENT_EVENT
	processEvents(1, evChan, lastAppliedVsn, doneChan, statsReporter, state)
}

// Test that the event is removed from the conflict detection cache after it is processed
// GIVEN: an event is added to to the conflict detection cache, and is added to the event channel
// WHEN: the event is processed, and successfully applied on the target
// THEN: the event should be removed from the conflict detection cache
func TestProcessEventsRemovesEventFromConflicDetectionCache(t *testing.T) {
	evChan := make(chan *tgtdb.Event, EVENT_CHANNEL_SIZE)
	lastAppliedVsn := int64(0)
	doneChan := make(chan bool, 1)
	statsReporter := &reporter.StreamImportStatsReporter{}
	state := NewImportDataState(exportDir)
	tdb = &mockYugabyteDB{}
	conflictDetectionCache = NewConflictDetectionCache(utils.NewStructMap[sqlname.NameTuple, []string](), []chan *tgtdb.Event{evChan}, POSTGRESQL)

	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	e := &tgtdb.Event{
		Vsn: 1,
		Op:  "c",
		TableNameTup: sqlname.NameTuple{
			CurrentName: oname,
			TargetName:  oname,
		},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE,
	}

	conflictDetectionCache.Put(e)
	evChan <- e
	evChan <- END_OF_QUEUE_SEGMENT_EVENT
	processEvents(1, evChan, lastAppliedVsn, doneChan, statsReporter, state)

	// Check that the event was removed from the cache
	if _, ok := conflictDetectionCache.m[e.Vsn]; ok {
		t.Errorf("Event not removed from conflict detection cache")
	}
}

// Even if event is ignored,
// (because vsn is less than lastAppliedVsn or it is source_db_importer and event is not from target_db_importer_fb),
// it should be removed from conflict detection cache
func TestProcessEventsRemovesIgnoredEventFromConflicDetectionCache(t *testing.T) {
	// to simulate the case where source db importer ignores
	// all events that are not from the target db exporter.
	importerRole = SOURCE_DB_IMPORTER_ROLE
	evChan := make(chan *tgtdb.Event, EVENT_CHANNEL_SIZE)
	lastAppliedVsn := int64(100) // so that event with vsn 1 is ignored.
	doneChan := make(chan bool, 1)
	statsReporter := &reporter.StreamImportStatsReporter{}
	state := NewImportDataState(exportDir)
	tdb = &mockYugabyteDB{}
	conflictDetectionCache = NewConflictDetectionCache(utils.NewStructMap[sqlname.NameTuple, []string](), []chan *tgtdb.Event{evChan}, POSTGRESQL)

	oname := sqlname.NewObjectName(YUGABYTEDB, "public", "public", "users")
	e1 := &tgtdb.Event{
		Vsn: 1, // so that it is less than lastAppliedVsn and ignored.
		Op:  "c",
		TableNameTup: sqlname.NameTuple{
			CurrentName: oname,
			TargetName:  oname,
		},
		ExporterRole: TARGET_DB_EXPORTER_FB_ROLE, // so that it is not ignored because importerRole is SOURCE_DB_IMPORTER_ROLE
	}

	e2 := &tgtdb.Event{
		Vsn: 200, // vsn greater than lastAppliedVSn so that is not ignored
		Op:  "c",
		TableNameTup: sqlname.NameTuple{
			CurrentName: oname,
			TargetName:  oname,
		},
		ExporterRole: SOURCE_DB_EXPORTER_ROLE, // not TARGET_DB_EXPORTER_FB_ROLE so that it is ignored.
	}

	conflictDetectionCache.Put(e1)
	conflictDetectionCache.Put(e2)
	evChan <- e1
	evChan <- e2
	evChan <- END_OF_QUEUE_SEGMENT_EVENT
	processEvents(1, evChan, lastAppliedVsn, doneChan, statsReporter, state)

	// Check that the event was removed from the cache
	if _, ok := conflictDetectionCache.m[e1.Vsn]; ok {
		t.Errorf("Event %v not removed from conflict detection cache", e1)
	}
	if _, ok := conflictDetectionCache.m[e2.Vsn]; ok {
		t.Errorf("Event %v not removed from conflict detection cache", e2)
	}
}
