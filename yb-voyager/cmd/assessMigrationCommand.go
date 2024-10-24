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
	"sort"
	"strings"
	"syscall"
	"text/template"

	"github.com/fatih/color"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var (
	assessmentMetadataDir            string
	assessmentMetadataDirFlag        string
	assessmentReport                 AssessmentReport
	assessmentDB                     *migassessment.AssessmentDB
	intervalForCapturingIOPS         int64
	assessMigrationSupportedDBTypes  = []string{POSTGRESQL, ORACLE}
	referenceOrTablePartitionPresent = false
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
	FeatureName        string       `json:"FeatureName"`
	Objects            []ObjectInfo `json:"Objects"`
	DisplayDDL         bool         `json:"-"` // just used by html format to display the DDL for some feature and object names for other
	DocsLink           string       `json:"DocsLink,omitempty"`
	FeatureDescription string       `json:"FeatureDescription,omitempty"`
}

type ObjectInfo struct {
	ObjectName   string
	SqlStatement string
}

var assessMigrationCmd = &cobra.Command{
	Use:   "assess-migration",
	Short: fmt.Sprintf("Assess the migration from source (%s) database to YugabyteDB.", strings.Join(assessMigrationSupportedDBTypes, ", ")),
	Long:  fmt.Sprintf("Assess the migration from source (%s) database to YugabyteDB.", strings.Join(assessMigrationSupportedDBTypes, ", ")),

	PreRun: func(cmd *cobra.Command, args []string) {
		validateSourceDBTypeForAssessMigration()
		setExportFlagsDefaults()
		validateSourceSchema()
		validatePortRange()
		validateSSLMode()
		validateOracleParams()
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
			utils.ErrExit("failed to assess migration: %s", err)
		}
		packAndSendAssessMigrationPayload(COMPLETE, "")
	},
}

func packAndSendAssessMigrationPayload(status string, errMsg string) {
	if !shouldSendCallhome() {
		return
	}
	payload := createCallhomePayload()

	payload.MigrationPhase = ASSESS_MIGRATION_PHASE

	var tableSizingStats, indexSizingStats []callhome.ObjectSizingStats
	if assessmentReport.TableIndexStats != nil {
		for _, stat := range *assessmentReport.TableIndexStats {
			newStat := callhome.ObjectSizingStats{
				//redacting schema and object name
				ObjectName:      "XXX",
				ReadsPerSecond:  utils.SafeDereferenceInt64(stat.ReadsPerSecond),
				WritesPerSecond: utils.SafeDereferenceInt64(stat.WritesPerSecond),
				SizeInBytes:     utils.SafeDereferenceInt64(stat.SizeInBytes),
			}
			if stat.IsIndex {
				indexSizingStats = append(indexSizingStats, newStat)
			} else {
				tableSizingStats = append(tableSizingStats, newStat)
			}
		}
	}
	schemaSummaryCopy := utils.SchemaSummary{
		MigrationComplexity: assessmentReport.SchemaSummary.MigrationComplexity,
		Notes:               assessmentReport.SchemaSummary.Notes,
		DBObjects: lo.Map(schemaAnalysisReport.SchemaSummary.DBObjects, func(dbObject utils.DBObject, _ int) utils.DBObject {
			dbObject.ObjectNames = ""
			return dbObject
		}),
	}

	unsupportedDatatypesList := lo.Map(assessmentReport.UnsupportedDataTypes, func(datatype utils.TableColumnsDataTypes, _ int) string {
		return datatype.DataType
	})

	assessPayload := callhome.AssessMigrationPhasePayload{
		UnsupportedFeatures: callhome.MarshalledJsonString(lo.Map(assessmentReport.UnsupportedFeatures, func(feature UnsupportedFeature, _ int) callhome.UnsupportedFeature {
			return callhome.UnsupportedFeature{
				FeatureName: feature.FeatureName,
				ObjectCount: len(feature.Objects),
			}
		})),
		UnsupportedDatatypes: callhome.MarshalledJsonString(unsupportedDatatypesList),
		TableSizingStats:     callhome.MarshalledJsonString(tableSizingStats),
		IndexSizingStats:     callhome.MarshalledJsonString(indexSizingStats),
		SchemaSummary:        callhome.MarshalledJsonString(schemaSummaryCopy),
		IopsInterval:         intervalForCapturingIOPS,
	}
	if status == ERROR {
		assessPayload.Error = "ERROR" // removing error for now, TODO to see if we want to keep it
	}
	if assessmentMetadataDirFlag == "" {
		sourceDBDetails := callhome.SourceDBDetails{
			DBType:    source.DBType,
			DBVersion: source.DBVersion,
			DBSize:    source.DBSize,
		}
		payload.SourceDBDetails = callhome.MarshalledJsonString(sourceDBDetails)
		assessPayload.SourceConnectivity = true
	} else {
		assessPayload.SourceConnectivity = false
	}
	payload.PhasePayload = callhome.MarshalledJsonString(assessPayload)
	payload.Status = status

	err := callhome.SendPayload(&payload)
	if err == nil && (status == COMPLETE || status == ERROR) {
		callHomeErrorOrCompletePayloadSent = true
	}
}

func registerSourceDBConnFlagsForAM(cmd *cobra.Command) {
	cmd.Flags().StringVar(&source.DBType, "source-db-type", "",
		fmt.Sprintf("source database type: (%s)\n", strings.Join(assessMigrationSupportedDBTypes, ", ")))

	cmd.MarkFlagRequired("source-db-type")

	cmd.Flags().StringVar(&source.Host, "source-db-host", "localhost",
		"source database server host")

	cmd.Flags().IntVar(&source.Port, "source-db-port", 0,
		"source database server port number. Default: PostgreSQL(5432), Oracle(1521)")

	cmd.Flags().StringVar(&source.User, "source-db-user", "",
		"connect to source database as the specified user")

	// TODO: All sensitive parameters can be taken from the environment variable
	cmd.Flags().StringVar(&source.Password, "source-db-password", "",
		"source password to connect as the specified user. Alternatively, you can also specify the password by setting the environment variable SOURCE_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")

	cmd.Flags().StringVar(&source.DBName, "source-db-name", "",
		"source database name to be migrated to YugabyteDB")

	cmd.Flags().StringVar(&source.Schema, "source-db-schema", "",
		"source schema name(s) to export\n"+
			`Note: in case of PostgreSQL, it can be a single or comma separated list of schemas: "schema1,schema2,schema3"`)

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

	cmd.Flags().StringVar(&source.DBSid, "oracle-db-sid", "",
		"[For Oracle Only] Oracle System Identifier (SID) that you wish to use while exporting data from Oracle instances")

	cmd.Flags().StringVar(&source.OracleHome, "oracle-home", "",
		"[For Oracle Only] Path to set $ORACLE_HOME environment variable. tnsnames.ora is found in $ORACLE_HOME/network/admin")

	cmd.Flags().StringVar(&source.TNSAlias, "oracle-tns-alias", "",
		"[For Oracle Only] Name of TNS Alias you wish to use to connect to Oracle instance. Refer to documentation to learn more about configuring tnsnames.ora and aliases")
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

	BoolVar(assessMigrationCmd.Flags(), &source.RunGuardrailsChecks, "run-guardrails-checks", false, "run guardrails checks before assess migration. (only valid for PostgreSQL)")
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

	assessmentDir := filepath.Join(exportDir, "assessment")
	migassessment.AssessmentDir = assessmentDir
	migassessment.SourceDBType = source.DBType

	if source.Password == "" {
		source.Password, err = askPassword("source DB", source.User, "SOURCE_DB_PASSWORD")
		if err != nil {
			return fmt.Errorf("failed to get source DB password: %w", err)
		}
	}

	if assessmentMetadataDirFlag == "" { // only in case of source connectivity
		err := source.DB().Connect()
		if err != nil {
			utils.ErrExit("error connecting source db: %v", err)
		}

		// We will require source db connection for the below checks
		// Check if required binaries are installed.
		if source.RunGuardrailsChecks {
			binaryCheckIssues, err := checkDependenciesForExport()
			if err != nil {
				return fmt.Errorf("failed to check dependencies for assess migration: %w", err)
			} else if len(binaryCheckIssues) > 0 {
				return fmt.Errorf("\n%s\n%s", color.RedString("\nMissing dependencies for assess migration:"), strings.Join(binaryCheckIssues, "\n"))
			}
		}

		res := source.DB().CheckSchemaExists()
		if !res {
			return fmt.Errorf("schema %q does not exist", source.Schema)
		}

		// Check if source db has permissions to assess migration
		if source.RunGuardrailsChecks {
			missingPerms, err := source.DB().GetMissingExportSchemaPermissions()
			if err != nil {
				return fmt.Errorf("failed to get missing assess migration permissions: %w", err)
			}
			if len(missingPerms) > 0 {
				color.Red("\nPermissions missing in the source database for assess migration:\n")
				output := strings.Join(missingPerms, "\n")
				utils.PrintAndLog("%s\n\n", output)

				link := "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/#prepare-the-source-database"
				fmt.Println("Check the documentation to prepare the database for migration:", color.BlueString(link))

				reply := utils.AskPrompt("\nDo you want to continue anyway")
				if !reply {
					return fmt.Errorf("grant the required permissions and try again")
				}
			}
		}

		fetchSourceInfo()

		source.DB().Disconnect()
	}

	startEvent := createMigrationAssessmentStartedEvent()
	controlPlane.MigrationAssessmentStarted(startEvent)

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

func fetchSourceInfo() {
	var err error
	source.DBVersion = source.DB().GetVersion()
	source.DBSize, err = source.DB().GetDatabaseSize()
	if err != nil {
		log.Errorf("error getting database size: %v", err) //can just log as this is used for call-home only
	}
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

func IsMigrationAssessmentDone(metaDBInstance *metadb.MetaDB) (bool, error) {
	record, err := metaDBInstance.GetMigrationStatusRecord()
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

	sizeDetails, err := assessmentReport.CalculateSizeDetails(source.DBType)
	if err != nil {
		utils.PrintAndLog("Failed to calculate the size details of the tableIndexStats: %v", err)
	}

	finalReport := AssessMigrationPayload{
		AssessmentJsonReport: assessmentReport,
		SourceSizeDetails: SourceDBSizeDetails{
			TotalIndexSize:     sizeDetails.TotalIndexSize,
			TotalTableSize:     sizeDetails.TotalTableSize,
			TotalTableRowCount: sizeDetails.TotalTableRowCount,
			TotalDBSize:        source.DBSize,
		},
		TargetRecommendations: TargetSizingRecommendations{
			TotalColocatedSize: sizeDetails.TotalColocatedSize,
			TotalShardedSize:   sizeDetails.TotalShardedSize,
		},
		MigrationComplexity: assessmentReport.SchemaSummary.MigrationComplexity,
		ConversionIssues:    schemaAnalysisReport.Issues,
	}

	finalReportBytes, err := json.Marshal(finalReport)
	if err != nil {
		utils.PrintAndLog("Failed to serialise the final report to json (ERR IGNORED): %s", err)
	}

	ev.Report = string(finalReportBytes)
	return ev
}

type SizeDetails struct {
	TotalIndexSize     int64
	TotalTableSize     int64
	TotalTableRowCount int64
	TotalColocatedSize int64
	TotalShardedSize   int64
}

func (ar *AssessmentReport) CalculateSizeDetails(dbType string) (SizeDetails, error) {
	var details SizeDetails
	colocatedTables, err := ar.GetColocatedTablesRecommendation()
	if err != nil {
		return details, fmt.Errorf("failed to get the colocated tables recommendation: %v", err)
	}

	if ar.TableIndexStats != nil {
		for _, stat := range *ar.TableIndexStats {
			if stat.IsIndex {
				details.TotalIndexSize += utils.SafeDereferenceInt64(stat.SizeInBytes)
			} else {
				var tableName string
				switch dbType {
				case ORACLE:
					tableName = stat.ObjectName // in case of oracle, colocatedTables have unqualified table names
				case POSTGRESQL:
					tableName = fmt.Sprintf("%s.%s", stat.SchemaName, stat.ObjectName)
				default:
					return details, fmt.Errorf("dbType %s is not yet supported for calculating size details", dbType)
				}
				details.TotalTableSize += utils.SafeDereferenceInt64(stat.SizeInBytes)
				details.TotalTableRowCount += utils.SafeDereferenceInt64(stat.RowCount)
				if slices.Contains(colocatedTables, tableName) {
					details.TotalColocatedSize += utils.SafeDereferenceInt64(stat.SizeInBytes)
				} else {
					details.TotalShardedSize += utils.SafeDereferenceInt64(stat.SizeInBytes)
				}
			}
		}
	}
	return details, nil
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
	reportsFilePattern := filepath.Join(assessmentDir, "reports", fmt.Sprintf("%s.*", ASSESSMENT_FILE_NAME))
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

	tnsAdmin, err := getTNSAdmin(source)
	if err != nil {
		return fmt.Errorf("error getting tnsAdmin: %v", err)
	}
	envVars := []string{fmt.Sprintf("ORACLE_PASSWORD=%s", source.Password),
		fmt.Sprintf("TNS_ADMIN=%s", tnsAdmin),
		fmt.Sprintf("ORACLE_HOME=%s", source.GetOracleHome()),
	}
	log.Infof("environment variables passed to oracle gather metadata script: %v", envVars)
	return runGatherAssessmentMetadataScript(scriptPath, envVars,
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
	return runGatherAssessmentMetadataScript(scriptPath, []string{fmt.Sprintf("PGPASSWORD=%s", source.Password)},
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
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, envVars...)
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
		// collecting both initial and final measurement in the same table
		tableName = lo.Ternary(strings.Contains(tableName, migassessment.TABLE_INDEX_IOPS),
			migassessment.TABLE_INDEX_IOPS, tableName)

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

//go:embed templates/assessmentReport.template
var bytesTemplate []byte

func generateAssessmentReport() (err error) {
	utils.PrintAndLog("Generating assessment report...")

	err = getAssessmentReportContentFromAnalyzeSchema()
	if err != nil {
		return fmt.Errorf("failed to generate assessment report content from analyze schema: %w", err)
	}

	unsupportedFeatures, err := fetchUnsupportedObjectTypes()
	if err != nil {
		return fmt.Errorf("failed to fetch unsupported object types: %w", err)
	}
	assessmentReport.UnsupportedFeatures = append(assessmentReport.UnsupportedFeatures, unsupportedFeatures...)

	if utils.GetEnvAsBool("REPORT_UNSUPPORTED_QUERY_CONSTRUCTS", true) {
		unsupportedQueries, err := fetchUnsupportedQueryConstructs()
		if err != nil {
			return fmt.Errorf("failed to fetch unsupported queries on YugabyteDB: %w", err)
		}
		assessmentReport.UnsupportedQueryConstructs = unsupportedQueries
	}

	assessmentReport.VoyagerVersion = utils.YB_VOYAGER_VERSION
	unsupportedDataTypes, unsupportedDataTypesForLiveMigration, unsupportedDataTypesForLiveMigrationWithFForFB, err := fetchColumnsWithUnsupportedDataTypes()
	if err != nil {
		return fmt.Errorf("failed to fetch columns with unsupported data types: %w", err)
	}
	assessmentReport.UnsupportedDataTypes = unsupportedDataTypes
	assessmentReport.UnsupportedDataTypesDesc = "Data types of the source database that are not supported on the target YugabyteDB."

	assessmentReport.Sizing = migassessment.SizingReport
	assessmentReport.TableIndexStats, err = assessmentDB.FetchAllStats()
	if err != nil {
		return fmt.Errorf("fetching all stats info from AssessmentDB: %w", err)
	}

	addNotesToAssessmentReport()
	addMigrationCaveatsToAssessmentReport(unsupportedDataTypesForLiveMigration, unsupportedDataTypesForLiveMigrationWithFForFB)
	postProcessingOfAssessmentReport()

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

func getAssessmentReportContentFromAnalyzeSchema() error {
	schemaAnalysisReport := analyzeSchemaInternal(&source)
	assessmentReport.SchemaSummary = schemaAnalysisReport.SchemaSummary
	assessmentReport.SchemaSummaryDBObjectsDesc = "Objects that will be created on the target YugabyteDB."
	if source.DBType == ORACLE {
		assessmentReport.SchemaSummaryDBObjectsDesc += " Some of the index and sequence names might be different from those in the source database."
	}

	// set invalidCount to zero so that it doesn't show up in the report
	for i := 0; i < len(assessmentReport.SchemaSummary.DBObjects); i++ {
		assessmentReport.SchemaSummary.DBObjects[i].InvalidCount = 0
	}

	// fetching unsupportedFeaturing with the help of Issues report in SchemaReport
	var unsupportedFeatures []UnsupportedFeature
	var err error
	switch source.DBType {
	case ORACLE:
		unsupportedFeatures, err = fetchUnsupportedOracleFeaturesFromSchemaReport(schemaAnalysisReport)
	case POSTGRESQL:
		unsupportedFeatures, err = fetchUnsupportedPGFeaturesFromSchemaReport(schemaAnalysisReport)
	default:
		panic(fmt.Sprintf("unsupported source db type %q", source.DBType))
	}
	if err != nil {
		return fmt.Errorf("failed to fetch %s unsupported features: %w", source.DBType, err)
	}
	assessmentReport.UnsupportedFeatures = append(assessmentReport.UnsupportedFeatures, unsupportedFeatures...)
	assessmentReport.UnsupportedFeaturesDesc = "Features of the source database that are not supported on the target YugabyteDB."
	return nil
}

func getUnsupportedFeaturesFromSchemaAnalysisReport(featureName string, issueReason string, schemaAnalysisReport utils.SchemaReport, displayDDLInHTML bool, description string) UnsupportedFeature {
	log.Info("filtering issues for feature: ", featureName)
	objects := make([]ObjectInfo, 0)
	link := "" // for oracle we shouldn't display any line for links
	for _, issue := range schemaAnalysisReport.Issues {
		if strings.Contains(issue.Reason, issueReason) {
			objectInfo := ObjectInfo{
				ObjectName:   issue.ObjectName,
				SqlStatement: issue.SqlStatement,
			}
			if source.DBType == POSTGRESQL {
				link = issue.DocsLink
			}
			objects = append(objects, objectInfo)
		}
	}
	return UnsupportedFeature{featureName, objects, displayDDLInHTML, link, description}
}

func fetchUnsupportedPGFeaturesFromSchemaReport(schemaAnalysisReport utils.SchemaReport) ([]UnsupportedFeature, error) {
	log.Infof("fetching unsupported features for PG...")
	unsupportedFeatures := make([]UnsupportedFeature, 0)
	for _, indexMethod := range unsupportedIndexMethods {
		displayIndexMethod := strings.ToUpper(indexMethod)
		feature := fmt.Sprintf("%s indexes", displayIndexMethod)
		reason := fmt.Sprintf(INDEX_METHOD_ISSUE_REASON, displayIndexMethod)
		unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(feature, reason, schemaAnalysisReport, false, ""))
	}
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Constraint triggers", CONSTRAINT_TRIGGER_ISSUE_REASON, schemaAnalysisReport, false, ""))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Inherited tables", INHERITANCE_ISSUE_REASON, schemaAnalysisReport, false, ""))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Tables with stored generated columns", STORED_GENERATED_COLUMN_ISSUE_REASON, schemaAnalysisReport, false, ""))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Conversion objects", CONVERSION_ISSUE_REASON, schemaAnalysisReport, false, ""))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Gin indexes on multi-columns", GIN_INDEX_MULTI_COLUMN_ISSUE_REASON, schemaAnalysisReport, false, ""))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Setting attribute=value on column", ALTER_TABLE_SET_ATTRUBUTE_ISSUE, schemaAnalysisReport, true, ""))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Disabling rule on table", ALTER_TABLE_DISABLE_RULE_ISSUE, schemaAnalysisReport, true, ""))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Clustering table on index", ALTER_TABLE_CLUSTER_ON_ISSUE, schemaAnalysisReport, true, ""))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Storage parameters in DDLs", STORAGE_PARAMETERS_DDL_STMT_ISSUE, schemaAnalysisReport, true, ""))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Extensions", UNSUPPORTED_EXTENSION_ISSUE, schemaAnalysisReport, false, ""))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Exclusion constraints", EXCLUSION_CONSTRAINT_ISSUE, schemaAnalysisReport, false, ""))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Deferrable constraints", DEFERRABLE_CONSTRAINT_ISSUE, schemaAnalysisReport, false, ""))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("View with check option", VIEW_CHECK_OPTION_ISSUE, schemaAnalysisReport, false, ""))
	unsupportedFeatures = append(unsupportedFeatures, getIndexesOnComplexTypeUnsupportedFeature(schemaAnalysisReport, UnsupportedIndexDatatypes))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Unlogged tables", ISSUE_UNLOGGED_TABLE, schemaAnalysisReport, false, ""))

	return unsupportedFeatures, nil
}

func getIndexesOnComplexTypeUnsupportedFeature(schemaAnalysisiReport utils.SchemaReport, unsupportedIndexDatatypes []string) UnsupportedFeature {
	indexesOnComplexTypesFeature := UnsupportedFeature{
		FeatureName: "Index on complex datatypes",
		DisplayDDL:  false,
		Objects:     []ObjectInfo{},
	}
	unsupportedIndexDatatypes = append(unsupportedIndexDatatypes, "array")             // adding it here only as we know issue form analyze will come with type
	unsupportedIndexDatatypes = append(unsupportedIndexDatatypes, "user_defined_type") // adding it here as we UDTs will come with this type.
	for _, unsupportedType := range unsupportedIndexDatatypes {
		indexes := getUnsupportedFeaturesFromSchemaAnalysisReport(fmt.Sprintf("%s indexes", unsupportedType), fmt.Sprintf(ISSUE_INDEX_WITH_COMPLEX_DATATYPES, unsupportedType), schemaAnalysisReport, false, "")
		for _, object := range indexes.Objects {
			formattedObject := object
			formattedObject.ObjectName = fmt.Sprintf("%s: %s", strings.ToUpper(unsupportedType), object.ObjectName)
			indexesOnComplexTypesFeature.Objects = append(indexesOnComplexTypesFeature.Objects, formattedObject)
			if indexesOnComplexTypesFeature.DocsLink == "" {
				indexesOnComplexTypesFeature.DocsLink = indexes.DocsLink
			}
		}
	}

	return indexesOnComplexTypesFeature
}

func fetchUnsupportedOracleFeaturesFromSchemaReport(schemaAnalysisReport utils.SchemaReport) ([]UnsupportedFeature, error) {
	log.Infof("fetching unsupported features for Oracle...")
	unsupportedFeatures := make([]UnsupportedFeature, 0)
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport("Compound Triggers", COMPOUND_TRIGGER_ISSUE_REASON, schemaAnalysisReport, false, ""))
	return unsupportedFeatures, nil
}

var OracleUnsupportedIndexTypes = []string{"CLUSTER INDEX", "DOMAIN INDEX", "FUNCTION-BASED DOMAIN INDEX", "IOT - TOP INDEX", "NORMAL/REV INDEX", "FUNCTION-BASED NORMAL/REV INDEX"}

func fetchUnsupportedObjectTypes() ([]UnsupportedFeature, error) {
	if source.DBType != ORACLE {
		return nil, nil
	}

	query := fmt.Sprintf(`SELECT schema_name, object_name, object_type FROM %s`, migassessment.OBJECT_TYPE_MAPPING)
	rows, err := assessmentDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying-%s: %w", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("error closing rows while fetching object type mapping metadata: %v", err)
		}
	}()

	var unsupportedIndexes, virtualColumns, inheritedTypes, unsupportedPartitionTypes []ObjectInfo
	for rows.Next() {
		var schemaName, objectName, objectType string
		err = rows.Scan(&schemaName, &objectName, &objectType)
		if err != nil {
			return nil, fmt.Errorf("error scanning rows:%w", err)
		}

		if slices.Contains(OracleUnsupportedIndexTypes, objectType) {
			unsupportedIndexes = append(unsupportedIndexes, ObjectInfo{
				ObjectName: fmt.Sprintf("Index Name: %s, Index Type=%s", objectName, objectType),
			})
		} else if objectType == VIRTUAL_COLUMN {
			virtualColumns = append(virtualColumns, ObjectInfo{ObjectName: objectName})
		} else if objectType == INHERITED_TYPE {
			inheritedTypes = append(inheritedTypes, ObjectInfo{ObjectName: objectName})
		} else if objectType == REFERENCE_PARTITION || objectType == SYSTEM_PARTITION {
			referenceOrTablePartitionPresent = true
			unsupportedPartitionTypes = append(unsupportedPartitionTypes, ObjectInfo{ObjectName: fmt.Sprintf("Table Name: %s, Partition Method: %s", objectName, objectType)})
		}
	}

	unsupportedFeatures := make([]UnsupportedFeature, 0)
	unsupportedFeatures = append(unsupportedFeatures, UnsupportedFeature{"Unsupported Indexes", unsupportedIndexes, false, "", ""})
	unsupportedFeatures = append(unsupportedFeatures, UnsupportedFeature{"Virtual Columns", virtualColumns, false, "", ""})
	unsupportedFeatures = append(unsupportedFeatures, UnsupportedFeature{"Inherited Types", inheritedTypes, false, "", ""})
	unsupportedFeatures = append(unsupportedFeatures, UnsupportedFeature{"Unsupported Partitioning Methods", unsupportedPartitionTypes, false, "", ""})
	return unsupportedFeatures, nil
}

func fetchUnsupportedQueryConstructs() ([]utils.UnsupportedQueryConstruct, error) {
	if source.DBType != POSTGRESQL {
		return nil, nil
	}
	query := fmt.Sprintf("SELECT DISTINCT query from %s", migassessment.DB_QUERIES_SUMMARY)
	rows, err := assessmentDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying=%s on assessmentDB: %w", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("error closing rows while fetching database queries summary metadata: %v", err)
		}
	}()

	var executedQueries []string
	for rows.Next() {
		var executedQuery string
		err := rows.Scan(&executedQuery)
		if err != nil {
			return nil, fmt.Errorf("error scanning rows: %w", err)
		}
		executedQueries = append(executedQueries, executedQuery)
	}

	if len(executedQueries) == 0 {
		log.Infof("queries info not present in the assessment metadata for detecting unsupported query constructs")
		return nil, nil
	}

	var result []utils.UnsupportedQueryConstruct
	for i := 0; i < len(executedQueries); i++ {
		query := executedQueries[i]
		log.Debugf("fetching unsupported query constructs for query - [%s]", query)
		queryParser := queryparser.New(query)
		err := queryParser.Parse()
		if err != nil {
			log.Errorf("failed to parse query - [%s]: %v", query, err)
		}

		unsupportedConstructs, err := queryParser.GetUnsupportedQueryConstructs()
		if err != nil {
			log.Errorf("failed while trying to fetch unsupported constructs from parse tree of query - [%s]: %s",
				query, err.Error())
		}
		if unsupportedConstructs != nil {
			result = append(result, unsupportedConstructs...)
		}
	}

	// sort the slice to group same constructType in html and json reports
	log.Infof("sorting the result slice based on construct type")
	sort.Slice(result, func(i, j int) bool {
		return result[i].ConstructType <= result[j].ConstructType
	})
	return result, nil
}

func fetchColumnsWithUnsupportedDataTypes() ([]utils.TableColumnsDataTypes, []utils.TableColumnsDataTypes, []utils.TableColumnsDataTypes, error) {
	var unsupportedDataTypes, unsupportedDataTypesForLiveMigration, unsupportedDataTypesForLiveMigrationWithFForFB []utils.TableColumnsDataTypes

	query := fmt.Sprintf(`SELECT schema_name, table_name, column_name, data_type FROM %s`,
		migassessment.TABLE_COLUMNS_DATA_TYPES)
	rows, err := assessmentDB.Query(query)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error querying-%s on assessmentDB: %w", query, err)
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
			return nil, nil, nil, fmt.Errorf("error scanning rows: %w", err)
		}

		allColumnsDataTypes = append(allColumnsDataTypes, columnDataTypes)
	}

	var sourceUnsupportedDataTypes, liveMigrationUnsupportedDataTypes, liveMigrationWithFForFBUnsupportedDatatypes []string

	switch source.DBType {
	case POSTGRESQL:
		sourceUnsupportedDataTypes = srcdb.PostgresUnsupportedDataTypes
		liveMigrationUnsupportedDataTypes, _ = lo.Difference(srcdb.PostgresUnsupportedDataTypesForDbzm, srcdb.PostgresUnsupportedDataTypes)
		unsupportedDataTypesForDbzmYBOnly, _ := lo.Difference(srcdb.YugabyteUnsupportedDataTypesForDbzm, srcdb.PostgresUnsupportedDataTypes)
		liveMigrationWithFForFBUnsupportedDatatypes, _ = lo.Difference(unsupportedDataTypesForDbzmYBOnly, liveMigrationUnsupportedDataTypes)
	case ORACLE:
		sourceUnsupportedDataTypes = srcdb.OracleUnsupportedDataTypes
	default:
		panic(fmt.Sprintf("invalid source db type %q", source.DBType))
	}
	// filter columns with unsupported data types using sourceUnsupportedDataTypes
	for i := 0; i < len(allColumnsDataTypes); i++ {
		//Using this ContainsAnyStringFromSlice as the catalog we use for fetching datatypes uses the data_type only
		// which just contains the base type for example VARCHARs it won't include any length, precision or scale information
		//of these types there are other columns available for these information so we just do string match of types with our list
		splits := strings.Split(allColumnsDataTypes[i].DataType, ".")
		typeName := splits[len(splits)-1] //using typename only for the cases we are checking it from the static list of type names
		typeName = strings.TrimSuffix(typeName, "[]")
		if utils.ContainsAnyStringFromSlice(sourceUnsupportedDataTypes, typeName) {
			unsupportedDataTypes = append(unsupportedDataTypes, allColumnsDataTypes[i])
		}
		if utils.ContainsAnyStringFromSlice(liveMigrationUnsupportedDataTypes, typeName) {
			unsupportedDataTypesForLiveMigration = append(unsupportedDataTypesForLiveMigration, allColumnsDataTypes[i])
		}
		if utils.ContainsAnyStringFromSlice(liveMigrationWithFForFBUnsupportedDatatypes, typeName) ||
			utils.ContainsAnyStringFromSlice(compositeTypes, allColumnsDataTypes[i].DataType) || // if type is UDT
			(strings.HasSuffix(allColumnsDataTypes[i].DataType, "[]") && //if type is array and is ENUM in the list
				utils.ContainsAnyStringFromSlice(enumTypes, strings.TrimSuffix(allColumnsDataTypes[i].DataType, "[]"))) {

			/*
			TODO test this for Oracle case if there is any special handling required
			For Live mgiration with FF or FB, It is meant to be for the datatypes that are going to be in YB after migration 
			so it makes sense to use the analyzeSchema `compositeTypes` or `enumTypes` and check from there but some information 
			we are still using from Source which might need a better way in case of Oracle as for PG it doesn't really makes a difference in
			source or analyzeSchema's results.
			*/
			//reporting types in the list YugabyteUnsupportedDataTypesForDbzm, UDT and array on ENUMs columns as unsupported with live migration with ff/fb
			unsupportedDataTypesForLiveMigrationWithFForFB = append(unsupportedDataTypesForLiveMigrationWithFForFB, allColumnsDataTypes[i])

		}

	}

	return unsupportedDataTypes, unsupportedDataTypesForLiveMigration, unsupportedDataTypesForLiveMigrationWithFForFB, nil
}

const (
	ORACLE_PARTITION_DEFAULT_COLOCATION = `For sharding/colocation recommendations, each partition is treated individually. During the export schema phase, all the partitions of a partitioned table are currently created as colocated by default. 
To manually modify the schema, please refer: <a class="highlight-link" href="https://github.com/yugabyte/yb-voyager/issues/1581">https://github.com/yugabyte/yb-voyager/issues/1581</a>.`

	ORACLE_UNSUPPPORTED_PARTITIONING = `Reference and System Partitioned tables are created as normal tables, but are not considered for target cluster sizing recommendations.`

	GIN_INDEXES = `There are some BITMAP indexes present in the schema that will get converted to GIN indexes, but GIN indexes are partially supported in YugabyteDB as mentioned in <a class="highlight-link" href="https://github.com/yugabyte/yugabyte-db/issues/7850">https://github.com/yugabyte/yugabyte-db/issues/7850</a> so take a look and modify them if not supported.`

	DESCRIPTION_ADD_PK_TO_PARTITION_TABLE = `After export schema, the ALTER table should be merged with CREATE table for partitioned tables as alter of partitioned tables to add primary key is not supported.`
	DESCRIPTION_FOREIGN_TABLES            = `During the export schema phase, SERVER and USER MAPPING objects are not exported. These should be manually created to make the foreign tables work.`
	DESCRIPTION_POLICY_ROLE_ISSUE         = `There are some policies that are created for certain users/roles. During the export schema phase, USERs and GRANTs are not exported. Therefore, they will have to be manually created before running import schema.`
)

const FOREIGN_TABLE_NOTE = `There are some Foreign tables in the schema, but during the export schema phase, exported schema does not include the SERVER and USER MAPPING objects. Therefore, you must manually create these objects before import schema. For more information on each of them, run analyze-schema. `

func addNotesToAssessmentReport() {
	log.Infof("adding notes to assessment report")
	switch source.DBType {
	case ORACLE:
		partitionSqlFPath := filepath.Join(assessmentMetadataDir, "schema", "partitions", "partition.sql")
		// file exists and isn't empty (containing PARTITIONs DDLs)
		if utils.FileOrFolderExists(partitionSqlFPath) && !utils.IsFileEmpty(partitionSqlFPath) {
			assessmentReport.Notes = append(assessmentReport.Notes, ORACLE_PARTITION_DEFAULT_COLOCATION)
		}
		if referenceOrTablePartitionPresent {
			assessmentReport.Notes = append(assessmentReport.Notes, ORACLE_UNSUPPPORTED_PARTITIONING)
		}

		// checking if gin indexes are present.
		for _, dbObj := range schemaAnalysisReport.SchemaSummary.DBObjects {
			if dbObj.ObjectType == "INDEX" {
				if strings.Contains(dbObj.Details, GIN_INDEX_DETAILS) {
					assessmentReport.Notes = append(assessmentReport.Notes, GIN_INDEXES)
					break
				}
			}
		}
	}
}

func addMigrationCaveatsToAssessmentReport(unsupportedDataTypesForLiveMigration []utils.TableColumnsDataTypes, unsupportedDataTypesForLiveMigrationWithFForFB []utils.TableColumnsDataTypes) {
	switch source.DBType {
	case POSTGRESQL:
		log.Infof("add migration caveats to assessment report")
		migrationCaveats := make([]UnsupportedFeature, 0)
		migrationCaveats = append(migrationCaveats, getUnsupportedFeaturesFromSchemaAnalysisReport("Alter partitioned tables to add Primary Key", ADDING_PK_TO_PARTITIONED_TABLE_ISSUE_REASON,
			schemaAnalysisReport, true, DESCRIPTION_ADD_PK_TO_PARTITION_TABLE))
		migrationCaveats = append(migrationCaveats, getUnsupportedFeaturesFromSchemaAnalysisReport("Foreign tables", FOREIGN_TABLE_ISSUE_REASON,
			schemaAnalysisReport, false, DESCRIPTION_FOREIGN_TABLES))
		migrationCaveats = append(migrationCaveats, getUnsupportedFeaturesFromSchemaAnalysisReport("Policies", POLICY_ROLE_ISSUE,
			schemaAnalysisReport, false, DESCRIPTION_POLICY_ROLE_ISSUE))

		if len(unsupportedDataTypesForLiveMigration) > 0 {
			columns := make([]ObjectInfo, 0)
			for _, col := range unsupportedDataTypesForLiveMigration {
				columns = append(columns, ObjectInfo{ObjectName: fmt.Sprintf("%s.%s.%s (%s)", col.SchemaName, col.TableName, col.ColumnName, col.DataType)})
			}
			migrationCaveats = append(migrationCaveats, UnsupportedFeature{"Unsupported Data Types for Live Migration", columns, false, UNSUPPORTED_DATATYPE_LIVE_MIGRATION_DOC_LINK, UNSUPPORTED_DATATYPES_FOR_LIVE_MIGRATION_ISSUE})
		}
		if len(unsupportedDataTypesForLiveMigrationWithFForFB) > 0 {
			columns := make([]ObjectInfo, 0)
			for _, col := range unsupportedDataTypesForLiveMigrationWithFForFB {
				columns = append(columns, ObjectInfo{ObjectName: fmt.Sprintf("%s.%s.%s (%s)", col.SchemaName, col.TableName, col.ColumnName, col.DataType)})
			}
			migrationCaveats = append(migrationCaveats, UnsupportedFeature{"Unsupported Data Types for Live Migration with Fall-forward/Fallback", columns, false, UNSUPPORTED_DATATYPE_LIVE_MIGRATION_DOC_LINK, UNSUPPORTED_DATATYPES_FOR_LIVE_MIGRATION_WITH_FF_FB_ISSUE})
		}
		for _, caveat := range migrationCaveats {
			if len(caveat.Objects) > 0 {
				//Not populating the MigrationCaveats section in case there are no caveats at all
				//TODO: fix it with proper solution in template for all sections
				assessmentReport.MigrationCaveats = migrationCaveats
				break
			}
		}
	}
}

func postProcessingOfAssessmentReport() {
	switch source.DBType {
	case ORACLE:
		log.Infof("post processing of assessment report to remove the schema name from fully qualified table names")
		for i := range assessmentReport.Sizing.SizingRecommendation.ShardedTables {
			parts := strings.Split(assessmentReport.Sizing.SizingRecommendation.ShardedTables[i], ".")
			if len(parts) > 1 {
				assessmentReport.Sizing.SizingRecommendation.ShardedTables[i] = parts[1]
			}
		}

		for i := range assessmentReport.Sizing.SizingRecommendation.ColocatedTables {
			parts := strings.Split(assessmentReport.Sizing.SizingRecommendation.ColocatedTables[i], ".")
			if len(parts) > 1 {
				assessmentReport.Sizing.SizingRecommendation.ColocatedTables[i] = parts[1]
			}
		}
	}
}

func generateAssessmentReportJson(reportDir string) error {
	jsonReportFilePath := filepath.Join(reportDir, fmt.Sprintf("%s%s", ASSESSMENT_FILE_NAME, JSON_EXTENSION))
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
	htmlReportFilePath := filepath.Join(reportDir, fmt.Sprintf("%s%s", ASSESSMENT_FILE_NAME, HTML_EXTENSION))
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
	if source.DBType == POSTGRESQL {
		// marking this as empty to not display this in html report for PG
		assessmentReport.SchemaSummary.SchemaNames = []string{}
	}
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

	source.DBType = strings.ToLower(source.DBType)
	if !slices.Contains(assessMigrationSupportedDBTypes, source.DBType) {
		utils.ErrExit("Error: Invalid source-db-type: %q. Supported source db types for assess-migration are: [%v]",
			source.DBType, strings.Join(assessMigrationSupportedDBTypes, ", "))
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
