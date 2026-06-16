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
	"os"
	"path/filepath"
	"strings"
)

// CountEventsInQueueSegments counts non-empty NDJSON lines in data/queue/segment.*.ndjson.
func CountEventsInQueueSegments(exportDir string) (int, error) {
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
			count, err := countEventsInNDJSONSegmentFile(filePath)
			if err != nil {
				return 0, err
			}
			totalEvents += count
		}
	}

	return totalEvents, nil
}

func countEventsInNDJSONSegmentFile(filePath string) (int, error) {
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
