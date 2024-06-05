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
===== 	Test functions to test getTargetMetadata function	=====
*/

/*
===== 	Test functions to test shardingBasedOnTableSizeAndCount function	=====
*/

/*
===== 	Test functions to test shardingBasedOnOperations function	=====
*/

/*
===== 	Test functions to test checkShardedTableLimit function	=====
*/

/*
===== 	Test functions to test findNumNodesNeededBasedOnThroughputRequirement function	=====
*/

/*
===== 	Test functions to test findNumNodesNeededBasedOnTabletsRequired function	=====
*/

/*
===== 	Test functions to test pickBestRecommendation function	=====
*/

/*
===== 	Test functions to test calculateTimeTakenAndParallelJobsForImport function	=====
*/

/*
===== 	Test functions to test getReasoning function	=====
*/

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
