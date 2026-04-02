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
package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeQueryStatsBasedOnQuery(t *testing.T) {
	tests := []struct {
		name     string
		input    []*QueryStats
		expected []*QueryStats
	}{
		{
			name: "Simple merge of two entries with same query",
			input: []*QueryStats{
				{
					QueryID:         123,
					QueryText:       "SELECT * FROM users",
					ExecutionCount:  100,
					RowsProcessed:   1000,
					TotalExecTime:   1500.5,
					AverageExecTime: 15.005,
					MinExecTime:     5.2,
					MaxExecTime:     25.8,
				},
				{
					QueryID:         123,
					QueryText:       "SELECT * FROM users",
					ExecutionCount:  50,
					RowsProcessed:   500,
					TotalExecTime:   750.0,
					AverageExecTime: 15.0,
					MinExecTime:     3.0,
					MaxExecTime:     30.0,
				},
			},
			expected: []*QueryStats{
				{
					QueryID:         123,
					QueryText:       "SELECT * FROM users",
					ExecutionCount:  150,                // 100 + 50
					RowsProcessed:   1500,               // 1000 + 500
					TotalExecTime:   2250.5,             // 1500.5 + 750.0
					AverageExecTime: 15.003333333333334, // 2250.5 / 150
					MinExecTime:     3.0,                // min(5.2, 3.0)
					MaxExecTime:     30.0,               // max(25.8, 30.0)
				},
			},
		},
		{
			name: "Different queries remain separate",
			input: []*QueryStats{
				{
					QueryID:         123,
					QueryText:       "SELECT * FROM users",
					ExecutionCount:  100,
					RowsProcessed:   1000,
					TotalExecTime:   1500.5,
					AverageExecTime: 15.005,
					MinExecTime:     5.2,
					MaxExecTime:     25.8,
				},
				{
					QueryID:         456,
					QueryText:       "SELECT * FROM products",
					ExecutionCount:  50,
					RowsProcessed:   500,
					TotalExecTime:   750.0,
					AverageExecTime: 15.0,
					MinExecTime:     10.0,
					MaxExecTime:     20.0,
				},
			},
			expected: []*QueryStats{
				{
					QueryID:         123,
					QueryText:       "SELECT * FROM users",
					ExecutionCount:  100,
					RowsProcessed:   1000,
					TotalExecTime:   1500.5,
					AverageExecTime: 15.005,
					MinExecTime:     5.2,
					MaxExecTime:     25.8,
				},
				{
					QueryID:         456,
					QueryText:       "SELECT * FROM products",
					ExecutionCount:  50,
					RowsProcessed:   500,
					TotalExecTime:   750.0,
					AverageExecTime: 15.0,
					MinExecTime:     10.0,
					MaxExecTime:     20.0,
				},
			},
		},
		{
			name: "Multi-node merge (primary + 2 replicas)",
			input: []*QueryStats{
				{
					QueryID:         123,
					QueryText:       "SELECT * FROM users WHERE id = $1",
					ExecutionCount:  1000,
					RowsProcessed:   1000,
					TotalExecTime:   15000.5,
					AverageExecTime: 15.0,
					MinExecTime:     2.1,
					MaxExecTime:     50.3,
				},
				{
					QueryID:         123,
					QueryText:       "SELECT * FROM users WHERE id = $1",
					ExecutionCount:  500,
					RowsProcessed:   500,
					TotalExecTime:   7500.0,
					AverageExecTime: 15.0,
					MinExecTime:     2.0,
					MaxExecTime:     48.5,
				},
				{
					QueryID:         123,
					QueryText:       "SELECT * FROM users WHERE id = $1",
					ExecutionCount:  300,
					RowsProcessed:   300,
					TotalExecTime:   4500.0,
					AverageExecTime: 15.0,
					MinExecTime:     2.2,
					MaxExecTime:     52.1,
				},
			},
			expected: []*QueryStats{
				{
					QueryID:         123,
					QueryText:       "SELECT * FROM users WHERE id = $1",
					ExecutionCount:  1800,               // 1000 + 500 + 300
					RowsProcessed:   1800,               // 1000 + 500 + 300
					TotalExecTime:   27000.5,            // 15000.5 + 7500 + 4500
					AverageExecTime: 15.000277777777778, // 27000.5 / 1800
					MinExecTime:     2.0,                // min(2.1, 2.0, 2.2)
					MaxExecTime:     52.1,               // max(50.3, 48.5, 52.1)
				},
			},
		},
		{
			name:     "Empty input",
			input:    []*QueryStats{},
			expected: []*QueryStats{},
		},
		{
			name: "Single entry",
			input: []*QueryStats{
				{
					QueryID:         123,
					QueryText:       "SELECT * FROM users",
					ExecutionCount:  100,
					RowsProcessed:   1000,
					TotalExecTime:   1500.5,
					AverageExecTime: 15.005,
					MinExecTime:     5.2,
					MaxExecTime:     25.8,
				},
			},
			expected: []*QueryStats{
				{
					QueryID:         123,
					QueryText:       "SELECT * FROM users",
					ExecutionCount:  100,
					RowsProcessed:   1000,
					TotalExecTime:   1500.5,
					AverageExecTime: 15.005,
					MinExecTime:     5.2,
					MaxExecTime:     25.8,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeQueryStatsBasedOnQuery(tt.input)
			assert.Equal(t, len(tt.expected), len(result), "Number of merged entries should match")

			// Create maps for easier comparison (order doesn't matter)
			expectedMap := make(map[string]*QueryStats)
			for _, entry := range tt.expected {
				expectedMap[entry.QueryText] = entry
			}

			resultMap := make(map[string]*QueryStats)
			for _, entry := range result {
				resultMap[entry.QueryText] = entry
			}

			// Compare each query
			for queryText, expected := range expectedMap {
				actual, ok := resultMap[queryText]
				assert.True(t, ok, "Expected query not found: %s", queryText)
				if ok {
					assert.Equal(t, expected.QueryID, actual.QueryID)
					assert.Equal(t, expected.QueryText, actual.QueryText)
					assert.Equal(t, expected.ExecutionCount, actual.ExecutionCount)
					assert.Equal(t, expected.RowsProcessed, actual.RowsProcessed)
					assert.InDelta(t, expected.TotalExecTime, actual.TotalExecTime, 0.001)
					assert.InDelta(t, expected.AverageExecTime, actual.AverageExecTime, 0.001)
					assert.InDelta(t, expected.MinExecTime, actual.MinExecTime, 0.001)
					assert.InDelta(t, expected.MaxExecTime, actual.MaxExecTime, 0.001)
				}
			}
		})
	}
}
