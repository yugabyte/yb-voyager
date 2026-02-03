# YB-Voyager Failure Injection Test Plan

**Version**: 1.0  
**Date**: February 3, 2026  
**Status**: Planning Phase

---

## Table of Contents

1. [Overview](#overview)
2. [Objectives](#objectives)
3. [Test Categories](#test-categories)
4. [Phase 1: Export-Data Tests](#phase-1-export-data-tests)
5. [Phase 2: Import-Data Tests](#phase-2-import-data-tests)
6. [Implementation Roadmap](#implementation-roadmap)
7. [Technical Requirements](#technical-requirements)
8. [Success Criteria](#success-criteria)
9. [Appendix](#appendix)

---

## Overview

This document outlines a comprehensive test plan for validating YB-Voyager's resilience and durability guarantees during data migration. The tests use failure injection techniques (Byteman for Java, Failpoint for Go) to simulate various failure scenarios and verify that the system:

1. **Never loses data** (durability)
2. **Recovers gracefully** from failures (resilience)
3. **Handles duplicates** correctly (idempotency)
4. **Provides clear error messages** (observability)

---

## Objectives

### Primary Goals
1. **Validate Durability**: Ensure no data loss under any failure scenario
2. **Validate Recovery**: Verify resume/restart functionality works correctly
3. **Validate Deduplication**: Confirm event deduplication prevents duplicate processing
4. **Validate Error Handling**: Ensure clear, actionable error messages

### Secondary Goals
1. Document failure recovery behavior
2. Establish baseline for future regression testing
3. Identify and fix edge cases in failure handling
4. Build confidence in production deployments

---

## Test Categories

### Category 1: Export-Data CDC Tests
Tests for failures during CDC (Change Data Capture) export phase.

**Failure Points Covered:**
- Batch processing entry (before batch starts)
- Event processing (during transformation)
- Queue writing (I/O operations)
- Offset commits (state persistence)
- Phase transitions (snapshot → CDC)

### Category 2: Export-Data Snapshot Tests
Tests for failures during snapshot export phase.

**Failure Points Covered:**
- Table snapshot export
- Snapshot completion
- Snapshot-to-CDC transition

### Category 3: Import-Data Tests
Tests for failures during data import to target database.

**Failure Points Covered:**
- Event import processing
- Batch commits
- Data transformations
- Database errors (transient and permanent)
- Partial batch failures

---

## Phase 1: Export-Data Tests

### Test 1.1: CDC Batch Processing Failure (Mid-Stream) ✅

**Status**: ✅ **COMPLETED**

**File**: `yb-voyager/cmd/export_data_cdc_batch_failure_resume_test.go`

**Objective**: Validate recovery when a CDC batch fails mid-stream (after some batches have been successfully processed).

**Scenario**:
1. Start CDC export (snapshot-and-changes mode)
2. Complete snapshot phase (50 rows exported to data files)
3. Process first CDC batch successfully (20 events → queue segments)
4. **Inject failure on 2nd CDC batch** (before batch processing starts)
5. Process crashes
6. Resume export (3rd batch of 20 events also pending)
7. Verify all CDC events recovered (60 total)
8. Verify no duplicate events (deduplication working)

**Technical Details**:
```go
// Byteman Rule
bytemanHelper.AddRuleFromBuilder(
    testutils.NewRule("fail_cdc_batch_2").
        AtMarker(testutils.MarkerCDC, "before-batch").
        If("incrementCounter(\"cdc_batch\") == 2").
        ThrowException("java.lang.RuntimeException", 
            "TEST: Simulated batch processing failure on batch 2"),
)
```

**Injection Point**: `BytemanMarkers.cdc("before-batch")` - counter == 2

**Expected Results**:
- After failure: 20 CDC events in queue (batch 1 only)
- After resume: 60 CDC events in queue (all 3 batches)
- No duplicate VSNs (event IDs)
- Source row count: 110 (50 snapshot + 60 CDC)

**Validates**:
- ✅ Mid-stream recovery (some state committed)
- ✅ Debezium offset-based replay
- ✅ Event deduplication (VSN-based)
- ✅ Graceful crash handling

---

### Test 1.2: First CDC Batch Failure (Cold Start Durability)

**Status**: ❌ **NOT STARTED** (needs debugging)

**File**: `yb-voyager/cmd/export_data_first_cdc_batch_failure_test.go`

**Objective**: Validate recovery when the very first CDC batch fails (no CDC offsets committed yet).

**Scenario**:
1. Start CDC export (snapshot-and-changes mode)
2. Complete snapshot phase (50 rows)
3. First CDC batch arrives (50 events)
4. **Inject failure on 1st CDC batch** (before processing)
5. Process crashes
6. Resume export
7. Verify all CDC events recovered (50 total)
8. Verify no duplicate events

**Technical Details**:
```go
// Byteman Rule
bytemanHelper.AddRuleFromBuilder(
    testutils.NewRule("fail_first_cdc_batch").
        AtMarker(testutils.MarkerCDC, "before-batch").
        If("incrementCounter(\"first_batch_counter\") == 1").
        ThrowException("java.io.IOException", 
            "TEST: Simulated failure on first CDC batch"),
)
```

**Injection Point**: `BytemanMarkers.cdc("before-batch")` - counter == 1

**Expected Results**:
- After failure: **0 CDC events** in queue (no CDC state)
- After resume: 50 CDC events in queue (full replay from beginning)
- No duplicate VSNs
- Source row count: 100 (50 snapshot + 50 CDC)

**Validates**:
- ✅ "Cold start" recovery (no prior CDC state)
- ✅ Recovery from zero CDC offset
- ✅ Full CDC replay capability

**Difference from Test 1.1**:
| Aspect | Test 1.1 | Test 1.2 |
|--------|----------|----------|
| Failure point | 2nd batch | 1st batch |
| CDC offsets before failure | Some committed | None committed |
| Resume behavior | Resume from offset | Replay from beginning |
| Scenario | Mid-stream | Cold start |

**Known Issues**:
- Previous attempt failed (injection didn't trigger)
- Need to debug: Check Byteman loading, CDC phase start, marker execution

**Next Steps**:
1. Add debug logging to verify Byteman rules loaded
2. Dump Debezium logs to see if CDC batches are processed
3. Verify `before-batch` marker is being hit
4. Check if counter logic is correct

---

### Test 1.3: Offset Commit Failure (Deduplication Test)

**Status**: ❌ **NOT STARTED** (requires new Byteman marker)

**File**: `yb-voyager/cmd/export_data_offset_commit_failure_test.go`

**Objective**: Validate durability and deduplication when offset commit fails after events are successfully written.

**Scenario**:
1. Start CDC export
2. Complete snapshot phase (30 rows)
3. CDC batch arrives (20 events)
4. Events are processed successfully ✅
5. Events are written to queue segments ✅
6. **Inject failure during offset commit** ❌
7. Process crashes
8. Resume export
9. Same CDC batch replayed (offset wasn't committed)
10. Verify events NOT duplicated in queue (deduplication works)

**Technical Details**:
```go
// Byteman Rule (after adding new marker)
bytemanHelper.AddRuleFromBuilder(
    testutils.NewRule("fail_offset_commit").
        AtMarker(testutils.MarkerCDC, "before-offset-commit").
        If("incrementCounter(\"commit_counter\") == 1").
        ThrowException("java.io.IOException", 
            "TEST: Simulated offset commit failure"),
)
```

**Injection Point**: `BytemanMarkers.cdc("before-offset-commit")` ⚠️ **NEW MARKER REQUIRED**

**Expected Results**:
- After failure: 20 CDC events in queue (written successfully)
- After resume: Still 20 CDC events (duplicates filtered by VSN)
- Deduplication cache prevents rewriting same events
- Final row count correct

**Validates**:
- ✅ At-least-once delivery semantics
- ✅ Event deduplication (idempotency)
- ✅ Graceful handling of replayed events
- ✅ VSN-based duplicate detection

**Why This is Critical**:
This is the **classic distributed systems problem**:
- Events written to durable storage ✅
- Coordination state (offset) fails to commit ❌
- System must handle replay without creating duplicates

**Code Changes Required**:
```java
// In YbExporterConsumer.java - handleBatch()

handleBatchComplete();  // Events written and fsynced

// ✨ ADD THIS MARKER
BytemanMarkers.cdc("before-offset-commit");

// Commit offsets
for (ChangeEvent<Object, Object> event : changeEvents) {
    committer.markProcessed(event);
}
committer.markBatchFinished();
```

---

### Test 1.4: Queue Write Failure (I/O Error Simulation)

**Status**: ❌ **NOT STARTED** (requires new Byteman marker)

**File**: `yb-voyager/cmd/export_data_queue_write_failure_test.go`

**Objective**: Validate recovery when writing to queue segments fails (disk full, I/O errors, permission issues).

**Scenario**:
1. Start CDC export
2. Complete snapshot phase (30 rows)
3. CDC batch arrives (20 events)
4. Events are processed successfully ✅
5. **Inject failure when writing to queue** ❌
6. Process crashes
7. Resume export
8. Events replayed and written successfully

**Technical Details**:
```go
// Byteman Rule (after adding new marker)
bytemanHelper.AddRuleFromBuilder(
    testutils.NewRule("fail_queue_write").
        AtMarker(testutils.MarkerCDC, "before-write-record").
        If("incrementCounter(\"write_counter\") == 10").
        ThrowException("java.io.IOException", 
            "TEST: Simulated queue write failure (disk I/O error)"),
)
```

**Injection Point**: `BytemanMarkers.cdc("before-write-record")` ⚠️ **NEW MARKER REQUIRED**

**Expected Results**:
- After failure: < 20 CDC events in queue (partial write, then failure)
- No offset committed (write failed)
- After resume: 20 CDC events in queue (full batch replayed)
- No duplicates (deduplication or offset logic prevents)

**Validates**:
- ✅ I/O error handling
- ✅ Partial write recovery
- ✅ No offset commit on write failure
- ✅ Full batch replay on resume

**Realistic Scenarios This Covers**:
- Disk full during export
- Network storage disconnection
- File system permission errors
- Disk I/O errors

**Code Changes Required**:
```java
// In YbExporterConsumer.java - handleBatch()

for (ChangeEvent<Object, Object> event : changeEvents) {
    BytemanMarkers.cdc("before-process-record");
    
    // Process event
    Record r = processRecord(event);
    
    // ✨ ADD THIS MARKER
    BytemanMarkers.cdc("before-write-record");
    
    // Write to queue
    writer.writeRecord(r);
    
    BytemanMarkers.cdc("after-process-record");
}
```

---

### Test 1.5: Snapshot Phase Failure

**Status**: ❌ **NOT STARTED** (needs investigation of snapshot markers)

**File**: `yb-voyager/cmd/export_data_snapshot_failure_test.go`

**Objective**: Validate recovery when snapshot export fails mid-process.

**Scenario**:
1. Start export in snapshot-and-changes mode
2. Begin snapshot of table with 100 rows
3. **Inject failure after 50 rows exported**
4. Process crashes
5. Resume export
6. Verify all 100 rows eventually exported
7. Verify CDC phase starts correctly after snapshot recovery

**Technical Details**:
```go
// Byteman Rule (need to identify correct marker)
bytemanHelper.AddRuleFromBuilder(
    testutils.NewRule("fail_snapshot_mid_export").
        AtMarker(testutils.MarkerSnapshot, "after-batch").
        If("incrementCounter(\"snapshot_batch\") == 5").
        ThrowException("java.lang.RuntimeException", 
            "TEST: Simulated snapshot export failure"),
)
```

**Injection Point**: Need to investigate available snapshot markers:
- `BytemanMarkers.snapshot("before-batch")`?
- `BytemanMarkers.snapshot("after-batch")`?
- `BytemanMarkers.snapshot("before-complete")`?

**Expected Results**:
- After failure: Partial snapshot data in data files
- After resume: Full snapshot data (100 rows)
- CDC phase starts after snapshot completes
- No duplicate rows in snapshot data

**Validates**:
- ✅ Snapshot resumption
- ✅ Partial snapshot recovery
- ✅ Snapshot-to-CDC transition after recovery

**Investigation Required**:
1. Search for snapshot markers in code
2. Understand snapshot state tracking
3. Verify snapshot resume logic exists

---

### Test 1.6: Snapshot-to-CDC Transition Failure

**Status**: ❌ **NOT STARTED** (advanced scenario)

**File**: `yb-voyager/cmd/export_data_transition_failure_test.go`

**Objective**: Validate recovery when failure occurs during the critical transition from snapshot to CDC phase.

**Scenario**:
1. Start export in snapshot-and-changes mode
2. Complete snapshot phase (100 rows)
3. **Inject failure during transition to CDC phase**
4. Process crashes
5. Resume export
6. Verify CDC phase starts correctly
7. Verify no CDC events missed during transition

**Technical Details**:
```go
// Byteman Rule (need to identify transition marker)
bytemanHelper.AddRuleFromBuilder(
    testutils.NewRule("fail_transition").
        AtMarker(testutils.MarkerSnapshot, "before-complete").
        ThrowException("java.lang.RuntimeException", 
            "TEST: Simulated transition failure"),
)
```

**Injection Point**: Marker at snapshot completion / CDC initiation

**Expected Results**:
- After failure: Snapshot complete, CDC not started
- After resume: CDC phase starts successfully
- No CDC events missed (replication slot preserved)

**Validates**:
- ✅ Critical transition point handling
- ✅ Replication slot preservation
- ✅ No event loss during transition

**Why This is Critical**:
The snapshot-to-CDC transition is a critical moment where:
- Snapshot has completed
- CDC must start from the snapshot's consistent point
- Any failure could cause event loss or duplication

---

### Test 1.7: Multiple Batch Failures

**Status**: ❌ **NOT STARTED** (advanced scenario)

**File**: `yb-voyager/cmd/export_data_multiple_batch_failures_test.go`

**Objective**: Validate recovery when multiple consecutive batches fail.

**Scenario**:
1. Start CDC export
2. Batch 1 succeeds (20 events)
3. Batch 2 fails (inject failure)
4. Resume, batch 2 succeeds (20 events)
5. Batch 3 fails (inject failure again)
6. Resume, batch 3 succeeds (20 events)
7. Verify all 60 events present, no duplicates

**Validates**:
- ✅ Repeated recovery
- ✅ Resilience under multiple failures
- ✅ Consistent deduplication across restarts

---

## Phase 2: Import-Data Tests

### Test 2.1: Event Import Failure (Mid-Batch)

**Status**: ❌ **NOT STARTED**

**File**: `yb-voyager/cmd/import_data_event_failure_test.go`

**Objective**: Validate recovery when event import fails mid-batch.

**Scenario**:
1. Export CDC events to queue (60 events)
2. Start import process
3. Import 10 events successfully
4. **Inject failure on 11th event**
5. Process crashes
6. Resume import
7. Verify all 60 events imported (batch replayed)

**Technical Details**:
```go
// Go Failpoint
fpEnv := testutils.GetFailpointEnvVar(
    "github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importEventProcessError=10*off->return()",
)
```

**Injection Point**: Go failpoint in event processing loop

**Expected Results**:
- After failure: < 60 events imported
- Batch rolled back (transactional)
- After resume: All 60 events imported
- No duplicate events in target DB

**Validates**:
- ✅ Import resume functionality
- ✅ Event replay on failure
- ✅ No partial batch commits

---

### Test 2.2: Batch Commit Failure

**Status**: ⚠️ **PARTIALLY IMPLEMENTED** (extend existing test)

**File**: `yb-voyager/cmd/import_data_failpoint_test.go` (existing)

**Objective**: Validate recovery when batch commit fails.

**Current State**:
- Test already exists: `TestImportDataWithFailpoint`
- Tests commit error injection
- Validates batch rollback and resume

**Enhancements Needed**:
1. Add more comprehensive assertions
2. Test with CDC events (currently uses snapshot data)
3. Add deduplication validation
4. Test with multiple batch commit failures

**Technical Details**:
```go
// Existing failpoint
fpEnv := testutils.GetFailpointEnvVar(
    "github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/importBatchCommitError=4*off->return()",
)
```

**Validates**:
- ✅ Batch commit failure handling
- ✅ Transactional rollback
- ✅ Resume from last successful commit

---

### Test 2.3: Transformation Failure

**Status**: ❌ **NOT STARTED**

**File**: `yb-voyager/cmd/import_data_transformation_failure_test.go`

**Objective**: Validate recovery when data transformation fails during import.

**Scenario**:
1. Export data with columns requiring transformation
2. Start import (e.g., type conversions, encoding changes)
3. **Inject failure in transformation logic**
4. Process crashes
5. Resume import
6. Verify transformation completes successfully

**Technical Details**:
```go
// Go Failpoint (need to identify transformation code path)
fpEnv := testutils.GetFailpointEnvVar(
    "github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/transformationError=5*off->return()",
)
```

**Expected Results**:
- After failure: Import crashed during transformation
- After resume: Transformation completes, all data imported correctly

**Validates**:
- ✅ Transformation error handling
- ✅ Recovery from transformation failures
- ✅ Data integrity after transformation resume

**Test Data Examples**:
- VARCHAR to TEXT conversions
- Timestamp format changes
- Numeric precision adjustments
- Encoding conversions (UTF-8, etc.)

---

### Test 2.4: Database Error - Transient (Connection Errors)

**Status**: ❌ **NOT STARTED**

**File**: `yb-voyager/cmd/import_data_transient_db_error_test.go`

**Objective**: Validate retry logic for transient database errors.

**Scenario**:
1. Start import process
2. **Inject transient connection error** (network timeout)
3. Verify automatic retry occurs
4. Import succeeds after retry
5. Verify all data imported correctly

**Technical Details**:
```go
// Go Failpoint - transient error
fpEnv := testutils.GetFailpointEnvVar(
    "github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/connectionError=1*return()",
)
```

**Expected Results**:
- Import encounters connection error
- Automatic retry succeeds
- All data imported (no manual intervention)

**Validates**:
- ✅ Retry logic for transient errors
- ✅ Exponential backoff (if implemented)
- ✅ Eventual consistency

**Error Types to Test**:
- Connection timeouts
- Network errors
- Temporary database unavailability
- Connection pool exhaustion

---

### Test 2.5: Database Error - Permanent (Constraint Violations)

**Status**: ❌ **NOT STARTED**

**File**: `yb-voyager/cmd/import_data_permanent_db_error_test.go`

**Objective**: Validate graceful failure and clear error reporting for permanent errors.

**Scenario**:
1. Create table with constraints (unique, check, foreign key)
2. Export data that will violate constraints
3. Start import
4. **Constraint violation occurs**
5. Verify clear error message
6. Verify graceful failure (no corruption)

**Technical Details**:
```sql
-- Test schema with constraints
CREATE TABLE test_table (
    id INT PRIMARY KEY,
    email VARCHAR UNIQUE,
    age INT CHECK (age >= 0),
    parent_id INT REFERENCES parent_table(id)
);
```

**Expected Results**:
- Import fails with clear error message
- Error identifies: table, row, constraint, violation type
- No partial data committed (batch rolled back)
- User can fix data and retry

**Validates**:
- ✅ Clear error reporting
- ✅ Graceful failure (no corruption)
- ✅ Batch rollback on constraint violation
- ✅ Actionable error messages

**Error Types to Test**:
- Unique constraint violations
- Check constraint violations
- Foreign key violations
- NOT NULL violations
- Data type mismatches

---

### Test 2.6: Partial Batch Failure

**Status**: ❌ **NOT STARTED**

**File**: `yb-voyager/cmd/import_data_partial_batch_failure_test.go`

**Objective**: Validate that partial batch failures result in full batch rollback (transactional behavior).

**Scenario**:
1. Prepare batch of 10 rows
2. Start import
3. **Inject failure on 5th row** of batch
4. Verify entire batch rolled back (all 10 rows)
5. Resume import
6. Verify all 10 rows imported (full batch replay)

**Technical Details**:
```go
// Go Failpoint
fpEnv := testutils.GetFailpointEnvVar(
    "github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb/batchRowError=5*off->return()",
)
```

**Expected Results**:
- After failure: 0 rows from that batch in DB (full rollback)
- After resume: All 10 rows from batch in DB
- No partial commits

**Validates**:
- ✅ Transactional batch processing
- ✅ Full batch rollback on any row failure
- ✅ Atomic batch commits (all-or-nothing)

**Why This is Critical**:
- Ensures consistency (no partial batches)
- Simplifies recovery (replay full batch)
- Prevents data corruption

---

## Implementation Roadmap

### Phase 0: Preparation (Week 1)

**Tasks**:
1. ✅ Create comprehensive test plan document (this document)
2. Identify all required Byteman markers
3. Add new Byteman markers to Java code
4. Set up test infrastructure improvements

**Deliverables**:
- [ ] Test plan reviewed and approved
- [ ] New Byteman markers added:
  - [ ] `BytemanMarkers.cdc("before-offset-commit")`
  - [ ] `BytemanMarkers.cdc("before-write-record")`
  - [ ] Snapshot markers (if missing)
- [ ] Test helper utilities enhanced

---

### Phase 1: Quick Wins (Weeks 2-3)

**Goal**: Implement tests with existing infrastructure

**Tests**:
1. [ ] Test 1.2: First CDC Batch Failure (debug and fix)
2. [ ] Test 2.2: Batch Commit Failure (extend existing test)
3. [ ] Test 1.5: Snapshot Phase Failure (if markers exist)

**Success Criteria**:
- 3 new tests passing
- CI/CD integration complete
- Documentation updated

---

### Phase 2: Core Export Tests (Weeks 4-5)

**Goal**: Complete critical export-data durability tests

**Tests**:
1. [ ] Test 1.3: Offset Commit Failure
2. [ ] Test 1.4: Queue Write Failure
3. [ ] Test 1.6: Snapshot-to-CDC Transition

**Success Criteria**:
- All critical durability scenarios covered
- Deduplication thoroughly tested
- Edge cases documented

---

### Phase 3: Import Tests (Weeks 6-7)

**Goal**: Complete import-data failure tests

**Tests**:
1. [ ] Test 2.1: Event Import Failure
2. [ ] Test 2.3: Transformation Failure
3. [ ] Test 2.4: Transient DB Errors
4. [ ] Test 2.5: Permanent DB Errors
5. [ ] Test 2.6: Partial Batch Failure

**Success Criteria**:
- All import failure scenarios covered
- Error messages validated
- Retry logic tested

---

### Phase 4: Advanced Scenarios (Week 8+)

**Goal**: Test complex and edge case scenarios

**Tests**:
1. [ ] Test 1.7: Multiple Batch Failures
2. [ ] Cascading failures (export + import failures)
3. [ ] Stress tests (repeated failures)
4. [ ] Performance impact of retries

**Success Criteria**:
- Advanced scenarios documented
- System behavior under stress validated

---

## Technical Requirements

### Code Changes Required

#### Java Code Changes (debezium-server-voyager)

**File**: `debezium-server-voyager/debezium-server-voyagerexporter/src/main/java/io/debezium/server/ybexporter/YbExporterConsumer.java`

**Changes Needed**:

```java
public void handleBatch(List<ChangeEvent<Object, Object>> changeEvents,
                       DebeziumEngine.RecordCommitter<ChangeEvent<Object, Object>> committer) 
                       throws InterruptedException {
    
    BytemanMarkers.cdc("before-batch");
    
    for (ChangeEvent<Object, Object> event : changeEvents) {
        BytemanMarkers.cdc("before-process-record");
        
        // ... process event ...
        Record r = convertToRecord(event);
        
        // ✨ NEW MARKER - Test 1.4
        BytemanMarkers.cdc("before-write-record");
        
        if (checkIfEventNeedsToBeWritten(r)) {
            writer.writeRecord(r);
        }
        
        checkIfSnapshotComplete(r);
        BytemanMarkers.cdc("after-process-record");
    }
    
    handleBatchComplete();
    
    // ✨ NEW MARKER - Test 1.3
    BytemanMarkers.cdc("before-offset-commit");
    
    // Mark events as processed (commit offsets)
    for (ChangeEvent<Object, Object> event : changeEvents) {
        committer.markProcessed(event);
    }
    committer.markBatchFinished();
    
    handleSnapshotOnlyComplete();
    BytemanMarkers.cdc("after-batch");
}
```

**Snapshot Markers** (need to verify existence):
- Investigation needed for snapshot phase markers
- May need to add markers in snapshot handler code

---

#### Go Code Changes (yb-voyager)

**File**: `yb-voyager/src/tgtdb/batch_import.go` (or similar)

**Changes Needed**:

Add failpoint markers for import tests:

```go
import "github.com/pingcap/failpoint"

func (bi *BatchImporter) ProcessEvent(event *Event) error {
    // Failpoint for Test 2.1
    failpoint.Inject("importEventProcessError", func() {
        failpoint.Return(fmt.Errorf("failpoint: event process error"))
    })
    
    // ... existing event processing ...
    
    // Failpoint for Test 2.3
    failpoint.Inject("transformationError", func() {
        failpoint.Return(fmt.Errorf("failpoint: transformation error"))
    })
    
    return nil
}

func (bi *BatchImporter) CommitBatch() error {
    // Existing failpoint for Test 2.2
    failpoint.Inject("importBatchCommitError", func() {
        failpoint.Return(fmt.Errorf("failpoint: batch commit error"))
    })
    
    return bi.db.Commit()
}

func (bi *BatchImporter) ProcessBatchRow(row int) error {
    // Failpoint for Test 2.6
    failpoint.Inject("batchRowError", func() {
        failpoint.Return(fmt.Errorf("failpoint: row processing error"))
    })
    
    return nil
}
```

---

### Test Infrastructure Requirements

#### 1. Byteman Helper Enhancements

**File**: `yb-voyager/test/utils/byteman_helper.go`

**Enhancements Needed**:
- [ ] Add helper for debugging (dump rules, logs)
- [ ] Add helper for verifying marker execution
- [ ] Improve error messages when injection doesn't trigger
- [ ] Add timeout handling for long-running tests

**Example**:
```go
// Add to BytemanHelper
func (bh *BytemanHelper) DumpDebugInfo(t *testing.T) {
    // Dump Byteman rules
    rules, _ := os.ReadFile(bh.rulesFilePath)
    t.Logf("Byteman Rules:\n%s", string(rules))
    
    // Dump last N lines of Debezium logs
    logs := bh.GetLastLogLines(100)
    t.Logf("Last 100 Debezium log lines:\n%s", logs)
}
```

---

#### 2. Event Counting Utilities

**File**: `yb-voyager/cmd/export_data_cdc_batch_failure_resume_test.go`

**Current Implementation**:
```go
func countEventsInQueueSegments(exportDir string) (int, error) {
    // Counts CDC events in queue segments
}

func verifyNoEventDuplicates(t *testing.T, exportDir string) {
    // Verifies all VSNs are unique
}
```

**Enhancements Needed**:
- [ ] Add helper to count snapshot events
- [ ] Add helper to verify event ordering
- [ ] Add helper to compare source vs. exported row counts
- [ ] Add helper to verify data consistency

---

#### 3. Test Data Generators

**New Utilities Needed**:

```go
// Generate test data with specific characteristics
type TestDataGenerator struct {
    NumRows        int
    NumBatches     int
    BatchInterval  time.Duration
    Schema         string
    Table          string
}

func (tdg *TestDataGenerator) GenerateSnapshot(container *testcontainers.TestContainer) error {
    // Generate initial snapshot data
}

func (tdg *TestDataGenerator) GenerateCDCEvents(container *testcontainers.TestContainer, batchFunc func(int)) error {
    // Generate CDC events in controlled batches
}
```

---

### CI/CD Integration

#### GitHub Actions Workflow

**File**: `.github/workflows/failpoint-tests.yml`

**Current State**: Runs failpoint tests with `gotestfmt`

**Enhancements Needed**:

```yaml
name: Failure Injection Tests

on:
  pull_request:
    paths:
      - 'yb-voyager/**'
      - 'debezium-server-voyager/**'
  push:
    branches: [main, develop]
  schedule:
    - cron: '0 2 * * *'  # Nightly run

jobs:
  export-tests:
    name: Export-Data Failure Tests
    runs-on: ubuntu-latest
    timeout-minutes: 60
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Test Environment
        run: |
          # Install dependencies
          # Setup Byteman
          # Setup test databases
      
      - name: Run Export Failure Tests
        run: |
          cd yb-voyager
          failpoint-ctl enable
          go test -json -v -count=1 -tags=failpoint \
            -run "TestCDCBatchFailure|TestFirstCDCBatch|TestOffsetCommit|TestQueueWrite|TestSnapshot" \
            ./cmd 2>&1 | tee /tmp/export-tests.log | gotestfmt
          failpoint-ctl disable
      
      - name: Upload Test Logs
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: export-test-logs
          path: |
            /tmp/export-tests.log
            /tmp/yb-voyager-export*/logs/
  
  import-tests:
    name: Import-Data Failure Tests
    runs-on: ubuntu-latest
    timeout-minutes: 60
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Run Import Failure Tests
        run: |
          cd yb-voyager
          failpoint-ctl enable
          go test -json -v -count=1 -tags=failpoint \
            -run "TestImportEvent|TestBatchCommit|TestTransformation|TestDBError" \
            ./cmd 2>&1 | tee /tmp/import-tests.log | gotestfmt
          failpoint-ctl disable
      
      - name: Upload Test Logs
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: import-test-logs
          path: /tmp/import-tests.log
```

---

## Success Criteria

### Overall Success Criteria

**Coverage**:
- [ ] All identified failure points have tests
- [ ] Both export and import phases covered
- [ ] Edge cases documented and tested

**Quality**:
- [ ] All tests pass consistently (no flakiness)
- [ ] Tests run in reasonable time (< 5 min each)
- [ ] Clear, actionable failure messages

**Documentation**:
- [ ] Each test has clear documentation
- [ ] Failure scenarios documented
- [ ] Recovery procedures documented

**CI/CD**:
- [ ] All tests integrated into CI/CD pipeline
- [ ] Tests run on every PR
- [ ] Test results easily accessible

---

### Per-Test Success Criteria

Each test must:

1. **Functional Requirements**:
   - [ ] Injects failure at correct point
   - [ ] Verifies expected failure behavior
   - [ ] Verifies successful recovery
   - [ ] Validates data integrity

2. **Non-Functional Requirements**:
   - [ ] Runs in < 5 minutes
   - [ ] No flakiness (100% pass rate in CI)
   - [ ] Clear logging at each phase
   - [ ] Cleanup on failure (no orphaned containers/files)

3. **Documentation Requirements**:
   - [ ] Test purpose clearly documented
   - [ ] Failure scenario explained
   - [ ] Expected behavior documented
   - [ ] Known limitations noted

---

## Appendix

### A. Byteman Marker Reference

Current markers available in codebase:

**CDC Markers** (`BytemanMarkers.cdc(...)`):
- ✅ `before-batch` - Before processing a CDC batch
- ✅ `before-process-record` - Before processing each record
- ✅ `after-process-record` - After processing each record
- ✅ `after-batch` - After batch complete
- ✅ `get-writer` - When getting queue writer
- ⚠️ `before-write-record` - **TO BE ADDED** (Test 1.4)
- ⚠️ `before-offset-commit` - **TO BE ADDED** (Test 1.3)

**Snapshot Markers** (`BytemanMarkers.snapshot(...)`):
- ❓ Need to investigate what markers exist
- ❓ May need to add: `before-batch`, `after-batch`, `before-complete`

---

### B. Go Failpoint Reference

Current failpoints available:

**Import Failpoints**:
- ✅ `importBatchCommitError` - Fail on batch commit (Test 2.2)
- ⚠️ `importEventProcessError` - **TO BE ADDED** (Test 2.1)
- ⚠️ `transformationError` - **TO BE ADDED** (Test 2.3)
- ⚠️ `connectionError` - **TO BE ADDED** (Test 2.4)
- ⚠️ `batchRowError` - **TO BE ADDED** (Test 2.6)

---

### C. Test Execution Commands

**Run all failure injection tests**:
```bash
cd yb-voyager
failpoint-ctl enable
go test -v -count=1 -tags=failpoint ./cmd
failpoint-ctl disable
```

**Run specific test category**:
```bash
# Export tests only
go test -v -count=1 -tags=failpoint -run "^TestCDC|^TestSnapshot|^TestExport" ./cmd

# Import tests only
go test -v -count=1 -tags=failpoint -run "^TestImport" ./cmd
```

**Run specific test with debug output**:
```bash
failpoint-ctl enable
go test -v -count=1 -tags=failpoint -run TestCDCBatchFailureAndResume ./cmd
failpoint-ctl disable
```

**Run with gotestfmt (formatted output)**:
```bash
failpoint-ctl enable
go test -json -v -count=1 -tags=failpoint ./cmd 2>&1 | gotestfmt
failpoint-ctl disable
```

---

### D. Debugging Failed Tests

**When a test fails**:

1. **Check Byteman is loaded**:
```bash
# In test output, look for:
"Byteman rules written (XXX bytes)"
```

2. **Check injection occurred**:
```bash
# Look for in Debezium logs:
">>> BYTEMAN: <rule_name>"
```

3. **Dump Debezium logs**:
```bash
# Logs are in: /tmp/yb-voyager-export*/logs/debezium.log
tail -100 /tmp/yb-voyager-export*/logs/debezium.log
```

4. **Verify marker execution**:
```bash
# Add to test temporarily:
t.Logf("Byteman rules: %s", bytemanHelper.GetRulesContent())
```

5. **Check CDC phase started**:
```bash
# Look for in Debezium logs:
"streaming changes to a local queue file"
```

---

### E. Related Documentation

- [Byteman Programmer's Guide](https://downloads.jboss.org/byteman/latest/byteman-programmers-guide.html)
- [Go Failpoint Documentation](https://github.com/pingcap/failpoint)
- [YB-Voyager Test Utils](yb-voyager/test/utils/)
- [Existing Failpoint Test](yb-voyager/cmd/import_data_failpoint_test.go)

---

**End of Document**
