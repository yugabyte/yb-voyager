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
	_ "embed"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"

	log "github.com/sirupsen/logrus"
)

type SourceDBMetadata struct {
	SchemaName      string         `db:"schema_name"`
	ObjectName      string         `db:"object_name"`
	RowCount        sql.NullInt64  `db:"row_count,string"`
	ColumnCount     sql.NullInt64  `db:"column_count,string"`
	Reads           int64          `db:"reads,string"`
	Writes          int64          `db:"writes,string"`
	ReadsPerSec     int64          `db:"reads_per_second,string"`
	WritesPerSec    int64          `db:"writes_per_second,string"`
	IsIndex         bool           `db:"is_index,string"`
	ParentTableName sql.NullString `db:"parent_table_name"`
	Size            float64        `db:"size_in_bytes,string"`
}

type ExpDataColocatedLimit struct {
	maxColocatedSizeSupported  sql.NullFloat64 `db:"max_colocated_db_size_gb,string"`
	numCores                   sql.NullFloat64 `db:"num_cores,string"`
	memPerCore                 sql.NullFloat64 `db:"mem_per_core,string"`
	maxSupportedNumTables      sql.NullInt64   `db:"max_num_tables,string"`
	minSupportedNumTables      sql.NullFloat64 `db:"min_num_tables,string"`
	maxSupportedSelectsPerCore sql.NullFloat64 `db:"max_selects_per_core,string"`
	maxSupportedInsertsPerCore sql.NullFloat64 `db:"max_inserts_per_core,string"`
}

const (
	COLOCATED_LIMITS_TABLE    = "colocated_limits"
	SHARDED_SIZING_TABLE      = "sharded_sizing"
	COLOCATED_LOAD_TIME_TABLE = "colocated_load_time"
	SHARDED_LOAD_TIME_TABLE   = "sharded_load_time"
	// GITHUB_RAW_LINK use raw github link to fetch the file from repository using the api:
	// https://raw.githubusercontent.com/{username-or-organization}/{repository}/{branch}/{path-to-file}
	GITHUB_RAW_LINK          = "https://raw.githubusercontent.com/yugabyte/yb-voyager/main/yb-voyager/src/migassessment/resources"
	EXPERIMENT_DATA_FILENAME = "yb_2024_0_source.db"
)

var ExperimentDB *sql.DB

//go:embed resources/yb_2024_0_source.db
var experimentData20240 []byte

func SizingAssessment() error {

	log.Infof("loading metadata files for sharding assessment")
	sourceTableMetadata, sourceIndexMetadata, totalSourceDBSize, err := loadSourceMetadata()
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("failed to load source metadata: %v", err)
		return fmt.Errorf("failed to load source metadata: %w", err)
	}

	err = createConnectionToExperimentData()
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("failed to connect to experiment data: %v", err)
		return fmt.Errorf("failed to connect to experiment data: %w", err)
	}

	colocatedObjects, _, coresToUse, shardedObjects, reasoning, err :=
		generateShardingRecommendations(sourceTableMetadata, sourceIndexMetadata, totalSourceDBSize)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("error generating sharding recommendations: %v", err)
		return fmt.Errorf("error generating sharding recommendations: %w", err)
	}

	// only use the remaining sharded objects and its size for further recommendation processing
	numNodes, optimalSelectConnections, optimalInsertConnections, err :=
		generateSizingRecommendations(shardedObjects, len(colocatedObjects), coresToUse)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("error generate sizing recommendations: %v", err)
		return fmt.Errorf("error generate sizing recommendations: %w", err)
	}

	// calculate time taken for colocated import
	importTimeForColocatedObjects, parallelThreadsColocated, err :=
		calculateTimeTakenAndParallelThreadsForImport(COLOCATED_LOAD_TIME_TABLE, colocatedObjects,
			coresToUse.numCores.Float64, coresToUse.memPerCore.Float64)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("calculate time taken for colocated objects import: %v", err)
		return fmt.Errorf("calculate time taken for colocated object import: %w", err)
	}

	// calculate time taken for sharded import
	importTimeForShardedObjects, parallelThreadsSharded, err :=
		calculateTimeTakenAndParallelThreadsForImport(SHARDED_LOAD_TIME_TABLE, shardedObjects,
			coresToUse.numCores.Float64, coresToUse.memPerCore.Float64)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("calculate time taken for sharded objects import: %v", err)
		return fmt.Errorf("calculate time taken for sharded objects import: %w", err)
	}
	sizingRecommendation := &SizingRecommendation{
		ColocatedTables:                 fetchObjectNames(colocatedObjects),
		ShardedTables:                   fetchObjectNames(shardedObjects),
		VCPUsPerInstance:                coresToUse.numCores.Float64,
		MemoryPerInstance:               coresToUse.numCores.Float64 * coresToUse.memPerCore.Float64,
		NumNodes:                        numNodes,
		OptimalSelectConnectionsPerNode: optimalSelectConnections,
		OptimalInsertConnectionsPerNode: optimalInsertConnections,
		ParallelVoyagerThreads:          math.Min(float64(parallelThreadsColocated), float64(parallelThreadsSharded)),
		ColocatedReasoning:              reasoning,
		ImportTimeTakenInMin:            importTimeForColocatedObjects + importTimeForShardedObjects,
	}
	SizingReport.SizingRecommendation = *sizingRecommendation

	return nil
}

func fetchObjectNames(dbObjects []SourceDBMetadata) []string {
	var objectNames []string
	for _, dbObject := range dbObjects {
		objectNames = append(objectNames, dbObject.SchemaName+"."+dbObject.ObjectName)
	}
	return objectNames
}

/*
loadSourceMetadata connects to the assessment metadata of the source database and generates the slice of objects
for tables and indexes. It also returns the total size of source db in GB. Primary key size is not considered in the
calculation.
Returns:

	[]SourceDBMetadata: all table objects from source db
	[]SourceDBMetadata: all index objects from source db
	float64: total size of source db
*/
func loadSourceMetadata() ([]SourceDBMetadata, []SourceDBMetadata, float64, error) {
	SourceMetaDB, err := utils.ConnectToSqliteDatabase(GetSourceMetadataDBFilePath())
	if err != nil {
		return nil, nil, 0.0, fmt.Errorf("cannot connect to source metadata database: %w", err)
	}
	return getSourceMetadata(SourceMetaDB)
}

/*
generateShardingRecommendations analyzes source database metadata to generate sharding recommendations.
It calculates the required number of nodes, CPU cores, and memory for colocated objects. It tries to fit in all source
objects(in increasing order of size) as colocated and scales vertically to achieve it going upto 16 cores.
Then onwards, we make the rest of the objects as sharded. The function returns a slice containing metadata for sharded
objects and the total size of sharded objects. Additionally, it updates a global report structure with information on
colocated and sharded tables, recommended instance configurations, and import time estimates for colocated object
import.
Parameters:

	sourceTableMetadata: A slice containing metadata for source database tables.
	sourceIndexMetadata: A slice containing metadata for source database indexes.
	totalSourceDBSize: The total size of the source database in gigabytes.

Returns:

	[]SourceDBMetadata: A slice containing metadata for sharded objects. Used for sharding recommendations.
	float64: The total size of sharded objects in gigabytes. Used for sharding recommendations.
	ExpDataColocatedLimit: Information about the instance type to use for the experiment
	SourceDBMetadata[]: list of sharded objects
	string: reasoning for decision-making.
*/

func generateShardingRecommendations(sourceTableMetadata []SourceDBMetadata, sourceIndexMetadata []SourceDBMetadata,
	totalSourceDBSize float64) ([]SourceDBMetadata, float64, ExpDataColocatedLimit, []SourceDBMetadata, string, error) {
	var previousReasoning string
	previousCores := ExpDataColocatedLimit{
		maxColocatedSizeSupported:  sql.NullFloat64{Float64: -1, Valid: true},
		numCores:                   sql.NullFloat64{Float64: -1, Valid: true},
		memPerCore:                 sql.NullFloat64{Float64: -1, Valid: true},
		maxSupportedNumTables:      sql.NullInt64{Int64: -1, Valid: true},
		minSupportedNumTables:      sql.NullFloat64{Float64: -1, Valid: true},
		maxSupportedSelectsPerCore: sql.NullFloat64{Float64: -1, Valid: true},
		maxSupportedInsertsPerCore: sql.NullFloat64{Float64: -1, Valid: true},
	}

	query := fmt.Sprintf(`
		SELECT max_colocated_db_size_gb, 
			   num_cores, 
			   mem_per_core, 
			   max_num_tables, 
			   min_num_tables, 
			   max_selects_per_core, 
			   max_inserts_per_core 
		FROM %v 
		ORDER BY num_cores DESC
	`, COLOCATED_LIMITS_TABLE)
	rows, err := ExperimentDB.Query(query)
	if err != nil {
		return nil, 0.0, ExpDataColocatedLimit{}, nil, "", fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", query, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close the result set for query [%v]", query)
		}
	}()

	for rows.Next() {
		var r1 ExpDataColocatedLimit
		var cumulativeObjectCount int64 = 0
		var cumulativeSelectOpsPerSec int64 = 0
		var cumulativeInsertOpsPerSec int64 = 0
		var cumulativeSizeSum float64 = 0
		var colocatedObjects []SourceDBMetadata
		var currentReasoning string
		var allObjectsColocated bool = true

		if err := rows.Scan(&r1.maxColocatedSizeSupported, &r1.numCores, &r1.memPerCore, &r1.maxSupportedNumTables,
			&r1.minSupportedNumTables, &r1.maxSupportedSelectsPerCore, &r1.maxSupportedInsertsPerCore); err != nil {
			return nil, 0.0, ExpDataColocatedLimit{}, nil, previousReasoning, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", query, err)
		}
		currentReasoning = fmt.Sprintf(" Recommended instance with %vvCPU and %vGiB memory could ",
			r1.numCores.Float64, r1.numCores.Float64*r1.memPerCore.Float64)

		for i, table := range sourceTableMetadata {
			// check if current table has any indexes and fetch all indexes
			indexesOfTable, indexesSizeSum, indexReads, indexWrites := checkAndFetchIndexes(table, sourceIndexMetadata)
			cumulativeObjectCount += int64(len(indexesOfTable)) + 1
			objectTotalSize := table.Size + indexesSizeSum
			cumulativeSizeSum += objectTotalSize
			cumulativeSelectOpsPerSec += table.ReadsPerSec + indexReads
			cumulativeInsertOpsPerSec += table.WritesPerSec + indexWrites

			neededCores :=
				math.Ceil(float64(cumulativeSelectOpsPerSec)/r1.maxSupportedSelectsPerCore.Float64 +
					float64(cumulativeInsertOpsPerSec)/r1.maxSupportedInsertsPerCore.Float64)

			if (neededCores <= r1.numCores.Float64) &&
				(cumulativeObjectCount <= r1.maxSupportedNumTables.Int64) &&
				(cumulativeSizeSum <= r1.maxColocatedSizeSupported.Float64) {
				colocatedObjects = append(colocatedObjects, table)
				// append all indexes into colocated
				colocatedObjects = append(colocatedObjects, indexesOfTable...)

			} else {
				allObjectsColocated = false
				if neededCores > r1.numCores.Float64 {
					currentReasoning = "Max throughput for instance type reached: " + currentReasoning +
						fmt.Sprintf("support %v objects which require %v select ops/sec and %v insert "+
							"ops/sec in total. ", len(colocatedObjects),
							cumulativeSelectOpsPerSec-table.ReadsPerSec-indexReads,
							cumulativeInsertOpsPerSec-table.WritesPerSec-indexWrites)
				}
				if cumulativeObjectCount > r1.maxSupportedNumTables.Int64 {
					currentReasoning = "Max supported number of tables reached: " + currentReasoning +
						fmt.Sprintf("support %v objects as colocated. ", len(colocatedObjects))
				}
				if cumulativeSizeSum > r1.maxColocatedSizeSupported.Float64 {
					currentReasoning = "Max colocated database size reached: " + currentReasoning +
						fmt.Sprintf("support %v objects with %0.4f GB size as colocated. ",
							len(colocatedObjects), cumulativeSizeSum-objectTotalSize)
				}

				if previousCores.numCores.Float64 != -1 {
					return append(sourceTableMetadata, sourceIndexMetadata...), totalSourceDBSize, previousCores, nil, previousReasoning, nil
				} else {
					var shardedObjects []SourceDBMetadata
					var cumulativeSizeSharded float64 = 0
					var cumulativeReadsSharded int64 = 0
					var cumulativeWritesSharded int64 = 0

					for _, remainingTable := range sourceTableMetadata[i:] {
						shardedObjects = append(shardedObjects, remainingTable)
						// fetch all associated indexes
						indexesOfShardedTable, indexesSizeSumSharded, cumulativeSelectsIdx, cumulativeInsertsIdx :=
							checkAndFetchIndexes(remainingTable, sourceIndexMetadata)
						shardedObjects = append(shardedObjects, indexesOfShardedTable...)
						cumulativeSizeSharded += remainingTable.Size + indexesSizeSumSharded
						cumulativeReadsSharded += remainingTable.ReadsPerSec + cumulativeSelectsIdx
						cumulativeWritesSharded += remainingTable.WritesPerSec + cumulativeInsertsIdx
					}
					currentReasoning += fmt.Sprintf("Rest %v objects with %0.4f GB size and %v select ops/sec and"+
						" %v write ops/sec requirement need to be migrated as sharded.", len(shardedObjects),
						cumulativeSizeSharded, cumulativeReadsSharded, cumulativeWritesSharded)
					return colocatedObjects, cumulativeSizeSum - objectTotalSize, r1, shardedObjects, currentReasoning, nil
				}
			}
		}
		previousReasoning = currentReasoning
		if allObjectsColocated {
			previousReasoning += fmt.Sprintf("fit all %v objects of size %0.4fGB as colocated",
				len(sourceTableMetadata)+len(sourceIndexMetadata), totalSourceDBSize)
		}
		previousCores = r1
	}
	return append(sourceTableMetadata, sourceIndexMetadata...), totalSourceDBSize, previousCores, nil, previousReasoning, nil
}

/*
generateSizingRecommendations generates sizing recommendations for sharded objects based on various factors such as
table limits and throughput data. It checks table limits to determine the maximum number of supported CPU cores for the
given number of sharded objects. The function calculates aggregate select and write throughput data for the sharded
objects and then analyzes the throughput data to generate additional recommendations. It calculates optimal connections
per node, considering the supported CPU cores, and updates the global report structure with the calculated values.
Additionally, the function estimates the import time for sharded objects and adds it to the total import time in
the report.
Parameters:

	shardedObjectMetadata: A slice containing metadata for sharded objects.
	numColocatedObjects: Total number of colocated objects.
	coresToUse: cores to use for sizing recommendation.

Returns:

	float64: recommended number of nodes
	int64: optimal select connections per node recommendation
	int64: optimal insert connections per node recommendation
*/
func generateSizingRecommendations(shardedObjectMetadata []SourceDBMetadata, numColocatedObjects int,
	coresToUse ExpDataColocatedLimit) (float64, int64, int64, error) {
	// 3 is the minimum number of nodes recommended
	var numNodes float64 = 3
	var optimalSelectConnectionsPerNode int64 = 0
	var optimalInsertConnectionsPerNode int64 = 0

	if len(shardedObjectMetadata) > 0 {
		// table limit check
		arrayOfSupportedCores, err :=
			checkTableLimits(len(shardedObjectMetadata), coresToUse.numCores.Float64, numColocatedObjects)
		if err != nil {
			return numNodes, 0, 0, fmt.Errorf("check table limits: %w", err)
		}
		// calculate throughput data
		var sumSourceSelectThroughput int64 = 0
		var sumSourceWriteThroughput int64 = 0
		for _, metadata := range shardedObjectMetadata {
			sumSourceSelectThroughput += metadata.ReadsPerSec
			sumSourceWriteThroughput += metadata.WritesPerSec
		}

		throughputData, err := getThroughputData(sumSourceSelectThroughput, sumSourceWriteThroughput, coresToUse.numCores.Float64)
		if err != nil {
			return numNodes, 0, 0, fmt.Errorf("get throughput data: %w", err)
		}
		numNodes = math.Max(throughputData+1, numNodes)
		// calculate impact of table count : not in this version
		//values = calculateTableCountImpact(values, int64(len(srcMeta)))

		// calculate impact of horizontal scaling : not in this version
		//calculateImpactOfHorizontalScaling(values)

		// get connections per core
		optimalSelectConnectionsPerNode, optimalInsertConnectionsPerNode, err = getConnectionsPerCore(arrayOfSupportedCores[0])
		if err != nil {
			return numNodes, 0, 0, fmt.Errorf("get connections per core: %w", err)
		}
	}
	return numNodes, optimalSelectConnectionsPerNode, optimalInsertConnectionsPerNode, nil
}

/*
getConnectionsPerCore retrieves the optimal number of select and insert connections per core based on the specified
number of CPU cores. It constructs a SQL query to retrieve the select and insert connection values from a database
table based on the maximum throughput dimension and the provided number of CPU cores. The function returns the optimal
number of select and insert connections per core obtained from the query.
Parameters:

	numCores: The number of CPU cores for which optimal connections per core are to be retrieved.

Returns:

	int64: The optimal number of select connections per core.
	int64: The optimal number of insert connections per core.
*/
func getConnectionsPerCore(numCores int) (int64, int64, error) {
	var selectConnectionsPerCore, insertConnectionsPerCore int64
	selectQuery := fmt.Sprintf(`
		SELECT select_conn_per_node, 
			   insert_conn_per_node 
		FROM %v 
		WHERE dimension LIKE 'MaxThroughput' 
			AND num_cores = ? 
		ORDER BY num_nodes 
		LIMIT 1
	`, SHARDED_SIZING_TABLE)
	row := ExperimentDB.QueryRow(selectQuery, numCores)

	if err := row.Scan(&selectConnectionsPerCore, &insertConnectionsPerCore); err != nil {
		if err == sql.ErrNoRows {
			log.Errorf("no records found in Experiment data: %v", SHARDED_SIZING_TABLE)
		} else {
			return 0, 0, fmt.Errorf("error while fetching connecctions per core with query [%s]: %w", selectQuery, err)
		}
	}

	return selectConnectionsPerCore, insertConnectionsPerCore, nil
}

/*
checkTableLimits queries a database table to find the number of CPU cores required to support a specified number
of tables while ensuring table limits are not exceeded. It constructs a SQL query to retrieve the number of CPU cores
from a sharded_sizing table where the number of tables exceeds a given threshold and the number of CPU cores meets
or exceeds a specified value per instance. The query filters results based on a dimension criterion and orders the
retrieved data by the number of CPU cores. The function returns a slice containing the CPU core values that satisfy
the specified conditions. If no suitable CPU core values are found, it returns nil.
Parameters:

	numShardedObjects: The required number of tables for which CPU core limits need to be checked.
	numColocatedObjects: The number of colocated objects. Only used for reasoning in this function.

Returns:

	[]int: A slice containing the number of CPU cores that meet the criteria for supporting the specified number of tables.
*/
func checkTableLimits(numShardedObjects int, coresPerNode float64, numColocatedObjects int) ([]int, error) {
	// added num_cores >= VCPUPerInstance from colo recommendation as that is the starting point
	selectQuery := fmt.Sprintf(`
			SELECT num_cores 
			FROM %s 
			WHERE num_tables > ? 
				AND num_cores >= ? 
				AND dimension LIKE '%%TableLimits-3nodeRF=3%%' 
			ORDER BY num_cores
		`, SHARDED_SIZING_TABLE)
	rows, err := ExperimentDB.Query(selectQuery, numShardedObjects, coresPerNode)
	if err != nil {
		return nil, fmt.Errorf("error while fetching cores info with query [%s]: %w", selectQuery, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close result set for query: [%s]", selectQuery)
		}
	}()

	var valuesArray []int
	for rows.Next() {

		var numCores int
		if err := rows.Scan(&numCores); err != nil {
			return nil, fmt.Errorf("error while fetching cores info with query [%s]: %w", selectQuery, err)
		}
		valuesArray = append(valuesArray, numCores)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error while fetching cores info with query [%s]: %w", selectQuery, err)
	}

	if len(valuesArray) > 0 {
		return valuesArray, nil
	} else {
		return nil, fmt.Errorf("cannot support %v colocated objects and %v sharded objects",
			numColocatedObjects, numShardedObjects)
	}
}

/*
getThroughputData calculates the required number of nodes based on the provided select and write throughput metrics.
It constructs a SQL query to retrieve throughput data from a database table and processes the results to determine
the total number of CPU cores needed for both read and write operations. The function then adjusts the total number
of nodes in the final report based on the calculated CPU core count and the specified number of virtual CPUs per instance.
Parameters:

	selectThroughput: The select throughput metric representing the number of read operations per second.
	writeThroughput: The write throughput metric representing the number of write operations per second.
	numCores: number of cores to be used for calculating nodes to be added

Returns:

	float64: recommended number of nodes to be added
*/
func getThroughputData(selectThroughput int64, writeThroughput int64, numCores float64) (float64, error) {

	var currentRow [5]float64
	var nodesToAdd float64 = 0
	selectQuery := fmt.Sprintf(`
		SELECT foo.* 
		FROM (
			SELECT id, 
				   ROUND((? / inserts_per_core) + 0.5) AS insert_total_cores,
				   ROUND((? / selects_per_core) + 0.5) AS select_total_cores, 
				   num_cores, 
				   num_nodes 
			FROM %s 
			WHERE dimension = 'MaxThroughput' 
				AND num_cores >= ?
		) AS foo 
		ORDER BY num_cores DESC;
	`, SHARDED_SIZING_TABLE)
	rows, err := ExperimentDB.Query(selectQuery, writeThroughput, selectThroughput, numCores)
	if err != nil {
		return 0.0, fmt.Errorf("error while fetching throughput info with query [%s]: %w", selectQuery, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close result set for query: [%s]", selectQuery)
		}
	}()

	for rows.Next() {
		if err := rows.Scan(&currentRow[0], &currentRow[1], &currentRow[2], &currentRow[3], &currentRow[4]); err != nil {
			return 0.0, fmt.Errorf("error while fetching throughput info with query [%s]: %w", selectQuery, err)
		}
		nodesNeededForRow := math.Max(math.Ceil((currentRow[1]+currentRow[2])/numCores), 1)
		if nodesNeededForRow <= 3 {
			nodesToAdd = nodesNeededForRow
		} else if nodesNeededForRow <= 5 {
			return nodesToAdd, nil
		} else {
			return nodesNeededForRow, nil
		}
	}
	return nodesToAdd, nil
}

/*
calculateTimeTakenAndParallelThreadsForImport estimates the time taken for import of database objects based on their type, size,
and the specified CPU and memory configurations. It calculates the total size of the database objects to be migrated,
then queries experimental data to find import time estimates for similar object sizes and configurations. The
function adjusts the import time based on the ratio of the total size of the objects to be migrated to the maximum
size found in the experimental data. The import time is then converted from seconds to minutes and returned.
Parameters:

	objectType: A string indicating the type of database objects to be migrated (e.g., "colocated" or "sharded").
	dbObjects: A slice containing metadata for the database objects to be migrated.
	vCPUPerInstance: The number of virtual CPUs per instance used for import.
	memPerCore: The memory allocated per CPU core used for import.

Returns:

	float64: The estimated time taken for import in minutes.
	int64: Total parallel threads used for import.
*/
func calculateTimeTakenAndParallelThreadsForImport(tableName string, dbObjects []SourceDBMetadata,
	vCPUPerInstance float64, memPerCore float64) (float64, int64, error) {
	// the total size of colocated objects
	var size float64 = 0
	var timeTakenOfFetchedRow float64
	var maxSizeOfFetchedRow float64
	var parallelThreads int64
	for _, dbObject := range dbObjects {
		size += dbObject.Size
	}

	// find the rows in experiment data about the approx row matching the size
	selectQuery := fmt.Sprintf(`
		SELECT csv_size_gb, 
			   migration_time_secs, 
			   parallel_threads 
		FROM %v 
		WHERE num_cores = ? 
			AND mem_per_core = ? 
			AND csv_size_gb >= ? 
		UNION ALL 
		SELECT csv_size_gb, 
			   migration_time_secs, 
			   parallel_threads 
		FROM %v 
		WHERE csv_size_gb = (
			SELECT MAX(csv_size_gb) 
			FROM %v
			WHERE num_cores = ?
		) 
		LIMIT 1;
	`, tableName, tableName, tableName)
	row := ExperimentDB.QueryRow(selectQuery, vCPUPerInstance, memPerCore, size, vCPUPerInstance)

	if err := row.Scan(&maxSizeOfFetchedRow, &timeTakenOfFetchedRow, &parallelThreads); err != nil {
		if err == sql.ErrNoRows {
			log.Errorf("No rows were returned by the query to experiment table: %v", tableName)
		} else {
			return 0.0, 0, fmt.Errorf("error while fetching import time info with query [%s]: %w", selectQuery, err)
		}
	}

	importTime := ((timeTakenOfFetchedRow * size) / maxSizeOfFetchedRow) / 60
	return math.Ceil(importTime), parallelThreads, nil
}

/*
getSourceMetadata retrieves metadata for source database tables and indexes along with the total size of the source
database.
Returns:

	[]SourceDBMetadata: Metadata for source database tables.
	[]SourceDBMetadata: Metadata for source database indexes.
	float64: The total size of the source database in gigabytes.
*/
func getSourceMetadata(sourceDB *sql.DB) ([]SourceDBMetadata, []SourceDBMetadata, float64, error) {
	query := fmt.Sprintf(`
		SELECT schema_name, 
			   object_name, 
			   row_count, 
			   reads_per_second, 
			   writes_per_second, 
			   is_index, 
			   parent_table_name, 
			   size_in_bytes 
		FROM %v 
		ORDER BY size_in_bytes ASC
	`, GetTableIndexStatName())
	rows, err := sourceDB.Query(query)
	if err != nil {
		return nil, nil, 0.0, fmt.Errorf("failed to query source metadata with query [%s]: %w", query, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close result set for query: [%s]", query)
		}
	}()

	// Iterate over the rows
	var sourceTableMetadata []SourceDBMetadata
	var sourceIndexMetadata []SourceDBMetadata

	var totalSourceDBSize float64 = 0
	for rows.Next() {
		var metadata SourceDBMetadata
		if err := rows.Scan(&metadata.SchemaName, &metadata.ObjectName, &metadata.RowCount, &metadata.ReadsPerSec, &metadata.WritesPerSec,
			&metadata.IsIndex, &metadata.ParentTableName, &metadata.Size); err != nil {
			return nil, nil, 0.0, fmt.Errorf("failed to read from result set of query source metadata [%s]: %w", query, err)
		}
		// convert bytes to GB
		metadata.Size = utils.BytesToGB(metadata.Size)
		if metadata.IsIndex {
			sourceIndexMetadata = append(sourceIndexMetadata, metadata)
		} else {
			sourceTableMetadata = append(sourceTableMetadata, metadata)
		}
		totalSourceDBSize += metadata.Size
	}
	if err := rows.Err(); err != nil {
		return nil, nil, 0.0, fmt.Errorf("failed to query source metadata with query [%s]: %w", query, err)
	}
	if err := sourceDB.Close(); err != nil {
		log.Warnf("failed to close connection to sourceDB metadata")
	}
	return sourceTableMetadata, sourceIndexMetadata, totalSourceDBSize, nil
}

func createConnectionToExperimentData() error {
	filePath, err := getExperimentFile()
	if err != nil {
		return fmt.Errorf("failed to get experiment file: %w", err)
	}
	DbConnection, err := utils.ConnectToSqliteDatabase(filePath)
	if err != nil {
		return fmt.Errorf("failed to connect to experiment data database: %w", err)
	}
	ExperimentDB = DbConnection
	return nil
}

func getExperimentFile() (string, error) {
	fetchedFromRemote := false
	if checkInternetAccess() {
		existsOnRemote, err := checkAndDownloadFileExistsOnRemoteRepo()
		if err != nil {
			return "", err
		}
		if existsOnRemote {
			fetchedFromRemote = true
		}
	}
	if !fetchedFromRemote {
		err := os.WriteFile(filepath.Join(AssessmentMetadataDir, EXPERIMENT_DATA_FILENAME), experimentData20240, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write experiment data file: %w", err)
		}
	}

	return filepath.Join(AssessmentMetadataDir, EXPERIMENT_DATA_FILENAME), nil
}

func checkAndDownloadFileExistsOnRemoteRepo() (bool, error) {
	// check if the file exists on remote github repository using the raw link
	remotePath := GITHUB_RAW_LINK + "/" + EXPERIMENT_DATA_FILENAME
	resp, err := http.Get(remotePath)
	if err != nil {
		return false, fmt.Errorf("failed to make GET request: %w", err)
	}
	defer func() {
		if closingErr := resp.Body.Close(); closingErr != nil {
			log.Warnf("failed to close the response body for GET api")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	downloadPath := filepath.Join(AssessmentMetadataDir, EXPERIMENT_DATA_FILENAME)
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	err = os.WriteFile(downloadPath, bodyBytes, 0644)
	if err != nil {
		return false, fmt.Errorf("failed to write file: %w", err)
	}

	return true, nil
}

/*
checkAndFetchIndexes checks for indexes associated with a specific database table and fetches their metadata.
It iterates through a slice of index metadata and selects indexes that belong to the specified table by comparing
their parent table names. The function returns a slice containing metadata for indexes associated with the table
and the total size of those indexes.
Parameters:

	table: Metadata for the database table for which indexes are to be checked.
	indexes: A slice containing metadata for all indexes in the database.

Returns:

	[]SourceDBMetadata: Metadata for indexes associated with the specified table.
	float64: The total size of indexes associated with the specified table.
	int64 : sum of read ops per second for all indexes of the table
	int64 : sum of write ops per second for all indexes of the table
*/
func checkAndFetchIndexes(table SourceDBMetadata, indexes []SourceDBMetadata) ([]SourceDBMetadata, float64, int64, int64) {
	indexesOfTable := make([]SourceDBMetadata, 0)
	var indexesSizeSum float64 = 0
	var cumulativeSelectOpsPerSecIdx int64 = 0
	var cumulativeInsertOpsPerSecIdx int64 = 0
	for _, index := range indexes {
		if index.ParentTableName.Valid && (index.ParentTableName.String == (table.SchemaName + "." + table.ObjectName)) {
			indexesOfTable = append(indexesOfTable, index)
			indexesSizeSum += index.Size
			cumulativeSelectOpsPerSecIdx += index.ReadsPerSec
			cumulativeInsertOpsPerSecIdx += index.WritesPerSec
		}
	}

	return indexesOfTable, indexesSizeSum, cumulativeSelectOpsPerSecIdx, cumulativeInsertOpsPerSecIdx
}
