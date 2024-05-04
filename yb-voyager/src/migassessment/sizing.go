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

type ExpDataShardedLimit struct {
	numCores              sql.NullFloat64 `db:"num_cores,string"`
	memPerCore            sql.NullFloat64 `db:"mem_per_core,string"`
	maxSupportedNumTables sql.NullInt64   `db:"max_num_tables,string"`
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

type ExpDataShardedThroughput struct {
	numCores                   sql.NullFloat64 `db:"num_cores,string"`
	memPerCore                 sql.NullFloat64 `db:"mem_per_core,string"`
	maxSupportedSelectsPerCore sql.NullFloat64 `db:"max_selects_per_core,string"`
	maxSupportedInsertsPerCore sql.NullFloat64 `db:"max_inserts_per_core,string"`
	selectConnPerNode          sql.NullInt64   `db:"select_conn_per_node,string"`
	insertConnPerNode          sql.NullInt64   `db:"insert_conn_per_node,string"`
}

type IntermediateRecommendation struct {
	ColocatedTables                 []SourceDBMetadata
	ShardedTables                   []SourceDBMetadata
	ColocatedSize                   float64
	ShardedSize                     float64
	NumNodes                        float64
	VCPUsPerInstance                int
	MemoryPerCore                   int
	OptimalSelectConnectionsPerNode int64
	OptimalInsertConnectionsPerNode int64
	EstimatedTimeInMinForImport     float64
	ParallelVoyagerJobs             float64
	FailureReasoning                string
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
	DBS_DIR                  = "dbs"
)

var ExperimentDB *sql.DB
var experimentDBPath = filepath.Join(AssessmentDir, DBS_DIR, EXPERIMENT_DATA_FILENAME)

//go:embed resources/yb_2024_0_source.db
var experimentData20240 []byte

func SizingAssessment(assessmentMetadataDir string) error {

	log.Infof("loading metadata files for sharding assessment")
	sourceTableMetadata, sourceIndexMetadata, _, err := loadSourceMetadata(assessmentMetadataDir)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("failed to load source metadata: %v", err)
		return fmt.Errorf("failed to load source metadata: %w", err)
	}

	err = createConnectionToExperimentData()
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("failed to connect to experiment data: %v", err)
		return fmt.Errorf("failed to connect to experiment data: %w", err)
	}

	colocatedLimits, err := loadColocatedLimit()
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("error fetching the colocated limits: %v", err)
		return fmt.Errorf("error fetching the colocated limits: %w", err)
	}

	shardedLimits, err := loadShardedTableLimits()
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("error fetching the sharded limits: %v", err)
		return fmt.Errorf("error fetching the colocated limits: %w", err)
	}

	shardedThroughput, err := loadShardedThroughput()
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("error fetching the sharded throughput: %v", err)
		return fmt.Errorf("error fetching the sharded throughput: %w", err)
	}

	sizingRecommendationPerCore := createSizingRecommendationStructure(colocatedLimits)

	sizingRecommendationPerCore = shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata,
		colocatedLimits, sizingRecommendationPerCore)

	sizingRecommendationPerCore = shardingBasedOnOperations(sourceIndexMetadata, colocatedLimits, sizingRecommendationPerCore)

	sizingRecommendationPerCore = checkShardedTableLimit(sourceIndexMetadata, shardedLimits, sizingRecommendationPerCore)

	sizingRecommendationPerCore = findNumNodesNeeded(sourceIndexMetadata, shardedThroughput, sizingRecommendationPerCore)

	finalSizingRecommendation := pickBestRecommendation(sizingRecommendationPerCore)

	if finalSizingRecommendation.FailureReasoning != "" {
		SizingReport.FailureReasoning = finalSizingRecommendation.FailureReasoning
		return fmt.Errorf("error picking best recommendation: %w", finalSizingRecommendation.FailureReasoning)
	}

	colocatedObjects := getListOfIndexesAlongWithObjects(finalSizingRecommendation.ColocatedTables, sourceIndexMetadata)
	shardedObjects := getListOfIndexesAlongWithObjects(finalSizingRecommendation.ShardedTables, sourceIndexMetadata)

	// calculate time taken for colocated import
	importTimeForColocatedObjects, parallelVoyagerJobsColocated, err :=
		calculateTimeTakenAndParallelJobsForImport(COLOCATED_LOAD_TIME_TABLE, colocatedObjects,
			finalSizingRecommendation.VCPUsPerInstance, finalSizingRecommendation.MemoryPerCore)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("calculate time taken for colocated data import: %v", err)
		return fmt.Errorf("calculate time taken for colocated data import: %w", err)
	}

	// calculate time taken for sharded import
	importTimeForShardedObjects, parallelVoyagerJobsSharded, err :=
		calculateTimeTakenAndParallelJobsForImport(SHARDED_LOAD_TIME_TABLE, shardedObjects,
			finalSizingRecommendation.VCPUsPerInstance, finalSizingRecommendation.MemoryPerCore)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("calculate time taken for sharded data import: %v", err)
		return fmt.Errorf("calculate time taken for sharded data import: %w", err)
	}

	sizingRecommendation := &SizingRecommendation{
		ColocatedTables:                 fetchObjectNames(colocatedObjects),
		ShardedTables:                   fetchObjectNames(shardedObjects),
		VCPUsPerInstance:                finalSizingRecommendation.VCPUsPerInstance,
		MemoryPerInstance:               finalSizingRecommendation.VCPUsPerInstance * finalSizingRecommendation.MemoryPerCore,
		NumNodes:                        finalSizingRecommendation.NumNodes,
		OptimalSelectConnectionsPerNode: finalSizingRecommendation.OptimalSelectConnectionsPerNode,
		OptimalInsertConnectionsPerNode: finalSizingRecommendation.OptimalInsertConnectionsPerNode,
		ParallelVoyagerJobs:             math.Min(float64(parallelVoyagerJobsColocated), float64(parallelVoyagerJobsSharded)),
		ColocatedReasoning:              getReasoning(finalSizingRecommendation, shardedObjects, colocatedObjects),
		EstimatedTimeInMinForImport:     importTimeForColocatedObjects + importTimeForShardedObjects,
	}
	SizingReport.SizingRecommendation = *sizingRecommendation

	return nil
}

func getReasoning(recommendation IntermediateRecommendation, shardedObjects []SourceDBMetadata, colocatedObjects []SourceDBMetadata) string {
	colocatedObjectsSize, colocatedReads, colocatedWrites := getObjectsSize(colocatedObjects)
	reasoning := fmt.Sprintf("Recommended instance type with %v vCPU and %vGiB memory could fit ",
		recommendation.VCPUsPerInstance, recommendation.VCPUsPerInstance*recommendation.MemoryPerCore)

	if len(colocatedObjects) > 0 {
		reasoning += fmt.Sprintf("%v object(s) with %0.4fGB size and throughput requirement of %v reads/sec"+
			" and %v writes/sec as colocated.", len(colocatedObjects), colocatedObjectsSize, colocatedReads,
			colocatedWrites)
	}
	if len(shardedObjects) > 0 {
		shardedObjectsSize, shardedReads, shardedWrites := getObjectsSize(shardedObjects)
		shardedReasoning := fmt.Sprintf("%v object(s) with %0.4fGB size and throughput requirement of %v reads/sec"+
			" and %v writes/sec ", len(shardedObjects), shardedObjectsSize,
			shardedReads, shardedWrites)
		if len(colocatedObjects) > 0 {
			reasoning += "Rest " + shardedReasoning + "need to imported as sharded. "
		} else {
			reasoning += shardedReasoning + "as sharded"
		}

	}
	return reasoning
}

func getObjectsSize(objects []SourceDBMetadata) (float64, int64, int64) {
	var objectsSize float64 = 0
	var objectSelectOps int64 = 0
	var objectInsertOps int64 = 0

	for _, object := range objects {
		objectsSize += object.Size
		objectSelectOps += object.ReadsPerSec
		objectInsertOps += object.WritesPerSec
	}
	return objectsSize, objectSelectOps, objectInsertOps
}

func getListOfIndexesAlongWithObjects(tableList []SourceDBMetadata, sourceIndexMetadata []SourceDBMetadata) []SourceDBMetadata {
	var indexesAndObject []SourceDBMetadata
	for _, table := range tableList {
		indexes, _, _, _ := checkAndFetchIndexes(table, sourceIndexMetadata)
		indexesAndObject = append(indexesAndObject, indexes...)
		indexesAndObject = append(indexesAndObject, table)
	}
	return indexesAndObject
}

func pickBestRecommendation(recommendation map[int]IntermediateRecommendation) IntermediateRecommendation {
	// find the one with least number of nodes
	var minCores int = math.MaxUint32
	var finalRecommendation IntermediateRecommendation
	var foundRecommendation bool = false
	var maxCores int = math.MinInt32

	for _, rec := range recommendation {
		if maxCores < rec.VCPUsPerInstance {
			maxCores = rec.VCPUsPerInstance
		}
		if rec.FailureReasoning == "" {
			foundRecommendation = true
			if minCores > int(rec.NumNodes)*rec.VCPUsPerInstance {
				finalRecommendation = rec
				minCores = int(rec.NumNodes) * rec.VCPUsPerInstance
			} else if minCores == int(rec.NumNodes)*rec.VCPUsPerInstance {
				// recommend the higher core machine is the number of cores are same across machines.
				if rec.VCPUsPerInstance > finalRecommendation.VCPUsPerInstance {
					finalRecommendation = rec
				}
			}
		}
	}
	if !foundRecommendation {
		finalRecommendation = recommendation[maxCores]
	}
	return finalRecommendation
}

func findNumNodesNeeded(sourceIndexMetadata []SourceDBMetadata, shardedLimits []ExpDataShardedThroughput,
	recommendation map[int]IntermediateRecommendation) map[int]IntermediateRecommendation {
	for _, shardedLimit := range shardedLimits {
		previousRecommendation := recommendation[int(shardedLimit.numCores.Float64)]
		var cumulativeSelectOpsPerSec int64 = 0
		var cumulativeInsertOpsPerSec int64 = 0

		for _, table := range previousRecommendation.ShardedTables {
			_, _, indexReads, indexWrites := checkAndFetchIndexes(table, sourceIndexMetadata)
			cumulativeSelectOpsPerSec += table.ReadsPerSec + indexReads
			cumulativeInsertOpsPerSec += table.WritesPerSec + indexWrites
		}
		neededCores :=
			math.Ceil(float64(cumulativeSelectOpsPerSec)/shardedLimit.maxSupportedSelectsPerCore.Float64 +
				float64(cumulativeInsertOpsPerSec)/shardedLimit.maxSupportedInsertsPerCore.Float64)
		// assumption - one node has been taken over by colocated. TBD. Need to fix this.
		nodesNeeded := math.Ceil(neededCores/shardedLimit.numCores.Float64) + 1
		// assumption - rf3 setups only. Fix this
		if nodesNeeded < 3 {
			nodesNeeded = 3
		}
		recommendation[int(shardedLimit.numCores.Float64)] = IntermediateRecommendation{
			ColocatedTables:                 previousRecommendation.ColocatedTables,
			ShardedTables:                   previousRecommendation.ShardedTables,
			VCPUsPerInstance:                previousRecommendation.VCPUsPerInstance,
			MemoryPerCore:                   previousRecommendation.MemoryPerCore,
			NumNodes:                        nodesNeeded,
			OptimalSelectConnectionsPerNode: shardedLimit.selectConnPerNode.Int64,
			OptimalInsertConnectionsPerNode: shardedLimit.insertConnPerNode.Int64,
			ParallelVoyagerJobs:             previousRecommendation.ParallelVoyagerJobs,
			ColocatedSize:                   previousRecommendation.ColocatedSize,
			ShardedSize:                     previousRecommendation.ShardedSize,
			EstimatedTimeInMinForImport:     previousRecommendation.EstimatedTimeInMinForImport,
			FailureReasoning:                previousRecommendation.FailureReasoning,
		}
	}
	return recommendation
}

func loadShardedThroughput() ([]ExpDataShardedThroughput, error) {
	selectQuery := fmt.Sprintf(`
			SELECT inserts_per_core,
				   selects_per_core, 
				   num_cores, 
				   memory_per_core,
				   select_conn_per_node, 
			   	   insert_conn_per_node 
			FROM %s 
			WHERE dimension = 'MaxThroughput' 
			ORDER BY num_cores DESC;
	`, SHARDED_SIZING_TABLE)
	rows, err := ExperimentDB.Query(selectQuery)
	if err != nil {
		return nil, fmt.Errorf("error while fetching throughput info with query [%s]: %w", selectQuery, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close result set for query: [%s]", selectQuery)
		}
	}()

	var shardedThroughput []ExpDataShardedThroughput
	for rows.Next() {
		var throughput ExpDataShardedThroughput
		if err := rows.Scan(&throughput.maxSupportedInsertsPerCore,
			&throughput.maxSupportedSelectsPerCore, &throughput.numCores,
			&throughput.memPerCore, &throughput.selectConnPerNode, &throughput.insertConnPerNode); err != nil {
			return nil, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", selectQuery, err)
		}
		shardedThroughput = append(shardedThroughput, throughput)
	}
	return shardedThroughput, nil
}

func checkShardedTableLimit(sourceIndexMetadata []SourceDBMetadata, shardedLimits []ExpDataShardedLimit, recommendation map[int]IntermediateRecommendation) map[int]IntermediateRecommendation {

	for _, shardedLimit := range shardedLimits {
		var totalObjectCount int64 = 0
		previousRecommendation := recommendation[int(shardedLimit.numCores.Float64)]
		for _, table := range previousRecommendation.ShardedTables {
			indexes, _, _, _ := checkAndFetchIndexes(table, sourceIndexMetadata)
			totalObjectCount += int64(len(indexes)) + 1

		}
		if totalObjectCount > shardedLimit.maxSupportedNumTables.Int64 {
			failureReasoning := fmt.Sprintf("Cannot support %v sharded objects on a machine with %v cores and %v "+
				"memory per core", totalObjectCount, previousRecommendation.VCPUsPerInstance,
				previousRecommendation.MemoryPerCore)
			recommendation[int(shardedLimit.numCores.Float64)] = IntermediateRecommendation{
				ColocatedTables:                 []SourceDBMetadata{},
				ShardedTables:                   []SourceDBMetadata{},
				VCPUsPerInstance:                previousRecommendation.VCPUsPerInstance,
				MemoryPerCore:                   previousRecommendation.MemoryPerCore,
				NumNodes:                        previousRecommendation.NumNodes,
				OptimalSelectConnectionsPerNode: previousRecommendation.OptimalSelectConnectionsPerNode,
				OptimalInsertConnectionsPerNode: previousRecommendation.OptimalInsertConnectionsPerNode,
				ParallelVoyagerJobs:             previousRecommendation.ParallelVoyagerJobs,
				ColocatedSize:                   0,
				ShardedSize:                     0,
				EstimatedTimeInMinForImport:     previousRecommendation.EstimatedTimeInMinForImport,
				FailureReasoning:                failureReasoning,
			}
		}
	}
	return recommendation
}

func calculateSizeOfObjects() {

}

func shardingBasedOnOperations(sourceIndexMetadata []SourceDBMetadata,
	colocatedLimits []ExpDataColocatedLimit, recommendation map[int]IntermediateRecommendation) map[int]IntermediateRecommendation {

	for _, colocatedLimit := range colocatedLimits {
		var colocatedObjects []SourceDBMetadata
		var cumulativeColocatedSizeSum float64 = 0
		var numColocated int = 0
		var cumulativeSelectOpsPerSec int64 = 0
		var cumulativeInsertOpsPerSec int64 = 0
		previousRecommendation := recommendation[int(colocatedLimit.numCores.Float64)]

		for _, table := range previousRecommendation.ColocatedTables {
			_, indexesSizeSum, indexReads, indexWrites := checkAndFetchIndexes(table, sourceIndexMetadata)
			newSelectOpsPerSec := cumulativeSelectOpsPerSec + table.ReadsPerSec + indexReads
			newInsertOpsPerSec := cumulativeInsertOpsPerSec + table.WritesPerSec + indexWrites
			objectTotalSize := table.Size + indexesSizeSum

			neededCores :=
				math.Ceil(float64(newSelectOpsPerSec)/colocatedLimit.maxSupportedSelectsPerCore.Float64 +
					float64(newInsertOpsPerSec)/colocatedLimit.maxSupportedInsertsPerCore.Float64)

			if neededCores <= colocatedLimit.numCores.Float64 {
				colocatedObjects = append(colocatedObjects, table)
				cumulativeSelectOpsPerSec = newSelectOpsPerSec
				cumulativeInsertOpsPerSec = newInsertOpsPerSec
				cumulativeColocatedSizeSum += objectTotalSize
				numColocated++
			} else {
				break
			}
		}
		shardedObjects := previousRecommendation.ShardedTables
		var cumulativeSizeSharded float64 = 0

		for _, remainingTable := range previousRecommendation.ColocatedTables[numColocated:] {
			shardedObjects = append(shardedObjects, remainingTable)
			_, indexesSizeSumSharded, _, _ := checkAndFetchIndexes(remainingTable, sourceIndexMetadata)
			cumulativeSizeSharded += remainingTable.Size + indexesSizeSumSharded
		}

		recommendation[int(colocatedLimit.numCores.Float64)] = IntermediateRecommendation{
			ColocatedTables:                 colocatedObjects,
			ShardedTables:                   shardedObjects,
			VCPUsPerInstance:                previousRecommendation.VCPUsPerInstance,
			MemoryPerCore:                   previousRecommendation.MemoryPerCore,
			NumNodes:                        previousRecommendation.NumNodes,
			OptimalSelectConnectionsPerNode: previousRecommendation.OptimalSelectConnectionsPerNode,
			OptimalInsertConnectionsPerNode: previousRecommendation.OptimalInsertConnectionsPerNode,
			ParallelVoyagerJobs:             previousRecommendation.ParallelVoyagerJobs,
			ColocatedSize:                   cumulativeColocatedSizeSum,
			ShardedSize:                     cumulativeSizeSharded,
			EstimatedTimeInMinForImport:     previousRecommendation.EstimatedTimeInMinForImport,
		}
	}
	return recommendation
}

func shardingBasedOnTableSizeAndCount(sourceTableMetadata []SourceDBMetadata, sourceIndexMetadata []SourceDBMetadata,
	colocatedLimits []ExpDataColocatedLimit, recommendation map[int]IntermediateRecommendation) map[int]IntermediateRecommendation {

	for _, colocatedLimit := range colocatedLimits {
		var cumulativeColocatedSizeSum float64 = 0
		var colocatedObjects []SourceDBMetadata
		var numColocated int = 0
		var cumulativeObjectCount int64 = 0

		for _, table := range sourceTableMetadata {
			indexesOfTable, indexesSizeSum, _, _ := checkAndFetchIndexes(table, sourceIndexMetadata)
			newObjectCount := cumulativeObjectCount + int64(len(indexesOfTable)) + 1
			objectTotalSize := table.Size + indexesSizeSum
			newCumulativeSize := cumulativeColocatedSizeSum + objectTotalSize

			if newCumulativeSize <= colocatedLimit.maxColocatedSizeSupported.Float64 &&
				(newObjectCount <= colocatedLimit.maxSupportedNumTables.Int64) {
				cumulativeObjectCount = newObjectCount
				cumulativeColocatedSizeSum = newCumulativeSize
				colocatedObjects = append(colocatedObjects, table)
				numColocated++
			} else {
				break
			}
		}
		var shardedObjects []SourceDBMetadata
		var cumulativeSizeSharded float64 = 0

		for _, remainingTable := range sourceTableMetadata[numColocated:] {
			shardedObjects = append(shardedObjects, remainingTable)
			_, indexesSizeSumSharded, _, _ := checkAndFetchIndexes(remainingTable, sourceIndexMetadata)
			cumulativeSizeSharded += remainingTable.Size + indexesSizeSumSharded
		}
		previousRecommendation := recommendation[int(colocatedLimit.numCores.Float64)]

		recommendation[int(colocatedLimit.numCores.Float64)] = IntermediateRecommendation{
			ColocatedTables:                 colocatedObjects,
			ShardedTables:                   shardedObjects,
			VCPUsPerInstance:                previousRecommendation.VCPUsPerInstance,
			MemoryPerCore:                   previousRecommendation.MemoryPerCore,
			NumNodes:                        previousRecommendation.NumNodes,
			OptimalSelectConnectionsPerNode: previousRecommendation.OptimalSelectConnectionsPerNode,
			OptimalInsertConnectionsPerNode: previousRecommendation.OptimalInsertConnectionsPerNode,
			ParallelVoyagerJobs:             previousRecommendation.ParallelVoyagerJobs,
			ColocatedSize:                   cumulativeColocatedSizeSum,
			ShardedSize:                     cumulativeSizeSharded,
			EstimatedTimeInMinForImport:     previousRecommendation.EstimatedTimeInMinForImport,
		}
	}
	return recommendation
}

func createSizingRecommendationStructure(colocatedLimits []ExpDataColocatedLimit) map[int]IntermediateRecommendation {
	recommendationPerCore := make(map[int]IntermediateRecommendation)
	for _, colocatedLimit := range colocatedLimits {
		var sizingRecommendation IntermediateRecommendation
		sizingRecommendation.MemoryPerCore = int(colocatedLimit.memPerCore.Float64)
		sizingRecommendation.VCPUsPerInstance = int(colocatedLimit.numCores.Float64)
		recommendationPerCore[sizingRecommendation.VCPUsPerInstance] = sizingRecommendation
	}
	return recommendationPerCore
}

func loadShardedTableLimits() ([]ExpDataShardedLimit, error) {
	// added num_cores >= VCPUPerInstance from colo recommendation as that is the starting point
	selectQuery := fmt.Sprintf(`
			SELECT num_cores, memory_per_core, num_tables 
			FROM %s 
			WHERE dimension LIKE '%%TableLimits-3nodeRF=3%%' 
			ORDER BY num_cores
		`, SHARDED_SIZING_TABLE)
	rows, err := ExperimentDB.Query(selectQuery)

	if err != nil {
		return nil, fmt.Errorf("error while fetching cores info with query [%s]: %w", selectQuery, err)
	}

	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close result set for query: [%s]", selectQuery)
		}
	}()
	var shardedLimits []ExpDataShardedLimit

	for rows.Next() {
		var r1 ExpDataShardedLimit
		if err := rows.Scan(&r1.numCores, &r1.memPerCore, &r1.maxSupportedNumTables); err != nil {
			return nil, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", selectQuery, err)
		}
		shardedLimits = append(shardedLimits, r1)
	}
	return shardedLimits, nil
}

func loadColocatedLimit() ([]ExpDataColocatedLimit, error) {
	var colocatedLimits []ExpDataColocatedLimit
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
		return nil, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", query, err)
	}

	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close the result set for query [%v]", query)
		}
	}()

	for rows.Next() {
		var r1 ExpDataColocatedLimit
		if err := rows.Scan(&r1.maxColocatedSizeSupported, &r1.numCores, &r1.memPerCore, &r1.maxSupportedNumTables,
			&r1.minSupportedNumTables, &r1.maxSupportedSelectsPerCore, &r1.maxSupportedInsertsPerCore); err != nil {
			return nil, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", query, err)
		}
		colocatedLimits = append(colocatedLimits, r1)
	}
	return colocatedLimits, nil
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
func loadSourceMetadata(assessmentMetadataDir string) ([]SourceDBMetadata, []SourceDBMetadata, float64, error) {
	filePath := GetSourceMetadataDBFilePath()
	if filePath == "assessment.db" {
		if AssessmentDir == "" {
			filePath = filepath.Join(assessmentMetadataDir, "assessment.db")
		} else {
			filePath = filepath.Join(AssessmentDir, "assessment.db")
		}
	}
	fmt.Println("source metadata file path:", filePath)
	SourceMetaDB, err := utils.ConnectToSqliteDatabase(filePath)
	if err != nil {
		return nil, nil, 0.0, fmt.Errorf("cannot connect to source metadata database: %w", err)
	}
	return getSourceMetadata(SourceMetaDB)
}

/*
calculateTimeTakenAndParallelJobsForImport estimates the time taken for import of database objects based on their type, size,
and the specified CPU and memory configurations. It calculates the total size of the database objects to be migrated,
then queries experimental data to find import time estimates for similar object sizes and configurations. The
function adjusts the import time based on the ratio of the total size of the objects to be migrated to the maximum
size found in the experimental data. The import time is then converted from seconds to minutes and returned.
Parameters:

	tableName: A string indicating the type of database objects to be imported (e.g., "colocated" or "sharded").
	dbObjects: A slice containing metadata for the database objects to be migrated.
	vCPUPerInstance: The number of virtual CPUs per instance used for import.
	memPerCore: The memory allocated per CPU core used for import.

Returns:

	float64: The estimated time taken for import in minutes.
	int64: Total parallel jobs used for import.
*/
func calculateTimeTakenAndParallelJobsForImport(tableName string, dbObjects []SourceDBMetadata,
	vCPUPerInstance int, memPerCore int) (float64, int64, error) {
	// the total size of colocated objects
	var size float64 = 0
	var timeTakenOfFetchedRow float64
	var maxSizeOfFetchedRow float64
	var parallelJobs int64
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

	if err := row.Scan(&maxSizeOfFetchedRow, &timeTakenOfFetchedRow, &parallelJobs); err != nil {
		if err == sql.ErrNoRows {
			log.Errorf("No rows were returned by the query to experiment table: %v", tableName)
		} else {
			return 0.0, 0, fmt.Errorf("error while fetching import time info with query [%s]: %w", selectQuery, err)
		}
	}

	importTime := ((timeTakenOfFetchedRow * size) / maxSizeOfFetchedRow) / 60
	return math.Ceil(importTime), parallelJobs, nil
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
		err := os.WriteFile(experimentDBPath, experimentData20240, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write experiment data file: %w", err)
		}
	}

	return experimentDBPath, nil
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

	downloadPath := experimentDBPath
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
