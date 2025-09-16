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
package pgss

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

// ParseFromCSV parses a CSV file and returns normalized QueryStats entries
func ParseFromCSV(csvPath string) ([]QueryStats, error) {
	file, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PGSS CSV file %s: %w", csvPath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	var entries []QueryStats
	lineNumber := 1 // Header is line 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to read CSV row at line %d: %w", lineNumber, err)
		} else if len(headers) != len(record) {
			return nil, fmt.Errorf("invalid PGSS CSV structure: headers count does not match record count at line %d", lineNumber)
		}

		entry, err := parseCSVRecord(headers, record)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CSV row at line %d: %w", lineNumber, err)
		}

		// Entry is valid if parsing succeeded
		entries = append(entries, *entry)
		lineNumber++
	}

	log.Infof("PGSS CSV parsing completed with %d entries", len(entries))
	return entries, nil
}

// parseCSVRecord converts a single CSV record to QueryStats using the column mapping
func parseCSVRecord(headers []string, record []string) (*QueryStats, error) {
	entry := &QueryStats{}

	// Helper function to safely get value by column name
	getValue := func(columnName string) string {
		if columnName == "" {
			return ""
		}
		for i, header := range headers {
			if header == columnName {
				// Note: headers and columns len is same
				return record[i]
			}
		}
		return ""
	}

	var err error

	// QueryID (required)
	queryIDValue := getValue("queryid")
	if queryIDValue == "" {
		return nil, fmt.Errorf("missing queryid")
	}
	entry.QueryID, err = strconv.ParseInt(queryIDValue, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid queryid: %s", queryIDValue)
	}

	// Query (required)
	entry.Query = strings.TrimSpace(getValue("query"))
	if entry.Query == "" {
		return nil, fmt.Errorf("missing or empty query")
	}

	// Calls (required)
	callsValue := getValue("calls")
	if callsValue == "" {
		return nil, fmt.Errorf("missing calls")
	}
	entry.Calls, err = strconv.ParseInt(callsValue, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid calls: %s", callsValue)
	}
	if entry.Calls <= 0 {
		return nil, fmt.Errorf("calls must be > 0, got %d", entry.Calls)
	}

	// Rows (optional, default to 0)
	rowsValue := getValue("rows")
	if rowsValue != "" {
		if val, err := strconv.ParseInt(rowsValue, 10, 64); err == nil {
			entry.Rows = val
		}
	}

	err = parseFloatOrZero(getValue("total_exec_time"), "total_exec_time", &entry.TotalExecTime)
	if err != nil {
		return nil, err
	}
	err = parseFloatOrZero(getValue("mean_exec_time"), "mean_exec_time", &entry.MeanExecTime)
	if err != nil {
		return nil, err
	}
	err = parseFloatOrZero(getValue("min_exec_time"), "min_exec_time", &entry.MinExecTime)
	if err != nil {
		return nil, err
	}
	err = parseFloatOrZero(getValue("max_exec_time"), "max_exec_time", &entry.MaxExecTime)
	if err != nil {
		return nil, err
	}
	err = parseFloatOrZero(getValue("stddev_exec_time"), "stddev_exec_time", &entry.StddevExecTime)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

// parseFloatOrZero parses a float and sets it to entry struct's field via pointer, returning error if invalid
func parseFloatOrZero(value, fieldName string, target *float64) (err error) {
	if value == "" {
		*target = 0.0 // Empty/NULL values are valid, default to 0.0
		return
	}

	*target, err = strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("invalid %s: %s", fieldName, value)
	}
	return
}
