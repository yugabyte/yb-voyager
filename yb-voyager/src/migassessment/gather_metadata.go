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
	"fmt"
	"path/filepath"
	"strings"

	goerrors "github.com/go-errors/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ux"
)

type GatherAssessmentMetadataStageConfig struct {
	Source                    *srcdb.Source
	AssessmentMetadataDir     string
	AssessmentMetadataDirFlag string
	ExportDir                 string
	SchemaDir                 string
	ValidatedReplicaEndpoints []srcdb.ReplicaEndpoint
	PgssEnabledForAssessment bool
	IOPSInterval             int64
	Tracker                  *ux.ProgressTracker
}

func RunGatherAssessmentMetadataStage(config GatherAssessmentMetadataStageConfig) (*AssessmentDB, error) {
	var assessmentDB *AssessmentDB
	err := startGatherMetadataStage(config.Source.DBType, len(config.ValidatedReplicaEndpoints) > 0, config.Tracker, func() error {
		var err error
		assessmentDB, err = InitAndOpenAssessmentDB()
		if err != nil {
			return err
		}

		err = gatherAssessmentMetadata(config)
		if err != nil {
			return fmt.Errorf("failed to gather assessment metadata: %w", err)
		}

		err = parseExportedSchemaFileForAssessmentIfRequired(config)
		if err != nil {
			return fmt.Errorf("failed to parse exported schema file for assessment: %w", err)
		}

		err = PopulateMetadataCSVIntoAssessmentDB(assessmentDB, config.AssessmentMetadataDir)
		if err != nil {
			return fmt.Errorf("failed to populate metadata CSV into SQLite DB: %w", err)
		}
		return nil
	})
	if err != nil {
		if assessmentDB != nil {
			closeErr := assessmentDB.Close()
			if closeErr != nil {
				log.Warnf("failed to close assessment DB after gather metadata stage failure: %v", closeErr)
			}
		}
		return nil, err
	}
	return assessmentDB, nil
}

func startGatherMetadataStage(sourceDBType string, hasReplicas bool, tracker *ux.ProgressTracker, stageFunc ux.ProgressStageFunc) error {
	gatherStepCount, prepareStaticStage := gatherMetadataStageConfig(sourceDBType, hasReplicas)
	if prepareStaticStage {
		// Parallel replica path uses its own internal progress display,
		// and Oracle uses a static summary banner without per-step spinners.
		return tracker.PrepareStage("Gathering metadata", gatherStepCount, stageFunc)
	}
	return tracker.StartStage("Gathering metadata", gatherStepCount, stageFunc)
}

func gatherMetadataStageConfig(sourceDBType string, hasReplicas bool) (int, bool) {
	switch sourceDBType {
	case constants.ORACLE:
		return 0, true
	case constants.POSTGRESQL:
		return CountPGGatherSteps(), hasReplicas
	default:
		return 0, false
	}
}

// gatherAssessmentMetadata collects metadata from the source database.
func gatherAssessmentMetadata(config GatherAssessmentMetadataStageConfig) error {
	if config.AssessmentMetadataDirFlag != "" {
		return nil // assessment metadata files are provided by the user inside assessmentMetadataDir
	}

	log.Infof("gathering metadata and stats from '%s' source database...", config.Source.DBType)

	switch config.Source.DBType {
	case constants.POSTGRESQL:
		err := GatherAssessmentMetadataFromPG(
			config.Source,
			config.ValidatedReplicaEndpoints,
			config.AssessmentMetadataDir,
			config.PgssEnabledForAssessment,
			config.IOPSInterval,
			config.Tracker,
		)
		if err != nil {
			return fmt.Errorf("error gathering metadata and stats from source PG database: %w", err)
		}
	case constants.ORACLE:
		err := GatherAssessmentMetadataFromOracle(config.Source, config.AssessmentMetadataDir)
		if err != nil {
			return fmt.Errorf("error gathering metadata and stats from source Oracle database: %w", err)
		}
	default:
		return goerrors.Errorf("source DB Type %s is not yet supported for metadata and stats gathering", config.Source.DBType)
	}
	log.Infof("gathered assessment metadata files at '%s'", config.AssessmentMetadataDir)
	return nil
}

/*
It is due to the differences in how tools like ora2pg, and pg_dump exports the schema
pg_dump - export schema in single .sql file which is later on segregated by voyager in respective .sql file
ora2pg - export schema in given .sql file, and we have to call it for each object type to export schema
*/
func parseExportedSchemaFileForAssessmentIfRequired(config GatherAssessmentMetadataStageConfig) (err error) {
	if config.Source.DBType == constants.ORACLE {
		return nil // already parsed into schema files while exporting
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err = goerrors.Errorf("export schema failed: %v", recovered)
		}
	}()

	log.Infof("set 'schemaDir' as: %s", config.SchemaDir)
	config.Source.ApplyExportSchemaObjectListFilter()
	config.Source.DB().ExportSchema(config.ExportDir, config.SchemaDir)
	if !utils.FileOrFolderExistsWithGlobPattern(filepath.Join(config.SchemaDir, "*", "*.sql")) {
		return goerrors.Errorf("no parsed schema files found in %s", config.SchemaDir)
	}
	return nil
}

func PopulateMetadataCSVIntoAssessmentDB(assessmentDB *AssessmentDB, assessmentMetadataDir string) error {
	// Collect CSV files from metadata directory
	// Two supported structures:
	//   1. Multi-node (PostgreSQL): assessmentMetadataDir/node-*/*.csv
	//   2. Single-node (Oracle, MySQL, etc.): assessmentMetadataDir/*.csv
	var metadataFilesPath []string

	// Check for multi-node structure first (node-* directories)
	nodeDirs, err := filepath.Glob(filepath.Join(assessmentMetadataDir, "node-*"))
	if err != nil {
		return fmt.Errorf("error looking for node data directories in %s: %w", assessmentMetadataDir, err)
	}

	if len(nodeDirs) > 0 {
		// Multi-node structure: Collect CSV files from each node directory
		for _, nodeDir := range nodeDirs {
			nodeFiles, err := filepath.Glob(filepath.Join(nodeDir, "*.csv"))
			if err != nil {
				return fmt.Errorf("error looking for csv files in directory %s: %w", nodeDir, err)
			}
			metadataFilesPath = append(metadataFilesPath, nodeFiles...)
		}
		log.Infof("Found %d CSV files across %d node(s) for population into assessment DB", len(metadataFilesPath), len(nodeDirs))
	} else {
		// Single-node structure: Collect CSV files directly from metadata directory
		metadataFilesPath, err = filepath.Glob(filepath.Join(assessmentMetadataDir, "*.csv"))
		if err != nil {
			return fmt.Errorf("error looking for csv files in directory %s: %w", assessmentMetadataDir, err)
		}
		log.Infof("Found %d CSV files in metadata directory for population into assessment DB", len(metadataFilesPath))
	}

	for _, metadataFilePath := range metadataFilesPath {
		baseFileName := filepath.Base(metadataFilePath)
		metric := strings.TrimSuffix(baseFileName, filepath.Ext(baseFileName))
		tableName := strings.Replace(metric, "-", "_", -1)
		// collecting both initial and final measurement in the same table
		tableName = lo.Ternary(strings.Contains(tableName, TABLE_INDEX_IOPS),
			TABLE_INDEX_IOPS, tableName)

		// check if the table exist in the assessment db or not
		// possible scenario: if gather scripts are run manually, not via voyager
		err := assessmentDB.CheckIfTableExists(tableName)
		if err != nil {
			return fmt.Errorf("error checking if table %s exists: %w", tableName, err)
		}

		log.Infof("populating metadata from file %s into table %s", metadataFilePath, tableName)
		err = assessmentDB.LoadCSVFileIntoTable(metadataFilePath, tableName)
		if err != nil {
			return fmt.Errorf("error loading CSV file %s: %w", metadataFilePath, err)
		}

		log.Infof("populated metadata from file %s into table %s", metadataFilePath, tableName)
	}

	err = assessmentDB.PopulateMigrationAssessmentStats()
	if err != nil {
		return fmt.Errorf("failed to populate migration assessment stats: %w", err)
	}
	return nil
}
