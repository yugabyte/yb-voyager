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
	"sort"
	"strings"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

type SourceDBMetadata struct {
	SchemaName      string          `db:"schema_name"`
	ObjectName      string          `db:"object_name"`
	RowCount        sql.NullFloat64 `db:"row_count,string"`
	ColumnCount     sql.NullInt64   `db:"column_count,string"`
	Reads           sql.NullInt64   `db:"reads,string"`
	Writes          sql.NullInt64   `db:"writes,string"`
	ReadsPerSec     sql.NullInt64   `db:"reads_per_second,string"`
	WritesPerSec    sql.NullInt64   `db:"writes_per_second,string"`
	IsIndex         bool            `db:"is_index,string"`
	ParentTableName sql.NullString  `db:"parent_table_name"`
	Size            sql.NullFloat64 `db:"size_in_bytes,string"`
}

type ExpDataShardedLimit struct {
	numCores              sql.NullFloat64 `db:"num_cores,string"`
	memPerCore            sql.NullFloat64 `db:"mem_per_core,string"`
	maxSupportedNumTables sql.NullInt64   `db:"max_num_tables,string"`
}

type ExpDataColocatedLimit struct {
	maxColocatedSizeSupported sql.NullFloat64 `db:"max_colocated_db_size_gb,string"`
	numCores                  sql.NullFloat64 `db:"num_cores,string"`
	memPerCore                sql.NullFloat64 `db:"mem_per_core,string"`
	maxSupportedNumTables     sql.NullInt64   `db:"max_num_tables,string"`
	minSupportedNumTables     sql.NullFloat64 `db:"min_num_tables,string"`
}

type ExpDataThroughput struct {
	numCores                   sql.NullFloat64 `db:"num_cores,string"`
	memPerCore                 sql.NullFloat64 `db:"mem_per_core,string"`
	maxSupportedSelectsPerCore sql.NullFloat64 `db:"max_selects_per_core,string"`
	maxSupportedInsertsPerCore sql.NullFloat64 `db:"max_inserts_per_core,string"`
	selectConnPerNode          sql.NullInt64   `db:"select_conn_per_node,string"`
	insertConnPerNode          sql.NullInt64   `db:"insert_conn_per_node,string"`
}

type ExpDataLoadTime struct {
	csvSizeGB         sql.NullFloat64 `db:"csv_size_gb,string"`
	migrationTimeSecs sql.NullFloat64 `db:"migration_time_secs,string"`
	rowCount          sql.NullFloat64 `db:"row_count,string"`
}

type ExpDataLoadTimeIndexImpact struct {
	numIndexes                    sql.NullFloat64 `db:"num_indexes,string"`
	multiplicationFactorSharded   sql.NullFloat64 `db:"multiplication_factor_sharded,string"`
	multiplicationFactorColocated sql.NullFloat64 `db:"multiplication_factor_colocated,string"`
}

type ExpDataLoadTimeColumnsImpact struct {
	numColumns                    sql.NullInt64   `db:"number_of_columns,string"`
	multiplicationFactorSharded   sql.NullFloat64 `db:"multiplication_factor_sharded,string"`
	multiplicationFactorColocated sql.NullFloat64 `db:"multiplication_factor_colocated,string"`
}

type ExpDataLoadTimeNumNodesImpact struct {
	numNodes             sql.NullInt64   `db:"num_nodes,string"`
	importThroughputMbps sql.NullFloat64 `db:"import_throughput_mbps,string"`
}

type IntermediateRecommendation struct {
	ColocatedTables                                    []SourceDBMetadata
	ShardedTables                                      []SourceDBMetadata
	ColocatedSize                                      float64
	ShardedSize                                        float64
	NumNodes                                           float64
	VCPUsPerInstance                                   int
	MemoryPerCore                                      int
	OptimalSelectConnectionsPerNode                    int64
	OptimalInsertConnectionsPerNode                    int64
	EstimatedTimeInMinForImport                        float64
	EstimatedTimeInMinForImportWithoutRedundantIndexes float64
	FailureReasoning                                   string
	CoresNeeded                                        float64
}

type ExperimentDataAvailableYbVersion struct {
	versionId        int64
	expDataYbVersion *ybversion.YBVersion
	expDataIsDefault bool
}

const (
	COLOCATED_LIMITS_TABLE         = "colocated_limits"
	COLOCATED_SIZING_TABLE         = "colocated_sizing"
	SHARDED_SIZING_TABLE           = "sharded_sizing"
	COLOCATED_LOAD_TIME_TABLE      = "colocated_load_time"
	SHARDED_LOAD_TIME_TABLE        = "sharded_load_time"
	LOAD_TIME_INDEX_IMPACT_TABLE   = "load_time_index_impact"
	LOAD_TIME_COLUMNS_IMPACT_TABLE = "load_time_columns_impact"
	LOAD_TIME_NUM_NODES_IMPACT     = "load_time_num_nodes_impact"
	// GITHUB_RAW_LINK use raw github link to fetch the file from repository using the api:
	// https://raw.githubusercontent.com/{username-or-organization}/{repository}/{branch}/{path-to-file}
	GITHUB_RAW_LINK                 = "https://raw.githubusercontent.com/yugabyte/yb-voyager/main/yb-voyager/src/migassessment/resources"
	EXPERIMENT_DATA_FILENAME        = "yb_experiment_data_source.db"
	DBS_DIR                         = "dbs"
	SIZE_UNIT_GB                    = "GB"
	SIZE_UNIT_MB                    = "MB"
	LOW_PHASE_SHARD_COUNT           = 1
	LOW_PHASE_SIZE_THRESHOLD_GB     = 0.512
	HIGH_PHASE_SHARD_COUNT          = 24
	HIGH_PHASE_SIZE_THRESHOLD_GB    = 10
	FINAL_PHASE_SIZE_THRESHOLD_GB   = 100
	MAX_TABLETS_PER_TABLE           = 256
	PREFER_REMOTE_EXPERIMENT_DB     = false
	COLOCATED_MAX_INDEXES_THRESHOLD = 2
	// COLOCATED / SHARDED Object types
	COLOCATED = "colocated"
	SHARDED   = "sharded"
)

func getExperimentDBPath(assessmentDir string) string {
	if AssessmentDir == "" {
		return filepath.Join(assessmentDir, DBS_DIR, EXPERIMENT_DATA_FILENAME)
	} else {
		return filepath.Join(AssessmentDir, DBS_DIR, EXPERIMENT_DATA_FILENAME)
	}
}

//go:embed resources/yb_experiment_data_source.db
var experimentData []byte

var SourceMetadataObjectTypesToUse = []string{
	"%table%",
	"%index%",
	"materialized view",
}

func SizingAssessment(targetDbVersion *ybversion.YBVersion, sourceDBType string, assessmentDir string) error {

	log.Infof("loading metadata files for sharding assessment")
	sourceTableMetadata, sourceIndexMetadata, sourceUniqueIndexesMetadata, _, err := loadSourceMetadata(GetSourceMetadataDBFilePath(), sourceDBType, assessmentDir)

	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("failed to load source metadata: %v", err)
		return fmt.Errorf("failed to load source metadata: %w", err)
	}

	experimentDB, err := createConnectionToExperimentData(assessmentDir)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("failed to connect to experiment data: %v", err)
		return fmt.Errorf("failed to connect to experiment data: %w", err)
	}

	// fetch yb versions with available experiment data, use default version if experiment data of supported release is
	// not available
	experimentDbAvailableYbVersions, defaultYbVersionId, err := loadYbVersionsWithExperimentData(experimentDB)
	ybVersionIdToUse := defaultYbVersionId
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("failed to load yb versions: %v", err)
		return fmt.Errorf("failed to load yb versions: %w", err)
	}
	if len(experimentDbAvailableYbVersions) != 0 {
		// find closest yb version from experiment data to targetYbVersion or default
		ybVersionIdToUse = findClosestVersion(targetDbVersion, experimentDbAvailableYbVersions, defaultYbVersionId)
	}
	log.Infof(fmt.Sprintf("Experiment data yb version id used for sizing assessment: %v\n", ybVersionIdToUse))
	fmt.Printf("Experiment data yb version id used for sizing assessment: %v\n", ybVersionIdToUse)

	colocatedLimits, err := loadColocatedLimit(experimentDB, ybVersionIdToUse)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("error fetching the colocated limits: %v", err)
		return fmt.Errorf("error fetching the colocated limits: %w", err)
	}

	colocatedThroughput, err := loadExpDataThroughput(experimentDB, COLOCATED_SIZING_TABLE, ybVersionIdToUse)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("error fetching the colocated throughput: %v", err)
		return fmt.Errorf("error fetching the colocated throughput: %w", err)
	}

	shardedLimits, err := loadShardedTableLimits(experimentDB, ybVersionIdToUse)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("error fetching the sharded limits: %v", err)
		return fmt.Errorf("error fetching the colocated limits: %w", err)
	}

	shardedThroughput, err := loadExpDataThroughput(experimentDB, SHARDED_SIZING_TABLE, ybVersionIdToUse)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("error fetching the sharded throughput: %v", err)
		return fmt.Errorf("error fetching the sharded throughput: %w", err)
	}

	sizingRecommendationPerCore := createSizingRecommendationStructure(colocatedLimits)
	sizingRecommendationPerCoreShardingStrategy := createSizingRecommendationStructure(colocatedLimits)

	for key, rec := range sizingRecommendationPerCoreShardingStrategy {
		rec.ShardedTables = append(rec.ShardedTables, sourceTableMetadata...)
		sizingRecommendationPerCoreShardingStrategy[key] = rec
	}

	sizingRecommendationPerCore = shardingBasedOnTableSizeAndCount(sourceTableMetadata, sourceIndexMetadata,
		colocatedLimits, sizingRecommendationPerCore)

	sizingRecommendationPerCore = shardingBasedOnOperations(sourceIndexMetadata, colocatedThroughput, sizingRecommendationPerCore)

	sizingRecommendationPerCore = checkShardedTableLimit(sourceIndexMetadata, shardedLimits, sizingRecommendationPerCore)

	sizingRecommendationPerCoreShardingStrategy = checkShardedTableLimit(sourceIndexMetadata, shardedLimits, sizingRecommendationPerCoreShardingStrategy)

	sizingRecommendationPerCore = findNumNodesNeededBasedOnThroughputRequirement(sourceIndexMetadata, shardedThroughput, sizingRecommendationPerCore)
	sizingRecommendationPerCoreShardingStrategy = findNumNodesNeededBasedOnThroughputRequirement(sourceIndexMetadata, shardedThroughput, sizingRecommendationPerCoreShardingStrategy)

	sizingRecommendationPerCore = findNumNodesNeededBasedOnTabletsRequired(sourceIndexMetadata, shardedLimits, sizingRecommendationPerCore)
	sizingRecommendationPerCoreShardingStrategy = findNumNodesNeededBasedOnTabletsRequired(sourceIndexMetadata, shardedLimits, sizingRecommendationPerCoreShardingStrategy)

	finalSizingRecommendationColoShardedCombined := pickBestRecommendation(sizingRecommendationPerCore)
	if finalSizingRecommendationColoShardedCombined.FailureReasoning != "" {
		SizingReport.FailureReasoning = finalSizingRecommendationColoShardedCombined.FailureReasoning
		return fmt.Errorf("error picking best recommendation: %v", finalSizingRecommendationColoShardedCombined.FailureReasoning)
	}
	finalSizingRecommendationAllSharded := pickBestRecommendation(sizingRecommendationPerCoreShardingStrategy)
	if finalSizingRecommendationAllSharded.FailureReasoning != "" {
		SizingReport.FailureReasoning = finalSizingRecommendationAllSharded.FailureReasoning
		return fmt.Errorf("error picking best recommendation: %v", finalSizingRecommendationAllSharded.FailureReasoning)
	}

	finalSizingRecommendation, pickReasoning := pickBestOutOfTwo(finalSizingRecommendationColoShardedCombined, finalSizingRecommendationAllSharded, sourceIndexMetadata)

	colocatedObjects, cumulativeIndexCountColocated :=
		getListOfIndexesAlongWithObjects(finalSizingRecommendation.ColocatedTables, sourceIndexMetadata)
	shardedObjects, cumulativeIndexCountSharded :=
		getListOfIndexesAlongWithObjects(finalSizingRecommendation.ShardedTables, sourceIndexMetadata)

	// get load times data from experimental database for colocated Tables
	colocatedLoadTimes, err := getExpDataLoadTime(experimentDB, finalSizingRecommendation.VCPUsPerInstance,
		finalSizingRecommendation.MemoryPerCore, COLOCATED_LOAD_TIME_TABLE, ybVersionIdToUse)
	if err != nil {
		return fmt.Errorf("error while fetching colocated load time info: %w", err)
	}

	// get load times data from experimental database for sharded Tables
	shardedLoadTimes, err := getExpDataLoadTime(experimentDB, finalSizingRecommendation.VCPUsPerInstance,
		finalSizingRecommendation.MemoryPerCore, SHARDED_LOAD_TIME_TABLE, ybVersionIdToUse)
	if err != nil {
		return fmt.Errorf("error while fetching sharded load time info: %w", err)
	}

	// get experimental data for impact of indexes on import time
	indexImpactOnLoadTimeCommon, err := getExpDataIndexImpactOnLoadTime(experimentDB,
		finalSizingRecommendation.VCPUsPerInstance, finalSizingRecommendation.MemoryPerCore, ybVersionIdToUse)
	if err != nil {
		return fmt.Errorf("error while fetching experiment data for impact of index on load time: %w", err)
	}

	// get experimental data for impact of number of columns on import time
	columnsImpactOnLoadTimeCommon, err := getExpDataNumColumnsImpactOnLoadTime(experimentDB,
		finalSizingRecommendation.VCPUsPerInstance, finalSizingRecommendation.MemoryPerCore, ybVersionIdToUse)
	if err != nil {
		return fmt.Errorf("error while fetching experiment data for impact of number of columns on load time: %w", err)
	}

	// get experimental data for throughput scaling with number of nodes
	numNodesImpactOnLoadTimeSharded, err := getExpDataNumNodesImpactOnLoadTime(experimentDB,
		finalSizingRecommendation.VCPUsPerInstance, finalSizingRecommendation.MemoryPerCore, ybVersionIdToUse)
	if err != nil {
		return fmt.Errorf("error while fetching experiment data for throughput scaling with number of nodes: %w", err)
	}

	// calculate time taken for colocated import
	numNodesImportTimeDivisorColocated := 1.0
	importTimeForColocatedObjects, importTimeForColocatedObjectsWithoutRedundantIndexes, err := calculateTimeTakenForImport(
		finalSizingRecommendation.ColocatedTables, sourceUniqueIndexesMetadata, sourceIndexMetadata, colocatedLoadTimes,
		indexImpactOnLoadTimeCommon, columnsImpactOnLoadTimeCommon, COLOCATED, numNodesImportTimeDivisorColocated)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("calculate time taken for colocated data import: %v", err)
		return fmt.Errorf("calculate time taken for colocated data import: %w", err)
	}

	// find ratio of throughput of 3 nodes from experiment data vs the closest throughput of the recommended number of nodes
	// if the number of nodes is 3, then no scaling is needed
	numNodesImportTimeDivisorSharded := lo.Ternary(finalSizingRecommendation.NumNodes == 3, 1.0,
		findNumNodesThroughputScalingImportTimeDivisor(numNodesImpactOnLoadTimeSharded, finalSizingRecommendation.NumNodes))

	// calculate time taken for sharded import
	importTimeForShardedObjects, importTimeForShardedObjectsWithoutRedundantIndexes, err := calculateTimeTakenForImport(
		finalSizingRecommendation.ShardedTables, sourceUniqueIndexesMetadata, sourceIndexMetadata, shardedLoadTimes,
		indexImpactOnLoadTimeCommon, columnsImpactOnLoadTimeCommon, SHARDED, numNodesImportTimeDivisorSharded)
	if err != nil {
		SizingReport.FailureReasoning = fmt.Sprintf("calculate time taken for sharded data import: %v", err)
		return fmt.Errorf("calculate time taken for sharded data import: %w", err)
	}
	reasoning := getReasoning(finalSizingRecommendation, shardedObjects, cumulativeIndexCountSharded, colocatedObjects,
		cumulativeIndexCountColocated)

	reasoning += pickReasoning

	sizingRecommendation := &SizingRecommendation{
		ColocatedTables:                 fetchObjectNames(finalSizingRecommendation.ColocatedTables),
		ShardedTables:                   fetchObjectNames(finalSizingRecommendation.ShardedTables),
		VCPUsPerInstance:                finalSizingRecommendation.VCPUsPerInstance,
		MemoryPerInstance:               finalSizingRecommendation.VCPUsPerInstance * finalSizingRecommendation.MemoryPerCore,
		NumNodes:                        finalSizingRecommendation.NumNodes,
		OptimalSelectConnectionsPerNode: finalSizingRecommendation.OptimalSelectConnectionsPerNode,
		OptimalInsertConnectionsPerNode: finalSizingRecommendation.OptimalInsertConnectionsPerNode,
		ColocatedReasoning:              reasoning,
		EstimatedTimeInMinForImport:     importTimeForColocatedObjects + importTimeForShardedObjects,
		EstimatedTimeInMinForImportWithoutRedundantIndexes: importTimeForColocatedObjectsWithoutRedundantIndexes + importTimeForShardedObjectsWithoutRedundantIndexes,
	}
	SizingReport.SizingRecommendation = *sizingRecommendation

	return nil
}

/*
pickBestRecommendation selects the best recommendation from a map of recommendations by optimizing for the cores. Hence,
we chose the setup where the number of cores is less.
Parameters:
  - recommendations: A map where the key is the number of vCPUs per instance and the value is an IntermediateRecommendation struct.

Returns:
  - The best IntermediateRecommendation based on the defined criteria.
*/
func pickBestRecommendation(recommendations map[int]IntermediateRecommendation) IntermediateRecommendation {
	var recs []IntermediateRecommendation
	for _, v := range recommendations {
		recs = append(recs, v)
	}
	// descending order sort
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].VCPUsPerInstance > recs[j].VCPUsPerInstance
	})

	// find the one with the least number of nodes
	var minCores int = math.MaxUint32
	var finalRecommendation IntermediateRecommendation
	var foundRecommendation bool = false
	var maxCores int = math.MinInt32

	// Iterate over each recommendation
	for _, rec := range recs {
		// Update maxCores with the maximum number of vCPUs per instance across recommendations. If none of the cores
		// abe to satisfy the criteria, recommendation with maxCores will be used as final recommendation
		if maxCores < rec.VCPUsPerInstance {
			maxCores = rec.VCPUsPerInstance
		}
		// Check if the recommendation has no failure reasoning (i.e., it's a valid recommendation)
		if rec.FailureReasoning == "" {
			foundRecommendation = true
			// Update finalRecommendation if the current recommendation has fewer cores.
			log.Infof(fmt.Sprintf("vCPU: %v & cores required: %v gives nodes required: %v\n", rec.VCPUsPerInstance, rec.CoresNeeded, rec.NumNodes))
			if minCores > int(rec.CoresNeeded) {
				finalRecommendation = rec
				minCores = int(rec.CoresNeeded)
			} else if minCores == int(rec.CoresNeeded) {
				// If the number of cores is the same across machines, recommend the machine with higher core count
				if rec.VCPUsPerInstance > finalRecommendation.VCPUsPerInstance {
					finalRecommendation = rec
				}
			}
		}
	}
	// If no valid recommendation was found, select the recommendation with the maximum number of cores
	if !foundRecommendation {
		finalRecommendation = recommendations[maxCores]
		// notify customers to reach out to the Yugabyte customer support team for further assistance
		finalRecommendation.FailureReasoning = "Unable to determine appropriate sizing recommendation. Reach out to the Yugabyte customer support team at https://support.yugabyte.com for further assistance."
	}

	// Return the best recommendation
	return finalRecommendation
}

/*
pickBestOutOfTwo selects the best recommendation from two recommendations by choosing the one with fewer cores needed.
It returns the selected recommendation along with reasoning explaining the choice.

Parameters:
  - rec1: First IntermediateRecommendation to compare
  - rec2: Second IntermediateRecommendation to compare
  - sourceIndexMetadata: A slice of SourceDBMetadata structs representing source indexes

Returns:
  - The best IntermediateRecommendation based on fewer cores needed
  - A string explaining the reasoning behind the choice, including cores comparison and detailed object counts
*/
func pickBestOutOfTwo(rec1, rec2 IntermediateRecommendation, sourceIndexMetadata []SourceDBMetadata) (IntermediateRecommendation, string) {
	var selectedRec, notSelectedRec IntermediateRecommendation
	var reasoning string

	// Compare cores needed - pick the one with fewer cores, preferring all-sharded when equal
	if rec1.CoresNeeded < rec2.CoresNeeded {
		selectedRec = rec1
		notSelectedRec = rec2
		reasoning = fmt.Sprintf("\nSelected colocated+sharded strategy requiring %.0f cores over all-sharded strategy requiring %.0f cores. ",
			rec1.CoresNeeded, rec2.CoresNeeded)
	} else if rec1.CoresNeeded > rec2.CoresNeeded {
		selectedRec = rec2
		notSelectedRec = rec1
		reasoning = fmt.Sprintf("\nSelected all-sharded strategy requiring %.0f cores over colocated+sharded strategy requiring %.0f cores. ",
			rec2.CoresNeeded, rec1.CoresNeeded)
	} else {
		// Equal cores - prefer all-sharded (rec2)
		selectedRec = rec2
		notSelectedRec = rec1
		reasoning = fmt.Sprintf("\nSelected all-sharded strategy (same %.0f cores as colocated+sharded strategy, preferring all-sharded). ",
			rec2.CoresNeeded)
	}

	// Get detailed object counts for non-selected recommendations
	_, notSelectedColocatedIndexCount := getListOfIndexesAlongWithObjects(notSelectedRec.ColocatedTables, sourceIndexMetadata)
	_, notSelectedShardedIndexCount := getListOfIndexesAlongWithObjects(notSelectedRec.ShardedTables, sourceIndexMetadata)

	// Calculate table counts
	notSelectedColocatedTableCount := len(notSelectedRec.ColocatedTables)
	notSelectedShardedTableCount := len(notSelectedRec.ShardedTables)

	// Add detailed information about not selected recommendations
	reasoning += fmt.Sprintf("Non-selected recommendation: %d colocated objects (%d tables, %d indexes) and %d sharded objects (%d tables, %d indexes).",
		notSelectedColocatedTableCount+notSelectedColocatedIndexCount, notSelectedColocatedTableCount, notSelectedColocatedIndexCount,
		notSelectedShardedTableCount+notSelectedShardedIndexCount, notSelectedShardedTableCount, notSelectedShardedIndexCount)

	return selectedRec, reasoning
}

/*
findNumNodesNeededBasedOnThroughputRequirement calculates the number of nodes needed based on sharded throughput limits and updates the recommendation accordingly.
Parameters:
  - sourceIndexMetadata: A slice of SourceDBMetadata structs representing source indexes.
  - shardedThroughputSlice: A slice of ExpDataShardedThroughput structs representing sharded throughput limits.
  - recommendation: A map where the key is the number of vCPUs per instance and the value is an IntermediateRecommendation struct.

Returns:
  - An updated map of recommendations with the number of nodes needed.
*/
func findNumNodesNeededBasedOnThroughputRequirement(sourceIndexMetadata []SourceDBMetadata, shardedThroughputSlice []ExpDataThroughput,
	recommendation map[int]IntermediateRecommendation) map[int]IntermediateRecommendation {
	// Iterate over sharded throughput limits
	for _, shardedThroughput := range shardedThroughputSlice {
		// Get previous recommendation for the current num of cores
		previousRecommendation := recommendation[int(shardedThroughput.numCores.Float64)]
		var cumulativeSelectOpsPerSec int64 = 0
		var cumulativeInsertOpsPerSec int64 = 0

		// Calculate cumulative operations per second for sharded tables
		for _, table := range previousRecommendation.ShardedTables {
			// Check and fetch indexes for the current table
			_, _, indexReads, indexWrites := checkAndFetchIndexes(table, sourceIndexMetadata)
			cumulativeSelectOpsPerSec += lo.Ternary(table.ReadsPerSec.Valid, table.ReadsPerSec.Int64, 0) + indexReads
			cumulativeInsertOpsPerSec += lo.Ternary(table.WritesPerSec.Valid, table.WritesPerSec.Int64, 0) + indexWrites
		}

		// Calculate needed cores based on cumulative operations per second
		neededCores :=
			math.Ceil(float64(cumulativeSelectOpsPerSec)/shardedThroughput.maxSupportedSelectsPerCore.Float64 +
				float64(cumulativeInsertOpsPerSec)/shardedThroughput.maxSupportedInsertsPerCore.Float64)

		nodesNeeded := math.Ceil(neededCores / shardedThroughput.numCores.Float64)
		// Assumption: If there are any colocated objects - one node will be utilized as colocated tablet leader.
		// Add it explicitly.
		if len(previousRecommendation.ColocatedTables) > 0 {
			nodesNeeded += 1
			neededCores += float64(previousRecommendation.VCPUsPerInstance)
		}

		// Assumption: minimum required replication is 3, so minimum nodes recommended would be 3.
		// Choose max of nodes needed and 3 and same for neededCores.
		nodesNeeded = math.Max(nodesNeeded, 3)
		neededCores = math.Max(neededCores, float64(previousRecommendation.VCPUsPerInstance*3))

		// Update recommendation with the number of nodes needed
		recommendation[int(shardedThroughput.numCores.Float64)] = IntermediateRecommendation{
			ColocatedTables:                 previousRecommendation.ColocatedTables,
			ShardedTables:                   previousRecommendation.ShardedTables,
			VCPUsPerInstance:                previousRecommendation.VCPUsPerInstance,
			MemoryPerCore:                   previousRecommendation.MemoryPerCore,
			NumNodes:                        nodesNeeded,
			OptimalSelectConnectionsPerNode: int64(math.Min(float64(previousRecommendation.OptimalSelectConnectionsPerNode), float64(shardedThroughput.selectConnPerNode.Int64))),
			OptimalInsertConnectionsPerNode: int64(math.Min(float64(previousRecommendation.OptimalInsertConnectionsPerNode), float64(shardedThroughput.insertConnPerNode.Int64))),
			ColocatedSize:                   previousRecommendation.ColocatedSize,
			ShardedSize:                     previousRecommendation.ShardedSize,
			EstimatedTimeInMinForImport:     previousRecommendation.EstimatedTimeInMinForImport,
			EstimatedTimeInMinForImportWithoutRedundantIndexes: previousRecommendation.EstimatedTimeInMinForImportWithoutRedundantIndexes,
			FailureReasoning: previousRecommendation.FailureReasoning,
			CoresNeeded:      neededCores,
		}
	}
	// Return updated recommendation map
	return recommendation
}

/*
findNumNodesNeededBasedOnTabletsRequired calculates the number of nodes needed based on tablets required by each
table and its indexes and updates the recommendation accordingly.
Parameters:
  - sourceIndexMetadata: A slice of SourceDBMetadata structs representing source indexes.
  - shardedLimits: A slice of ExpDataShardedThroughput structs representing sharded throughput limits.
  - recommendation: A map where the key is the number of vCPUs per instance and the value is an IntermediateRecommendation struct.

Returns:
  - An updated map of recommendations with the number of nodes needed.
*/
func findNumNodesNeededBasedOnTabletsRequired(sourceIndexMetadata []SourceDBMetadata,
	shardedLimits []ExpDataShardedLimit,
	recommendation map[int]IntermediateRecommendation) map[int]IntermediateRecommendation {
	// Iterate over each intermediate recommendation where failureReasoning is empty
	for i, rec := range recommendation {
		totalTabletsRequired := 0
		if len(rec.ShardedTables) != 0 && rec.FailureReasoning == "" {
			// Iterate over each table and its indexes to find out how many tablets are needed
			for _, table := range rec.ShardedTables {
				_, tabletsRequired := getThresholdAndTablets(rec.NumNodes, lo.Ternary(table.Size.Valid, table.Size.Float64, 0))
				for _, index := range sourceIndexMetadata {
					if index.ParentTableName.Valid && (index.ParentTableName.String == (table.SchemaName + "." + table.ObjectName)) {
						// calculating tablets required for each of the index
						_, tabletsRequiredForIndex := getThresholdAndTablets(rec.NumNodes, lo.Ternary(index.Size.Valid, index.Size.Float64, 0))
						// tablets required for each table is the sum of tablets required for the table and its indexes
						tabletsRequired += tabletsRequiredForIndex
					}
				}
				// adding total tablets required across all tables
				totalTabletsRequired += tabletsRequired
			}
			// assuming table limits is also a tablet limit
			// get shardedLimit of current recommendation
			for _, record := range shardedLimits {
				if record.numCores.Valid && int(record.numCores.Float64) == rec.VCPUsPerInstance {
					// considering RF=3, hence total required tablets would be 3 times(1 tablet leader and 2 followers) the totalTabletsRequired
					// adding 100% buffer for the tablets required by multiplier of 2
					nodesRequired := math.Ceil(float64(totalTabletsRequired*3*2) / float64(record.maxSupportedNumTables.Int64))
					// update recommendation to use the maximum of the existing recommended nodes and nodes calculated based on tablets
					// Caveat: if new nodes required is more than the existing recommended nodes, we would need to
					// re-evaluate tablets required. Although, in this iteration we've skipping re-evaluation.
					currentMaxRequiredNodes := math.Max(nodesRequired, rec.NumNodes)
					rec.CoresNeeded = lo.Ternary(nodesRequired <= rec.NumNodes, rec.CoresNeeded, currentMaxRequiredNodes*float64(rec.VCPUsPerInstance))
					rec.NumNodes = currentMaxRequiredNodes
					recommendation[i] = rec
				}
			}
		}
	}
	// return updated recommendations
	return recommendation
}

/*
getThresholdAndTablets determines the size threshold and number of tablets needed for a given table size.

Parameters:
- previousNumNodes: float64 - The number of nodes in the previous recommendation.
- sizeGB: float64 - The size of the table in gigabytes.

Returns:
- float64: The size threshold in gigabytes for the table based on the phase.
- int: The number of tablets needed for the table.

Description:
This function calculates which size threshold applies to a table based on its size and determines the number of tablets required.
Following details/comments are with assumption that the previous recommended nodes is 3.
Similar works for other recommended nodes as well.:
  - For sizes up to the low phase limit (1*3 shards of 512 MB each, up to 1.5 GB), the low phase threshold is used. Where 1 is low phase shard count and 3 is the previous-recommended nodes.
  - After 1*3 shards, the high phase threshold is used.
  - Intermediate phase upto 30 GB is calculated based on 3 tablets of 10 GB each.
  - For sizes up to the high phase limit (72(24*3) shards of 10 GB each, up to 720 GB), the high phase threshold is used.
  - For larger sizes, the final phase threshold (100 GB) is used.
*/
func getThresholdAndTablets(previousNumNodes float64, sizeGB float64) (float64, int) {
	var tablets = math.Ceil(sizeGB / LOW_PHASE_SIZE_THRESHOLD_GB)

	if tablets <= (LOW_PHASE_SHARD_COUNT * previousNumNodes) {
		// table size is less than 1.5GB, hence 1*3 tablets of 512MB each will be enough
		return LOW_PHASE_SIZE_THRESHOLD_GB, int(tablets)
	} else {
		// table size is more than 1.5GB.
		// find out the per tablet size if it is less than 10GB which is high phase threshold
		perTabletSize := sizeGB / (LOW_PHASE_SHARD_COUNT * previousNumNodes)
		if perTabletSize <= HIGH_PHASE_SIZE_THRESHOLD_GB {
			// tablet count is still 1*3 but the size of each tablet is less than 10GB(table size < 30GB).
			return HIGH_PHASE_SIZE_THRESHOLD_GB, int(LOW_PHASE_SHARD_COUNT * previousNumNodes)
		} else {
			// table size is > 30GB, hence we need to increase the tablet count
			tablets = math.Ceil(LOW_PHASE_SHARD_COUNT*previousNumNodes + (sizeGB-LOW_PHASE_SHARD_COUNT*previousNumNodes*HIGH_PHASE_SIZE_THRESHOLD_GB)/HIGH_PHASE_SIZE_THRESHOLD_GB)
			if tablets <= (HIGH_PHASE_SHARD_COUNT * previousNumNodes) {
				// this means that table size is less than 720GB, hence 72(24*3) tablets of 10GB each will be enough
				return HIGH_PHASE_SIZE_THRESHOLD_GB, int(tablets)
			} else {
				// table size is more than 720 GB.
				// find out the per tablet size if it is less than 100GB which is final phase threshold
				perTabletSize = sizeGB / HIGH_PHASE_SHARD_COUNT
				if perTabletSize <= FINAL_PHASE_SIZE_THRESHOLD_GB {
					// tablet count is still 72(24*3) but the size of each tablet is less than 100GB(table size < 7200GB).
					return FINAL_PHASE_SIZE_THRESHOLD_GB, int(HIGH_PHASE_SHARD_COUNT * previousNumNodes)
				} else {
					// table size is > 7200GB, hence we need to increase the tablet count
					tablets = math.Ceil(HIGH_PHASE_SHARD_COUNT*previousNumNodes + (sizeGB-HIGH_PHASE_SHARD_COUNT*previousNumNodes*FINAL_PHASE_SIZE_THRESHOLD_GB)/FINAL_PHASE_SIZE_THRESHOLD_GB)
					if tablets <= (MAX_TABLETS_PER_TABLE * previousNumNodes) {
						// this means that table size is less than 76800GB. So 768(256*3) tablets of 100GB each will be enough
						return FINAL_PHASE_SIZE_THRESHOLD_GB, int(tablets)
					} else {
						// to support table size > 76800GB, tablets per table limit in YugabyteDB needs to be
						// set to 0(meaning no limit). Refer doc:
						//https://docs.yugabyte.com/preview/architecture/docdb-sharding/tablet-splitting/#final-phase
						tablets = math.Ceil(MAX_TABLETS_PER_TABLE + (sizeGB-MAX_TABLETS_PER_TABLE*previousNumNodes*FINAL_PHASE_SIZE_THRESHOLD_GB)/FINAL_PHASE_SIZE_THRESHOLD_GB)
						return FINAL_PHASE_SIZE_THRESHOLD_GB, int(tablets)
					}
				}
			}
		}
	}
}

/*
checkShardedTableLimit checks if the total number of sharded tables exceeds the sharded limit for each core configuration.
If the limit is exceeded, it updates the recommendation with a failure reasoning.
Parameters:
  - sourceIndexMetadata: A slice of SourceDBMetadata structs representing source indexes.
  - shardedLimits: A slice of ExpDataShardedLimit structs representing sharded limits.
  - recommendation: A map where the key is the number of vCPUs per instance and the value is an IntermediateRecommendation struct.

Returns:
  - An updated map of recommendations with failure reasoning if the sharded table limit is exceeded.
*/
func checkShardedTableLimit(sourceIndexMetadata []SourceDBMetadata, shardedLimits []ExpDataShardedLimit, recommendation map[int]IntermediateRecommendation) map[int]IntermediateRecommendation {

	for _, shardedLimit := range shardedLimits {
		var totalObjectCount int64 = 0

		// Get previous recommendation for the current cores
		previousRecommendation := recommendation[int(shardedLimit.numCores.Float64)]

		// Calculate total object count for sharded tables
		for _, table := range previousRecommendation.ShardedTables {
			// Check and fetch indexes for the current table
			indexes, _, _, _ := checkAndFetchIndexes(table, sourceIndexMetadata)
			totalObjectCount += int64(len(indexes)) + 1

		}
		// Check if total object count exceeds the max supported num tables limit
		if totalObjectCount > shardedLimit.maxSupportedNumTables.Int64 {
			// Generate failure reasoning
			failureReasoning := fmt.Sprintf("Cannot support %v sharded objects on a machine with %v cores "+
				"and %vGiB memory.", totalObjectCount, previousRecommendation.VCPUsPerInstance,
				previousRecommendation.VCPUsPerInstance*previousRecommendation.MemoryPerCore)

			// Update recommendation with failure reasoning
			recommendation[int(shardedLimit.numCores.Float64)] = IntermediateRecommendation{
				ColocatedTables:                 []SourceDBMetadata{},
				ShardedTables:                   []SourceDBMetadata{},
				VCPUsPerInstance:                previousRecommendation.VCPUsPerInstance,
				MemoryPerCore:                   previousRecommendation.MemoryPerCore,
				NumNodes:                        previousRecommendation.NumNodes,
				OptimalSelectConnectionsPerNode: previousRecommendation.OptimalSelectConnectionsPerNode,
				OptimalInsertConnectionsPerNode: previousRecommendation.OptimalInsertConnectionsPerNode,
				ColocatedSize:                   0,
				ShardedSize:                     0,
				EstimatedTimeInMinForImport:     previousRecommendation.EstimatedTimeInMinForImport,
				EstimatedTimeInMinForImportWithoutRedundantIndexes: previousRecommendation.EstimatedTimeInMinForImportWithoutRedundantIndexes,
				FailureReasoning: failureReasoning,
			}
		}
	}
	// Return updated recommendation map
	return recommendation
}

/*
shardingBasedOnOperations performs sharding based on operations (reads and writes) per second, taking into account colocated limits.
It updates the existing recommendations with information about colocated and sharded tables based on operations.
Parameters:
  - sourceIndexMetadata: A slice of SourceDBMetadata structs representing source indexes.
  - colocatedThroughput: A slice of ExpDataThroughput structs representing colocated limits.
  - recommendation: A map where the key is the number of vCPUs per instance and the value is an IntermediateRecommendation struct.

Returns:
  - An updated map of recommendations where sharding information based on operations has been incorporated.
*/
func shardingBasedOnOperations(sourceIndexMetadata []SourceDBMetadata,
	colocatedThroughputSlice []ExpDataThroughput, recommendation map[int]IntermediateRecommendation) map[int]IntermediateRecommendation {

	for _, colocatedThroughput := range colocatedThroughputSlice {
		var colocatedObjects []SourceDBMetadata
		var cumulativeColocatedSizeSum float64 = 0
		var numColocated int = 0
		var cumulativeSelectOpsPerSec int64 = 0
		var cumulativeInsertOpsPerSec int64 = 0

		// Get previous recommendation for the current num of cores
		previousRecommendation := recommendation[int(colocatedThroughput.numCores.Float64)]

		for _, table := range previousRecommendation.ColocatedTables {
			// Check and fetch indexes for the current table
			_, indexesSizeSum, indexReads, indexWrites := checkAndFetchIndexes(table, sourceIndexMetadata)

			// Calculate new operations per second
			newSelectOpsPerSec := cumulativeSelectOpsPerSec + lo.Ternary(table.ReadsPerSec.Valid, table.ReadsPerSec.Int64, 0) + indexReads
			newInsertOpsPerSec := cumulativeInsertOpsPerSec + lo.Ternary(table.WritesPerSec.Valid, table.WritesPerSec.Int64, 0) + indexWrites

			// Calculate total object size
			objectTotalSize := lo.Ternary(table.Size.Valid, table.Size.Float64, 0) + indexesSizeSum

			// Calculate needed cores based on operations
			neededCores :=
				math.Ceil(float64(newSelectOpsPerSec)/colocatedThroughput.maxSupportedSelectsPerCore.Float64 +
					float64(newInsertOpsPerSec)/colocatedThroughput.maxSupportedInsertsPerCore.Float64)

			if neededCores <= colocatedThroughput.numCores.Float64 {
				// Update cumulative counts and add table to colocated objects
				colocatedObjects = append(colocatedObjects, table)
				cumulativeSelectOpsPerSec = newSelectOpsPerSec
				cumulativeInsertOpsPerSec = newInsertOpsPerSec
				cumulativeColocatedSizeSum += objectTotalSize
				numColocated++
			} else {
				// Break the loop if needed cores are more than current
				break
			}
		}
		shardedObjects := previousRecommendation.ShardedTables
		var cumulativeSizeSharded float64 = 0

		// Iterate over remaining colocated tables for sharding
		for _, remainingTable := range previousRecommendation.ColocatedTables[numColocated:] {
			shardedObjects = append(shardedObjects, remainingTable)
			_, indexesSizeSumSharded, _, _ := checkAndFetchIndexes(remainingTable, sourceIndexMetadata)
			cumulativeSizeSharded += lo.Ternary(remainingTable.Size.Valid, remainingTable.Size.Float64, 0) + indexesSizeSumSharded
		}

		// Update recommendation for the current colocated limit
		recommendation[int(colocatedThroughput.numCores.Float64)] = IntermediateRecommendation{
			ColocatedTables:                 colocatedObjects,
			ShardedTables:                   shardedObjects,
			VCPUsPerInstance:                previousRecommendation.VCPUsPerInstance,
			MemoryPerCore:                   previousRecommendation.MemoryPerCore,
			NumNodes:                        previousRecommendation.NumNodes,
			OptimalSelectConnectionsPerNode: colocatedThroughput.selectConnPerNode.Int64,
			OptimalInsertConnectionsPerNode: colocatedThroughput.insertConnPerNode.Int64,
			ColocatedSize:                   cumulativeColocatedSizeSum,
			ShardedSize:                     cumulativeSizeSharded,
			EstimatedTimeInMinForImport:     previousRecommendation.EstimatedTimeInMinForImport,
			EstimatedTimeInMinForImportWithoutRedundantIndexes: previousRecommendation.EstimatedTimeInMinForImportWithoutRedundantIndexes,
		}
	}
	// Return updated recommendation map
	return recommendation
}

/*
shardingBasedOnTableSizeAndCount performs sharding based on table size and count, taking into account colocated limits.
It updates the existing recommendations with information about colocated and sharded tables.
Parameters:
  - sourceTableMetadata: A slice of SourceDBMetadata structs representing source tables.
  - sourceIndexMetadata: A slice of SourceDBMetadata structs representing source indexes.
  - colocatedLimits: A slice of ExpDataColocatedLimit structs representing colocated limits.
  - recommendation: A map where the key is the number of vCPUs per instance and the value is an IntermediateRecommendation struct.

Returns:
  - An updated map of recommendations where sharding information has been incorporated.
*/
func shardingBasedOnTableSizeAndCount(sourceTableMetadata []SourceDBMetadata,
	sourceIndexMetadata []SourceDBMetadata, colocatedLimits []ExpDataColocatedLimit,
	recommendation map[int]IntermediateRecommendation) map[int]IntermediateRecommendation {

	for _, colocatedLimit := range colocatedLimits {
		var cumulativeColocatedSizeSum float64 = 0
		var colocatedObjects []SourceDBMetadata
		var numColocated int = 0
		var cumulativeObjectCount int64 = 0

		var shardedObjects []SourceDBMetadata
		var cumulativeSizeSharded float64 = 0

		for _, table := range sourceTableMetadata {
			// Check and fetch indexes for the current table
			indexesOfTable, indexesSizeSum, _, _ := checkAndFetchIndexes(table, sourceIndexMetadata)
			// DB-12363: make tables having more than COLOCATED_MAX_INDEXES_THRESHOLD indexes as sharded
			// (irrespective of size or ops requirements)
			if len(indexesOfTable) > COLOCATED_MAX_INDEXES_THRESHOLD {
				shardedObjects = append(shardedObjects, table)
				cumulativeSizeSharded += lo.Ternary(table.Size.Valid, table.Size.Float64, 0) + indexesSizeSum
				// skip to next table
				continue
			}
			// Calculate new object count and total size
			newObjectCount := cumulativeObjectCount + int64(len(indexesOfTable)) + 1
			objectTotalSize := lo.Ternary(table.Size.Valid, table.Size.Float64, 0) + indexesSizeSum
			newCumulativeSize := cumulativeColocatedSizeSum + objectTotalSize

			// Check if adding the current table exceeds max colocated size supported or max supported num tables
			if newCumulativeSize <= colocatedLimit.maxColocatedSizeSupported.Float64 &&
				(newObjectCount <= colocatedLimit.maxSupportedNumTables.Int64) {
				cumulativeObjectCount = newObjectCount
				cumulativeColocatedSizeSum = newCumulativeSize
				colocatedObjects = append(colocatedObjects, table)
				numColocated++
			} else {
				// Break the loop if colocated limits are exceeded
				break
			}
		}

		// Iterate over remaining tables for sharding
		for _, remainingTable := range sourceTableMetadata[(len(shardedObjects) + numColocated):] {
			shardedObjects = append(shardedObjects, remainingTable)
			_, indexesSizeSumSharded, _, _ := checkAndFetchIndexes(remainingTable, sourceIndexMetadata)
			cumulativeSizeSharded += lo.Ternary(remainingTable.Size.Valid, remainingTable.Size.Float64, 0) + indexesSizeSumSharded
		}
		// Update recommendation for the current colocated limit
		previousRecommendation := recommendation[int(colocatedLimit.numCores.Float64)]
		recommendation[int(colocatedLimit.numCores.Float64)] = IntermediateRecommendation{
			ColocatedTables:                 colocatedObjects,
			ShardedTables:                   shardedObjects,
			VCPUsPerInstance:                previousRecommendation.VCPUsPerInstance,
			MemoryPerCore:                   previousRecommendation.MemoryPerCore,
			NumNodes:                        previousRecommendation.NumNodes,
			OptimalSelectConnectionsPerNode: previousRecommendation.OptimalSelectConnectionsPerNode,
			OptimalInsertConnectionsPerNode: previousRecommendation.OptimalInsertConnectionsPerNode,
			ColocatedSize:                   cumulativeColocatedSizeSum,
			ShardedSize:                     cumulativeSizeSharded,
			EstimatedTimeInMinForImport:     previousRecommendation.EstimatedTimeInMinForImport,
			EstimatedTimeInMinForImportWithoutRedundantIndexes: previousRecommendation.EstimatedTimeInMinForImportWithoutRedundantIndexes,
		}
	}
	// Return updated recommendation map
	return recommendation
}

/*
loadColocatedLimit fetches colocated limits from the experiment data table.
It retrieves various limits such as maximum colocated database size, number of cores, memory per core, etc.
Returns:
  - A slice of ExpDataColocatedLimit structs containing the fetched colocated limits.
  - An error if there was any issue during the data retrieval process.
*/
func loadColocatedLimit(experimentDB *sql.DB, ybVersionIdToUse int64) ([]ExpDataColocatedLimit, error) {
	var colocatedLimits []ExpDataColocatedLimit
	query := fmt.Sprintf(`
		SELECT max_colocated_db_size_gb, 
			   num_cores, 
			   mem_per_core, 
			   max_num_tables, 
			   min_num_tables
		FROM %v 
		WHERE yb_version_id = %d 
		ORDER BY num_cores DESC
	`, COLOCATED_LIMITS_TABLE, ybVersionIdToUse)
	rows, err := experimentDB.Query(query)
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
			&r1.minSupportedNumTables); err != nil {
			return nil, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", query, err)
		}
		colocatedLimits = append(colocatedLimits, r1)
	}
	// Return fetched colocated limits
	return colocatedLimits, nil
}

/*
loadShardedTableLimits fetches sharded table limits from the experiment data table.
It retrieves information such as the number of cores, memory per core, and maximum number of tables.
Returns:
  - A slice of ExpDataShardedLimit structs containing the fetched sharded table limits.
  - An error if there was any issue during the data retrieval process.
*/
func loadShardedTableLimits(experimentDB *sql.DB, ybVersionIdToUse int64) ([]ExpDataShardedLimit, error) {
	// added num_cores >= VCPUPerInstance from colo recommendation as that is the starting point
	selectQuery := fmt.Sprintf(`
			SELECT num_cores, memory_per_core, num_tables 
			FROM %s 
			WHERE dimension LIKE '%%TableLimits-3nodeRF=3%%' 
			AND yb_version_id = %d 
			ORDER BY num_cores
		`, SHARDED_SIZING_TABLE, ybVersionIdToUse)
	rows, err := experimentDB.Query(selectQuery)

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
	// Return fetched sharded table limits
	return shardedLimits, nil
}

/*
loadExpDataThroughput fetches sharded throughput information from the experiment data table.
It retrieves data such as inserts per core, selects per core, number of cores, memory per core, etc.
Parameters:

	experimentDB: A pointer to the experiment database.
	tableName: colocated or sharded table
	ybVersionIdToUse: yb version id to use w.r.t. given target yb version.

Returns:
  - A slice of ExpDataThroughput structs containing the fetched throughput information.
  - An error if there was any issue during the data retrieval process.
*/
func loadExpDataThroughput(experimentDB *sql.DB, tableName string, ybVersionIdToUse int64) ([]ExpDataThroughput, error) {
	selectQuery := fmt.Sprintf(`
			SELECT inserts_per_core,
				   selects_per_core, 
				   num_cores, 
				   memory_per_core,
				   select_conn_per_node, 
			   	   insert_conn_per_node 
			FROM %s 
			WHERE dimension = 'MaxThroughput' 
			AND yb_version_id = %d 
			ORDER BY num_cores DESC;
	`, tableName, ybVersionIdToUse)
	rows, err := experimentDB.Query(selectQuery)
	if err != nil {
		return nil, fmt.Errorf("error while fetching throughput info with query [%s]: %w", selectQuery, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close result set for query: [%s]", selectQuery)
		}
	}()

	var shardedThroughput []ExpDataThroughput
	for rows.Next() {
		var throughput ExpDataThroughput
		if err := rows.Scan(&throughput.maxSupportedInsertsPerCore,
			&throughput.maxSupportedSelectsPerCore, &throughput.numCores,
			&throughput.memPerCore, &throughput.selectConnPerNode, &throughput.insertConnPerNode); err != nil {
			return nil, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", selectQuery, err)
		}
		shardedThroughput = append(shardedThroughput, throughput)
	}
	// Return fetched sharded throughput information
	return shardedThroughput, nil
}

/*
loadSourceMetadata connects to the assessment metadata of the source database and generates the slice of objects
for tables and indexes. It also returns the total size of source db in GB. Primary key size is not considered in the
calculation.
Returns:

	[]SourceDBMetadata: all table objects from source db
	[]SourceDBMetadata: all index objects from source db
	[]SourceDBMetadata: all index objects from source db after filtering out redundant indexes.
	float64: total size of source db
*/
func loadSourceMetadata(filePath string, sourceDBType string, assessmentDir string) ([]SourceDBMetadata, []SourceDBMetadata, []SourceDBMetadata, float64, error) {
	filePath = GetSourceMetadataDBFilePath()
	if AssessmentDir == "" {
		filePath = filepath.Join(assessmentDir, filePath)
	} else {
		filePath = filepath.Join(AssessmentDir, filePath)
	}

	SourceMetaDB, err := utils.ConnectToSqliteDatabase(filePath)
	if err != nil {
		return nil, nil, nil, 0.0, fmt.Errorf("cannot connect to source metadata database: %w", err)
	}
	return getSourceMetadata(SourceMetaDB, sourceDBType)
}

/*
createSizingRecommendationStructure generates sizing recommendations based on colocated limits.
It creates recommendations per core and returns them in a map.
Parameters:
  - colocatedLimits: A slice of ExpDataColocatedLimit structs representing colocated limits.

Returns:
  - A map where the key is the number of vCPUs per instance and the value is an IntermediateRecommendation struct.
*/
func createSizingRecommendationStructure(colocatedLimits []ExpDataColocatedLimit) map[int]IntermediateRecommendation {
	recommendationPerCore := make(map[int]IntermediateRecommendation)
	for _, colocatedLimit := range colocatedLimits {
		var sizingRecommendation IntermediateRecommendation
		sizingRecommendation.MemoryPerCore = int(colocatedLimit.memPerCore.Float64)
		sizingRecommendation.VCPUsPerInstance = int(colocatedLimit.numCores.Float64)
		// Store recommendation in the map with vCPUs per instance as the key
		recommendationPerCore[sizingRecommendation.VCPUsPerInstance] = sizingRecommendation
	}
	// Return map containing recommendations per core
	return recommendationPerCore
}

/*
calculateTimeTakenForImport estimates the time taken for import of tables.
It queries experimental data to find import time estimates for similar object sizes and configurations. For every table
, it tries to find out how much time it would table for importing that table. The function adjusts the
import time on that table by multiplying it by factor based on the indexes. The import time is also converted to
minutes and returned. This function calculates two different import times: one with all indexes (including redundant)
and one with only unique indexes (excluding redundant).
Parameters:

	tables: A slice containing metadata for the database objects to be migrated.
	sourceUniqueIndexesMetadata: A slice containing metadata for unique indexes only.
	sourceAllIndexesMetadata: A slice containing metadata for all indexes including redundant ones.
	loadTimes: Experiment data for impact of load times on tables
	indexImpactData: Data containing impact of indexes on load time.
	numColumnImpactData: Data containing impact of number of columns on load time.
	objectType: COLOCATED or SHARDED
	numNodesImportTimeDivisorCommon: Divisor for impact of number of nodes on import time.

Returns:

	float64: The estimated time taken for import in minutes with all indexes.
	float64: The estimated time taken for import in minutes without redundant indexes.
	error: Error if any
*/
func calculateTimeTakenForImport(tables []SourceDBMetadata,
	sourceUniqueIndexesMetadata []SourceDBMetadata, sourceAllIndexesMetadata []SourceDBMetadata,
	loadTimes []ExpDataLoadTime, indexImpactData []ExpDataLoadTimeIndexImpact,
	numColumnImpactData []ExpDataLoadTimeColumnsImpact,
	objectType string, numNodesImportTimeDivisorCommon float64) (float64, float64, error) {
	var importTimeWithAllIndexes float64
	var importTimeWithoutRedundantIndexes float64

	// we need to calculate the time taken for import for every table.
	// For every index, the time taken for import increases.
	// find the rows in experiment data about the approx row matching the size
	for _, table := range tables {
		// find the closest record from experiment data for the size of the table
		tableSize := lo.Ternary(table.Size.Valid, table.Size.Float64, 0)
		rowsInTable := lo.Ternary(table.RowCount.Valid, table.RowCount.Float64, 0)

		// get multiplication factor for every table based on all indexes (including redundant)
		loadTimeMultiplicationFactorWrtAllIndexes := getMultiplicationFactorForImportTimeBasedOnIndexes(table,
			sourceAllIndexesMetadata, indexImpactData, objectType)

		// get multiplication factor for every table based on unique indexes only (excluding redundant)
		loadTimeMultiplicationFactorWrtUniqueIndexes := getMultiplicationFactorForImportTimeBasedOnIndexes(table,
			sourceUniqueIndexesMetadata, indexImpactData, objectType)

		// get multiplication factor for every table based on the number of columns in the table
		loadTimeMultiplicationFactorWrtNumColumns := getMultiplicationFactorForImportTimeBasedOnNumColumns(table,
			numColumnImpactData, objectType)

		tableImportTimeSec := findImportTimeFromExpDataLoadTime(loadTimes, tableSize, rowsInTable)

		// add import time with all indexes to total import time by converting it to minutes
		importTimeWithAllIndexes += (loadTimeMultiplicationFactorWrtAllIndexes * loadTimeMultiplicationFactorWrtNumColumns * tableImportTimeSec) / 60

		// add import time without redundant indexes to total import time by converting it to minutes
		importTimeWithoutRedundantIndexes += (loadTimeMultiplicationFactorWrtUniqueIndexes * loadTimeMultiplicationFactorWrtNumColumns * tableImportTimeSec) / 60
	}

	// divide the total import time by the divisor for number of nodes.
	return math.Ceil(importTimeWithAllIndexes / numNodesImportTimeDivisorCommon),
		math.Ceil(importTimeWithoutRedundantIndexes / numNodesImportTimeDivisorCommon), nil
}

/*
getExpDataLoadTime fetches load time information from the experiment data table.
Parameters:

	experimentDB: Connection to the experiment database
	vCPUPerInstance: Number of virtual CPUs per instance.
	memPerCore: Memory per core.
	ybVersionIdToUse: yb version id to use w.r.t. given target yb version.

Returns:

	[]ExpDataLoadTime: A slice containing the fetched load time information.
	error: Error if any.
*/
func getExpDataLoadTime(experimentDB *sql.DB, vCPUPerInstance int, memPerCore int,
	tableType string, ybVersionIdToUse int64) ([]ExpDataLoadTime, error) {
	selectQuery := fmt.Sprintf(`
		SELECT csv_size_gb, 
			   migration_time_secs,
			   row_count
		FROM %v 
		WHERE num_cores = ? 
			AND mem_per_core = ?
		    AND yb_version_id = ? 
		ORDER BY csv_size_gb;
	`, tableType)
	rows, err := experimentDB.Query(selectQuery, vCPUPerInstance, memPerCore, ybVersionIdToUse)

	if err != nil {
		return nil, fmt.Errorf("error while fetching load time info with query [%s]: %w", selectQuery, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close result set for query: [%s]", selectQuery)
		}
	}()

	var loadTimes []ExpDataLoadTime
	for rows.Next() {
		var loadTime ExpDataLoadTime
		if err = rows.Scan(&loadTime.csvSizeGB, &loadTime.migrationTimeSecs, &loadTime.rowCount); err != nil {
			return nil, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w",
				selectQuery, err)
		}
		loadTimes = append(loadTimes, loadTime)
	}
	return loadTimes, nil
}

/*
getExpDataIndexImpactOnLoadTime fetches data for impact of indexes on load time from the experiment data table.
Parameters:

	experimentDB: Connection to the experiment database
	vCPUPerInstance: Number of virtual CPUs per instance.
	memPerCore: Memory per core.
	ybVersionIdToUse: yb version id to use w.r.t. given target yb version.

Returns:

	[]ExpDataShardedLoadTimeIndexImpact: A slice containing the fetched load time information based on number of indexes.
	error: Error if any.
*/
func getExpDataIndexImpactOnLoadTime(experimentDB *sql.DB, vCPUPerInstance int,
	memPerCore int, ybVersionIdToUse int64) ([]ExpDataLoadTimeIndexImpact, error) {
	selectQuery := fmt.Sprintf(`
		SELECT number_of_indexes, 
			   multiplication_factor_sharded,
			   multiplication_factor_colocated
		FROM %v 
		WHERE num_cores = ? 
			AND mem_per_core = ? 
		AND yb_version_id = ? 
		ORDER BY number_of_indexes;
	`, LOAD_TIME_INDEX_IMPACT_TABLE)
	rows, err := experimentDB.Query(selectQuery, vCPUPerInstance, memPerCore, ybVersionIdToUse)

	if err != nil {
		return nil, fmt.Errorf("error while fetching index impact info with query [%s]: %w", selectQuery, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close result set for query: [%s]", selectQuery)
		}
	}()

	var loadTimeIndexImpacts []ExpDataLoadTimeIndexImpact
	for rows.Next() {
		var loadTimeIndexImpact ExpDataLoadTimeIndexImpact
		if err = rows.Scan(&loadTimeIndexImpact.numIndexes, &loadTimeIndexImpact.multiplicationFactorSharded,
			&loadTimeIndexImpact.multiplicationFactorColocated); err != nil {
			return nil, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", selectQuery, err)
		}
		loadTimeIndexImpacts = append(loadTimeIndexImpacts, loadTimeIndexImpact)
	}
	return loadTimeIndexImpacts, nil
}

/*
getExpDataNumColumnsImpactOnLoadTime fetches data for impact of number of columns on load time from the experiment
data table.
Parameters:

	experimentDB: Connection to the experiment database
	vCPUPerInstance: Number of virtual CPUs per instance.
	memPerCore: Memory per core.
	ybVersionIdToUse: yb version id to use w.r.t. given target yb version.

Returns:

	[]ExpDataLoadTimeColumnsImpact: A slice containing the fetched load time information based on number of indexes.
	error: Error if any.
*/
func getExpDataNumColumnsImpactOnLoadTime(experimentDB *sql.DB, vCPUPerInstance int,
	memPerCore int, ybVersionIdToUse int64) ([]ExpDataLoadTimeColumnsImpact, error) {
	selectQuery := fmt.Sprintf(`
		SELECT number_of_columns, 
			   multiplication_factor_sharded,
			   multiplication_factor_colocated
		FROM %v 
		WHERE num_cores = ? 
			AND mem_per_core = ?
		AND yb_version_id = ? 
		ORDER BY number_of_columns;
	`, LOAD_TIME_COLUMNS_IMPACT_TABLE)
	rows, err := experimentDB.Query(selectQuery, vCPUPerInstance, memPerCore, ybVersionIdToUse)

	if err != nil {
		return nil, fmt.Errorf("error while fetching columns impact info with query [%s]: %w", selectQuery, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close result set for query: [%s]", selectQuery)
		}
	}()

	var loadTimeColumnsImpacts []ExpDataLoadTimeColumnsImpact
	for rows.Next() {
		var loadTimeColumnsImpact ExpDataLoadTimeColumnsImpact
		if err = rows.Scan(&loadTimeColumnsImpact.numColumns, &loadTimeColumnsImpact.multiplicationFactorSharded,
			&loadTimeColumnsImpact.multiplicationFactorColocated); err != nil {
			return nil, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", selectQuery, err)
		}
		loadTimeColumnsImpacts = append(loadTimeColumnsImpacts, loadTimeColumnsImpact)
	}
	return loadTimeColumnsImpacts, nil
}

/*
getExpDataNumNodesImpactOnLoadTime fetches data for throughput scaling with number of nodes from the experiment
data table.
Parameters:

	experimentDB: Connection to the experiment database
	vCPUPerInstance: Number of virtual CPUs per instance.
	memPerCore: Memory per core.
	ybVersionIdToUse: yb version id to use w.r.t. given target yb version.

Returns:

	[]ExpDataLoadTimeNumNodesImpact: A slice containing the fetched throughput scaling information based on number of nodes.
	error: Error if any.
*/
func getExpDataNumNodesImpactOnLoadTime(experimentDB *sql.DB, vCPUPerInstance int,
	memPerCore int, ybVersionIdToUse int64) ([]ExpDataLoadTimeNumNodesImpact, error) {
	selectQuery := fmt.Sprintf(`
		SELECT num_nodes, 
			   import_throughput_mbps
		FROM %v 
		WHERE num_cores = ? 
			AND mem_per_core = ?
		AND yb_version_id = ? 
		ORDER BY num_nodes;
	`, LOAD_TIME_NUM_NODES_IMPACT)
	rows, err := experimentDB.Query(selectQuery, vCPUPerInstance, memPerCore, ybVersionIdToUse)

	if err != nil {
		return nil, fmt.Errorf("error while fetching num nodes impact info with query [%s]: %w", selectQuery, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close result set for query: [%s]", selectQuery)
		}
	}()

	var loadTimeNumNodesImpacts []ExpDataLoadTimeNumNodesImpact
	for rows.Next() {
		var loadTimeNumNodesImpact ExpDataLoadTimeNumNodesImpact
		if err = rows.Scan(&loadTimeNumNodesImpact.numNodes, &loadTimeNumNodesImpact.importThroughputMbps); err != nil {
			return nil, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", selectQuery, err)
		}
		loadTimeNumNodesImpacts = append(loadTimeNumNodesImpacts, loadTimeNumNodesImpact)
	}
	return loadTimeNumNodesImpacts, nil
}

/*
findImportTimeFromExpDataLoadTime finds the closest record from the experiment data based on row count or size
of the table. Out of objects close in terms of size and rows, prefer object having number of rows.
Parameters:

	loadTimes: A slice containing the fetched load time information.
	objectSize: The size of the table in gigabytes.
	rowsInTable: number of rows in the table.

Returns:

	float64: max load time wrt size or count.
*/
func findImportTimeFromExpDataLoadTime(loadTimes []ExpDataLoadTime, objectSize float64,
	rowsInTable float64) float64 {
	closestInSize := loadTimes[0]
	closestInRows := loadTimes[0]

	minSizeDiff := math.Abs(objectSize - closestInSize.csvSizeGB.Float64)
	minRowsDiff := math.Abs(rowsInTable - closestInRows.rowCount.Float64)

	for _, num := range loadTimes {
		sizeDiff := math.Abs(objectSize - num.csvSizeGB.Float64)
		rowsDiff := math.Abs(rowsInTable - num.rowCount.Float64)
		if sizeDiff < minSizeDiff {
			minSizeDiff = sizeDiff
			closestInSize = num
		}
		if rowsDiff < minRowsDiff {
			minRowsDiff = rowsDiff
			closestInRows = num
		}
	}

	// calculate the time taken for import based on csv size and row count
	importTimeWrtSize := (closestInSize.migrationTimeSecs.Float64 * objectSize) / closestInSize.csvSizeGB.Float64
	importTimeWrtRowCount := (closestInRows.migrationTimeSecs.Float64 * rowsInTable) / closestInRows.rowCount.Float64

	// return the load time which is maximum of the two
	return math.Ceil(math.Max(importTimeWrtSize, importTimeWrtRowCount))
}

/*
getMultiplicationFactorForImportTimeBasedOnIndexes calculates the multiplication factor for import time based on number
of indexes on the table.

Parameters:

	table: Metadata for the database table for which the multiplication factor is to be calculated.
	sourceUniqueIndexesMetadata: A slice containing metadata for the unique indexes in the database.
	indexImpacts: Experimental data containing impact of indexes on load time.
	objectType: COLOCATED or SHARDED

Returns:

	float64: The multiplication factor for import time based on the number of indexes on the table.
*/
func getMultiplicationFactorForImportTimeBasedOnIndexes(table SourceDBMetadata, sourceUniqueIndexesMetadata []SourceDBMetadata,
	indexImpacts []ExpDataLoadTimeIndexImpact, objectType string) float64 {
	var numberOfIndexesOnTable float64 = 0
	var multiplicationFactor float64 = 1

	for _, index := range sourceUniqueIndexesMetadata {
		if index.ParentTableName.Valid && index.ParentTableName.String == (table.SchemaName+"."+table.ObjectName) {
			numberOfIndexesOnTable += 1
		}
	}
	if numberOfIndexesOnTable == 0 {
		// if there are no indexes on table, return 1 immediately
		return 1
	} else {
		closest := indexImpacts[0]
		minDiff := math.Abs(numberOfIndexesOnTable - closest.numIndexes.Float64)

		for _, indexImpactData := range indexImpacts {
			diff := math.Abs(numberOfIndexesOnTable - indexImpactData.numIndexes.Float64)
			if diff < minDiff {
				minDiff = diff
				closest = indexImpactData
			}
		}
		// impact on load time for given table would be relative to the closest record's impact
		if objectType == COLOCATED {
			multiplicationFactor = (closest.multiplicationFactorColocated.Float64 / closest.numIndexes.Float64) * numberOfIndexesOnTable
		} else if objectType == SHARDED {
			multiplicationFactor = (closest.multiplicationFactorSharded.Float64 / closest.numIndexes.Float64) * numberOfIndexesOnTable
		}

		return multiplicationFactor
	}
}

/*
getMultiplicationFactorForImportTimeBasedOnNumColumns calculates the multiplication factor for import time based on
number of columns on the table.

Parameters:

	table: Metadata for the database table for which the multiplication factor is to be calculated.
	columnImpacts: Experimental data containing impact of number of columns on load time.
	objectType: COLOCATED or SHARDED

Returns:

	float64: The multiplication factor for import time based on the number of columns in the table.
*/
func getMultiplicationFactorForImportTimeBasedOnNumColumns(table SourceDBMetadata,
	columnImpacts []ExpDataLoadTimeColumnsImpact, objectType string) float64 {
	numOfColumnsInTable := lo.Ternary(table.ColumnCount.Valid, table.ColumnCount.Int64, 1)

	// Initialize the selectedImpact as nil and minDiff with a high value
	var selectedImpact ExpDataLoadTimeColumnsImpact
	minDiff := int64(math.MaxInt64)
	found := false

	for _, columnsImpactData := range columnImpacts {
		if columnsImpactData.numColumns.Int64 >= numOfColumnsInTable {
			diff := columnsImpactData.numColumns.Int64 - numOfColumnsInTable
			if diff < minDiff {
				minDiff = diff
				selectedImpact = columnsImpactData
				found = true
			}
		}
	}

	// If no suitable impact is found, use the one with the maximum ColumnCount
	if !found {
		for _, columnsImpactData := range columnImpacts {
			if columnsImpactData.numColumns.Int64 > selectedImpact.numColumns.Int64 {
				selectedImpact = columnsImpactData
			}
		}
	}

	var multiplicationFactor float64
	// multiplication factor is different for colocated and sharded tables.
	// multiplication factor would be maximum of the two:
	//	max of (mf of selected entry from experiment data, mf for table wrt selected entry)
	if objectType == COLOCATED {
		multiplicationFactor = math.Max(selectedImpact.multiplicationFactorColocated.Float64,
			(selectedImpact.multiplicationFactorColocated.Float64/float64(selectedImpact.numColumns.Int64))*float64(numOfColumnsInTable))
	} else if objectType == SHARDED {
		multiplicationFactor = math.Max(selectedImpact.multiplicationFactorSharded.Float64,
			(selectedImpact.multiplicationFactorSharded.Float64/float64(selectedImpact.numColumns.Int64))*float64(numOfColumnsInTable))
	}

	return multiplicationFactor
}

/*
getSourceMetadata retrieves metadata for source database tables, indexes, redundant indexes along with the total
size of the source database.
Returns:

	[]SourceDBMetadata: Metadata for source database tables.
	[]SourceDBMetadata: Metadata for source database indexes.
	[]SourceDBMetadata: Metadata for source database indexes after filtering out redundant indexes.
	float64: The total size of the source database in gigabytes.
*/
func getSourceMetadata(sourceDB *sql.DB, sourceDBType string) ([]SourceDBMetadata, []SourceDBMetadata, []SourceDBMetadata, float64, error) {
	// get source database tables, indexes and total size
	sourceTableMetadata, sourceIndexMetadata, totalSourceDBSize, err := getSourceMetadataTableIndexStats(sourceDB, GetTableIndexStatName())
	if err != nil {
		return nil, nil, nil, 0.0, fmt.Errorf("failed read source metadata table %v: %w", GetTableIndexStatName(), err)
	}
	// get source database redundant indexes
	redundantIndexes, err := getSourceMetadataRedundantIndexes(sourceDB, GetTableRedundantIndexesName(), sourceDBType)
	var uniqueSourceIndexMetadata []SourceDBMetadata
	if err != nil {
		log.Warnf("failed to read redundant indexes from %v: %v, continuing without filtering redundant indexes", GetTableRedundantIndexesName(), err)
		// If we can't get redundant indexes, just use all indexes without filtering
		uniqueSourceIndexMetadata = sourceIndexMetadata
	} else {
		// filter out all redudant indexes from sourceIndexMetadata
		uniqueSourceIndexMetadata = filterRedundantIndexes(sourceIndexMetadata, redundantIndexes, sourceDBType)
	}

	if err := sourceDB.Close(); err != nil {
		log.Warnf("failed to close connection to sourceDB metadata")
	}

	return sourceTableMetadata, sourceIndexMetadata, uniqueSourceIndexMetadata, totalSourceDBSize, nil
}

func getSourceMetadataTableIndexStats(sourceDB *sql.DB, sourceTableName string) ([]SourceDBMetadata, []SourceDBMetadata, float64, error) {
	// Construct the WHERE clause dynamically using LIKE
	var likeConditions []string
	for _, pattern := range SourceMetadataObjectTypesToUse {
		likeConditions = append(likeConditions, fmt.Sprintf("object_type LIKE '%s'", pattern))
	}
	// Join the LIKE conditions with OR
	whereClause := strings.Join(likeConditions, " OR ")

	query := fmt.Sprintf(`
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
	`, sourceTableName, whereClause)
	rows, err := sourceDB.Query(query)
	if err != nil {
		return nil, nil, 0.0, fmt.Errorf("failed to query source metadata table: %v with query [%s]: %w", sourceTableName, query, err)
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
			&metadata.IsIndex, &metadata.ParentTableName, &metadata.Size, &metadata.ColumnCount); err != nil {
			return nil, nil, 0.0, fmt.Errorf("failed to read from result set of query source metadata [%s]: %w", query, err)
		}
		// convert bytes to GB
		metadata.Size.Float64 = utils.BytesToGB(lo.Ternary(metadata.Size.Valid, metadata.Size.Float64, 0))
		if metadata.IsIndex {
			sourceIndexMetadata = append(sourceIndexMetadata, metadata)
		} else {
			sourceTableMetadata = append(sourceTableMetadata, metadata)
		}
		totalSourceDBSize += metadata.Size.Float64
	}
	if err := rows.Err(); err != nil {
		return nil, nil, 0.0, fmt.Errorf("failed to query source metadata with query [%s]: %w", query, err)
	}

	return sourceTableMetadata, sourceIndexMetadata, totalSourceDBSize, nil
}

/*
filterRedundantIndexes filters out redundant indexes from the sourceIndexMetadata slice.
It removes indexes where the combination of SchemaName.ParentTableName.ObjectName matches
redundant_schema_name.redundant_table_name.redundant_index_name from the redundant_indexes table.

Parameters:

	sourceIndexMetadata: Slice of source index metadata to be filtered
	redundantIndexes: Slice of redundant index information from the database

Returns:

	[]SourceDBMetadata: Filtered slice with redundant indexes removed
*/
func filterRedundantIndexes(sourceIndexMetadata []SourceDBMetadata, redundantIndexes []utils.RedundantIndexesInfo, sourceDBType string) []SourceDBMetadata {
	if len(redundantIndexes) == 0 {
		return sourceIndexMetadata
	}

	// Create a map for efficient lookup of redundant indexes
	redundantMap := make(map[string]bool)
	for _, ri := range redundantIndexes {
		// Use the helper method to get fully qualified index name
		key := ri.GetRedundantIndexCatalogObjectName()
		redundantMap[key] = true
	}

	// Filter out redundant indexes
	var filteredIndexes []SourceDBMetadata
	for _, indexMetadata := range sourceIndexMetadata {
		// Extract table name from parent table name (format: "schema.table")
		var tableName string
		if indexMetadata.ParentTableName.Valid {
			parts := strings.Split(indexMetadata.ParentTableName.String, ".")
			if len(parts) == 2 {
				tableName = parts[1] // Extract table name part
			} else {
				tableName = indexMetadata.ParentTableName.String // Fallback to full string
			}
		} else {
			// If parent table name is not valid, include the index (can't match against redundant list)
			filteredIndexes = append(filteredIndexes, indexMetadata)
			continue
		}

		// Create a temporary RedundantIndexesInfo to use the same helper method for consistent formatting
		indexSqlName := sqlname.NewObjectNameQualifiedWithTableName(sourceDBType, "",
			indexMetadata.ObjectName, indexMetadata.SchemaName, tableName)
		key := indexSqlName.Qualified.Unquoted

		// Only include index if it's not in the redundant list
		if !redundantMap[key] {
			filteredIndexes = append(filteredIndexes, indexMetadata)
		}
	}

	return filteredIndexes
}

/*
getSourceMetadataRedundantIndexes fetches redundant indexes from the redundant_indexes table using the provided database connection.
It returns a slice of RedundantIndexesInfo structs containing schema, table, and index names.

Parameters:

	sourceDB: Database connection to query the redundant_indexes table
	sourceTableName: Name of the table containing redundant index information

Returns:

	[]utils.RedundantIndexesInfo: Slice of redundant index information from the database
	error: Error if any issue occurs during the query
*/
func getSourceMetadataRedundantIndexes(sourceDB *sql.DB, sourceTableName string, sourceDBType string) ([]utils.RedundantIndexesInfo, error) {
	log.Infof("fetching redundant indexes from %q table", sourceTableName)
	query := fmt.Sprintf(`SELECT redundant_schema_name, redundant_table_name, redundant_index_name FROM %s`, sourceTableName)
	rows, err := sourceDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying redundant indexes-%s: %w", query, err)
	}
	defer rows.Close()

	redundantIndexes := make([]utils.RedundantIndexesInfo, 0)
	for rows.Next() {
		var ri utils.RedundantIndexesInfo
		if err := rows.Scan(&ri.RedundantSchemaName, &ri.RedundantTableName, &ri.RedundantIndexName); err != nil {
			return nil, fmt.Errorf("error scanning redundant index row: %w", err)
		}
		// Set the DBType to enable proper object name generation
		ri.DBType = sourceDBType
		redundantIndexes = append(redundantIndexes, ri)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading redundant index rows: %w", err)
	}

	return redundantIndexes, nil
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
			indexesSizeSum += lo.Ternary(index.Size.Valid, index.Size.Float64, 0)
			cumulativeSelectOpsPerSecIdx += lo.Ternary(index.ReadsPerSec.Valid, index.ReadsPerSec.Int64, 0)
			cumulativeInsertOpsPerSecIdx += lo.Ternary(index.ReadsPerSec.Valid, index.ReadsPerSec.Int64, 0)
		}
	}

	return indexesOfTable, indexesSizeSum, cumulativeSelectOpsPerSecIdx, cumulativeInsertOpsPerSecIdx
}

/*
getReasoning generates a string describing the reasoning behind a recommendation for an instance type.
It considers the characteristics of colocated and sharded objects.
Parameters:
  - recommendation: An IntermediateRecommendation struct containing information about the recommended instance type.
  - shardedObjects: A slice of SourceDBMetadata structs representing objects that need to be sharded.
  - colocatedObjects: A slice of SourceDBMetadata structs representing objects that can be colocated.

Returns:
  - A string describing the reasoning behind the recommendation.
*/
func getReasoning(recommendation IntermediateRecommendation, shardedObjects []SourceDBMetadata,
	cumulativeIndexCountSharded int, colocatedObjects []SourceDBMetadata, cumulativeIndexCountColocated int) string {
	// Calculate size and throughput of colocated objects
	colocatedObjectsSize, colocatedReads, colocatedWrites, sizeUnitColocated := getObjectsSize(colocatedObjects)
	reasoning := fmt.Sprintf("Recommended instance type with %v vCPU and %v GiB memory could fit ",
		recommendation.VCPUsPerInstance, recommendation.VCPUsPerInstance*recommendation.MemoryPerCore)

	// Add information about colocated objects if they exist
	if len(colocatedObjects) > 0 {
		reasoning += fmt.Sprintf("%v objects (%v tables/materialized views and %v explicit/implicit indexes) with %0.2f %v size "+
			"and throughput requirement of %v reads/sec and %v writes/sec as colocated.", len(colocatedObjects),
			len(colocatedObjects)-cumulativeIndexCountColocated, cumulativeIndexCountColocated, colocatedObjectsSize,
			sizeUnitColocated, colocatedReads, colocatedWrites)
	}
	// Add information about sharded objects if they exist
	if len(shardedObjects) > 0 {
		// Calculate size and throughput of sharded objects
		shardedObjectsSize, shardedReads, shardedWrites, sizeUnitSharded := getObjectsSize(shardedObjects)
		// Construct reasoning for sharded objects
		shardedReasoning := fmt.Sprintf("%v objects (%v tables/materialized views and %v explicit/implicit indexes) with %0.2f %v "+
			"size and throughput requirement of %v reads/sec and %v writes/sec ", len(shardedObjects),
			len(shardedObjects)-cumulativeIndexCountSharded, cumulativeIndexCountSharded, shardedObjectsSize,
			sizeUnitSharded, shardedReads, shardedWrites)
		// If colocated objects exist, add sharded objects information as rest of the objects need to be migrated as sharded
		if len(colocatedObjects) > 0 {
			reasoning += " Rest " + shardedReasoning + "need to be migrated as range partitioned tables."
		} else {
			reasoning += shardedReasoning + "as sharded."
		}

	}
	reasoning += " Non leaf partition tables/indexes and unsupported tables/indexes were not considered."
	return reasoning
}

/*
getObjectsSize calculates the total size and throughput requirements of a slice of SourceDBMetadata objects.
Parameters:
  - objects: A slice of SourceDBMetadata structs representing the database objects.

Returns:
  - The total size of the objects (in float64 representing GB).
  - The total number of select operations per second across all objects (in int64).
  - The total number of insert operations per second across all objects (in int64).
  - The size unit for the total size
*/
func getObjectsSize(objects []SourceDBMetadata) (float64, int64, int64, string) {
	var objectsSize float64 = 0
	var objectSelectOps int64 = 0
	var objectInsertOps int64 = 0
	var sizeUnit = SIZE_UNIT_GB

	for _, object := range objects {
		// Accumulate size and throughput values
		objectsSize += lo.Ternary(object.Size.Valid, object.Size.Float64, 0)
		objectSelectOps += lo.Ternary(object.ReadsPerSec.Valid, object.ReadsPerSec.Int64, 0)
		objectInsertOps += lo.Ternary(object.WritesPerSec.Valid, object.WritesPerSec.Int64, 0)
	}
	// if object size is less than 1 GB, convert it to MB
	if objectsSize < 1 {
		sizeUnit = SIZE_UNIT_MB
		objectsSize = objectsSize * 1024
	}

	// Return the accumulated size and throughput values
	return objectsSize, objectSelectOps, objectInsertOps, sizeUnit
}

/*
getListOfIndexesAlongWithObjects generates a list of indexes along with their corresponding tables from the given tableList.
Parameters:
  - tableList: A slice of SourceDBMetadata structs representing tables.
  - sourceIndexMetadata: A slice of SourceDBMetadata structs representing index metadata.

Returns:
  - A slice of SourceDBMetadata structs containing both indexes and tables.
  - total indexes of the tables
*/
func getListOfIndexesAlongWithObjects(tableList []SourceDBMetadata,
	sourceIndexMetadata []SourceDBMetadata) ([]SourceDBMetadata, int) {
	var indexesAndObject []SourceDBMetadata
	var cumulativeIndexCount int = 0

	for _, table := range tableList {
		// Check and fetch indexes for the current table
		indexes, _, _, _ := checkAndFetchIndexes(table, sourceIndexMetadata)
		indexesAndObject = append(indexesAndObject, indexes...)
		indexesAndObject = append(indexesAndObject, table)
		cumulativeIndexCount += len(indexes)
	}
	// Return the slice containing both indexes and tables
	return indexesAndObject, cumulativeIndexCount
}

func createConnectionToExperimentData(assessmentDir string) (*sql.DB, error) {
	filePath, err := getExperimentFile(assessmentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get experiment file: %w", err)
	}
	DbConnection, err := utils.ConnectToSqliteDatabase(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to experiment data database: %w", err)
	}
	return DbConnection, nil
}

func getExperimentFile(assessmentDir string) (string, error) {
	fetchedFromRemote := false
	if PREFER_REMOTE_EXPERIMENT_DB && checkInternetAccess() {
		existsOnRemote, err := checkAndDownloadFileExistsOnRemoteRepo(assessmentDir)
		if err != nil {
			return "", err
		}
		if existsOnRemote {
			fetchedFromRemote = true
		}
	}
	if !fetchedFromRemote {
		err := os.WriteFile(getExperimentDBPath(assessmentDir), experimentData, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write experiment data file: %w", err)
		}
	}

	return getExperimentDBPath(assessmentDir), nil
}

func checkAndDownloadFileExistsOnRemoteRepo(assessmentDir string) (bool, error) {
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

	downloadPath := getExperimentDBPath(assessmentDir)
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
fetchObjectNames extracts object names from the given database objects and returns them as a slice of strings.
Parameters:
  - dbObjects: A slice of SourceDBMetadata structs representing database objects.

Returns:
  - A slice of strings containing the names of the database objects in the format
*/
func fetchObjectNames(dbObjects []SourceDBMetadata) []string {
	var objectNames []string
	for _, dbObject := range dbObjects {
		objectNames = append(objectNames, dbObject.SchemaName+"."+dbObject.ObjectName)
	}
	return objectNames
}

/*
loadYbVersionsWithExperimentData fetches all versions of yugabyte for which experiment data is available.
It retrieves various limits such as maximum colocated database size, number of cores, memory per core, etc.

Returns:
  - A slice of ExperimentDataAvailableYbVersion with all supported yb versions with experiment data.
  - A default yugabyte version id to use in case no supported yb version data is available.
  - An error if there was any issue during the data retrieval process.
*/
func loadYbVersionsWithExperimentData(experimentDB *sql.DB) ([]ExperimentDataAvailableYbVersion, int64, error) {
	var experimentDataAvailableYbVersions []ExperimentDataAvailableYbVersion
	var defaultVersionId int64 = 0
	query := `SELECT yb_version_id, yb_version, is_default FROM experiment_data_yb_versions ORDER BY yb_version_id`

	rows, err := experimentDB.Query(query)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", query, err)
	}

	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Warnf("failed to close the result set for query [%v]", query)
		}
	}()

	for rows.Next() {
		var id int64
		var v string
		var isDefault bool

		if err := rows.Scan(&id, &v, &isDefault); err != nil {
			return nil, 0, fmt.Errorf("cannot fetch data from experiment data table with query [%s]: %w", query, err)
		}
		convertedVersion, err := ybversion.NewYBVersion(v)
		if isDefault {
			defaultVersionId = id
		}
		if err != nil {
			// if experiment data is not from supported yb version series, then ignore that release
			// fmt.Printf("version not converted and the error is [%v]\n", err)
			continue
		}
		r1 := ExperimentDataAvailableYbVersion{
			versionId:        id,
			expDataYbVersion: convertedVersion,
			expDataIsDefault: isDefault,
		}
		experimentDataAvailableYbVersions = append(experimentDataAvailableYbVersions, r1)
	}

	// return fetched available versions
	return experimentDataAvailableYbVersions, defaultVersionId, nil
}

/*
findClosestVersion Helper function to find the closest yb version

Parameters:
  - targetDbVersion: given target yugabyte database version
  - availableVersions: slice of available yugabyte versions which has experiment data available.
  - defaultYbVersionId: default experiment data to use in case finding closest version is unsuccessful

Returns:
  - returns the version id(index from experiment table) of the closest version to use for given target yb version.
*/
func findClosestVersion(targetDbVersion *ybversion.YBVersion, availableVersions []ExperimentDataAvailableYbVersion,
	defaultYbVersionId int64) int64 {
	var closest *ExperimentDataAvailableYbVersion

	for i := 0; i < len(availableVersions); i++ {
		available := availableVersions[i].expDataYbVersion

		// Check if available <= target
		if available.Version.Compare(targetDbVersion.Version) > 0 {
			// Skip versions greater than the target
			continue
		}
		if closest == nil {
			closest = &availableVersions[i]
		} else if available.Version.Compare(closest.expDataYbVersion.Version) > 0 {
			// Found a closer (but still <= target) version.
			closest = &availableVersions[i]
		}
	}
	if closest == nil {
		return defaultYbVersionId
	} else {
		return closest.versionId
	}
}

/*
findNumNodesThroughputScalingImportTimeDivisor calculates the relative import time divisor based on throughput scaling with number of nodes.

This function computes a scaling factor to adjust import time estimates based on the number of nodes in the cluster.
It uses experimental data that shows how import throughput scales with cluster size to determine the appropriate
time adjustment factor.

Algorithm:
1. Find the 3-node baseline entry from experimental data (used as reference point)
2. Find the closest experimental data point to the target number of nodes
3. Calculate per-node throughput from the closest data point
4. Estimate total throughput for target nodes: targetNodes * perNodeThroughput
5. Return ratio: estimatedThroughput / baselineThroughput

This ratio represents how much faster (>1.0) the import would be compared to the 3-node baseline.

Parameters:
  - numNodesImpactData: slice of experimental data for throughput scaling with different number of nodes
  - targetNumNodes: the target number of nodes to calculate the scaling ratio for

Returns:
  - float64: the relative import time divisor based on throughput scaling with number of nodes
*/
func findNumNodesThroughputScalingImportTimeDivisor(numNodesImpactData []ExpDataLoadTimeNumNodesImpact, targetNumNodes float64) float64 {
	// Check if the slice is empty
	if len(numNodesImpactData) == 0 {
		return 1.0 // Return 1.0 if no data available
	}

	// select the 3-node baseline entry
	var baselineData ExpDataLoadTimeNumNodesImpact
	for _, data := range numNodesImpactData {
		if data.numNodes.Valid && data.numNodes.Int64 == 3 && data.importThroughputMbps.Valid {
			baselineData = data
			break
		}
	}

	if !baselineData.importThroughputMbps.Valid || !baselineData.numNodes.Valid {
		return 1.0 // Return 1.0 if no baseline data available
	}

	// Find the closest entry to targetNumNodes
	closest := numNodesImpactData[0]
	minDiff := math.MaxFloat64

	for _, data := range numNodesImpactData {
		if data.numNodes.Valid && data.importThroughputMbps.Valid {
			diff := math.Abs(targetNumNodes - float64(data.numNodes.Int64))
			if diff < minDiff {
				minDiff = diff
				closest = data
			}
		}
	}

	// Calculate estimated throughput based on per-node throughput from closest entry
	perNodeThroughputForClosest := closest.importThroughputMbps.Float64 / float64(closest.numNodes.Int64)
	estimatedThroughput := targetNumNodes * perNodeThroughputForClosest

	// Return the scaling ratio relative to 3-node baseline
	return estimatedThroughput / baselineData.importThroughputMbps.Float64
}
