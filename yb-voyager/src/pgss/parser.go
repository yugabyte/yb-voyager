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
	entry.QueryID, err = getInt64FromJSON(jsonData, "queryid")
	if err != nil {
		return nil, err
	}

	entry.Query, err = getStringFromJSON(jsonData, "query")
	if err != nil {
		return nil, err
	}

	entry.Calls, err = getInt64FromJSON(jsonData, "calls")
	if err != nil {
		return nil, err
	}
	if entry.Calls <= 0 {
		return nil, fmt.Errorf("invalid calls: %d", entry.Calls)
	}

	// Optional field - rows
	entry.Rows, _ = getInt64FromJSON(jsonData, "rows")

	// Handle timing columns - check both new naming (total_exec_time) and old naming (total_time)
	entry.TotalExecTime = getFloat64OrZero(jsonData, "total_exec_time", "total_time")
	entry.MeanExecTime = getFloat64OrZero(jsonData, "mean_exec_time", "mean_time")
	entry.MinExecTime = getFloat64OrZero(jsonData, "min_exec_time", "min_time")
	entry.MaxExecTime = getFloat64OrZero(jsonData, "max_exec_time", "max_time")
	entry.StddevExecTime = getFloat64OrZero(jsonData, "stddev_exec_time", "stddev_time")

	return entry, nil
}

// getInt64FromJSON extracts an int64 value from JSON data
func getInt64FromJSON(data map[string]interface{}, key string) (int64, error) {
	val, ok := data[key]
	if !ok {
		return 0, fmt.Errorf("missing required field: %s", key)
	}

	switch v := val.(type) {
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("invalid type for %s: %T", key, val)
	}
}

// getStringFromJSON extracts a string value from JSON data
func getStringFromJSON(data map[string]interface{}, key string) (string, error) {
	val, ok := data[key]
	if !ok {
		return "", fmt.Errorf("missing required field: %s", key)
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("invalid type for %s: expected string, got %T", key, val)
	}

	str = strings.TrimSpace(str)
	if str == "" {
		return "", fmt.Errorf("empty value for %s", key)
	}

	return str, nil
}

// getFloat64OrZero extracts a float64 value from JSON data, trying multiple keys
// Returns 0.0 if none of the keys exist (for optional fields)
func getFloat64OrZero(data map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		if val, ok := data[key]; ok {
			switch v := val.(type) {
			case float64:
				return v
			case int64:
				return float64(v)
			case int:
				return float64(v)
			case string:
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					return f
				}
			}
		}
	}
	return 0.0
}
