//go:build failpoint

package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	goerrors "github.com/go-errors/errors"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func countEventsInQueueSegments(exportDir string) (int, error) {
	queueDir := filepath.Join(exportDir, "data", "queue")
	entries, err := os.ReadDir(queueDir)
	if err != nil {
		return 0, err
	}

	totalEvents := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "segment.") && strings.HasSuffix(entry.Name(), ".ndjson") {
			filePath := filepath.Join(queueDir, entry.Name())
			count, err := countEventsInFile(filePath)
			if err != nil {
				return 0, err
			}
			totalEvents += count
		}
	}

	return totalEvents, nil
}

func countEventsInFile(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == `\.` {
			continue
		}
		count++
	}
	return count, scanner.Err()
}

func verifyNoEventIDDuplicates(t *testing.T, exportDir string) {
	t.Helper()

	_, _ = verifyNoEventIDDuplicatesInternal(t, exportDir, true)
}

func verifyNoEventIDDuplicatesAfterFailure(t *testing.T, exportDir string) (int, int) {
	t.Helper()
	return verifyNoEventIDDuplicatesInternal(t, exportDir, false)
}

func verifyNoEventIDDuplicatesInternal(t *testing.T, exportDir string, failOnErrors bool) (int, int) {
	t.Helper()

	queueDir := filepath.Join(exportDir, "data", "queue")
	entries, err := os.ReadDir(queueDir)
	require.NoError(t, err, "Failed to read queue directory")

	eventIDSet := make(map[string]bool)
	duplicateCount := 0
	parseErrors := 0
	missingEventID := 0
	totalLines := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "segment.") && strings.HasSuffix(entry.Name(), ".ndjson") {
			filePath := filepath.Join(queueDir, entry.Name())
			file, err := os.Open(filePath)
			require.NoError(t, err, "Failed to open queue segment file")
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || line == `\.` {
					continue
				}
				totalLines++

				var eventData map[string]interface{}
				if err := json.Unmarshal([]byte(line), &eventData); err != nil {
					parseErrors++
					if parseErrors <= 3 {
						logTestf(t, "Malformed CDC line in %s: %s", filePath, line)
					}
					continue
				}

				eventIDRaw, ok := eventData["event_id"]
				if !ok || eventIDRaw == nil {
					missingEventID++
					continue
				}
				eventID, ok := eventIDRaw.(string)
				if !ok || eventID == "" || eventID == "null" {
					missingEventID++
					if missingEventID <= 3 {
						logTestf(t, "Missing event_id in %s: %s", filePath, line)
					}
					continue
				}

				if eventIDSet[eventID] {
					duplicateCount++
					logTestf(t, "WARNING: Duplicate event_id found: %s", eventID)
				} else {
					eventIDSet[eventID] = true
				}
			}
		}
	}

	if failOnErrors {
		require.Equal(t, 0, parseErrors, "Malformed CDC events found while parsing queue segments")
		require.Equal(t, 0, missingEventID, "Missing event_id values found while parsing queue segments")
	}
	require.Equal(t, 0, duplicateCount, "No duplicate events should exist (event deduplication should work)")
	logTestf(t, "✓ Verified %d unique event_id values with no duplicates (out of %d lines)", len(eventIDSet), totalLines)
	return parseErrors, missingEventID
}

func waitForCDCEventCount(t *testing.T, exportDir string, expected int, timeout time.Duration, pollInterval time.Duration) int {
	t.Helper()

	var lastCount int
	require.Eventually(t, func() bool {
		count, err := countEventsInQueueSegments(exportDir)
		if err != nil {
			logTestf(t, "Failed to count CDC events yet: %v", err)
			return false
		}
		lastCount = count
		logTestf(t, "Current CDC event count: %d / %d expected", count, expected)
		return count >= expected
	}, timeout, pollInterval, "Timed out waiting for CDC event count to reach %d (last=%d)", expected, lastCount)

	return lastCount
}

func waitForProcessExitOrKill(runner *testutils.VoyagerCommandRunner, exportDir string, timeout time.Duration) (bool, error) {
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Wait()
	}()

	select {
	case err := <-errCh:
		return false, err
	case <-time.After(timeout):
		_ = runner.Kill()
		_ = killDebeziumForExportDir(exportDir)
		return true, nil
	}
}

func waitForMarkerFile(path string, timeout time.Duration, pollInterval time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		matched, err := markerFilePresent(path)
		if err == nil && matched {
			return true, nil
		}
		time.Sleep(pollInterval)
	}
	return markerFilePresent(path)
}

func markerFilePresent(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(string(data), "hit"), nil
}

func waitForStreamingMode(exportDir string, timeout time.Duration, pollInterval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := dbzm.ReadExportStatus(filepath.Join(exportDir, "data", "export_status.json"))
		if err == nil && status != nil && status.Mode == dbzm.MODE_STREAMING {
			return nil
		}
		time.Sleep(pollInterval)
	}
	return goerrors.Errorf("timed out waiting for streaming mode")
}

func killDebeziumForExportDir(exportDir string) error {
	pidStr, err := dbzm.GetPIDOfDebeziumOnExportDir(exportDir, SOURCE_DB_EXPORTER_ROLE)
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	if err != nil {
		return err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

func hashSnapshotDescriptor(exportDir string) (string, error) {
	descriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
	data, err := os.ReadFile(descriptorPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func getSnapshotRowCountForTable(exportDir string, tableName string) (int64, error) {
	descriptor := datafile.OpenDescriptor(exportDir)
	if descriptor == nil || descriptor.DataFileList == nil {
		return 0, goerrors.Errorf("data file descriptor not found")
	}
	var total int64
	for _, entry := range descriptor.DataFileList {
		if strings.Contains(entry.TableName, tableName) {
			total += entry.RowCount
		}
	}
	return total, nil
}

func waitForSnapshotDescriptorHashSnapshotFailure(exportDir string, timeout time.Duration, pollInterval time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		hash, err := hashSnapshotDescriptorSnapshotFailure(exportDir)
		if err == nil && hash != "" {
			return hash, nil
		}
		time.Sleep(pollInterval)
	}
	return hashSnapshotDescriptorSnapshotFailure(exportDir)
}

func hashSnapshotDescriptorSnapshotFailure(exportDir string) (string, error) {
	descriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
	data, err := os.ReadFile(descriptorPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func setupSnapshotFailureTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_snapshot_fail CASCADE;",
		"CREATE SCHEMA test_schema_snapshot_fail;",
		`CREATE TABLE test_schema_snapshot_fail.cdc_snapshot_fail_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_snapshot_fail.cdc_snapshot_fail_test REPLICA IDENTITY FULL;`,
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_snapshot_fail.cdc_snapshot_fail_test (name, value)
		SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 100) i;`,
	)

	logTest(t, "Snapshot failure test schema created with 100 snapshot rows")
}

func setupTransitionFailureTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_transition CASCADE;",
		"CREATE SCHEMA test_schema_transition;",
		`CREATE TABLE test_schema_transition.cdc_transition_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_transition.cdc_transition_test REPLICA IDENTITY FULL;`,
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_transition.cdc_transition_test (name, value)
		SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 30) i;`,
	)

	logTest(t, "Transition test schema created with 30 snapshot rows")
}

func setupCDCTestDataForResume(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"CREATE SCHEMA IF NOT EXISTS test_schema;",
		`CREATE TABLE test_schema.cdc_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema.cdc_test REPLICA IDENTITY FULL;`,
		`INSERT INTO test_schema.cdc_test (name, value)
		SELECT 'initial_' || i, i * 10 FROM generate_series(1, 100) i;`,
	)
}

func setupFirstBatchTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"CREATE SCHEMA IF NOT EXISTS test_schema;",
		`CREATE TABLE test_schema.first_batch_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema.first_batch_test REPLICA IDENTITY FULL;`,
		`INSERT INTO test_schema.first_batch_test (name, value)
		SELECT 'initial_' || i, i * 10 FROM generate_series(1, 50) i;`,
	)
}

func setupMultipleFailureTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_multi_fail CASCADE;",
		"CREATE SCHEMA test_schema_multi_fail;",
		`CREATE TABLE test_schema_multi_fail.cdc_multi_fail_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_multi_fail.cdc_multi_fail_test REPLICA IDENTITY FULL;`,
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_multi_fail.cdc_multi_fail_test (name, value)
		SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 50) i;`,
	)

	logTest(t, "Multiple failure test schema created with 50 snapshot rows")
}

func assertSourceRowCount(t *testing.T, container testcontainers.TestContainer, expected int) {
	t.Helper()
	pgConn, err := container.GetConnection()
	require.NoError(t, err, "Failed to get PostgreSQL connection")
	defer pgConn.Close()

	var sourceRowCount int
	err = pgConn.QueryRow("SELECT COUNT(*) FROM test_schema_multi_fail.cdc_multi_fail_test").Scan(&sourceRowCount)
	require.NoError(t, err, "Failed to query source row count")
	require.Equal(t, expected, sourceRowCount, "Source row count should match expected")
}

func setupOffsetCommitTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_offset_commit CASCADE;",
		"CREATE SCHEMA test_schema_offset_commit;",
		`CREATE TABLE test_schema_offset_commit.cdc_offset_commit_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_offset_commit.cdc_offset_commit_test REPLICA IDENTITY FULL;`,
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_offset_commit.cdc_offset_commit_test (name, value)
		SELECT 'snapshot_' || i, i * 10 FROM generate_series(1, 50) i;`,
	)

	logTest(t, "Offset commit test schema created with 50 snapshot rows")
}

func collectEventIDsForOffsetCommitTest(exportDir string) (map[string]struct{}, error) {
	queueDir := filepath.Join(exportDir, "data", "queue")
	files, err := filepath.Glob(filepath.Join(queueDir, "segment.*.ndjson"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return map[string]struct{}{}, nil
	}

	eventIDs := make(map[string]struct{})
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || line == `\.` {
				continue
			}
			var eventData map[string]interface{}
			if err := json.Unmarshal([]byte(line), &eventData); err != nil {
				continue
			}
			eventIDRaw, ok := eventData["event_id"]
			if !ok || eventIDRaw == nil {
				continue
			}
			eventID, ok := eventIDRaw.(string)
			if !ok || eventID == "" || eventID == "null" {
				continue
			}
			eventIDs[eventID] = struct{}{}
		}
		if err := scanner.Err(); err != nil {
			f.Close()
			return nil, err
		}
		f.Close()
	}
	return eventIDs, nil
}

func readOffsetFileChecksum(exportDir string) string {
	offsetPath := filepath.Join(exportDir, "data", fmt.Sprintf("offsets.%s.dat", SOURCE_DB_EXPORTER_ROLE))
	data, err := os.ReadFile(offsetPath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func readOffsetFileContents(exportDir string) string {
	offsetPath := filepath.Join(exportDir, "data", fmt.Sprintf("offsets.%s.dat", SOURCE_DB_EXPORTER_ROLE))
	data, err := os.ReadFile(offsetPath)
	if err != nil {
		return ""
	}
	return string(data)
}

func countDedupSkipLogs(exportDir string) (int, error) {
	logPattern := filepath.Join(exportDir, "logs", "debezium-*.log")
	matches, err := filepath.Glob(logPattern)
	if err != nil {
		return 0, err
	}
	if len(matches) == 0 {
		return 0, goerrors.Errorf("debezium log not found in %s", logPattern)
	}

	f, err := os.Open(matches[0])
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Skipping record") && strings.Contains(line, "event dedup cache") {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return count, nil
}

func setupBeforeHandleBatchCompleteTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_before_batch_complete CASCADE;",
		"CREATE SCHEMA test_schema_before_batch_complete;",
		`CREATE TABLE test_schema_before_batch_complete.cdc_before_batch_complete_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			payload TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_before_batch_complete.cdc_before_batch_complete_test REPLICA IDENTITY FULL;`,
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_before_batch_complete.cdc_before_batch_complete_test (name, value, payload)
		SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
	)

	logTest(t, "Before-handleBatchComplete test schema created with 50 snapshot rows")
}

func setupQueueWriteFailureTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_queue_write CASCADE;",
		"CREATE SCHEMA test_schema_queue_write;",
		`CREATE TABLE test_schema_queue_write.cdc_queue_write_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			payload TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_queue_write.cdc_queue_write_test REPLICA IDENTITY FULL;`,
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_queue_write.cdc_queue_write_test (name, value, payload)
		SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
	)

	logTest(t, "Queue write failure test schema created with 50 snapshot rows")
}

func setupRotationMidBatchTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_rotation CASCADE;",
		"CREATE SCHEMA test_schema_rotation;",
		`CREATE TABLE test_schema_rotation.cdc_rotation_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			payload TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_rotation.cdc_rotation_test REPLICA IDENTITY FULL;`,
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_rotation.cdc_rotation_test (name, value, payload)
		SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
	)

	logTest(t, "Rotation test schema created with 50 snapshot rows")
}

func setupTruncationTestData(t *testing.T, container testcontainers.TestContainer) {
	container.ExecuteSqls(
		"DROP SCHEMA IF EXISTS test_schema_truncation CASCADE;",
		"CREATE SCHEMA test_schema_truncation;",
		`CREATE TABLE test_schema_truncation.cdc_truncation_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value INTEGER,
			payload TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		);`,
		`ALTER TABLE test_schema_truncation.cdc_truncation_test REPLICA IDENTITY FULL;`,
	)

	container.ExecuteSqls(
		`INSERT INTO test_schema_truncation.cdc_truncation_test (name, value, payload)
		SELECT 'snapshot_' || i, i * 10, repeat('s', 20000) FROM generate_series(1, 50) i;`,
	)

	logTest(t, "Truncation test schema created with 50 snapshot rows")
}

func waitForTruncationLog(exportDir string, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		matched, err := checkTruncationLog(exportDir)
		if err == nil && matched {
			return true, nil
		}
		time.Sleep(2 * time.Second)
	}
	return checkTruncationLog(exportDir)
}

func checkTruncationLog(exportDir string) (bool, error) {
	logPattern := filepath.Join(exportDir, "logs", "debezium-*.log")
	matches, err := filepath.Glob(logPattern)
	if err != nil {
		return false, err
	}
	if len(matches) == 0 {
		return false, nil
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), "Truncating queue segment"), nil
}

func assertEventCountDoesNotExceed(t *testing.T, exportDir string, max int, timeout, pollInterval time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		count, err := countEventsInQueueSegments(exportDir)
		require.NoError(t, err, "Failed to count events while checking upper bound")
		if count > max {
			require.Fail(t, "Event count exceeded expected max", "count=%d max=%d", count, max)
		}
		time.Sleep(pollInterval)
	}
}

func listQueueSegmentFiles(exportDir string) ([]string, error) {
	queueDir := filepath.Join(exportDir, "data", "queue")
	return filepath.Glob(filepath.Join(queueDir, "segment.*.ndjson"))
}

func parseQueueSegmentNum(filePath string) (int64, error) {
	base := filepath.Base(filePath)
	parts := strings.Split(base, ".")
	if len(parts) != 3 {
		return -1, fmt.Errorf("unexpected queue segment filename: %s", base)
	}
	segmentNum, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return -1, fmt.Errorf("parse queue segment number from %s: %w", base, err)
	}
	return segmentNum, nil
}

func isQueueSegmentClosed(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lastNonEmpty string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lastNonEmpty = line
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return lastNonEmpty == `\.`, nil
}

func getQueueSegmentFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func getQueueSegmentCommittedSize(exportDir string, segmentNum int64) (int64, error) {
	metaDB, err := metadb.NewMetaDB(exportDir)
	if err != nil {
		return -1, err
	}
	return metaDB.GetLastValidOffsetInSegmentFile(segmentNum)
}
