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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// ParseFromCSV parses a CSV file and returns normalized PgStatStatements entries
func ParseFromCSV(csvPath string) ([]*PgStatStatements, error) {
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

	var entries []*PgStatStatements
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
		entries = append(entries, entry)
		lineNumber++
	}

	log.Infof("PGSS CSV parsing completed with %d entries", len(entries))
	// since there is a possibility of same query text for different entries(due to userid difference), we need to merge them
	return MergePgStatStatementsBasedOnQuery(entries), nil
}

// parseCSVRecord converts a single CSV record to PgStatStatements struct
func parseCSVRecord(headers []string, record []string) (*PgStatStatements, error) {
	var err error
	entry := &PgStatStatements{}

	// Check if this is the new JSONB format (has pgss_data column)
	for i, header := range headers {
		if header == "pgss_data" {
			// New JSONB format
			return parseJSONBRecord(record[i])
		}
	}

	// Old CSV format - keep existing parsing logic for backward compatibility
	for i, header := range headers {
		value := record[i]
		switch header {
		case "source_node":
			// Skip source_node, we don't need it in the struct
			continue
		case "queryid":
			if value == "" {
				return nil, fmt.Errorf("missing queryid")
			}
			entry.QueryID, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid queryid: %s", value)
			}
		case "query":
			entry.Query = strings.TrimSpace(value)
			if entry.Query == "" {
				return nil, fmt.Errorf("missing or empty query")
			}
		case "calls":
			if value == "" {
				return nil, fmt.Errorf("missing calls")
			}
			entry.Calls, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid calls: %s", value)
			}
			if entry.Calls <= 0 {
				return nil, fmt.Errorf("invalid calls: %s", value)
			}
		case "rows":
			if value != "" {
				entry.Rows, err = strconv.ParseInt(value, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid rows: %s", value)
				}
			}
		case "total_exec_time":
			err = parseFloatOrZero(value, "total_exec_time", &entry.TotalExecTime)
			if err != nil {
				return nil, err
			}
		case "mean_exec_time":
			err = parseFloatOrZero(value, "mean_exec_time", &entry.MeanExecTime)
			if err != nil {
				return nil, err
			}
		case "min_exec_time":
			err = parseFloatOrZero(value, "min_exec_time", &entry.MinExecTime)
			if err != nil {
				return nil, err
			}
		case "max_exec_time":
			err = parseFloatOrZero(value, "max_exec_time", &entry.MaxExecTime)
			if err != nil {
				return nil, err
			}
		case "stddev_exec_time":
			err = parseFloatOrZero(value, "stddev_exec_time", &entry.StddevExecTime)
			if err != nil {
				return nil, err
			}
		}
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

// parseJSONBRecord parses a JSONB string and extracts PgStatStatements fields
func parseJSONBRecord(jsonStr string) (*PgStatStatements, error) {
	var jsonData map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSONB: %w", err)
	}

	entry := &PgStatStatements{}

	// Extract required fields
	entry.QueryID, err = utils.GetInt64FromJSON(jsonData, "queryid")
	if err != nil {
		return nil, err
	}

	entry.Query, err = utils.GetStringFromJSON(jsonData, "query")
	if err != nil {
		return nil, err
	}

	entry.Calls, err = utils.GetInt64FromJSON(jsonData, "calls")
	if err != nil {
		return nil, err
	}
	if entry.Calls <= 0 {
		return nil, fmt.Errorf("invalid calls: %d", entry.Calls)
	}

	// Optional field - rows
	entry.Rows, _ = utils.GetInt64FromJSON(jsonData, "rows")

	// Handle timing columns - check both new naming (total_exec_time) and old naming (total_time)
	entry.TotalExecTime = utils.GetFloat64OrZero(jsonData, "total_exec_time", "total_time")
	entry.MeanExecTime = utils.GetFloat64OrZero(jsonData, "mean_exec_time", "mean_time")
	entry.MinExecTime = utils.GetFloat64OrZero(jsonData, "min_exec_time", "min_time")
	entry.MaxExecTime = utils.GetFloat64OrZero(jsonData, "max_exec_time", "max_time")
	entry.StddevExecTime = utils.GetFloat64OrZero(jsonData, "stddev_exec_time", "stddev_time")

	return entry, nil
}
