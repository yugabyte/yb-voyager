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

package cmd

import (
	"bufio"
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var (
	assessmentMetadataDir     string
	assessmentMetadataDirFlag string
	assessmentReport          AssessmentReport
	assessmentDB              *migassessment.AssessmentDB
	intervalForCapturingIOPS  int64
)
var sourceConnectionFlags = []string{
	"source-db-host",
	"source-db-password",
	"source-db-name",
	"source-db-port",
	"source-db-schema",
	"source-db-user",
	"source-ssl-cert",
	"source-ssl-crl",
	"source-ssl-key",
	"source-ssl-mode",
	"source-ssl-root-cert",
}

type UnsupportedFeature struct {
	FeatureName string   `json:"FeatureName"`
	ObjectNames []string `json:"ObjectNames"`
}

var assessMigrationCmd = &cobra.Command{
	Use:   "assess-migration",
	Short: "Assess the migration from source (PostgreSQL) database to YugabyteDB.",
	Long:  `Assess the migration from source (PostgreSQL) database to YugabyteDB.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		validateSourceDBTypeForAssessMigration()
		setExportFlagsDefaults()
		validateSourceSchema()
		validatePortRange()
		validateSSLMode()
		if cmd.Flags().Changed("assessment-metadata-dir") {
			validateAssessmentMetadataDirFlag()
			for _, f := range sourceConnectionFlags {
				if cmd.Flags().Changed(f) {
					utils.ErrExit("Cannot pass `--source-*` connection related flags when `--assessment-metadata-dir` is provided.\nPlease re-run the command without these flags")
				}
			}
		} else {
			cmd.MarkFlagRequired("source-db-user")
			cmd.MarkFlagRequired("source-db-name")
			//Update this later as per db-types TODO
			cmd.MarkFlagRequired("source-db-schema")
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		err := assessMigration()
		if err != nil {
			packAndSendAssessMigrationPayload(ERROR, err.Error())
			utils.ErrExit("failed to assess migration: %v", err)
		}
		packAndSendAssessMigrationPayload(COMPLETE, "")
	},
}

func packAndSendAssessMigrationPayload(status string, errMsg string) {
	if !callhome.SendDiagnostics {
		return
	}
	payload := createCallhomePayload()
	payload.MigrationPhase = ASSESS_MIGRATION_PHASE

	featuresBytes, err := json.Marshal(assessmentReport.UnsupportedFeatures)
	if err != nil {
		log.Errorf("callhome: error in parsing unsupported features from assessment report: %s", err)
	}
	datatypesBytes, err := json.Marshal(assessmentReport.UnsupportedDataTypes)
	if err != nil {
		log.Errorf("callhome: error in parsing unsupported features from assessment report: %s", err)
	}
	var tableSizingStats, indexSizingStats []callhome.ObjectSizingStats
	if assessmentReport.TableIndexStats != nil {
		for _, stat := range *assessmentReport.TableIndexStats {
			newStat := callhome.ObjectSizingStats{
				SchemaName:      stat.SchemaName,
				ObjectName:      stat.ObjectName,
				ReadsPerSecond:  *stat.ReadsPerSecond,
				WritesPerSecond: *stat.WritesPerSecond,
				SizeInBytes:     *stat.SizeInBytes,
			}
			if stat.IsIndex {
				indexSizingStats = append(indexSizingStats, newStat)
			} else {
				tableSizingStats = append(tableSizingStats, newStat)
			}
		}
	}
	tableBytes, err := json.Marshal(tableSizingStats)
	if err != nil {
		log.Errorf("callhome: error in parsing the table sizing stats: %v", err)
	}
	indexBytes, err := json.Marshal(indexSizingStats)
	if err != nil {
		log.Errorf("callhome: error in parsing the index sizing stats: %v", err)
	}
	schemaSummaryCopy := utils.SchemaSummary{
		SchemaNames: assessmentReport.SchemaSummary.SchemaNames,
		Notes:       assessmentReport.SchemaSummary.Notes,
	}
	for _, dbObject := range assessmentReport.SchemaSummary.DBObjects {
		//Creating a copy and not adding objectNames here, as those will anyway be available
		//at analyze-schema step so no need to have non-relevant information to not clutter the payload
		//only counts are useful at this point
		dbObjectCopy := utils.DBObject{
			ObjectType:   dbObject.ObjectType,
			TotalCount:   dbObject.TotalCount,
			InvalidCount: dbObject.InvalidCount,
			Details:      dbObject.Details,
		}
		schemaSummaryCopy.DBObjects = append(schemaSummaryCopy.DBObjects, dbObjectCopy)
	}
	schemaSummaryCopyBytes, err := json.Marshal(schemaSummaryCopy)
	if err != nil {
		log.Errorf("callhome: error parsing schema summary: %v", err)
	}
	assessPayload := callhome.AssessMigrationPhasePayload{
		UnsupportedFeatures:  string(featuresBytes),
		UnsupportedDataTypes: string(datatypesBytes),
		TableSizingStats:     string(tableBytes),
		IndexSizingStats:     string(indexBytes),
		SchemaSummary:        string(schemaSummaryCopyBytes),
	}
	if status == ERROR {
		assessPayload.Error = errMsg
	}
	if assessmentMetadataDirFlag == "" {
		sourceDBDetails := callhome.SourceDBDetails{
			Host:      source.Host,
			DBType:    source.DBType,
			DBVersion: source.DBVersion,
			DBSize:    source.DBSize,
		}
		sourceDBBytes, err := json.Marshal(sourceDBDetails)
		if err != nil {
			log.Errorf("callhome: error in parsing sourcedb details: %v", err)
		}
		payload.SourceDBDetails = string(sourceDBBytes)
	}
	assessPayloadBytes, err := json.Marshal(assessPayload)
	if err != nil {
		log.Errorf("callhome: error while parsing 'database_objects' json: %v", err)
	}
	payload.PhasePayload = string(assessPayloadBytes)
	payload.Status = status

	callhome.SendPayload(&payload)
	callHomePayloadSent = true
}

func registerSourceDBConnFlagsForAM(cmd *cobra.Command) {
	cmd.Flags().StringVar(&source.DBType, "source-db-type", "",
		"source database type: (postgresql)\n")

	cmd.MarkFlagRequired("source-db-type")

	cmd.Flags().StringVar(&source.Host, "source-db-host", "localhost",
		"source database server host")

	cmd.Flags().IntVar(&source.Port, "source-db-port", 0,
		"source database server port number. Default: PostgreSQL(5432)")

	cmd.Flags().StringVar(&source.User, "source-db-user", "",
		"connect to source database as the specified user")

	// TODO: All sensitive parameters can be taken from the environment variable
	cmd.Flags().StringVar(&source.Password, "source-db-password", "",
		"source password to connect as the specified user. Alternatively, you can also specify the password by setting the environment variable SOURCE_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")

	cmd.Flags().StringVar(&source.DBName, "source-db-name", "",
		"source database name to be migrated to YugabyteDB")

	cmd.Flags().StringVar(&source.Schema, "source-db-schema", "",
		"source schema name(s) to export\n"+
			`Note: in case of multiple schemas, use a comma separated list of schemas: "schema1,schema2,schema3"`)

	// TODO SSL related more args will come. Explore them later.
	cmd.Flags().StringVar(&source.SSLCertPath, "source-ssl-cert", "",
		"Path of the file containing source SSL Certificate")

	cmd.Flags().StringVar(&source.SSLMode, "source-ssl-mode", "prefer",
		"specify the source SSL mode out of: (disable, allow, prefer, require, verify-ca, verify-full)")

	cmd.Flags().StringVar(&source.SSLKey, "source-ssl-key", "",
		"Path of the file containing source SSL Key")

	cmd.Flags().StringVar(&source.SSLRootCert, "source-ssl-root-cert", "",
		"Path of the file containing source SSL Root Certificate")

	cmd.Flags().StringVar(&source.SSLCRL, "source-ssl-crl", "",
		"Path of the file containing source SSL Root Certificate Revocation List (CRL)")
}

func init() {
	rootCmd.AddCommand(assessMigrationCmd)
	registerCommonGlobalFlags(assessMigrationCmd)
	registerSourceDBConnFlagsForAM(assessMigrationCmd)

	BoolVar(assessMigrationCmd.Flags(), &startClean, "start-clean", false,
		"cleans up the project directory for schema or data files depending on the export command (default false)")

	// optional flag to take metadata and stats directory path in case it is not in exportDir
	assessMigrationCmd.Flags().StringVar(&assessmentMetadataDirFlag, "assessment-metadata-dir", "",
		"Directory path where assessment metadata like source DB metadata and statistics are stored. Optional flag, if not provided, "+
			"it will be assumed to be present at default path inside the export directory.")

	assessMigrationCmd.Flags().Int64Var(&intervalForCapturingIOPS, "iops-capture-interval", 120,
		"Interval (in seconds) at which voyager will gather IOPS metadata from source database for the given schema(s). (only valid for PostgreSQL)")

}

func assessMigration() (err error) {
	assessmentMetadataDir = lo.Ternary(assessmentMetadataDirFlag != "", assessmentMetadataDirFlag,
		filepath.Join(exportDir, "assessment", "metadata"))
	// setting schemaDir to use later on - gather assessment metadata, segregating into schema files per object etc..
	schemaDir = filepath.Join(assessmentMetadataDir, "schema")

	checkStartCleanForAssessMigration(assessmentMetadataDirFlag != "")
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)

	err = retrieveMigrationUUID()
	if err != nil {
		return fmt.Errorf("failed to get migration UUID: %w", err)
	}

	startEvent := createMigrationAssessmentStartedEvent()
	controlPlane.MigrationAssessmentStarted(startEvent)

	assessmentDir := filepath.Join(exportDir, "assessment")
	migassessment.AssessmentDir = assessmentDir
	migassessment.SourceDBType = source.DBType
	initAssessmentDB() // Note: migassessment.AssessmentDir needs to be set beforehand

	err = gatherAssessmentMetadata()
	if err != nil {
		return fmt.Errorf("failed to gather assessment metadata: %w", err)
	}

	parseExportedSchemaFileForAssessmentIfRequired()

	err = populateMetadataCSVIntoAssessmentDB()
	if err != nil {
		return fmt.Errorf("failed to populate metadata CSV into SQLite DB: %w", err)
	}

	err = runAssessment()
	if err != nil {
		utils.PrintAndLog("failed to run assessment: %v", err)
	}

	err = generateAssessmentReport()
	if err != nil {
		return fmt.Errorf("failed to generate assessment report: %w", err)
	}

	utils.PrintAndLog("Migration assessment completed successfully.")
	completedEvent := createMigrationAssessmentCompletedEvent()
	controlPlane.MigrationAssessmentCompleted(completedEvent)
	err = SetMigrationAssessmentDoneInMSR()
	if err != nil {
		return fmt.Errorf("failed to set migration assessment completed in MSR: %w", err)
	}
	return nil
}

func SetMigrationAssessmentDoneInMSR() error {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.MigrationAssessmentDone = true
	})
	if err != nil {
		return fmt.Errorf("failed to update migration status record with migration assessment done flag: %w", err)
	}
	return nil
}

func IsMigrationAssessmentDone() (bool, error) {
	record, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return false, fmt.Errorf("failed to get migration status record: %w", err)
	}
	return record.MigrationAssessmentDone, nil
}

func ClearMigrationAssessmentDone() error {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		if record.MigrationAssessmentDone {
			record.MigrationAssessmentDone = false
		}
	})
	if err != nil {
		return fmt.Errorf("failed to clear migration status record with migration assessment done flag: %w", err)
	}
	return nil
}

func createMigrationAssessmentStartedEvent() *cp.MigrationAssessmentStartedEvent {
	ev := &cp.MigrationAssessmentStartedEvent{}
	initBaseSourceEvent(&ev.BaseEvent, "ASSESS MIGRATION")
	return ev
}

func createMigrationAssessmentCompletedEvent() *cp.MigrationAssessmentCompletedEvent {
	ev := &cp.MigrationAssessmentCompletedEvent{}
	initBaseSourceEvent(&ev.BaseEvent, "ASSESS MIGRATION")

	finalReport := AssessMigrationPayload{
		AssessmentJsonReport: assessmentReport,
		SourceSizeDetails: SourceDBSizeDetails{
			TotalDBSize: source.DBSize,
		},
		TargetRecommendations: TargetSizingRecommendations{},
		MigrationComplexity:   getMigrationComplexity(), //TODO: to figure out proper algorithm
		ConversionIssues:      schemaAnalysisReport.Issues,
	}
	colocatedTables, err := assessmentReport.GetColocatedTablesRecommendation()
	if err != nil {
		utils.PrintAndLog("Failed to get the colocated tables recommendation from assessmentReport: %v", err)
	}
	if assessmentReport.TableIndexStats != nil {
		for _, stat := range *assessmentReport.TableIndexStats {
			if stat.IsIndex {
				finalReport.SourceSizeDetails.TotalIndexSize += *stat.SizeInBytes
			} else {
				finalReport.SourceSizeDetails.TotalTableSize += *stat.SizeInBytes
				finalReport.SourceSizeDetails.TotalTableRowCount += *stat.RowCount
				if slices.Contains(colocatedTables, fmt.Sprintf("%s.%s", stat.SchemaName, stat.ObjectName)) {
					finalReport.TargetRecommendations.TotalColocatedSize += *stat.SizeInBytes
				} else {
					finalReport.TargetRecommendations.TotalShardedSize += *stat.SizeInBytes
				}
			}
		}
	}

	finalReportBytes, err := json.Marshal(finalReport)
	if err != nil {
		utils.PrintAndLog("Failed to serialise the final report to json (ERR IGNORED): %s", err)
	}

	ev.Report = string(finalReportBytes)
	return ev
}

func getMigrationComplexity() string {
	return "NOT AVAILABLE"
}

func runAssessment() error {
	log.Infof("running assessment for migration from '%s' to YugabyteDB", source.DBType)

	err := migassessment.SizingAssessment()
	if err != nil {
		log.Errorf("failed to perform sizing and sharding assessment: %v", err)
		return fmt.Errorf("failed to perform sizing and sharding assessment: %w", err)
	}

	assessmentReport.Sizing = migassessment.SizingReport

	shardedTables, _ := assessmentReport.GetShardedTablesRecommendation()
	colocatedTables, _ := assessmentReport.GetColocatedTablesRecommendation()
	log.Infof("Recommendation: colocated tables: %v", colocatedTables)
	log.Infof("Recommendation: sharded tables: %v", shardedTables)
	log.Infof("Recommendation: Cluster size: %s", assessmentReport.GetClusterSizingRecommendation())
	return nil
}

func checkStartCleanForAssessMigration(metadataDirPassedByUser bool) {
	assessmentDir := filepath.Join(exportDir, "assessment")
	reportsFilePattern := filepath.Join(assessmentDir, "reports", "assessmentReport.*")
	metadataFilesPattern := filepath.Join(assessmentMetadataDir, "*.csv")
	schemaFilesPattern := filepath.Join(assessmentMetadataDir, "schema", "*", "*.sql")
	dbsFilePattern := filepath.Join(assessmentDir, "dbs", "*.db")

	assessmentAlreadyDone := utils.FileOrFolderExistsWithGlobPattern(reportsFilePattern) || utils.FileOrFolderExistsWithGlobPattern(dbsFilePattern)
	if !metadataDirPassedByUser {
		assessmentAlreadyDone = assessmentAlreadyDone || utils.FileOrFolderExistsWithGlobPattern(metadataFilesPattern) ||
			utils.FileOrFolderExistsWithGlobPattern(schemaFilesPattern)
	}

	if assessmentAlreadyDone {
		if startClean {
			utils.CleanDir(filepath.Join(assessmentDir, "metadata"))
			utils.CleanDir(filepath.Join(assessmentDir, "reports"))
			utils.CleanDir(filepath.Join(assessmentDir, "dbs"))
			err := ClearMigrationAssessmentDone()
			if err != nil {
				utils.ErrExit("failed to start clean: %v", err)
			}
		} else {
			utils.ErrExit("assessment metadata or reports files already exist in the assessment directory at '%s'. Use the --start-clean flag to clear the directory before proceeding.", assessmentDir)
		}
	}
}

func gatherAssessmentMetadata() (err error) {
	if assessmentMetadataDirFlag != "" {
		return nil // assessment metadata files are provided by the user inside assessmentMetadataDir
	}

	// setting schema objects types to export before creating the project directories
	source.ExportObjectTypeList = utils.GetExportSchemaObjectList(source.DBType)
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)

	if source.Password == "" {
		source.Password, err = askPassword("source DB", source.User, "SOURCE_DB_PASSWORD")
		if err != nil {
			return fmt.Errorf("failed to get source DB password: %w", err)
		}
	}

	utils.PrintAndLog("gathering metadata and stats from '%s' source database...", source.DBType)
	switch source.DBType {
	case POSTGRESQL:
		err := gatherAssessmentMetadataFromPG()
		if err != nil {
			return fmt.Errorf("error gathering metadata and stats from source PG database: %w", err)
		}
	case ORACLE:
		err := gatherAssessmentMetadataFromOracle()
		if err != nil {
			return fmt.Errorf("error gathering metadata and stats from source Oracle database: %w", err)
		}
	default:
		return fmt.Errorf("source DB Type %s is not yet supported for metadata and stats gathering", source.DBType)
	}
	utils.PrintAndLog("gathered assessment metadata files at '%s'", assessmentMetadataDir)
	return nil
}

func gatherAssessmentMetadataFromOracle() (err error) {
	if assessmentMetadataDirFlag != "" {
		return nil
	}

	scriptPath, err := findGatherMetadataScriptPath(ORACLE)
	if err != nil {
		return err
	}

	log.Infof("using script: %s", scriptPath)
	return runGatherAssessmentMetadataScript(scriptPath, []string{"ORACLE_PASSWORD=" + source.Password},
		source.DB().GetConnectionUriWithoutPassword(), strings.ToUpper(source.Schema), assessmentMetadataDir)
}

func gatherAssessmentMetadataFromPG() (err error) {
	if assessmentMetadataDirFlag != "" {
		return nil
	}

	scriptPath, err := findGatherMetadataScriptPath(POSTGRESQL)
	if err != nil {
		return err
	}

	log.Infof("using script: %s", scriptPath)
	return runGatherAssessmentMetadataScript(scriptPath, []string{"PGPASSWORD=" + source.Password},
		source.DB().GetConnectionUriWithoutPassword(), source.Schema, assessmentMetadataDir, fmt.Sprintf("%d", intervalForCapturingIOPS))
}

func findGatherMetadataScriptPath(dbType string) (string, error) {
	var defaultScriptPath string
	switch dbType {
	case POSTGRESQL:
		defaultScriptPath = "/etc/yb-voyager/gather-assessment-metadata/postgresql/yb-voyager-pg-gather-assessment-metadata.sh"
	case ORACLE:
		defaultScriptPath = "/etc/yb-voyager/gather-assessment-metadata/oracle/yb-voyager-oracle-gather-assessment-metadata.sh"
	default:
		panic(fmt.Sprintf("invalid source db type %q", dbType))
	}

	homebrewVoyagerDir := fmt.Sprintf("yb-voyager@%s", utils.YB_VOYAGER_VERSION)
	possiblePathsForScript := []string{
		defaultScriptPath,
		filepath.Join("/", "opt", "homebrew", "Cellar", homebrewVoyagerDir, utils.YB_VOYAGER_VERSION, defaultScriptPath),
		filepath.Join("/", "usr", "local", "Cellar", homebrewVoyagerDir, utils.YB_VOYAGER_VERSION, defaultScriptPath),
	}

	for _, path := range possiblePathsForScript {
		if utils.FileOrFolderExists(path) {
			log.Infof("found the gather assessment metadata script at: %s", path)
			return path, nil
		}
	}

	return "", fmt.Errorf("script not found in possible paths: %v", possiblePathsForScript)
}

func runGatherAssessmentMetadataScript(scriptPath string, envVars []string, scriptArgs ...string) error {
	cmd := exec.Command(scriptPath, scriptArgs...)
	log.Infof("running script: %s", cmd.String())
	cmd.Env = append(cmd.Env, envVars...)
	cmd.Env = append(cmd.Env, "PATH="+os.Getenv("PATH"))
	cmd.Dir = assessmentMetadataDir
	cmd.Stdin = os.Stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("error starting gather assessment metadata script: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Errorf("[stderr of script]: %s", scanner.Text())
			fmt.Printf("%s\n", scanner.Text())
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		log.Infof("[stdout of script]: %s", scanner.Text())
		fmt.Printf("%s\n", scanner.Text())
	}

	err = cmd.Wait()
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() == 2 {
					log.Infof("Exit without error as user opted not to continue in the script.")
					os.Exit(0)
				}
			}
		}
		return fmt.Errorf("error waiting for gather assessment metadata script to complete: %w", err)
	}
	return nil
}

/*
It is due to the differences in how tools like ora2pg, and pg_dump exports the schema
pg_dump - export schema in single .sql file which is later on segregated by voyager in respective .sql file
ora2pg - export schema in given .sql file, and we have to call it for each object type to export schema
*/
func parseExportedSchemaFileForAssessmentIfRequired() {
	if source.DBType == ORACLE {
		return // already parsed into schema files while exporting
	}

	log.Infof("set 'schemaDir' as: %s", schemaDir)
	source.ApplyExportSchemaObjectListFilter()
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)
	err := source.DB().Connect()
	if err != nil {
		utils.ErrExit("error connecting source db: %v", err)
	}
	source.DBVersion = source.DB().GetVersion()
	source.DBSize, err = source.DB().GetDatabaseSize()
	if err != nil {
		log.Errorf("error getting database size: %v", err) //can just log as this is used for call-home only
	}
	source.DB().Disconnect()
	source.DB().ExportSchema(exportDir, schemaDir)
}

func populateMetadataCSVIntoAssessmentDB() error {
	metadataFilesPath, err := filepath.Glob(filepath.Join(assessmentMetadataDir, "*.csv"))
	if err != nil {
		return fmt.Errorf("error looking for csv files in directory %s: %w", assessmentMetadataDir, err)
	}

	for _, metadataFilePath := range metadataFilesPath {
		baseFileName := filepath.Base(metadataFilePath)
		metric := strings.TrimSuffix(baseFileName, filepath.Ext(baseFileName))
		tableName := strings.Replace(metric, "-", "_", -1)
		log.Infof("populating metadata from file %s into table %s", metadataFilePath, tableName)
		file, err := os.Open(metadataFilePath)
		if err != nil {
			log.Warnf("error opening file %s: %v", metadataFilePath, err)
			return nil
		}

		csvReader := csv.NewReader(file)
		csvReader.ReuseRecord = true
		rows, err := csvReader.ReadAll()
		if err != nil {
			log.Errorf("error reading csv file %s: %v", metadataFilePath, err)
			return fmt.Errorf("error reading csv file %s: %w", metadataFilePath, err)
		}

		// collecting both initial and final measurement in the same table
		if strings.Contains(tableName, migassessment.TABLE_INDEX_IOPS) {
			tableName = migassessment.TABLE_INDEX_IOPS
		}

		err = assessmentDB.BulkInsert(tableName, rows)
		if err != nil {
			return fmt.Errorf("error bulk inserting data into %s table: %w", tableName, err)
		}

		log.Infof("populated metadata from file %s into table %s", metadataFilePath, tableName)
	}

	err = assessmentDB.PopulateMigrationAssessmentStats()
	if err != nil {
		return fmt.Errorf("failed to populate migration assessment stats: %w", err)
	}
	return nil
}

//go:embed assessmentReport.template
var bytesTemplate []byte

func generateAssessmentReport() (err error) {
	utils.PrintAndLog("Generating assessment report...")

	err = getAssessmentReportContentFromAnalyzeSchema()
	if err != nil {
		return fmt.Errorf("failed to generate assessment report content from analyze schema: %w", err)
	}

	assessmentReport.UnsupportedDataTypes, err = fetchColumnsWithUnsupportedDataTypes()
	if err != nil {
		return fmt.Errorf("failed to fetch columns with unsupported data types: %w", err)
	}

	assessmentReport.Sizing = migassessment.SizingReport
	assessmentReport.TableIndexStats, err = assessmentDB.FetchAllStats()
	if err != nil {
		return fmt.Errorf("fetching all stats info from AssessmentDB: %w", err)
	}

	assessmentReportDir := filepath.Join(exportDir, "assessment", "reports")
	err = generateAssessmentReportJson(assessmentReportDir)
	if err != nil {
		return fmt.Errorf("failed to generate assessment report JSON: %w", err)
	}

	err = generateAssessmentReportHtml(assessmentReportDir)
	if err != nil {
		return fmt.Errorf("failed to generate assessment report HTML: %w", err)
	}
	return nil
}

func getAssessmentReportContentFromAnalyzeSchema() (err error) {
	schemaAnalysisReport := analyzeSchemaInternal(&source)
	assessmentReport.SchemaSummary = schemaAnalysisReport.SchemaSummary

	// set invalidCount to zero so that it doesn't show up in the report
	for i := 0; i < len(assessmentReport.SchemaSummary.DBObjects); i++ {
		assessmentReport.SchemaSummary.DBObjects[i].InvalidCount = 0
	}

	unsupportedFeatures, err := fetchUnsupportedFeaturesOfSourceInYb(schemaAnalysisReport)
	if err != nil {
		return fmt.Errorf("failed to fetch unsupported features: %w", err)
	}
	assessmentReport.UnsupportedFeatures = unsupportedFeatures

	return nil
}

func fetchUnsupportedFeaturesOfSourceInYb(schemaAnalysisReport utils.SchemaReport) ([]UnsupportedFeature, error) {
	switch source.DBType {
	case POSTGRESQL:
		return fetchUnsupportedFeaturesForPG(schemaAnalysisReport)
	case ORACLE:
		return nil, nil
	default:
		panic("unsupported source db type")
	}
}

func fetchUnsupportedFeaturesForPG(schemaAnalysisReport utils.SchemaReport) ([]UnsupportedFeature, error) {
	log.Infof("fetching unsupported features for PG...")
	unsupportedFeatures := make([]UnsupportedFeature, 0)
	filterIssues := func(featureName, issueReason string) {
		log.Info("filtering issues for feature: ", featureName)
		objectNames := make([]string, 0)
		for _, issue := range schemaAnalysisReport.Issues {
			if strings.Contains(issue.Reason, issueReason) {
				objectNames = append(objectNames, issue.ObjectName)
			}
		}
		unsupportedFeatures = append(unsupportedFeatures, UnsupportedFeature{featureName, objectNames})
	}

	filterIssues("GIST indexes", GIST_INDEX_ISSUE_REASON)
	filterIssues("Constraint triggers", CONSTRAINT_TRIGGER_ISSUE_REASON)
	filterIssues("Inherited tables", INHERITANCE_ISSUE_REASON)
	filterIssues("Tables with Stored generated columns", STORED_GENERATED_COLUMN_ISSUE_REASON)

	return unsupportedFeatures, nil
}

func fetchColumnsWithUnsupportedDataTypes() ([]utils.TableColumnsDataTypes, error) {
	var unsupportedDataTypes []utils.TableColumnsDataTypes

	query := fmt.Sprintf(`SELECT schema_name, table_name, column_name, data_type FROM %s`,
		migassessment.TABLE_COLUMNS_DATA_TYPES)
	rows, err := assessmentDB.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("error closing rows while fetching unsupported datatypes metadata: %v", err)
		}
	}()

	var allColumnsDataTypes []utils.TableColumnsDataTypes
	for rows.Next() {
		var columnDataTypes utils.TableColumnsDataTypes
		err := rows.Scan(&columnDataTypes.SchemaName, &columnDataTypes.TableName,
			&columnDataTypes.ColumnName, &columnDataTypes.DataType)
		if err != nil {
			return nil, fmt.Errorf("error scanning rows: %w", err)
		}

		allColumnsDataTypes = append(allColumnsDataTypes, columnDataTypes)
	}

	// filter columns with unsupported data types using srcdb.PostgresUnsupportedDataTypesForDbzm
	pgUnsupportedDataTypes := srcdb.PostgresUnsupportedDataTypesForDbzm
	for i := 0; i < len(allColumnsDataTypes); i++ {
		if utils.ContainsAnySubstringFromSlice(pgUnsupportedDataTypes, allColumnsDataTypes[i].DataType) {
			unsupportedDataTypes = append(unsupportedDataTypes, allColumnsDataTypes[i])
		}
	}

	return unsupportedDataTypes, nil
}

func generateAssessmentReportJson(reportDir string) error {
	jsonReportFilePath := filepath.Join(reportDir, "assessmentReport.json")
	log.Infof("writing assessment report to file: %s", jsonReportFilePath)
	strReport, err := json.MarshalIndent(assessmentReport, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal the assessment report: %w", err)
	}

	err = os.WriteFile(jsonReportFilePath, strReport, 0644)
	if err != nil {
		return fmt.Errorf("failed to write assessment report to file: %w", err)
	}

	utils.PrintAndLog("generated JSON assessment report at: %s", jsonReportFilePath)
	return nil
}

func generateAssessmentReportHtml(reportDir string) error {
	htmlReportFilePath := filepath.Join(reportDir, "assessmentReport.html")
	log.Infof("writing assessment report to file: %s", htmlReportFilePath)

	file, err := os.Create(htmlReportFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file for %q: %w", filepath.Base(htmlReportFilePath), err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Errorf("failed to close file %q: %v", htmlReportFilePath, err)
		}
	}()

	log.Infof("creating template for assessment report...")
	funcMap := template.FuncMap{
		"split": split,
	}
	tmpl := template.Must(template.New("report").Funcs(funcMap).Parse(string(bytesTemplate)))

	log.Infof("execute template for assessment report...")
	err = tmpl.Execute(file, assessmentReport)
	if err != nil {
		return fmt.Errorf("failed to render the assessment report: %w", err)
	}

	utils.PrintAndLog("generated HTML assessment report at: %s", htmlReportFilePath)
	return nil
}

func split(value string, delimiter string) []string {
	return strings.Split(value, delimiter)
}

func validateSourceDBTypeForAssessMigration() {
	if source.DBType == "" {
		utils.ErrExit("Error: required flag \"source-db-type\" not set")
	}

	assessMigrationSupportedDBTypes := []string{POSTGRESQL, ORACLE}
	source.DBType = strings.ToLower(source.DBType)
	if !slices.Contains(assessMigrationSupportedDBTypes, source.DBType) {
		utils.ErrExit("Error: Invalid source-db-type: %q. Supported source db types for assess-migration are: %v", assessMigrationSupportedDBTypes, source.DBType)
	}
}

func validateAssessmentMetadataDirFlag() {
	if assessmentMetadataDirFlag != "" {
		if !utils.FileOrFolderExists(assessmentMetadataDirFlag) {
			utils.ErrExit("assessment metadata directory %q provided with `--assessment-metadata-dir` flag does not exist", assessmentMetadataDirFlag)
		} else {
			log.Infof("using provided assessment metadata directory: %s", assessmentMetadataDirFlag)
		}
	}
}
