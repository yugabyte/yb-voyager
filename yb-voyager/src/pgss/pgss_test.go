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
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestParseFromCSVFormats(t *testing.T) {
	tests := []struct {
		name        string
		csvData     string
		expectedLen int
		validate    func(t *testing.T, entries []*PgStatStatements) //write validations based on entries sorted by QueryID
	}{
		{
			name: "PostgreSQL 11-12 format (9 columns without exec)",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
123,"SELECT * FROM users",100,1000,1500.5,15.005,5.2,25.8,3.5
456,"SELECT id FROM products",50,500,750.0,15.0,10.0,20.0,2.1`,
			expectedLen: 2,
			validate: func(t *testing.T, entries []*PgStatStatements) {
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
321,"UPDATE users SET last_login = NOW()",75,75,150.0,2.0,1.0,5.0,0.8
789,"SELECT count(*) FROM orders",200,1,50.0,0.25,0.1,0.5,0.05`,
			expectedLen: 2,
			validate: func(t *testing.T, entries []*PgStatStatements) {
				entry1 := entries[1]
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
			validate: func(t *testing.T, entries []*PgStatStatements) {
				entry1 := entries[0]
				assert.Equal(t, int64(999), entry1.QueryID, "Entry1 QueryID should match")
				assert.Equal(t, "SELECT pg_stat_reset()", entry1.Query, "Entry1 Query should match")
				assert.Equal(t, 1.5, entry1.TotalExecTime, "Entry1 TotalExecTime should match")

				entry2 := entries[1]
				assert.Equal(t, int64(1001), entry2.QueryID, "Entry2 QueryID should match")
				assert.Equal(t, 30000.0, entry2.TotalExecTime, "Entry2 TotalExecTime should match")
			},
		},
		{
			name:        "Only header",
			csvData:     `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time`,
			expectedLen: 0,
			validate: func(t *testing.T, entries []*PgStatStatements) {
				assert.Len(t, entries, 0, "Should have expected number of entries")
			},
		},
		{
			name: "Same queries resulting in merging of entries",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
123,"SELECT * FROM users",100,1000,1500.5,15.005,5.2,25.8,3.5
123,"SELECT * FROM users",50,500,750.0,15.0,10.0,20.0,2.1`,
			expectedLen: 1,
			validate: func(t *testing.T, entries []*PgStatStatements) {
				assert.Len(t, entries, 1, "Should have expected number of entries")
				assert.Equal(t, int64(150), entries[0].Calls, "Calls should match")
				assert.Equal(t, int64(1500), entries[0].Rows, "Rows should match")
				assert.Equal(t, 2250.5, entries[0].TotalExecTime, "TotalExecTime should match")
				assert.Equal(t, 15.003333333333334, entries[0].MeanExecTime, "MeanExecTime should match")
				assert.Equal(t, 5.2, entries[0].MinExecTime, "MinExecTime should match")
				assert.Equal(t, 25.8, entries[0].MaxExecTime, "MaxExecTime should match")
				assert.Equal(t, 3.5, entries[0].StddevExecTime, "StddevExecTime should match")
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

			sort.Slice(entries, func(i, j int) bool {
				return entries[i].QueryID < entries[j].QueryID
			})

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

		// 2. Missing Required Column Errors
		{
			name: "Missing required queryid column",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
,"SELECT * FROM users",100,1000,1500.5,15.005,5.2,25.8,3.5`,
			expectError: true,
			expectedErr: "missing queryid",
		},
		{
			name: "Missing required query column",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
123,,100,1000,1500.5,15.005,5.2,25.8,3.5`,
			expectError: true,
			expectedErr: "missing or empty query",
		},
		{
			name: "Missing multiple required columns",
			csvData: `queryid,query,calls
123,"SELECT * FROM users",0`,
			expectError: true,
			expectedErr: "invalid calls: 0",
		},

		// 3. Data Validation Errors (Rows get skipped, but function succeeds)
		{
			name: "Invalid queryid (non-numeric)",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
invalid_id,"SELECT * FROM users",100,1000,1500.5,15.005,5.2,25.8,3.5
123,"SELECT * FROM users",100,1000,1500.5,15.005,5.2,25.8,3.5`,
			expectError: true,
			expectedErr: "invalid queryid: invalid_id",
		},
		{
			name: "Empty query text",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
123,"",50,500,750.0,15.0,10.0,20.0,2.1
456,"SELECT * FROM products",100,1000,1500.5,15.005,5.2,25.8,3.5`,
			expectError: true,
			expectedErr: "missing or empty query",
		},
		{
			name: "Invalid calls (non-numeric)",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
123,"SELECT * FROM users",invalid_calls,1000,1500.5,15.005,5.2,25.8,3.5`,
			expectError: true,
			expectedErr: "invalid calls: invalid_calls",
		},
		{
			name: "Invalid float in timing field",
			csvData: `queryid,query,calls,rows,total_exec_time,mean_exec_time,min_exec_time,max_exec_time,stddev_exec_time
123,"SELECT * FROM users",100,1000,invalid_float,15.005,5.2,25.8,3.5`,
			expectError: true,
			expectedErr: "invalid total_exec_time: invalid_float",
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

func TestMergePgStatStatements(t *testing.T) {
	tests := []struct {
		name     string
		input    []*PgStatStatements
		expected []*PgStatStatements
	}{
		{
			name: "Simple merge of two entries with same query",
			input: []*PgStatStatements{
				{
					QueryID:        123,
					Query:          "SELECT * FROM users",
					Calls:          100,
					Rows:           1000,
					TotalExecTime:  1500.5,
					MeanExecTime:   15.005,
					MinExecTime:    5.2,
					MaxExecTime:    25.8,
					StddevExecTime: 3.5,
				},
				{
					QueryID:        123,
					Query:          "SELECT * FROM users",
					Calls:          50,
					Rows:           500,
					TotalExecTime:  750.0,
					MeanExecTime:   15.0,
					MinExecTime:    3.0,
					MaxExecTime:    30.0,
					StddevExecTime: 2.1,
				},
			},
			expected: []*PgStatStatements{
				{
					QueryID:        123,
					Query:          "SELECT * FROM users",
					Calls:          150,                // 100 + 50
					Rows:           1500,               // 1000 + 500
					TotalExecTime:  2250.5,             // 1500.5 + 750.0
					MeanExecTime:   15.003333333333334, // 2250.5 / 150
					MinExecTime:    3.0,                // min(5.2, 3.0)
					MaxExecTime:    30.0,               // max(25.8, 30.0)
					StddevExecTime: 3.5,                // Currently disabled, keeps first value
				},
			},
		},
		{
			name: "Different queries remain separate",
			input: []*PgStatStatements{
				{
					QueryID:        123,
					Query:          "SELECT * FROM users",
					Calls:          100,
					Rows:           1000,
					TotalExecTime:  1500.5,
					MeanExecTime:   15.005,
					MinExecTime:    5.2,
					MaxExecTime:    25.8,
					StddevExecTime: 3.5,
				},
				{
					QueryID:        456,
					Query:          "SELECT * FROM products", // Different query
					Calls:          50,
					Rows:           500,
					TotalExecTime:  750.0,
					MeanExecTime:   15.0,
					MinExecTime:    10.0,
					MaxExecTime:    20.0,
					StddevExecTime: 2.1,
				},
			},
			expected: []*PgStatStatements{
				{
					QueryID:        123,
					Query:          "SELECT * FROM users",
					Calls:          100,
					Rows:           1000,
					TotalExecTime:  1500.5,
					MeanExecTime:   15.005,
					MinExecTime:    5.2,
					MaxExecTime:    25.8,
					StddevExecTime: 3.5,
				},
				{
					QueryID:        456,
					Query:          "SELECT * FROM products",
					Calls:          50,
					Rows:           500,
					TotalExecTime:  750.0,
					MeanExecTime:   15.0,
					MinExecTime:    10.0,
					MaxExecTime:    20.0,
					StddevExecTime: 2.1,
				},
			},
		},
		{
			name: "Mixed scenario - some merge, some don't",
			input: []*PgStatStatements{
				{
					QueryID:        100,
					Query:          "SELECT count(*) FROM orders",
					Calls:          200,
					Rows:           1,
					TotalExecTime:  50.0,
					MeanExecTime:   0.25,
					MinExecTime:    0.1,
					MaxExecTime:    0.5,
					StddevExecTime: 0.05,
				},
				{
					QueryID:        200,
					Query:          "SELECT id FROM products",
					Calls:          150,
					Rows:           300,
					TotalExecTime:  75.0,
					MeanExecTime:   0.5,
					MinExecTime:    0.2,
					MaxExecTime:    1.0,
					StddevExecTime: 0.1,
				},
				{
					QueryID:        300, // Same query as first entry
					Query:          "SELECT count(*) FROM orders",
					Calls:          100,
					Rows:           1,
					TotalExecTime:  30.0,
					MeanExecTime:   0.3,
					MinExecTime:    0.05, // Lower minimum
					MaxExecTime:    0.8,  // Higher maximum
					StddevExecTime: 0.08,
				},
			},
			expected: []*PgStatStatements{
				{
					QueryID:        100, // Should keep first entry's QueryID
					Query:          "SELECT count(*) FROM orders",
					Calls:          300,                 // 200 + 100
					Rows:           2,                   // 1 + 1
					TotalExecTime:  80.0,                // 50.0 + 30.0
					MeanExecTime:   0.26666666666666666, // 80.0 / 300
					MinExecTime:    0.05,                // min(0.1, 0.05)
					MaxExecTime:    0.8,                 // max(0.5, 0.8)
					StddevExecTime: 0.05,                // Currently disabled, keeps first value
				},
				{
					QueryID:        200,
					Query:          "SELECT id FROM products",
					Calls:          150,
					Rows:           300,
					TotalExecTime:  75.0,
					MeanExecTime:   0.5,
					MinExecTime:    0.2,
					MaxExecTime:    1.0,
					StddevExecTime: 0.1,
				},
			},
		},
		{
			name:     "Empty input",
			input:    []*PgStatStatements{},
			expected: []*PgStatStatements{},
		},
		{
			name: "Single entry",
			input: []*PgStatStatements{
				{
					QueryID:        999,
					Query:          "SELECT pg_stat_reset()",
					Calls:          5,
					Rows:           5,
					TotalExecTime:  1.5,
					MeanExecTime:   0.3,
					MinExecTime:    0.1,
					MaxExecTime:    0.8,
					StddevExecTime: 0.2,
				},
			},
			expected: []*PgStatStatements{
				{
					QueryID:        999,
					Query:          "SELECT pg_stat_reset()",
					Calls:          5,
					Rows:           5,
					TotalExecTime:  1.5,
					MeanExecTime:   0.3,
					MinExecTime:    0.1,
					MaxExecTime:    0.8,
					StddevExecTime: 0.2,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergePgStatStatementsBasedOnQuery(tt.input)

			// Since the function returns entries in map iteration order which is not deterministic,
			// we need to sort both result and expected for comparison
			// Sort by Query for consistent comparison
			sort.Slice(result, func(i, j int) bool {
				return result[i].Query < result[j].Query
			})
			sort.Slice(tt.expected, func(i, j int) bool {
				return tt.expected[i].Query < tt.expected[j].Query
			})

			assert.Len(t, result, len(tt.expected), "Should have expected number of entries")
			for i, expected := range tt.expected {
				actual := result[i]
				assert.Equal(t, expected.QueryID, actual.QueryID, "QueryID should match for entry %d", i)
				assert.Equal(t, expected.Query, actual.Query, "Query should match for entry %d", i)
				assert.Equal(t, expected.Calls, actual.Calls, "Calls should match for entry %d", i)
				assert.Equal(t, expected.Rows, actual.Rows, "Rows should match for entry %d", i)
				assert.Equal(t, expected.TotalExecTime, actual.TotalExecTime, "TotalExecTime should match for entry %d", i)
				assert.Equal(t, expected.MeanExecTime, actual.MeanExecTime, "MeanExecTime should match for entry %d", i)
				assert.Equal(t, expected.MinExecTime, actual.MinExecTime, "MinExecTime should match for entry %d", i)
				assert.Equal(t, expected.MaxExecTime, actual.MaxExecTime, "MaxExecTime should match for entry %d", i)

				// TODO: merge logic is disable for stddev_exec_time right now
				// assert.Equal(t, expected.StddevExecTime, actual.StddevExecTime, "StddevExecTime should match for entry %d", i)
			}
		})
	}
}
