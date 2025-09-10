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

// ColumnMapping struct helps to map the QueryStats struct's fields to the actual pgss column names from CSV file
type ColumnMapping struct {
	QueryID        string // Always "queryid"
	Query          string // Always "query"
	Calls          string // Always "calls"
	Rows           string // Always "rows"
	TotalExecTime  string // "total_time" (PG11-12) OR "total_exec_time" (PG13+)
	MeanExecTime   string // "mean_time" (PG11-12) OR "mean_exec_time" (PG13+)
	MinExecTime    string // "min_time" (PG11-12) OR "min_exec_time" (PG13+)
	MaxExecTime    string // "max_time" (PG11-12) OR "max_exec_time" (PG13+)
	StddevExecTime string // "stddev_time" (PG11-12) OR "stddev_exec_time" (PG13+)
}

func CreateColumnMapping(headers []string) *ColumnMapping {
	mapping := &ColumnMapping{}

	for _, header := range headers {
		switch header {
		// Core fields (same across all versions)
		case "queryid":
			mapping.QueryID = header
		case "query":
			mapping.Query = header
		case "calls":
			mapping.Calls = header
		case "rows":
			mapping.Rows = header

		// Timing fields (version-dependent names)
		case "total_time", "total_exec_time":
			mapping.TotalExecTime = header
		case "mean_time", "mean_exec_time":
			mapping.MeanExecTime = header
		case "min_time", "min_exec_time":
			mapping.MinExecTime = header
		case "max_time", "max_exec_time":
			mapping.MaxExecTime = header
		case "stddev_time", "stddev_exec_time":
			mapping.StddevExecTime = header
		}
	}

	return mapping
}

// Validate validates the column mapping by checking each field directly
func (m *ColumnMapping) Validate() error {
	var missingFields []string
	if m.QueryID == "" {
		missingFields = append(missingFields, "queryid")
	}
	if m.Query == "" {
		missingFields = append(missingFields, "query")
	}
	if m.Calls == "" {
		missingFields = append(missingFields, "calls")
	}
	if m.Rows == "" {
		missingFields = append(missingFields, "rows")
	}
	if m.TotalExecTime == "" {
		missingFields = append(missingFields, "total_exec_time")
	}
	if m.MeanExecTime == "" {
		missingFields = append(missingFields, "mean_exec_time")
	}
	if m.MinExecTime == "" {
		missingFields = append(missingFields, "min_exec_time")
	}
	if m.MaxExecTime == "" {
		missingFields = append(missingFields, "max_exec_time")
	}
	if m.StddevExecTime == "" {
		missingFields = append(missingFields, "stddev_exec_time")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("missing required fields in CSV headers: %v", missingFields)
	}
	return nil
}

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

	mapping := CreateColumnMapping(headers)
	if err = mapping.Validate(); err != nil {
		return nil, fmt.Errorf("invalid PGSS CSV structure: %w", err)
	}

	var entries []QueryStats
	lineNumber := 1 // Header is line 0
	validEntries := 0
	skippedEntries := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to read CSV row at line %d: %w", lineNumber, err)
		} else if len(headers) != len(record) {
			return nil, fmt.Errorf("invalid PGSS CSV structure: headers count does not match record count at line %d", lineNumber)
		}

		entry, err := parseCSVRecord(headers, record, mapping)
		if err != nil {
			log.Warnf("Skipping invalid PGSS record at line %d: %v", lineNumber, err)
			skippedEntries++
			continue
		}

		// Entry is valid if parsing succeeded
		entries = append(entries, *entry)
		validEntries++
		lineNumber++
	}

	log.Infof("PGSS CSV parsing completed: %d valid entries, %d skipped entries", validEntries, skippedEntries)
	return entries, nil
}

// parseCSVRecord converts a single CSV record to QueryStats using the column mapping
func parseCSVRecord(headers []string, record []string, mapping *ColumnMapping) (*QueryStats, error) {
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
	queryIDValue := getValue(mapping.QueryID)
	if queryIDValue == "" {
		return nil, fmt.Errorf("missing queryid field")
	}
	entry.QueryID, err = strconv.ParseInt(queryIDValue, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid queryid '%s': %w", queryIDValue, err)
	}

	// Query (required)
	queryValue := getValue(mapping.Query)
	if queryValue == "" {
		return nil, fmt.Errorf("missing query field")
	}
	entry.Query = strings.TrimSpace(queryValue)
	if entry.Query == "" {
		return nil, fmt.Errorf("query text cannot be empty")
	}

	// Calls (required)
	callsValue := getValue(mapping.Calls)
	if callsValue == "" {
		return nil, fmt.Errorf("missing calls field")
	}
	entry.Calls, err = strconv.ParseInt(callsValue, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid calls '%s': %w", callsValue, err)
	}
	if entry.Calls <= 0 {
		return nil, fmt.Errorf("calls must be greater than zero, got %d", entry.Calls)
	}

	// Rows (optional, default to 0)
	rowsValue := getValue(mapping.Rows)
	if rowsValue != "" {
		if val, err := strconv.ParseInt(rowsValue, 10, 64); err == nil {
			entry.Rows = val
		}
	}

	entry.TotalExecTime = parseFloatSafe(getValue(mapping.TotalExecTime))
	entry.MeanExecTime = parseFloatSafe(getValue(mapping.MeanExecTime))
	entry.MinExecTime = parseFloatSafe(getValue(mapping.MinExecTime))
	entry.MaxExecTime = parseFloatSafe(getValue(mapping.MaxExecTime))
	entry.StddevExecTime = parseFloatSafe(getValue(mapping.StddevExecTime))

	return entry, nil
}

// parseFloatSafe safely parses a float, returning 0.0 if parsing fails
func parseFloatSafe(value string) float64 {
	if value == "" {
		return 0.0
	}
	if val, err := strconv.ParseFloat(value, 64); err == nil {
		return val
	}
	return 0.0
}
