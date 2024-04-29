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

var DB *sql.DB
var SourceMetaDB *sql.DB
var fileName = "/yb_2024_0_source.db"

//go:embed resources/yb_2024_0_source.db
var experimentData20240 []byte

func SizingAssessment(assessmentMetadataDir string) error {
	log.Infof("loading metadata files for sharding assessment")
	sourceTableMetadata, sourceIndexMetadata, totalSourceDBSize, err := loadSourceMetadata()
	if err != nil {
		return fmt.Errorf("failed to load source metadata: %w", err)
	}

	createConnectionToExperimentData(assessmentMetadataDir)
	colocatedObjects, colocatedObjectsSize, coresToUse, shardedObjects, err :=
		generateShardingRecommendations(sourceTableMetadata, sourceIndexMetadata, totalSourceDBSize)
	if err != nil {
		return fmt.Errorf("generate sharding recommendations: %w", err)
	}

	// only use the remaining sharded objects and its size for further recommendation processing
	numNodes, optimalSelectConnections, optimalInsertConnections, err :=
		generateSizingRecommendations(shardedObjects, totalSourceDBSize-colocatedObjectsSize, coresToUse)
	if err != nil {
		return fmt.Errorf("generate sizing recommendations: %w", err)
	}

	// calculate time taken for colocated migration
	migrationTimeForColocatedObjects, parallelThreadsColocated, err :=
		calculateTimeTakenAndParallelThreadsForMigration("colocated", colocatedObjects,
			coresToUse.numCores.Float64, coresToUse.memPerCore.Float64)
	if err != nil {
		return fmt.Errorf("calculate time taken for colocated migration: %w", err)
	}

	// calculate time taken for sharded migration
	migrationTimeForShardedObjects, parallelThreadsSharded, err :=
		calculateTimeTakenAndParallelThreadsForMigration("sharded", shardedObjects,
			coresToUse.numCores.Float64, coresToUse.memPerCore.Float64)
	if err != nil {
		return fmt.Errorf("calculate time taken for sharded migration: %w", err)
	}

	// reasoning for colocation/sharding
	reasoning := fmt.Sprintf("Recommended instance with %vvCPU and %vGiB memory could fit: ",
		coresToUse.numCores.Float64, coresToUse.numCores.Float64*coresToUse.memPerCore.Float64)
	if len(shardedObjects) > 0 {
		reasoning += fmt.Sprintf("%v objects with size %0.3f GB as colocated. "+
			"Rest %v objects of size %0.3f GB can be imported as sharded tables",
			len(colocatedObjects), colocatedObjectsSize, len(shardedObjects), totalSourceDBSize-colocatedObjectsSize)
	} else {
		reasoning += fmt.Sprintf("All %v objects of size %0.3fGB as colocated",
			len(sourceTableMetadata)+len(sourceIndexMetadata), totalSourceDBSize)
	}

	//assessmentMetadataDir

	SizingReport = &SizingAssessmentReport{
		ColocatedTables:                 fetchObjectNames(colocatedObjects),
		ColocatedReasoning:              reasoning,
		ShardedTables:                   fetchObjectNames(shardedObjects),
		NumNodes:                        numNodes,
		VCPUsPerInstance:                coresToUse.numCores.Float64,
		MemoryPerInstance:               coresToUse.numCores.Float64 * coresToUse.memPerCore.Float64,
		MigrationTimeTakenInMin:         migrationTimeForColocatedObjects + migrationTimeForShardedObjects,
		ParallelVoyagerThreadsSharded:   parallelThreadsSharded,
		ParallelVoyagerThreadsColocated: parallelThreadsColocated,
		OptimalSelectConnectionsPerNode: optimalSelectConnections,
		OptimalInsertConnectionsPerNode: optimalInsertConnections,
	}
	return nil
}

func fetchObjectNames(dbObjects []SourceDBMetadata) []string {
	var names []string
	for _, dbObject := range dbObjects {
		names = append(names, dbObject.SchemaName+"."+dbObject.ObjectName)
	}
	return names
}

/*
loadSourceMetadata connects to the assessment metadata of the source database and generates the slice of objects
for tables and indexes. It also returns the total size of source db in GB
Returns:

	[]SourceDBMetadata: all table objects from source db
	[]SourceDBMetadata: all index objects from source db
	float64: total size of source db
*/
func loadSourceMetadata() ([]SourceDBMetadata, []SourceDBMetadata, float64, error) {
	err := ConnectSourceMetaDatabase(GetDBFilePath())
	if err != nil {
		return nil, nil, 0.0, fmt.Errorf("cannot connect to source metadata database: %w", err)
	}
	return getSourceMetadata()
}

/*
generateShardingRecommendations analyzes source database metadata to generate sharding recommendations.
It calculates the required number of nodes, CPU cores, and memory for colocated objects. It tries to fit in all source
objects(in increasing order of size) as colocated and scales vertically to achieve it going upto 16 cores.
Then onwards, we make the rest of the objects as sharded. The function returns a slice containing metadata for sharded
objects and the total size of sharded objects. Additionally, it updates a global report structure with information on
colocated and sharded tables, recommended instance configurations, and migration time estimates for colocated object
migration.
Parameters:

	sourceTableMetadata: A slice containing metadata for source database tables.
	sourceIndexMetadata: A slice containing metadata for source database indexes.
	totalSourceDBSize: The total size of the source database in gigabytes.

Returns:

	[]SourceDBMetadata: A slice containing metadata for sharded objects. Used for sharding recommendations.
	float64: The total size of sharded objects in gigabytes. Used for sharding recommendations.
	ExpDataColocatedLimit: Information about the instance type to use for the experiment
	SourceDBMetadata[]: list of sharded objects
*/

func generateShardingRecommendations(sourceTableMetadata []SourceDBMetadata, sourceIndexMetadata []SourceDBMetadata,
	totalSourceDBSize float64) ([]SourceDBMetadata, float64, ExpDataColocatedLimit, []SourceDBMetadata, error) {
	previousCores := ExpDataColocatedLimit{
		maxColocatedSizeSupported:  sql.NullFloat64{Float64: -1, Valid: true},
		numCores:                   sql.NullFloat64{Float64: -1, Valid: true},
		memPerCore:                 sql.NullFloat64{Float64: -1, Valid: true},
		maxSupportedNumTables:      sql.NullInt64{Int64: -1, Valid: true},
		minSupportedNumTables:      sql.NullFloat64{Float64: -1, Valid: true},
		maxSupportedSelectsPerCore: sql.NullFloat64{Float64: -1, Valid: true},
		maxSupportedInsertsPerCore: sql.NullFloat64{Float64: -1, Valid: true},
	}

	query := "SELECT max_colocated_db_size_gb,num_cores,mem_per_core,max_num_tables,min_num_tables,max_selects_per_core,max_inserts_per_core FROM colocated_limits order by num_cores DESC"
	rows, err := DB.Query(query)
	if err != nil {
		return nil, 0.0, ExpDataColocatedLimit{}, nil, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", query, err)
	}
	defer rows.Close()

	for rows.Next() {
		var r1 ExpDataColocatedLimit
		var cumulativeObjectCount int64 = 0
		var cumulativeSelectOpsPerSec int64 = 0
		var cumulativeInsertOpsPerSec int64 = 0
		var cumulativeSizeSum float64 = 0
		var colocatedObjects []SourceDBMetadata

		if err := rows.Scan(&r1.maxColocatedSizeSupported, &r1.numCores, &r1.memPerCore, &r1.maxSupportedNumTables,
			&r1.minSupportedNumTables, &r1.maxSupportedSelectsPerCore, &r1.maxSupportedInsertsPerCore); err != nil {
			return nil, 0.0, ExpDataColocatedLimit{}, nil, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", query, err)
		}
		for i, table := range sourceTableMetadata {
			// check if current table has any indexes and fetch all indexes
			indexesOfTable, indexesSizeSum, indexReads, indexWrites := checkAndFetchIndexes(table, sourceIndexMetadata)
			cumulativeObjectCount += int64(len(indexesOfTable)) + 1

			cumulativeSizeSum += table.Size + indexesSizeSum
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
				if previousCores.numCores.Float64 != -1 {
					return append(sourceTableMetadata, sourceIndexMetadata...), totalSourceDBSize, previousCores, nil, nil
				} else {
					var shardedObjects []SourceDBMetadata

					for _, remainingTable := range sourceTableMetadata[i:] {
						shardedObjects = append(shardedObjects, remainingTable)
						// fetch all associated indexes
						indexesOfShardedTable, _, _, _ := checkAndFetchIndexes(remainingTable, sourceIndexMetadata)
						shardedObjects = append(shardedObjects, indexesOfShardedTable...)
					}

					return colocatedObjects, cumulativeSizeSum, r1, shardedObjects, nil
				}
			}
		}
		previousCores = r1
	}
	return append(sourceTableMetadata, sourceIndexMetadata...), totalSourceDBSize, previousCores, nil, nil
}

/*
generateSizingRecommendations generates sizing recommendations for sharded objects based on various factors such as
table limits and throughput data. It checks table limits to determine the maximum number of supported CPU cores for the
given number of sharded objects. The function calculates aggregate select and write throughput data for the sharded
objects and then analyzes the throughput data to generate additional recommendations. It calculates optimal connections
per node, considering the supported CPU cores, and updates the global report structure with the calculated values.
Additionally, the function estimates the migration time for sharded objects and adds it to the total migration time in
the report.
Parameters:

	shardedObjectMetadata: A slice containing metadata for sharded objects.
	shardedObjectsSize: The total size of sharded objects in gigabytes.

Returns:

	float64: recommended number of nodes
	int64: optimal select connections per node recommendation
	int64: optimal insert connections per node recommendation
*/
func generateSizingRecommendations(shardedObjectMetadata []SourceDBMetadata, shardedObjectsSize float64,
	coresToUse ExpDataColocatedLimit) (float64, int64, int64, error) {
	var numNodes float64 = 0
	var optimalSelectConnectionsPerNode int64 = 0
	var optimalInsertConnectionsPerNode int64 = 0

	if len(shardedObjectMetadata) > 0 {
		// table limit check
		arrayOfSupportedCores, err := checkTableLimits(len(shardedObjectMetadata), coresToUse.numCores.Float64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("check table limits: %w", err)
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
			return 0, 0, 0, fmt.Errorf("get throughput data: %w", err)
		}
		numNodes = math.Max(throughputData+1, 3)
		// calculate impact of table count : not in this version
		//values = calculateTableCountImpact(values, int64(len(srcMeta)))

		// calculate impact of horizontal scaling : not in this version
		//calculateImpactOfHorizontalScaling(values)

		// get connections per core
		optimalSelectConnectionsPerNode, optimalInsertConnectionsPerNode, err = getConnectionsPerCore(arrayOfSupportedCores[0])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("get connections per core: %w", err)
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
	selectQuery := "select select_conn_per_node, insert_conn_per_node from sharded_sizing " +
		"where dimension like 'MaxThroughput' and num_cores = ? order by num_nodes limit 1"
	row := DB.QueryRow(selectQuery, numCores)

	if err := row.Scan(&selectConnectionsPerCore, &insertConnectionsPerCore); err != nil {
		if err == sql.ErrNoRows {
			log.Errorf("no records found in Experiment data: sharded_sizing")
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

	reqTables: The required number of tables for which CPU core limits need to be checked.

Returns:

	[]int: A slice containing the number of CPU cores that meet the criteria for supporting the specified number of tables.
*/
func checkTableLimits(sourceDBObjects int, coresPerNode float64) ([]int, error) {
	// added num_cores >= VCPUPerInstance from colo recommendation as that is the starting point
	selectQuery := "SELECT num_cores FROM sharded_sizing WHERE num_tables > ? AND num_cores >= ? AND " +
		"dimension LIKE '%TableLimits-3nodeRF=3%' ORDER BY num_cores"
	rows, err := DB.Query(selectQuery, sourceDBObjects, coresPerNode)
	if err != nil {
		return nil, fmt.Errorf("error while fetching cores info with query [%s]: %w", selectQuery, err)
	}
	defer rows.Close()

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
		return nil, fmt.Errorf("Cannot support %v objects", sourceDBObjects)
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

	selectQuery := "SELECT foo.* FROM (SELECT id, ROUND((? / inserts_per_core) + 0.5) AS insert_total_cores," +
		"ROUND((? / selects_per_core) + 0.5) AS select_total_cores, num_cores, num_nodes FROM sharded_sizing " +
		"WHERE dimension = 'MaxThroughput' AND num_cores >= ?) AS foo ORDER BY num_cores DESC;"
	rows, err := DB.Query(selectQuery, writeThroughput, selectThroughput, numCores)
	if err != nil {
		return 0.0, fmt.Errorf("error while fetching throughput info with query [%s]: %w", selectQuery, err)
	}
	defer rows.Close()

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
calculateTimeTakenAndParallelThreadsForMigration estimates the time taken for migration of database objects based on their type, size,
and the specified CPU and memory configurations. It calculates the total size of the database objects to be migrated,
then queries experimental data to find migration time estimates for similar object sizes and configurations. The
function adjusts the migration time based on the ratio of the total size of the objects to be migrated to the maximum
size found in the experimental data. The migration time is then converted from seconds to minutes and returned.
Parameters:

	objectType: A string indicating the type of database objects to be migrated (e.g., "colocated" or "sharded").
	dbObjects: A slice containing metadata for the database objects to be migrated.
	vCPUPerInstance: The number of virtual CPUs per instance used for migration.
	memPerCore: The memory allocated per CPU core used for migration.

Returns:

	float64: The estimated time taken for migration in minutes.
	int64: Total parallel threads used for migration.
*/
func calculateTimeTakenAndParallelThreadsForMigration(objectType string, dbObjects []SourceDBMetadata,
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
	selectQuery := fmt.Sprintf("SELECT csv_size_gb, migration_time_secs, parallel_threads from %v_load_time where "+
		"num_cores = ? and mem_per_core = ? and csv_size_gb >= ? UNION ALL "+
		"SELECT csv_size_gb, migration_time_secs, parallel_threads from sharded_load_time WHERE csv_size_gb = (SELECT MAX(csv_size_gb) "+
		"FROM sharded_load_time) LIMIT 1;", objectType)
	row := DB.QueryRow(selectQuery, vCPUPerInstance, memPerCore, size)

	if err := row.Scan(&maxSizeOfFetchedRow, &timeTakenOfFetchedRow, &parallelThreads); err != nil {
		if err == sql.ErrNoRows {
			log.Errorf("No rows were returned by the query to experiment table: sharded_load_time")
		} else {
			return 0.0, 0, fmt.Errorf("error while fetching migration time info with query [%s]: %w", selectQuery, err)
		}
	}

	migrationTime := ((timeTakenOfFetchedRow * size) / maxSizeOfFetchedRow) / 60
	return math.Ceil(migrationTime), parallelThreads, nil
}

func ConnectSourceMetaDatabase(file string) error {
	db, err := sql.Open("sqlite3", file)
	if err != nil {
		return err
	}
	SourceMetaDB = db
	return nil
}

func ConnectExperimentDataDatabase(file string) error {
	db, err := sql.Open("sqlite3", file)
	if err != nil {
		return err
	}
	DB = db
	return nil
}

/*
getSourceMetadata retrieves metadata for source database tables and indexes along with the total size of the source
database.
Returns:

	[]SourceDBMetadata: Metadata for source database tables.
	[]SourceDBMetadata: Metadata for source database indexes.
	float64: The total size of the source database in gigabytes.
*/
func getSourceMetadata() ([]SourceDBMetadata, []SourceDBMetadata, float64, error) {
	query := fmt.Sprintf("SELECT schema_name, object_name,row_count,reads_per_second,writes_per_second,"+
		"is_index,parent_table_name,size_in_bytes FROM %v ORDER BY size_in_bytes ASC", GetTableIndexStatName())
	rows, err := SourceMetaDB.Query(query)
	if err != nil {
		return nil, nil, 0.0, fmt.Errorf("failed to query source metadata with query [%s]: %w", query, err)
	}
	defer rows.Close()

	// Iterate over the rows
	var sourceTableMetadata []SourceDBMetadata
	var sourceIndexMetadata []SourceDBMetadata

	var totalSourceDBSize float64 = 0
	for rows.Next() {
		var metadata SourceDBMetadata
		if err := rows.Scan(&metadata.SchemaName, &metadata.ObjectName, &metadata.RowCount, &metadata.ReadsPerSec, &metadata.WritesPerSec,
			&metadata.IsIndex, &metadata.ParentTableName, &metadata.Size); err != nil {
			log.Fatal(err)
		}
		// convert bytes to GB
		metadata.Size = bytesToGB(metadata.Size)
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
	SourceMetaDB.Close()
	return sourceTableMetadata, sourceIndexMetadata, totalSourceDBSize, nil
}

/*
bytesToGB function converts the size of the source object from bytes to GB as it is required for further calculation
Parameters:

	sizeInBytes: size of source object in bytes

Returns:

	sizeInGB: size of source object in gigabytes
*/
func bytesToGB(sizeInBytes float64) float64 {
	sizeInGB := sizeInBytes / (1024 * 1024 * 1024)
	// any value less than a 1 MB is considered as 0
	if sizeInGB < 0.001 {
		return 0
	}
	return sizeInGB
}

func createConnectionToExperimentData(assessmentMetadataDir string) error {
	filePath, err := getExperimentFile(assessmentMetadataDir)
	if err != nil {
		return fmt.Errorf("failed to get experiment file: %w", err)
	}
	err = ConnectExperimentDataDatabase(filePath)
	if err != nil {
		return fmt.Errorf("failed to connect to experiment data database: %w", err)
	}
	return nil
}

func getExperimentFile(assessmentMetadataDir string) (string, error) {
	fetchedFromRemote := false
	if checkInternetAccess() {
		existsOnRemote, err := checkAndDownloadFileExistsOnRemoteRepo(assessmentMetadataDir)
		if err != nil {
			return "", err
		}
		if existsOnRemote {
			fetchedFromRemote = true
		}
	}
	if !fetchedFromRemote {
		err := os.WriteFile(assessmentMetadataDir+fileName, experimentData20240, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write experiment data file: %w", err)
		}
	}
	return assessmentMetadataDir + fileName, nil
}

func checkAndDownloadFileExistsOnRemoteRepo(assessmentMetadataDir string) (bool, error) {
	remotePath :=
		"https://raw.githubusercontent.com/yugabyte/yb-voyager/main/yb-voyager/src/migassessment/resources" + fileName
	resp, err := http.Get(remotePath)
	if err != nil {
		return false, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Errorf("failed to close response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		return false, nil
	} else {
		downloadPath := assessmentMetadataDir + fileName
		out, err := os.Create(downloadPath)
		if err != nil {
			return false, err
		}
		defer func(out *os.File) {
			err := out.Close()
			if err != nil {
				log.Errorf("failed to close file: %v", err)
			}
		}(out)
		_, err = io.Copy(out, resp.Body)
		return err == nil, err
	}
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
