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
	"testing"

	"github.com/stretchr/testify/assert"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

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
				assert.Equal(t, int64(123), entry1.QueryID, "Entry1 QueryID should match")
				assert.Equal(t, "SELECT * FROM users", entry1.Query, "Entry1 Query should match")
				assert.Equal(t, 1500.5, entry1.TotalExecTime, "Entry1 TotalExecTime should match")
				assert.Equal(t, 15.005, entry1.MeanExecTime, "Entry1 MeanExecTime should match")
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
				assert.Equal(t, int64(789), entry1.QueryID, "Entry1 QueryID should match")
				assert.Equal(t, "SELECT count(*) FROM orders", entry1.Query, "Entry1 Query should match")
				assert.Equal(t, 50.0, entry1.TotalExecTime, "Entry1 TotalExecTime should match")
				assert.Equal(t, 0.25, entry1.MeanExecTime, "Entry1 MeanExecTime should match")
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
				assert.Equal(t, int64(999), entry1.QueryID, "Entry1 QueryID should match")
				assert.Equal(t, "SELECT pg_stat_reset()", entry1.Query, "Entry1 Query should match")
				assert.Equal(t, 1.5, entry1.TotalExecTime, "Entry1 TotalExecTime should match")

				entry2 := entries[1]
				assert.Equal(t, int64(1001), entry2.QueryID, "Entry2 QueryID should match")
				assert.Equal(t, 30000.0, entry2.TotalExecTime, "Entry2 TotalExecTime should match")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary CSV file for this test case
			csvPath := testutils.CreateTempCSVFile(t, tt.csvData)

			// Parse the CSV
			entries, err := ParseFromCSV(csvPath)
			assert.NoError(t, err, "ParseFromCSV should not fail")
			assert.Len(t, entries, tt.expectedLen, "Should have expected number of entries")

			// Run custom validation
			if tt.validate != nil && len(entries) > 0 {
				tt.validate(t, entries)
			}
		})
	}
}

// TestParseInvalidDataInvalid tests all invalid data scenarios
func TestParseInvalidDataInvalid(t *testing.T) {
	tests := []struct {
		name        string
		csvData     string // For CSV content errors
		expectError bool   // true = function returns error, false = succeeds but skips rows
		expectedErr string // expected error message (if expectError = true)
		expectedLen int    // expected valid entries (if expectError = false)
	}{

		// 1. CSV Structure Errors (Function should return error immediately)
		{
			name: "Headers and records column count mismatch",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
123,"SELECT * FROM users",100,1000,1500.5,15.005`,
			expectError: true,
			expectedErr: "wrong number of fields",
		},

		// 2. Column Mapping Validation Errors (Function should return error immediately)
		{
			name: "Missing required queryid column",
			csvData: `query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
"SELECT * FROM users",100,1000,1500.5,15.005,5.2,25.8,3.5`,
			expectError: true,
			expectedErr: "missing required fields in CSV headers: [queryid]",
		},
		{
			name: "Missing required query column",
			csvData: `queryid,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
123,100,1000,1500.5,15.005,5.2,25.8,3.5`,
			expectError: true,
			expectedErr: "missing required fields in CSV headers: [query]",
		},
		{
			name: "Missing multiple required columns",
			csvData: `queryid,query
123,"SELECT * FROM users"`,
			expectError: true,
			expectedErr: "missing required fields in CSV headers:",
		},

		// 3. Data Validation Errors (Rows get skipped, but function succeeds)
		{
			name: "Invalid queryid (non-numeric)",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
invalid_id,"SELECT * FROM users",100,1000,1500.5,15.005,5.2,25.8,3.5
123,"SELECT * FROM users",100,1000,1500.5,15.005,5.2,25.8,3.5`,
			expectError: false,
			expectedLen: 1, // 1st row gets skipped
		},
		{
			name: "Empty query text",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
123,"",50,500,750.0,15.0,10.0,20.0,2.1
456,"SELECT * FROM products",100,1000,1500.5,15.005,5.2,25.8,3.5`,
			expectError: false,
			expectedLen: 1, // 1st row gets skipped
		},
		{
			name: "Multiple invalid rows",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
invalid_id,"SELECT * FROM users",100,1000,1500.5,15.005,5.2,25.8,3.5
123,"",50,500,750.0,15.0,10.0,20.0,2.1
456,"SELECT * FROM products",invalid_calls,500,750.0,15.0,10.0,20.0,2.1
789,"SELECT * FROM orders",100,1000,1500.5,15.005,5.2,25.8,3.5`,
			expectError: false,
			expectedLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary CSV file for this test case
			csvPath := testutils.CreateTempCSVFile(t, tt.csvData)

			entries, err := ParseFromCSV(csvPath)
			if tt.expectError {
				assert.Error(t, err, "Expected ParseFromCSV to return an error")
				assert.Contains(t, err.Error(), tt.expectedErr, "Error message should contain expected text")
			} else {
				assert.NoError(t, err, "ParseFromCSV should not fail")
				assert.Len(t, entries, tt.expectedLen, "Should have expected number of valid entries")
			}
		})
	}
}
