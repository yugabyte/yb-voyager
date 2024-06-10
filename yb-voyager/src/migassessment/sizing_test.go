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

package migassessment

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

const (
	SOURCEDB_SELECT_QUERY = "SELECT schema_name, object_name, row_count, reads_per_second, writes_per_second, " +
		"is_index, parent_table_name, size_in_bytes FROM table_index_stats ORDER BY size_in_bytes ASC"
)

var SourceDBColumns = []string{"schema_name", "object_name", "row_count", "reads_per_second", "writes_per_second",
	"is_index", "parent_table_name", "size_in_bytes"}

var colocatedLimits = []ExpDataColocatedLimit{
	{
		maxColocatedSizeSupported:  sql.NullFloat64{Float64: 113, Valid: true},
		numCores:                   sql.NullFloat64{Float64: 2, Valid: true},
		memPerCore:                 sql.NullFloat64{Float64: 4, Valid: true},
		maxSupportedNumTables:      sql.NullInt64{Int64: 2000, Valid: true},
		minSupportedNumTables:      sql.NullFloat64{Float64: 1, Valid: true},
		maxSupportedSelectsPerCore: sql.NullFloat64{Float64: 1175, Valid: true},
		maxSupportedInsertsPerCore: sql.NullFloat64{Float64: 357, Valid: true},
	},
	{
		maxColocatedSizeSupported:  sql.NullFloat64{Float64: 113, Valid: true},
		numCores:                   sql.NullFloat64{Float64: 4, Valid: true},
		memPerCore:                 sql.NullFloat64{Float64: 4, Valid: true},
		maxSupportedNumTables:      sql.NullInt64{Int64: 2000, Valid: true},
		minSupportedNumTables:      sql.NullFloat64{Float64: 1, Valid: true},
		maxSupportedSelectsPerCore: sql.NullFloat64{Float64: 1230, Valid: true},
		maxSupportedInsertsPerCore: sql.NullFloat64{Float64: 400, Valid: true},
	},
	{
		maxColocatedSizeSupported:  sql.NullFloat64{Float64: 113, Valid: true},
		numCores:                   sql.NullFloat64{Float64: 8, Valid: true},
		memPerCore:                 sql.NullFloat64{Float64: 4, Valid: true},
		maxSupportedNumTables:      sql.NullInt64{Int64: 5000, Valid: true},
		minSupportedNumTables:      sql.NullFloat64{Float64: 1, Valid: true},
		maxSupportedSelectsPerCore: sql.NullFloat64{Float64: 1246, Valid: true},
		maxSupportedInsertsPerCore: sql.NullFloat64{Float64: 608, Valid: true},
	},
	{
		maxColocatedSizeSupported:  sql.NullFloat64{Float64: 113, Valid: true},
		numCores:                   sql.NullFloat64{Float64: 16, Valid: true},
		memPerCore:                 sql.NullFloat64{Float64: 4, Valid: true},
		maxSupportedNumTables:      sql.NullInt64{Int64: 5000, Valid: true},
		minSupportedNumTables:      sql.NullFloat64{Float64: 1, Valid: true},
		maxSupportedSelectsPerCore: sql.NullFloat64{Float64: 1220, Valid: true},
		maxSupportedInsertsPerCore: sql.NullFloat64{Float64: 755, Valid: true},
	},
}

/*
===== 	Test functions to test getSourceMetadata function	=====
*/
func TestGetSourceMetadata_Success(t *testing.T) {
	db, mock := createMockDB(t)
	rows := sqlmock.NewRows(SourceDBColumns).
		AddRow("public", "table1", 1000, 10, 5, false, "", 1048576000).
		AddRow("public", "index1", 0, 0, 0, true, "table1", 104857600)

	mock.ExpectQuery(SOURCEDB_SELECT_QUERY).WillReturnRows(rows)

	sourceTableMetadata, sourceIndexMetadata, totalSourceDBSize, err := getSourceMetadata(db)
	// assert if there are errors
	assert.NoError(t, err)
	// check if the total tables are equal to expected tables
	assert.Len(t, sourceTableMetadata, 1)
	// check if the total indexes are equal to expected indexes
	assert.Len(t, sourceIndexMetadata, 1)
	// check if the total size of the source database is equal to the expected size
	assert.True(t, 1.07 == Round(totalSourceDBSize, 2))
	// check if the values of the source table metadata are equal to the expected values
	assert.Equal(t, "public", sourceTableMetadata[0].SchemaName)
	assert.Equal(t, "table1", sourceTableMetadata[0].ObjectName)
	assert.Equal(t, "public", sourceIndexMetadata[0].SchemaName)
	assert.Equal(t, "index1", sourceIndexMetadata[0].ObjectName)
}

func TestGetSourceMetadata_QueryError(t *testing.T) {
	db, mock := createMockDB(t)
	mock.ExpectQuery(SOURCEDB_SELECT_QUERY).WillReturnError(errors.New("query error"))

	_, _, _, err := getSourceMetadata(db)
	assert.Error(t, err)
	// check if the error returned contains the expected string
	assert.Contains(t, err.Error(), "failed to query source metadata")
}

func TestGetSourceMetadata_RowScanError(t *testing.T) {
	db, mock := createMockDB(t)
	// 4th column is expected to be int, but as it is float, it will throw an error
	rows := sqlmock.NewRows(SourceDBColumns).AddRow("public", "table1", 1000, 10.5, 5, false, "", 1048576000).
		RowError(1, errors.New("row scan error"))
	mock.ExpectQuery(SOURCEDB_SELECT_QUERY).WillReturnRows(rows)

	_, _, _, err := getSourceMetadata(db)
	assert.Error(t, err)
	// check if the error is as expected
	assert.Contains(t, err.Error(), "failed to read from result set of query source metadata")
}

func TestGetSourceMetadata_NoRows(t *testing.T) {
	db, mock := createMockDB(t)
	rows := sqlmock.NewRows(SourceDBColumns)
	mock.ExpectQuery(SOURCEDB_SELECT_QUERY).WillReturnRows(rows)
	sourceTableMetadata, sourceIndexMetadata, totalSourceDBSize, err := getSourceMetadata(db)

	assert.NoError(t, err)
	//  since there is no mock data, all the fields are expected to be empty
	assert.Empty(t, sourceTableMetadata)
	assert.Empty(t, sourceIndexMetadata)
	assert.Equal(t, 0.0, totalSourceDBSize)
}

/*
===== 	Test functions to test shardingBasedOnTableSizeAndCount function	=====
*/
func TestShardingBasedOnTableSizeAndCount_Basic(t *testing.T) {
	sourceTableMetadata := []SourceDBMetadata{
		{SchemaName: "public", ObjectName: "table1", Size: 100},
	}
	var sourceIndexMetadata []SourceDBMetadata
	recommendation := map[int]IntermediateRecommendation{2: {}, 4: {}, 8: {}, 16: {}}

	expectedRecommendation := map[int]IntermediateRecommendation{
		2: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 100},
			},
			ShardedTables: nil,
			ColocatedSize: 100,
			ShardedSize:   0,
		},
		4: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 100},
			},
			ShardedTables: nil,
			ColocatedSize: 100,
			ShardedSize:   0,
		},
		8: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 100},
			},
			ShardedTables: nil,
			ColocatedSize: 100,
			ShardedSize:   0,
		},
		16: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 100},
			},
			ShardedTables: nil,
			ColocatedSize: 100,
			ShardedSize:   0,
		},
	}

	actualRecommendation := shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata, colocatedLimits, recommendation)
	assert.Equal(t, expectedRecommendation, actualRecommendation)
}

func TestShardingBasedOnTableSizeAndCount_WithIndexes(t *testing.T) {
	sourceTableMetadata := []SourceDBMetadata{
		{SchemaName: "public", ObjectName: "table1", Size: 100},
	}
	sourceIndexMetadata := []SourceDBMetadata{
		{SchemaName: "public", ObjectName: "index1", Size: 20, IsIndex: true, ParentTableName: sql.NullString{String: "table1", Valid: true}},
	}
	recommendation := map[int]IntermediateRecommendation{2: {}, 4: {}, 8: {}, 16: {}}

	expectedRecommendation := map[int]IntermediateRecommendation{
		2: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 100},
			},
			ShardedTables: nil,
			ColocatedSize: 100, // Table size
			ShardedSize:   0,
		},
		4: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 100},
			},
			ShardedTables: nil,
			ColocatedSize: 100, // Table size + index size
			ShardedSize:   0,
		},
		8: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 100},
			},
			ShardedTables: nil,
			ColocatedSize: 100, // Table size + index size
			ShardedSize:   0,
		},
		16: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 100},
			},
			ShardedTables: nil,
			ColocatedSize: 100, // Table size + index size
			ShardedSize:   0,
		},
	}

	result := shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata, colocatedLimits, recommendation)
	assert.Equal(t, expectedRecommendation, result)
}

func TestShardingBasedOnTableSizeAndCount_ColocatedLimitExceededBySize(t *testing.T) {
	sourceTableMetadata := []SourceDBMetadata{
		{SchemaName: "public", ObjectName: "table1", Size: 110},
		{SchemaName: "public", ObjectName: "table2", Size: 500},
	}
	sourceIndexMetadata := []SourceDBMetadata{}
	recommendation := map[int]IntermediateRecommendation{2: {}, 4: {}, 8: {}, 16: {}}

	expectedRecommendation := map[int]IntermediateRecommendation{
		2: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 110},
			},
			ShardedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table2", Size: 500},
			},
			ColocatedSize: 110,
			ShardedSize:   500,
		},
		4: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 110},
			},
			ShardedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table2", Size: 500},
			},
			ColocatedSize: 110,
			ShardedSize:   500,
		},
		8: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 110},
			},
			ShardedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table2", Size: 500},
			},
			ColocatedSize: 110,
			ShardedSize:   500,
		},
		16: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 110},
			},
			ShardedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table2", Size: 500},
			},
			ColocatedSize: 110,
			ShardedSize:   500,
		},
	}

	result := shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata, colocatedLimits, recommendation)
	assert.Equal(t, expectedRecommendation, result)
}

func TestShardingBasedOnTableSizeAndCount_ColocatedLimitExceededByCount(t *testing.T) {
	const numTables = 16000
	sourceTableMetadata := make([]SourceDBMetadata, numTables)
	for i := 0; i < numTables; i++ {
		sourceTableMetadata[i] = SourceDBMetadata{SchemaName: "public", ObjectName: fmt.Sprintf("table%v", i), Size: 0.0001}
	}

	sourceIndexMetadata := []SourceDBMetadata{}
	recommendation := map[int]IntermediateRecommendation{2: {}, 4: {}, 8: {}, 16: {}}

	expectedResults := make(map[int]map[string]int)
	expectedResults[2] = map[string]int{
		"lenColocatedTables": 2000,
		"lenShardedTables":   14000,
	}
	expectedResults[4] = map[string]int{
		"lenColocatedTables": 2000,
		"lenShardedTables":   14000,
	}
	expectedResults[8] = map[string]int{
		"lenColocatedTables": 5000,
		"lenShardedTables":   11000,
	}
	expectedResults[16] = map[string]int{
		"lenColocatedTables": 5000,
		"lenShardedTables":   11000,
	}

	actualRecommendationsResult := shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata, colocatedLimits, recommendation)
	for key, rec := range actualRecommendationsResult {
		assert.Equal(t, expectedResults[key]["lenColocatedTables"], len(rec.ColocatedTables))
		assert.Equal(t, expectedResults[key]["lenShardedTables"], len(rec.ShardedTables))
	}
}

func TestShardingBasedOnTableSizeAndCount_NoColocatedTables(t *testing.T) {
	sourceTableMetadata := []SourceDBMetadata{
		{SchemaName: "public", ObjectName: "table1", Size: 600},
		{SchemaName: "public", ObjectName: "table2", Size: 500},
	}
	sourceIndexMetadata := []SourceDBMetadata{}
	recommendation := map[int]IntermediateRecommendation{2: {}, 4: {}, 8: {}, 16: {}}

	expectedResults := make(map[int]map[string]int)
	expectedResults[2] = map[string]int{
		"lenColocatedTables": 0,
		"lenShardedTables":   2,
	}
	expectedResults[4] = map[string]int{
		"lenColocatedTables": 0,
		"lenShardedTables":   2,
	}
	expectedResults[8] = map[string]int{
		"lenColocatedTables": 0,
		"lenShardedTables":   2,
	}
	expectedResults[16] = map[string]int{
		"lenColocatedTables": 0,
		"lenShardedTables":   2,
	}

	result := shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata, colocatedLimits, recommendation)
	for key, rec := range result {
		assert.Equal(t, expectedResults[key]["lenColocatedTables"], len(rec.ColocatedTables))
		assert.Equal(t, expectedResults[key]["lenShardedTables"], len(rec.ShardedTables))
	}
}

/*
===== 	Test functions to test shardingBasedOnOperations function	=====
*/
func TestShardingBasedOnOperations(t *testing.T) {
	// Define test data
	sourceIndexMetadata := []SourceDBMetadata{
		{ObjectName: "table1", Size: 10.0, ReadsPerSec: 1000000, WritesPerSec: 50000000},
	}
	recommendation := map[int]IntermediateRecommendation{
		2: {
			ColocatedTables: []SourceDBMetadata{{ObjectName: "table1", Size: 10.0, ReadsPerSec: 1000000, WritesPerSec: 50000000}},
			ShardedTables:   []SourceDBMetadata{},
			ColocatedSize:   10.0,
			ShardedSize:     0.0,
		},
		4: {
			ColocatedTables: []SourceDBMetadata{{ObjectName: "table1", Size: 10.0, ReadsPerSec: 1000000, WritesPerSec: 50000000}},
			ShardedTables:   []SourceDBMetadata{},
			ColocatedSize:   10.0,
			ShardedSize:     0.0,
		},
		8: {
			ColocatedTables: []SourceDBMetadata{{ObjectName: "table1", Size: 10.0, ReadsPerSec: 1000000, WritesPerSec: 50000000}},
			ShardedTables:   []SourceDBMetadata{},
			ColocatedSize:   10.0,
			ShardedSize:     0.0,
		},
		16: {
			ColocatedTables: []SourceDBMetadata{{ObjectName: "table1", Size: 10.0, ReadsPerSec: 1000000, WritesPerSec: 50000000}},
			ShardedTables:   []SourceDBMetadata{},
			ColocatedSize:   10.0,
			ShardedSize:     0.0,
		},
	}

	// Run the function
	updatedRecommendation := shardingBasedOnOperations(sourceIndexMetadata, colocatedLimits, recommendation)

	// expected is that the table should be removed from colocated and added to sharded as the ops requirement is high
	for _, rec := range updatedRecommendation {
		assert.Empty(t, rec.ColocatedTables)
		assert.Len(t, rec.ShardedTables, 1)
		assert.Equal(t, "table1", rec.ShardedTables[0].ObjectName)
	}
}

/*
===== 	Test functions to test checkShardedTableLimit function	=====
*/
func TestCheckShardedTableLimit(t *testing.T) {
	// Define test data
	var sourceIndexMetadata []SourceDBMetadata

	shardedLimits := []ExpDataShardedLimit{
		{numCores: sql.NullFloat64{Float64: 16, Valid: true}, maxSupportedNumTables: sql.NullInt64{Int64: 1, Valid: true}},
	}

	recommendation := map[int]IntermediateRecommendation{
		16: {
			ShardedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 10.0, ReadsPerSec: 100, WritesPerSec: 50},
				{SchemaName: "public", ObjectName: "table2", Size: 10.0, ReadsPerSec: 100, WritesPerSec: 50},
			},
			VCPUsPerInstance: 16,
			MemoryPerCore:    4,
		},
	}

	// Run the function
	updatedRecommendation := checkShardedTableLimit(sourceIndexMetadata, shardedLimits, recommendation)

	// check if the Failure reasoning matches with the one generated
	for _, rec := range updatedRecommendation {
		// establish expected reasoning
		expectedFailureReasoning := fmt.Sprintf("Cannot support 2 sharded objects on a machine with %d cores"+
			" and %dGiB memory.", rec.VCPUsPerInstance, rec.VCPUsPerInstance*rec.MemoryPerCore)
		assert.Equal(t, expectedFailureReasoning, rec.FailureReasoning)
	}
}

/*
===== 	Test functions to test findNumNodesNeededBasedOnThroughputRequirement function	=====
*/
func TestFindNumNodesNeededBasedOnThroughputRequirement_Positive(t *testing.T) {
	// Define test data
	sourceIndexMetadata := []SourceDBMetadata{
		{ObjectName: "table1", Size: 10.0, ReadsPerSec: 100, WritesPerSec: 50},
		{ObjectName: "table2", Size: 20.0, ReadsPerSec: 200, WritesPerSec: 100},
	}

	shardedLimits := []ExpDataShardedThroughput{
		{
			numCores:                   sql.NullFloat64{Float64: 4},
			maxSupportedSelectsPerCore: sql.NullFloat64{Float64: 200},
			maxSupportedInsertsPerCore: sql.NullFloat64{Float64: 100},
			selectConnPerNode:          sql.NullInt64{Int64: 50},
			insertConnPerNode:          sql.NullInt64{Int64: 25},
		},
	}

	recommendation := map[int]IntermediateRecommendation{
		4: {
			ColocatedTables: []SourceDBMetadata{},
			ShardedTables:   []SourceDBMetadata{{ObjectName: "table1", Size: 10.0, ReadsPerSec: 100, WritesPerSec: 50}},
		},
	}

	// Run the function
	updatedRecommendation := findNumNodesNeededBasedOnThroughputRequirement(sourceIndexMetadata, shardedLimits, recommendation)

	// for 4 cores data, expected results are
	var expectedOptimalSelectConnectionsPerNode int64 = 50
	var expectedOptimalInsertConnectionsPerNode int64 = 25
	var expectedNumNodesNeeded float64 = 3

	// Check the results
	recommendationToVerify := updatedRecommendation[4]
	assert.Equal(t, expectedOptimalSelectConnectionsPerNode, recommendationToVerify.OptimalSelectConnectionsPerNode)
	assert.Equal(t, expectedOptimalInsertConnectionsPerNode, recommendationToVerify.OptimalInsertConnectionsPerNode)
	assert.Equal(t, expectedNumNodesNeeded, recommendationToVerify.NumNodes)
}

func TestFindNumNodesNeededBasedOnThroughputRequirement_Negative(t *testing.T) {
	// Define test data
	var sourceIndexMetadata []SourceDBMetadata

	shardedLimits := []ExpDataShardedThroughput{
		{
			numCores:                   sql.NullFloat64{Float64: 4},
			maxSupportedSelectsPerCore: sql.NullFloat64{Float64: 200},
			maxSupportedInsertsPerCore: sql.NullFloat64{Float64: 100},
		},
	}

	recommendation := map[int]IntermediateRecommendation{
		4: {
			ShardedTables:    []SourceDBMetadata{{ObjectName: "table2", Size: 20.0, ReadsPerSec: 2000, WritesPerSec: 5000}},
			VCPUsPerInstance: 4,
			MemoryPerCore:    4,
		},
	}

	// Run the function
	updatedRecommendation := findNumNodesNeededBasedOnThroughputRequirement(sourceIndexMetadata, shardedLimits, recommendation)

	// validate the expected number of nodes
	assert.Equal(t, updatedRecommendation[4].NumNodes, float64(15))
}

/*
===== 	Test functions to test findNumNodesNeededBasedOnTabletsRequired function	=====
*/

func TestFindNumNodesNeededBasedOnTabletsRequired_Positive_NoChange(t *testing.T) {
	// Define test data
	var sourceIndexMetadata []SourceDBMetadata

	shardedLimits := []ExpDataShardedLimit{
		{
			numCores:              sql.NullFloat64{Float64: 4, Valid: true},
			maxSupportedNumTables: sql.NullInt64{Int64: 20, Valid: true},
		},
	}

	recommendation := map[int]IntermediateRecommendation{
		4: {
			ColocatedTables: []SourceDBMetadata{},
			ShardedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 20},
				{SchemaName: "public", ObjectName: "table2", Size: 120},
			},
			VCPUsPerInstance: 4,
			NumNodes:         3,
		},
	}

	// Run the function
	updatedRecommendation := findNumNodesNeededBasedOnTabletsRequired(sourceIndexMetadata, shardedLimits, recommendation)

	// check if the num nodes in updated recommendation is same as before(3) meaning no scaling is required
	assert.Equal(t, float64(3), updatedRecommendation[4].NumNodes)
}

func TestFindNumNodesNeededBasedOnTabletsRequired_Positive(t *testing.T) {
	// Define test data
	sourceIndexMetadata := []SourceDBMetadata{
		{SchemaName: "public", ObjectName: "index1", Size: 7, ParentTableName: sql.NullString{String: "public.table1"}},
		{SchemaName: "public", ObjectName: "index2", Size: 3, ParentTableName: sql.NullString{String: "public.table2"}},
	}

	shardedLimits := []ExpDataShardedLimit{
		{
			numCores:              sql.NullFloat64{Float64: 4, Valid: true},
			maxSupportedNumTables: sql.NullInt64{Int64: 20, Valid: true},
		},
	}

	recommendation := map[int]IntermediateRecommendation{
		4: {
			ColocatedTables: []SourceDBMetadata{},
			ShardedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: 250},
				{SchemaName: "public", ObjectName: "table2", Size: 120},
			},
			VCPUsPerInstance: 4,
			NumNodes:         3,
		},
	}

	// Run the function
	updatedRecommendation := findNumNodesNeededBasedOnTabletsRequired(sourceIndexMetadata, shardedLimits, recommendation)

	// check if the num nodes in updated recommendation has increased. Meaning scaling is required.
	assert.Equal(t, float64(6), updatedRecommendation[4].NumNodes)
}

/*
===== 	Test functions to test pickBestRecommendation function	=====
*/

func TestPickBestRecommendation(t *testing.T) {
	recommendations := map[int]IntermediateRecommendation{
		4: {
			VCPUsPerInstance: 4,
			NumNodes:         10,
			FailureReasoning: "",
		},
		8: {
			VCPUsPerInstance: 8,
			NumNodes:         3,
			FailureReasoning: "",
		},
	}
	bestPick := pickBestRecommendation(recommendations)
	// validate the best recommendation which is 8 vcpus per instance is picked up
	assert.Equal(t, 8, bestPick.VCPUsPerInstance)
}

/*
===== 	Test functions to test calculateTimeTakenAndParallelJobsForImport function	=====
*/

func TestCalculateTimeTakenAndParallelJobsForImport_Positive(t *testing.T) {
	db, mock := createMockDB(t)
	// Define the mock response for the query
	rows := sqlmock.NewRows([]string{"csv_size_gb", "migration_time_secs", "parallel_threads"}).
		AddRow(100, 6000, 4)
	mock.ExpectQuery("(?i)SELECT csv_size_gb, migration_time_secs, parallel_threads FROM .* WHERE num_cores = .*").
		WithArgs(4, 4, 50.0, 4).
		WillReturnRows(rows)

	// Define test data
	dbObjects := []SourceDBMetadata{{Size: 30.0}, {Size: 20.0}}

	// Call the function
	estimatedTime, parallelJobs, err := calculateTimeTakenAndParallelJobsForImport(COLOCATED_LOAD_TIME_TABLE, dbObjects, 4, 4, db)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Define expected results
	expectedTime := 50.0 // Calculated as ((6000 * 50) / 100) / 60
	expectedJobs := int64(4)

	if estimatedTime != expectedTime || parallelJobs != expectedJobs {
		t.Errorf("calculateTimeTakenAndParallelJobsForImport() = (%v, %v), want (%v, %v)", estimatedTime, parallelJobs, expectedTime, expectedJobs)
	}

	// Ensure all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}

/*
===== 	Test functions to test getReasoning function	=====
*/
func TestGetReasoning_NoObjects(t *testing.T) {
	recommendation := IntermediateRecommendation{VCPUsPerInstance: 4, MemoryPerCore: 16}
	var shardedObjects []SourceDBMetadata
	var colocatedObjects []SourceDBMetadata
	expected := "Recommended instance type with 4 vCPU and 64 GiB memory could fit "

	result := getReasoning(recommendation, shardedObjects, 0, colocatedObjects, 0)
	if result != expected {
		t.Errorf("Expected %q but got %q", expected, result)
	}
}

func TestGetReasoning_OnlyColocatedObjects(t *testing.T) {
	recommendation := IntermediateRecommendation{VCPUsPerInstance: 8, MemoryPerCore: 8}
	var shardedObjects []SourceDBMetadata
	colocatedObjects := []SourceDBMetadata{
		{Size: 50.0, ReadsPerSec: 1000, WritesPerSec: 500},
		{Size: 30.0, ReadsPerSec: 2000, WritesPerSec: 1500},
	}
	expected := "Recommended instance type with 8 vCPU and 64 GiB memory could fit 2 objects(2 tables and 0 explicit/implicit indexes) with 80.00 GB size and throughput requirement of 3000 reads/sec and 2000 writes/sec as colocated."

	result := getReasoning(recommendation, shardedObjects, 0, colocatedObjects, 0)
	if result != expected {
		t.Errorf("Expected %q but got %q", expected, result)
	}
}

func TestGetReasoning_OnlyShardedObjects(t *testing.T) {
	recommendation := IntermediateRecommendation{VCPUsPerInstance: 16, MemoryPerCore: 4}
	shardedObjects := []SourceDBMetadata{
		{Size: 100.0, ReadsPerSec: 4000, WritesPerSec: 3000},
		{Size: 200.0, ReadsPerSec: 5000, WritesPerSec: 4000},
	}
	var colocatedObjects []SourceDBMetadata
	expected := "Recommended instance type with 16 vCPU and 64 GiB memory could fit 2 objects(2 tables and 0 explicit/implicit indexes) with 300.00 GB size and throughput requirement of 9000 reads/sec and 7000 writes/sec as sharded."

	result := getReasoning(recommendation, shardedObjects, 0, colocatedObjects, 0)
	if result != expected {
		t.Errorf("Expected %q but got %q", expected, result)
	}
}

func TestGetReasoning_ColocatedAndShardedObjects(t *testing.T) {
	recommendation := IntermediateRecommendation{VCPUsPerInstance: 32, MemoryPerCore: 2}
	shardedObjects := []SourceDBMetadata{
		{Size: 150.0, ReadsPerSec: 7000, WritesPerSec: 6000},
	}
	colocatedObjects := []SourceDBMetadata{
		{Size: 70.0, ReadsPerSec: 3000, WritesPerSec: 2000},
		{Size: 50.0, ReadsPerSec: 2000, WritesPerSec: 1000},
	}
	expected := "Recommended instance type with 32 vCPU and 64 GiB memory could fit 2 objects(2 tables and 0 explicit/implicit indexes) with 120.00 GB size and throughput requirement of 5000 reads/sec and 3000 writes/sec as colocated. Rest 1 objects(1 tables and 0 explicit/implicit indexes) with 150.00 GB size and throughput requirement of 7000 reads/sec and 6000 writes/sec need to be migrated as range partitioned tables"

	result := getReasoning(recommendation, shardedObjects, 0, colocatedObjects, 0)
	if result != expected {
		t.Errorf("Expected %q but got %q", expected, result)
	}
}

func TestGetReasoning_Indexes(t *testing.T) {
	recommendation := IntermediateRecommendation{VCPUsPerInstance: 4, MemoryPerCore: 16}
	shardedObjects := []SourceDBMetadata{
		{Size: 200.0, ReadsPerSec: 6000, WritesPerSec: 4000},
	}
	colocatedObjects := []SourceDBMetadata{
		{Size: 100.0, ReadsPerSec: 3000, WritesPerSec: 2000},
	}
	expected := "Recommended instance type with 4 vCPU and 64 GiB memory could fit 1 objects(0 tables and 1 explicit/implicit indexes) with 100.00 GB size and throughput requirement of 3000 reads/sec and 2000 writes/sec as colocated. Rest 1 objects(0 tables and 1 explicit/implicit indexes) with 200.00 GB size and throughput requirement of 6000 reads/sec and 4000 writes/sec need to be migrated as range partitioned tables"

	result := getReasoning(recommendation, shardedObjects, 1, colocatedObjects, 1)
	if result != expected {
		t.Errorf("Expected %q but got %q", expected, result)
	}
}

/*
====================HELPER FUNCTIONS====================
*/

// Round rounds a float64 to n decimal places.
func Round(val float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

func createMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}
