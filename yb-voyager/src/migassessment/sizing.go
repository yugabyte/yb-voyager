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
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type SourceDBMetadata struct {
	SchemaName      string         `json:"schema_name"`
	ObjectName      string         `json:"object_name"`
	RowCount        sql.NullInt64  `json:"row_count,string"`
	ReadsPerSec     int64          `json:"reads,string"`
	WritesPerSec    int64          `json:"writes,string"`
	IsIndex         bool           `json:"isIndex,string"`
	ParentTableName sql.NullString `json:"parent_table_name"`
	Size            float64        `json:"size,string"`
}

var baseDownloadPath = "src/migassessment/resources/remote/"
var DB *sql.DB
var SourceMetaDB *sql.DB

func SizingAssessment() error {
	log.Infof("loading metadata files for sharding assessment")
	sourceTableMetadata, sourceIndexMetadata, totalSourceDBSize := loadSourceMetadata()
	createConnectionToExperimentData(assessmentParams.TargetYBVersion)
	shardedObjects, shardedObjectsSize :=
		generateShardingRecommendations(sourceTableMetadata, sourceIndexMetadata, totalSourceDBSize)

	// only use the remaining sharded objects and its size for further recommendation processing
	generateSizingRecommendations(shardedObjects, shardedObjectsSize)
	return nil
}

/*
loadSourceMetadata connects to the assessment metadata of the source database and generates the slice of objects
for tables and indexes. It also returns the total size of source db in GB
Returns:

	[]SourceDBMetadata: all table objects from source db
	[]SourceDBMetadata: all index objects from source db
	float64: total size of source db
*/
func loadSourceMetadata() ([]SourceDBMetadata, []SourceDBMetadata, float64) {
	err := ConnectSourceMetaDatabase(GetDBFilePath())
	if err != nil {
		panic(err)
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
*/
func generateShardingRecommendations(sourceTableMetadata []SourceDBMetadata, sourceIndexMetadata []SourceDBMetadata,
	totalSourceDBSize float64) ([]SourceDBMetadata, float64) {
	var selectedRow [5]string // Assuming 5 columns in the table
	var cumulativeSum float64
	var colocatedObjects []SourceDBMetadata
	var shardedObjects []SourceDBMetadata
	var shardedObjectsSize float64 = 0
	var coloObjectNames []string
	var shardedObjectNames []string
	var index int
	numSourceObjects := len(sourceTableMetadata) + len(sourceIndexMetadata)
	row := DB.QueryRow("SELECT * FROM colocated_limits WHERE max_num_tables > ? AND min_num_tables <= ? AND "+
		"max_colocated_db_size_gb >= ? UNION ALL SELECT * FROM colocated_limits WHERE max_colocated_db_size_gb = "+
		"(SELECT MAX(max_colocated_db_size_gb) FROM colocated_limits) LIMIT 1;", numSourceObjects, numSourceObjects,
		totalSourceDBSize)

	var S float64
	if err := row.Scan(&selectedRow[0], &selectedRow[1], &selectedRow[2], &selectedRow[3], &selectedRow[4]); err != nil {
		if err == sql.ErrNoRows {
			log.Println("No rows were returned by the query.")
		} else {
			log.Fatal(err)
		}
	}

	S, _ = strconv.ParseFloat(selectedRow[0], 64)
	for i, key := range sourceTableMetadata {
		// check if current table has any indexes and fetch all indexes
		indexesOfTable, indexesSizeSum := checkAndFetchIndexes(key, sourceIndexMetadata)
		cumulativeSum += key.Size + indexesSizeSum
		if cumulativeSum > S {
			break
		}
		index = i
		colocatedObjects = append(colocatedObjects, key)
		coloObjectNames = append(coloObjectNames, key.SchemaName+"."+key.ObjectName)

		// append all indexes into colocated
		colocatedObjects = append(colocatedObjects, indexesOfTable...)
		// append all index names into colocated result
		for _, key := range indexesOfTable {
			coloObjectNames = append(coloObjectNames, key.SchemaName+"."+key.ObjectName)
		}
	}

	for _, key := range sourceTableMetadata[index+1:] {
		shardedObjectNames = append(shardedObjectNames, key.SchemaName+"."+key.ObjectName)
		shardedObjects = append(shardedObjects, key)
		// fetch all associated indexes
		indexesOfTable, indexesSizeSum := checkAndFetchIndexes(key, sourceIndexMetadata)
		shardedObjects = append(shardedObjects, indexesOfTable...)

		// add the sum of size of sharded objects
		shardedObjectsSize += key.Size + indexesSizeSum
		for _, key := range indexesOfTable {
			shardedObjectNames = append(shardedObjectNames, key.SchemaName+"."+key.ObjectName)
		}
	}

	vCPUPerInstance, _ := strconv.ParseFloat(selectedRow[1], 64)
	memPerCore, _ := strconv.ParseFloat(selectedRow[2], 64)

	reasoning := fmt.Sprintf("Recommended instance with %vvCPU and %vGiB memory could fit: ",
		vCPUPerInstance, vCPUPerInstance*memPerCore)

	if len(shardedObjectNames) > 0 {
		reasoning += fmt.Sprintf("%v objects with size %0.3f GB as colocated. "+
			"Rest %v objects of size %0.3f GB can be imported as sharded tables",
			len(coloObjectNames), totalSourceDBSize-shardedObjectsSize, len(shardedObjectNames), shardedObjectsSize)
	} else {
		reasoning += fmt.Sprintf("All %v objects of size %0.3fGB as colocated",
			len(sourceTableMetadata)+len(sourceIndexMetadata), totalSourceDBSize)
	}

	SizingReport = &AssessmentReport{
		ColocatedTables:         coloObjectNames,
		ColocatedReasoning:      reasoning,
		ShardedTables:           shardedObjectNames,
		NumNodes:                3,
		VCPUsPerInstance:        vCPUPerInstance,
		MemoryPerInstance:       vCPUPerInstance * memPerCore,
		MigrationTimeTakenInMin: calculateTimeTakenForMigration("colocated", colocatedObjects, vCPUPerInstance, memPerCore),
	}
	return shardedObjects, shardedObjectsSize
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
*/
func generateSizingRecommendations(shardedObjectMetadata []SourceDBMetadata, shardedObjectsSize float64) {
	if len(SizingReport.ShardedTables) > 0 {
		// table limit check
		arrayOfSupportedCores := checkTableLimits(len(shardedObjectMetadata))
		// calculate throughput data
		var sumSourceSelectThroughput int64 = 0
		var sumSourceWriteThroughput int64 = 0
		for _, metadata := range shardedObjectMetadata {
			sumSourceSelectThroughput += metadata.ReadsPerSec
			sumSourceWriteThroughput += metadata.WritesPerSec
		}

		getThroughputData(sumSourceSelectThroughput, sumSourceWriteThroughput)
		// calculate impact of table count : not in this version
		//values = calculateTableCountImpact(values, int64(len(srcMeta)))

		// calculate impact of horizontal scaling: not in this version
		//calculateImpactOfHorizontalScaling(values)

		// get connections per core
		SizingReport.OptimalSelectConnectionsPerNode, SizingReport.OptimalInsertConnectionsPerNode = getConnectionsPerCore(arrayOfSupportedCores[0])
		// calculate time taken for sharded migration
		migrationTimeForShardedObjects :=
			calculateTimeTakenForMigration("sharded", shardedObjectMetadata, SizingReport.VCPUsPerInstance, 4)
		SizingReport.MigrationTimeTakenInMin += migrationTimeForShardedObjects
	}
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
func getConnectionsPerCore(numCores int) (int64, int64) {
	var selectConnectionsPerCore, insertConnectionsPerCore int64
	selectQuery := "select select_conn_per_node, insert_conn_per_node from sharded_sizing " +
		"where dimension like 'MaxThroughput' and num_cores = ? order by num_nodes limit 1"
	row := DB.QueryRow(selectQuery, numCores)

	if err := row.Scan(&selectConnectionsPerCore, &insertConnectionsPerCore); err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No rows were returned by the query.")
		} else {
			log.Fatal(err)
		}
	}

	return selectConnectionsPerCore, insertConnectionsPerCore
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
func checkTableLimits(reqTables int) []int {
	// added num_cores >= VCPUPerInstance from colo recommendation as that is the starting point
	selectQuery := "SELECT num_cores FROM sharded_sizing WHERE num_tables > ? AND num_cores >= ? AND " +
		"dimension LIKE '%TableLimits-3nodeRF=3%' ORDER BY num_cores"
	rows, err := DB.Query(selectQuery, reqTables, SizingReport.VCPUsPerInstance)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var valuesArray []int
	for rows.Next() {
		var numCores int
		if err := rows.Scan(&numCores); err != nil {
			log.Fatal(err)
		}
		valuesArray = append(valuesArray, numCores)
	}

	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	if len(valuesArray) > 0 {
		return valuesArray
	} else {
		return nil
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
*/
func getThroughputData(selectThroughput int64, writeThroughput int64) {
	selectQuery := "SELECT foo.* FROM (SELECT id, ROUND((? / inserts_per_core) + 0.5) AS insert_total_cores," +
		"ROUND((? / selects_per_core) + 0.5) AS select_total_cores, num_cores, num_nodes FROM sharded_sizing " +
		"WHERE dimension = 'MaxThroughput' AND num_cores >= ?) AS foo ORDER BY select_total_cores + insert_total_cores," +
		"num_cores;"
	rows, err := DB.Query(selectQuery, writeThroughput, selectThroughput, SizingReport.VCPUsPerInstance)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	allMaps := convertToMap(rows)
	var insertTotalCores float64 = 0
	var selectTotalCores float64 = 0
	for _, value := range allMaps {
		insertTotalCores += value["insert_total_cores"].(float64)
		selectTotalCores += value["select_total_cores"].(float64)
	}

	// add the additional nodes to the total
	SizingReport.NumNodes += math.Max(math.Ceil((selectTotalCores+insertTotalCores)/float64(SizingReport.VCPUsPerInstance)), 1)
}

/*
calculateTimeTakenForMigration estimates the time taken for migration of database objects based on their type, size,
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
*/
func calculateTimeTakenForMigration(objectType string, dbObjects []SourceDBMetadata, vCPUPerInstance float64, memPerCore float64) float64 {
	// the total size of colocated objects
	var size float64 = 0
	var timeTakenOfFetchedRow float64
	var maxSizeOfFetchedRow float64
	for _, dbObject := range dbObjects {
		size += dbObject.Size
	}

	// find the rows in experiment data about the approx row matching the size
	selectQuery := fmt.Sprintf("SELECT csv_size_gb, migration_time_secs from %v_load_time where "+
		"num_cores = ? and mem_per_core = ? and csv_size_gb >= ? UNION ALL "+
		"SELECT csv_size_gb, migration_time_secs from sharded_load_time WHERE csv_size_gb = (SELECT MAX(csv_size_gb) "+
		"FROM sharded_load_time) LIMIT 1;", objectType)
	row := DB.QueryRow(selectQuery, vCPUPerInstance, memPerCore, size)

	if err := row.Scan(&maxSizeOfFetchedRow, &timeTakenOfFetchedRow); err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No rows were returned by the query.")
		} else {
			log.Fatal(err)
		}
	}

	migrationTime := ((timeTakenOfFetchedRow * size) / maxSizeOfFetchedRow) / 60
	return math.Ceil(migrationTime)
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
func getSourceMetadata() ([]SourceDBMetadata, []SourceDBMetadata, float64) {
	query := "SELECT schema_name, object_name,row_count,reads,writes,isIndex,parent_table_name,size FROM migration_assessment_stats ORDER BY size ASC"
	rows, err := SourceMetaDB.Query(query)
	if err != nil {
		fmt.Println("no records found")
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
		log.Fatal(err)
	}
	SourceMetaDB.Close()
	return sourceTableMetadata, sourceIndexMetadata, totalSourceDBSize
}

/*
bytesToGB function converts the size of the source object from bytes to GB as it is required for further calculation
Parameters:

	sizeInBytes: size of source object in bytes

Returns:

	sizeInGB: size of source object in gigabytes
*/
func bytesToGB(sizeInBytes float64) float64 {
	return sizeInBytes / (1024 * 1024 * 1024)
}

func createConnectionToExperimentData(targetYbVersion string) {
	filePath := getExperimentFile(targetYbVersion)
	err := ConnectExperimentDataDatabase(filePath)
	if err != nil {
		panic(err)
	}
}

func getExperimentFile(targetYbVersion string) string {
	filePath := "src/migassessment/resources/yb_" + strings.ReplaceAll(targetYbVersion, ".", "_") + "_source.db"
	if checkInternetAccess() {
		remoteFileExists := checkFileExistsOnRemoteRepo(filePath)
		if remoteFileExists {
			filePath = strings.ReplaceAll(filePath, "src/migassessment/resources/", baseDownloadPath)
		} else {
			// check if local file exists
			isFileExist := utils.FileOrFolderExists(filePath)
			if !isFileExist {
				panic("file doesn't exist")
			}
		}
	} else {
		// no network access
		fmt.Println("No network access. Checking file locally...")
		// check if local file exists
		isFileExist := utils.FileOrFolderExists(filePath)
		if !isFileExist {
			panic("file doesn't exist")
		}
	}
	return filePath
}

func checkFileExistsOnRemoteRepo(fileName string) bool {
	remotePath := "https://raw.githubusercontent.com/yb-voyager/sshaikh/mat/yb-voyager/" + fileName
	resp, _ := http.Get(remotePath)

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		//fmt.Println("File does not exist on remote location")
		return false
	} else {
		downloadPath := strings.ReplaceAll(fileName, "resources/", baseDownloadPath)
		out, _ := os.Create(downloadPath)
		defer func(out *os.File) {
			err := out.Close()
			if err != nil {
				panic(err)
			}
		}(out)
		_, err := io.Copy(out, resp.Body)
		return err == nil
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
*/
func checkAndFetchIndexes(table SourceDBMetadata, indexes []SourceDBMetadata) ([]SourceDBMetadata, float64) {
	indexesOfTable := make([]SourceDBMetadata, 0)
	var indexesSizeSum float64 = 0
	for _, index := range indexes {
		if index.ParentTableName.Valid && (index.ParentTableName.String == (table.SchemaName + "." + table.ObjectName)) {
			indexesOfTable = append(indexesOfTable, index)
			indexesSizeSum += index.Size
		}
	}
	return indexesOfTable, indexesSizeSum
}
