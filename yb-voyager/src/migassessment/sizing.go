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
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

type SizingParams struct {
	// add any sizing specific parameters required from user here
	// dummy params for now
	UseNvme           bool `toml:"use_nvme"`
	MultiAzDeployment bool `toml:"multi_az_deployment"`
}

type SourceDBMetadata struct {
	ObjectName      string `json:"object_name"`
	RowCount        int64  `json:"row_count,string"`
	ColCount        int64  `json:"col_count,string"`
	ReadsPerSec     int64  `json:"reads_per_second,string"`
	WritesPerSec    int64  `json:"writes_per_second,string"`
	IsIndex         bool   `json:"is_index,string"`
	ParentTableName string `json:"parent_table_name"`
	SizeInGB        int64  `json:"size_in_gb,string"`
}

var baseDownloadPath = "src/migassessment/resources/remote/"
var DB *sql.DB
var SourceMetaDB *sql.DB

func SizingAssessment() error {
	log.Infof("loading metadata files for sharding assessment")
	srcMeta, totalSourceDBSize := loadSourceMetadata()
	createConnectionToExperimentData(assessmentParams.TargetYBVersion)
	generateShardingRecommendations(srcMeta, totalSourceDBSize)
	// print recommendation till this point
	//PrintAssessmentReport()
	generateSizingRecommendations(srcMeta, totalSourceDBSize)
	//PrintAssessmentReport()
	return nil
}

func PrintAssessmentReport() {
	fmt.Println("\n--------------------------------------------")
	fmt.Println("\tColocated Tables: ", FinalReport.ColocatedTables)
	fmt.Println("\tColocated Reasoning: ", FinalReport.ColocatedReasoning)
	fmt.Println("\tSharded Tables: ", FinalReport.ShardedTables)
	fmt.Println("\tNumNodes: ", FinalReport.NumNodes)
	fmt.Println("\tVCPUsPerInstance: ", FinalReport.VCPUsPerInstance)
	fmt.Println("\tMemoryPerInstance: ", FinalReport.MemoryPerInstance)
	fmt.Println("\tOptimalSelectConnectionsPerNode: ", FinalReport.OptimalSelectConnectionsPerNode)
	fmt.Println("\tOptimalInsertConnectionsPerNode: ", FinalReport.OptimalInsertConnectionsPerNode)
	fmt.Println("\tMigration time taken in min: ", FinalReport.MigrationTimeTakenInMin)
	fmt.Println("--------------------------------------------")
}

func loadSourceMetadata() ([]SourceDBMetadata, int64) {
	err := ConnectSourceMetaDatabase("src/migassessment/source_info_test3.db")
	checkErr(err)
	srcMeta, totalSourceDBSize := getSourceMetadata()
	return srcMeta, totalSourceDBSize
}

func getSourceMetadata() ([]SourceDBMetadata, int64) {
	rows, err := SourceMetaDB.Query("SELECT * FROM source_metadata ORDER BY size_in_gb ASC")
	if err != nil {
		fmt.Println("no records found")
	}
	defer rows.Close()

	// Iterate over the rows
	var sourceMetadata []SourceDBMetadata
	var totalSourceDBSize int64 = 0
	for rows.Next() {
		var metadata SourceDBMetadata
		if err := rows.Scan(&metadata.ObjectName, &metadata.RowCount, &metadata.ColCount, &metadata.ReadsPerSec,
			&metadata.WritesPerSec, &metadata.IsIndex, &metadata.ParentTableName, &metadata.SizeInGB); err != nil {
			log.Fatal(err)
		}
		sourceMetadata = append(sourceMetadata, metadata)
		totalSourceDBSize += metadata.SizeInGB
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
	SourceMetaDB.Close()
	return sourceMetadata, totalSourceDBSize
}

func createConnectionToExperimentData(targetYbVersion string) {
	filePath := getExperimentFile(targetYbVersion)
	err := ConnectExperimentDataDatabase(filePath)
	checkErr(err)
}

func getExperimentFile(targetYbVersion string) string {
	filePath := "src/migassessment/resources/yb_" + strings.ReplaceAll(targetYbVersion, ".", "_") + "_source.db"
	if checkInternetAccess() {
		remoteFileExists := checkFileExistsOnRemoteRepo(filePath)
		if remoteFileExists {
			filePath = strings.ReplaceAll(filePath, "src/migassessment/resources/", baseDownloadPath)
		} else {
			// check if local file exists
			isFileExist := checkLocalFileExists(filePath)
			if !isFileExist {
				panic("file doesn't exist")
			}
		}
	} else {
		// no network access
		fmt.Println("No network access. Checking file locally...")
		// check if local file exists
		isFileExist := checkLocalFileExists(filePath)
		if !isFileExist {
			panic("file doesn't exist")
		}
	}
	return filePath
}

func generateShardingRecommendations(srcMeta []SourceDBMetadata, totalSourceDBSize int64) {
	var selectedRow [6]interface{} // Assuming 6 columns in the table
	row := DB.QueryRow("SELECT * FROM limits WHERE max_num_tables > ? AND min_num_tables <= ? AND "+
		"max_size >= ? UNION ALL SELECT * FROM limits WHERE max_size = (SELECT MAX(max_size) "+
		"FROM limits) LIMIT 1;", len(srcMeta), len(srcMeta), totalSourceDBSize)

	var S int64
	if err := row.Scan(&selectedRow[0], &selectedRow[1], &selectedRow[2], &selectedRow[3], &selectedRow[4], &selectedRow[5]); err != nil {
		if err == sql.ErrNoRows {
			log.Println("No rows were returned by the query.")
		} else {
			log.Fatal(err)
		}
	}
	S = selectedRow[0].(int64) // Assuming max_size is the first column
	var cumulativeSum int64
	var colocatedTables []SourceDBMetadata
	var coloTables []string
	var shardedTables []string
	var index int
	for i, key := range srcMeta {
		cumulativeSum += key.SizeInGB
		// need to find all indexes for the current tables
		if cumulativeSum > S {
			break
		}
		index = i
		colocatedTables = append(colocatedTables, key)
		coloTables = append(coloTables, key.ObjectName)
	}
	for _, key := range srcMeta[index+1:] {
		shardedTables = append(shardedTables, key.ObjectName)
	}
	reasoning := "all tables could be fit into colocated db with recommended instance type"
	if len(shardedTables) >= 0 {
		reasoning = "few tables fit as colocated and rest as sharded"
	}
	FinalReport = &Report{
		ColocatedTables:    coloTables,
		ColocatedReasoning: reasoning,
		ShardedTables:      shardedTables,
		VCPUsPerInstance:   selectedRow[2].(int64),
		MemoryPerInstance:  selectedRow[2].(int64) * selectedRow[3].(int64),
	}
}

func generateSizingRecommendations(srcMeta []SourceDBMetadata, totalSourceDBSize int64) {
	if len(FinalReport.ShardedTables) == 0 {
		fmt.Println("Skipping sizing assessment as all tables can be fit into colocated")
	} else {
		// table limit check
		arrayOfSupportedCores := checkTableLimits(len(srcMeta))
		fmt.Println(arrayOfSupportedCores)
		// calculate throughput data
		var sumSourceSelectThroughput int64 = 0
		var sumSourceWriteThroughput int64 = 0
		for _, metadata := range srcMeta {
			sumSourceSelectThroughput += metadata.ReadsPerSec
			sumSourceWriteThroughput += metadata.WritesPerSec
		}
		fmt.Printf("source-select-throughput: %d\tsource-write-throughput: %d\n", sumSourceSelectThroughput, sumSourceWriteThroughput)
		getThroughputData(sumSourceSelectThroughput, sumSourceWriteThroughput)
		// calculate impact of table count : not in this version
		//values = calculateTableCountImpact(values, int64(len(srcMeta)))

		// calculate impact of horizontal scaling: not in this version
		//calculateImpactOfHorizontalScaling(values)

		// get connections per core
		FinalReport.OptimalSelectConnectionsPerNode, FinalReport.OptimalInsertConnectionsPerNode = getConnectionsPerCore(arrayOfSupportedCores[0])
	}
}

func getConnectionsPerCore(numCores int) (int64, int64) {
	var selectConnectionsPerCore, insertConnectionsPerCore int64
	selectQuery := "select select_conn_per_node, insert_conn_per_node from sizing " +
		"where dimension like 'MaxThroughput' and num_cores = ? order by num_nodes limit 1"
	row := DB.QueryRow(selectQuery, numCores)

	if err := row.Scan(&selectConnectionsPerCore, &insertConnectionsPerCore); err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No rows were returned by the query.")
		} else {
			log.Fatal(err)
		}
	}
	fmt.Println("Select connections per core:", selectConnectionsPerCore)
	fmt.Println("Insert connections per core:", insertConnectionsPerCore)
	return selectConnectionsPerCore, insertConnectionsPerCore
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
		out, err := os.Create(downloadPath)
		defer out.Close()
		_, err = io.Copy(out, resp.Body)

		if err != nil {
			panic(err)
		}
		return true
	}
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

func checkTableLimits(reqTables int) []int {
	// added num_cores >= VCPUPerInstance from colo recommendation as that is the starting point
	selectQuery := "SELECT num_cores FROM sizing WHERE num_tables > ? AND num_cores >= ? AND dimension LIKE '%TableLimits-3nodeRF=3%' ORDER BY num_cores"
	rows, err := DB.Query(selectQuery, reqTables, FinalReport.VCPUsPerInstance)
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
		//*logs = append(*logs, fmt.Sprintf("No results found for required %d tables.", reqTables))
		return nil
	}
}

func getThroughputData(selectThroughput int64, writeThroughput int64) {
	selectQuery := "SELECT foo.* FROM (SELECT id, ROUND((? / inserts_per_core) + 0.5) AS insert_total_cores," +
		"ROUND((? / selects_per_core) + 0.5) AS select_total_cores, num_cores, num_nodes FROM sizing " +
		"WHERE dimension = 'MaxThroughput' AND num_cores >= ?) AS foo ORDER BY select_total_cores + insert_total_cores," +
		"num_cores;"
	rows, err := DB.Query(selectQuery, writeThroughput, selectThroughput, FinalReport.VCPUsPerInstance)
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
	fmt.Println("insert total cores:", insertTotalCores)
	fmt.Println("select total cores:", selectTotalCores)
	FinalReport.NumNodes = math.Ceil((selectTotalCores + insertTotalCores) / float64(FinalReport.VCPUsPerInstance))
}

/*
========== Unused functions as of this version ====================
*/
func calculateTableCountImpact(values []map[string]string, numTables int64) []map[string]string {
	var impactOnSelect float64 = 0
	var impactOnInsert float64 = 0
	//var numCores int64 = 16

	selectQuery := "select selects_per_core, inserts_per_core, num_tables, num_cores, num_nodes from " +
		"sizing where dimension like 'MaxThroughput-TableCount' and num_cores = 16 order by num_tables asc;"
	rows, err := DB.Query(selectQuery)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	allMaps := convertToMapOfStringString(convertToMap(rows))
	//calculate impact on select
	impactOnSelect = calculateTableCountImpactForCores(allMaps, "selects_per_core", numTables)
	// calculate impact on inserts
	impactOnInsert = calculateTableCountImpactForCores(allMaps, "inserts_per_core", numTables)
	fmt.Println("\n\nBefore data: ", values)
	for i, value := range values {
		values[i] = calculateCoresAfterImpact("select", value, impactOnSelect, 1)
		values[i] = calculateCoresAfterImpact("insert", value, impactOnInsert, 1)
	}
	fmt.Println("After data:", values)

	return values
}

func calculateCoresAfterImpact(field string, rowJSON map[string]string, impact float64, itr float64) map[string]string {
	coreParam := field + "_total_cores"
	reqCores, _ := strconv.ParseFloat(rowJSON[coreParam], 64)
	impact = math.Round(impact*1000) / 1000                           // Round to 3 decimal places
	reqCores = math.Round((reqCores+(reqCores*impact*itr))*100) / 100 // Round to 2 decimal places
	rowJSON[coreParam] = fmt.Sprintf("%f", reqCores)
	return rowJSON
}

func calculateTableCountImpactForCores(values []map[string]string, field string, numTables int64) float64 {
	value1, _ := strconv.ParseFloat(values[0][field], 64)
	var value2 float64 = 0
	for i := 0; i < len(values); i++ {
		if i < len(values)-1 {
			val, _ := strconv.ParseFloat(values[i]["num_tables"], 64)
			if val > float64(numTables) {
				value2, _ = strconv.ParseFloat(values[i-1][field], 64)
				break
			}
		}
	}
	impact := (value1 - value2) / value1
	return impact
}

func printRows(db *sql.DB, tableName string) {
	rows, err := db.Query(fmt.Sprintf("SELECT * from %v", tableName))
	if err != nil {
		fmt.Println("no records found")
	}
	defer rows.Close()
	allMaps := convertToMap(rows)
	printMap(allMaps)

	err = rows.Err()
	if err != nil {
		fmt.Println("error occurred")
	}
}

/*func calculateImpactOfHorizontalScaling(jsonResults []map[string]string) []map[string]string{} {
	var hsResults []map[string]interface{}
	for _, rowJson := range jsonResults {
		numCores,_ := strconv.ParseFloat(rowJson["num_cores"], 64)
		selectTotalCores,_ := strconv.ParseFloat(rowJson["select_total_cores"], 64)
		InsertTotalCores,_ := strconv.ParseFloat(rowJson["insert_total_cores"], 64)
		selectReqNodes := math.Ceil(selectTotalCores / numCores)
		insertReqNodes := math.Ceil(InsertTotalCores / numCores)
		reqNodes := selectReqNodes + insertReqNodes
		if reqNodes > 3 {
			numCores := strconv.ParseFloat(rowJson["num_cores"], 64)
			selectQuery := "select selects_per_core, inserts_per_core, num_tables, num_cores, num_nodes from sizing1 " +
				" where dimension like 'HorizontalScaling' and num_cores = ? and num_nodes in (3,6) " +
				"order by num_cores, num_nodes"
			rows, err := DB.Query(selectQuery, numCores)
			if err != nil {
				log.Printf("Error executing query: %v", err)
				continue
			}
			defer rows.Close()

			var hsResultsMap map[string]interface{}
			for rows.Next() {
				var selectsPerCore, insertsPerCore, numTables, numCores, numNodes interface{}
				if err := rows.Scan(&selectsPerCore, &insertsPerCore, &numTables, &numCores, &numNodes); err != nil {
					log.Printf("Error scanning row: %v", err)
					continue
				}
				hsResultsMap = map[string]interface{}{
					"selects_per_core": selectsPerCore,
					"inserts_per_core": insertsPerCore,
					"num_tables":       numTables,
					"num_cores":        numCores,
					"num_nodes":        numNodes,
				}
			}
			hsResults = append(hsResults, hsResultsMap)

			coreImpactSelect := calculateHorizontalScalingImpact(hsResultsMap, "selects_per_core")
			if coreImpactSelect > 0 {
				rowJson = calculateCoresAfterImpact("select", rowJson, coreImpactSelect, (reqNodes - 3))
			}
			coreImpactInsert := calculateHorizontalScalingImpact(hsResultsMap, "inserts_per_core")
			if coreImpactInsert > 0 {
				rowJson = calculateCoresAfterImpact("insert", rowJson, coreImpactInsert, (reqNodes - 3))
			}
			fmt.Println("Impact of extra node on", numCores, "cores - select :", coreImpactSelect, "and insert :", coreImpactInsert)
			fmt.Println("\nUpdated recommendation after calculating Impact of Horizontal Scalability ::")
			//printJsonData(jsonResults, logs)
		} else {
			fmt.Println( "No horizontal scaling results found for", numCores, "cores.")
		}
	}
	return jsonResults
}*/

/*func calculateHorizontalScalingImpact(hsResultsMap map[string]interface{}, key string) float64 {
	if val, ok := hsResultsMap[key].(float64); ok {
		return val
	}
	return 0
}*/

/*func findRecommendationForColocatedTables() {
	// Example input tables with respective sizes
	data := map[string]int64{
		"table1": 40,
		"table2": 20,
		"table3": 30,
		"table4": 10,
		"table5": 5,
		"table6": 50,
	}

	// Sort the map based on values and get the sum of values.
	sortedKeys, totalSum := sortByValue(data)
	// Connect to SQLite database
	db, err := sql.Open("sqlite3", "src/migassessment/resources/yb_2_20_source.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	var selectedRow [6]interface{} // Assuming 6 columns in the table
	rows, err := db.Query("SELECT * FROM limits WHERE max_num_tables > ? AND min_num_tables <= ? AND max_size <= ? ORDER BY max_size DESC LIMIT 1", len(data), len(data), totalSum)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var S int64
	for rows.Next() {
		if err := rows.Scan(&selectedRow[0], &selectedRow[1], &selectedRow[2], &selectedRow[3], &selectedRow[4], &selectedRow[5]); err != nil {
			panic(err)
		}
		S = selectedRow[0].(int64) // Assuming max_size is the first column
		var cumulativeSum int64
		var colocatedTables []string
		index := 0
		for i, key := range sortedKeys {
			cumulativeSum += data[key]
			if cumulativeSum > S {
				break
			}
			index = i
			colocatedTables = append(colocatedTables, key)
		}

		fmt.Printf("\nTotal size of source tables: %v\n", totalSum)
		fmt.Println("Recommended list of colocated Tables", colocatedTables)
		fmt.Println("Recommended list of sharded tables", sortedKeys[index+1:])
		fmt.Println("Recommended instance type row from limits table:", selectedRow)
		//fmt.Println("max size is : ", S)
		fmt.Println("\n")

	}
	if err := rows.Err(); err != nil {
		panic(err)
	}

}*/

func sortByValue(m map[string]int64) ([]string, int64) {
	var sum int64

	// Create a slice of key-value pairs.
	var sortedKeys []string
	for key, value := range m {
		sortedKeys = append(sortedKeys, key)
		sum += value
	}

	// Sort the slice based on the values of the map.
	sort.Slice(sortedKeys, func(i, j int) bool {
		return m[sortedKeys[i]] < m[sortedKeys[j]]
	})

	return sortedKeys, sum
}
