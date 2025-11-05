# Test Plan for RandomBatchProducer

## Overview
This document outlines a comprehensive test plan for `RandomBatchProducer`, focusing on:
- Basic functionality and correctness
- Batch completeness and verification that batches match between sequential and random producers
- Edge cases and error handling

**Note**: `NextBatch()`, `IsBatchAvailable()`, and `Done()` are only called from a single goroutine, so we don't test concurrent calls to these methods. Randomness is assumed to work correctly and doesn't need verification. State management checks are integrated into the basic functionality and batch completeness tests.

## Test Categories

### 1. Basic Functionality Tests

#### 1.1 Single Batch Production and Consumption
- **Test Case**: Create producer with a file that produces exactly 1 batch
- **Verify**: 
  - `NextBatch()` returns the batch
  - `Done()` returns false initially, true after consuming the batch
  - `IsBatchAvailable()` returns false initially, true when batch available, false after consumption
  - Batch number matches expected value
  - State consistency: If `Done()` is true, `IsBatchAvailable()` must be false

#### 1.2 Multiple Batches Production and Consumption
- **Test Case**: Create producer with a file that produces N batches (e.g., 5, 10, 20)
- **Verify**:
  - `Done()` returns false initially and while batches are being produced/consumed
  - `Done()` returns true only after all batches consumed
  - `IsBatchAvailable()` returns false initially, true when batches available, false when `Done()` is true
  - All N batches are eventually consumed
  - Each batch is unique (no duplicates)
  - Total batches consumed equals N
  - State consistency: If `Done()` is true, `IsBatchAvailable()` must be false; If `IsBatchAvailable()` is true, `Done()` must be false

#### 1.3 Empty File Handling
- **Test Case**: Create producer with an empty file (no data rows)
- **Verify**:
  - `NextBatch()` returns error immediately or after producer finishes
  - `Done()` returns false initially, true after producer finishes
  - `IsBatchAvailable()` returns false throughout
  - State consistency: `Done()` true implies `IsBatchAvailable()` false
  - No panic or crash

#### 1.4 File with Only Header
- **Test Case**: Create producer with a file containing only header (CSV format)
- **Verify**:
  - Producer handles gracefully
  - `Done()` returns false initially, true after producer finishes
  - `IsBatchAvailable()` returns false throughout
  - No batches are produced
  - State consistency maintained

### 2. Batch Completeness Tests

#### 2.1 Single Batch - Match Verification
- **Test Case**: Create producer with a file that produces exactly 1 batch
- **Verify**:
  - Batch count from random producer matches sequential producer (1 batch)
  - Batch contents match between random and sequential producer
  - Batch number matches sequential producer's batch number
  - Batch metadata (file path, table name, etc.) matches
  - `Done()` state transitions: false initially, false while batch available, true after consumption
  - `IsBatchAvailable()` state transitions: false initially, true when batch available, false after consumption
  - State consistency maintained throughout

#### 2.2 Multiple Batches - Match Verification
- **Test Case**: Create producer with a file that produces N batches (e.g., 5, 10, 20)
- **Verify**:
  - Batch count from random producer matches sequential producer (N batches)
  - All batches from sequential producer are present in random producer (by batch number)
  - Batch contents match between random and sequential producer for each batch
  - All batch numbers from 1 to N appear in consumed batches
  - No gaps in batch numbers
  - No duplicate batches (each batch consumed exactly once)
  - Batch metadata matches for all batches
  - `Done()` state transitions correctly throughout lifecycle
  - `IsBatchAvailable()` state transitions correctly throughout lifecycle
  - State consistency: If `Done()` is true, `IsBatchAvailable()` must be false
  - State consistency: If `IsBatchAvailable()` is true, `Done()` must be false

### 3. Edge Cases and Error Handling

#### 3.1 NextBatch() Called When No Batches Available (Producer Still Running)
- **Test Case**: Call `NextBatch()` before any batches are produced
- **Verify**:
  - Returns error "no batches available"
  - Does not block
  - Producer continues running in background
  - `Done()` returns false
  - `IsBatchAvailable()` returns false

#### 3.2 NextBatch() Called When No Batches Available (Producer Finished)
- **Test Case**: Call `NextBatch()` after all batches consumed
- **Verify**:
  - Returns error "no batches available"
  - `Done()` returns true
  - `IsBatchAvailable()` returns false
  - State consistency maintained
  - No panic

#### 3.3 Sequential Producer Error Propagation
- **Test Case**: Sequential producer encounters error during batch production
- **Verify**:
  - Error is propagated from `startProducingBatches()`
  - Error handling works correctly
  - State is consistent after error
  - `Done()` and `IsBatchAvailable()` reflect correct state

#### 3.4 Close() Called Before Producer Finishes
- **Test Case**: Call `Close()` while batches are still being produced
- **Verify**:
  - Sequential producer is closed
  - No panics or resource leaks
  - Graceful shutdown

#### 3.5 Close() Called Multiple Times
- **Test Case**: Call `Close()` multiple times
- **Verify**:
  - No panics
  - Idempotent behavior
  - Resources cleaned up correctly

#### 3.6 NextBatch() Called After Close()
- **Test Case**: Call `NextBatch()` after `Close()` is called
- **Verify**:
  - Behavior is well-defined (either returns error or handles gracefully)
  - No panics

### 4. Integration with SequentialFileBatchProducer

#### 4.1 Sequential Producer Integration - Normal Flow
- **Test Case**: Verify correct integration with SequentialFileBatchProducer
- **Verify**:
  - Sequential producer is created correctly
  - All batches from sequential producer are consumed
  - Sequential producer's `Done()` is checked correctly
  - Sequential producer is closed properly

#### 4.2 Sequential Producer Integration - Recovery Scenario
- **Test Case**: Test with SequentialFileBatchProducer that has pending batches (recovery)
- **Verify**:
  - Pending batches are included in random selection
  - All batches (pending + new) are consumed
  - Batch count and contents match sequential producer
  - No duplicates

#### 4.3 Sequential Producer Error Handling
- **Test Case**: Sequential producer fails during initialization
- **Verify**:
  - Error is returned from `NewRandomFileBatchProducer()`
  - No producer is created
  - No resource leaks

## Test Implementation Notes

### Test Utilities Needed
- Helper function to create test files with specific batch counts
- Helper function to create SequentialFileBatchProducer with test files (using real implementation)
- Helper function to wait for producer goroutine to finish (with timeout)
- Helper function to verify batch uniqueness
- Helper function to verify all batches are consumed (completeness check)
- Helper function to verify batch count matches between sequential and random producer
- Helper function to verify batch contents match between sequential and random producer
- Helper function to collect all batches from sequential producer for comparison

### Test Data Setup
- Small files (1-5 batches) for basic tests
- Medium files (10-50 batches) for completeness tests
- Empty files
- Files with headers only
- Files with various formats (CSV, etc.)

### Testing Tools
- Use `sync.WaitGroup` for coordinating producer goroutine with test
- Use timeouts to prevent hanging tests
- Use channels or polling to wait for producer goroutine completion

### Metrics to Verify
- Total batches consumed = batches produced by sequential producer
- Batch count from random producer matches sequential producer
- Batch contents match between random and sequential producer
- No duplicate batches
- All batch numbers present (1 to N)
- Correct state transitions for `Done()` and `IsBatchAvailable()`
- State consistency maintained
- No deadlocks or hangs
- Producer goroutine completes successfully

## Success Criteria

1. **Correctness**: All batches from sequential producer are consumed exactly once
2. **Batch Matching**: Batch count and contents match between sequential and random producers
3. **State Management**: `Done()` and `IsBatchAvailable()` reflect correct state throughout lifecycle
4. **State Consistency**: If `Done()` is true, `IsBatchAvailable()` must be false (and vice versa)
5. **Error Handling**: Errors are handled gracefully without crashes
6. **Producer Goroutine**: Background producer goroutine completes successfully

## Priority Levels

### High Priority (Must Have)
- Basic functionality tests (1.1, 1.2)
- Batch completeness - single batch (2.1)
- Batch completeness - multiple batches (2.2)
- Error handling (3.1, 3.2, 3.3)
- Sequential producer integration (4.1)

### Medium Priority (Should Have)
- Empty file handling (1.3)
- File with only header (1.4)
- Close() behavior (3.4, 3.5, 3.6)
- Sequential producer integration - recovery (4.2)
- Sequential producer error handling (4.3)

### Low Priority (Nice to Have)
- (No low priority tests currently)

