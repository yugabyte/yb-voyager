//go:build unit

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
	"os"
	"path/filepath"
	"testing"
)

func TestCreateColumnMapping(t *testing.T) {
	tests := []struct {
		name     string
		headers  []string
		expected ColumnMapping
	}{
		{
			name:    "PG 11-12 headers",
			headers: []string{"queryid", "query", "calls", "rows", "total_time", "mean_time", "min_time", "max_time", "stddev_time"},
			expected: ColumnMapping{
				QueryID:        "queryid",
				Query:          "query",
				Calls:          "calls",
				Rows:           "rows",
				TotalExecTime:  "total_time",
				MeanExecTime:   "mean_time",
				MinExecTime:    "min_time",
				MaxExecTime:    "max_time",
				StddevExecTime: "stddev_time",
			},
		},
		{
			name:    "PG 13+ headers",
			headers: []string{"queryid", "query", "calls", "rows", "total_exec_time", "mean_exec_time", "min_exec_time", "max_exec_time", "stddev_exec_time"},
			expected: ColumnMapping{
				QueryID:        "queryid",
				Query:          "query",
				Calls:          "calls",
				Rows:           "rows",
				TotalExecTime:  "total_exec_time",
				MeanExecTime:   "mean_exec_time",
				MinExecTime:    "min_exec_time",
				MaxExecTime:    "max_exec_time",
				StddevExecTime: "stddev_exec_time",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapping := CreateColumnMapping(tt.headers)
			if mapping.QueryID != tt.expected.QueryID {
				t.Errorf("QueryID mapping = %v, want %v", mapping.QueryID, tt.expected.QueryID)
			}
			if mapping.TotalExecTime != tt.expected.TotalExecTime {
				t.Errorf("TotalExecTime mapping = %v, want %v", mapping.TotalExecTime, tt.expected.TotalExecTime)
			}
			if mapping.MeanExecTime != tt.expected.MeanExecTime {
				t.Errorf("MeanExecTime mapping = %v, want %v", mapping.MeanExecTime, tt.expected.MeanExecTime)
			}
		})
	}
}

func TestParseFromCSVFormats(t *testing.T) {
	tests := []struct {
		name        string
		csvData     string
		expectedLen int
		validate    func(t *testing.T, entries []QueryStats)
	}{
		{
			name: "PostgreSQL 11-12 format (9 columns without exec)",
			csvData: `queryid,query,calls,rows,total_time,mean_time,min_time,max_time,stddev_time
123,"SELECT * FROM users",100,1000,1500.5,15.005,5.2,25.8,3.5
456,"SELECT id FROM products",50,500,750.0,15.0,10.0,20.0,2.1`,
			expectedLen: 2,
			validate: func(t *testing.T, entries []QueryStats) {
				entry1 := entries[0]
				if entry1.QueryID != 123 {
					t.Errorf("Entry1 QueryID = %v, want %v", entry1.QueryID, 123)
				}
				if entry1.Query != "SELECT * FROM users" {
					t.Errorf("Entry1 Query = %v, want %v", entry1.Query, "SELECT * FROM users")
				}
				if entry1.TotalExecTime != 1500.5 {
					t.Errorf("Entry1 TotalExecTime = %v, want %v", entry1.TotalExecTime, 1500.5)
				}
				if entry1.MeanExecTime != 15.005 {
					t.Errorf("Entry1 MeanExecTime = %v, want %v", entry1.MeanExecTime, 15.005)
				}
			},
		},
		{
			name: "PostgreSQL 13+ format (9 columns with exec)",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
789,"SELECT count(*) FROM orders",200,1,50.0,0.25,0.1,0.5,0.05
321,"UPDATE users SET last_login = NOW()",75,75,150.0,2.0,1.0,5.0,0.8`,
			expectedLen: 2,
			validate: func(t *testing.T, entries []QueryStats) {
				entry1 := entries[0]
				if entry1.QueryID != 789 {
					t.Errorf("Entry1 QueryID = %v, want %v", entry1.QueryID, 789)
				}
				if entry1.Query != "SELECT count(*) FROM orders" {
					t.Errorf("Entry1 Query = %v, want %v", entry1.Query, "SELECT count(*) FROM orders")
				}
				if entry1.TotalExecTime != 50.0 {
					t.Errorf("Entry1 TotalExecTime = %v, want %v", entry1.TotalExecTime, 50.0)
				}
				if entry1.MeanExecTime != 0.25 {
					t.Errorf("Entry1 MeanExecTime = %v, want %v", entry1.MeanExecTime, 0.25)
				}
			},
		},
		{
			name: "PostgreSQL 13+ with all columns (real-world scenario)",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time,shared_blks_hit,shared_blks_read,shared_blks_dirtied,shared_blks_written,local_blks_hit,local_blks_read,local_blks_dirtied,local_blks_written,temp_blks_read,temp_blks_written,blk_read_time,blk_write_time,wal_records,wal_fpi,wal_bytes
999,"SELECT pg_stat_reset()",5,5,1.5,0.3,0.1,0.8,0.2,100,10,5,2,0,0,0,0,0,0,0.1,0.05,3,1,512
1001,"CREATE INDEX CONCURRENTLY idx_test ON table_test (column)",1,0,30000.0,30000.0,30000.0,30000.0,0.0,5000,2000,1000,500,100,50,25,10,500,250,1500.0,800.0,1000,200,102400`,
			expectedLen: 2,
			validate: func(t *testing.T, entries []QueryStats) {
				entry1 := entries[0]
				if entry1.QueryID != 999 {
					t.Errorf("Entry1 QueryID = %v, want %v", entry1.QueryID, 999)
				}
				if entry1.Query != "SELECT pg_stat_reset()" {
					t.Errorf("Entry1 Query = %v, want %v", entry1.Query, "SELECT pg_stat_reset()")
				}

				entry2 := entries[1]
				if entry2.QueryID != 1001 {
					t.Errorf("Entry2 QueryID = %v, want %v", entry2.QueryID, 1001)
				}
				// Test long-running query scenario
				if entry2.TotalExecTime != 30000.0 {
					t.Errorf("Entry2 TotalExecTime = %v, want %v", entry2.TotalExecTime, 30000.0)
				}
			},
		},
		{
			name: "Missing one column from 9 (missing rows column)",
			csvData: `queryid,query,calls,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
555,"SELECT name FROM customers",25,125.0,5.0,1.0,10.0,2.5
666,"DELETE FROM logs WHERE date < ?",10,200.0,20.0,5.0,50.0,15.0`,
			expectedLen: 2,
			validate: func(t *testing.T, entries []QueryStats) {
				entry1 := entries[0]
				if entry1.QueryID != 555 {
					t.Errorf("Entry1 QueryID = %v, want %v", entry1.QueryID, 555)
				}
				if entry1.Query != "SELECT name FROM customers" {
					t.Errorf("Entry1 Query = %v, want %v", entry1.Query, "SELECT name FROM customers")
				}
				if entry1.Calls != 25 {
					t.Errorf("Entry1 Calls = %v, want %v", entry1.Calls, 25)
				}
				// Rows should be 0 since missing
				if entry1.Rows != 0 {
					t.Errorf("Entry1 Rows = %v, want %v (default for missing field)", entry1.Rows, 0)
				}
				// Timing fields should still be parsed
				if entry1.TotalExecTime != 125.0 {
					t.Errorf("Entry1 TotalExecTime = %v, want %v", entry1.TotalExecTime, 125.0)
				}
				if entry1.MinExecTime != 1.0 {
					t.Errorf("Entry1 MinExecTime = %v, want %v", entry1.MinExecTime, 1.0)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary CSV file for this test case
			tempDir := t.TempDir()
			csvPath := filepath.Join(tempDir, "test_pgss.csv")

			err := os.WriteFile(csvPath, []byte(tt.csvData), 0644)
			if err != nil {
				t.Fatalf("Failed to create test CSV file: %v", err)
			}

			// Parse the CSV
			entries, err := ParseFromCSV(csvPath)
			if err != nil {
				t.Fatalf("ParseFromCSV failed: %v", err)
			}

			// Validate expected length
			if len(entries) != tt.expectedLen {
				t.Errorf("Expected %d entries, got %d", tt.expectedLen, len(entries))
			}

			// Run custom validation
			if tt.validate != nil && len(entries) > 0 {
				tt.validate(t, entries)
			}
		})
	}
}

func TestParseInvalidData(t *testing.T) {
	// Test that parsing bad data gets rejected with detailed error messages
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test_invalid_pgss.csv")

	// CSV with invalid data that should be gracefully handled
	invalidData := `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
invalid_id,"SELECT * FROM users",100,1000,1500.5,15.005,5.2,25.8,3.5
123,"",50,500,750.0,15.0,10.0,20.0,2.1
456,"SELECT * FROM products",invalid_calls,500,750.0,15.0,10.0,20.0,2.1`

	err := os.WriteFile(csvPath, []byte(invalidData), 0644)
	if err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	entries, err := ParseFromCSV(csvPath)
	if err != nil {
		t.Fatalf("ParseFromCSV should not fail: %v", err)
	}

	// Should have 0 valid entries since all have validation issues that cause them to be skipped:
	// Row 1: invalid queryid (can't parse) - skipped with detailed log
	// Row 2: empty query - skipped with detailed log
	// Row 3: invalid calls (can't parse) - skipped with detailed log
	if len(entries) != 0 {
		t.Errorf("Expected 0 valid entries (all should be skipped due to validation errors), got %d", len(entries))
	}
}
