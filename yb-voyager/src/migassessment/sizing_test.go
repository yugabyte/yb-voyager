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

package migassessment

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

var AssessmentDbSelectQuery = fmt.Sprintf("(?i)SELECT schema_name,.* FROM %v .* ORDER BY .* ASC", TABLE_INDEX_STATS)
var AssessmentDBColumns = []string{"schema_name", "object_name", "row_count", "reads_per_second", "writes_per_second",
	"is_index", "parent_table_name", "size_in_bytes", "column_count"}

var colocatedThroughput = []ExpDataThroughput{
	{
		numCores:                   sql.NullFloat64{Float64: 4, Valid: true},
		memPerCore:                 sql.NullFloat64{Float64: 4, Valid: true},
		maxSupportedSelectsPerCore: sql.NullFloat64{Float64: 1230, Valid: true},
		maxSupportedInsertsPerCore: sql.NullFloat64{Float64: 400, Valid: true},
		selectConnPerNode:          sql.NullInt64{Int64: 8, Valid: true},
		insertConnPerNode:          sql.NullInt64{Int64: 12, Valid: true},
	},
	{
		numCores:                   sql.NullFloat64{Float64: 8, Valid: true},
		memPerCore:                 sql.NullFloat64{Float64: 4, Valid: true},
		maxSupportedSelectsPerCore: sql.NullFloat64{Float64: 1246, Valid: true},
		maxSupportedInsertsPerCore: sql.NullFloat64{Float64: 608, Valid: true},
		selectConnPerNode:          sql.NullInt64{Int64: 16, Valid: true},
		insertConnPerNode:          sql.NullInt64{Int64: 24, Valid: true},
	},
	{
		numCores:                   sql.NullFloat64{Float64: 16, Valid: true},
		memPerCore:                 sql.NullFloat64{Float64: 4, Valid: true},
		maxSupportedSelectsPerCore: sql.NullFloat64{Float64: 1220, Valid: true},
		maxSupportedInsertsPerCore: sql.NullFloat64{Float64: 755, Valid: true},
		selectConnPerNode:          sql.NullInt64{Int64: 32, Valid: true},
		insertConnPerNode:          sql.NullInt64{Int64: 48, Valid: true},
	},
}
var colocatedLimits = []ExpDataColocatedLimit{
	{
		maxColocatedSizeSupported: sql.NullFloat64{Float64: 113, Valid: true},
		numCores:                  sql.NullFloat64{Float64: 4, Valid: true},
		memPerCore:                sql.NullFloat64{Float64: 4, Valid: true},
		maxSupportedNumTables:     sql.NullInt64{Int64: 2000, Valid: true},
		minSupportedNumTables:     sql.NullFloat64{Float64: 1, Valid: true},
	},
	{
		maxColocatedSizeSupported: sql.NullFloat64{Float64: 113, Valid: true},
		numCores:                  sql.NullFloat64{Float64: 8, Valid: true},
		memPerCore:                sql.NullFloat64{Float64: 4, Valid: true},
		maxSupportedNumTables:     sql.NullInt64{Int64: 5000, Valid: true},
		minSupportedNumTables:     sql.NullFloat64{Float64: 1, Valid: true},
	},
	{
		maxColocatedSizeSupported: sql.NullFloat64{Float64: 113, Valid: true},
		numCores:                  sql.NullFloat64{Float64: 16, Valid: true},
		memPerCore:                sql.NullFloat64{Float64: 4, Valid: true},
		maxSupportedNumTables:     sql.NullInt64{Int64: 5000, Valid: true},
		minSupportedNumTables:     sql.NullFloat64{Float64: 1, Valid: true},
	},
}

/*
===== 	Test functions to test getSourceMetadata function	=====
*/
// verify successfully able to connect and load the source metadata
func TestGetSourceMetadata_SuccessReadingSourceMetadata(t *testing.T) {
	db, mock := createMockDB(t)
	rows := sqlmock.NewRows(AssessmentDBColumns).
		AddRow("public", "table1", 1000, 10, 5, false, "", 1048576000, 5).
		AddRow("public", "index1", 0, 0, 0, true, "table1", 104857600, 5)

	mock.ExpectQuery(AssessmentDbSelectQuery).WillReturnRows(rows)

	// Mock the redundant indexes query
	redundantIndexRows := sqlmock.NewRows([]string{"redundant_schema_name", "redundant_table_name", "redundant_index_name"})
	redundantIndexQuery := fmt.Sprintf(`SELECT redundant_schema_name, redundant_table_name, redundant_index_name FROM %s`, GetTableRedundantIndexesName())
	mock.ExpectQuery(regexp.QuoteMeta(redundantIndexQuery)).WillReturnRows(redundantIndexRows)

	sourceTableMetadata, sourceIndexMetadata, _, totalSourceDBSize, err := getSourceMetadata(db, SourceDBType)
	// assert if there are errors
	assert.NoError(t, err)
	// check if the total tables are equal to expected tables
	assert.Len(t, sourceTableMetadata, 1)
	// check if the total indexes are equal to expected indexes
	assert.Len(t, sourceIndexMetadata, 1)
	// check if the total size of the source database is equal to the expected size
	assert.Equal(t, 1.07, Round(totalSourceDBSize, 2))
	// check if the values of the source table metadata are equal to the expected values
	assert.Equal(t, "public", sourceTableMetadata[0].SchemaName)
	assert.Equal(t, "table1", sourceTableMetadata[0].ObjectName)
	assert.Equal(t, "public", sourceIndexMetadata[0].SchemaName)
	assert.Equal(t, "index1", sourceIndexMetadata[0].ObjectName)
}

// if table_index_stat does not exist in the assessment.db or one of the required column does not exist in the table
func TestGetSourceMetadata_QueryErrorIfTableDoesNotExistOrColumnsUnavailable(t *testing.T) {
	db, mock := createMockDB(t)
	mock.ExpectQuery(AssessmentDbSelectQuery).WillReturnError(errors.New("query error"))

	_, _, _, _, err := getSourceMetadata(db, SourceDBType)
	assert.Error(t, err)
	// check if the error returned contains the expected string
	assert.Contains(t, err.Error(), "failed to query source metadata")
}

// verify if the all columns datatypes are correct. Expecting failure in case of unsupported data type. Example:
// expected column value is int but in assessment.db, float value is provided. Hence, it will fail to read/typecast
func TestGetSourceMetadata_RowScanError(t *testing.T) {
	db, mock := createMockDB(t)
	// 4th column is expected to be int, but as it is float, it will throw an error
	rows := sqlmock.NewRows(AssessmentDBColumns).AddRow("public", "table1", 1000, 10.5, 5, false, "", 1048576000, 5).
		RowError(1, errors.New("row scan error"))
	mock.ExpectQuery(AssessmentDbSelectQuery).WillReturnRows(rows)

	_, _, _, _, err := getSourceMetadata(db, SourceDBType)
	assert.Error(t, err)
	// check if the error is as expected
	assert.Contains(t, err.Error(), "failed to read from result set of query source metadata")
}

// verify if there are no rows in the source metadata
func TestGetSourceMetadata_NoRows(t *testing.T) {
	db, mock := createMockDB(t)
	rows := sqlmock.NewRows(AssessmentDBColumns)
	mock.ExpectQuery(AssessmentDbSelectQuery).WillReturnRows(rows)

	// Mock the redundant indexes query
	redundantIndexRows := sqlmock.NewRows([]string{"redundant_schema_name", "redundant_table_name", "redundant_index_name"})
	redundantIndexQuery := fmt.Sprintf(`SELECT redundant_schema_name, redundant_table_name, redundant_index_name FROM %s`, GetTableRedundantIndexesName())
	mock.ExpectQuery(regexp.QuoteMeta(redundantIndexQuery)).WillReturnRows(redundantIndexRows)

	sourceTableMetadata, sourceIndexMetadata, _, totalSourceDBSize, err := getSourceMetadata(db, SourceDBType)

	assert.NoError(t, err)
	//  since there is no mock data, all the fields are expected to be empty
	assert.Empty(t, sourceTableMetadata)
	assert.Empty(t, sourceIndexMetadata)
	assert.Equal(t, 0.0, totalSourceDBSize)
}

/*
===== 	Test functions to test shardingBasedOnTableSizeAndCount function	=====
*/
// validate if the source table of size in colocated limit is correctly placed in colocated table recommendation
func TestShardingBasedOnTableSizeAndCount_TableWithSizePlacedInColocated(t *testing.T) {
	sourceTableMetadata := []SourceDBMetadata{
		{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 100, Valid: true}},
	}
	var sourceIndexMetadata []SourceDBMetadata
	recommendation := map[int]IntermediateRecommendation{4: {}, 8: {}, 16: {}}

	expectedRecommendation := map[int]IntermediateRecommendation{
		4: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 100, Valid: true}},
			},
			ShardedTables: nil,
			ColocatedSize: 100,
			ShardedSize:   0,
		},
		8: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 100, Valid: true}},
			},
			ShardedTables: nil,
			ColocatedSize: 100,
			ShardedSize:   0,
		},
		16: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 100, Valid: true}},
			},
			ShardedTables: nil,
			ColocatedSize: 100,
			ShardedSize:   0,
		},
	}

	actualRecommendation :=
		shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata, colocatedLimits, recommendation)
	assert.Equal(t, expectedRecommendation, actualRecommendation)
}

// validate if the source table and index of size in colocated limit is correctly placed in colocated table
// recommendation
func TestShardingBasedOnTableSizeAndCount_WithIndexes_ColocateAll(t *testing.T) {
	sourceTableMetadata := []SourceDBMetadata{
		{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 100, Valid: true}},
	}
	sourceIndexMetadata := []SourceDBMetadata{
		{
			SchemaName: "public", ObjectName: "index1", Size: sql.NullFloat64{Float64: 10, Valid: true}, IsIndex: true,
			ParentTableName: sql.NullString{String: "public.table1", Valid: true},
		},
	}
	recommendation := map[int]IntermediateRecommendation{4: {}, 8: {}, 16: {}}

	expectedRecommendation := map[int]IntermediateRecommendation{
		4: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 100, Valid: true}},
			},
			ShardedTables: nil,
			ColocatedSize: 110, // Table size + index size
		},
		8: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 100, Valid: true}},
			},
			ShardedTables: nil,
			ColocatedSize: 110, // Table size + index size
		},
		16: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 100, Valid: true}},
			},
			ShardedTables: nil,
			ColocatedSize: 110, // Table size + index size
		},
	}

	result :=
		shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata, colocatedLimits, recommendation)
	assert.Equal(t, expectedRecommendation, result)
}

// validate if the source table and index of size in colocated limit exceeds the size limit and respective tables need
// to be put in sharded
func TestShardingBasedOnTableSizeAndCount_ColocatedLimitExceededBySize(t *testing.T) {
	sourceTableMetadata := []SourceDBMetadata{
		{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 110, Valid: true}},
		{SchemaName: "public", ObjectName: "table2", Size: sql.NullFloat64{Float64: 500, Valid: true}},
	}
	var sourceIndexMetadata []SourceDBMetadata
	recommendation := map[int]IntermediateRecommendation{4: {}, 8: {}, 16: {}}

	expectedRecommendation := map[int]IntermediateRecommendation{
		4: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 110, Valid: true}},
			},
			ShardedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table2", Size: sql.NullFloat64{Float64: 500, Valid: true}},
			},
			ColocatedSize: 110,
			ShardedSize:   500,
		},
		8: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 110, Valid: true}},
			},
			ShardedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table2", Size: sql.NullFloat64{Float64: 500, Valid: true}},
			},
			ColocatedSize: 110,
			ShardedSize:   500,
		},
		16: {
			ColocatedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 110, Valid: true}},
			},
			ShardedTables: []SourceDBMetadata{
				{SchemaName: "public", ObjectName: "table2", Size: sql.NullFloat64{Float64: 500, Valid: true}},
			},
			ColocatedSize: 110,
			ShardedSize:   500,
		},
	}

	result :=
		shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata, colocatedLimits, recommendation)
	assert.Equal(t, expectedRecommendation, result)
}

// validate if the source table and index of size in colocated limit exceeds limit of maximum supported num objects
// and respective tables need to be put in sharded
func TestShardingBasedOnTableSizeAndCount_ColocatedLimitExceededByCount(t *testing.T) {
	const numTables = 16000
	sourceTableMetadata := make([]SourceDBMetadata, numTables)
	for i := 0; i < numTables; i++ {
		sourceTableMetadata[i] =
			SourceDBMetadata{SchemaName: "public", ObjectName: fmt.Sprintf("table%v", i), Size: sql.NullFloat64{Float64: 0.0001, Valid: true}}
	}

	var sourceIndexMetadata []SourceDBMetadata
	recommendation := map[int]IntermediateRecommendation{4: {}, 8: {}, 16: {}}

	expectedResults := make(map[int]map[string]int)
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

	actualRecommendationsResult :=
		shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata, colocatedLimits, recommendation)
	for key, rec := range actualRecommendationsResult {
		assert.Equal(t, expectedResults[key]["lenColocatedTables"], len(rec.ColocatedTables))
		assert.Equal(t, expectedResults[key]["lenShardedTables"], len(rec.ShardedTables))
	}
}

// validate if the tables of size more than max colocated size supported are put in the sharded tables
func TestShardingBasedOnTableSizeAndCount_NoColocatedTables(t *testing.T) {
	sourceTableMetadata := []SourceDBMetadata{
		{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 600, Valid: true}},
		{SchemaName: "public", ObjectName: "table2", Size: sql.NullFloat64{Float64: 500, Valid: true}},
	}
	var sourceIndexMetadata []SourceDBMetadata
	recommendation := map[int]IntermediateRecommendation{4: {}, 8: {}, 16: {}}

	expectedResults := make(map[int]map[string]int)
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

	result :=
		shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata, colocatedLimits, recommendation)
	for key, rec := range result {
		// assert that there are no colocated tables
		assert.Equal(t, expectedResults[key]["lenColocatedTables"], len(rec.ColocatedTables))
		// assert that there are 2 sharded tables
		assert.Equal(t, expectedResults[key]["lenShardedTables"], len(rec.ShardedTables))
	}
}

// validate if the tables with more than COLOCATED_MAX_INDEXES_THRESHOLD indexes are put in the sharded tables
func TestShardingBasedOnTableSizeAndCount_TableWithMoreThanThresholdIndexesAsSharded(t *testing.T) {
	expectedShardedTableName := "table1"
	sourceTableMetadata := []SourceDBMetadata{
		{SchemaName: "public", ObjectName: expectedShardedTableName, Size: sql.NullFloat64{Float64: 10, Valid: true}},
		{SchemaName: "public", ObjectName: "table2", Size: sql.NullFloat64{Float64: 2, Valid: true}},
		{SchemaName: "public", ObjectName: "table3", Size: sql.NullFloat64{Float64: 5, Valid: true}},
	}
	sourceIndexMetadata := []SourceDBMetadata{
		{
			SchemaName: "public", ObjectName: "index1-t1",
			ParentTableName: sql.NullString{String: "public.table1", Valid: true},
			Size:            sql.NullFloat64{Float64: 1, Valid: true},
		},
		{
			SchemaName: "public", ObjectName: "index2-t1",
			ParentTableName: sql.NullString{String: "public.table1", Valid: true},
			Size:            sql.NullFloat64{Float64: 1, Valid: true},
		},
		{
			SchemaName: "public", ObjectName: "index3-t1",
			ParentTableName: sql.NullString{String: "public.table1", Valid: true},
			Size:            sql.NullFloat64{Float64: 1, Valid: true},
		},
		{
			SchemaName: "public", ObjectName: "index6-t1",
			ParentTableName: sql.NullString{String: "public.table1", Valid: true},
			Size:            sql.NullFloat64{Float64: 1, Valid: true},
		},
	}
	recommendation := map[int]IntermediateRecommendation{4: {}, 8: {}, 16: {}}

	expectedResults := make(map[int]map[string]int)
	expectedResults[4] = map[string]int{
		"lenColocatedTables": 2,
		"lenShardedTables":   1,
	}
	expectedResults[8] = map[string]int{
		"lenColocatedTables": 2,
		"lenShardedTables":   1,
	}
	expectedResults[16] = map[string]int{
		"lenColocatedTables": 2,
		"lenShardedTables":   1,
	}

	result :=
		shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata, colocatedLimits, recommendation)
	for key, rec := range result {
		// assert that expectedShardedTableName is the one that is sharded
		assert.Equal(t, expectedShardedTableName, rec.ShardedTables[0].ObjectName)
		// assert that there are no colocated tables
		assert.Equal(t, expectedResults[key]["lenColocatedTables"], len(rec.ColocatedTables))
		// assert that there are 2 sharded tables
		assert.Equal(t, expectedResults[key]["lenShardedTables"], len(rec.ShardedTables))
	}
}

// validate if the tables with COLOCATED_MAX_INDEXES_THRESHOLD or fewer indexes are put in the colocated tables
func TestShardingBasedOnTableSizeAndCount_TableWithThresholdOrLessIndexesAsColocated(t *testing.T) {
	sourceTableMetadata := []SourceDBMetadata{
		{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 10, Valid: true}},
		{SchemaName: "public", ObjectName: "table2", Size: sql.NullFloat64{Float64: 2, Valid: true}},
	}
	sourceIndexMetadata := []SourceDBMetadata{
		{
			SchemaName: "public", ObjectName: "index1-t1",
			ParentTableName: sql.NullString{String: "public.table1", Valid: true},
			Size:            sql.NullFloat64{Float64: 1, Valid: true},
		},
		{
			SchemaName: "public", ObjectName: "index2-t1",
			ParentTableName: sql.NullString{String: "public.table1", Valid: true},
			Size:            sql.NullFloat64{Float64: 1, Valid: true},
		},
	}
	recommendation := map[int]IntermediateRecommendation{4: {}, 8: {}, 16: {}}

	expectedResults := make(map[int]map[string]int)
	expectedResults[4] = map[string]int{
		"lenColocatedTables": 2,
		"lenShardedTables":   0,
	}
	expectedResults[8] = map[string]int{
		"lenColocatedTables": 2,
		"lenShardedTables":   0,
	}
	expectedResults[16] = map[string]int{
		"lenColocatedTables": 2,
		"lenShardedTables":   0,
	}

	result :=
		shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata, colocatedLimits, recommendation)
	for key, rec := range result {
		// assert that there are no colocated tables
		assert.Equal(t, expectedResults[key]["lenColocatedTables"], len(rec.ColocatedTables))
		// assert that there are 2 sharded tables
		assert.Equal(t, expectedResults[key]["lenShardedTables"], len(rec.ShardedTables))
	}
}

/*
===== 	Test functions to test shardingBasedOnOperations function	=====
*/
// validate that table throughput requirements are supported and sharding is not needed
func TestShardingBasedOnOperations_CanSupportOpsRequirement(t *testing.T) {
	// Define test data
	sourceIndexMetadata := []SourceDBMetadata{
		{
			ObjectName: "table1", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
			ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 100},
			WritesPerSec: sql.NullInt64{Valid: true, Int64: 50},
		},
	}
	recommendation := map[int]IntermediateRecommendation{
		4: {
			ColocatedTables: []SourceDBMetadata{
				{
					ObjectName: "table1", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 100},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 50},
				},
			},
			ShardedTables: []SourceDBMetadata{},
			ColocatedSize: 10.0,
			ShardedSize:   0.0,
		},
		8: {
			ColocatedTables: []SourceDBMetadata{
				{
					ObjectName: "table1", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 100},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 50},
				},
			},
			ShardedTables: []SourceDBMetadata{},
			ColocatedSize: 10.0,
			ShardedSize:   0.0,
		},
		16: {
			ColocatedTables: []SourceDBMetadata{
				{
					ObjectName: "table1", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 100},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 50},
				},
			},
			ShardedTables: []SourceDBMetadata{},
			ColocatedSize: 10.0,
			ShardedSize:   0.0,
		},
	}

	// Run the function
	updatedRecommendation := shardingBasedOnOperations(sourceIndexMetadata, colocatedThroughput, recommendation)

	// expected is that the table should be removed from colocated and added to sharded as the ops requirement is high
	for _, rec := range updatedRecommendation {
		assert.Empty(t, rec.ShardedTables)
		assert.Len(t, rec.ColocatedTables, 1)
		assert.Equal(t, "table1", rec.ColocatedTables[0].ObjectName)
	}
}

// validate that the tables should be removed from colocated and added to sharded as the ops requirement is higher than
// supported
func TestShardingBasedOnOperations_CannotSupportOpsAndNeedsSharding(t *testing.T) {
	// Define test data
	sourceIndexMetadata := []SourceDBMetadata{
		{
			ObjectName: "table1", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
			ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 1000000},
			WritesPerSec: sql.NullInt64{Valid: true, Int64: 50000000},
		},
	}
	recommendation := map[int]IntermediateRecommendation{
		4: {
			ColocatedTables: []SourceDBMetadata{
				{
					ObjectName: "table1", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 1000000},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 50000000},
				},
			},
			ShardedTables: []SourceDBMetadata{},
			ColocatedSize: 10.0,
			ShardedSize:   0.0,
		},
		8: {
			ColocatedTables: []SourceDBMetadata{
				{
					ObjectName: "table1", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 1000000},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 50000000},
				},
			},
			ShardedTables: []SourceDBMetadata{},
			ColocatedSize: 10.0,
			ShardedSize:   0.0,
		},
		16: {
			ColocatedTables: []SourceDBMetadata{
				{
					ObjectName: "table1", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 1000000},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 50000000},
				},
			},
			ShardedTables: []SourceDBMetadata{},
			ColocatedSize: 10.0,
			ShardedSize:   0.0,
		},
	}

	// Run the function
	updatedRecommendation := shardingBasedOnOperations(sourceIndexMetadata, colocatedThroughput, recommendation)

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
// validate that the sharded table limits is not exceeded and failure reasoning should be empty
func TestCheckShardedTableLimit_WithinLimit(t *testing.T) {
	// Define test data
	var sourceIndexMetadata []SourceDBMetadata
	shardedLimits := []ExpDataShardedLimit{
		{
			numCores:              sql.NullFloat64{Float64: 16, Valid: true},
			maxSupportedNumTables: sql.NullInt64{Int64: 100, Valid: true},
		},
	}
	recommendation := map[int]IntermediateRecommendation{
		16: {
			ShardedTables: []SourceDBMetadata{
				{
					SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 100},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 50},
				},
				{
					SchemaName: "public", ObjectName: "table2", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 100},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 50},
				},
			},
			VCPUsPerInstance: 16,
			MemoryPerCore:    4,
		},
	}

	// Run the function
	updatedRecommendation := checkShardedTableLimit(sourceIndexMetadata, shardedLimits, recommendation)
	for _, rec := range updatedRecommendation {
		// failure reasoning should be empty
		assert.Empty(t, rec.FailureReasoning)
	}
}

// validate that the sharded table limits is exceeded and failure reasoning is generated as expected
func TestCheckShardedTableLimit_LimitExceeded(t *testing.T) {
	// Define test data
	var sourceIndexMetadata []SourceDBMetadata
	shardedLimits := []ExpDataShardedLimit{
		{
			numCores:              sql.NullFloat64{Float64: 16, Valid: true},
			maxSupportedNumTables: sql.NullInt64{Int64: 1, Valid: true},
		},
	}
	recommendation := map[int]IntermediateRecommendation{
		16: {
			ShardedTables: []SourceDBMetadata{
				{
					SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 100},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 50},
				},
				{
					SchemaName: "public", ObjectName: "table2", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 100},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 50},
				},
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
// validate that the throughput requirements are supported and no scaling is needed
func TestFindNumNodesNeededBasedOnThroughputRequirement_CanSupportOps(t *testing.T) {
	// Define test data
	sourceIndexMetadata := []SourceDBMetadata{
		{
			ObjectName: "table1", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
			ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 100},
			WritesPerSec: sql.NullInt64{Valid: true, Int64: 50},
		},
		{
			ObjectName: "table2", Size: sql.NullFloat64{Float64: 20.0, Valid: true},
			ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 200},
			WritesPerSec: sql.NullInt64{Valid: true, Int64: 100},
		},
	}

	shardedThroughput := []ExpDataThroughput{
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
			ShardedTables: []SourceDBMetadata{
				{
					ObjectName: "table1", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 100},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 50},
				},
			},
			OptimalSelectConnectionsPerNode: 50,
			OptimalInsertConnectionsPerNode: 25,
		},
	}

	// Run the function
	updatedRecommendation :=
		findNumNodesNeededBasedOnThroughputRequirement(sourceIndexMetadata, shardedThroughput, recommendation)

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

// validate that the throughput requirements cannot be supported by existing nodes and scaling is needed
func TestFindNumNodesNeededBasedOnThroughputRequirement_NeedMoreNodes(t *testing.T) {
	// Define test data
	var sourceIndexMetadata []SourceDBMetadata

	shardedLimits := []ExpDataThroughput{
		{
			numCores:                   sql.NullFloat64{Float64: 4},
			maxSupportedSelectsPerCore: sql.NullFloat64{Float64: 200},
			maxSupportedInsertsPerCore: sql.NullFloat64{Float64: 100},
		},
	}

	recommendation := map[int]IntermediateRecommendation{
		4: {
			ShardedTables: []SourceDBMetadata{
				{
					ObjectName: "table2", Size: sql.NullFloat64{Float64: 20.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 2000},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 5000},
				},
			},
			VCPUsPerInstance: 4,
			MemoryPerCore:    4,
		},
	}

	// Run the function
	updatedRecommendation :=
		findNumNodesNeededBasedOnThroughputRequirement(sourceIndexMetadata, shardedLimits, recommendation)

	// validate the expected number of nodes
	assert.Equal(t, updatedRecommendation[4].NumNodes, float64(15))
}

// validate that when colocated tables exist, 1 additional node and corresponding cores are added
func TestFindNumNodesNeededBasedOnThroughputRequirement_WithColocatedTablesAddsOneNode(t *testing.T) {
	// Define test data
	var sourceIndexMetadata []SourceDBMetadata
	shardedThroughput := []ExpDataThroughput{
		{
			numCores:                   sql.NullFloat64{Float64: 4},
			maxSupportedSelectsPerCore: sql.NullFloat64{Float64: 1000},
			maxSupportedInsertsPerCore: sql.NullFloat64{Float64: 500},
		},
	}

	recommendation := map[int]IntermediateRecommendation{
		4: {
			ColocatedTables: []SourceDBMetadata{
				{
					ObjectName: "colocated_table1", Size: sql.NullFloat64{Float64: 10.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 100},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 50},
				},
			},
			ShardedTables: []SourceDBMetadata{
				{
					ObjectName: "sharded_table1", Size: sql.NullFloat64{Float64: 20.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 8000},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 4000},
				},
			},
			VCPUsPerInstance: 4,
			MemoryPerCore:    4,
		},
	}

	// Run the function
	updatedRecommendation :=
		findNumNodesNeededBasedOnThroughputRequirement(sourceIndexMetadata, shardedThroughput, recommendation)

	// With colocated tables present, expect +1 node and +4 cores (VCPUsPerInstance)
	// Base calculation: (8000/1000 + 4000/500) = 16 cores needed -> 4 nodes needed
	// With colocated tables: 4 + 1 = 5 nodes, 16 + 4 = 20 cores needed
	// Minimum enforcement: max(5, 3) = 5 nodes, max(20, 12) = 20 cores
	expectedNumNodes := float64(5)
	expectedCoresNeeded := float64(20)

	// Check the results
	recommendationToVerify := updatedRecommendation[4]
	assert.Equal(t, expectedNumNodes, recommendationToVerify.NumNodes)
	assert.Equal(t, expectedCoresNeeded, recommendationToVerify.CoresNeeded)
}

// validate that when no colocated tables exist, no additional node/cores are added
func TestFindNumNodesNeededBasedOnThroughputRequirement_WithoutColocatedTablesNoExtraNode(t *testing.T) {
	// Define test data
	var sourceIndexMetadata []SourceDBMetadata
	shardedThroughput := []ExpDataThroughput{
		{
			numCores:                   sql.NullFloat64{Float64: 4},
			maxSupportedSelectsPerCore: sql.NullFloat64{Float64: 1000},
			maxSupportedInsertsPerCore: sql.NullFloat64{Float64: 500},
		},
	}

	recommendation := map[int]IntermediateRecommendation{
		4: {
			ColocatedTables: []SourceDBMetadata{}, // No colocated tables
			ShardedTables: []SourceDBMetadata{
				{
					ObjectName: "sharded_table1", Size: sql.NullFloat64{Float64: 20.0, Valid: true},
					ReadsPerSec:  sql.NullInt64{Valid: true, Int64: 8000},
					WritesPerSec: sql.NullInt64{Valid: true, Int64: 4000},
				},
			},
			VCPUsPerInstance: 4,
			MemoryPerCore:    4,
		},
	}

	// Run the function
	updatedRecommendation :=
		findNumNodesNeededBasedOnThroughputRequirement(sourceIndexMetadata, shardedThroughput, recommendation)

	// Without colocated tables, no extra node/cores should be added
	// Base calculation: (8000/1000 + 4000/500) = 16 cores needed -> 4 nodes needed
	// No colocated tables: just 4 nodes, cores: 16 cores
	expectedNumNodes := float64(4)
	expectedCoresNeeded := float64(16)

	// Check the results
	recommendationToVerify := updatedRecommendation[4]
	assert.Equal(t, expectedNumNodes, recommendationToVerify.NumNodes)
	assert.Equal(t, expectedCoresNeeded, recommendationToVerify.CoresNeeded)
}

/*
===== 	Test functions to test findNumNodesNeededBasedOnTabletsRequired function	=====
*/
// validate that the tablets requirements are supported and no scaling is needed
func TestFindNumNodesNeededBasedOnTabletsRequired_CanSupportTablets(t *testing.T) {
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
				{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 10, Valid: true}},
				{SchemaName: "public", ObjectName: "table2", Size: sql.NullFloat64{Float64: 60, Valid: true}},
			},
			VCPUsPerInstance: 4,
			NumNodes:         3,
			CoresNeeded:      10,
		},
	}

	// Run the function
	updatedRecommendation :=
		findNumNodesNeededBasedOnTabletsRequired(sourceIndexMetadata, shardedLimits, recommendation)

	// check if the num nodes in updated recommendation is same as before(3) meaning no scaling is required
	assert.Equal(t, float64(3), updatedRecommendation[4].NumNodes)
	assert.Equal(t, float64(10), updatedRecommendation[4].CoresNeeded)
}

// validate that the tablets cannot be supported by existing nodes and scaling is needed
func TestFindNumNodesNeededBasedOnTabletsRequired_NeedMoreNodes(t *testing.T) {
	// Define test data
	sourceIndexMetadata := []SourceDBMetadata{
		{
			SchemaName: "public", ObjectName: "index1", Size: sql.NullFloat64{Float64: 7, Valid: true},
			ParentTableName: sql.NullString{String: "public.table1"},
		},
		{
			SchemaName: "public", ObjectName: "index2", Size: sql.NullFloat64{Float64: 3, Valid: true},
			ParentTableName: sql.NullString{String: "public.table2"},
		},
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
				{SchemaName: "public", ObjectName: "table1", Size: sql.NullFloat64{Float64: 125, Valid: true}},
				{SchemaName: "public", ObjectName: "table2", Size: sql.NullFloat64{Float64: 60, Valid: true}},
			},
			VCPUsPerInstance: 4,
			NumNodes:         3,
			CoresNeeded:      10,
		},
	}

	// Run the function
	updatedRecommendation :=
		findNumNodesNeededBasedOnTabletsRequired(sourceIndexMetadata, shardedLimits, recommendation)

	// check if the num nodes and cores in updated recommendation has increased. Meaning scaling is required.
	assert.Equal(t, float64(6), updatedRecommendation[4].NumNodes)
	assert.Equal(t, float64(24), updatedRecommendation[4].CoresNeeded)
}

/*
===== 	Test functions to test pickBestRecommendationStrategy function	=====
*/
// Test VCPUsPerInstance logic - rec1 has fewer resultant cores, should be selected
func TestPickBestRecommendationStrategy_VCPULogic_Rec1FewerResultantCores(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 4,
		CoresNeeded:      12,
		NumNodes:         3, // resultant_cores = 4 * 3 = 12
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      16,
		NumNodes:         2, // resultant_cores = 8 * 2 = 16
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table3"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table4"}},
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	assert.Equal(t, rec1, selectedRec)
	assert.Contains(t, reasoning, "Selected colocated+sharded strategy with 4 VCPUs per instance (12.0 resultant cores)")
	assert.Contains(t, reasoning, "over all-sharded strategy with 8 VCPUs per instance (16.0 resultant cores)")
}

// Test VCPUsPerInstance logic - equal resultant cores, should select higher VCPUs
func TestPickBestRecommendationStrategy_VCPULogic_EqualResultantCores_SelectHigherVCPUs(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 4,
		CoresNeeded:      12,
		NumNodes:         3, // resultant_cores = 4 * 3 = 12
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 6,
		CoresNeeded:      12,
		NumNodes:         2, // resultant_cores = 6 * 2 = 12
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table3"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table4"}},
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	assert.Equal(t, rec2, selectedRec)
	assert.Contains(t, reasoning, "Selected all-sharded strategy with 6 VCPUs per instance (12.0 resultant cores)")
	assert.Contains(t, reasoning, "over colocated+sharded strategy with 4 VCPUs per instance (12.0 resultant cores)")
}

// Test VCPUsPerInstance logic - rec2 has fewer resultant cores, should be selected
func TestPickBestRecommendationStrategy_VCPULogic_Rec2FewerResultantCores(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 16,
		CoresNeeded:      20,
		NumNodes:         3, // resultant_cores = 16 * 3 = 48
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      15,
		NumNodes:         4, // resultant_cores = 8 * 4 = 32
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table3"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table4"}},
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	assert.Equal(t, rec2, selectedRec)
	assert.Contains(t, reasoning, "Selected all-sharded strategy with 8 VCPUs per instance (32.0 resultant cores)")
	assert.Contains(t, reasoning, "over colocated+sharded strategy with 16 VCPUs per instance (48.0 resultant cores)")
}

// Test VCPUsPerInstance logic - rec1 has fewer VCPUs but more resultant cores, should select rec2 due to fewer resultant cores
func TestPickBestRecommendationStrategy_VCPULogic_Rec1FewerVCPUsButMoreResultantCores(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 4,
		CoresNeeded:      20,
		NumNodes:         7, // resultant_cores = 4 * 7 = 28
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      16,
		NumNodes:         3, // resultant_cores = 8 * 3 = 24
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table3"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table4"}},
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	// Should select rec2 due to fewer resultant cores
	assert.Equal(t, rec2, selectedRec)
	assert.Contains(t, reasoning, "Selected all-sharded strategy with 8 VCPUs per instance (24.0 resultant cores)")
	assert.Contains(t, reasoning, "over colocated+sharded strategy with 4 VCPUs per instance (28.0 resultant cores)")
}

// Test VCPUsPerInstance logic - rec2 has fewer VCPUs but more resultant cores, should select rec1 due to fewer resultant cores
func TestPickBestRecommendationStrategy_VCPULogic_Rec2FewerVCPUsButMoreResultantCores(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 16,
		CoresNeeded:      15,
		NumNodes:         3, // resultant_cores = 16 * 3 = 48
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      25,
		NumNodes:         7, // resultant_cores = 8 * 7 = 56
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table3"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table4"}},
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	// Should select rec1 due to fewer resultant cores
	assert.Equal(t, rec1, selectedRec)
	assert.Contains(t, reasoning, "Selected colocated+sharded strategy with 16 VCPUs per instance (48.0 resultant cores)")
	assert.Contains(t, reasoning, "over all-sharded strategy with 8 VCPUs per instance (56.0 resultant cores)")
}

// Test same VCPUsPerInstance - should use node comparison logic
func TestPickBestRecommendationStrategy_SameVCPUs_UseNodeComparison_Rec1FewerNodes(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      16,
		NumNodes:         3,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      20,
		NumNodes:         5,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table3"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table4"}},
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	assert.Equal(t, rec1, selectedRec)
	assert.Contains(t, reasoning, "Selected colocated+sharded strategy requiring 3 nodes over all-sharded strategy requiring 5 nodes")
}

// Test same VCPUsPerInstance - should use node comparison logic
func TestPickBestRecommendationStrategy_SameVCPUs_UseNodeComparison_Rec2FewerNodes(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      20,
		NumNodes:         6,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      16,
		NumNodes:         4,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table3"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table4"}},
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	assert.Equal(t, rec2, selectedRec)
	assert.Contains(t, reasoning, "Selected all-sharded strategy requiring 4 nodes over colocated+sharded strategy requiring 6 nodes")
}

// Test same VCPUsPerInstance and same nodes - should prefer all-sharded (rec2)
func TestPickBestRecommendationStrategy_SameVCPUsAndNodes_PreferAllSharded(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      16,
		NumNodes:         4,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      16,
		NumNodes:         4,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table3"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table4"}},
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	assert.Equal(t, rec2, selectedRec)
	assert.Contains(t, reasoning, "Selected all-sharded strategy (same 4 nodes as colocated+sharded strategy, preferring all-sharded)")
}

// Test with indexes - verify object counts in reasoning
func TestPickBestRecommendationStrategy_WithIndexes_VerifyObjectCounts(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 4,
		CoresNeeded:      12,
		NumNodes:         4, // resultant_cores = 4 * 4 = 16
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}, {ObjectName: "table2"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table3"}},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      16,
		NumNodes:         3, // resultant_cores = 8 * 3 = 24
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table4"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table5"}, {ObjectName: "table6"}},
	}
	sourceIndexMetadata := []SourceDBMetadata{
		{ObjectName: "idx1", ParentTableName: sql.NullString{String: "public.table1", Valid: true}},
		{ObjectName: "idx2", ParentTableName: sql.NullString{String: "public.table3", Valid: true}},
		{ObjectName: "idx3", ParentTableName: sql.NullString{String: "public.table5", Valid: true}},
	}

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	assert.Equal(t, rec1, selectedRec)
	assert.Contains(t, reasoning, "Selected colocated+sharded strategy with 4 VCPUs per instance")
	// Should include object counts for non-selected recommendation (rec2)
	assert.Contains(t, reasoning, "Non-selected recommendation:")
	assert.Contains(t, reasoning, "1 colocated objects (1 tables, 0 indexes)")
	assert.Contains(t, reasoning, "2 sharded objects (2 tables, 0 indexes)")
}

// Test edge case - equal resultant cores, should prefer higher VCPUs
func TestPickBestRecommendationStrategy_EdgeCase_EqualResultantCores(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 4,
		CoresNeeded:      0,
		NumNodes:         3, // resultant_cores = 4 * 3 = 12
		ColocatedTables:  []SourceDBMetadata{},
		ShardedTables:    []SourceDBMetadata{},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 6,
		CoresNeeded:      0,
		NumNodes:         2, // resultant_cores = 6 * 2 = 12
		ColocatedTables:  []SourceDBMetadata{},
		ShardedTables:    []SourceDBMetadata{},
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	// Should select rec2 due to higher VCPUs with equal resultant cores
	assert.Equal(t, rec2, selectedRec)
	assert.Contains(t, reasoning, "Selected all-sharded strategy with 6 VCPUs per instance (12.0 resultant cores)")
}

// Test edge case - large values
func TestPickBestRecommendationStrategy_EdgeCase_LargeValues(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 64,
		CoresNeeded:      1000,
		NumNodes:         50, // resultant_cores = 64 * 50 = 3200
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 32,
		CoresNeeded:      800,
		NumNodes:         80, // resultant_cores = 32 * 80 = 2560
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table3"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table4"}},
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	// Should select rec2 due to fewer resultant cores
	assert.Equal(t, rec2, selectedRec)
	assert.Contains(t, reasoning, "Selected all-sharded strategy with 32 VCPUs per instance (2560.0 resultant cores)")
	assert.Contains(t, reasoning, "over colocated+sharded strategy with 64 VCPUs per instance (3200.0 resultant cores)")
}

// Test complex scenario with indexes affecting object counts
func TestPickBestRecommendationStrategy_ComplexScenario_WithMultipleIndexes(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      24,
		NumNodes:         5, // resultant_cores = 8 * 5 = 40
		ColocatedTables: []SourceDBMetadata{
			{ObjectName: "users", SchemaName: "public"},
			{ObjectName: "orders", SchemaName: "public"},
		},
		ShardedTables: []SourceDBMetadata{
			{ObjectName: "products", SchemaName: "public"},
		},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 16,
		CoresNeeded:      32,
		NumNodes:         4, // resultant_cores = 16 * 4 = 64
		ColocatedTables: []SourceDBMetadata{
			{ObjectName: "categories", SchemaName: "public"},
		},
		ShardedTables: []SourceDBMetadata{
			{ObjectName: "inventory", SchemaName: "public"},
			{ObjectName: "reviews", SchemaName: "public"},
		},
	}
	sourceIndexMetadata := []SourceDBMetadata{
		{ObjectName: "idx_users_email", ParentTableName: sql.NullString{String: "public.users", Valid: true}},
		{ObjectName: "idx_orders_user", ParentTableName: sql.NullString{String: "public.orders", Valid: true}},
		{ObjectName: "idx_products_name", ParentTableName: sql.NullString{String: "public.products", Valid: true}},
		{ObjectName: "idx_categories_name", ParentTableName: sql.NullString{String: "public.categories", Valid: true}},
		{ObjectName: "idx_inventory_product", ParentTableName: sql.NullString{String: "public.inventory", Valid: true}},
		{ObjectName: "idx_reviews_product", ParentTableName: sql.NullString{String: "public.reviews", Valid: true}},
	}

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	// Should select rec1 due to fewer resultant cores
	assert.Equal(t, rec1, selectedRec)
	assert.Contains(t, reasoning, "Selected colocated+sharded strategy with 8 VCPUs per instance (40.0 resultant cores)")
	assert.Contains(t, reasoning, "over all-sharded strategy with 16 VCPUs per instance (64.0 resultant cores)")

	// Verify object counts for non-selected recommendation (rec2)
	assert.Contains(t, reasoning, "Non-selected recommendation:")
	assert.Contains(t, reasoning, "2 colocated objects (1 tables, 1 indexes)")
	assert.Contains(t, reasoning, "4 sharded objects (2 tables, 2 indexes)")
}

// Test reasoning format when using resultant cores comparison
func TestPickBestRecommendationStrategy_ReasoningFormat_ResultantCoresComparison(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 4,
		CoresNeeded:      25,
		NumNodes:         7, // resultant_cores = 4 * 7 = 28
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      20,
		NumNodes:         5, // resultant_cores = 8 * 5 = 40
		ColocatedTables:  []SourceDBMetadata{},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	// Should select rec1 due to fewer resultant cores
	assert.Equal(t, rec1, selectedRec)
	assert.Contains(t, reasoning, "Selected colocated+sharded strategy with 4 VCPUs per instance (28.0 resultant cores)")
	assert.Contains(t, reasoning, "over all-sharded strategy with 8 VCPUs per instance (40.0 resultant cores)")
}

// Test when same VCPUsPerInstance - should fallback to node comparison
func TestPickBestRecommendationStrategy_SameVCPUs_FallbackToNodeComparison(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      25,
		NumNodes:         7, // same VCPUsPerInstance, so compare by nodes
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{},
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      20,
		NumNodes:         5, // fewer nodes
		ColocatedTables:  []SourceDBMetadata{},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	// Should select rec2 due to fewer nodes
	assert.Equal(t, rec2, selectedRec)
	assert.Contains(t, reasoning, "Selected all-sharded strategy requiring 5 nodes over colocated+sharded strategy requiring 7 nodes")
	assert.NotContains(t, reasoning, "resultant cores") // Should not mention resultant cores in node comparison fallback
}

// Test failure reasoning scenarios
func TestPickBestRecommendationStrategy_BothHaveFailureReasoning(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 4,
		CoresNeeded:      12,
		NumNodes:         3,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
		FailureReasoning: "Cannot support throughput requirements",
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      16,
		NumNodes:         2,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table3"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table4"}},
		FailureReasoning: "Exceeds table limits",
	}
	var sourceIndexMetadata []SourceDBMetadata

	_, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	// Should return specific error message when both have failure reasoning
	assert.Equal(t, "Both colocated+sharded and all-sharded strategies could not support the requirements. Unable to determine appropriate sizing recommendation.", reasoning)
}

func TestPickBestRecommendationStrategy_Rec1HasFailureRec2DoesNot(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 4,
		CoresNeeded:      12,
		NumNodes:         3,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
		FailureReasoning: "Cannot support throughput requirements",
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      16,
		NumNodes:         2,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table3"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table4"}},
		FailureReasoning: "", // No failure reasoning
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	// Should select rec2 since it has no failure reasoning
	assert.Equal(t, rec2, selectedRec)
	assert.Equal(t, "Selected all-sharded strategy as colocated+sharded strategy could not support the requirements: Cannot support throughput requirements", reasoning)
}

func TestPickBestRecommendationStrategy_Rec2HasFailureRec1DoesNot(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 4,
		CoresNeeded:      12,
		NumNodes:         3,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
		FailureReasoning: "", // No failure reasoning
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      16,
		NumNodes:         2,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table3"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table4"}},
		FailureReasoning: "Exceeds table limits",
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, _ := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	// Should select rec1 since it has no failure reasoning
	assert.Equal(t, rec1, selectedRec)
}

func TestPickBestRecommendationStrategy_BothHaveEmptyFailureReasoning(t *testing.T) {
	rec1 := IntermediateRecommendation{
		VCPUsPerInstance: 4,
		CoresNeeded:      12,
		NumNodes:         3,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table1"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table2"}},
		FailureReasoning: "", // Empty failure reasoning
	}
	rec2 := IntermediateRecommendation{
		VCPUsPerInstance: 8,
		CoresNeeded:      16,
		NumNodes:         2,
		ColocatedTables:  []SourceDBMetadata{{ObjectName: "table3"}},
		ShardedTables:    []SourceDBMetadata{{ObjectName: "table4"}},
		FailureReasoning: "", // Empty failure reasoning
	}
	var sourceIndexMetadata []SourceDBMetadata

	selectedRec, reasoning := pickBestRecommendationStrategy(rec1, rec2, sourceIndexMetadata)

	// Should apply normal comparison logic and select rec1 due to fewer resultant cores
	assert.Equal(t, rec1, selectedRec)
	assert.Contains(t, reasoning, "Selected colocated+sharded strategy with 4 VCPUs per instance (12.0 resultant cores)")
	assert.Contains(t, reasoning, "over all-sharded strategy with 8 VCPUs per instance (16.0 resultant cores)")
}

/*
===== 	Test functions to test pickBestRecommendation function	=====
*/
// validate if the recommendation with optimal nodes and cores is picked up
func TestPickBestRecommendation_PickOneWithOptimalNodesAndCores(t *testing.T) {
	recommendations := map[int]IntermediateRecommendation{
		4: {
			VCPUsPerInstance: 4,
			NumNodes:         10,
			CoresNeeded:      40,
			FailureReasoning: "",
		},
		8: {
			VCPUsPerInstance: 8,
			NumNodes:         3,
			CoresNeeded:      24,
			FailureReasoning: "",
		},
	}
	bestPick := pickBestRecommendation(recommendations)
	// validate the best recommendation which is 8 vcpus per instance is picked up
	assert.Equal(t, 8, bestPick.VCPUsPerInstance)
}

// validate if the recommendation with optimal nodes is picked up when some of the recommendation has failure reasoning
func TestPickBestRecommendation_PickOneWithOptimalNodesAndCoresWhenSomeHasFailures(t *testing.T) {
	recommendations := map[int]IntermediateRecommendation{
		4: {
			VCPUsPerInstance: 4,
			NumNodes:         10,
			CoresNeeded:      40,
			FailureReasoning: "has some failure",
		},
		8: {
			VCPUsPerInstance: 8,
			NumNodes:         3,
			CoresNeeded:      24,
			FailureReasoning: "has some failure as well",
		},
		16: {
			VCPUsPerInstance: 16,
			NumNodes:         3,
			CoresNeeded:      48,
			FailureReasoning: "",
		},
	}
	bestPick := pickBestRecommendation(recommendations)
	// validate the best recommendation which is 16 vcpus per instance is picked up
	assert.Equal(t, 16, bestPick.VCPUsPerInstance)
}

// validate if the recommendation with optimal nodes and cores is picked up for showing failure reasoning
func TestPickBestRecommendation_PickLastMaxCoreRecommendationWhenNoneCanSupport(t *testing.T) {
	recommendations := map[int]IntermediateRecommendation{
		4: {
			VCPUsPerInstance: 4,
			NumNodes:         10,
			CoresNeeded:      12,
			FailureReasoning: "has some failure reasoning",
		},
		8: {
			VCPUsPerInstance: 8,
			NumNodes:         3,
			CoresNeeded:      24,
			FailureReasoning: "has some failure reasoning",
		},
		16: {
			VCPUsPerInstance: 16,
			NumNodes:         3,
			CoresNeeded:      48,
			FailureReasoning: "has some failure reasoning",
		},
	}
	bestPick := pickBestRecommendation(recommendations)
	// validate the best recommendation which is 16 vcpus per instance is picked up
	assert.Equal(t, 16, bestPick.VCPUsPerInstance)
}

// validate if the recommendation with min required cores is used.
func TestPickBestRecommendation_PickRecommendationWithMinCoresRequired(t *testing.T) {
	recommendations := map[int]IntermediateRecommendation{
		4: {
			VCPUsPerInstance: 4,
			NumNodes:         83,
			CoresNeeded:      329,
		},
		8: {
			VCPUsPerInstance: 8,
			NumNodes:         37,
			CoresNeeded:      296,
		},
		16: {
			VCPUsPerInstance: 16,
			NumNodes:         19,
			CoresNeeded:      290,
		},
	}
	bestPick := pickBestRecommendation(recommendations)
	// validate the best recommendation which is 16 vcpus per instance is picked up
	assert.Equal(t, 16, bestPick.VCPUsPerInstance)
}

// validate if the recommendation with higher vCPU is selected if required cores are same.
func TestPickBestRecommendation_PickHigherVCPURecommendationWhenSameCoresRequired(t *testing.T) {
	recommendations := map[int]IntermediateRecommendation{
		4: {
			VCPUsPerInstance: 4,
			NumNodes:         12,
			CoresNeeded:      48,
		},
		8: {
			VCPUsPerInstance: 8,
			NumNodes:         6,
			CoresNeeded:      48,
		},
		16: {
			VCPUsPerInstance: 16,
			NumNodes:         3,
			CoresNeeded:      48,
		},
	}
	bestPick := pickBestRecommendation(recommendations)
	// validate the best recommendation which is 16 vcpus per instance is picked up
	assert.Equal(t, 16, bestPick.VCPUsPerInstance)
}

/*
===== 	Test functions to test calculateTimeTakenForImport function	=====
*/
// validate the formula to calculate the import time for colocated table without index
func TestCalculateTimeTakenForImport_ValidateImportTimeTableWithoutIndex_Colocated(t *testing.T) {
	// Define test data
	colocatedTables := []SourceDBMetadata{
		{
			ObjectName: "table0", SchemaName: "public",
			ColumnCount: sql.NullInt64{Int64: 5, Valid: true},
			Size:        sql.NullFloat64{Float64: 23.0, Valid: true},
			RowCount:    sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	var sourceIndexMetadata []SourceDBMetadata
	colocatedLoadTimes := []ExpDataLoadTime{
		{
			csvSizeGB: sql.NullFloat64{Float64: 19}, migrationTimeSecs: sql.NullFloat64{Float64: 1134},
			rowCount: sql.NullFloat64{Float64: 100000},
		},
		{
			csvSizeGB: sql.NullFloat64{Float64: 29}, migrationTimeSecs: sql.NullFloat64{Float64: 1657},
			rowCount: sql.NullFloat64{Float64: 100000},
		},
	}
	var indexImpacts []ExpDataLoadTimeIndexImpact
	columnImpacts := []ExpDataLoadTimeColumnsImpact{ // doesn't have any impact because multiplication factor is 1
		{
			numColumns:                    sql.NullInt64{Int64: 5, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 1.0},
			multiplicationFactorSharded:   sql.NullFloat64{Float64: 1.0},
		},
	}

	// Call the function
	estimatedTimeWithAllIndexes, estimatedTimeWithoutRedundantIndexes, err := calculateTimeTakenForImport(colocatedTables,
		sourceIndexMetadata, sourceIndexMetadata, colocatedLoadTimes, indexImpacts, columnImpacts, COLOCATED, 1.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Define expected results
	// Calculated as table0: 1 * ((1134 * 23) / 19) / 60
	expectedTime := 23.0
	if estimatedTimeWithAllIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() with all indexes = (%v), want (%v)", estimatedTimeWithAllIndexes, expectedTime)
	}
	if estimatedTimeWithoutRedundantIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() without redundant indexes = (%v), want (%v)", estimatedTimeWithoutRedundantIndexes, expectedTime)
	}

}

// validate the formula to calculate the import time for colocated table with one index
func TestCalculateTimeTakenForImport_ValidateImportTimeTableWithOneIndex_Colocated(t *testing.T) {
	// Define test data
	colocatedTables := []SourceDBMetadata{
		{
			ObjectName: "table0", Size: sql.NullFloat64{Float64: 23.0, Valid: true},
			SchemaName: "public", ColumnCount: sql.NullInt64{Int64: 5, Valid: true},
			RowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	sourceIndexMetadata := []SourceDBMetadata{
		{ObjectName: "table0_idx1", ParentTableName: sql.NullString{Valid: true, String: "public.table0"}},
	}
	colocatedLoadTimes := []ExpDataLoadTime{
		{
			csvSizeGB: sql.NullFloat64{Float64: 19}, migrationTimeSecs: sql.NullFloat64{Float64: 1461},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
		{
			csvSizeGB: sql.NullFloat64{Float64: 29}, migrationTimeSecs: sql.NullFloat64{Float64: 2009},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}

	indexImpacts := []ExpDataLoadTimeIndexImpact{
		{
			numIndexes:                    sql.NullFloat64{Float64: 1, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 1.77777},
		},
	}
	columnImpacts := []ExpDataLoadTimeColumnsImpact{ // doesn't have any impact because multiplication factor is 1
		{
			numColumns:                    sql.NullInt64{Int64: 5, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 1.0},
			multiplicationFactorSharded:   sql.NullFloat64{Float64: 1.0},
		},
	}

	// Call the function
	estimatedTimeWithAllIndexes, estimatedTimeWithoutRedundantIndexes, err := calculateTimeTakenForImport(colocatedTables,
		sourceIndexMetadata, sourceIndexMetadata, colocatedLoadTimes, indexImpacts, columnImpacts, COLOCATED, 1.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Define expected results
	// Calculated as table0: 1.77777 * ((1461 * 23) / 19) / 60
	expectedTime := 53.0 // double the time required when there are no indexes.
	if estimatedTimeWithAllIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() with all indexes = (%v), want (%v)", estimatedTimeWithAllIndexes, expectedTime)
	}
	if estimatedTimeWithoutRedundantIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() without redundant indexes = (%v), want (%v)", estimatedTimeWithoutRedundantIndexes, expectedTime)
	}

}

// validate the formula to calculate the import time for colocated table with 5 indexes
func TestCalculateTimeTakenForImport_ValidateImportTimeTableWithFiveIndexes_Colocated(t *testing.T) {
	// Define test data
	colocatedTables := []SourceDBMetadata{
		{
			ObjectName: "table0", SchemaName: "public", Size: sql.NullFloat64{Float64: 23.0, Valid: true},
			ColumnCount: sql.NullInt64{Int64: 5, Valid: true}, RowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	sourceIndexMetadata := []SourceDBMetadata{
		{ObjectName: "table0_idx1", ParentTableName: sql.NullString{Valid: true, String: "public.table0"}},
		{ObjectName: "table0_idx2", ParentTableName: sql.NullString{Valid: true, String: "public.table0"}},
		{ObjectName: "table0_idx3", ParentTableName: sql.NullString{Valid: true, String: "public.table0"}},
		{ObjectName: "table0_idx4", ParentTableName: sql.NullString{Valid: true, String: "public.table0"}},
		{ObjectName: "table0_idx5", ParentTableName: sql.NullString{Valid: true, String: "public.table0"}},
	}
	colocatedLoadTimes := []ExpDataLoadTime{
		{
			csvSizeGB: sql.NullFloat64{Float64: 19}, migrationTimeSecs: sql.NullFloat64{Float64: 1461},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
		{
			csvSizeGB: sql.NullFloat64{Float64: 29}, migrationTimeSecs: sql.NullFloat64{Float64: 2009},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	//TODO: modify index impact with actual colocated data when it is available and adjust the calculations
	indexImpacts := []ExpDataLoadTimeIndexImpact{
		{numIndexes: sql.NullFloat64{Float64: 1}, multiplicationFactorColocated: sql.NullFloat64{Float64: 1.77777}},
		{numIndexes: sql.NullFloat64{Float64: 5}, multiplicationFactorColocated: sql.NullFloat64{Float64: 4.66666}},
	}
	columnsImpact := []ExpDataLoadTimeColumnsImpact{ // doesn't have any impact because multiplication factor is 1
		{
			numColumns:                    sql.NullInt64{Int64: 5, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 1.0},
			multiplicationFactorSharded:   sql.NullFloat64{Float64: 1.0},
		},
	}
	// Call the function
	estimatedTimeWithAllIndexes, estimatedTimeWithoutRedundantIndexes, err := calculateTimeTakenForImport(colocatedTables,
		sourceIndexMetadata, sourceIndexMetadata, colocatedLoadTimes, indexImpacts, columnsImpact, COLOCATED, 1.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Define expected results
	// Calculated as table0: 4.66666 * ((1461 * 23) / 19) / 60
	expectedTime := 138.0
	if estimatedTimeWithAllIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() with all indexes = (%v), want (%v)", estimatedTimeWithAllIndexes, expectedTime)
	}
	if estimatedTimeWithoutRedundantIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() without redundant indexes = (%v), want (%v)", estimatedTimeWithoutRedundantIndexes, expectedTime)
	}

}

// validate the formula to calculate the import time for sharded table without index
func TestCalculateTimeTakenForImport_ValidateImportTimeTableWithoutIndex_Sharded(t *testing.T) {
	// Define test data
	shardedTables := []SourceDBMetadata{
		{
			ObjectName: "table0", SchemaName: "public", Size: sql.NullFloat64{Float64: 23.0, Valid: true},
			ColumnCount: sql.NullInt64{Int64: 5, Valid: true}, RowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	var sourceIndexMetadata []SourceDBMetadata
	shardedLoadTimes := []ExpDataLoadTime{
		{
			csvSizeGB: sql.NullFloat64{Float64: 19}, migrationTimeSecs: sql.NullFloat64{Float64: 1134},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
		{
			csvSizeGB: sql.NullFloat64{Float64: 29}, migrationTimeSecs: sql.NullFloat64{Float64: 1657},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	var indexImpacts []ExpDataLoadTimeIndexImpact
	columnsImpact := []ExpDataLoadTimeColumnsImpact{ // doesn't have any impact because multiplication factor is 1
		{
			numColumns:                    sql.NullInt64{Int64: 5, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 1.0},
			multiplicationFactorSharded:   sql.NullFloat64{Float64: 1.0},
		},
	}
	// Call the function
	estimatedTimeWithAllIndexes, estimatedTimeWithoutRedundantIndexes, err := calculateTimeTakenForImport(shardedTables, sourceIndexMetadata,
		sourceIndexMetadata, shardedLoadTimes, indexImpacts, columnsImpact, SHARDED, 1.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Define expected results
	// Calculated as table0: 1 * ((1134 * 23) / 19) / 60
	expectedTime := 23.0
	if estimatedTimeWithAllIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() with all indexes = (%v), want (%v)", estimatedTimeWithAllIndexes, expectedTime)
	}
	if estimatedTimeWithoutRedundantIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() without redundant indexes = (%v), want (%v)", estimatedTimeWithoutRedundantIndexes, expectedTime)
	}

}

// validate the formula to calculate the import time for sharded table with one index
func TestCalculateTimeTakenForImport_ValidateImportTimeTableWithOneIndex_Sharded(t *testing.T) {
	// Define test data
	shardedTables := []SourceDBMetadata{
		{
			ObjectName: "table0", SchemaName: "public", Size: sql.NullFloat64{Float64: 23.0, Valid: true},
			ColumnCount: sql.NullInt64{Int64: 5, Valid: true}, RowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	sourceIndexMetadata := []SourceDBMetadata{
		{ObjectName: "table0_idx1", ParentTableName: sql.NullString{Valid: true, String: "public.table0"}},
	}
	shardedLoadTimes := []ExpDataLoadTime{
		{
			csvSizeGB: sql.NullFloat64{Float64: 19}, migrationTimeSecs: sql.NullFloat64{Float64: 1134},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
		{
			csvSizeGB: sql.NullFloat64{Float64: 29}, migrationTimeSecs: sql.NullFloat64{Float64: 1657},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	indexImpacts := []ExpDataLoadTimeIndexImpact{
		{numIndexes: sql.NullFloat64{Float64: 1}, multiplicationFactorSharded: sql.NullFloat64{Float64: 1.76}},
	}
	columnsImpact := []ExpDataLoadTimeColumnsImpact{ // doesn't have any impact because multiplication factor is 1
		{
			numColumns:                    sql.NullInt64{Int64: 5, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 1.0},
			multiplicationFactorSharded:   sql.NullFloat64{Float64: 1.0},
		},
	}
	// Call the function
	estimatedTimeWithAllIndexes, estimatedTimeWithoutRedundantIndexes, err :=
		calculateTimeTakenForImport(shardedTables, sourceIndexMetadata, sourceIndexMetadata, shardedLoadTimes,
			indexImpacts, columnsImpact, SHARDED, 1.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Define expected results
	// Calculated as table0: 1.76 * ((1134 * 23) / 19) / 60
	expectedTime := 41.0 // double the time required when there are no indexes.

	if estimatedTimeWithAllIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() with all indexes = (%v), want (%v)", estimatedTimeWithAllIndexes, expectedTime)
	}
	if estimatedTimeWithoutRedundantIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() without redundant indexes = (%v), want (%v)", estimatedTimeWithoutRedundantIndexes, expectedTime)
	}

}

// validate the formula to calculate the import time for sharded table with 5 indexes
func TestCalculateTimeTakenForImport_ValidateImportTimeTableWithFiveIndexes_Sharded(t *testing.T) {
	// Define test data
	shardedTables := []SourceDBMetadata{
		{
			ObjectName: "table0", SchemaName: "public", Size: sql.NullFloat64{Float64: 23.0, Valid: true},
			ColumnCount: sql.NullInt64{Int64: 5, Valid: true}, RowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	sourceIndexMetadata := []SourceDBMetadata{
		{ObjectName: "table0_idx1", ParentTableName: sql.NullString{Valid: true, String: "public.table0"}},
		{ObjectName: "table0_idx2", ParentTableName: sql.NullString{Valid: true, String: "public.table0"}},
		{ObjectName: "table0_idx3", ParentTableName: sql.NullString{Valid: true, String: "public.table0"}},
		{ObjectName: "table0_idx4", ParentTableName: sql.NullString{Valid: true, String: "public.table0"}},
		{ObjectName: "table0_idx5", ParentTableName: sql.NullString{Valid: true, String: "public.table0"}},
	}
	shardedLoadTimes := []ExpDataLoadTime{
		{
			csvSizeGB: sql.NullFloat64{Float64: 19}, migrationTimeSecs: sql.NullFloat64{Float64: 1134},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
		{
			csvSizeGB: sql.NullFloat64{Float64: 29}, migrationTimeSecs: sql.NullFloat64{Float64: 1657},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}

	indexImpacts := []ExpDataLoadTimeIndexImpact{
		{numIndexes: sql.NullFloat64{Float64: 1}, multiplicationFactorSharded: sql.NullFloat64{Float64: 1.76}},
		{numIndexes: sql.NullFloat64{Float64: 5}, multiplicationFactorSharded: sql.NullFloat64{Float64: 4.6}},
	}
	columnsImpact := []ExpDataLoadTimeColumnsImpact{ // doesn't have any impact because multiplication factor is 1
		{
			numColumns:                    sql.NullInt64{Int64: 5, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 1.0},
			multiplicationFactorSharded:   sql.NullFloat64{Float64: 1.0},
		},
	}
	// Call the function
	estimatedTimeWithAllIndexes, estimatedTimeWithoutRedundantIndexes, err :=
		calculateTimeTakenForImport(shardedTables, sourceIndexMetadata, sourceIndexMetadata, shardedLoadTimes,
			indexImpacts, columnsImpact, SHARDED, 1.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Define expected results
	// Calculated as table0: 4.6 * ((1134 * 23) / 19) / 60
	expectedTime := 106.0

	if estimatedTimeWithAllIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() with all indexes = (%v), want (%v)", estimatedTimeWithAllIndexes, expectedTime)
	}
	if estimatedTimeWithoutRedundantIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() without redundant indexes = (%v), want (%v)", estimatedTimeWithoutRedundantIndexes, expectedTime)
	}

}

// validate the formula to calculate the import time for colocated table without index having 5 columns
func TestCalculateTimeTakenForImport_ValidateImportTimeTableWithoutIndex5Columns_Colocated(t *testing.T) {
	// Define test data
	colocatedTables := []SourceDBMetadata{
		{
			ObjectName: "table0", SchemaName: "public", ColumnCount: sql.NullInt64{Int64: 5, Valid: true},
			Size: sql.NullFloat64{Float64: 23.0, Valid: true}, RowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	var sourceIndexMetadata []SourceDBMetadata
	colocatedLoadTimes := []ExpDataLoadTime{
		{
			csvSizeGB: sql.NullFloat64{Float64: 19}, migrationTimeSecs: sql.NullFloat64{Float64: 1134},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
		{
			csvSizeGB: sql.NullFloat64{Float64: 29}, migrationTimeSecs: sql.NullFloat64{Float64: 1657},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	var indexImpacts []ExpDataLoadTimeIndexImpact // doesn't have any impact as there are no indexes
	columnImpacts := []ExpDataLoadTimeColumnsImpact{
		{
			numColumns:                    sql.NullInt64{Int64: 5, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 1.0},
			multiplicationFactorSharded:   sql.NullFloat64{Float64: 1.0},
		},
	}

	// Call the function
	estimatedTimeWithAllIndexes, estimatedTimeWithoutRedundantIndexes, err := calculateTimeTakenForImport(colocatedTables,
		sourceIndexMetadata, sourceIndexMetadata, colocatedLoadTimes, indexImpacts, columnImpacts, COLOCATED, 1.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Define expected results
	// Calculated as table0: 1 * ((1134 * 23) / 19) / 60
	expectedTime := 23.0

	if estimatedTimeWithAllIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() with all indexes = (%v), want (%v)", estimatedTimeWithAllIndexes, expectedTime)
	}
	if estimatedTimeWithoutRedundantIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() without redundant indexes = (%v), want (%v)", estimatedTimeWithoutRedundantIndexes, expectedTime)
	}

}

// validate the formula to calculate the import time for colocated table without index having 40 columns
func TestCalculateTimeTakenForImport_ValidateImportTimeTableWithoutIndex40Columns_Colocated(t *testing.T) {
	// Define test data
	colocatedTables := []SourceDBMetadata{
		{
			ObjectName: "table0", SchemaName: "public", ColumnCount: sql.NullInt64{Int64: 40, Valid: true},
			Size: sql.NullFloat64{Float64: 23.0, Valid: true}, RowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	var sourceIndexMetadata []SourceDBMetadata
	colocatedLoadTimes := []ExpDataLoadTime{
		{
			csvSizeGB: sql.NullFloat64{Float64: 19}, migrationTimeSecs: sql.NullFloat64{Float64: 1134},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
		{
			csvSizeGB: sql.NullFloat64{Float64: 29}, migrationTimeSecs: sql.NullFloat64{Float64: 1657},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	var indexImpacts []ExpDataLoadTimeIndexImpact // doesn't have any impact as there are no indexes
	columnImpacts := []ExpDataLoadTimeColumnsImpact{
		{
			numColumns:                    sql.NullInt64{Int64: 5, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 1.0},
			multiplicationFactorSharded:   sql.NullFloat64{Float64: 1.0},
		},
		{
			numColumns:                    sql.NullInt64{Int64: 40, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 1.57},
			multiplicationFactorSharded:   sql.NullFloat64{Float64: 1.45},
		},
	}

	// Call the function
	estimatedTimeWithAllIndexes, estimatedTimeWithoutRedundantIndexes, err := calculateTimeTakenForImport(colocatedTables,
		sourceIndexMetadata, sourceIndexMetadata, colocatedLoadTimes, indexImpacts, columnImpacts, COLOCATED, 1.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Define expected results
	// Calculated as table0: 1.57 * ((1134 * 23) / 19) / 60
	expectedTime := 36.0

	if estimatedTimeWithAllIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() with all indexes = (%v), want (%v)", estimatedTimeWithAllIndexes, expectedTime)
	}
	if estimatedTimeWithoutRedundantIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() without redundant indexes = (%v), want (%v)", estimatedTimeWithoutRedundantIndexes, expectedTime)
	}

}

// validate the formula to calculate the import time for colocated table without index having 100 columns
func TestCalculateTimeTakenForImport_ValidateImportTimeTableWithoutIndex100Columns_Colocated(t *testing.T) {
	// Define test data
	colocatedTables := []SourceDBMetadata{
		{
			ObjectName: "table0", SchemaName: "public", ColumnCount: sql.NullInt64{Int64: 100, Valid: true},
			Size: sql.NullFloat64{Float64: 23.0, Valid: true}, RowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	var sourceIndexMetadata []SourceDBMetadata
	colocatedLoadTimes := []ExpDataLoadTime{
		{
			csvSizeGB: sql.NullFloat64{Float64: 19}, migrationTimeSecs: sql.NullFloat64{Float64: 1134},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
		{
			csvSizeGB: sql.NullFloat64{Float64: 29}, migrationTimeSecs: sql.NullFloat64{Float64: 1657},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	var indexImpacts []ExpDataLoadTimeIndexImpact // doesn't have any impact as there are no indexes
	columnImpacts := []ExpDataLoadTimeColumnsImpact{
		{
			numColumns:                    sql.NullInt64{Int64: 80, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 2.24},
			multiplicationFactorSharded:   sql.NullFloat64{Float64: 2.3},
		},
		{
			numColumns:                    sql.NullInt64{Int64: 160, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 4.13},
			multiplicationFactorSharded:   sql.NullFloat64{Float64: 4.36},
		},
	}

	// Call the function
	estimatedTimeWithAllIndexes, estimatedTimeWithoutRedundantIndexes, err := calculateTimeTakenForImport(colocatedTables,
		sourceIndexMetadata, sourceIndexMetadata, colocatedLoadTimes, indexImpacts, columnImpacts, COLOCATED, 1.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Define expected results
	// multiplication factor: high MF of 160 columns ==> 4.13
	// Calculated as table0: 4.13 * ((1134 * 23) / 19) / 60
	expectedTime := 95.0

	if estimatedTimeWithAllIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() with all indexes = (%v), want (%v)", estimatedTimeWithAllIndexes, expectedTime)
	}
	if estimatedTimeWithoutRedundantIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() without redundant indexes = (%v), want (%v)", estimatedTimeWithoutRedundantIndexes, expectedTime)
	}

}

// validate the formula to calculate the import time for colocated table without index having 250 columns
func TestCalculateTimeTakenForImport_ValidateImportTimeTableWithoutIndex250Columns_Colocated(t *testing.T) {
	// Define test data
	colocatedTables := []SourceDBMetadata{
		{
			ObjectName: "table0", SchemaName: "public", ColumnCount: sql.NullInt64{Int64: 250, Valid: true},
			Size: sql.NullFloat64{Float64: 23.0, Valid: true}, RowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	var sourceIndexMetadata []SourceDBMetadata
	colocatedLoadTimes := []ExpDataLoadTime{
		{
			csvSizeGB: sql.NullFloat64{Float64: 19}, migrationTimeSecs: sql.NullFloat64{Float64: 1134},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
		{
			csvSizeGB: sql.NullFloat64{Float64: 29}, migrationTimeSecs: sql.NullFloat64{Float64: 1657},
			rowCount: sql.NullFloat64{Float64: 100000, Valid: true},
		},
	}
	var indexImpacts []ExpDataLoadTimeIndexImpact // doesn't have any impact as there are no indexes
	columnImpacts := []ExpDataLoadTimeColumnsImpact{
		{
			numColumns:                    sql.NullInt64{Int64: 80, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 2.24},
			multiplicationFactorSharded:   sql.NullFloat64{Float64: 2.3},
		},
		{
			numColumns:                    sql.NullInt64{Int64: 160, Valid: true},
			multiplicationFactorColocated: sql.NullFloat64{Float64: 4.13},
			multiplicationFactorSharded:   sql.NullFloat64{Float64: 4.36},
		},
	}

	// Call the function
	estimatedTimeWithAllIndexes, estimatedTimeWithoutRedundantIndexes, err := calculateTimeTakenForImport(colocatedTables,
		sourceIndexMetadata, sourceIndexMetadata, colocatedLoadTimes, indexImpacts, columnImpacts, COLOCATED, 1.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Define expected results
	// multiplication factor: closest MF of 160 columns ==> (4.13/160 : x/250) ==> x = 6.45
	// Calculated as table0: 6.45 * ((1134 * 23) / 19) / 60
	expectedTime := 148.0

	if estimatedTimeWithAllIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() with all indexes = (%v), want (%v)", estimatedTimeWithAllIndexes, expectedTime)
	}
	if estimatedTimeWithoutRedundantIndexes != expectedTime {
		t.Errorf("calculateTimeTakenForImport() without redundant indexes = (%v), want (%v)", estimatedTimeWithoutRedundantIndexes, expectedTime)
	}

}

/*
===== 	Test functions to test getReasoning function	=====
*/
// validate reasoning when there are no objects(empty colocated and sharded objects)
func TestGetReasoning_NoObjects(t *testing.T) {
	recommendation := IntermediateRecommendation{VCPUsPerInstance: 4, MemoryPerCore: 16}
	var shardedObjects []SourceDBMetadata
	var colocatedObjects []SourceDBMetadata
	expected := "Recommended instance type with 4 vCPU and 64 GiB memory could fit  Non leaf partition tables/indexes and unsupported tables/indexes were not considered."

	result := getReasoning(recommendation, shardedObjects, 0, colocatedObjects, 0)
	if result != expected {
		t.Errorf("Expected %q but got %q", expected, result)
	}
}

// validate reasoning when there are only colocated objects
func TestGetReasoning_OnlyColocatedObjects(t *testing.T) {
	recommendation := IntermediateRecommendation{VCPUsPerInstance: 8, MemoryPerCore: 8}
	var shardedObjects []SourceDBMetadata
	colocatedObjects := []SourceDBMetadata{
		{Size: sql.NullFloat64{Valid: true, Float64: 50.0}, ReadsPerSec: sql.NullInt64{Valid: true, Int64: 1000}, WritesPerSec: sql.NullInt64{Valid: true, Int64: 500}},
		{Size: sql.NullFloat64{Valid: true, Float64: 30.0}, ReadsPerSec: sql.NullInt64{Valid: true, Int64: 2000}, WritesPerSec: sql.NullInt64{Valid: true, Int64: 1500}},
	}
	expected := "Recommended instance type with 8 vCPU and 64 GiB memory could fit 2 objects (2 tables/materialized views and 0 " +
		"explicit/implicit indexes) with 80.00 GB size and throughput requirement of 3000 reads/sec and " +
		"2000 writes/sec as colocated. Non leaf partition tables/indexes and unsupported tables/indexes were not considered."

	result := getReasoning(recommendation, shardedObjects, 0, colocatedObjects, 0)
	if result != expected {
		t.Errorf("Expected %q but got %q", expected, result)
	}
}

// validate reasoning when there are only sharded objects
func TestGetReasoning_OnlyShardedObjects(t *testing.T) {
	recommendation := IntermediateRecommendation{VCPUsPerInstance: 16, MemoryPerCore: 4}
	shardedObjects := []SourceDBMetadata{
		{Size: sql.NullFloat64{Valid: true, Float64: 100.0}, ReadsPerSec: sql.NullInt64{Valid: true, Int64: 4000}, WritesPerSec: sql.NullInt64{Valid: true, Int64: 3000}},
		{Size: sql.NullFloat64{Valid: true, Float64: 200.0}, ReadsPerSec: sql.NullInt64{Valid: true, Int64: 5000}, WritesPerSec: sql.NullInt64{Valid: true, Int64: 4000}},
	}
	var colocatedObjects []SourceDBMetadata
	expected := "Recommended instance type with 16 vCPU and 64 GiB memory could fit 2 objects (2 tables/materialized views and 0 " +
		"explicit/implicit indexes) with 300.00 GB size and throughput requirement of 9000 reads/sec and " +
		"7000 writes/sec as sharded. Non leaf partition tables/indexes and unsupported tables/indexes were not considered."

	result := getReasoning(recommendation, shardedObjects, 0, colocatedObjects, 0)
	if result != expected {
		t.Errorf("Expected %q but got %q", expected, result)
	}
}

// validate reasoning when there are colocated and sharded objects
func TestGetReasoning_ColocatedAndShardedObjects(t *testing.T) {
	recommendation := IntermediateRecommendation{VCPUsPerInstance: 32, MemoryPerCore: 2}
	shardedObjects := []SourceDBMetadata{
		{Size: sql.NullFloat64{Valid: true, Float64: 150.0}, ReadsPerSec: sql.NullInt64{Valid: true, Int64: 7000}, WritesPerSec: sql.NullInt64{Valid: true, Int64: 6000}},
	}
	colocatedObjects := []SourceDBMetadata{
		{Size: sql.NullFloat64{Valid: true, Float64: 70.0}, ReadsPerSec: sql.NullInt64{Valid: true, Int64: 3000}, WritesPerSec: sql.NullInt64{Valid: true, Int64: 2000}},
		{Size: sql.NullFloat64{Valid: true, Float64: 50.0}, ReadsPerSec: sql.NullInt64{Valid: true, Int64: 2000}, WritesPerSec: sql.NullInt64{Valid: true, Int64: 1000}},
	}
	expected := "Recommended instance type with 32 vCPU and 64 GiB memory could fit 2 objects (2 tables/materialized views and 0 " +
		"explicit/implicit indexes) with 120.00 GB size and throughput requirement of 5000 reads/sec and " +
		"3000 writes/sec as colocated. Rest 1 objects (1 tables/materialized views and 0 explicit/implicit indexes) with 150.00 GB " +
		"size and throughput requirement of 7000 reads/sec and 6000 writes/sec need to be migrated as range " +
		"partitioned tables. Non leaf partition tables/indexes and unsupported tables/indexes were not considered."

	result := getReasoning(recommendation, shardedObjects, 0, colocatedObjects, 0)
	if result != expected {
		t.Errorf("Expected %q but got %q", expected, result)
	}
}

// validate reasoning when there are colocated and sharded objects with indexes
func TestGetReasoning_Indexes(t *testing.T) {
	recommendation := IntermediateRecommendation{VCPUsPerInstance: 4, MemoryPerCore: 16}
	shardedObjects := []SourceDBMetadata{
		{Size: sql.NullFloat64{Valid: true, Float64: 200.0}, ReadsPerSec: sql.NullInt64{Valid: true, Int64: 6000}, WritesPerSec: sql.NullInt64{Valid: true, Int64: 4000}},
	}
	colocatedObjects := []SourceDBMetadata{
		{Size: sql.NullFloat64{Valid: true, Float64: 100.0}, ReadsPerSec: sql.NullInt64{Valid: true, Int64: 3000}, WritesPerSec: sql.NullInt64{Valid: true, Int64: 2000}},
	}
	expected := "Recommended instance type with 4 vCPU and 64 GiB memory could fit 1 objects (0 tables/materialized views and " +
		"1 explicit/implicit indexes) with 100.00 GB size and throughput requirement of 3000 reads/sec and " +
		"2000 writes/sec as colocated. Rest 1 objects (0 tables/materialized views and 1 explicit/implicit indexes) with 200.00 GB size " +
		"and throughput requirement of 6000 reads/sec and 4000 writes/sec need to be migrated as range " +
		"partitioned tables. Non leaf partition tables/indexes and unsupported tables/indexes were not considered."

	result := getReasoning(recommendation, shardedObjects, 1, colocatedObjects, 1)
	if result != expected {
		t.Errorf("Expected %q but got %q", expected, result)
	}
}

/*
===== 	Test functions to test getThresholdAndTablets function	=====
*/
func TestGetThresholdAndTablets(t *testing.T) {
	tests := []struct {
		previousNumNodes  float64
		sizeGB            float64
		expectedThreshold float64
		expectedTablets   int
	}{
		// test data where previousNumNodes is 3
		{3, 0.512, LOW_PHASE_SIZE_THRESHOLD_GB, 1},
		{3, 1, LOW_PHASE_SIZE_THRESHOLD_GB, 2},
		{3, 1.1, LOW_PHASE_SIZE_THRESHOLD_GB, 3},
		{3, 2, HIGH_PHASE_SIZE_THRESHOLD_GB, 3},
		{3, 10, HIGH_PHASE_SIZE_THRESHOLD_GB, 3},
		{3, 229, HIGH_PHASE_SIZE_THRESHOLD_GB, 23},
		{3, 241, HIGH_PHASE_SIZE_THRESHOLD_GB, 25},
		{3, 711, HIGH_PHASE_SIZE_THRESHOLD_GB, 72},
		{3, 7180, FINAL_PHASE_SIZE_THRESHOLD_GB, 72},
		{3, 7201, FINAL_PHASE_SIZE_THRESHOLD_GB, 73},
		// test data where previousNumNodes is 4
		{4, 0.512, LOW_PHASE_SIZE_THRESHOLD_GB, 1},
		{4, 1, LOW_PHASE_SIZE_THRESHOLD_GB, 2},
		{4, 1.1, LOW_PHASE_SIZE_THRESHOLD_GB, 3},
		{4, 2, LOW_PHASE_SIZE_THRESHOLD_GB, 4},
		{4, 10, HIGH_PHASE_SIZE_THRESHOLD_GB, 4},
		{4, 229, HIGH_PHASE_SIZE_THRESHOLD_GB, 23},
		{4, 241, HIGH_PHASE_SIZE_THRESHOLD_GB, 25},
		{4, 949, HIGH_PHASE_SIZE_THRESHOLD_GB, 95},
		{4, 951, HIGH_PHASE_SIZE_THRESHOLD_GB, 96},
		{4, 961, FINAL_PHASE_SIZE_THRESHOLD_GB, 96},
	}

	for _, test := range tests {
		threshold, tablets := getThresholdAndTablets(test.previousNumNodes, test.sizeGB)
		if threshold != test.expectedThreshold || tablets != test.expectedTablets {
			t.Errorf("getThresholdAndTablets(%v, %v) = (%v, %v); want (%v, %v)",
				test.previousNumNodes, test.sizeGB, threshold, tablets, test.expectedThreshold, test.expectedTablets)
		}
	}

}

/*
===== 	Test functions to test findImportTimeFromExpDataLoadTime function	=====
*/
func TestFindImportTimeFromExpDataLoadTime_RowsArePreferred(t *testing.T) {
	loadTimes := []ExpDataLoadTime{
		{
			csvSizeGB:         sql.NullFloat64{Valid: true, Float64: 19},
			migrationTimeSecs: sql.NullFloat64{Valid: true, Float64: 1500},
			rowCount:          sql.NullFloat64{Valid: true, Float64: 100000000},
		},
		{
			csvSizeGB:         sql.NullFloat64{Valid: true, Float64: 19},
			migrationTimeSecs: sql.NullFloat64{Valid: true, Float64: 3000},
			rowCount:          sql.NullFloat64{Valid: true, Float64: 300000000},
		},
		{
			csvSizeGB:         sql.NullFloat64{Valid: true, Float64: 19},
			migrationTimeSecs: sql.NullFloat64{Valid: true, Float64: 4500},
			rowCount:          sql.NullFloat64{Valid: true, Float64: 600000000},
		},
	}
	expectedImportTime := 3750.0

	var objectSize float64 = 19
	var rowsInTable float64 = 500000000
	actualImportTime := findImportTimeFromExpDataLoadTime(loadTimes, objectSize, rowsInTable)

	if actualImportTime != expectedImportTime {
		t.Errorf("Expected %f but go %f", expectedImportTime, actualImportTime)
	}
}

func TestFindImportTimeFromExpDataLoadTime_SizePreferredIfRowsAreSame(t *testing.T) {
	loadTimes := []ExpDataLoadTime{
		{
			csvSizeGB:         sql.NullFloat64{Valid: true, Float64: 24},
			migrationTimeSecs: sql.NullFloat64{Valid: true, Float64: 2800},
			rowCount:          sql.NullFloat64{Valid: true, Float64: 100000000},
		},
		{
			csvSizeGB:         sql.NullFloat64{Valid: true, Float64: 25},
			migrationTimeSecs: sql.NullFloat64{Valid: true, Float64: 3000},
			rowCount:          sql.NullFloat64{Valid: true, Float64: 100000000},
		},
		{
			csvSizeGB:         sql.NullFloat64{Valid: true, Float64: 30},
			migrationTimeSecs: sql.NullFloat64{Valid: true, Float64: 4500},
			rowCount:          sql.NullFloat64{Valid: true, Float64: 100000000},
		},
	}
	expectedImportTime := 3000.0

	var objectSize float64 = 25
	var rowsInTable float64 = 100000000
	actualImportTime := findImportTimeFromExpDataLoadTime(loadTimes, objectSize, rowsInTable)
	if actualImportTime != expectedImportTime {
		t.Errorf("Expected %v but got %v", expectedImportTime, actualImportTime)
	}
}

/*
===== 	Test functions to test findClosestVersion function	=====
*/
// closest version to respective targetYbVersion should be actual closest or default
func TestFindClosestVersion(t *testing.T) {
	defaultYbVersionId := 2
	targetVersionAndClosest := map[string]int64{
		"2024.1.0.0": 1,
		"2024.1.2.0": 1,
		"2024.2.0.0": 2,
		"2024.2.3.0": 2,
		// in case of no closest found, default should be used
		"2.21.0.0": 2,
		"2.23.0.0": 2,
		"2.25.0.0": 2,
		"2.14.1.0": 2,
		"2.18.2.0": 2,
		"2.20.1.2": 2,
	}
	version20241, _ := ybversion.NewYBVersion("2024.1.0.0")
	version20242, _ := ybversion.NewYBVersion("2024.2.0.0")

	experimentDataAvailableVersions := []ExperimentDataAvailableYbVersion{
		{
			versionId:        1,
			expDataYbVersion: version20241,
			expDataIsDefault: false,
		},
		{
			versionId:        2,
			expDataYbVersion: version20242,
			expDataIsDefault: true,
		},
	}

	for tagetVersion, expectedClosestId := range targetVersionAndClosest {
		targetDBVersion, _ := ybversion.NewYBVersion(tagetVersion)
		actualClosestId := findClosestVersion(targetDBVersion, experimentDataAvailableVersions, int64(defaultYbVersionId))
		if actualClosestId != expectedClosestId {
			t.Errorf("Expected %v but got %v", expectedClosestId, actualClosestId)
		}
	}

}

/*
===== 	Test functions to test findNumNodesThroughputScalingImportTimeDivisor function	=====
*/

// Actual experimental data for throughput scaling with different number of nodes
// Data sourced from num_nodes_throughput_scaling table for 4 cores, 4GB memory per core configuration
var numNodesThroughputScalingTestData = []ExpDataLoadTimeNumNodesImpact{
	{
		numNodes:             sql.NullInt64{Int64: 3, Valid: true},
		importThroughputMbps: sql.NullFloat64{Float64: 14.64, Valid: true},
	},
	{
		numNodes:             sql.NullInt64{Int64: 6, Valid: true},
		importThroughputMbps: sql.NullFloat64{Float64: 21.35, Valid: true},
	},
	{
		numNodes:             sql.NullInt64{Int64: 9, Valid: true},
		importThroughputMbps: sql.NullFloat64{Float64: 26.55, Valid: true},
	},
}

// Experimental data for 8 cores, 4GB memory per core configuration
var numNodesThroughputScalingTestData8Core = []ExpDataLoadTimeNumNodesImpact{
	{
		numNodes:             sql.NullInt64{Int64: 3, Valid: true},
		importThroughputMbps: sql.NullFloat64{Float64: 29.84, Valid: true},
	},
	{
		numNodes:             sql.NullInt64{Int64: 6, Valid: true},
		importThroughputMbps: sql.NullFloat64{Float64: 46.64, Valid: true},
	},
	{
		numNodes:             sql.NullInt64{Int64: 9, Valid: true},
		importThroughputMbps: sql.NullFloat64{Float64: 61.79, Valid: true},
	},
}

// Experimental data for 16 cores, 4GB memory per core configuration
var numNodesThroughputScalingTestData16Core = []ExpDataLoadTimeNumNodesImpact{
	{
		numNodes:             sql.NullInt64{Int64: 3, Valid: true},
		importThroughputMbps: sql.NullFloat64{Float64: 49.31, Valid: true},
	},
	{
		numNodes:             sql.NullInt64{Int64: 6, Valid: true},
		importThroughputMbps: sql.NullFloat64{Float64: 76.9, Valid: true},
	},
	{
		numNodes:             sql.NullInt64{Int64: 9, Valid: true},
		importThroughputMbps: sql.NullFloat64{Float64: 124.52, Valid: true},
	},
}

// validate that the function returns correct ratio for exact match (3 nodes)
func TestFindNumNodesThroughputScalingImportTimeDivisor_ExactMatch3Nodes(t *testing.T) {
	targetNumNodes := 3.0
	// For exact match with 3-node baseline: estimatedThroughput = 3 * (14.64/3) = 14.64
	// Ratio = 14.64 / 14.64 = 1.0
	expectedRatio := 1.0

	actualRatio := findNumNodesThroughputScalingImportTimeDivisor(numNodesThroughputScalingTestData, targetNumNodes)
	assert.Equal(t, expectedRatio, actualRatio)
}

// validate unit test covering different number of nodes
func TestFindNumNodesThroughputScalingImportTimeDivisor_AllRequestedCounts(t *testing.T) {
	testCases := []struct {
		targetNodes   float64
		expectedRatio float64
		description   string
	}{
		{3.0, 1.0, "Exact match for 3 nodes"},
		{4.0, Round(4.0*(14.64/3.0)/14.64, 3), "4 nodes, closest to 3"},
		{5.0, Round(5.0*(21.35/6.0)/14.64, 3), "5 nodes, closest to 6"},
		{6.0, Round(6.0*(21.35/6.0)/14.64, 3), "Exact match for 6 nodes"},
		{7.0, Round(7.0*(21.35/6.0)/14.64, 3), "7 nodes, closest to 6"},
		{8.0, Round(8.0*(26.55/9.0)/14.64, 3), "8 nodes, closest to 9"},
		{9.0, Round(9.0*(26.55/9.0)/14.64, 3), "Exact match for 9 nodes"},
		{10.0, Round(10.0*(26.55/9.0)/14.64, 3), "10 nodes, closest to 9"},
		{12.0, Round(12.0*(26.55/9.0)/14.64, 3), "12 nodes, closest to 9"},
		{18.0, Round(18.0*(26.55/9.0)/14.64, 3), "18 nodes, closest to 9"},
		{21.0, Round(21.0*(26.55/9.0)/14.64, 3), "21 nodes, closest to 9"},
		{100.0, Round(100.0*(26.55/9.0)/14.64, 3), "100 nodes, closest to 9"},
		{1000.0, Round(1000.0*(26.55/9.0)/14.64, 3), "1000 nodes, closest to 9"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actualRatio := findNumNodesThroughputScalingImportTimeDivisor(numNodesThroughputScalingTestData, tc.targetNodes)
			assert.Equal(t, tc.expectedRatio, Round(actualRatio, 3),
				fmt.Sprintf("Failed for %v nodes: expected %v, got %v", tc.targetNodes, tc.expectedRatio, actualRatio))
		})
	}
}

// validate test with 8-core configuration data
func TestFindNumNodesThroughputScalingImportTimeDivisor_8CoreConfig(t *testing.T) {
	testCases := []struct {
		targetNodes   float64
		expectedRatio float64
		description   string
	}{
		{3.0, 1.0, "Exact match for 3 nodes with 8-core config"},
		{5.0, Round(5.0*(46.64/6.0)/29.84, 3), "5 nodes, closest to 6 with 8-core config"},
		{12.0, Round(12.0*(61.79/9.0)/29.84, 3), "12 nodes, closest to 9 with 8-core config"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actualRatio := findNumNodesThroughputScalingImportTimeDivisor(numNodesThroughputScalingTestData8Core, tc.targetNodes)
			assert.Equal(t, tc.expectedRatio, Round(actualRatio, 3),
				fmt.Sprintf("Failed for %v nodes: expected %v, got %v", tc.targetNodes, tc.expectedRatio, actualRatio))
		})
	}
}

// validate test with 16-core configuration data
func TestFindNumNodesThroughputScalingImportTimeDivisor_16CoreConfig(t *testing.T) {
	testCases := []struct {
		targetNodes   float64
		expectedRatio float64
		description   string
	}{
		{3.0, 1.0, "Exact match for 3 nodes with 16-core config"},
		{7.0, Round(7.0*(76.9/6.0)/49.31, 3), "7 nodes, closest to 6 with 16-core config"},
		{21.0, Round(21.0*(124.52/9.0)/49.31, 3), "21 nodes, closest to 9 with 16-core config"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actualRatio := findNumNodesThroughputScalingImportTimeDivisor(numNodesThroughputScalingTestData16Core, tc.targetNodes)
			assert.Equal(t, tc.expectedRatio, Round(actualRatio, 3),
				fmt.Sprintf("Failed for %v nodes: expected %v, got %v", tc.targetNodes, tc.expectedRatio, actualRatio))
		})
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

// Test functions for redundant index filtering

func TestFetchRedundantIndexes(t *testing.T) {
	tests := []struct {
		name           string
		mockRows       [][]interface{}
		expectedResult []utils.RedundantIndexesInfo
		expectError    bool
	}{
		{
			name: "successful fetch with multiple redundant indexes",
			mockRows: [][]interface{}{
				{"schema1", "table1", "idx1"},
				{"schema2", "table2", "idx2"},
				{"schema1", "table1", "idx3"},
			},
			expectedResult: []utils.RedundantIndexesInfo{
				{DBType: constants.POSTGRESQL, RedundantSchemaName: "schema1", RedundantTableName: "table1", RedundantIndexName: "idx1"},
				{DBType: constants.POSTGRESQL, RedundantSchemaName: "schema2", RedundantTableName: "table2", RedundantIndexName: "idx2"},
				{DBType: constants.POSTGRESQL, RedundantSchemaName: "schema1", RedundantTableName: "table1", RedundantIndexName: "idx3"},
			},
			expectError: false,
		},
		{
			name:           "successful fetch with no redundant indexes",
			mockRows:       [][]interface{}{},
			expectedResult: []utils.RedundantIndexesInfo{},
			expectError:    false,
		},
		{
			name: "single redundant index",
			mockRows: [][]interface{}{
				{"public", "users", "idx_email_duplicate"},
			},
			expectedResult: []utils.RedundantIndexesInfo{
				{DBType: constants.POSTGRESQL, RedundantSchemaName: "public", RedundantTableName: "users", RedundantIndexName: "idx_email_duplicate"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up SourceDBType for the test
			originalSourceDBType := SourceDBType
			SourceDBType = constants.POSTGRESQL // Set to a valid database type for testing
			defer func() {
				SourceDBType = originalSourceDBType // Restore original value
			}()

			db, mock := createMockDB(t)
			defer db.Close()

			// Setup mock expectation
			query := fmt.Sprintf(`SELECT redundant_schema_name, redundant_table_name, redundant_index_name FROM %s`, GetTableRedundantIndexesName())
			expectedQuery := regexp.QuoteMeta(query)

			rows := sqlmock.NewRows([]string{"redundant_schema_name", "redundant_table_name", "redundant_index_name"})
			for _, row := range tt.mockRows {
				rows.AddRow(row[0], row[1], row[2])
			}
			mock.ExpectQuery(expectedQuery).WillReturnRows(rows)

			// Execute the function
			sourceDBType := constants.POSTGRESQL
			result, err := getSourceMetadataRedundantIndexes(db, GetTableRedundantIndexesName(), sourceDBType)

			// Assert results
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			// Assert all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFetchRedundantIndexesQueryError(t *testing.T) {
	// Set up SourceDBType for the test
	originalSourceDBType := SourceDBType
	SourceDBType = "postgresql" // Set to a valid database type for testing
	defer func() {
		SourceDBType = originalSourceDBType // Restore original value
	}()

	db, mock := createMockDB(t)
	defer db.Close()

	// Setup mock to return an error
	query := fmt.Sprintf(`SELECT redundant_schema_name, redundant_table_name, redundant_index_name FROM %s`, GetTableRedundantIndexesName())
	expectedQuery := regexp.QuoteMeta(query)
	mock.ExpectQuery(expectedQuery).WillReturnError(fmt.Errorf("database connection error"))

	// Execute the function
	sourceDBType := constants.POSTGRESQL
	result, err := getSourceMetadataRedundantIndexes(db, GetTableRedundantIndexesName(), sourceDBType)

	// Assert error
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "error querying redundant indexes")

	// Assert all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFilterRedundantIndexes(t *testing.T) {
	tests := []struct {
		name                   string
		sourceIndexMetadata    []SourceDBMetadata
		redundantIndexes       []utils.RedundantIndexesInfo
		expectedFilteredCount  int
		expectedRemainingNames []string
	}{
		{
			name: "filter out single redundant index",
			sourceIndexMetadata: []SourceDBMetadata{
				{
					SchemaName:      "schema1",
					ObjectName:      "idx1",
					ParentTableName: sql.NullString{String: "schema1.table1", Valid: true},
					IsIndex:         true,
				},
				{
					SchemaName:      "schema1",
					ObjectName:      "idx2",
					ParentTableName: sql.NullString{String: "schema1.table1", Valid: true},
					IsIndex:         true,
				},
			},
			redundantIndexes: []utils.RedundantIndexesInfo{
				{DBType: constants.POSTGRESQL, RedundantSchemaName: "schema1", RedundantTableName: "table1", RedundantIndexName: "idx1"},
			},
			expectedFilteredCount:  1,
			expectedRemainingNames: []string{"idx2"},
		},
		{
			name: "filter out multiple redundant indexes",
			sourceIndexMetadata: []SourceDBMetadata{
				{
					SchemaName:      "schema1",
					ObjectName:      "idx1",
					ParentTableName: sql.NullString{String: "schema1.table1", Valid: true},
					IsIndex:         true,
				},
				{
					SchemaName:      "schema1",
					ObjectName:      "idx2",
					ParentTableName: sql.NullString{String: "schema1.table1", Valid: true},
					IsIndex:         true,
				},
				{
					SchemaName:      "schema2",
					ObjectName:      "idx3",
					ParentTableName: sql.NullString{String: "schema2.table2", Valid: true},
					IsIndex:         true,
				},
			},
			redundantIndexes: []utils.RedundantIndexesInfo{
				{DBType: constants.POSTGRESQL, RedundantSchemaName: "schema1", RedundantTableName: "table1", RedundantIndexName: "idx1"},
				{DBType: constants.POSTGRESQL, RedundantSchemaName: "schema2", RedundantTableName: "table2", RedundantIndexName: "idx3"},
			},
			expectedFilteredCount:  1,
			expectedRemainingNames: []string{"idx2"},
		},
		{
			name: "no redundant indexes to filter",
			sourceIndexMetadata: []SourceDBMetadata{
				{
					SchemaName:      "schema1",
					ObjectName:      "idx1",
					ParentTableName: sql.NullString{String: "schema1.table1", Valid: true},
					IsIndex:         true,
				},
				{
					SchemaName:      "schema1",
					ObjectName:      "idx2",
					ParentTableName: sql.NullString{String: "schema1.table1", Valid: true},
					IsIndex:         true,
				},
			},
			redundantIndexes:       []utils.RedundantIndexesInfo{},
			expectedFilteredCount:  2,
			expectedRemainingNames: []string{"idx1", "idx2"},
		},
		{
			name: "redundant index not matching any source index",
			sourceIndexMetadata: []SourceDBMetadata{
				{
					SchemaName:      "schema1",
					ObjectName:      "idx1",
					ParentTableName: sql.NullString{String: "schema1.table1", Valid: true},
					IsIndex:         true,
				},
			},
			redundantIndexes: []utils.RedundantIndexesInfo{
				{DBType: constants.POSTGRESQL, RedundantSchemaName: "schema2", RedundantTableName: "table2", RedundantIndexName: "idx_nonexistent"},
			},
			expectedFilteredCount:  1,
			expectedRemainingNames: []string{"idx1"},
		},
		{
			name: "index with invalid parent table name",
			sourceIndexMetadata: []SourceDBMetadata{
				{
					SchemaName:      "schema1",
					ObjectName:      "idx1",
					ParentTableName: sql.NullString{Valid: false},
					IsIndex:         true,
				},
				{
					SchemaName:      "schema1",
					ObjectName:      "idx2",
					ParentTableName: sql.NullString{String: "schema1.table1", Valid: true},
					IsIndex:         true,
				},
			},
			redundantIndexes: []utils.RedundantIndexesInfo{
				{DBType: constants.POSTGRESQL, RedundantSchemaName: "schema1", RedundantTableName: "table1", RedundantIndexName: "idx2"},
			},
			expectedFilteredCount:  1,
			expectedRemainingNames: []string{"idx1"},
		},
		{
			name: "parent table name with unexpected format",
			sourceIndexMetadata: []SourceDBMetadata{
				{
					SchemaName:      "schema1",
					ObjectName:      "idx1",
					ParentTableName: sql.NullString{String: "just_table_name", Valid: true},
					IsIndex:         true,
				},
			},
			redundantIndexes: []utils.RedundantIndexesInfo{
				{DBType: constants.POSTGRESQL, RedundantSchemaName: "schema1", RedundantTableName: "just_table_name", RedundantIndexName: "idx1"},
			},
			expectedFilteredCount:  0,
			expectedRemainingNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up SourceDBType for the test
			originalSourceDBType := SourceDBType
			SourceDBType = constants.POSTGRESQL // Set to a valid database type for testing
			defer func() {
				SourceDBType = originalSourceDBType // Restore original value
			}()

			result := filterRedundantIndexes(tt.sourceIndexMetadata, tt.redundantIndexes, SourceDBType)

			// Assert the count of filtered indexes
			assert.Equal(t, tt.expectedFilteredCount, len(result))

			// Assert the remaining index names
			var remainingNames []string
			for _, index := range result {
				remainingNames = append(remainingNames, index.ObjectName)
			}
			assert.ElementsMatch(t, tt.expectedRemainingNames, remainingNames)
		})
	}
}

func TestGetSourceMetadataWithRedundantIndexFiltering(t *testing.T) {
	// Set up SourceDBType for the test
	originalSourceDBType := SourceDBType
	SourceDBType = "postgresql" // Set to a valid database type for testing
	defer func() {
		SourceDBType = originalSourceDBType // Restore original value
	}()

	db, mock := createMockDB(t)
	defer db.Close()

	// Setup mock for main metadata query
	metadataQuery := fmt.Sprintf(`
		SELECT schema_name, 
			   object_name, 
			   row_count, 
			   reads_per_second, 
			   writes_per_second, 
			   is_index, 
			   parent_table_name, 
			   size_in_bytes,
			   column_count 
		FROM %v 
		WHERE %s
		ORDER BY IFNULL(size_in_bytes, 0) ASC
	`, GetTableIndexStatName(), strings.Join([]string{
		fmt.Sprintf("object_type LIKE '%s'", "%table%"),
		fmt.Sprintf("object_type LIKE '%s'", "%index%"),
		fmt.Sprintf("object_type LIKE '%s'", "materialized view"),
	}, " OR "))

	metadataRows := sqlmock.NewRows([]string{
		"schema_name", "object_name", "row_count", "reads_per_second", "writes_per_second",
		"is_index", "parent_table_name", "size_in_bytes", "column_count",
	}).
		AddRow("schema1", "table1", 1000, 10, 5, false, nil, 1024*1024*1024, 5).        // table
		AddRow("schema1", "idx1", nil, 5, 2, true, "schema1.table1", 512*1024*1024, 2). // index (should be filtered)
		AddRow("schema1", "idx2", nil, 3, 1, true, "schema1.table1", 256*1024*1024, 1)  // index (should remain)

	mock.ExpectQuery(regexp.QuoteMeta(strings.ReplaceAll(metadataQuery, "\n\t\t", " "))).WillReturnRows(metadataRows)

	// Setup mock for redundant indexes query
	redundantQuery := fmt.Sprintf(`SELECT redundant_schema_name, redundant_table_name, redundant_index_name FROM %s`, GetTableRedundantIndexesName())
	redundantRows := sqlmock.NewRows([]string{"redundant_schema_name", "redundant_table_name", "redundant_index_name"}).
		AddRow("schema1", "table1", "idx1")

	mock.ExpectQuery(regexp.QuoteMeta(redundantQuery)).WillReturnRows(redundantRows)

	// Expect database close
	mock.ExpectClose()

	// Execute the function
	tables, indexes, filteredIndexes, totalSize, err := getSourceMetadata(db, SourceDBType)

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tables))
	assert.Equal(t, 2, len(indexes))         // Original indexes should remain unfiltered
	assert.Equal(t, 1, len(filteredIndexes)) // Only idx2 should remain, idx1 should be filtered
	assert.Equal(t, "table1", tables[0].ObjectName)
	assert.Equal(t, "idx2", filteredIndexes[0].ObjectName)
	assert.Greater(t, totalSize, 0.0)

	// Assert all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSourceMetadataWithRedundantIndexError(t *testing.T) {
	// Set up SourceDBType for the test
	originalSourceDBType := SourceDBType
	SourceDBType = "postgresql" // Set to a valid database type for testing
	defer func() {
		SourceDBType = originalSourceDBType // Restore original value
	}()

	db, mock := createMockDB(t)
	defer db.Close()

	// Setup mock for main metadata query
	metadataQuery := fmt.Sprintf(`
		SELECT schema_name, 
			   object_name, 
			   row_count, 
			   reads_per_second, 
			   writes_per_second, 
			   is_index, 
			   parent_table_name, 
			   size_in_bytes,
			   column_count 
		FROM %v 
		WHERE %s
		ORDER BY IFNULL(size_in_bytes, 0) ASC
	`, GetTableIndexStatName(), strings.Join([]string{
		fmt.Sprintf("object_type LIKE '%s'", "%table%"),
		fmt.Sprintf("object_type LIKE '%s'", "%index%"),
		fmt.Sprintf("object_type LIKE '%s'", "materialized view"),
	}, " OR "))

	metadataRows := sqlmock.NewRows([]string{
		"schema_name", "object_name", "row_count", "reads_per_second", "writes_per_second",
		"is_index", "parent_table_name", "size_in_bytes", "column_count",
	}).
		AddRow("schema1", "table1", 1000, 10, 5, false, nil, 1024*1024*1024, 5).       // table
		AddRow("schema1", "idx1", nil, 5, 2, true, "schema1.table1", 512*1024*1024, 2) // index

	mock.ExpectQuery(regexp.QuoteMeta(strings.ReplaceAll(metadataQuery, "\n\t\t", " "))).WillReturnRows(metadataRows)

	// Setup mock for redundant indexes query to return error
	redundantQuery := fmt.Sprintf(`SELECT redundant_schema_name, redundant_table_name, redundant_index_name FROM %s`, GetTableRedundantIndexesName())
	mock.ExpectQuery(regexp.QuoteMeta(redundantQuery)).WillReturnError(fmt.Errorf("redundant table does not exist"))

	// Expect database close
	mock.ExpectClose()

	// Execute the function
	tables, indexes, _, totalSize, err := getSourceMetadata(db, SourceDBType)

	// Assert results - should continue despite redundant index error
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tables))
	assert.Equal(t, 1, len(indexes)) // index should remain since filtering failed gracefully
	assert.Equal(t, "table1", tables[0].ObjectName)
	assert.Equal(t, "idx1", indexes[0].ObjectName)
	assert.Greater(t, totalSize, 0.0)

	// Assert all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}
