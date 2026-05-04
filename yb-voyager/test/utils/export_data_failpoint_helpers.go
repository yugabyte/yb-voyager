//go:build failpoint_export || failpoint_import || failpoint_cutover

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
package testutils

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	goerrors "github.com/go-errors/errors"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

func VerifyNoEventIDDuplicates(t *testing.T, exportDir string) {
	t.Helper()

	_, _ = verifyNoEventIDDuplicatesInternal(t, exportDir, true)
}

func VerifyNoEventIDDuplicatesAfterFailure(t *testing.T, exportDir string) (int, int) {
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
						t.Logf("Malformed CDC line in %s: %s", filePath, line)
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
						t.Logf("Missing event_id in %s: %s", filePath, line)
					}
					continue
				}

				if eventIDSet[eventID] {
					duplicateCount++
					t.Logf("WARNING: Duplicate event_id found: %s", eventID)
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
	t.Logf("Verified %d unique event_id values with no duplicates (out of %d lines)", len(eventIDSet), totalLines)
	return parseErrors, missingEventID
}

func HashSnapshotDescriptor(exportDir string) (string, error) {
	descriptorPath := filepath.Join(exportDir, datafile.DESCRIPTOR_PATH)
	data, err := os.ReadFile(descriptorPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func GetSnapshotRowCountForTable(exportDir string, tableName string) (int64, error) {
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

func CollectEventIDsForOffsetCommitTest(exportDir string) (map[string]struct{}, error) {
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

func ReadOffsetFileChecksum(exportDir string, exporterRole string) string {
	offsetPath := filepath.Join(exportDir, "data", fmt.Sprintf("offsets.%s.dat", exporterRole))
	data, err := os.ReadFile(offsetPath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func ReadOffsetFileContents(exportDir string, exporterRole string) string {
	offsetPath := filepath.Join(exportDir, "data", fmt.Sprintf("offsets.%s.dat", exporterRole))
	data, err := os.ReadFile(offsetPath)
	if err != nil {
		return ""
	}
	return string(data)
}

func CountDedupSkipLogs(exportDir string) (int, error) {
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

func WaitForTruncationLog(exportDir string, timeout time.Duration) (bool, error) {
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

// ParseTruncationTargetSize extracts the "to size N" value from the Debezium truncation log.
// The log format is: "Truncating queue segment <num> at path <path> to size <size>"
func ParseTruncationTargetSize(exportDir string) (int64, error) {
	logPattern := filepath.Join(exportDir, "logs", "debezium-*.log")
	matches, err := filepath.Glob(logPattern)
	if err != nil {
		return -1, err
	}
	if len(matches) == 0 {
		return -1, goerrors.Errorf("no debezium log files found")
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return -1, err
	}
	re := regexp.MustCompile(`Truncating queue segment \d+ at path .+ to size (\d+)`)
	m := re.FindStringSubmatch(string(data))
	if m == nil {
		return -1, goerrors.Errorf("truncation log line not found")
	}
	return strconv.ParseInt(m[1], 10, 64)
}

func AssertEventCountDoesNotExceed(t *testing.T, exportDir string, max int, timeout, pollInterval time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		count, err := CountEventsInQueueSegments(exportDir)
		require.NoError(t, err, "Failed to count events while checking upper bound")
		if count > max {
			require.Fail(t, "Event count exceeded expected max", "count=%d max=%d", count, max)
		}
		time.Sleep(pollInterval)
	}
}

func ListQueueSegmentFiles(exportDir string) ([]string, error) {
	queueDir := filepath.Join(exportDir, "data", "queue")
	return filepath.Glob(filepath.Join(queueDir, "segment.*.ndjson"))
}

func ParseQueueSegmentNum(filePath string) (int64, error) {
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

func FindSegmentNumRange(segmentFiles []string) (lowestPath string, lowestNum int64, highestPath string, highestNum int64, err error) {
	lowestNum = -1
	highestNum = -1
	for _, path := range segmentFiles {
		num, parseErr := ParseQueueSegmentNum(path)
		if parseErr != nil {
			return "", -1, "", -1, parseErr
		}
		if lowestNum == -1 || num < lowestNum {
			lowestNum = num
			lowestPath = path
		}
		if num > highestNum {
			highestNum = num
			highestPath = path
		}
	}
	return lowestPath, lowestNum, highestPath, highestNum, nil
}

func IsQueueSegmentClosed(filePath string) (bool, error) {
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

func GetQueueSegmentFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func GetQueueSegmentCommittedSize(exportDir string, segmentNum int64) (int64, error) {
	metaDB, err := metadb.NewMetaDB(exportDir)
	if err != nil {
		return -1, err
	}
	return metaDB.GetLastValidOffsetInSegmentFile(segmentNum)
}

// ReportTableName converts a dot-separated table name (e.g. "schema.table") to the
// quoted format used in the data-migration-report JSON (e.g. `"schema"."table"`).
func ReportTableName(dotNotation string) string {
	parts := strings.SplitN(dotNotation, ".", 2)
	if len(parts) == 2 {
		return fmt.Sprintf(`"%s"."%s"`, parts[0], parts[1])
	}
	return fmt.Sprintf(`"%s"`, dotNotation)
}
