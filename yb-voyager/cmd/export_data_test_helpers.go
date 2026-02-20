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

	"database/sql"

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
						testutils.LogTestf(t, "Malformed CDC line in %s: %s", filePath, line)
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
						testutils.LogTestf(t, "Missing event_id in %s: %s", filePath, line)
					}
					continue
				}

				if eventIDSet[eventID] {
					duplicateCount++
					testutils.LogTestf(t, "WARNING: Duplicate event_id found: %s", eventID)
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
	testutils.LogTestf(t, "✓ Verified %d unique event_id values with no duplicates (out of %d lines)", len(eventIDSet), totalLines)
	return parseErrors, missingEventID
}

func waitForCDCEventCount(t *testing.T, exportDir string, expected int, timeout time.Duration, pollInterval time.Duration) int {
	t.Helper()

	var lastCount int
	require.Eventually(t, func() bool {
		count, err := countEventsInQueueSegments(exportDir)
		if err != nil {
			testutils.LogTestf(t, "Failed to count CDC events yet: %v", err)
			return false
		}
		lastCount = count
		testutils.LogTestf(t, "Current CDC event count: %d / %d expected", count, expected)
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
		return -1, goerrors.Errorf("unexpected queue segment filename: %s", base)
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


func waitForFallForwardEnabled(t *testing.T, exportDir string, timeout time.Duration, pollInterval time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msr := getMigrationStatusFromMetaDB(t, exportDir)
		if msr.FallForwardEnabled {
			return true
		}
		time.Sleep(pollInterval)
	}
	return false
}

func waitForTargetDBConfInMetaDB(t *testing.T, exportDir string, timeout time.Duration, pollInterval time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msr := getMigrationStatusFromMetaDB(t, exportDir)
		if msr.TargetDBConf != nil {
			return true
		}
		time.Sleep(pollInterval)
	}
	return false
}

func initiateCutoverToTarget(t *testing.T, exportDir string) {
	t.Helper()
	cutoverRunner := testutils.NewVoyagerCommandRunner(nil, "initiate cutover to target", []string{
		"--export-dir", exportDir,
		"--prepare-for-fall-back", "false",
		"--yes",
	}, nil, false)
	err := cutoverRunner.Run()
	require.NoError(t, err, "Failed to initiate cutover to target")
}

func getMigrationStatusFromMetaDB(t *testing.T, exportDir string) *metadb.MigrationStatusRecord {
	t.Helper()

	mdb, err := metadb.NewMetaDB(exportDir)
	require.NoError(t, err, "Failed to open metaDB")

	msr, err := mdb.GetMigrationStatusRecord()
	require.NoError(t, err, "Failed to get migration status record")
	require.NotNil(t, msr, "Migration status record should not be nil")
	return msr
}

func waitForExportFromTargetStarted(t *testing.T, exportDir string, timeout time.Duration, pollInterval time.Duration) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msr := getMigrationStatusFromMetaDB(t, exportDir)
		if msr.ExportFromTargetFallForwardStarted || msr.ExportFromTargetFallBackStarted {
			return true
		}
		time.Sleep(pollInterval)
	}
	return false
}

// setMetaDBForCutoverCheck sets the package-level metaDB variable so that
// getCutoverStatus() (which reads from the global) works in the test context.
// It returns a cleanup function that restores the previous value.
func setMetaDBForCutoverCheck(t *testing.T, exportDir string) func() {
	t.Helper()

	mdb, err := metadb.NewMetaDB(exportDir)
	require.NoError(t, err, "Failed to open metaDB for cutover check")

	prev := metaDB
	metaDB = mdb
	return func() { metaDB = prev }
}

func verifyCutoverIsNotComplete(t *testing.T, exportDir string) {
	t.Helper()

	restore := setMetaDBForCutoverCheck(t, exportDir)
	defer restore()

	status := getCutoverStatus()
	require.NotEqual(t, COMPLETED, status,
		"Cutover should NOT be COMPLETED when export-data-from-target has not started")
	testutils.LogTestf(t, "Verified: cutover status is %q (not COMPLETED)", status)
}

func verifyCutoverIsComplete(t *testing.T, exportDir string) {
	t.Helper()

	restore := setMetaDBForCutoverCheck(t, exportDir)
	defer restore()

	status := getCutoverStatus()
	require.Equal(t, COMPLETED, status,
		"Cutover should be COMPLETED after export-data-from-target starts successfully")
	testutils.LogTestf(t, "Verified: cutover status is %q", status)
}

func waitForRowCount(t *testing.T, container testcontainers.TestContainer, table string, expected int64, timeout time.Duration, pollInterval time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := container.GetConnection()
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}
		var count int64
		err = conn.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		conn.Close()
		if err == nil && count >= expected {
			testutils.LogTestf(t, "Row count for %s: %d (expected >= %d)", table, count, expected)
			return true
		}
		time.Sleep(pollInterval)
	}
	return false
}

func queryRowCount(t *testing.T, container testcontainers.TestContainer, table string) int64 {
	t.Helper()
	conn, err := container.GetConnection()
	require.NoError(t, err, "Failed to get DB connection")
	defer conn.Close()
	var count int64
	err = conn.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
	require.NoError(t, err, "Failed to query row count")
	return count
}

// waitForRowCountOnDB polls an existing *sql.DB until the expected count is reached.
// Useful when the container's GetConnection() returns a new connection each time.
func waitForRowCountOnDB(t *testing.T, db *sql.DB, table string, expected int64, timeout time.Duration, pollInterval time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var count int64
		err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err == nil && count >= expected {
			testutils.LogTestf(t, "Row count for %s: %d (expected >= %d)", table, count, expected)
			return true
		}
		time.Sleep(pollInterval)
	}
	return false
}
